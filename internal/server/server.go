// Package server exposes the store over a JSON API and serves the
// embedded frontend.
package server

import (
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"homepage/internal/auth"
	"homepage/internal/store"
)

const (
	// maxIconBytes caps uploaded icon size.
	maxIconBytes = 2 << 20
	// maxWallpaperBytes caps uploaded wallpaper size; 8K photos fit.
	maxWallpaperBytes = 40 << 20
)

// Options are the settings the server passes on to the frontend.
type Options struct {
	Version string
	// IdleWait is how long the page waits for input before showing wallpapers.
	IdleWait time.Duration
}

type server struct {
	store *store.Store
	opts  Options
}

// New returns the HTTP handler for auth, the API, icons, wallpapers and the
// frontend in web. The API and images require a signed-in user; the app
// shell does not, so it can load offline and send the user to the login page.
func New(st *store.Store, authn *auth.Service, web fs.FS, opts Options) (http.Handler, error) {
	s := &server{store: st, opts: opts}
	static, err := newStatic(web)
	if err != nil {
		return nil, err
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/version", s.getVersion)
	authn.Register(mux)

	// Everything registered on private is only reachable with a session.
	private := http.NewServeMux()
	mux.Handle("/api/", authn.Require(private))
	mux.Handle("/icons/", authn.Require(private))
	mux.Handle("/wallpapers/", authn.Require(private))
	mux.Handle("/", static)

	private.HandleFunc("GET /api/snapshot", s.getSnapshot)

	private.HandleFunc("POST /api/sites", s.createSite)
	private.HandleFunc("PUT /api/sites/{id}", s.updateSite)
	private.HandleFunc("DELETE /api/sites/{id}", s.deleteSite)

	private.HandleFunc("POST /api/groups", s.createGroup)
	private.HandleFunc("PUT /api/groups/order", s.reorderGroups)
	private.HandleFunc("PUT /api/groups/{id}", s.renameGroup)
	private.HandleFunc("PUT /api/groups/{id}/work", s.setGroupHiddenAtWork)
	private.HandleFunc("DELETE /api/groups/{id}", s.deleteGroup)

	private.HandleFunc("PUT /api/domains/{domain}/icon", s.setDomainIcon)
	private.HandleFunc("DELETE /api/domains/{domain}/icon", s.clearDomainIcon)
	private.HandleFunc("DELETE /api/domains/{domain}", s.deleteDomain)

	private.HandleFunc("POST /api/wallpapers", s.addWallpaper)
	private.HandleFunc("PUT /api/wallpapers/current", s.setCurrentWallpaper)
	private.HandleFunc("DELETE /api/wallpapers/{id}", s.deleteWallpaper)

	private.HandleFunc("GET /icons/{name}", s.getIcon)
	private.HandleFunc("GET /wallpapers/{name}", s.getWallpaper)
	private.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "接口不存在")
	})
	return mux, nil
}

func userID(r *http.Request) int64 { return auth.User(r.Context()).ID }

func (s *server) getVersion(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"version": s.opts.Version})
}

func (s *server) getSnapshot(w http.ResponseWriter, r *http.Request) {
	s.respondSnapshot(w, r, nil)
}

func (s *server) createSite(w http.ResponseWriter, r *http.Request) {
	var in store.SiteInput
	if !decode(w, r, &in) {
		return
	}
	_, err := s.store.CreateSite(r.Context(), userID(r), in)
	s.respondSnapshot(w, r, err)
}

func (s *server) updateSite(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var in store.SiteInput
	if !decode(w, r, &in) {
		return
	}
	s.respondSnapshot(w, r, s.store.UpdateSite(r.Context(), userID(r), id, in))
}

func (s *server) deleteSite(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	s.respondSnapshot(w, r, s.store.DeleteSite(r.Context(), userID(r), id))
}

type groupInput struct {
	Name string `json:"name"`
}

func (s *server) createGroup(w http.ResponseWriter, r *http.Request) {
	var in groupInput
	if !decode(w, r, &in) {
		return
	}
	_, err := s.store.CreateGroup(r.Context(), userID(r), in.Name)
	s.respondSnapshot(w, r, err)
}

func (s *server) renameGroup(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var in groupInput
	if !decode(w, r, &in) {
		return
	}
	s.respondSnapshot(w, r, s.store.RenameGroup(r.Context(), userID(r), id, in.Name))
}

func (s *server) setGroupHiddenAtWork(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var in struct {
		Hidden bool `json:"hidden"`
	}
	if !decode(w, r, &in) {
		return
	}
	s.respondSnapshot(w, r, s.store.SetGroupHiddenAtWork(r.Context(), userID(r), id, in.Hidden))
}

func (s *server) deleteGroup(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	s.respondSnapshot(w, r, s.store.DeleteGroup(r.Context(), userID(r), id))
}

