package fpl

import (
	"sort"
	"testing"
)

// stdSquad mirrors league 64's settings.squad: 15-man squad, 11 play, one GK,
// at least 3 DEF / 2 MID / 1 FWD, slot 12 locked to a keeper.
var stdSquad = SquadSettings{
	Size: 15, SelectGKP: 2, SelectDEF: 5, SelectMID: 5, SelectFWD: 3, Play: 11,
	MinPlayGKP: 1, MaxPlayGKP: 1, MinPlayDEF: 3, MinPlayMID: 2, MinPlayFWD: 1,
	PositionTypeLocks: map[string]string{"12": "GKP"},
}

func pick(el ElementID, slot int, pos Pos) Pick {
	return Pick{Element: el, Position: slot, Pos: pos, Multiplier: 1}
}

// squad1442 is a valid submitted 15: GK / 4 DEF / 4 MID / 2 FWD starting, with a
// backup GK, DEF, MID and FWD on the bench in slots 12-15.
func squad1442() []Pick {
	return []Pick{
		pick(1, 1, PosGK),
		pick(2, 2, PosDEF), pick(3, 3, PosDEF), pick(4, 4, PosDEF), pick(5, 5, PosDEF),
		pick(6, 6, PosMID), pick(7, 7, PosMID), pick(8, 8, PosMID), pick(9, 9, PosMID),
		pick(10, 10, PosFWD), pick(11, 11, PosFWD),
		pick(12, 12, PosGK), pick(13, 13, PosDEF), pick(14, 14, PosMID), pick(15, 15, PosFWD),
	}
}

// liveWith builds a LiveGW: every element in mins gets that many minutes, every
// fixture id in doneFixtures is finished-provisional (others are in play), and
// each element in explains is linked to the given fixture.
func liveWith(mins map[ElementID]int, explains map[ElementID]int, doneFixtures ...int) LiveGW {
	l := LiveGW{Elements: map[ElementID]LiveElement{}}
	seen := map[int]bool{}
	addFix := func(id int, done bool) {
		if seen[id] {
			return
		}
		seen[id] = true
		l.Fixtures = append(l.Fixtures, LiveFixture{ID: id, Started: true, Finished: done, FinishedProvisional: done})
	}
	for _, id := range doneFixtures {
		addFix(id, true)
	}
	for _, fx := range explains {
		addFix(fx, false)
	}
	for el, m := range mins {
		le := LiveElement{Stats: LiveStats{Minutes: m}}
		if fx, ok := explains[el]; ok {
			le.Explain = []LiveExplain{{Fixture: fx}}
		}
		l.Elements[el] = le
	}
	return l
}

