package store

import (
	"database/sql"
	"fmt"
)

// migrations are applied in order; PRAGMA user_version records progress.
// Never edit a released migration, append a new one instead.
var migrations = []string{
	`
	CREATE TABLE site_groups (
		id         INTEGER PRIMARY KEY,
		name       TEXT    NOT NULL,
		is_default INTEGER NOT NULL DEFAULT 0,
		position   INTEGER NOT NULL DEFAULT 0,
		created_at TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
	);
	INSERT INTO site_groups (id, name, is_default, position) VALUES (1, '默认分组', 1, 0);

	CREATE TABLE domains (
		domain     TEXT PRIMARY KEY,
		icon       TEXT,
		created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
	);

	CREATE TABLE sites (
		id         INTEGER PRIMARY KEY,
		title      TEXT    NOT NULL,
		url        TEXT    NOT NULL,
		domain     TEXT    NOT NULL,
		group_id   INTEGER NOT NULL REFERENCES site_groups (id),
		position   INTEGER NOT NULL,
		created_at TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
	);
	CREATE INDEX sites_group ON sites (group_id, position);
	CREATE INDEX sites_domain ON sites (domain);
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
