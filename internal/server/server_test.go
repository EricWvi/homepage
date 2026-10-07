package server

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"homepage/internal/auth"
	"homepage/internal/store"
)

var pngHeader = []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR")

const origin = "https://home.test"

// testServer is the handler plus a store for minting sessions.
type testServer struct {
	http.Handler
	store *store.Store
	// session is sent as the cookie on every request made with do.
	session string
}

func newTest(t *testing.T) *testServer {
	t.Helper()
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	// The provider is discovered lazily, so these tests never reach it.
	authn := auth.New(st, auth.Config{PublicURL: origin, Issuer: "https://idp.invalid", ClientID: "x", ClientSecret: "y"})
	web := fstest.MapFS{
		"index.html":        {Data: []byte("<!doctype html><title>home</title>")},
		"assets/app-abc.js": {Data: []byte("console.log(1)")},
		"sw.js":             {Data: []byte("self.addEventListener('fetch',()=>{})")},
		".gitkeep":          {Data: nil},
	}
	h, err := New(st, authn, web, Options{Version: "test", IdleWait: 90 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	ts := &testServer{Handler: h, store: st}
	ts.session = ts.signIn(t, "alice")
	return ts
}

// signIn creates a user and returns a session token for it.
func (ts *testServer) signIn(t *testing.T, subject string) string {
	t.Helper()
	ctx := context.Background()
	u, err := ts.store.UpsertUser(ctx, store.Identity{Issuer: "https://idp.test", Subject: subject, Name: subject})
	if err != nil {
		t.Fatal(err)
	}
	token, err := ts.store.CreateSession(ctx, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	return token
}

func do(t *testing.T, ts *testServer, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if ts.session != "" {
		req.AddCookie(&http.Cookie{Name: "homepage_session", Value: ts.session})
	}
	rec := httptest.NewRecorder()
	ts.ServeHTTP(rec, req)
	return rec
}

type snapshotBody struct {
	store.Snapshot
	User            store.User `json:"user"`
	IdleWaitSeconds int        `json:"idleWaitSeconds"`
}

func decodeSnapshot(t *testing.T, rec *httptest.ResponseRecorder) snapshotBody {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	var snap snapshotBody
	if err := json.Unmarshal(rec.Body.Bytes(), &snap); err != nil {
		t.Fatal(err)
	}
	return snap
}

func errorOf(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct{ Error string }
	json.Unmarshal(rec.Body.Bytes(), &body)
	return body.Error
}

func TestSiteLifecycle(t *testing.T) {
	h := newTest(t)
	snap := decodeSnapshot(t, do(t, h, "POST", "/api/sites", `{"title":"Go","url":"go.dev"}`))
	if len(snap.Sites) != 1 || snap.Sites[0].Domain != "go.dev" || len(snap.Domains) != 1 {
		t.Fatalf("snapshot = %+v", snap)
	}
	if snap.User != (store.User{Name: "alice"}) {
		t.Fatalf("user = %+v", snap.User)
	}
	id := snap.Sites[0].ID

	snap = decodeSnapshot(t, do(t, h, "PUT", "/api/sites/"+itoa(id),
		`{"title":"Golang","url":"https://go.dev","links":[{"title":"","url":"doc"}]}`))
	if snap.Sites[0].Title != "Golang" || len(snap.Sites[0].Links) != 1 ||
		snap.Sites[0].Links[0] != (store.SiteLink{Title: "doc", URL: "https://go.dev/doc"}) {
		t.Fatalf("sites = %+v", snap.Sites)
	}

	rec := do(t, h, "PUT", "/api/sites/"+itoa(id), `{"title":"Golang","url":"https://go.dev","links":[{"url":"https://example.com"}]}`)
	if rec.Code != http.StatusBadRequest || !strings.Contains(errorOf(t, rec), "同域名") {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}

	snap = decodeSnapshot(t, do(t, h, "DELETE", "/api/sites/"+itoa(id), ""))
	if len(snap.Sites) != 0 || len(snap.Domains) != 1 {
		t.Fatalf("snapshot = %+v", snap)
	}
}

func TestValidationErrorsAreReadable(t *testing.T) {
	h := newTest(t)
	rec := do(t, h, "POST", "/api/sites", `{"title":"","url":"go.dev"}`)
	if rec.Code != http.StatusBadRequest || errorOf(t, rec) != "标题不能为空" {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	rec = do(t, h, "DELETE", "/api/sites/42", "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d", rec.Code)
	}
	rec = do(t, h, "GET", "/api/nope", "")
	if rec.Code != http.StatusNotFound || !strings.HasPrefix(rec.Header().Get("Content-Type"), "application/json") {
		t.Fatalf("status = %d, type = %s", rec.Code, rec.Header().Get("Content-Type"))
	}
}

func TestMoveSite(t *testing.T) {
	h := newTest(t)
	decodeSnapshot(t, do(t, h, "POST", "/api/sites", `{"title":"a","url":"a.com"}`))
	snap := decodeSnapshot(t, do(t, h, "POST", "/api/sites", `{"title":"b","url":"b.com"}`))
	b := snap.Sites[1].ID
	snap = decodeSnapshot(t, do(t, h, "PUT", "/api/sites/"+itoa(b)+"/move", `{"after":0}`))
	if snap.Sites[0].Title != "b" || snap.Sites[0].SortKey >= snap.Sites[1].SortKey {
		t.Fatalf("sites = %+v", snap.Sites)
	}
	if rec := do(t, h, "PUT", "/api/sites/"+itoa(b)+"/move", `{"after":`+itoa(b)+`}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("moving after itself: status = %d", rec.Code)
	}
}

func TestGroups(t *testing.T) {
	h := newTest(t)
	decodeSnapshot(t, do(t, h, "POST", "/api/groups", `{"name":"a"}`))
	snap := decodeSnapshot(t, do(t, h, "POST", "/api/groups", `{"name":"b"}`))
	a, b := snap.Groups[1].ID, snap.Groups[2].ID

	snap = decodeSnapshot(t, do(t, h, "PUT", "/api/groups/order", `{"ids":[`+itoa(b)+`,`+itoa(a)+`]}`))
	if snap.Groups[1].ID != b {
		t.Fatalf("groups = %+v", snap.Groups)
	}
	snap = decodeSnapshot(t, do(t, h, "PUT", "/api/groups/"+itoa(a), `{"name":"renamed"}`))
	if snap.Groups[2].Name != "renamed" {
		t.Fatalf("groups = %+v", snap.Groups)
	}
	snap = decodeSnapshot(t, do(t, h, "PUT", "/api/groups/"+itoa(a)+"/work", `{"hidden":true}`))
	if !snap.Groups[2].HiddenAtWork || snap.Groups[1].HiddenAtWork {
		t.Fatalf("groups = %+v", snap.Groups)
	}
	snap = decodeSnapshot(t, do(t, h, "DELETE", "/api/groups/"+itoa(a), ""))
	if len(snap.Groups) != 2 {
		t.Fatalf("groups = %+v", snap.Groups)
	}
	if rec := do(t, h, "DELETE", "/api/groups/"+itoa(snap.Groups[0].ID), ""); rec.Code != http.StatusBadRequest {
		t.Fatalf("deleting default group: status = %d", rec.Code)
	}
}

func TestDomainIconUploadAndServe(t *testing.T) {
	h := newTest(t)
	decodeSnapshot(t, do(t, h, "POST", "/api/sites", `{"title":"Go","url":"go.dev"}`))

	snap := decodeSnapshot(t, do(t, h, "PUT", "/api/domains/go.dev/icon", string(pngHeader)))
	icon := snap.Domains[0].Icon
	if icon == nil {
		t.Fatal("icon not set")
	}

	rec := do(t, h, "GET", "/icons/"+*icon, "")
	if rec.Code != http.StatusOK || !bytes.Equal(rec.Body.Bytes(), pngHeader) {
		t.Fatalf("status = %d", rec.Code)
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "private, max-age=31536000, immutable" {
		t.Fatalf("Cache-Control = %q", cc)
	}

	if rec := do(t, h, "GET", "/icons/../homepage.db", ""); rec.Code == http.StatusOK {
		t.Fatal("path traversal served a file")
	}

	big := strings.Repeat("x", maxIconBytes+1)
	if rec := do(t, h, "PUT", "/api/domains/go.dev/icon", big); rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized upload: status = %d", rec.Code)
	}

	if rec := do(t, h, "DELETE", "/api/domains/go.dev", ""); rec.Code != http.StatusConflict {
		t.Fatalf("deleting used domain: status = %d", rec.Code)
	}
	snap = decodeSnapshot(t, do(t, h, "DELETE", "/api/domains/go.dev/icon", ""))
	if snap.Domains[0].Icon != nil {
		t.Fatal("icon not cleared")
	}
}

func TestStaticCaching(t *testing.T) {
	h := newTest(t)
	h.session = "" // the app shell must load without signing in
	for _, tc := range []struct {
		path, cache string
		code        int
	}{
		{"/", "no-cache", 200},
		{"/sw.js", "no-cache", 200},
		{"/assets/app-abc.js", "public, max-age=31536000, immutable", 200},
		{"/some/page", "no-cache", 200},
		{"/assets/missing.js", "", 404},
		{"/.gitkeep", "", 404},
	} {
		rec := do(t, h, "GET", tc.path, "")
		if rec.Code != tc.code || rec.Header().Get("Cache-Control") != tc.cache {
			t.Errorf("%s: status = %d, Cache-Control = %q", tc.path, rec.Code, rec.Header().Get("Cache-Control"))
		}
	}

	rec := do(t, h, "GET", "/", "")
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("If-None-Match", rec.Header().Get("ETag"))
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotModified {
		t.Fatalf("revalidation: status = %d", rec.Code)
	}
}

func itoa(n int64) string {
	b, _ := json.Marshal(n)
	return string(b)
}

func TestAPIAndIconsRequireSession(t *testing.T) {
	h := newTest(t)
	snap := decodeSnapshot(t, do(t, h, "POST", "/api/sites", `{"title":"Go","url":"go.dev"}`))
	snap = decodeSnapshot(t, do(t, h, "PUT", "/api/domains/go.dev/icon", string(pngHeader)))
	icon := "/icons/" + *snap.Domains[0].Icon

	for _, session := range []string{"", "forged-token"} {
		h.session = session
		for _, path := range []string{"/api/snapshot", "/api/nope", icon} {
			if rec := do(t, h, "GET", path, ""); rec.Code != http.StatusUnauthorized {
				t.Errorf("session %q, GET %s: status = %d", session, path, rec.Code)
			}
		}
	}
	h.session = ""
	if rec := do(t, h, "GET", "/api/version", ""); rec.Code != http.StatusOK {
		t.Errorf("version: status = %d", rec.Code)
	}
}

func TestUsersHaveSeparateData(t *testing.T) {
	h := newTest(t)
	decodeSnapshot(t, do(t, h, "POST", "/api/sites", `{"title":"Go","url":"go.dev"}`))
	alice := h.session

	h.session = h.signIn(t, "bob")
	snap := decodeSnapshot(t, do(t, h, "GET", "/api/snapshot", ""))
	if len(snap.Sites) != 0 || len(snap.Domains) != 0 || len(snap.Groups) != 1 || snap.User.Name != "bob" {
		t.Fatalf("bob sees %+v", snap)
	}

	h.session = alice
	if snap := decodeSnapshot(t, do(t, h, "GET", "/api/snapshot", "")); len(snap.Sites) != 1 {
		t.Fatalf("alice sees %+v", snap)
	}
}

func TestCrossOriginWritesAreRejected(t *testing.T) {
	h := newTest(t)
	for _, tc := range []struct {
		origin string
		code   int
	}{
		{"https://evil.test", http.StatusForbidden},
		{"https://other.home.test", http.StatusForbidden},
		{origin, http.StatusOK},
		{"", http.StatusOK},
	} {
		req := httptest.NewRequest("POST", "/api/groups", strings.NewReader(`{"name":"g"}`))
		req.AddCookie(&http.Cookie{Name: "homepage_session", Value: h.session})
		if tc.origin != "" {
			req.Header.Set("Origin", tc.origin)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != tc.code {
			t.Errorf("Origin %q: status = %d, want %d", tc.origin, rec.Code, tc.code)
		}
	}
}

func TestLogoutEndsOnlyThisSession(t *testing.T) {
	h := newTest(t)
	other := h.signIn(t, "alice") // same user, another browser

	rec := do(t, h, "POST", "/auth/logout", "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("logout: status = %d", rec.Code)
	}
	if c := rec.Result().Cookies(); len(c) != 1 || c[0].Name != "homepage_session" || c[0].MaxAge >= 0 {
		t.Fatalf("logout cookies = %+v", c)
	}
	if rec := do(t, h, "GET", "/api/snapshot", ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("after logout: status = %d", rec.Code)
	}
	h.session = other
	if rec := do(t, h, "GET", "/api/snapshot", ""); rec.Code != http.StatusOK {
		t.Fatalf("other browser: status = %d", rec.Code)
	}
}

func TestWallpapers(t *testing.T) {
	h := newTest(t)
	var img bytes.Buffer
	png.Encode(&img, image.NewRGBA(image.Rect(0, 0, 64, 40)))

	snap := decodeSnapshot(t, do(t, h, "POST", "/api/wallpapers", img.String()))
	if len(snap.Wallpapers) != 1 || snap.CurrentWallpaper != nil || snap.IdleWaitSeconds != 90 {
		t.Fatalf("snapshot = %+v", snap)
	}
	wp := snap.Wallpapers[0]

	for _, name := range []string{wp.File, wp.Thumb} {
		rec := do(t, h, "GET", "/wallpapers/"+name, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: status = %d", name, rec.Code)
		}
		if cc := rec.Header().Get("Cache-Control"); cc != "private, max-age=31536000, immutable" {
			t.Fatalf("%s: Cache-Control = %q", name, cc)
		}
	}

	snap = decodeSnapshot(t, do(t, h, "PUT", "/api/wallpapers/current", `{"id":`+itoa(wp.ID)+`}`))
	if snap.CurrentWallpaper == nil || *snap.CurrentWallpaper != wp.ID {
		t.Fatalf("current = %v", snap.CurrentWallpaper)
	}

	bob := *h
	bob.session = h.signIn(t, "bob")
	if rec := do(t, &bob, "PUT", "/api/wallpapers/current", `{"id":`+itoa(wp.ID)+`}`); rec.Code != http.StatusNotFound {
		t.Fatalf("bob using alice's wallpaper: status = %d", rec.Code)
	}

	if rec := do(t, h, "POST", "/api/wallpapers", "not an image"); rec.Code != http.StatusBadRequest {
		t.Fatalf("bad upload: status = %d", rec.Code)
	}
	if rec := do(t, h, "POST", "/api/wallpapers", strings.Repeat("x", maxWallpaperBytes+1)); rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized upload: status = %d", rec.Code)
	}

	snap = decodeSnapshot(t, do(t, h, "DELETE", "/api/wallpapers/"+itoa(wp.ID), ""))
	if len(snap.Wallpapers) != 0 || snap.CurrentWallpaper != nil {
		t.Fatalf("after delete: %+v", snap)
	}
	if rec := do(t, h, "GET", "/wallpapers/"+wp.File, ""); rec.Code != http.StatusNotFound {
		t.Fatalf("deleted file: status = %d", rec.Code)
	}
}
