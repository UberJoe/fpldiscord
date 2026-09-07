package fpl

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

// entryEventPath matches /entry/{id}/event/{gw}.
var entryEventPath = regexp.MustCompile(`^/entry/\d+/event/\d+$`)

// draftStub serves the testdata fixtures for every endpoint Build reads. Per-path
// overrides let a test force a bad status or content-type on one endpoint.
type draftStub struct {
	t         *testing.T
	statusFor map[string]int
	ctypeFor  map[string]string
	hitCount  map[string]int
}

func newDraftStub(t *testing.T) *draftStub {
	return &draftStub{
		t:         t,
		statusFor: map[string]int{},
		ctypeFor:  map[string]string{},
		hitCount:  map[string]int{},
	}
}

func (d *draftStub) server() *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(d.handle))
	d.t.Cleanup(srv.Close)
	return srv
}

func (d *draftStub) handle(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("User-Agent") == "" {
		d.t.Errorf("%s: request sent no User-Agent", r.URL.Path)
	}
	if r.Header.Get("Accept") != "application/json" {
		d.t.Errorf("%s: Accept header = %q", r.URL.Path, r.Header.Get("Accept"))
	}
	d.hitCount[r.URL.Path]++

	file := d.fileFor(r.URL.Path)
	if file == "" {
		http.Error(w, "no fixture for "+r.URL.Path, http.StatusNotFound)
		return
	}

	ct := "application/json"
	if v, ok := d.ctypeFor[r.URL.Path]; ok {
		ct = v
	}
	status := http.StatusOK
	if v, ok := d.statusFor[r.URL.Path]; ok {
		status = v
	}
	body, err := os.ReadFile(filepath.Join("testdata", file))
	if err != nil {
		d.t.Fatalf("read fixture %s: %v", file, err)
	}
	w.Header().Set("Content-Type", ct)
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

func (d *draftStub) fileFor(path string) string {
	switch path {
	case "/game":
		return "game.json"
	case "/bootstrap-static":
		return "bootstrap-static.json"
	case "/league/64/details":
		return "league-details.json"
	case "/league/64/element-status":
		return "element-status.json"
	case "/draft/league/64/transactions":
		return "transactions.json"
	case "/event/4/live":
		return "live.json"
	}
	if entryEventPath.MatchString(path) {
		return "entry-event.json"
	}
	return ""
}

func buildFromStub(t *testing.T) (*Snapshot, *draftStub) {
	t.Helper()
	stub := newDraftStub(t)
	srv := stub.server()
	c := NewClient("64")
	c.baseURL = srv.URL
	snap, err := c.Build(context.Background())
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}
	return snap, stub
}

func TestBuild_ParsesEveryEndpoint(t *testing.T) {
	snap, stub := buildFromStub(t)

	if snap.LeagueName != "Coq au Ian" {
		t.Errorf("LeagueName = %q", snap.LeagueName)
	}
	if snap.LeagueMode != ModeClassic {
		t.Errorf("LeagueMode = %q, want classic", snap.LeagueMode)
	}
	if snap.CurrentGW != 4 {
		t.Errorf("CurrentGW = %d, want 4", snap.CurrentGW)
	}
	if snap.GWFinished {
		t.Error("GWFinished = true, want false")
	}
	if snap.ElementCount != 4 {
		t.Errorf("ElementCount = %d, want 4", snap.ElementCount)
	}
	if got := len(snap.Transactions); got != 2 {
		t.Errorf("len(Transactions) = %d, want 2", got)
	}
	if got := len(snap.ElementStatus); got != 4 {
		t.Errorf("len(ElementStatus) = %d, want 4", got)
	}
	if got := len(snap.LeagueDetails.Standings); got != 2 {
		t.Errorf("len(Standings) = %d, want 2", got)
	}
	if len(snap.LeagueDetails.Matches) != 0 {
		t.Errorf("classic league carried %d matches, want 0", len(snap.LeagueDetails.Matches))
	}
	if _, ok := snap.Entries[EntryID(39880)]; !ok {
		t.Error("Entries missing entry_id 39880")
	}
	if _, ok := snap.Entries[EntryID(89)]; !ok {
		t.Error("Entries missing entry_id 89")
	}
	// One /entry/{id}/event/{gw} call per league member.
	entryHits := 0
	for path, n := range stub.hitCount {
		if entryEventPath.MatchString(path) {
			entryHits += n
		}
	}
	if entryHits != 2 {
		t.Errorf("entry-event calls = %d, want 2", entryHits)
	}

	// Squad settings feed ticket-03 auto-subs.
	if snap.Bootstrap.Settings.Squad.PositionTypeLocks["12"] != "GKP" {
		t.Errorf("squad position_type_locks[12] = %q, want GKP", snap.Bootstrap.Settings.Squad.PositionTypeLocks["12"])
	}
}

