package web

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/UberJoe/fpldiscord/internal/fpl"
)

// staticProvider is a fake SnapshotProvider that always returns the same
// snapshot (or nil to model the pre-first-snapshot boot window).
type staticProvider struct{ snap *fpl.Snapshot }

func (p staticProvider) Current() *fpl.Snapshot { return p.snap }

func apiServer(snap *fpl.Snapshot) http.Handler {
	return New(slog.New(slog.NewTextHandler(io.Discard, nil)), staticProvider{snap: snap}).Handler()
}

// xiPicks returns a submitted XI (positions 1..11) starting at the given element id.
func xiPicks(startElem int) []fpl.Pick {
	ps := make([]fpl.Pick, 0, 11)
	for i := 0; i < 11; i++ {
		ps = append(ps, fpl.Pick{Element: fpl.ElementID(startElem + i), Position: i + 1, Multiplier: 1})
	}
	return ps
}

// classicSnapshot is a two-manager classic-league snapshot for GW5.
//
//   - M1 (LeagueEntryID 1, EntryID 500): official rank 1, season total 100, no
//     live points this GW.
//   - M2 (LeagueEntryID 2, EntryID 600): official rank 2, season total 90, 20
//     live points this GW (element 201).
//
// So live order is M2 (110) then M1 (100): the live GW flips the top two.
func classicSnapshot() *fpl.Snapshot {
	return &fpl.Snapshot{
		BuiltAt:    time.Date(2026, 9, 6, 14, 3, 0, 0, time.UTC),
		LeagueName: "FPL Draft 26/27",
		LeagueMode: fpl.ModeClassic,
		CurrentGW:  5,
		GWFinished: false,
		Game:       fpl.Game{CurrentEvent: 5, WaiversProcessed: false},
		Transactions: []fpl.Transaction{
			{Kind: "w", Event: 3},
			{Kind: "w", Event: 4},
		},
		LeagueDetails: fpl.LeagueDetails{
			League: fpl.League{Name: "FPL Draft 26/27", Scoring: "c"},
			LeagueEntries: []fpl.LeagueEntry{
				{ID: 1, EntryID: 500, EntryName: "Bruno Dos Tres", PlayerFirstName: "Joe"},
				{ID: 2, EntryID: 600, EntryName: "Salah Days", PlayerFirstName: "Sam"},
			},
			Standings: []fpl.Standing{
				{Rank: 1, LeagueEntry: 1, Total: 100, EventTotal: 12},
				{Rank: 2, LeagueEntry: 2, Total: 90, EventTotal: 40},
			},
		},
		Live: map[int]fpl.LiveGW{
			5: {
				Fixtures: []fpl.LiveFixture{{ID: 1, Started: true, FinishedProvisional: true}},
				Elements: map[fpl.ElementID]fpl.LiveElement{
					201: {Stats: fpl.LiveStats{TotalPoints: 20, Minutes: 90}},
				},
			},
		},
		Entries: map[fpl.EntryID]fpl.EntryEvent{
			500: {Picks: xiPicks(101)},
			600: {Picks: xiPicks(201)},
		},
	}
}

func getJSON(t *testing.T, h http.Handler, path string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	var body map[string]any
	if rec.Body.Len() > 0 {
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("%s: body not JSON: %v\n%s", path, err, rec.Body.String())
		}
	}
	return rec, body
}

func TestStandings_EnvelopeShapeAndMetaKeys(t *testing.T) {
	rec, body := getJSON(t, apiServer(classicSnapshot()), "/api/standings")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	meta, ok := body["meta"].(map[string]any)
	if !ok {
		t.Fatalf("meta missing or not an object: %v", body["meta"])
	}
	if _, ok := body["data"].(map[string]any); !ok {
		t.Fatalf("data missing or not an object: %v", body["data"])
	}

	wantKeys := []string{
		"matchLive", "pollAfterMs", "stale", "builtAt", "leagueName",
		"leagueMode", "currentGw", "gwFinished", "processedGws",
	}
	for _, k := range wantKeys {
		if _, present := meta[k]; !present {
			t.Errorf("meta.%s missing", k)
		}
	}
	if meta["builtAt"] != "2026-09-06T14:03:00Z" {
		t.Errorf("meta.builtAt = %v, want 2026-09-06T14:03:00Z", meta["builtAt"])
	}
	if meta["leagueName"] != "FPL Draft 26/27" {
		t.Errorf("meta.leagueName = %v", meta["leagueName"])
	}
	if meta["leagueMode"] != "classic" {
		t.Errorf("meta.leagueMode = %v, want classic", meta["leagueMode"])
	}
	if meta["currentGw"].(float64) != 5 {
		t.Errorf("meta.currentGw = %v, want 5", meta["currentGw"])
	}
	gws, ok := meta["processedGws"].([]any)
	if !ok || len(gws) != 2 || gws[0].(float64) != 3 || gws[1].(float64) != 4 {
		t.Errorf("meta.processedGws = %v, want [3 4]", meta["processedGws"])
	}
}