func (s *server) reorderGroups(w http.ResponseWriter, r *http.Request) {
	var in struct {
		IDs []int64 `json:"ids"`
	}
	if !decode(w, r, &in) {
		return
	}
	s.respondSnapshot(w, r, s.store.ReorderGroups(r.Context(), userID(r), in.IDs))
}

// setDomainIcon takes the raw image bytes as the request body.
func (s *server) setDomainIcon(w http.ResponseWriter, r *http.Request) {
	data, ok := readUpload(w, r, maxIconBytes, "图标不能超过 2 MB")
	if !ok {
		return
	}
	_, err := s.store.SetDomainIcon(r.Context(), userID(r), r.PathValue("domain"), data)
	s.respondSnapshot(w, r, err)
}

func (s *server) clearDomainIcon(w http.ResponseWriter, r *http.Request) {
	s.respondSnapshot(w, r, s.store.ClearDomainIcon(r.Context(), userID(r), r.PathValue("domain")))
}

func (s *server) deleteDomain(w http.ResponseWriter, r *http.Request) {
	s.respondSnapshot(w, r, s.store.DeleteDomain(r.Context(), userID(r), r.PathValue("domain")))
}

// addWallpaper takes the raw image bytes as the request body.
func (s *server) addWallpaper(w http.ResponseWriter, r *http.Request) {
	data, ok := readUpload(w, r, maxWallpaperBytes, "壁纸不能超过 40 MB")
	if !ok {
		return
	}
	_, err := s.store.AddWallpaper(r.Context(), userID(r), data)
	s.respondSnapshot(w, r, err)
}

func (s *server) deleteWallpaper(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	s.respondSnapshot(w, r, s.store.DeleteWallpaper(r.Context(), userID(r), id))
}

func (s *server) setCurrentWallpaper(w http.ResponseWriter, r *http.Request) {
	var in struct {
		ID int64 `json:"id"`
	}
	if !decode(w, r, &in) {
		return
	}
	s.respondSnapshot(w, r, s.store.SetCurrentWallpaper(r.Context(), userID(r), in.ID))
}

func (s *server) getIcon(w http.ResponseWriter, r *http.Request) {
	serveImmutable(w, r, s.store.IconPath)
}

func (s *server) getWallpaper(w http.ResponseWriter, r *http.Request) {
	serveImmutable(w, r, s.store.WallpaperPath)
}

// serveImmutable serves an uploaded image. Names are content hashes, so the
// response never changes and may be cached for a year. It is private
// because it requires a session.
func serveImmutable(w http.ResponseWriter, r *http.Request, pathOf func(string) (string, error)) {
	path, err := pathOf(r.PathValue("name"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	h := w.Header()
	h.Set("Cache-Control", "private, max-age=31536000, immutable")
	h.Set("X-Content-Type-Options", "nosniff")
	// Uploaded SVGs may contain scripts; never let them run.
	h.Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; sandbox")
	http.ServeFile(w, r, path)
}

// readUpload reads a raw upload body of at most limit bytes.
func readUpload(w http.ResponseWriter, r *http.Request, limit int64, tooBigMsg string) ([]byte, bool) {
	data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, limit))
	if err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			writeError(w, http.StatusRequestEntityTooLarge, tooBigMsg)
			return nil, false
		}
		writeError(w, http.StatusBadRequest, "读取上传内容失败")
		return nil, false
	}
	return data, true
}

// snapshotResponse is the user's state plus who they are and the
// server-wide settings the page needs offline.
type snapshotResponse struct {
	store.Snapshot
	User            store.User `json:"user"`
	IdleWaitSeconds int        `json:"idleWaitSeconds"`
}

// respondSnapshot reports err, or on success returns the full current
// state so the client can replace its copy in one step.
func (s *server) respondSnapshot(w http.ResponseWriter, r *http.Request, err error) {
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	user := auth.User(r.Context())
	snap, err := s.store.Snapshot(r.Context(), user.ID)
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, snapshotResponse{
		Snapshot:        snap,
		User:            user,
		IdleWaitSeconds: int(s.opts.IdleWait / time.Second),
	})
}

func writeStoreError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, store.ErrInvalid):
		writeError(w, http.StatusBadRequest, detail(err, store.ErrInvalid))
	case errors.Is(err, store.ErrConflict):
		writeError(w, http.StatusConflict, detail(err, store.ErrConflict))
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "内容不存在或已被删除")
	default:
		slog.Error("request failed", "method", r.Method, "path", r.URL.Path, "err", err)
		writeError(w, http.StatusInternalServerError, "服务器内部错误")
	}
}

// detail strips the sentinel prefix from a wrapped store error.
func detail(err, sentinel error) string {
	return strings.TrimPrefix(err.Error(), sentinel.Error()+": ")
}

func pathID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "无效的 id")
		return 0, false
	}
	return id, true
}

func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 256<<10)).Decode(v); err != nil {
		writeError(w, http.StatusBadRequest, "请求格式不正确")
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
