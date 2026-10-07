package store

import (
	"context"
	"database/sql"
	"errors"
	"net/url"
	"strings"
	"unicode/utf8"
)

const (
	maxTitleLen = 200
	maxURLLen   = 2048
)

// SiteInput is the user-editable part of a site. A zero GroupID means the
// user's default group.
type SiteInput struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	GroupID int64  `json:"groupId"`
}

// normalize validates the input and derives the site's domain. A URL
// without a scheme is treated as https.
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
	return in, strings.ToLower(u.Hostname()), nil
}

// CreateSite appends a site to its group and registers its domain if the
// domain is new to the user.
func (s *Store) CreateSite(ctx context.Context, userID int64, in SiteInput) (Site, error) {
	in, domain, err := in.normalize()
	if err != nil {
		return Site{}, err
	}
	site := Site{Title: in.Title, URL: in.URL, Domain: domain}
	err = s.inTx(ctx, func(tx *sql.Tx) error {
		if site.GroupID, err = resolveGroup(ctx, tx, userID, in.GroupID); err != nil {
			return err
		}
		if site.Position, err = nextSitePosition(ctx, tx, userID, site.GroupID); err != nil {
			return err
		}
		if err := ensureDomain(ctx, tx, userID, domain); err != nil {
			return err
		}
		res, err := tx.ExecContext(ctx,
			`INSERT INTO sites (user_id, title, url, domain, group_id, position) VALUES (?, ?, ?, ?, ?, ?)`,
			userID, site.Title, site.URL, site.Domain, site.GroupID, site.Position)
		if err != nil {
			return err
		}
		site.ID, err = res.LastInsertId()
		return err
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
		var position int
		err := tx.QueryRowContext(ctx,
			`SELECT group_id, position FROM sites WHERE id = ? AND user_id = ?`, id, userID).Scan(&groupID, &position)
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
			if position, err = nextSitePosition(ctx, tx, userID, target); err != nil {
				return err
			}
		}
		if err := ensureDomain(ctx, tx, userID, domain); err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx,
			`UPDATE sites SET title = ?, url = ?, domain = ?, group_id = ?, position = ? WHERE id = ?`,
			in.Title, in.URL, domain, target, position, id)
		return err
	})
}

// DeleteSite removes a site. Its domain and icon are kept.
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

func nextSitePosition(ctx context.Context, tx *sql.Tx, userID, groupID int64) (int, error) {
	var pos int
	err := tx.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(position), 0) + 1 FROM sites WHERE user_id = ? AND group_id = ?`,
		userID, groupID).Scan(&pos)
	return pos, err
}

func ensureDomain(ctx context.Context, tx *sql.Tx, userID int64, domain string) error {
	_, err := tx.ExecContext(ctx,
		`INSERT OR IGNORE INTO domains (user_id, domain) VALUES (?, ?)`, userID, domain)
	return err
}
