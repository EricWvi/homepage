package store

import (
	"database/sql"
	"fmt"
)

// migrations are applied in order; PRAGMA user_version records progress.
// Never edit a released migration, append a new one instead.
var migrations = []string{
	`
	CREATE TABLE users (
		id         INTEGER PRIMARY KEY,
		issuer     TEXT NOT NULL,
		subject    TEXT NOT NULL,
		name       TEXT NOT NULL DEFAULT '',
		email      TEXT NOT NULL DEFAULT '',
		created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
		UNIQUE (issuer, subject)
	);

	-- Sessions never expire; a row lives until the user signs out.
	CREATE TABLE sessions (
		token_hash   TEXT    PRIMARY KEY,
		user_id      INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
		created_at   INTEGER NOT NULL,
		last_seen_at INTEGER NOT NULL
	);
	CREATE INDEX sessions_user ON sessions (user_id);

	CREATE TABLE site_groups (
		id         INTEGER PRIMARY KEY,
		user_id    INTEGER NOT NULL REFERENCES users (id),
		name       TEXT    NOT NULL,
		is_default INTEGER NOT NULL DEFAULT 0,
		position   INTEGER NOT NULL DEFAULT 0,
		created_at TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
		UNIQUE (user_id, id)
	);
	CREATE UNIQUE INDEX site_groups_one_default ON site_groups (user_id) WHERE is_default = 1;

	CREATE TABLE domains (
		user_id    INTEGER NOT NULL REFERENCES users (id),
		domain     TEXT    NOT NULL,
		icon       TEXT,
		created_at TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
		PRIMARY KEY (user_id, domain)
	);
	CREATE INDEX domains_icon ON domains (icon);

	-- Composite keys make it impossible for a site to point at another
	-- user's group or domain.
	CREATE TABLE sites (
		id         INTEGER PRIMARY KEY,
		user_id    INTEGER NOT NULL,
		title      TEXT    NOT NULL,
		url        TEXT    NOT NULL,
		domain     TEXT    NOT NULL,
		group_id   INTEGER NOT NULL,
		position   INTEGER NOT NULL,
		created_at TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
		FOREIGN KEY (user_id, group_id) REFERENCES site_groups (user_id, id),
		FOREIGN KEY (user_id, domain) REFERENCES domains (user_id, domain)
	);
	CREATE INDEX sites_group ON sites (user_id, group_id, position);
	CREATE INDEX sites_domain ON sites (user_id, domain);
	`,
	`
	-- Quick links to other pages on a site's domain. They belong to the
	-- site, so ownership follows sites.user_id.
	CREATE TABLE site_links (
		id       INTEGER PRIMARY KEY,
		site_id  INTEGER NOT NULL REFERENCES sites (id) ON DELETE CASCADE,
		title    TEXT    NOT NULL,
		url      TEXT    NOT NULL,
		position INTEGER NOT NULL
	);
	CREATE INDEX site_links_site ON site_links (site_id, position);
	`,
	`
	-- Files are named by content hash and shared between users, like icons.
	CREATE TABLE wallpapers (
		id         INTEGER PRIMARY KEY,
		user_id    INTEGER NOT NULL REFERENCES users (id),
		file       TEXT    NOT NULL,
		thumb      TEXT    NOT NULL,
		width      INTEGER NOT NULL,
		height     INTEGER NOT NULL,
		created_at TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
		UNIQUE (user_id, file)
	);
	CREATE INDEX wallpapers_file ON wallpapers (file);

	ALTER TABLE users ADD COLUMN wallpaper_id INTEGER REFERENCES wallpapers (id) ON DELETE SET NULL;
	`,
}

func migrate(db *sql.DB) error {
	var version int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}
	for i := version; i < len(migrations); i++ {
		tx, err := db.Begin()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(migrations[i]); err != nil {
			tx.Rollback()
			return fmt.Errorf("migration %d: %w", i+1, err)
		}
		if _, err := tx.Exec(fmt.Sprintf(`PRAGMA user_version = %d`, i+1)); err != nil {
			tx.Rollback()
			return fmt.Errorf("migration %d: %w", i+1, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("migration %d: %w", i+1, err)
		}
	}
	return nil
}
