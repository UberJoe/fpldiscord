package web

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type fakeSnap struct {
	t  time.Time
	ok bool
}

func (f fakeSnap) BuiltAt() (time.Time, bool) { return f.t, f.ok }

func testServer(snap SnapshotProvider) http.Handler {
	return New(slog.New(slog.NewTextHandler(io.Discard, nil)), snap).Handler()
}

func TestHealthz_AlwaysOK_WithBuiltAtWhenSnapshotExists(t *testing.T) {
	built := time.Date(2026, 9, 6, 14, 3, 0, 0, time.UTC)
	rec := httptest.NewRecorder()
	testServer(fakeSnap{t: built, ok: true}).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body not JSON: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf("status field = %v, want ok", body["status"])
	}
	if body["builtAt"] != "2026-09-06T14:03:00Z" {
		t.Errorf("builtAt = %v, want 2026-09-06T14:03:00Z", body["builtAt"])
	}
}

func TestHealthz_OKBeforeFirstSnapshot_NoBuiltAt(t *testing.T) {
	rec := httptest.NewRecorder()
	testServer(fakeSnap{ok: false}).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if _, present := body["builtAt"]; present {
		t.Errorf("builtAt present before first snapshot: %v", body["builtAt"])
	}
}

func TestSPA_ServesIndexForUnknownRoute(t *testing.T) {
	rec := httptest.NewRecorder()
	testServer(fakeSnap{ok: false}).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/manager/42", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("SPA fallback status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct == "" {
		t.Error("no Content-Type on SPA response")
	}
}

func TestSPA_ServesIndexAtRoot(t *testing.T) {
	rec := httptest.NewRecorder()
	testServer(fakeSnap{ok: false}).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("root status = %d, want 200", rec.Code)
	}
}

func TestRecoverer_TurnsPanicInto500(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := recoverer(log, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}
