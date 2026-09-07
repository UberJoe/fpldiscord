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

// liveStandingsSnapshot: GW5, two classic managers. M1 (LE 1 / entry 500) is
// official rank 1 on 100 season points with no live points this GW; M2 (LE 2 /
// entry 600) is rank 2 on 90 but scores 20 live (element 601), so live order is
// M2 then M1.
func liveStandingsSnapshot() *Snapshot {
	var p pieces
	p.currentGW = 5
	p.game = Game{CurrentEvent: 5}

	els := make([]Element, 0, 11)
	live := LiveGW{
		Fixtures: []LiveFixture{{ID: 1, Started: true, FinishedProvisional: true}},
		Elements: map[ElementID]LiveElement{
			601: {Stats: LiveStats{TotalPoints: 20, Minutes: 90}},
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
			{Rank: 1, LeagueEntry: 1, Total: 100, EventTotal: 10},
			{Rank: 2, LeagueEntry: 2, Total: 90, EventTotal: 40},
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
	if first.TotalPoints != 90 || first.LiveGwPoints != 20 || first.LivePoints != 110 {
		t.Errorf("rows[0] points = %d / %d / %d, want 90 / 20 / 110", first.TotalPoints, first.LiveGwPoints, first.LivePoints)
	}

	if second.EntryID != 500 || second.LiveRank != 2 || second.Arrow != -1 {
		t.Errorf("rows[1] = %+v, want entry 500 live rank 2 arrow -1", second)
	}
	if second.LiveGwPoints != 0 || second.LivePoints != 100 {
		t.Errorf("rows[1] points = gw %d live %d, want 0 / 100", second.LiveGwPoints, second.LivePoints)
	}
}

func TestLiveStandings_NilInH2HMode(t *testing.T) {
	snap := liveStandingsSnapshot()
	snap.LeagueMode = ModeH2H
	if got := snap.LiveStandings(); got != nil {
		t.Errorf("LiveStandings() in h2h = %+v, want nil", got)
	}
}
