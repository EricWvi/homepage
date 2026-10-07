package store

import (
	"context"
	"database/sql"
	"errors"
	"net/url"
	"strings"
	"unicode/utf8"

	"roci.dev/fracdex"
)

const (
	maxTitleLen = 200
	maxURLLen   = 2048
)

// SiteInput is the user-editable part of a site. A zero GroupID means the
// user's default group. Links replace the site's existing links.
type SiteInput struct {
	Title   string     `json:"title"`
	URL     string     `json:"url"`
	GroupID int64      `json:"groupId"`
	Links   []SiteLink `json:"links"`
}

// normalize validates the input and derives the site's domain. A URL
// without a scheme is treated as https. Links are resolved against the
// site URL and must stay on its domain.
func (in SiteInput) normalize() (SiteInput, string, error) {
	in.Title = strings.TrimSpace(in.Title)
	in.URL = strings.TrimSpace(in.URL)
	if in.Title == "" {
		return in, "", invalid("标题不能为空")
	}
	if utf8.RuneCountInString(in.Title) > maxTitleLen {
		return in, "", invalid("标题过长")
	}
	if in.URL == "" {
		return in, "", invalid("链接不能为空")
	}
	if len(in.URL) > maxURLLen {
		return in, "", invalid("链接过长")
	}
	if !strings.Contains(in.URL, "://") {
		in.URL = "https://" + in.URL
	}
	u, err := url.Parse(in.URL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" {
		return in, "", invalid("链接格式不正确")
	}
	domain := strings.ToLower(u.Hostname())
	if in.Links, err = normalizeLinks(in.Links, u, domain); err != nil {
		return in, "", err
	}
	return in, domain, nil
}

// CreateSite appends a site to its group and registers its domain if the
// domain is new to the user.
func (s *Store) CreateSite(ctx context.Context, userID int64, in SiteInput) (Site, error) {
	in, domain, err := in.normalize()
	if err != nil {
		return Site{}, err
	}
	site := Site{Title: in.Title, URL: in.URL, Domain: domain, Links: in.Links}
	err = s.inTx(ctx, func(tx *sql.Tx) error {
		if site.GroupID, err = resolveGroup(ctx, tx, userID, in.GroupID); err != nil {
			return err
		}
		if site.SortKey, err = nextSortKey(ctx, tx, userID, site.GroupID); err != nil {
			return err
		}
		if err := ensureDomain(ctx, tx, userID, domain); err != nil {
			return err
		}
		res, err := tx.ExecContext(ctx,
			`INSERT INTO sites (user_id, title, url, domain, group_id, sort_key) VALUES (?, ?, ?, ?, ?, ?)`,
			userID, site.Title, site.URL, site.Domain, site.GroupID, site.SortKey)
		if err != nil {
			return err
		}
		if site.ID, err = res.LastInsertId(); err != nil {
			return err
		}
		return replaceLinks(ctx, tx, site.ID, site.Links)
	})
	return site, err
}

// UpdateSite edits a site. Moving it to another group appends it there.
func (s *Store) UpdateSite(ctx context.Context, userID, id int64, in SiteInput) error {
	in, domain, err := in.normalize()
	if err != nil {
		return err
	}
	return s.inTx(ctx, func(tx *sql.Tx) error {
		var groupID int64
		var sortKey string
		err := tx.QueryRowContext(ctx,
			`SELECT group_id, sort_key FROM sites WHERE id = ? AND user_id = ?`, id, userID).Scan(&groupID, &sortKey)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		target, err := resolveGroup(ctx, tx, userID, in.GroupID)
		if err != nil {
			return err
		}
		if target != groupID {
			if sortKey, err = nextSortKey(ctx, tx, userID, target); err != nil {
				return err
			}
		}
		if err := ensureDomain(ctx, tx, userID, domain); err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx,
			`UPDATE sites SET title = ?, url = ?, domain = ?, group_id = ?, sort_key = ? WHERE id = ?`,
			in.Title, in.URL, domain, target, sortKey, id)
		if err != nil {
			return err
		}
		return replaceLinks(ctx, tx, id, in.Links)
	})
}

// MoveSite moves a site within its group to just after the site with id
// after, or to the front when after is 0.
func (s *Store) MoveSite(ctx context.Context, userID, id, after int64) error {
	return s.inTx(ctx, func(tx *sql.Tx) error {
		var groupID int64
		err := tx.QueryRowContext(ctx,
			`SELECT group_id FROM sites WHERE id = ? AND user_id = ?`, id, userID).Scan(&groupID)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		var lower string
		if after != 0 {
			if after == id {
				return invalid("不能移动到自身之后")
			}
			err := tx.QueryRowContext(ctx,
				`SELECT sort_key FROM sites WHERE id = ? AND user_id = ? AND group_id = ?`,
				after, userID, groupID).Scan(&lower)
			if errors.Is(err, sql.ErrNoRows) {
				return invalid("只能在同一分组内移动")
			}
			if err != nil {
				return err
			}
		}
		var upper string
		err = tx.QueryRowContext(ctx,
			`SELECT COALESCE(MIN(sort_key), '') FROM sites
			 WHERE user_id = ? AND group_id = ? AND id != ? AND sort_key > ?`,
			userID, groupID, id, lower).Scan(&upper)
		if err != nil {
			return err
		}
		key, err := fracdex.KeyBetween(lower, upper)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `UPDATE sites SET sort_key = ? WHERE id = ?`, key, id)
		return err
	})
}

// DeleteSite removes a site and its links. Its domain and icon are kept.
func (s *Store) DeleteSite(ctx context.Context, userID, id int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM sites WHERE id = ? AND user_id = ?`, id, userID)
	if err != nil {
		return err
	}
	return requireAffected(res)
}

// resolveGroup maps a requested group id to one the user owns; 0 selects
// the default group.
func resolveGroup(ctx context.Context, tx *sql.Tx, userID, groupID int64) (int64, error) {
	if groupID == 0 {
		return defaultGroupID(ctx, tx, userID)
	}
	var one int
	err := tx.QueryRowContext(ctx,
		`SELECT 1 FROM site_groups WHERE id = ? AND user_id = ?`, groupID, userID).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, invalid("分组不存在")
	}
	return groupID, err
}

// lastSortKey returns the key of the group's last site, or "" when it is empty.
func lastSortKey(ctx context.Context, tx *sql.Tx, userID, groupID int64) (string, error) {
	var key string
	err := tx.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(sort_key), '') FROM sites WHERE user_id = ? AND group_id = ?`,
		userID, groupID).Scan(&key)
	return key, err
}

// nextSortKey returns a key that places a site at the end of the group.
func nextSortKey(ctx context.Context, tx *sql.Tx, userID, groupID int64) (string, error) {
	last, err := lastSortKey(ctx, tx, userID, groupID)
	if err != nil {
		return "", err
	}
	return fracdex.KeyBetween(last, "")
}

func ensureDomain(ctx context.Context, tx *sql.Tx, userID int64, domain string) error {
	_, err := tx.ExecContext(ctx,
		`INSERT OR IGNORE INTO domains (user_id, domain) VALUES (?, ?)`, userID, domain)
	return err
}
