package web

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/UberJoe/fpldiscord/internal/fpl"
)

// waiverSnapshot is a classic snapshot whose processed gameweeks are 3 and 4.
// GW4 has a contested waiver (Sam wins element 20 on priority 1, Joe is
// denied-other on priority 4), a free-agent pickup, and a trade that must never
// reach the feed.
func waiverSnapshot() *fpl.Snapshot {
	return &fpl.Snapshot{
		BuiltAt:    time.Date(2026, 9, 6, 14, 3, 0, 0, time.UTC),
		LeagueName: "FPL Draft 26/27",
		LeagueMode: fpl.ModeClassic,
		CurrentGW:  5,
		Game:       fpl.Game{CurrentEvent: 5},
		Bootstrap: fpl.Bootstrap{Elements: []fpl.Element{
			{ID: 20, WebName: "Salah"},
			{ID: 21, WebName: "Núñez"},
			{ID: 22, WebName: "Højlund"},
			{ID: 23, WebName: "Watkins"},
		}},
		LeagueDetails: fpl.LeagueDetails{
			League: fpl.League{Name: "FPL Draft 26/27", Scoring: "c"},
			LeagueEntries: []fpl.LeagueEntry{
				{ID: 1, EntryID: 500, PlayerFirstName: "Sam"},
				{ID: 2, EntryID: 600, PlayerFirstName: "Joe"},
			},
		},
		Transactions: []fpl.Transaction{
			{Entry: 600, Event: 4, ElementIn: 20, ElementOut: 21, Kind: "w", Result: "do", Priority: 4, Index: 2},
			{Entry: 500, Event: 4, ElementIn: 20, ElementOut: 22, Kind: "w", Result: "a", Priority: 1, Index: 1},
			{Entry: 500, Event: 4, ElementIn: 23, ElementOut: 0, Kind: "f", Result: "a", Priority: 0, Index: 3},
			{Entry: 600, Event: 4, ElementIn: 21, ElementOut: 23, Kind: "t", Result: "a", Priority: 0, Index: 4},
			{Entry: 500, Event: 3, ElementIn: 22, ElementOut: 20, Kind: "w", Result: "a", Priority: 2, Index: 1},
		},
	}
}

func TestWaivers_ResolvedRowsSortedByIndexWithBidOrder(t *testing.T) {
	rec, body := getJSON(t, apiServer(waiverSnapshot()), "/api/waivers?gw=4")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	data := body["data"].(map[string]any)
	if data["gw"].(float64) != 4 {
		t.Errorf("data.gw = %v, want 4", data["gw"])
	}
	rows, ok := data["rows"].([]any)
	if !ok || len(rows) != 3 {
		t.Fatalf("data.rows = %v, want 3 entries (trade excluded)", data["rows"])
	}

	first := rows[0].(map[string]any)
	for _, k := range []string{"ownerName", "entryId", "in", "out", "type", "status", "priority", "index"} {
		if _, present := first[k]; !present {
			t.Errorf("row missing key %q", k)
		}
	}
	if first["index"].(float64) != 1 || first["priority"].(float64) != 1 {
		t.Errorf("rows[0] index/priority = %v/%v, want 1/1", first["index"], first["priority"])
	}
	if first["ownerName"] != "Sam" || first["status"] != "accepted" || first["type"] != "waiver" {
		t.Errorf("rows[0] = %v", first)
	}
	if first["in"] != "Salah" || first["out"] != "Hojlund" {
		t.Errorf("rows[0] in/out = %v/%v, want Salah/Hojlund (accent-stripped)", first["in"], first["out"])
	}

	second := rows[1].(map[string]any)
	if second["ownerName"] != "Joe" || second["status"] != "failed" {
		t.Errorf("rows[1] = %v, want Joe / failed", second)
	}
	if second["in"] != "Salah" || second["priority"].(float64) != 4 {
		t.Errorf("rows[1] in/priority = %v/%v, want Salah out-bid at priority 4", second["in"], second["priority"])
	}

	third := rows[2].(map[string]any)
	if third["type"] != "freeAgent" || third["in"] != "Watkins" || third["out"] != "" {
		t.Errorf("rows[2] = %v, want freeAgent / Watkins / no drop", third)
	}
}

