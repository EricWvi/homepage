// Package store persists groups, sites and domain icons.
//
// Metadata lives in SQLite; icon bytes live as files in <data_dir>/icons,
// named by a content hash so they can be served with a long-lived HTTP cache.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite" // CGO-free SQLite driver
)

// DefaultGroupID is the id of the built-in group that has no title and
// always sits at the top of the page.
const DefaultGroupID int64 = 1

var (
	// ErrNotFound reports a missing group, site or domain.
	ErrNotFound = errors.New("not found")
	// ErrInvalid reports rejected input; the wrapped message is user-facing.
	ErrInvalid = errors.New("invalid input")
	// ErrConflict reports an operation that the current state forbids.
	ErrConflict = errors.New("conflict")
)

// Group is a section of the page. The default group has no title.
type Group struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	IsDefault bool   `json:"isDefault"`
	Position  int    `json:"position"`
}

// Site is a saved link.
type Site struct {
	ID       int64  `json:"id"`
	Title    string `json:"title"`
	URL      string `json:"url"`
	Domain   string `json:"domain"`
	GroupID  int64  `json:"groupId"`
	Position int    `json:"position"`
}

// Domain owns the icon shown for every site on that host. Domains outlive
// their sites and are only removed explicitly.
type Domain struct {
	Domain string `json:"domain"`
	// Icon is the hashed icon file name, or nil when none was uploaded.
	Icon      *string `json:"icon"`
	SiteCount int     `json:"siteCount"`
}

// Snapshot is the complete state the frontend renders from.
type Snapshot struct {
	Groups  []Group  `json:"groups"`
	Sites   []Site   `json:"sites"`
	Domains []Domain `json:"domains"`
}

// Store is safe for concurrent use.
type Store struct {
	db      *sql.DB
	iconDir string
}

// Open opens (and migrates) the database inside dataDir.
func Open(dataDir string) (*Store, error) {
	iconDir := filepath.Join(dataDir, "icons")
	if err := os.MkdirAll(iconDir, 0o755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}
	dsn := "file:" + filepath.Join(dataDir, "homepage.db") +
		"?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	// A single connection serialises writes and keeps transactions simple.
	db.SetMaxOpenConns(1)
	if err := migrate(db); err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db: db, iconDir: iconDir}, nil
}

// Close closes the database.
func (s *Store) Close() error { return s.db.Close() }

// Snapshot returns all groups, sites and domains in display order.
func (s *Store) Snapshot(ctx context.Context) (Snapshot, error) {
	snap := Snapshot{Groups: []Group{}, Sites: []Site{}, Domains: []Domain{}}

	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, is_default, position FROM site_groups ORDER BY is_default DESC, position, id`)
	if err != nil {
		return snap, err
	}
	for rows.Next() {
		var g Group
		if err := rows.Scan(&g.ID, &g.Name, &g.IsDefault, &g.Position); err != nil {
			rows.Close()
			return snap, err
		}
		snap.Groups = append(snap.Groups, g)
	}
	rows.Close()

	rows, err = s.db.QueryContext(ctx,
		`SELECT id, title, url, domain, group_id, position FROM sites ORDER BY group_id, position, id`)
	if err != nil {
		return snap, err
	}
	for rows.Next() {
		var st Site
		if err := rows.Scan(&st.ID, &st.Title, &st.URL, &st.Domain, &st.GroupID, &st.Position); err != nil {
			rows.Close()
			return snap, err
		}
		snap.Sites = append(snap.Sites, st)
	}
	rows.Close()

	rows, err = s.db.QueryContext(ctx, `
		SELECT d.domain, d.icon, COUNT(s.id)
		FROM domains d LEFT JOIN sites s ON s.domain = d.domain
		GROUP BY d.domain ORDER BY d.domain`)
	if err != nil {
		return snap, err
	}
	defer rows.Close()
	for rows.Next() {
		var d Domain
		if err := rows.Scan(&d.Domain, &d.Icon, &d.SiteCount); err != nil {
			return snap, err
		}
		snap.Domains = append(snap.Domains, d)
	}
	return snap, rows.Err()
}

func (s *Store) inTx(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

func invalid(msg string) error { return fmt.Errorf("%w: %s", ErrInvalid, msg) }
