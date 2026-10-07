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

var (
	// ErrNotFound reports a missing group, site or domain.
	ErrNotFound = errors.New("not found")
	// ErrInvalid reports rejected input; the wrapped message is user-facing.
	ErrInvalid = errors.New("invalid input")
	// ErrConflict reports an operation that the current state forbids.
	ErrConflict = errors.New("conflict")
)

// Group is a section of the page. Every user has exactly one default
// group, which has no title and always sits at the top.
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
	// Links are other pages on the same domain, in the user's order.
	Links []SiteLink `json:"links"`
}

// SiteLink is a quick link to another page on its site's domain.
type SiteLink struct {
	Title string `json:"title"`
	URL   string `json:"url"`
}

// Domain owns the icon shown for every site on that host. Domains outlive
// their sites and are only removed explicitly.
type Domain struct {
	Domain string `json:"domain"`
	// Icon is the hashed icon file name, or nil when none was uploaded.
	Icon      *string `json:"icon"`
	SiteCount int     `json:"siteCount"`
}

// Wallpaper is an image in the user's wallpaper library.
type Wallpaper struct {
	ID int64 `json:"id"`
	// File and Thumb are hashed file names under /wallpapers/.
	File   string `json:"file"`
	Thumb  string `json:"thumb"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

// Snapshot is the complete state the frontend renders from.
type Snapshot struct {
	Groups  []Group  `json:"groups"`
	Sites   []Site   `json:"sites"`
	Domains []Domain `json:"domains"`
	// Wallpapers are newest first.
	Wallpapers []Wallpaper `json:"wallpapers"`
	// CurrentWallpaper is the wallpaper last shown, or nil.
	CurrentWallpaper *int64 `json:"currentWallpaperId"`
}

// Store is safe for concurrent use. Every query on groups, sites and
// domains is scoped to one user.
type Store struct {
	db           *sql.DB
	iconDir      string
	wallpaperDir string
}

// Open opens (and migrates) the database inside dataDir.
func Open(dataDir string) (*Store, error) {
	iconDir := filepath.Join(dataDir, "icons")
	wallpaperDir := filepath.Join(dataDir, "wallpapers")
	for _, dir := range []string{iconDir, wallpaperDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create data dir: %w", err)
		}
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
	return &Store{db: db, iconDir: iconDir, wallpaperDir: wallpaperDir}, nil
}

// Close closes the database.
func (s *Store) Close() error { return s.db.Close() }

// Snapshot returns the user's groups, sites and domains in display order.
func (s *Store) Snapshot(ctx context.Context, userID int64) (Snapshot, error) {
	snap := Snapshot{Groups: []Group{}, Sites: []Site{}, Domains: []Domain{}, Wallpapers: []Wallpaper{}}

	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, is_default, position FROM site_groups
		 WHERE user_id = ? ORDER BY is_default DESC, position, id`, userID)
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
		`SELECT id, title, url, domain, group_id, position FROM sites
		 WHERE user_id = ? ORDER BY group_id, position, id`, userID)
	if err != nil {
		return snap, err
	}
	siteIndex := map[int64]int{}
	for rows.Next() {
		st := Site{Links: []SiteLink{}}
		if err := rows.Scan(&st.ID, &st.Title, &st.URL, &st.Domain, &st.GroupID, &st.Position); err != nil {
			rows.Close()
			return snap, err
		}
		siteIndex[st.ID] = len(snap.Sites)
		snap.Sites = append(snap.Sites, st)
	}
	rows.Close()

	rows, err = s.db.QueryContext(ctx,
		`SELECT l.site_id, l.title, l.url FROM site_links l JOIN sites s ON s.id = l.site_id
		 WHERE s.user_id = ? ORDER BY l.site_id, l.position`, userID)
	if err != nil {
		return snap, err
	}
	for rows.Next() {
		var siteID int64
		var l SiteLink
		if err := rows.Scan(&siteID, &l.Title, &l.URL); err != nil {
			rows.Close()
			return snap, err
		}
		if i, ok := siteIndex[siteID]; ok {
			snap.Sites[i].Links = append(snap.Sites[i].Links, l)
		}
	}
	rows.Close()

	rows, err = s.db.QueryContext(ctx, `
		SELECT d.domain, d.icon, COUNT(s.id)
		FROM domains d LEFT JOIN sites s ON s.user_id = d.user_id AND s.domain = d.domain
		WHERE d.user_id = ?
		GROUP BY d.domain ORDER BY d.domain`, userID)
	if err != nil {
		return snap, err
	}
	for rows.Next() {
		var d Domain
		if err := rows.Scan(&d.Domain, &d.Icon, &d.SiteCount); err != nil {
			rows.Close()
			return snap, err
		}
		snap.Domains = append(snap.Domains, d)
	}
	rows.Close()

	if snap.Wallpapers, err = s.wallpapers(ctx, userID); err != nil {
		return snap, err
	}
	err = s.db.QueryRowContext(ctx, `SELECT wallpaper_id FROM users WHERE id = ?`, userID).Scan(&snap.CurrentWallpaper)
	if errors.Is(err, sql.ErrNoRows) {
		err = nil
	}
	return snap, err
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
