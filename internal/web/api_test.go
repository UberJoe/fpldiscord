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
			// Draft's Total already includes EventTotal, so the frozen base the
			// live table sorts on is Total-EventTotal: M1 110, M2 95. last_rank
			// mirrors the official rank (nobody moved last week), so the
			// week-over-week arrow matches the live re-sort's +1 / -1.
			Standings: []fpl.Standing{
				{Rank: 1, LastRank: 1, RankSort: 1, LeagueEntry: 1, Total: 150, EventTotal: 40},
				{Rank: 2, LastRank: 2, RankSort: 2, LeagueEntry: 2, Total: 130, EventTotal: 35},
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

// managerSnapshot is a one-manager classic snapshot for GW5 wired for the
// /api/manager drill-down: a full 15-man squad, bootstrap elements/teams/squad
// settings, and a live feed in which the slot-5 DEF (element 6) blanked in a
// finished fixture, so the auto-sub brings on the slot-13 bench DEF (element 7).
// Scoring XI total: 2+5+1+3+6+4+2+7+1+8+3 = 42.
func managerSnapshot() *fpl.Snapshot {
	el := func(id, typ, team int, name string) fpl.Element {
		return fpl.Element{ID: fpl.ElementID(id), WebName: name, ElementType: typ, Team: team}
	}
	pick := func(elem, pos int) fpl.Pick {
		return fpl.Pick{Element: fpl.ElementID(elem), Position: pos, Multiplier: 1}
	}
	stat := func(pts, mins int) fpl.LiveElement {
		return fpl.LiveElement{Stats: fpl.LiveStats{TotalPoints: pts, Minutes: mins}}
	}

	return &fpl.Snapshot{
		BuiltAt:    time.Date(2026, 9, 6, 14, 3, 0, 0, time.UTC),
		LeagueName: "FPL Draft 26/27",
		LeagueMode: fpl.ModeClassic,
		CurrentGW:  5,
		GWFinished: false,
		Game:       fpl.Game{CurrentEvent: 5},
		Bootstrap: fpl.Bootstrap{
			Teams: []fpl.Team{{ID: 1, ShortName: "ARS"}, {ID: 2, ShortName: "CRY"}},
			Settings: fpl.Settings{Squad: fpl.SquadSettings{
				Size: 15, Play: 11,
				MinPlayGKP: 1, MaxPlayGKP: 1, MinPlayDEF: 3, MinPlayMID: 2, MinPlayFWD: 1,
				PositionTypeLocks: map[string]string{"12": "GKP"},
			}},
			Elements: []fpl.Element{
				el(1, 1, 1, "P1"), el(2, 1, 1, "P2"),
				el(3, 2, 1, "P3"), el(4, 2, 1, "P4"), el(5, 2, 1, "P5"),
				el(6, 2, 2, "P6"), el(7, 2, 1, "P7"),
				el(8, 3, 1, "P8"), el(9, 3, 1, "P9"), el(10, 3, 1, "P10"),
				el(11, 3, 1, "P11"), el(12, 3, 1, "P12"),
				el(13, 4, 1, "Håland"), el(14, 4, 1, "P14"), el(15, 4, 1, "P15"),
			},
		},
		LeagueDetails: fpl.LeagueDetails{
			League: fpl.League{Name: "FPL Draft 26/27", Scoring: "c"},
			LeagueEntries: []fpl.LeagueEntry{
				{ID: 1, EntryID: 500, EntryName: "Bruno Dos Tres", PlayerFirstName: "Joe"},
			},
			Standings: []fpl.Standing{{Rank: 1, LeagueEntry: 1, Total: 100}},
		},
		Live: map[int]fpl.LiveGW{
			5: {
				Fixtures: []fpl.LiveFixture{{ID: 1, Started: true, Finished: true, FinishedProvisional: true}},
				Elements: map[fpl.ElementID]fpl.LiveElement{
					1: stat(2, 90),
					3: stat(5, 90), 4: stat(1, 90), 5: stat(3, 90),
					6: {Stats: fpl.LiveStats{TotalPoints: 0, Minutes: 0}, Explain: []fpl.LiveExplain{{Fixture: 1}}},
					7: stat(6, 90),
					8: stat(4, 90), 9: stat(2, 90), 10: stat(7, 90), 11: stat(1, 90),
					13: stat(8, 90), 14: stat(3, 90),
				},
			},
		},
		Entries: map[fpl.EntryID]fpl.EntryEvent{
			500: {Picks: []fpl.Pick{
				pick(1, 1),
				pick(3, 2), pick(4, 3), pick(5, 4), pick(6, 5),
				pick(8, 6), pick(9, 7), pick(10, 8), pick(11, 9),
				pick(13, 10), pick(14, 11),
				pick(2, 12), pick(7, 13), pick(12, 14), pick(15, 15),
			}},
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

func TestManager_EnvelopeFieldsAndFullSquad(t *testing.T) {
	rec, body := getJSON(t, apiServer(managerSnapshot()), "/api/manager/500")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	meta, ok := body["meta"].(map[string]any)
	if !ok {
		t.Fatalf("meta missing or not an object: %v", body["meta"])
	}
	if meta["currentGw"].(float64) != 5 {
		t.Errorf("meta.currentGw = %v, want 5", meta["currentGw"])
	}
	data, ok := body["data"].(map[string]any)
	if !ok {
		t.Fatalf("data missing or not an object: %v", body["data"])
	}

	if data["entryId"].(float64) != 500 {
		t.Errorf("data.entryId = %v, want 500", data["entryId"])
	}
	if data["ownerName"] != "Joe" {
		t.Errorf("data.ownerName = %v, want Joe", data["ownerName"])
	}
	if data["gw"].(float64) != 5 {
		t.Errorf("data.gw = %v, want 5", data["gw"])
	}
	if data["provisional"] != true {
		t.Errorf("data.provisional = %v, want true", data["provisional"])
	}
	if data["total"].(float64) != 42 {
		t.Errorf("data.total = %v, want 42 (auto-subbed XI)", data["total"])
	}

	players, ok := data["players"].([]any)
	if !ok || len(players) != 15 {
		t.Fatalf("data.players = %v, want 15 entries", data["players"])
	}

	for _, k := range []string{
		"elementId", "webName", "teamShort", "pos", "squadSlot",
		"points", "minutes", "inScoringXI", "autoSubbedIn", "autoSubbedOut",
	} {
		if _, present := players[0].(map[string]any)[k]; !present {
			t.Errorf("player object missing key %q", k)
		}
	}

	byElem := func(id float64) map[string]any {
		t.Helper()
		for _, p := range players {
			m := p.(map[string]any)
			if m["elementId"].(float64) == id {
				return m
			}
		}
		t.Fatalf("element %v not in players", id)
		return nil
	}

	out := byElem(6)
	if out["autoSubbedOut"] != true || out["inScoringXI"] != false {
		t.Errorf("element 6 markers = subOut %v inXI %v, want true/false",
			out["autoSubbedOut"], out["inScoringXI"])
	}
	if out["teamShort"] != "CRY" {
		t.Errorf("element 6 teamShort = %v, want CRY", out["teamShort"])
	}

	in := byElem(7)
	if in["autoSubbedIn"] != true || in["inScoringXI"] != true {
		t.Errorf("element 7 markers = subIn %v inXI %v, want true/true",
			in["autoSubbedIn"], in["inScoringXI"])
	}
	if in["squadSlot"].(float64) != 13 {
		t.Errorf("element 7 squadSlot = %v, want 13", in["squadSlot"])
	}

	if byElem(13)["webName"] != "Haland" {
		t.Errorf("element 13 webName = %v, want Haland (accent-stripped)", byElem(13)["webName"])
	}
}

func TestManager_UnknownIdIs404(t *testing.T) {
	rec, body := getJSON(t, apiServer(managerSnapshot()), "/api/manager/999999")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	if body["error"] == nil || body["error"] == "" {
		t.Errorf("404 body missing error string: %v", body)
	}
	if _, present := body["meta"]; present {
		t.Errorf("404 body should carry no envelope: %v", body)
	}
}

func TestManager_NonNumericIdIs404(t *testing.T) {
	rec, _ := getJSON(t, apiServer(managerSnapshot()), "/api/manager/not-a-number")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestManager_ETagAnd304(t *testing.T) {
	h := apiServer(managerSnapshot())

	rec1, _ := getJSON(t, h, "/api/manager/500")
	etag := rec1.Header().Get("ETag")
	// builtAt 2026-09-06T14:03:00Z -> unix 1788703380.
	if etag != `"manager-500-1788703380"` {
		t.Fatalf("ETag = %q, want \"manager-500-1788703380\"", etag)
	}
	if cc := rec1.Header().Get("Cache-Control"); cc != "no-cache" {
		t.Errorf("Cache-Control = %q, want no-cache", cc)
	}

	rec2 := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/manager/500", nil)
	req.Header.Set("If-None-Match", etag)
	h.ServeHTTP(rec2, req)

	if rec2.Code != http.StatusNotModified {
		t.Fatalf("status = %d, want 304", rec2.Code)
	}
	if rec2.Body.Len() != 0 {
		t.Errorf("304 body = %q, want empty", rec2.Body.String())
	}
}

func TestManager_PreSnapshot503(t *testing.T) {
	rec, _ := getJSON(t, apiServer(nil), "/api/manager/500")
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
	if ra := rec.Header().Get("Retry-After"); ra != "3" {
		t.Errorf("Retry-After = %q, want 3", ra)
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

	// M2 is first on live points (frozen 95 + 20 = 115) despite official rank 2.
	if first["entryId"].(float64) != 600 {
		t.Errorf("rows[0].entryId = %v, want 600 (M2)", first["entryId"])
	}
	if first["ownerName"] != "Sam" || first["entryName"] != "Salah Days" {
		t.Errorf("rows[0] names = %v / %v", first["ownerName"], first["entryName"])
	}
	if first["officialRank"].(float64) != 2 || first["liveRank"].(float64) != 1 {
		t.Errorf("rows[0] ranks = official %v live %v, want 2 / 1", first["officialRank"], first["liveRank"])
	}
	// The payload carries last_rank (the "from" end of the arrow) and the arrow
	// is week-over-week: M2 was last_rank 2, is now live rank 1, so +1.
	if first["lastRank"].(float64) != 2 {
		t.Errorf("rows[0].lastRank = %v, want 2", first["lastRank"])
	}
	if first["arrow"].(float64) != 1 {
		t.Errorf("rows[0].arrow = %v, want 1", first["arrow"])
	}
	if first["totalPoints"].(float64) != 95 || first["liveGwPoints"].(float64) != 20 || first["livePoints"].(float64) != 115 {
		t.Errorf("rows[0] points = total %v gw %v live %v, want 95 / 20 / 115",
			first["totalPoints"], first["liveGwPoints"], first["livePoints"])
	}

	// M1 drops to live rank 2 (frozen 110 + 0), arrow -1.
	if second["entryId"].(float64) != 500 {
		t.Errorf("rows[1].entryId = %v, want 500 (M1)", second["entryId"])
	}
	if second["lastRank"].(float64) != 1 {
		t.Errorf("rows[1].lastRank = %v, want 1", second["lastRank"])
	}
	if second["arrow"].(float64) != -1 {
		t.Errorf("rows[1].arrow = %v, want -1", second["arrow"])
	}
	if second["totalPoints"].(float64) != 110 || second["liveGwPoints"].(float64) != 0 || second["livePoints"].(float64) != 110 {
		t.Errorf("rows[1] points = total %v gw %v live %v, want 110 / 0 / 110",
			second["totalPoints"], second["liveGwPoints"], second["livePoints"])
	}
}
