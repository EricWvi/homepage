package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"unicode/utf8"

	"roci.dev/fracdex"
)

const maxNameLen = 100

func cleanName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", invalid("分组名称不能为空")
	}
	if utf8.RuneCountInString(name) > maxNameLen {
		return "", invalid("分组名称过长")
	}
	return name, nil
}

// CreateGroup appends a custom group after the user's existing ones.
func (s *Store) CreateGroup(ctx context.Context, userID int64, name string) (Group, error) {
	name, err := cleanName(name)
	if err != nil {
		return Group{}, err
	}
	g := Group{Name: name}
	err = s.inTx(ctx, func(tx *sql.Tx) error {
		if err := tx.QueryRowContext(ctx,
			`SELECT COALESCE(MAX(position), 0) + 1 FROM site_groups WHERE user_id = ? AND is_default = 0`,
			userID).Scan(&g.Position); err != nil {
			return err
		}
		res, err := tx.ExecContext(ctx,
			`INSERT INTO site_groups (user_id, name, position) VALUES (?, ?, ?)`, userID, g.Name, g.Position)
		if err != nil {
			return err
		}
		g.ID, err = res.LastInsertId()
		return err
	})
	return g, err
}

// RenameGroup renames a custom group. The default group has no visible
// title and cannot be renamed.
func (s *Store) RenameGroup(ctx context.Context, userID, id int64, name string) error {
	name, err := cleanName(name)
	if err != nil {
		return err
	}
	return s.inTx(ctx, func(tx *sql.Tx) error {
		if err := requireCustomGroup(ctx, tx, userID, id, "默认分组不能重命名"); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `UPDATE site_groups SET name = ? WHERE id = ?`, name, id)
		return err
	})
}

// SetGroupHiddenAtWork chooses whether a custom group is left out in work
// mode. The default group is always shown.
func (s *Store) SetGroupHiddenAtWork(ctx context.Context, userID, id int64, hidden bool) error {
	return s.inTx(ctx, func(tx *sql.Tx) error {
		if err := requireCustomGroup(ctx, tx, userID, id, "默认分组始终显示"); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `UPDATE site_groups SET hidden_at_work = ? WHERE id = ?`, hidden, id)
		return err
	})
}

// DeleteGroup deletes a custom group and moves its sites, in order, to the
// end of the default group.
func (s *Store) DeleteGroup(ctx context.Context, userID, id int64) error {
	return s.inTx(ctx, func(tx *sql.Tx) error {
		if err := requireCustomGroup(ctx, tx, userID, id, "默认分组不能删除"); err != nil {
			return err
		}
		defaultID, err := defaultGroupID(ctx, tx, userID)
		if err != nil {
			return err
		}
		rows, err := tx.QueryContext(ctx,
			`SELECT id FROM sites WHERE user_id = ? AND group_id = ? ORDER BY sort_key, id`, userID, id)
		if err != nil {
			return err
		}
		var siteIDs []int64
		for rows.Next() {
			var sid int64
			if err := rows.Scan(&sid); err != nil {
				rows.Close()
				return err
			}
			siteIDs = append(siteIDs, sid)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		last, err := lastSortKey(ctx, tx, userID, defaultID)
		if err != nil {
			return err
		}
		keys, err := fracdex.NKeysBetween(last, "", uint(len(siteIDs)))
		if err != nil {
			return err
		}
		for i, sid := range siteIDs {
			if _, err := tx.ExecContext(ctx,
				`UPDATE sites SET group_id = ?, sort_key = ? WHERE id = ?`, defaultID, keys[i], sid); err != nil {
				return err
			}
		}
		_, err = tx.ExecContext(ctx, `DELETE FROM site_groups WHERE id = ?`, id)
		return err
	})
}

// ReorderGroups sets the order of the user's custom groups. ids must list
// every custom group exactly once.
func (s *Store) ReorderGroups(ctx context.Context, userID int64, ids []int64) error {
	return s.inTx(ctx, func(tx *sql.Tx) error {
		var count int
		if err := tx.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM site_groups WHERE user_id = ? AND is_default = 0`, userID).Scan(&count); err != nil {
			return err
		}
		if len(ids) != count {
			return invalid("分组列表与当前分组不一致")
		}
		seen := make(map[int64]bool, len(ids))
		for i, id := range ids {
			if seen[id] {
				return invalid("分组列表包含重复项")
			}
			seen[id] = true
			res, err := tx.ExecContext(ctx,
				`UPDATE site_groups SET position = ? WHERE id = ? AND user_id = ? AND is_default = 0`,
				i+1, id, userID)
			if err != nil {
				return err
			}
			if n, _ := res.RowsAffected(); n == 0 {
				return invalid("分组列表与当前分组不一致")
			}
		}
		return nil
	})
}

// requireCustomGroup reports ErrNotFound for groups of other users and
// rejects the default group with msg.
func requireCustomGroup(ctx context.Context, tx *sql.Tx, userID, id int64, msg string) error {
	var isDefault bool
	err := tx.QueryRowContext(ctx,
		`SELECT is_default FROM site_groups WHERE id = ? AND user_id = ?`, id, userID).Scan(&isDefault)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if isDefault {
		return invalid(msg)
	}
	return nil
}

func defaultGroupID(ctx context.Context, tx *sql.Tx, userID int64) (int64, error) {
	var id int64
	err := tx.QueryRowContext(ctx,
		`SELECT id FROM site_groups WHERE user_id = ? AND is_default = 1`, userID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrNotFound
	}
	return id, err
}

func requireAffected(res sql.Result) error {
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
