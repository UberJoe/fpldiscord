package fpl

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// draftStub serves canned bodies for the three endpoints Build() reads.
type draftStub struct {
	bootstrap   string
	game        string
	details     string
	contentType string
	status      int
}

func (d draftStub) server(t *testing.T) *httptest.Server {
	t.Helper()
	ct := d.contentType
	if ct == "" {
		ct = "application/json"
	}
	status := d.status
	if status == 0 {
		status = http.StatusOK
	}
	mux := http.NewServeMux()
	write := func(body string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("User-Agent") == "" {
				t.Errorf("%s: request sent no User-Agent", r.URL.Path)
			}
			w.Header().Set("Content-Type", ct)
			w.WriteHeader(status)
			_, _ = w.Write([]byte(body))
		}
	}
	mux.HandleFunc("/bootstrap-static", write(d.bootstrap))
	mux.HandleFunc("/game", write(d.game))
	mux.HandleFunc("/league/64/details", write(d.details))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func clientFor(srv *httptest.Server) *Client {
	c := NewClient("64")
	c.baseURL = srv.URL
	return c
}

func TestBuild_ParsesMinimalSnapshot(t *testing.T) {
	srv := draftStub{
		bootstrap: `{"elements":[{"id":1},{"id":2},{"id":3}],"events":{"current":4}}`,
		game:      `{"current_event":4,"current_event_finished":false,"next_event":5}`,
		details:   `{"league":{"name":"FPL Draft 26/27","scoring":"c"}}`,
	}.server(t)

	snap, err := clientFor(srv).Build(context.Background())
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}
	if snap.LeagueName != "FPL Draft 26/27" {
		t.Errorf("LeagueName = %q", snap.LeagueName)
	}
	if snap.LeagueMode != ModeClassic {
		t.Errorf("LeagueMode = %q, want classic", snap.LeagueMode)
	}
	if snap.CurrentGW != 4 {
		t.Errorf("CurrentGW = %d, want 4", snap.CurrentGW)
	}
	if snap.ElementCount != 3 {
		t.Errorf("ElementCount = %d, want 3", snap.ElementCount)
	}
	if snap.BuiltAt.IsZero() {
		t.Error("BuiltAt is zero")
	}
}

func TestBuild_H2HModeDerivedFromScoring(t *testing.T) {
	srv := draftStub{
		bootstrap: `{"elements":[],"events":{"current":1}}`,
		game:      `{"current_event":1}`,
		details:   `{"league":{"name":"x","scoring":"h"}}`,
	}.server(t)

	snap, err := clientFor(srv).Build(context.Background())
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}
	if snap.LeagueMode != ModeH2H {
		t.Errorf("LeagueMode = %q, want h2h", snap.LeagueMode)
	}
}

func TestBuild_NullCurrentEventFallsBackToNext(t *testing.T) {
	srv := draftStub{
		bootstrap: `{"elements":[],"events":{"current":null}}`,
		game:      `{"current_event":null,"next_event":1}`,
		details:   `{"league":{"name":"x","scoring":"c"}}`,
	}.server(t)

	snap, err := clientFor(srv).Build(context.Background())
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}
	if snap.CurrentGW != 1 {
		t.Errorf("CurrentGW = %d, want 1 (next_event fallback)", snap.CurrentGW)
	}
}

func TestBuild_RejectsNon200(t *testing.T) {
	srv := draftStub{
		bootstrap: `{}`, game: `{}`, details: `{}`,
		status: http.StatusInternalServerError,
	}.server(t)

	if _, err := clientFor(srv).Build(context.Background()); err == nil {
		t.Fatal("Build() succeeded on 500; want error")
	}
}

func TestBuild_RejectsNonJSONContentType(t *testing.T) {
	srv := draftStub{
		bootstrap:   `<html>nope</html>`,
		game:        `{}`,
		details:     `{}`,
		contentType: "text/html",
	}.server(t)

	if _, err := clientFor(srv).Build(context.Background()); err == nil {
		t.Fatal("Build() succeeded on text/html; want error")
	}
}

func TestStore_CurrentNilUntilSet(t *testing.T) {
	s := NewStore()
	if s.Current() != nil {
		t.Fatal("Current() non-nil before first Set")
	}
	snap := &Snapshot{LeagueName: "x"}
	s.Set(snap)
	if s.Current() != snap {
		t.Fatal("Current() did not return the set snapshot")
	}
}

func TestBuildFirst_PublishesAndReturnsNil(t *testing.T) {
	srv := draftStub{
		bootstrap: `{"elements":[],"events":{"current":1}}`,
		game:      `{"current_event":1}`,
		details:   `{"league":{"name":"x","scoring":"c"}}`,
	}.server(t)
	store := NewStore()

	if err := BuildFirst(context.Background(), clientFor(srv), store, 5*time.Second); err != nil {
		t.Fatalf("BuildFirst() error: %v", err)
	}
	if store.Current() == nil {
		t.Fatal("BuildFirst() did not publish a snapshot")
	}
}

func TestBuildFirst_TimesOutWhenBuildNeverSucceeds(t *testing.T) {
	srv := draftStub{bootstrap: `{}`, game: `{}`, details: `{}`, status: http.StatusBadGateway}.server(t)
	store := NewStore()

	err := BuildFirst(context.Background(), clientFor(srv), store, 300*time.Millisecond)
	if err == nil {
		t.Fatal("BuildFirst() returned nil on a permanently failing upstream")
	}
	if store.Current() != nil {
		t.Fatal("BuildFirst() published a snapshot despite never succeeding")
	}
}
