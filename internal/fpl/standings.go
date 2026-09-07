package fpl

import "sort"

// StandingRow is one row of the classic total-points league table: the
// manager's rank, their team name, cumulative season total, and this
// gameweek's total. Derived from league/details standings[] joined to
// league_entries.
type StandingRow struct {
	Rank       int
	EntryName  string
	Total      int
	EventTotal int
}

// Standings returns the classic league table, one row per league member,
// ascending by rank.
//
// Classic only: in an h2h league the standings[] rows carry the
// matches_*/points_* family instead of Total/EventTotal, so Standings returns
// nil and the caller renders the h2h variant — not built this season.
//
// The league_entries join is built locally (keyed on the typed LeagueEntryID,
// so the two id spaces cannot be conflated) rather than through the snapshot's
// unexported index, which keeps Standings a pure function of the exported
// Snapshot fields — the seam-1 and seam-4 tests construct a Snapshot literal
// and call it directly, including from the bot package.
func (s *Snapshot) Standings() []StandingRow {
	if s.LeagueMode != ModeClassic {
		return nil
	}

	nameByLeagueEntry := make(map[LeagueEntryID]string, len(s.LeagueDetails.LeagueEntries))
	for _, le := range s.LeagueDetails.LeagueEntries {
		nameByLeagueEntry[le.ID] = le.EntryName
	}

	rows := make([]StandingRow, 0, len(s.LeagueDetails.Standings))
	for _, st := range s.LeagueDetails.Standings {
		rows = append(rows, StandingRow{
			Rank:       st.Rank,
			EntryName:  nameByLeagueEntry[st.LeagueEntry],
			Total:      st.Total,
			EventTotal: st.EventTotal,
		})
	}

	sort.Slice(rows, func(i, j int) bool { return rows[i].Rank < rows[j].Rank })
	return rows
}
