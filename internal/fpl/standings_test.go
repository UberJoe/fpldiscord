package fpl

import (
	"testing"
	"time"
)

func TestStandings_ClassicRowsFromLeagueDetails(t *testing.T) {
	snap, _ := buildFromStub(t)

	rows := snap.Standings()
	if len(rows) != 2 {
		t.Fatalf("len(rows) = %d, want 2", len(rows))
	}
	// standings[] fixture is Ian (rank 1) then Bruno (rank 2), ascending by rank.
	if rows[0].Rank != 1 || rows[1].Rank != 2 {
		t.Fatalf("ranks = %d, %d; want 1, 2", rows[0].Rank, rows[1].Rank)
	}

	ian := rows[0]
	if ian.EntryName != "Coq au Vin" {
		t.Errorf("row[0].EntryName = %q, want Coq au Vin", ian.EntryName)
	}
	if ian.Total != 210 || ian.EventTotal != 55 {
		t.Errorf("row[0] totals = %d/%d, want 210/55", ian.Total, ian.EventTotal)
	}

	bruno := rows[1]
	if bruno.EntryName != "Bruno Dos Tres" {
		t.Errorf("row[1].EntryName = %q, want Bruno Dos Tres", bruno.EntryName)
	}
	if bruno.Total != 180 || bruno.EventTotal != 40 {
		t.Errorf("row[1] totals = %d/%d, want 180/40", bruno.Total, bruno.EventTotal)
	}
}

func TestStandings_SortedByRankClassicOnly(t *testing.T) {
	base := func(scoring string, standings []Standing) *Snapshot {
		var p pieces
		p.details = LeagueDetails{
			League: League{Name: "L", Scoring: scoring},
			LeagueEntries: []LeagueEntry{
				{ID: 1, EntryName: "Alpha"},
				{ID: 2, EntryName: "Beta"},
			},
			Standings: standings,
		}
		return assemble(p, time.Now().UTC(), false)
	}

	classic := base("c", []Standing{
		{Rank: 2, LeagueEntry: 2, Total: 50, EventTotal: 5},
		{Rank: 1, LeagueEntry: 1, Total: 90, EventTotal: 8},
	})
	rows := classic.Standings()
	if len(rows) != 2 || rows[0].Rank != 1 || rows[0].EntryName != "Alpha" {
		t.Fatalf("classic standings not sorted by rank: %+v", rows)
	}
	if rows[1].Rank != 2 || rows[1].EntryName != "Beta" {
		t.Errorf("row[1] = %+v, want Beta at rank 2", rows[1])
	}

	// h2h standings[] rows carry match/points-for data, not Total/EventTotal —
	// this ticket does not build that variant, so Standings() returns nil.
	h2h := base("h", []Standing{{Rank: 1, LeagueEntry: 1}})
	if got := h2h.Standings(); got != nil {
		t.Errorf("h2h Standings() = %+v, want nil", got)
	}
}

// liveStandingsSnapshot: GW5, two classic managers. Draft standings[].Total
// already includes this GW's EventTotal, so the frozen base is Total-EventTotal.
// M1 (LE 1 / entry 500) is official rank 1 — frozen 110, no live points this GW.
// M2 (LE 2 / entry 600) is rank 2 — frozen 95 — but scores 25 live (element
// 601) for a live 120, so live order is M2 then M1.
func liveStandingsSnapshot() *Snapshot {
	var p pieces
	p.currentGW = 5
	p.game = Game{CurrentEvent: 5}

	els := make([]Element, 0, 11)
	live := LiveGW{
		Fixtures: []LiveFixture{{ID: 1, Started: true, FinishedProvisional: true}},
		Elements: map[ElementID]LiveElement{
			601: {Stats: LiveStats{TotalPoints: 25, Minutes: 90}},
		},
	}
	for i := 0; i < 11; i++ {
		els = append(els, Element{ID: ElementID(500 + i), ElementType: 3})
		els = append(els, Element{ID: ElementID(600 + i), ElementType: 3})
	}
	p.bootstrap = Bootstrap{Elements: els}
	p.live = live

	p.details = LeagueDetails{
		League: League{Name: "L", Scoring: "c"},
		LeagueEntries: []LeagueEntry{
			{ID: 1, EntryID: 500, EntryName: "Team A", PlayerFirstName: "Joe"},
			{ID: 2, EntryID: 600, EntryName: "Team B", PlayerFirstName: "Sam"},
		},
		Standings: []Standing{
			{Rank: 1, LeagueEntry: 1, Total: 150, EventTotal: 40},
			{Rank: 2, LeagueEntry: 2, Total: 130, EventTotal: 35},
		},
	}
	p.entries = map[EntryID]EntryEvent{
		500: {Picks: xiSlots(500)},
		600: {Picks: xiSlots(600)},
	}
	return assemble(p, time.Now().UTC(), false)
}

// xiSlots is a submitted XI (positions 1..11) starting at startElem.
func xiSlots(startElem int) []Pick {
	ps := make([]Pick, 0, 11)
	for i := 0; i < 11; i++ {
		ps = append(ps, Pick{Element: ElementID(startElem + i), Position: i + 1, Multiplier: 1})
	}
	return ps
}

