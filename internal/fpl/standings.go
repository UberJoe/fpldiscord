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

// LiveStandingRow is one row of the live-ordered classic table behind the web
// Standings view: the manager's frozen season figures joined to this
// gameweek's live points (auto-subs applied), plus the live rank and the
// movement arrow. It carries EntryID because the web drill-down route keys on
// it; the classic /standings Discord command uses StandingRow instead.
type LiveStandingRow struct {
	EntryID      EntryID
	OwnerName    string
	EntryName    string
	OfficialRank int // Draft standings rank; not used for Arrow, kept per ADR 0001 for the client's later use
	LastRank     int // Draft last_rank: position in last week's final standings; 0 = no previous position
	LiveRank     int // joint 1..N, ties share a rank and the next rank skips
	Arrow        int // LastRank - LiveRank when LastRank != 0, else 0; positive = climbed since last week
	TotalPoints  int // frozen season total, excludes the live GW
	LiveGwPoints int // ManagerScore(EntryID, CurrentGW).Total
	LivePoints   int // TotalPoints + LiveGwPoints

	rankSort int // Draft rank_sort: strict tiebroken order, settles row order within a live-points tie
}

// LiveStandings returns the classic league table re-sorted by live points
// (frozen season total plus this gameweek so far), with a joint LiveRank and
// the week-over-week movement arrow assigned. All ordering and rank maths
// happen here so the web client only renders array order.
//
// Ranking matches the official Draft table: managers level on live points share
// a LiveRank and the next rank skips (1, 2, 2, 4). Row order within a tie is
// settled by the Draft strict-sort field (rank_sort), not the order the API
// returned rows in. The arrow is movement since last week's final standings —
// LastRank compared to LiveRank — so it stays put through a gameweek in
// progress and only reflects real net movement; a manager with no previous
// position (LastRank == 0) shows a flat arrow.
//
// Classic only: nil in h2h mode, matching Standings. A manager whose gameweek
// score cannot be computed yet (no picks submitted, or no live feed) counts as
// 0 live points rather than dropping out of the table.
func (s *Snapshot) LiveStandings() []LiveStandingRow {
	if s.LeagueMode != ModeClassic {
		return nil
	}

	entryByLeague := make(map[LeagueEntryID]LeagueEntry, len(s.LeagueDetails.LeagueEntries))
	for _, le := range s.LeagueDetails.LeagueEntries {
		entryByLeague[le.ID] = le
	}

	rows := make([]LiveStandingRow, 0, len(s.LeagueDetails.Standings))
	for _, st := range s.LeagueDetails.Standings {
		le := entryByLeague[st.LeagueEntry]

		liveGw := 0
		if ms, err := s.ManagerScore(le.EntryID, s.CurrentGW); err == nil {
			liveGw = ms.Total
		}

		// Draft's standings[].Total is the cumulative score *including* this
		// gameweek's EventTotal (it keeps updating through the GW and stays put
		// once it finishes, until the league rolls to the next GW). The live
		// table wants the frozen base without the current GW, so we can add our
		// own live figure back on top — adding to Total directly counts the
		// gameweek twice and reorders the table.
		frozen := st.Total - st.EventTotal

		rows = append(rows, LiveStandingRow{
			EntryID:      le.EntryID,
			OwnerName:    le.PlayerFirstName,
			EntryName:    le.EntryName,
			OfficialRank: st.Rank,
			LastRank:     st.LastRank,
			TotalPoints:  frozen,
			LiveGwPoints: liveGw,
			LivePoints:   frozen + liveGw,
			rankSort:     st.RankSort,
		})
	}

	// Live order: most live points first. A live-points tie is settled by the
	// Draft strict-sort field so row order is deterministic and does not depend
	// on the order the API returned the standings rows in. Stable so that, in
	// the unlikely event rank_sort is ever absent, the fallback is the Draft
	// slice order rather than something non-deterministic.
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].LivePoints != rows[j].LivePoints {
			return rows[i].LivePoints > rows[j].LivePoints
		}
		return rows[i].rankSort < rows[j].rankSort
	})

	// Joint live rank: managers level on live points share a rank and the next
	// rank skips (1, 2, 2, 4), matching the official Draft table. The arrow is
	// movement since last week's final standings (LastRank -> LiveRank);
	// LastRank == 0 (no previous position) shows flat.
	for i := range rows {
		rows[i].LiveRank = i + 1
		if i > 0 && rows[i].LivePoints == rows[i-1].LivePoints {
			rows[i].LiveRank = rows[i-1].LiveRank
		}
		if rows[i].LastRank != 0 {
			rows[i].Arrow = rows[i].LastRank - rows[i].LiveRank
		}
	}
	return rows
}
