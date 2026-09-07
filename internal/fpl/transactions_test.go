package fpl

import "testing"

// waiverSnap is a hand-built snapshot with a processed waiver round in GW4: two
// managers contest element 20 (Salah), Sam wins on priority 1 and Joe is
// denied-other; plus a free-agent pickup and a trade that must not appear.
func waiverSnap() *Snapshot {
	return &Snapshot{
		Bootstrap: Bootstrap{Elements: []Element{
			{ID: 20, WebName: "Salah"},
			{ID: 21, WebName: "Núñez"},
			{ID: 22, WebName: "Højlund"},
			{ID: 23, WebName: "Watkins"},
		}},
		LeagueDetails: LeagueDetails{LeagueEntries: []LeagueEntry{
			{EntryID: 500, PlayerFirstName: "Sam"},
			{EntryID: 600, PlayerFirstName: "Joe"},
		}},
		Transactions: []Transaction{
			{Entry: 600, Event: 4, ElementIn: 20, ElementOut: 21, Kind: "w", Result: "do", Priority: 4, Index: 2},
			{Entry: 500, Event: 4, ElementIn: 20, ElementOut: 22, Kind: "w", Result: "a", Priority: 1, Index: 1},
			{Entry: 500, Event: 4, ElementIn: 23, ElementOut: 0, Kind: "f", Result: "a", Priority: 0, Index: 3},
			{Entry: 600, Event: 4, ElementIn: 21, ElementOut: 23, Kind: "t", Result: "a", Priority: 0, Index: 4},
			{Entry: 500, Event: 3, ElementIn: 22, ElementOut: 20, Kind: "w", Result: "a", Priority: 2, Index: 1},
		},
	}
}

func TestLeagueTransactions_ResolvesCodesSortsByIndexExcludesTrades(t *testing.T) {
	rows := waiverSnap().LeagueTransactions(4)

	if len(rows) != 3 {
		t.Fatalf("len(rows) = %d, want 3 (trade excluded, GW3 filtered out)", len(rows))
	}

	// Sorted by Index: accepted waiver (1), denied waiver (2), free agent (3).
	if rows[0].Index != 1 || rows[1].Index != 2 || rows[2].Index != 3 {
		t.Fatalf("row indexes = %d/%d/%d, want 1/2/3", rows[0].Index, rows[1].Index, rows[2].Index)
	}

	win := rows[0]
	if win.OwnerName != "Sam" || win.In != "Salah" || win.Out != "Højlund" {
		t.Errorf("winning row = %+v", win)
	}
	if win.ElementIn != 20 {
		t.Errorf("winning row ElementIn = %d, want 20 (stable grouping key)", win.ElementIn)
	}
	if win.Type != WaiverTypeWaiver || win.Status != WaiverStatusAccepted {
		t.Errorf("winning row type/status = %s/%s, want waiver/accepted", win.Type, win.Status)
	}
	if win.Priority != 1 {
		t.Errorf("winning row priority = %d, want 1", win.Priority)
	}

	lost := rows[1]
	if lost.OwnerName != "Joe" || lost.Status != WaiverStatusFailed || lost.Priority != 4 {
		t.Errorf("losing row = %+v, want Joe / failed / priority 4", lost)
	}
	if lost.In != "Salah" {
		t.Errorf("losing row In = %q, want Salah (same contested player)", lost.In)
	}

	fa := rows[2]
	if fa.Type != WaiverTypeFreeAgent || fa.In != "Watkins" || fa.Out != "" {
		t.Errorf("free-agent row = %+v, want freeAgent / Watkins / no drop", fa)
	}
}

func TestLeagueTransactions_GWZeroReturnsAllSortedByGWThenIndex(t *testing.T) {
	rows := waiverSnap().LeagueTransactions(0)

	// Every non-trade row across both gameweeks: 1 in GW3, 3 in GW4.
	if len(rows) != 4 {
		t.Fatalf("len(rows) = %d, want 4", len(rows))
	}
	if rows[0].GW != 3 {
		t.Errorf("rows[0].GW = %d, want 3 (earliest gameweek first)", rows[0].GW)
	}
	for i := 1; i < len(rows); i++ {
		if rows[i].GW != 4 {
			t.Errorf("rows[%d].GW = %d, want 4", i, rows[i].GW)
		}
	}
}

func TestLeagueTransactions_UnknownGWIsEmpty(t *testing.T) {
	if rows := waiverSnap().LeagueTransactions(99); len(rows) != 0 {
		t.Fatalf("LeagueTransactions(99) = %v, want empty", rows)
	}
}