func TestBuild_EventsLookedUpByIDNotPosition(t *testing.T) {
	snap, _ := buildFromStub(t)

	gw4, ok := snap.Event(4)
	if !ok {
		t.Fatal("Event(4) not found")
	}
	if gw4.Name != "Gameweek 4" || gw4.Finished {
		t.Errorf("Event(4) = %+v, want unfinished Gameweek 4", gw4)
	}
	// events.data[4] (0-indexed) would be GW5 — prove we key on id.
	if gw4.ID != 4 {
		t.Errorf("Event(4).ID = %d", gw4.ID)
	}

	gw5, ok := snap.Event(5)
	if !ok {
		t.Fatal("Event(5) not found")
	}
	if !gw5.WaiversTime.Time().IsZero() {
		t.Errorf("Event(5).WaiversTime = %v, want zero (fixture has null)", gw5.WaiversTime.Time())
	}
	if got := gw4.WaiversTime.Time().UTC().Format("2006-01-02T15:04:05Z"); got != "2026-09-11T17:30:00Z" {
		t.Errorf("Event(4).WaiversTime = %s", got)
	}

	if _, ok := snap.Event(99); ok {
		t.Error("Event(99) unexpectedly found")
	}
}

func TestBuild_IDSpacesNeverConflated(t *testing.T) {
	snap, _ := buildFromStub(t)

	// Bruno: league_entries.id 39936, entry_id 39880 — genuinely different.
	byLeague, ok := snap.EntryByLeagueEntry(LeagueEntryID(39936))
	if !ok || byLeague.PlayerFirstName != "Bruno" {
		t.Fatalf("EntryByLeagueEntry(39936) = %+v, %v", byLeague, ok)
	}
	byEntry, ok := snap.EntryByEntryID(EntryID(39880))
	if !ok || byEntry.PlayerFirstName != "Bruno" {
		t.Fatalf("EntryByEntryID(39880) = %+v, %v", byEntry, ok)
	}
	// The membership id must NOT resolve in the entry-id space.
	if _, ok := snap.EntryByEntryID(EntryID(39936)); ok {
		t.Error("EntryByEntryID(39936) resolved — id spaces conflated")
	}

	// OwnerOf joins element_status.owner (entry_id 39880) -> league_entries.
	owner, ok := snap.OwnerOf(ElementID(11))
	if !ok || owner.PlayerFirstName != "Bruno" {
		t.Fatalf("OwnerOf(11) = %+v, %v; want Bruno", owner, ok)
	}
	// Element 13 is a free agent (owner null).
	if _, ok := snap.OwnerOf(ElementID(13)); ok {
		t.Error("OwnerOf(13) resolved — element 13 is a free agent")
	}
}