func TestLiveStandings_SortedByLivePointsWithRanksAndArrows(t *testing.T) {
	rows := liveStandingsSnapshot().LiveStandings()
	if len(rows) != 2 {
		t.Fatalf("len(rows) = %d, want 2", len(rows))
	}

	// M2 (entry 600) leads on live points despite official rank 2.
	first, second := rows[0], rows[1]
	if first.EntryID != 600 || first.OwnerName != "Sam" || first.EntryName != "Team B" {
		t.Errorf("rows[0] = %+v, want entry 600 / Sam / Team B", first)
	}
	if first.OfficialRank != 2 || first.LiveRank != 1 || first.Arrow != 1 {
		t.Errorf("rows[0] ranks = official %d live %d arrow %d, want 2 / 1 / 1", first.OfficialRank, first.LiveRank, first.Arrow)
	}
	if first.TotalPoints != 95 || first.LiveGwPoints != 25 || first.LivePoints != 120 {
		t.Errorf("rows[0] points = %d / %d / %d, want 95 / 25 / 120", first.TotalPoints, first.LiveGwPoints, first.LivePoints)
	}

	if second.EntryID != 500 || second.LiveRank != 2 || second.Arrow != -1 {
		t.Errorf("rows[1] = %+v, want entry 500 live rank 2 arrow -1", second)
	}
	if second.TotalPoints != 110 || second.LiveGwPoints != 0 || second.LivePoints != 110 {
		t.Errorf("rows[1] points = tot %d gw %d live %d, want 110 / 0 / 110", second.TotalPoints, second.LiveGwPoints, second.LivePoints)
	}
}

// liveStandingsFinishedGWSnapshot: GW3, finished but not yet rolled over — the
// state the live league sits in between a gameweek ending and its waivers
// running. The Draft standings[] Total already folds in this gameweek's
// EventTotal (verified against league 64: rank-1 Total 165 = frozen 111 +
// EventTotal 54). Three managers, each with a one-man live line worth exactly
// their EventTotal, so a correct LiveStandings reproduces the official order
// (frozen base + liveGw == Total once the GW is in) instead of re-sorting on
// Total + liveGw, which counts the gameweek twice.
func liveStandingsFinishedGWSnapshot() *Snapshot {
	var p pieces
	p.currentGW = 3
	p.game = Game{CurrentEvent: 3, CurrentEventFinished: true}

	els := make([]Element, 0, 33)
	for _, base := range []int{300, 400, 500} {
		for i := 0; i < 11; i++ {
			els = append(els, Element{ID: ElementID(base + i), ElementType: 3})
		}
	}
	p.bootstrap = Bootstrap{Elements: els}
	p.live = LiveGW{
		Fixtures: []LiveFixture{{ID: 1, Started: true, Finished: true, FinishedProvisional: true}},
		Elements: map[ElementID]LiveElement{
			300: {Stats: LiveStats{TotalPoints: 54, Minutes: 90}},
			400: {Stats: LiveStats{TotalPoints: 40, Minutes: 90}},
			500: {Stats: LiveStats{TotalPoints: 57, Minutes: 90}},
		},
	}
	p.details = LeagueDetails{
		League: League{Name: "L", Scoring: "c"},
		LeagueEntries: []LeagueEntry{
			{ID: 1, EntryID: 301, EntryName: "Jack XI", PlayerFirstName: "Jack"},
			{ID: 2, EntryID: 401, EntryName: "Gabriel XI", PlayerFirstName: "Gabriel"},
			{ID: 3, EntryID: 501, EntryName: "Joe XI", PlayerFirstName: "Joe"},
		},
		Standings: []Standing{
			{Rank: 1, LeagueEntry: 1, Total: 165, EventTotal: 54},
			{Rank: 2, LeagueEntry: 2, Total: 145, EventTotal: 40},
			{Rank: 3, LeagueEntry: 3, Total: 144, EventTotal: 57},
		},
	}
	p.entries = map[EntryID]EntryEvent{
		301: {Picks: xiSlots(300)},
		401: {Picks: xiSlots(400)},
		501: {Picks: xiSlots(500)},
	}
	return assemble(p, time.Now().UTC(), false)
}

func TestLiveStandings_FrozenBaseExcludesCurrentGW(t *testing.T) {
	rows := liveStandingsFinishedGWSnapshot().LiveStandings()
	if len(rows) != 3 {
		t.Fatalf("len(rows) = %d, want 3", len(rows))
	}

	// GW3 is in the books: livePoints must equal each manager's season Total
	// (frozen base + this gameweek), so live order == official order and every
	// arrow is flat. Sorting on Total + liveGw instead double-counts the GW and
	// floats the big GW scores (Joe, +57) above steadier managers (Gabriel).
	want := []struct {
		entryID    EntryID
		livePoints int
	}{
		{301, 165},
		{401, 145},
		{501, 144},
	}
	for i, w := range want {
		if rows[i].EntryID != w.entryID {
			t.Errorf("rows[%d].EntryID = %d, want %d (live order scrambled by double-counting the GW)", i, rows[i].EntryID, w.entryID)
		}
		if rows[i].LivePoints != w.livePoints {
			t.Errorf("rows[%d].LivePoints = %d, want %d", i, rows[i].LivePoints, w.livePoints)
		}
		if rows[i].LiveRank != rows[i].OfficialRank {
			t.Errorf("rows[%d] LiveRank %d != OfficialRank %d", i, rows[i].LiveRank, rows[i].OfficialRank)
		}
		if rows[i].Arrow != 0 {
			t.Errorf("rows[%d].Arrow = %d, want 0", i, rows[i].Arrow)
		}
	}
}

func TestLiveStandings_NilInH2HMode(t *testing.T) {
	snap := liveStandingsSnapshot()
	snap.LeagueMode = ModeH2H
	if got := snap.LiveStandings(); got != nil {
		t.Errorf("LiveStandings() in h2h = %+v, want nil", got)
	}
}
