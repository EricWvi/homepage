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

// SiteInput is the user-editable part of a site.
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
	if in.GroupID == 0 {
		in.GroupID = DefaultGroupID
	}
	return in, strings.ToLower(u.Hostname()), nil
}

// CreateSite appends a site to its group and registers its domain if the
// domain is new.
func (s *Store) CreateSite(ctx context.Context, in SiteInput) (Site, error) {
	in, domain, err := in.normalize()
	if err != nil {
		return Site{}, err
	}
	site := Site{Title: in.Title, URL: in.URL, Domain: domain, GroupID: in.GroupID}
	err = s.inTx(ctx, func(tx *sql.Tx) error {
		if err := checkGroup(ctx, tx, in.GroupID); err != nil {
			return err
		}
		if site.Position, err = nextSitePosition(ctx, tx, in.GroupID); err != nil {
			return err
		}
		if err := ensureDomain(ctx, tx, domain); err != nil {
			return err
		}
		res, err := tx.ExecContext(ctx,
			`INSERT INTO sites (title, url, domain, group_id, position) VALUES (?, ?, ?, ?, ?)`,
			site.Title, site.URL, site.Domain, site.GroupID, site.Position)
		if err != nil {
			return err
		}
		site.ID, err = res.LastInsertId()
		return err
	})
	return site, err
}

// UpdateSite edits a site. Moving it to another group appends it there.
func (s *Store) UpdateSite(ctx context.Context, id int64, in SiteInput) error {
	in, domain, err := in.normalize()
	if err != nil {
		return err
	}
	return s.inTx(ctx, func(tx *sql.Tx) error {
		var groupID int64
		var position int
		err := tx.QueryRowContext(ctx,
			`SELECT group_id, position FROM sites WHERE id = ?`, id).Scan(&groupID, &position)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if in.GroupID != groupID {
			if err := checkGroup(ctx, tx, in.GroupID); err != nil {
				return err
			}
			if position, err = nextSitePosition(ctx, tx, in.GroupID); err != nil {
				return err
			}
		}
		if err := ensureDomain(ctx, tx, domain); err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx,
			`UPDATE sites SET title = ?, url = ?, domain = ?, group_id = ?, position = ? WHERE id = ?`,
			in.Title, in.URL, domain, in.GroupID, position, id)
		return err
	})
}

// DeleteSite removes a site. Its domain and icon are kept.
func (s *Store) DeleteSite(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM sites WHERE id = ?`, id)
	if err != nil {
		return err
	}
	return requireAffected(res)
}

func checkGroup(ctx context.Context, tx *sql.Tx, id int64) error {
	err := groupExists(ctx, tx, id)
	if errors.Is(err, ErrNotFound) {
		return invalid("分组不存在")
	}
	return err
}

func nextSitePosition(ctx context.Context, tx *sql.Tx, groupID int64) (int, error) {
	var pos int
	err := tx.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(position), 0) + 1 FROM sites WHERE group_id = ?`, groupID).Scan(&pos)
	return pos, err
}

func ensureDomain(ctx context.Context, tx *sql.Tx, domain string) error {
	_, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO domains (domain) VALUES (?)`, domain)
	return err
}
