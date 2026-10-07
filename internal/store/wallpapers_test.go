package store

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"testing"
)

// testImage encodes a w×h image whose colour depends on seed, so different
// seeds give different files.
func testImage(t *testing.T, w, h int, seed uint8, encode func(*bytes.Buffer, image.Image) error) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{seed, uint8(x), uint8(y), 255})
		}
	}
	var buf bytes.Buffer
	if err := encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func pngImage(t *testing.T, w, h int, seed uint8) []byte {
	return testImage(t, w, h, seed, func(b *bytes.Buffer, m image.Image) error { return png.Encode(b, m) })
}

func TestAddWallpaperStoresImageAndThumbnail(t *testing.T) {
	s := openTest(t)
	uid := newUser(t, s, "alice")
	ctx := context.Background()
	data := testImage(t, 1200, 600, 1, func(b *bytes.Buffer, m image.Image) error { return jpeg.Encode(b, m, nil) })

	w, err := s.AddWallpaper(ctx, uid, data)
	if err != nil {
		t.Fatal(err)
	}
	if !wallpaperName.MatchString(w.File) || w.Width != 1200 || w.Height != 600 {
		t.Fatalf("wallpaper = %+v", w)
	}
	stored, _ := os.ReadFile(mustWallpaperPath(t, s, w.File))
	if !bytes.Equal(stored, data) {
		t.Fatal("stored file differs from upload")
	}
	thumb, _ := os.ReadFile(mustWallpaperPath(t, s, w.Thumb))
	cfg, format, err := image.DecodeConfig(bytes.NewReader(thumb))
	if err != nil || format != "jpeg" || cfg.Width != thumbWidth || cfg.Height != thumbWidth/2 {
		t.Fatalf("thumb = %s %dx%d, err = %v", format, cfg.Width, cfg.Height, err)
	}

	again, err := s.AddWallpaper(ctx, uid, data)
	if err != nil || again.ID != w.ID {
		t.Fatalf("re-adding: %+v, %v; want id %d", again, err, w.ID)
	}
	if n := len(snapshot(t, s, uid).Wallpapers); n != 1 {
		t.Fatalf("wallpapers = %d, want 1", n)
	}
}

func TestAddWallpaperRejectsBadInput(t *testing.T) {
	s := openTest(t)
	uid := newUser(t, s, "alice")
	for name, data := range map[string][]byte{
		"empty":     nil,
		"text":      []byte("hello"),
		"gif":       []byte("GIF89a\x01\x00\x01\x00\x00\x00\x00;"),
		"truncated": pngImage(t, 40, 40, 1)[:30],
	} {
		if _, err := s.AddWallpaper(context.Background(), uid, data); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: err = %v, want ErrInvalid", name, err)
		}
	}
	entries, _ := os.ReadDir(s.wallpaperDir)
	if len(entries) != 0 {
		t.Fatalf("rejected uploads left %d files", len(entries))
	}
}

func TestSnapshotListsWallpapersNewestFirst(t *testing.T) {
	s := openTest(t)
	uid := newUser(t, s, "alice")
	ctx := context.Background()
	first, _ := s.AddWallpaper(ctx, uid, pngImage(t, 40, 30, 1))
	second, _ := s.AddWallpaper(ctx, uid, pngImage(t, 40, 30, 2))
	snap := snapshot(t, s, uid)
	if len(snap.Wallpapers) != 2 || snap.Wallpapers[0].ID != second.ID || snap.Wallpapers[1].ID != first.ID {
		t.Fatalf("wallpapers = %+v", snap.Wallpapers)
	}
	if snap.CurrentWallpaper != nil {
		t.Fatalf("current = %v, want nil", *snap.CurrentWallpaper)
	}
}

func TestCurrentWallpaperIsOwnedAndClearedOnDelete(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	alice, bob := newUser(t, s, "alice"), newUser(t, s, "bob")
	w, _ := s.AddWallpaper(ctx, alice, pngImage(t, 40, 30, 1))

	if err := s.SetCurrentWallpaper(ctx, bob, w.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("bob setting alice's wallpaper: err = %v, want ErrNotFound", err)
	}
	if err := s.SetCurrentWallpaper(ctx, alice, w.ID); err != nil {
		t.Fatal(err)
	}
	if cur := snapshot(t, s, alice).CurrentWallpaper; cur == nil || *cur != w.ID {
		t.Fatalf("current = %v, want %d", cur, w.ID)
	}
	if err := s.DeleteWallpaper(ctx, bob, w.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("bob deleting alice's wallpaper: err = %v, want ErrNotFound", err)
	}
	if err := s.DeleteWallpaper(ctx, alice, w.ID); err != nil {
		t.Fatal(err)
	}
	snap := snapshot(t, s, alice)
	if len(snap.Wallpapers) != 0 || snap.CurrentWallpaper != nil {
		t.Fatalf("after delete: %+v, current %v", snap.Wallpapers, snap.CurrentWallpaper)
	}
}

func TestSharedWallpaperFilesSurviveWhileReferenced(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	alice, bob := newUser(t, s, "alice"), newUser(t, s, "bob")
	data := pngImage(t, 40, 30, 1)
	a, _ := s.AddWallpaper(ctx, alice, data)
	b, _ := s.AddWallpaper(ctx, bob, data)
	if a.File != b.File || a.ID == b.ID {
		t.Fatalf("alice %+v, bob %+v", a, b)
	}

	s.DeleteWallpaper(ctx, alice, a.ID)
	for _, name := range []string{b.File, b.Thumb} {
		if _, err := os.Stat(mustWallpaperPath(t, s, name)); err != nil {
			t.Fatalf("%s removed while bob still uses it", name)
		}
	}
	s.DeleteWallpaper(ctx, bob, b.ID)
	for _, name := range []string{b.File, b.Thumb} {
		if _, err := os.Stat(mustWallpaperPath(t, s, name)); !os.IsNotExist(err) {
			t.Fatalf("%s left behind: %v", name, err)
		}
	}
}

func TestWallpaperPathRejectsForeignNames(t *testing.T) {
	s := openTest(t)
	for _, name := range []string{"../homepage.db", "0123456789abcdef.gif", "x.jpg", "0123456789abcdef-thumb.png"} {
		if _, err := s.WallpaperPath(name); !errors.Is(err, ErrNotFound) {
			t.Errorf("%s: err = %v, want ErrNotFound", name, err)
		}
	}
}

func mustWallpaperPath(t *testing.T, s *Store, name string) string {
	t.Helper()
	path, err := s.WallpaperPath(name)
	if err != nil {
		t.Fatal(err)
	}
	return path
}
