package web

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/UberJoe/fpldiscord/internal/fpl"
	"github.com/UberJoe/fpldiscord/internal/store"
)

// fakePicks is a PicksProvider backed by a literal pick set and a fixed
// generation counter.
type fakePicks struct {
	picks []store.BettorPicks
	gen   uint64
}

func (f fakePicks) CurrentPicks(string) ([]store.BettorPicks, error) { return f.picks, nil }
func (f fakePicks) Gen() uint64                                      { return f.gen }

// fakeNamer maps a subset of Discord user ids to display names; anything else
// misses so the handler falls back to the raw id.
type fakeNamer map[string]string

func (f fakeNamer) MemberName(id string) (string, bool) {
	name, ok := f[id]
	return name, ok
}

// betSnapshot carries a dozen players with fixed cumulative season goals, enough
// to drive three bettors: one in, one provisionally out (a pick on zero), one
// bust (total over 21).
func betSnapshot() *fpl.Snapshot {
	el := func(id, goals int, name string) fpl.Element {
		return fpl.Element{ID: fpl.ElementID(id), WebName: name, GoalsScored: goals}
	}
	return &fpl.Snapshot{
		BuiltAt:    time.Date(2026, 9, 6, 14, 3, 0, 0, time.UTC),
		LeagueName: "FPL Draft 26/27",
		LeagueMode: fpl.ModeClassic,
		CurrentGW:  5,
		Game:       fpl.Game{CurrentEvent: 5},
		Bootstrap: fpl.Bootstrap{Elements: []fpl.Element{
			el(1, 5, "Alpha"), el(2, 5, "Bravo"), el(3, 5, "Charlie"), el(4, 5, "Delta"),
			el(5, 9, "Echo"), el(6, 9, "Foxtrot"), el(7, 9, "Golf"), el(8, 1, "Hotel"),
			el(9, 6, "India"), el(10, 6, "Núñez"), el(11, 5, "Kilo"), el(12, 0, "Lima"),
		}},
	}
}

func betPicks() []store.BettorPicks {
	return []store.BettorPicks{
		{DiscordUserID: "u-alice", Elements: [4]int{1, 2, 3, 4}},    // 20, all scored -> in
		{DiscordUserID: "u-bob", Elements: [4]int{5, 6, 7, 8}},      // 28 -> bust
		{DiscordUserID: "u-carol", Elements: [4]int{9, 10, 11, 12}}, // 17, one on zero -> provisionallyOut
	}
}

func betServer(snap *fpl.Snapshot, picks PicksProvider, namer MemberNamer) http.Handler {
	return New(slog.New(slog.NewTextHandler(io.Discard, nil)), staticProvider{snap: snap}).
		WithBet(picks, namer, "2026/27").
		Handler()
}

func TestBet_EnvelopeShapeAndFields(t *testing.T) {
	h := betServer(betSnapshot(), fakePicks{picks: betPicks(), gen: 7}, fakeNamer{
		"u-alice": "Alice", "u-carol": "Carol",
	})
	rec, body := getJSON(t, h, "/api/bet")
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
	if data["season"] != "2026/27" {
		t.Errorf("data.season = %v, want 2026/27", data["season"])
	}

	bettors, ok := data["bettors"].([]any)
	if !ok || len(bettors) != 3 {
		t.Fatalf("data.bettors = %v, want 3 rows", data["bettors"])
	}

	first := bettors[0].(map[string]any)
	for _, k := range []string{"displayName", "picks", "total", "status", "leader"} {
		if _, present := first[k]; !present {
			t.Errorf("bettor object missing key %q", k)
		}
	}
	picks, ok := first["picks"].([]any)
	if !ok || len(picks) != 4 {
		t.Fatalf("bettor.picks = %v, want 4 entries", first["picks"])
	}
	for _, k := range []string{"elementId", "webName", "goals"} {
		if _, present := picks[0].(map[string]any)[k]; !present {
			t.Errorf("pick object missing key %q", k)
		}
	}
}