func TestWaivers_OmittedGWResolvesToLatestProcessed(t *testing.T) {
	_, body := getJSON(t, apiServer(waiverSnapshot()), "/api/waivers")
	data := body["data"].(map[string]any)
	if data["gw"].(float64) != 4 {
		t.Errorf("data.gw = %v, want 4 (max processed)", data["gw"])
	}
}

func TestWaivers_OutOfRangeGWClampsAndEchoesResolved(t *testing.T) {
	_, hi := getJSON(t, apiServer(waiverSnapshot()), "/api/waivers?gw=999")
	if got := hi["data"].(map[string]any)["gw"].(float64); got != 4 {
		t.Errorf("gw=999 resolved to %v, want 4", got)
	}
	_, lo := getJSON(t, apiServer(waiverSnapshot()), "/api/waivers?gw=1")
	if got := lo["data"].(map[string]any)["gw"].(float64); got != 3 {
		t.Errorf("gw=1 resolved to %v, want 3", got)
	}
	// A non-numeric value is treated as absent, not a 400.
	rec, junk := getJSON(t, apiServer(waiverSnapshot()), "/api/waivers?gw=abc")
	if rec.Code != http.StatusOK {
		t.Fatalf("gw=abc status = %d, want 200", rec.Code)
	}
	if got := junk["data"].(map[string]any)["gw"].(float64); got != 4 {
		t.Errorf("gw=abc resolved to %v, want 4", got)
	}
}

func TestWaivers_InRangeGWWithNoRowsIsEmptyNotNull(t *testing.T) {
	// GW3 has exactly one row in the fixture; ask for it explicitly.
	_, body := getJSON(t, apiServer(waiverSnapshot()), "/api/waivers?gw=3")
	data := body["data"].(map[string]any)
	if data["gw"].(float64) != 3 {
		t.Fatalf("data.gw = %v, want 3", data["gw"])
	}
	rows, ok := data["rows"].([]any)
	if !ok || len(rows) != 1 {
		t.Fatalf("data.rows = %v, want 1 entry", data["rows"])
	}
}

func TestWaivers_ETagAnd304(t *testing.T) {
	h := apiServer(waiverSnapshot())

	rec1, _ := getJSON(t, h, "/api/waivers?gw=4")
	etag := rec1.Header().Get("ETag")
	// builtAt 2026-09-06T14:03:00Z -> unix 1788703380.
	if etag != `"waivers-4-1788703380"` {
		t.Fatalf("ETag = %q, want \"waivers-4-1788703380\"", etag)
	}

	rec2 := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/waivers?gw=4", nil)
	req.Header.Set("If-None-Match", etag)
	h.ServeHTTP(rec2, req)
	if rec2.Code != http.StatusNotModified {
		t.Fatalf("status = %d, want 304", rec2.Code)
	}
	if rec2.Body.Len() != 0 {
		t.Errorf("304 body = %q, want empty", rec2.Body.String())
	}
}

func TestWaivers_PreSnapshot503(t *testing.T) {
	rec, _ := getJSON(t, apiServer(nil), "/api/waivers?gw=4")
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
	if ra := rec.Header().Get("Retry-After"); ra != "3" {
		t.Errorf("Retry-After = %q, want 3", ra)
	}
}

func TestWaivers_MetaEnvelopePresent(t *testing.T) {
	_, body := getJSON(t, apiServer(waiverSnapshot()), "/api/waivers?gw=4")
	meta, ok := body["meta"].(map[string]any)
	if !ok {
		t.Fatalf("meta missing: %v", body)
	}
	gws, ok := meta["processedGws"].([]any)
	if !ok || len(gws) != 2 {
		t.Errorf("meta.processedGws = %v, want [3 4]", meta["processedGws"])
	}
}
