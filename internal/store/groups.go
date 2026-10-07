package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"unicode/utf8"
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

// CreateGroup appends a custom group after the existing ones.
func (s *Store) CreateGroup(ctx context.Context, name string) (Group, error) {
	name, err := cleanName(name)
	if err != nil {
		return Group{}, err
	}
	g := Group{Name: name}
	err = s.inTx(ctx, func(tx *sql.Tx) error {
		if err := tx.QueryRowContext(ctx,
			`SELECT COALESCE(MAX(position), 0) + 1 FROM site_groups WHERE is_default = 0`).Scan(&g.Position); err != nil {
			return err
		}
		res, err := tx.ExecContext(ctx,
			`INSERT INTO site_groups (name, position) VALUES (?, ?)`, g.Name, g.Position)
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
func (s *Store) RenameGroup(ctx context.Context, id int64, name string) error {
	if id == DefaultGroupID {
		return invalid("默认分组不能重命名")
	}
	name, err := cleanName(name)
	if err != nil {
		return err
	}
	res, err := s.db.ExecContext(ctx, `UPDATE site_groups SET name = ? WHERE id = ?`, name, id)
	if err != nil {
		return err
	}
	return requireAffected(res)
}

// DeleteGroup deletes a custom group and moves its sites, in order, to the
// end of the default group.
func (s *Store) DeleteGroup(ctx context.Context, id int64) error {
	if id == DefaultGroupID {
		return invalid("默认分组不能删除")
	}
	return s.inTx(ctx, func(tx *sql.Tx) error {
		if err := groupExists(ctx, tx, id); err != nil {
			return err
		}
		pos, err := nextSitePosition(ctx, tx, DefaultGroupID)
		if err != nil {
			return err
		}
		rows, err := tx.QueryContext(ctx,
			`SELECT id FROM sites WHERE group_id = ? ORDER BY position, id`, id)
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
		for i, sid := range siteIDs {
			if _, err := tx.ExecContext(ctx,
				`UPDATE sites SET group_id = ?, position = ? WHERE id = ?`, DefaultGroupID, pos+i, sid); err != nil {
				return err
			}
		}
		_, err = tx.ExecContext(ctx, `DELETE FROM site_groups WHERE id = ?`, id)
		return err
	})
}

// ReorderGroups sets the order of the custom groups. ids must list every
// custom group exactly once.
func (s *Store) ReorderGroups(ctx context.Context, ids []int64) error {
	return s.inTx(ctx, func(tx *sql.Tx) error {
		var count int
		if err := tx.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM site_groups WHERE is_default = 0`).Scan(&count); err != nil {
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
				`UPDATE site_groups SET position = ? WHERE id = ? AND is_default = 0`, i+1, id)
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

func groupExists(ctx context.Context, tx *sql.Tx, id int64) error {
	var one int
	err := tx.QueryRowContext(ctx, `SELECT 1 FROM site_groups WHERE id = ?`, id).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	return err
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
