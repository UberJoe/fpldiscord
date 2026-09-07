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
