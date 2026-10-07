package store

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"slices"
	"testing"
)

func TestSortKeysReplaceSitePositions(t *testing.T) {
	dir := t.TempDir()
	db, err := sql.Open("sqlite", "file:"+filepath.Join(dir, "homepage.db")+"?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatal(err)
	}
	// Bring the schema up to the last version with integer positions.
	before := len(migrations) - 1
	for i, m := range migrations[:before] {
		tx, _ := db.Begin()
		if err := m(tx); err != nil {
			t.Fatalf("migration %d: %v", i+1, err)
		}
		tx.Commit()
	}
	_, err = db.Exec(fmt.Sprintf(`PRAGMA user_version = %d;`, before) + `
		INSERT INTO users (id, issuer, subject) VALUES (1, 'i', 'alice');
		INSERT INTO site_groups (id, user_id, name, is_default) VALUES (1, 1, '默认分组', 1), (2, 1, 'g', 0);
		INSERT INTO domains (user_id, domain) VALUES (1, 'x.com');
		INSERT INTO sites (id, user_id, title, url, domain, group_id, position) VALUES
			(1, 1, 'third',  'x.com', 'x.com', 1, 7),
			(2, 1, 'first',  'x.com', 'x.com', 1, 1),
			(3, 1, 'other',  'x.com', 'x.com', 2, 1),
			(4, 1, 'second', 'x.com', 'x.com', 1, 3);
	`)
	db.Close()
	if err != nil {
		t.Fatal(err)
	}

	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var keys []string
	for _, st := range snapshot(t, s, 1).Sites {
		keys = append(keys, st.Title+":"+st.SortKey)
	}
	if want := []string{"first:a0", "second:a1", "third:a2", "other:a0"}; !slices.Equal(keys, want) {
		t.Fatalf("sites = %v, want %v", keys, want)
	}
}