func xiElements(xi []ScoredPlayer) []ElementID {
	out := make([]ElementID, len(xi))
	for i, p := range xi {
		out[i] = p.Element
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func hasElement(xi []ScoredPlayer, el ElementID) bool {
	for _, p := range xi {
		if p.Element == el {
			return true
		}
	}
	return false
}

func subbedIn(xi []ScoredPlayer, el ElementID) bool {
	for _, p := range xi {
		if p.Element == el {
			return p.AutoSubbedIn
		}
	}
	return false
}

func TestApplyAutoSubs_VerbatimSubsAreAuthoritative(t *testing.T) {
	picks := squad1442()
	// The GW is final: subs[] says swap the slot-2 DEF for the slot-13 DEF,
	// even though element 2 played a full 90. Verbatim wins over minutes.
	subs := []Sub{{ElementOut: 2, ElementIn: 13, Event: 4}}
	live := liveWith(map[ElementID]int{2: 90, 13: 20}, nil)

	xi := ApplyAutoSubs(picks, subs, live, stdSquad)

	if len(xi) != 11 {
		t.Fatalf("len(xi) = %d, want 11", len(xi))
	}
	if hasElement(xi, 2) {
		t.Error("element 2 still in XI — verbatim sub not applied")
	}
	if !hasElement(xi, 13) {
		t.Error("element 13 not brought into XI by verbatim sub")
	}
	if !subbedIn(xi, 13) {
		t.Error("element 13 not marked AutoSubbedIn")
	}
}

func TestApplyAutoSubs_UnfinishedMatchIsNeverSubbedOut(t *testing.T) {
	picks := squad1442()
	// Slot-5 DEF is on zero minutes but their fixture (id 7) is still in play.
	live := liveWith(
		map[ElementID]int{1: 90, 2: 90, 3: 90, 4: 90, 5: 0, 6: 90, 7: 90, 8: 90, 9: 90, 10: 90, 11: 90},
		map[ElementID]int{5: 7}, // fixture 7 not in doneFixtures -> in play
	)

	xi := ApplyAutoSubs(picks, nil, live, stdSquad)

	if !hasElement(xi, 5) {
		t.Error("element 5 subbed out while their match is unfinished")
	}
	if hasElement(xi, 13) {
		t.Error("bench element 13 came on for an unfinished-match starter")
	}
}

func TestApplyAutoSubs_NoFixtureLinkIsNeverSubbedOut(t *testing.T) {
	picks := squad1442()
	// Slot-5 DEF is on zero minutes with no explain block yet (kickoff to come).
	live := liveWith(
		map[ElementID]int{1: 90, 2: 90, 3: 90, 4: 90, 5: 0, 6: 90, 7: 90, 8: 90, 9: 90, 10: 90, 11: 90},
		nil,
	)

	xi := ApplyAutoSubs(picks, nil, live, stdSquad)

	if !hasElement(xi, 5) {
		t.Error("element 5 subbed out with no fixture confirmed finished")
	}
}

func TestApplyAutoSubs_BackupGKOnlyReplacesStartingGK(t *testing.T) {
	picks := squad1442()

	// The starting GK blanks in a finished match -> the slot-12 backup GK comes on.
	gkLive := liveWith(
		map[ElementID]int{1: 0, 2: 90, 3: 90, 4: 90, 5: 90, 6: 90, 7: 90, 8: 90, 9: 90, 10: 90, 11: 90},
		map[ElementID]int{1: 7}, 7,
	)
	xi := ApplyAutoSubs(picks, nil, gkLive, stdSquad)
	if hasElement(xi, 1) || !hasElement(xi, 12) || !subbedIn(xi, 12) {
		t.Errorf("starting GK not replaced by backup GK: xi=%v", xiElements(xi))
	}

	// An outfielder blanks -> the backup GK is skipped and the first bench
	// outfielder (slot-13 DEF) comes on instead; the XI stays legal.
	fwdLive := liveWith(
		map[ElementID]int{1: 90, 2: 90, 3: 90, 4: 90, 5: 90, 6: 90, 7: 90, 8: 90, 9: 90, 10: 90, 11: 0},
		map[ElementID]int{11: 7}, 7,
	)
	xi = ApplyAutoSubs(picks, nil, fwdLive, stdSquad)
	if hasElement(xi, 12) {
		t.Error("backup GK came on for an outfield blank")
	}
	if hasElement(xi, 11) {
		t.Error("blanking FWD (element 11) still in XI")
	}
	if !hasElement(xi, 13) || !subbedIn(xi, 13) {
		t.Errorf("first bench outfielder not auto-subbed in for the outfield blank: xi=%v", xiElements(xi))
	}
	if !formationOK(picksOf(xi), stdSquad) {
		t.Errorf("resulting XI is not a valid formation: xi=%v", xiElements(xi))
	}
}

func TestApplyAutoSubs_SkipsBenchPlayerThatBreaksFormation(t *testing.T) {
	// Submitted XI is GK / 3 DEF / 5 MID / 2 FWD, at the DEF minimum. Bench order
	// is backup GK, FWD, DEF, DEF. A DEF blanks: the first outfield bench player
	// (a FWD) would drop DEF to 2 (< min 3), so it must be skipped for the DEF
	// two slots further down the bench.
	picks := []Pick{
		pick(1, 1, PosGK),
		pick(2, 2, PosDEF), pick(3, 3, PosDEF), pick(4, 4, PosDEF),
		pick(5, 5, PosMID), pick(6, 6, PosMID), pick(7, 7, PosMID), pick(8, 8, PosMID), pick(9, 9, PosMID),
		pick(10, 10, PosFWD), pick(11, 11, PosFWD),
		pick(12, 12, PosGK), pick(13, 13, PosFWD), pick(14, 14, PosDEF), pick(15, 15, PosDEF),
	}
	live := liveWith(
		map[ElementID]int{1: 90, 2: 90, 3: 90, 4: 0, 5: 90, 6: 90, 7: 90, 8: 90, 9: 90, 10: 90, 11: 90},
		map[ElementID]int{4: 7}, 7,
	)

	xi := ApplyAutoSubs(picks, nil, live, stdSquad)

	if hasElement(xi, 4) {
		t.Error("blanking DEF (element 4) still in XI")
	}
	if hasElement(xi, 13) {
		t.Error("bench FWD (element 13) came on and broke the DEF minimum")
	}
	if !hasElement(xi, 14) || !subbedIn(xi, 14) {
		t.Errorf("bench DEF (element 14) not auto-subbed in: xi=%v", xiElements(xi))
	}
	if !formationOK(picksOf(xi), stdSquad) {
		t.Errorf("resulting XI is not a valid formation: xi=%v", xiElements(xi))
	}
}

func TestApplyAutoSubs_MultipleProvisionalSubs(t *testing.T) {
	picks := squad1442()
	// Slot-5 DEF and slot-9 MID both blank in finished matches.
	live := liveWith(
		map[ElementID]int{1: 90, 2: 90, 3: 90, 4: 90, 5: 0, 6: 90, 7: 90, 8: 90, 9: 0, 10: 90, 11: 90},
		map[ElementID]int{5: 7, 9: 7}, 7,
	)

	xi := ApplyAutoSubs(picks, nil, live, stdSquad)

	if hasElement(xi, 5) || hasElement(xi, 9) {
		t.Errorf("a blanking starter survived: xi=%v", xiElements(xi))
	}
	if !hasElement(xi, 13) || !hasElement(xi, 14) {
		t.Errorf("both bench outfielders should have come on: xi=%v", xiElements(xi))
	}
	if len(xi) != 11 {
		t.Fatalf("len(xi) = %d, want 11", len(xi))
	}
}

// picksOf reconstructs a []Pick from a scored XI so formationOK can re-check it.
func picksOf(xi []ScoredPlayer) []Pick {
	out := make([]Pick, len(xi))
	for i, p := range xi {
		out[i] = Pick{Element: p.Element, Position: p.Slot, Pos: p.Pos}
	}
	return out
}

func TestApplyAutoSubs_ProvisionalZeroMinuteSubIn(t *testing.T) {
	picks := squad1442()
	// No subs[] yet (GW live). Slot-5 DEF blanked and their fixture (id 7) is
	// finished-provisional -> first valid bench player (slot-13 DEF) comes on.
	live := liveWith(
		map[ElementID]int{1: 90, 2: 90, 3: 90, 4: 90, 5: 0, 6: 90, 7: 90, 8: 90, 9: 90, 10: 90, 11: 90},
		map[ElementID]int{5: 7},
		7,
	)

	xi := ApplyAutoSubs(picks, nil, live, stdSquad)

	if hasElement(xi, 5) {
		t.Error("blanking element 5 still in XI")
	}
	if !hasElement(xi, 13) || !subbedIn(xi, 13) {
		t.Errorf("element 13 not auto-subbed in: xi=%v", xiElements(xi))
	}
	if len(xi) != 11 {
		t.Fatalf("len(xi) = %d, want 11", len(xi))
	}
}
