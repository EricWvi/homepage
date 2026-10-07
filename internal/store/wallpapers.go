package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"image"
	"image/jpeg"
	_ "image/png" // register decoders for DecodeConfig and Decode
	"net/http"
	"os"
	"path/filepath"
	"regexp"

	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

const (
	// maxWallpaperPixels guards against decompression bombs; 100 MP is well
	// beyond any display.
	maxWallpaperPixels = 100_000_000
	// thumbWidth is wide enough for a sharp library tile on a 2x display.
	thumbWidth = 480
)

// wallpaperName matches the files AddWallpaper produces: the image and its
// JPEG thumbnail.
var wallpaperName = regexp.MustCompile(`^[0-9a-f]{16}(\.(jpg|png|webp)|-thumb\.jpg)$`)

// AddWallpaper stores an image in the user's library together with a
// thumbnail. Adding an image the user already has returns the existing one.
func (s *Store) AddWallpaper(ctx context.Context, userID int64, data []byte) (Wallpaper, error) {
	ext, err := wallpaperExt(data)
	if err != nil {
		return Wallpaper{}, err
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return Wallpaper{}, invalid("无法读取图片")
	}
	if cfg.Width*cfg.Height > maxWallpaperPixels {
		return Wallpaper{}, invalid("图片尺寸过大")
	}
	sum := sha256.Sum256(data)
	hash := hex.EncodeToString(sum[:8])
	w := Wallpaper{File: hash + ext, Thumb: hash + "-thumb.jpg", Width: cfg.Width, Height: cfg.Height}

	if err := s.writeWallpaperFile(w.File, func() ([]byte, error) { return data, nil }); err != nil {
		return Wallpaper{}, err
	}
	if err := s.writeWallpaperFile(w.Thumb, func() ([]byte, error) { return thumbnail(data) }); err != nil {
		s.removeWallpaperIfUnused(context.WithoutCancel(ctx), w)
		return Wallpaper{}, err
	}

	err = s.inTx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx,
			`INSERT OR IGNORE INTO wallpapers (user_id, file, thumb, width, height) VALUES (?, ?, ?, ?, ?)`,
			userID, w.File, w.Thumb, w.Width, w.Height); err != nil {
			return err
		}
		return tx.QueryRowContext(ctx,
			`SELECT id FROM wallpapers WHERE user_id = ? AND file = ?`, userID, w.File).Scan(&w.ID)
	})
	if err != nil {
		s.removeWallpaperIfUnused(context.WithoutCancel(ctx), w)
		return Wallpaper{}, err
	}
	return w, nil
}

// DeleteWallpaper removes a wallpaper from the user's library. If it was
// the current wallpaper, the user has none until another one is shown.
func (s *Store) DeleteWallpaper(ctx context.Context, userID, id int64) error {
	var w Wallpaper
	err := s.db.QueryRowContext(ctx,
		`SELECT file, thumb FROM wallpapers WHERE id = ? AND user_id = ?`, id, userID).Scan(&w.File, &w.Thumb)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM wallpapers WHERE id = ?`, id); err != nil {
		return err
	}
	s.removeWallpaperIfUnused(ctx, w)
	return nil
}

// SetCurrentWallpaper remembers which of the user's wallpapers is shown.
func (s *Store) SetCurrentWallpaper(ctx context.Context, userID, id int64) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE users SET wallpaper_id = ? WHERE id = ?
		 AND EXISTS (SELECT 1 FROM wallpapers WHERE id = ? AND user_id = ?)`, id, userID, id, userID)
	if err != nil {
		return err
	}
	return requireAffected(res)
}

// WallpaperPath returns the file path for a wallpaper or thumbnail name, or
// ErrNotFound for names AddWallpaper could not have produced.
func (s *Store) WallpaperPath(name string) (string, error) {
	if !wallpaperName.MatchString(name) {
		return "", ErrNotFound
	}
	return filepath.Join(s.wallpaperDir, name), nil
}

func (s *Store) wallpapers(ctx context.Context, userID int64) ([]Wallpaper, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, file, thumb, width, height FROM wallpapers WHERE user_id = ? ORDER BY id DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []Wallpaper{}
	for rows.Next() {
		var w Wallpaper
		if err := rows.Scan(&w.ID, &w.File, &w.Thumb, &w.Width, &w.Height); err != nil {
			return nil, err
		}
		list = append(list, w)
	}
	return list, rows.Err()
}

// writeWallpaperFile writes name unless it already exists; identical
// uploads share one file, so build only runs for new content.
func (s *Store) writeWallpaperFile(name string, build func() ([]byte, error)) error {
	path := filepath.Join(s.wallpaperDir, name)
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	data, err := build()
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(s.wallpaperDir, ".upload-*")
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

// removeWallpaperIfUnused deletes a wallpaper's files once no user's
// library contains them.
func (s *Store) removeWallpaperIfUnused(ctx context.Context, w Wallpaper) {
	var used int
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM wallpapers WHERE file = ?`, w.File).Scan(&used); err != nil || used > 0 {
		return
	}
	for _, name := range []string{w.File, w.Thumb} {
		if path, err := s.WallpaperPath(name); err == nil {
			os.Remove(path)
		}
	}
}

// thumbnail scales the image down to thumbWidth and encodes it as JPEG.
func thumbnail(data []byte) ([]byte, error) {
	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, invalid("无法读取图片")
	}
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	if w > thumbWidth {
		w, h = thumbWidth, max(1, h*thumbWidth/w)
	}
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, b, draw.Src, nil)
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: 82}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// wallpaperExt sniffs the image format; only formats the server can make a
// thumbnail of are accepted.
func wallpaperExt(data []byte) (string, error) {
	if len(data) == 0 {
		return "", invalid("壁纸文件为空")
	}
	switch http.DetectContentType(data) {
	case "image/jpeg":
		return ".jpg", nil
	case "image/png":
		return ".png", nil
	case "image/webp":
		return ".webp", nil
	}
	return "", invalid("不支持的图片格式，请使用 JPEG、PNG 或 WebP")
}
