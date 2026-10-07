package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"homepage/internal/store"
)

var pngHeader = []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR")

func newTest(t *testing.T) http.Handler {
	t.Helper()
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	web := fstest.MapFS{
		"index.html":        {Data: []byte("<!doctype html><title>home</title>")},
		"assets/app-abc.js": {Data: []byte("console.log(1)")},
		"sw.js":             {Data: []byte("self.addEventListener('fetch',()=>{})")},
		".gitkeep":          {Data: nil},
	}
	h, err := New(st, web, "test")
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func do(t *testing.T, h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func decodeSnapshot(t *testing.T, rec *httptest.ResponseRecorder) store.Snapshot {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	var snap store.Snapshot
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
	id := snap.Sites[0].ID

	snap = decodeSnapshot(t, do(t, h, "PUT", "/api/sites/"+itoa(id), `{"title":"Golang","url":"https://go.dev","groupId":1}`))
	if snap.Sites[0].Title != "Golang" {
		t.Fatalf("sites = %+v", snap.Sites)
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
	snap = decodeSnapshot(t, do(t, h, "DELETE", "/api/groups/"+itoa(a), ""))
	if len(snap.Groups) != 2 {
		t.Fatalf("groups = %+v", snap.Groups)
	}
	if rec := do(t, h, "DELETE", "/api/groups/1", ""); rec.Code != http.StatusBadRequest {
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
	if cc := rec.Header().Get("Cache-Control"); cc != "public, max-age=31536000, immutable" {
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
