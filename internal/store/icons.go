package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// iconName matches the file names produced by SetDomainIcon.
var iconName = regexp.MustCompile(`^[0-9a-f]{16}\.(png|jpg|gif|webp|ico|svg)$`)

// SetDomainIcon stores data as the icon for domain. The file is named after
// a hash of its content, so a changed icon always gets a new URL.
func (s *Store) SetDomainIcon(ctx context.Context, domain string, data []byte) (string, error) {
	ext, err := iconExt(data)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	name := hex.EncodeToString(sum[:8]) + ext
	if err := s.writeIcon(name, data); err != nil {
		return "", err
	}
	var old sql.NullString
	err = s.inTx(ctx, func(tx *sql.Tx) error {
		err := tx.QueryRowContext(ctx, `SELECT icon FROM domains WHERE domain = ?`, domain).Scan(&old)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `UPDATE domains SET icon = ? WHERE domain = ?`, name, domain)
		return err
	})
	if err != nil {
		s.removeIconIfUnused(context.WithoutCancel(ctx), name)
		return "", err
	}
	if old.Valid && old.String != name {
		s.removeIconIfUnused(ctx, old.String)
	}
	return name, nil
}

// ClearDomainIcon removes the uploaded icon of domain.
func (s *Store) ClearDomainIcon(ctx context.Context, domain string) error {
	old, err := s.domainIcon(ctx, domain)
	if err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE domains SET icon = NULL WHERE domain = ?`, domain); err != nil {
		return err
	}
	if old.Valid {
		s.removeIconIfUnused(ctx, old.String)
	}
	return nil
}

// DeleteDomain removes a domain that no site uses any more.
func (s *Store) DeleteDomain(ctx context.Context, domain string) error {
	var old sql.NullString
	err := s.inTx(ctx, func(tx *sql.Tx) error {
		err := tx.QueryRowContext(ctx, `SELECT icon FROM domains WHERE domain = ?`, domain).Scan(&old)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		var used int
		if err := tx.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM sites WHERE domain = ?`, domain).Scan(&used); err != nil {
			return err
		}
		if used > 0 {
			return fmt.Errorf("%w: 仍有 %d 个网站使用该域名", ErrConflict, used)
		}
		_, err = tx.ExecContext(ctx, `DELETE FROM domains WHERE domain = ?`, domain)
		return err
	})
	if err != nil {
		return err
	}
	if old.Valid {
		s.removeIconIfUnused(ctx, old.String)
	}
	return nil
}

// IconPath returns the file path for an icon name, or ErrNotFound for names
// SetDomainIcon could not have produced.
func (s *Store) IconPath(name string) (string, error) {
	if !iconName.MatchString(name) {
		return "", ErrNotFound
	}
	return filepath.Join(s.iconDir, name), nil
}

func (s *Store) domainIcon(ctx context.Context, domain string) (sql.NullString, error) {
	var icon sql.NullString
	err := s.db.QueryRowContext(ctx, `SELECT icon FROM domains WHERE domain = ?`, domain).Scan(&icon)
	if errors.Is(err, sql.ErrNoRows) {
		return icon, ErrNotFound
	}
	return icon, err
}

func (s *Store) writeIcon(name string, data []byte) error {
	path := filepath.Join(s.iconDir, name)
	if _, err := os.Stat(path); err == nil {
		return nil // same content already stored
	}
	tmp, err := os.CreateTemp(s.iconDir, ".upload-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// removeIconIfUnused deletes an icon file once no domain points at it.
// Several domains may share one file when their icons are identical.
func (s *Store) removeIconIfUnused(ctx context.Context, name string) {
	var used int
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM domains WHERE icon = ?`, name).Scan(&used); err != nil || used > 0 {
		return
	}
	if path, err := s.IconPath(name); err == nil {
		os.Remove(path)
	}
}

// iconExt sniffs the image format; only common icon formats are accepted.
func iconExt(data []byte) (string, error) {
	if len(data) == 0 {
		return "", invalid("图标文件为空")
	}
	switch http.DetectContentType(data) {
	case "image/png":
		return ".png", nil
	case "image/jpeg":
		return ".jpg", nil
	case "image/gif":
		return ".gif", nil
	case "image/webp":
		return ".webp", nil
	case "image/x-icon", "image/vnd.microsoft.icon":
		return ".ico", nil
	}
	if isSVG(data) {
		return ".svg", nil
	}
	return "", invalid("不支持的图片格式，请使用 PNG、JPEG、GIF、WebP、ICO 或 SVG")
}

func isSVG(data []byte) bool {
	head := data
	if len(head) > 1024 {
		head = head[:1024]
	}
	return strings.HasPrefix(http.DetectContentType(data), "text/") &&
		bytes.Contains(bytes.ToLower(head), []byte("<svg"))
}
