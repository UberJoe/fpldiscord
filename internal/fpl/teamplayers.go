package fpl

// TeamPlayer is one Draft player joined to their current owner. Ownership comes
// from element-status (owner is an EntryID) joined to league_entries, so it
// reflects the live roster the moment the GW20 mid-season redraft lands. It
// backs /owner (look a player up) and /teamlist (filter by owner).
type TeamPlayer struct {
	Element   ElementID
	WebName   string // display form, accents intact (e.g. "Højlund")
	Pos       Pos
	TeamShort string // the player's PL club short name, e.g. "ARS"

	OwnerEntryID EntryID // the owning manager's global entry id; 0 for a free agent
	OwnerName    string  // owner's player_first_name; "" for a free agent
	OwnerTeam    string  // owner's entry_name; "" for a free agent
}

// TeamPlayers returns one row per Draft player in bootstrap-static order, each
// joined to its current owner. A player with no element-status row, a nil
// owner, or an owner id absent from this league comes back as a free agent
// (zero OwnerEntryID, empty owner fields) rather than a half-populated row.
//
// The joins are built locally from the exported Snapshot fields (element-status,
// league_entries, bootstrap-static) rather than the unexported indexes, keeping
// TeamPlayers a pure function testable from a hand-built *Snapshot, including
// from sibling packages at the seam-4 boundary. Slice fields are ranged by
// index into a pointer, matching ManagerSquad, so the large Element/Team rows
// are not copied per iteration.
func (s *Snapshot) TeamPlayers() []TeamPlayer {
	ownerByElement := make(map[ElementID]EntryID, len(s.ElementStatus))
	for i := range s.ElementStatus {
		es := &s.ElementStatus[i]
		if es.Owner != nil {
			ownerByElement[es.Element] = *es.Owner
		}
	}

	type ownerInfo struct{ first, team string }
	infoByEntry := make(map[EntryID]ownerInfo, len(s.LeagueDetails.LeagueEntries))
	for i := range s.LeagueDetails.LeagueEntries {
		le := &s.LeagueDetails.LeagueEntries[i]
		infoByEntry[le.EntryID] = ownerInfo{le.PlayerFirstName, le.EntryName}
	}

	shortByTeam := make(map[int]string, len(s.Bootstrap.Teams))
	for i := range s.Bootstrap.Teams {
		t := &s.Bootstrap.Teams[i]
		shortByTeam[t.ID] = t.ShortName
	}

	out := make([]TeamPlayer, 0, len(s.Bootstrap.Elements))
	for i := range s.Bootstrap.Elements {
		e := &s.Bootstrap.Elements[i]
		tp := TeamPlayer{
			Element:   e.ID,
			WebName:   e.WebName,
			Pos:       Pos(e.ElementType),
			TeamShort: shortByTeam[e.Team],
		}
		if entry, ok := ownerByElement[e.ID]; ok {
			if info, ok := infoByEntry[entry]; ok {
				tp.OwnerEntryID = entry
				tp.OwnerName = info.first
				tp.OwnerTeam = info.team
			}
		}
		out = append(out, tp)
	}
	return out
}
