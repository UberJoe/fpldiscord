// Package web serves the embedded phone-first SPA and the /api/* JSON surface
// that feeds it. Ticket 01 establishes the http.Server wiring, the /healthz
// endpoint, the SPA file server with index.html fallback, and the shared
// logging + panic-recover wrapper. Ticket 05 adds the shared {meta, data}
// envelope and GET /api/standings.
package web

import (
	"encoding/json"
	"io/fs"
	"log/slog"
	"net/http"
	"time"

	"github.com/UberJoe/fpldiscord/internal/fpl"
)

// SnapshotProvider is the read side of fpl.Store that web needs: the current
// immutable snapshot, or nil before the first successful build. Kept as a
// consumer-side interface so the handlers can be driven by a hand-built
// *fpl.Snapshot in tests.
type SnapshotProvider interface {
	Current() *fpl.Snapshot
}

// Server bundles the HTTP handler for the bot's web surface.
type Server struct {
	log  *slog.Logger
	snap SnapshotProvider
}

// New builds a Server.
func New(log *slog.Logger, snap SnapshotProvider) *Server {
	return &Server{log: log, snap: snap}
}

// Handler returns the fully-wired http.Handler: /healthz, then the SPA file
// server with SPA fallback, all behind the logging + panic-recover wrapper.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.handleHealthz)
	mux.HandleFunc("GET /api/standings", s.handleStandings)

	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		// The embed directive guarantees dist/ exists; this is unreachable.
		panic("web: dist embed missing: " + err.Error())
	}
	mux.Handle("GET /", spaFileServer(sub))

	return recoverer(s.log, logRequests(s.log, mux))
}

func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	body := map[string]any{"status": "ok"}
	if snap := s.snap.Current(); snap != nil {
		body["builtAt"] = snap.BuiltAt.UTC().Format(time.RFC3339)
	}
	writeJSON(w, http.StatusOK, body)
}

// spaFileServer serves static files from fsys and falls back to index.html for
// any path that does not resolve to a file, so client-side routes (e.g.
// /manager/:id) load the app.
func spaFileServer(fsys fs.FS) http.Handler {
	fileServer := http.FileServerFS(fsys)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := fs.Stat(fsys, cleanPath(r.URL.Path)); err != nil {
			r = r.Clone(r.Context())
			r.URL.Path = "/"
		}
		fileServer.ServeHTTP(w, r)
	})
}

func cleanPath(p string) string {
	if p == "" || p == "/" {
		return "index.html"
	}
	return p[1:] // strip leading slash for fs.Stat
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// logRequests logs one line per request: method, path, status, duration.
func logRequests(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		log.Info("http",
			"method", r.Method,
			"path", r.URL.Path,
			"status", sw.status,
			"dur", time.Since(start).String(),
		)
	})
}

// recoverer turns a handler panic into a 500 and a logged error instead of
// crashing the process.
func recoverer(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				log.Error("panic in handler", "path", r.URL.Path, "value", v)
				w.WriteHeader(http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

type statusWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (w *statusWriter) WriteHeader(code int) {
	if !w.wroteHeader {
		w.status = code
		w.wroteHeader = true
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(b)
}