func TestBet_SortedByTotalDescWithBustsLastAndLeaderMarked(t *testing.T) {
	h := betServer(betSnapshot(), fakePicks{picks: betPicks(), gen: 1}, fakeNamer{
		"u-alice": "Alice", "u-carol": "Carol",
	})
	_, body := getJSON(t, h, "/api/bet")
	bettors := body["data"].(map[string]any)["bettors"].([]any)

	names := make([]string, len(bettors))
	for i, b := range bettors {
		names[i] = b.(map[string]any)["displayName"].(string)
	}
	// Alice (20, in) then Carol (17, provisionallyOut) then Bob (28, bust last).
	want := []string{"Alice", "Carol", "u-bob"}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("order = %v, want %v", names, want)
		}
	}

	alice := bettors[0].(map[string]any)
	if alice["total"].(float64) != 20 || alice["status"] != "in" || alice["leader"] != true {
		t.Errorf("alice = %v, want total 20 / in / leader", alice)
	}
	carol := bettors[1].(map[string]any)
	if carol["status"] != "provisionallyOut" || carol["leader"] != false {
		t.Errorf("carol = %v, want provisionallyOut / not leader", carol)
	}
	bob := bettors[2].(map[string]any)
	if bob["status"] != "bust" || bob["total"].(float64) != 28 {
		t.Errorf("bob = %v, want bust / 28", bob)
	}
}

func TestBet_DisplayNameFallsBackToRawIdOnMiss(t *testing.T) {
	// Only Alice is known to the namer; Bob and Carol fall back to their id.
	h := betServer(betSnapshot(), fakePicks{picks: betPicks(), gen: 1}, fakeNamer{"u-alice": "Alice"})
	_, body := getJSON(t, h, "/api/bet")
	bettors := body["data"].(map[string]any)["bettors"].([]any)

	got := map[string]bool{}
	for _, b := range bettors {
		got[b.(map[string]any)["displayName"].(string)] = true
	}
	if !got["Alice"] || !got["u-bob"] || !got["u-carol"] {
		t.Errorf("displayNames = %v, want Alice + raw-id fallbacks u-bob / u-carol", got)
	}
}

func TestBet_NilNamerFallsBackToRawId(t *testing.T) {
	h := betServer(betSnapshot(), fakePicks{picks: betPicks(), gen: 1}, nil)
	_, body := getJSON(t, h, "/api/bet")
	first := body["data"].(map[string]any)["bettors"].([]any)[0].(map[string]any)
	if first["displayName"] != "u-alice" {
		t.Errorf("displayName = %v, want raw id u-alice", first["displayName"])
	}
}

func TestBet_PicksCarryWebNameAccentStrippedAndGoals(t *testing.T) {
	h := betServer(betSnapshot(), fakePicks{picks: betPicks(), gen: 1}, fakeNamer{})
	_, body := getJSON(t, h, "/api/bet")
	bettors := body["data"].(map[string]any)["bettors"].([]any)

	// Carol's slot-2 pick is element 10, "Núñez" with 6 goals.
	var carol map[string]any
	for _, b := range bettors {
		if b.(map[string]any)["displayName"] == "u-carol" {
			carol = b.(map[string]any)
		}
	}
	if carol == nil {
		t.Fatal("carol row missing")
	}
	p := carol["picks"].([]any)[1].(map[string]any)
	if p["elementId"].(float64) != 10 || p["webName"] != "Nunez" || p["goals"].(float64) != 6 {
		t.Errorf("carol pick[1] = %v, want elementId 10 / Nunez / 6", p)
	}
}

