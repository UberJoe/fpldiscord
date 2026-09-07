package fpl

import (
	"testing"
	"time"
)

func TestTeamPlayers_JoinsOwnersFromLiveElementStatus(t *testing.T) {
	snap, _ := buildFromStub(t)

	rows := snap.TeamPlayers()
	if len(rows) != 4 {
		t.Fatalf("len(rows) = %d, want 4 (one per element)", len(rows))
	}

	by := map[ElementID]TeamPlayer{}
	for _, r := range rows {
		by[r.Element] = r
	}

	// element 11 Højlund: owned by entry 39880 -> Bruno / "Bruno Dos Tres", ARS, FWD.
	hojlund := by[11]
	if hojlund.WebName != "Højlund" || hojlund.Pos != PosFWD || hojlund.TeamShort != "ARS" {
		t.Errorf("Højlund row = %+v", hojlund)
	}
	if hojlund.OwnerEntryID != 39880 || hojlund.OwnerName != "Bruno" || hojlund.OwnerTeam != "Bruno Dos Tres" {
		t.Errorf("Højlund owner = %+v, want entry 39880 / Bruno / Bruno Dos Tres", hojlund)
	}

	// element 12 Muñoz: owned by entry 89 -> Ian / "Coq au Vin", CRY, DEF.
	munoz := by[12]
	if munoz.Pos != PosDEF || munoz.TeamShort != "CRY" || munoz.OwnerName != "Ian" || munoz.OwnerTeam != "Coq au Vin" {
		t.Errorf("Muñoz row = %+v", munoz)
	}

	// element 13 Raya: owner is null in element-status -> free agent.
	raya := by[13]
	if raya.OwnerEntryID != 0 || raya.OwnerName != "" || raya.OwnerTeam != "" {
		t.Errorf("Raya row = %+v, want free agent (zero owner fields)", raya)
	}
	if raya.Pos != PosGK || raya.TeamShort != "ARS" {
		t.Errorf("Raya row = %+v, want GK / ARS", raya)
	}
}

func TestTeamPlayers_FollowsBootstrapOrderAndUnknownOwnerIsFreeAgent(t *testing.T) {
	var p pieces
	owner := EntryID(500)
	stranger := EntryID(999) // not in league_entries
	p.bootstrap = Bootstrap{
		Elements: []Element{
			{ID: 3, WebName: "Gamma", ElementType: int(PosMID), Team: 1},
			{ID: 1, WebName: "Alpha", ElementType: int(PosGK), Team: 1},
			{ID: 2, WebName: "Beta", ElementType: int(PosDEF), Team: 2},
		},
		Teams: []Team{{ID: 1, ShortName: "ARS"}, {ID: 2, ShortName: "CRY"}},
	}
	p.details = LeagueDetails{
		League:        League{Scoring: "c"},
		LeagueEntries: []LeagueEntry{{ID: 1, EntryID: owner, EntryName: "Team A", PlayerFirstName: "Joe"}},
	}
	p.status = []ElementStatus{
		{Element: 1, Owner: &owner},
		{Element: 3, Owner: &stranger}, // owner id absent from the league
		// element 2 has no element-status row at all
	}
	snap := assemble(p, time.Now().UTC(), false)

	rows := snap.TeamPlayers()
	if len(rows) != 3 || rows[0].WebName != "Gamma" || rows[1].WebName != "Alpha" || rows[2].WebName != "Beta" {
		t.Fatalf("rows not in bootstrap order: %+v", rows)
	}
	if rows[1].OwnerName != "Joe" {
		t.Errorf("Alpha should be owned by Joe: %+v", rows[1])
	}
	// An owner id the league doesn't know, and a missing status row, both read
	// as free agents rather than a half-populated row.
	if rows[0].OwnerEntryID != 0 || rows[0].OwnerName != "" {
		t.Errorf("Gamma (unknown owner id) should be a free agent: %+v", rows[0])
	}
	if rows[2].OwnerEntryID != 0 || rows[2].OwnerName != "" {
		t.Errorf("Beta (no status row) should be a free agent: %+v", rows[2])
	}
}