func TestBuild_NumericTextFieldsParsedOnIngest(t *testing.T) {
	snap, _ := buildFromStub(t)

	hoj, ok := snap.Element(ElementID(11))
	if !ok {
		t.Fatal("Element(11) not found")
	}
	if hoj.Form.Float() != 4.5 {
		t.Errorf("Element(11).Form = %v, want 4.5", hoj.Form.Float())
	}
	if hoj.PointsPerGame.Float() != 6.0 {
		t.Errorf("Element(11).PointsPerGame = %v, want 6.0", hoj.PointsPerGame.Float())
	}
	if hoj.ExpectedGoals.Float() != 2.10 {
		t.Errorf("Element(11).ExpectedGoals = %v, want 2.10", hoj.ExpectedGoals.Float())
	}

	// Element 12 has "form": null — must parse to 0, not error the whole build.
	munoz, _ := snap.Element(ElementID(12))
	if munoz.Form.Float() != 0 {
		t.Errorf("Element(12).Form = %v, want 0 (null)", munoz.Form.Float())
	}
}

func TestBuild_MatchLiveDerivedFromFixtures(t *testing.T) {
	snap, _ := buildFromStub(t)

	// Fixture 31 is started and not finished_provisional -> a match is live.
	if !snap.MatchLive() {
		t.Error("MatchLive() = false, want true (fixture 31 in play)")
	}

	live, ok := snap.LiveGW(4)
	if !ok {
		t.Fatal("LiveGW(4) missing")
	}
	if got := live.Elements[ElementID(11)].Stats.TotalPoints; got != 9 {
		t.Errorf("live element 11 total_points = %d, want 9", got)
	}
	if got := live.Elements[ElementID(12)].Stats.DefensiveContribution; got != 12 {
		t.Errorf("live element 12 defensive_contribution = %d, want 12", got)
	}
}

func TestBuild_AutocompleteSlicesAreAccentStripped(t *testing.T) {
	snap, _ := buildFromStub(t)

	wantPlayers := map[string]string{ // WebName -> Stripped
		"Højlund": "Hojlund",
		"Muñoz":   "Munoz",
		"Sánchez": "Sanchez",
		"Raya":    "Raya",
	}
	if len(snap.PlayerNames) != len(wantPlayers) {
		t.Fatalf("len(PlayerNames) = %d, want %d", len(snap.PlayerNames), len(wantPlayers))
	}
	for _, pn := range snap.PlayerNames {
		want, ok := wantPlayers[pn.WebName]
		if !ok {
			t.Errorf("unexpected player name %q", pn.WebName)
			continue
		}
		if pn.Stripped != want {
			t.Errorf("PlayerName %q stripped to %q, want %q", pn.WebName, pn.Stripped, want)
		}
	}

	wantOwners := map[string]string{"Bruno": "Bruno", "Ian": "Ian"}
	if len(snap.OwnerNames) != len(wantOwners) {
		t.Fatalf("len(OwnerNames) = %d, want %d", len(snap.OwnerNames), len(wantOwners))
	}
	for _, on := range snap.OwnerNames {
		if _, ok := wantOwners[on.Name]; !ok {
			t.Errorf("unexpected owner name %q", on.Name)
		}
	}
}

func TestBuild_RejectsNon200(t *testing.T) {
	stub := newDraftStub(t)
	stub.statusFor["/bootstrap-static"] = http.StatusInternalServerError
	srv := stub.server()
	c := NewClient("64")
	c.baseURL = srv.URL

	if _, err := c.Build(context.Background()); err == nil {
		t.Fatal("Build() succeeded despite a 500 on bootstrap-static")
	}
}

func TestBuild_RejectsNonJSONContentType(t *testing.T) {
	stub := newDraftStub(t)
	stub.ctypeFor["/game"] = "text/html"
	srv := stub.server()
	c := NewClient("64")
	c.baseURL = srv.URL

	if _, err := c.Build(context.Background()); err == nil {
		t.Fatal("Build() succeeded despite text/html on game")
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

func TestModeFromScoring(t *testing.T) {
	for in, want := range map[string]LeagueMode{"c": ModeClassic, "h": ModeH2H, "": ModeClassic, "m": ModeClassic} {
		if got := modeFromScoring(in); got != want {
			t.Errorf("modeFromScoring(%q) = %q, want %q", in, got, want)
		}
	}
}