func TestBet_ETagFoldsStoreGenAnd304(t *testing.T) {
	// Every bettor's name resolves, so the ETag is the plain form.
	h := betServer(betSnapshot(), fakePicks{picks: betPicks(), gen: 42},
		fakeNamer{"u-alice": "Alice", "u-bob": "Bob", "u-carol": "Carol"})

	rec1, _ := getJSON(t, h, "/api/bet")
	etag := rec1.Header().Get("ETag")
	// builtAt 2026-09-06T14:03:00Z -> unix 1788703380; gen 42.
	if etag != `"bet-42-1788703380"` {
		t.Fatalf("ETag = %q, want \"bet-42-1788703380\"", etag)
	}
	if cc := rec1.Header().Get("Cache-Control"); cc != "no-cache" {
		t.Errorf("Cache-Control = %q, want no-cache", cc)
	}

	rec2 := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/bet", nil)
	req.Header.Set("If-None-Match", etag)
	h.ServeHTTP(rec2, req)
	if rec2.Code != http.StatusNotModified {
		t.Fatalf("status = %d, want 304", rec2.Code)
	}
	if rec2.Body.Len() != 0 {
		t.Errorf("304 body = %q, want empty", rec2.Body.String())
	}
}

func TestBet_ETagMovesWhenStoreGenChanges(t *testing.T) {
	full := fakeNamer{"u-alice": "Alice", "u-bob": "Bob", "u-carol": "Carol"}
	a := betServer(betSnapshot(), fakePicks{picks: betPicks(), gen: 1}, full)
	b := betServer(betSnapshot(), fakePicks{picks: betPicks(), gen: 2}, full)

	recA, _ := getJSON(t, a, "/api/bet")
	recB, _ := getJSON(t, b, "/api/bet")
	if recA.Header().Get("ETag") == recB.Header().Get("ETag") {
		t.Errorf("ETag did not change with storeGen: %q", recA.Header().Get("ETag"))
	}
}

func TestBet_PartialNameResolutionGetsADistinctETag(t *testing.T) {
	full := betServer(betSnapshot(), fakePicks{picks: betPicks(), gen: 5},
		fakeNamer{"u-alice": "Alice", "u-bob": "Bob", "u-carol": "Carol"})
	cold := betServer(betSnapshot(), fakePicks{picks: betPicks(), gen: 5}, fakeNamer{})

	recFull, _ := getJSON(t, full, "/api/bet")
	recCold, _ := getJSON(t, cold, "/api/bet")

	if recFull.Header().Get("ETag") == recCold.Header().Get("ETag") {
		t.Errorf("cold (raw-id) and warm responses share an ETag: %q", recFull.Header().Get("ETag"))
	}
	if !strings.HasSuffix(strings.Trim(recCold.Header().Get("ETag"), `"`), "-partial") {
		t.Errorf("cold ETag = %q, want a -partial suffix", recCold.Header().Get("ETag"))
	}
}

func TestBet_PreSnapshot503(t *testing.T) {
	h := betServer(nil, fakePicks{picks: betPicks()}, fakeNamer{})
	rec, body := getJSON(t, h, "/api/bet")
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
	if rec.Header().Get("Retry-After") != "3" {
		t.Errorf("Retry-After = %q, want 3", rec.Header().Get("Retry-After"))
	}
	if body["error"] != "starting up" {
		t.Errorf("error = %v, want \"starting up\"", body["error"])
	}
}

func TestBet_MetaEnvelopePresent(t *testing.T) {
	h := betServer(betSnapshot(), fakePicks{picks: betPicks(), gen: 1}, fakeNamer{})
	_, body := getJSON(t, h, "/api/bet")
	meta, ok := body["meta"].(map[string]any)
	if !ok {
		t.Fatalf("meta missing: %v", body)
	}
	if meta["builtAt"] != "2026-09-06T14:03:00Z" {
		t.Errorf("meta.builtAt = %v", meta["builtAt"])
	}
}

// Compile-time proof the real types satisfy the consumer interfaces.
var (
	_ PicksProvider = (*store.Store)(nil)
)