func TestStandings_PollAfterMsFlipsWithMatchLive(t *testing.T) {
	// Idle: the one fixture is finished-provisional -> not live -> 60000.
	_, idle := getJSON(t, apiServer(classicSnapshot()), "/api/standings")
	if got := idle["meta"].(map[string]any)["pollAfterMs"].(float64); got != 60000 {
		t.Errorf("idle pollAfterMs = %v, want 60000", got)
	}

	// Live: a started, not-yet-finished fixture -> live -> 20000.
	snap := classicSnapshot()
	snap.Live[5] = fpl.LiveGW{
		Fixtures: []fpl.LiveFixture{{ID: 1, Started: true, FinishedProvisional: false}},
		Elements: snap.Live[5].Elements,
	}
	_, live := getJSON(t, apiServer(snap), "/api/standings")
	m := live["meta"].(map[string]any)
	if got := m["pollAfterMs"].(float64); got != 20000 {
		t.Errorf("live pollAfterMs = %v, want 20000", got)
	}
	if m["matchLive"] != true {
		t.Errorf("live matchLive = %v, want true", m["matchLive"])
	}
}

func TestStandings_StalePassthrough(t *testing.T) {
	snap := classicSnapshot()
	snap.Stale = true
	_, body := getJSON(t, apiServer(snap), "/api/standings")
	if body["meta"].(map[string]any)["stale"] != true {
		t.Errorf("meta.stale = %v, want true", body["meta"].(map[string]any)["stale"])
	}
}

func TestAPI_PreSnapshot503WithRetryAfter(t *testing.T) {
	rec, body := getJSON(t, apiServer(nil), "/api/standings")

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
	if ra := rec.Header().Get("Retry-After"); ra != "3" {
		t.Errorf("Retry-After = %q, want 3", ra)
	}
	if body["error"] != "starting up" {
		t.Errorf("error = %v, want \"starting up\"", body["error"])
	}
	if body["retryAfterMs"].(float64) != 3000 {
		t.Errorf("retryAfterMs = %v, want 3000", body["retryAfterMs"])
	}
	if _, present := body["meta"]; present {
		t.Errorf("503 body should have no meta envelope: %v", body)
	}
}

func TestStandings_ETagAnd304(t *testing.T) {
	h := apiServer(classicSnapshot())

	rec1, _ := getJSON(t, h, "/api/standings")
	etag := rec1.Header().Get("ETag")
	// builtAt is 2026-09-06T14:03:00Z -> unix 1788703380.
	if etag != `"standings-1788703380"` {
		t.Fatalf("ETag = %q, want \"standings-1788703380\"", etag)
	}
	if cc := rec1.Header().Get("Cache-Control"); cc != "no-cache" {
		t.Errorf("Cache-Control = %q, want no-cache", cc)
	}

	rec2 := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/standings", nil)
	req.Header.Set("If-None-Match", etag)
	h.ServeHTTP(rec2, req)

	if rec2.Code != http.StatusNotModified {
		t.Fatalf("status = %d, want 304", rec2.Code)
	}
	if rec2.Body.Len() != 0 {
		t.Errorf("304 body = %q, want empty", rec2.Body.String())
	}
}

func TestStandings_H2HReturnsEmptyRows(t *testing.T) {
	snap := classicSnapshot()
	snap.LeagueMode = fpl.ModeH2H
	snap.LeagueDetails.League.Scoring = "h"

	rec, body := getJSON(t, apiServer(snap), "/api/standings")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	data := body["data"].(map[string]any)
	rows, ok := data["rows"].([]any)
	if !ok {
		t.Fatalf("data.rows is not an array (want [], got %T %v)", data["rows"], data["rows"])
	}
	if len(rows) != 0 {
		t.Errorf("data.rows = %v, want empty", rows)
	}
}

func TestStandings_PreSortedByLivePointsWithRanksAndArrows(t *testing.T) {
	_, body := getJSON(t, apiServer(classicSnapshot()), "/api/standings")

	rows := body["data"].(map[string]any)["rows"].([]any)
	if len(rows) != 2 {
		t.Fatalf("len(rows) = %d, want 2", len(rows))
	}

	first := rows[0].(map[string]any)
	second := rows[1].(map[string]any)

	// M2 is first on live points (90 + 20 = 110) despite official rank 2.
	if first["entryId"].(float64) != 600 {
		t.Errorf("rows[0].entryId = %v, want 600 (M2)", first["entryId"])
	}
	if first["ownerName"] != "Sam" || first["entryName"] != "Salah Days" {
		t.Errorf("rows[0] names = %v / %v", first["ownerName"], first["entryName"])
	}
	if first["officialRank"].(float64) != 2 || first["liveRank"].(float64) != 1 {
		t.Errorf("rows[0] ranks = official %v live %v, want 2 / 1", first["officialRank"], first["liveRank"])
	}
	if first["arrow"].(float64) != 1 {
		t.Errorf("rows[0].arrow = %v, want 1", first["arrow"])
	}
	if first["totalPoints"].(float64) != 90 || first["liveGwPoints"].(float64) != 20 || first["livePoints"].(float64) != 110 {
		t.Errorf("rows[0] points = total %v gw %v live %v, want 90 / 20 / 110",
			first["totalPoints"], first["liveGwPoints"], first["livePoints"])
	}

	// M1 drops to live rank 2 (100 + 0), arrow -1.
	if second["entryId"].(float64) != 500 {
		t.Errorf("rows[1].entryId = %v, want 500 (M1)", second["entryId"])
	}
	if second["arrow"].(float64) != -1 {
		t.Errorf("rows[1].arrow = %v, want -1", second["arrow"])
	}
	if second["liveGwPoints"].(float64) != 0 || second["livePoints"].(float64) != 100 {
		t.Errorf("rows[1] points = gw %v live %v, want 0 / 100", second["liveGwPoints"], second["livePoints"])
	}
}
