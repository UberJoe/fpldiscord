package fpl

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func mustJSON(t *testing.T, name string, v any) {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", filepath.FromSlash(name)))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	if err := json.Unmarshal(b, v); err != nil {
		t.Fatalf("unmarshal %s: %v", name, err)
	}
}

// scoreSnapshot assembles a Snapshot for GW5 from the score/* fixtures: one
// league member (entry 500) with the given submitted team and live feed, sharing
// game.json / bootstrap.json / details.json.
func scoreSnapshot(t *testing.T, entryEvent, live string) *Snapshot {
	t.Helper()
	var p pieces
	mustJSON(t, "score/game.json", &p.game)
	mustJSON(t, "score/bootstrap.json", &p.bootstrap)
	mustJSON(t, "score/details.json", &p.details)
	mustJSON(t, "score/"+live, &p.live)

	var ev EntryEvent
	mustJSON(t, "score/"+entryEvent, &ev)
	p.entries = map[EntryID]EntryEvent{EntryID(500): ev}
	p.currentGW = resolveCurrentGW(p.game)

	return assemble(p, time.Now().UTC(), false)
}

// loadScoreSnapshot is the default scenario: a submitted 1-4-4-2 whose slot-5 DEF
// blanked in a finished match and whose slot-9 MID blanked in a match still in
// play.
func loadScoreSnapshot(t *testing.T) *Snapshot {
	t.Helper()
	return scoreSnapshot(t, "entry-event.json", "live.json")
}

func TestManagerScore_SumsAutoSubbedXITrustingAPITotals(t *testing.T) {
	snap := loadScoreSnapshot(t)

	ms, err := snap.ManagerScore(EntryID(500), 0) // 0 -> current GW (5)
	if err != nil {
		t.Fatalf("ManagerScore: %v", err)
	}
	if ms.GW != 5 {
		t.Errorf("GW = %d, want 5", ms.GW)
	}
	if len(ms.Players) != 11 {
		t.Fatalf("len(Players) = %d, want 11", len(ms.Players))
	}

	// Blanking slot-5 DEF (105) in a finished match is auto-subbed for the
	// slot-13 DEF (113). Slot-9 MID (109) blanked too but their match is still
	// in play, so they stay.
	if hasElement(ms.Players, 105) {
		t.Error("element 105 not auto-subbed out of a finished match")
	}
	if !hasElement(ms.Players, 113) || !subbedIn(ms.Players, 113) {
		t.Error("element 113 not auto-subbed in")
	}
	if !hasElement(ms.Players, 109) {
		t.Error("element 109 subbed out while their match is unfinished")
	}

	// Total is the plain sum of the scoring XI's live total_points:
	// 101:2 102:6 103:1 104:2 113:6 106:9 107:2 108:5 109:1 110:7 111:2 = 43.
	if ms.Total != 43 {
		t.Errorf("Total = %d, want 43", ms.Total)
	}

	// total_points is taken verbatim — bonus is already in it, never re-added.
	for _, p := range ms.Players {
		if p.Element == 106 && p.Points != 9 {
			t.Errorf("element 106 Points = %d, want 9 (total_points as-is, bonus not re-added)", p.Points)
		}
	}
}

func TestManagerScore_VerbatimSubsFromFixture(t *testing.T) {
	// GW final: subs[] swaps the slot-2 DEF (102, played 90) for the slot-13 DEF
	// (113). Verbatim is authoritative — no provisional sub of the slot-5 blank.
	snap := scoreSnapshot(t, "entry-event-verbatim.json", "live.json")

	ms, err := snap.ManagerScore(EntryID(500), 5)
	if err != nil {
		t.Fatalf("ManagerScore: %v", err)
	}
	if hasElement(ms.Players, 102) || !hasElement(ms.Players, 113) || !subbedIn(ms.Players, 113) {
		t.Errorf("verbatim sub 102->113 not applied: %v", xiElements(ms.Players))
	}
	if !hasElement(ms.Players, 105) {
		t.Error("slot-5 blank (105) provisionally subbed in verbatim mode")
	}
	// 101:2 113:6 103:1 104:2 105:0 106:9 107:2 108:5 109:1 110:7 111:2 = 37.
	if ms.Total != 37 {
		t.Errorf("Total = %d, want 37", ms.Total)
	}
}

func TestManagerScore_BackupGKReplacesStartingGKFromFixture(t *testing.T) {
	// Starting GK (101) blanks in a finished match -> slot-12 backup GK (112).
	snap := scoreSnapshot(t, "entry-event.json", "live-gk-blank.json")

	ms, err := snap.ManagerScore(EntryID(500), 5)
	if err != nil {
		t.Fatalf("ManagerScore: %v", err)
	}
	if hasElement(ms.Players, 101) || !hasElement(ms.Players, 112) || !subbedIn(ms.Players, 112) {
		t.Errorf("backup GK 112 not auto-subbed for starting GK 101: %v", xiElements(ms.Players))
	}
	// 112:6 102:6 103:1 104:2 105:3 106:9 107:2 108:5 109:4 110:7 111:2 = 47.
	if ms.Total != 47 {
		t.Errorf("Total = %d, want 47", ms.Total)
	}
}

func TestManagerScore_SkipsFormationBreakingBenchFromFixture(t *testing.T) {
	// Submitted 1-3-5-2 (DEF at the minimum). A DEF (104) blanks; bench order is
	// backup GK, FWD, DEF, DEF. The FWD would drop DEF below 3, so the auto-sub
	// must skip it for the bench DEF (105).
	snap := scoreSnapshot(t, "entry-event-352.json", "live-352-defblank.json")

	ms, err := snap.ManagerScore(EntryID(500), 5)
	if err != nil {
		t.Fatalf("ManagerScore: %v", err)
	}
	if hasElement(ms.Players, 104) {
		t.Error("blanking DEF 104 still in XI")
	}
	if hasElement(ms.Players, 115) {
		t.Error("bench FWD 115 came on and broke the DEF minimum")
	}
	if !hasElement(ms.Players, 105) || !subbedIn(ms.Players, 105) {
		t.Errorf("bench DEF 105 not auto-subbed in: %v", xiElements(ms.Players))
	}
	// 101:2 102:4 103:3 105:5 106:6 107:2 108:7 109:1 114:4 110:8 111:3 = 45.
	if ms.Total != 45 {
		t.Errorf("Total = %d, want 45", ms.Total)
	}
}

func TestManagerScore_UnknownEntryIsAnError(t *testing.T) {
	snap := loadScoreSnapshot(t)
	if _, err := snap.ManagerScore(EntryID(9999), 5); err == nil {
		t.Fatal("ManagerScore(unknown entry) returned no error")
	}
}

func TestManagerScore_PastGWNotCarriedIsAnError(t *testing.T) {
	snap := loadScoreSnapshot(t)
	if _, err := snap.ManagerScore(EntryID(500), 3); err == nil {
		t.Fatal("ManagerScore for a GW the snapshot does not carry returned no error")
	}
}
