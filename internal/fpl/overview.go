package fpl

import (
	"sort"
	"time"
)

// OverviewStatName is a resolved goalscorer-display event. The Draft live feed
// carries a fixture's per-player stat breakdown under raw keys ("goals_scored",
// "assists", …) mixed in with stats that are not part of a goalscorer display —
// bonus / bps and, since 2025/26, the defensive_contribution family
// (defensive_contribution, clearances_blocks_interceptions, recoveries,
// tackles). Overview keeps only the four events below and drops the rest.
type OverviewStatName string

const (
	OverviewGoal    OverviewStatName = "goal"
	OverviewAssist  OverviewStatName = "assist"
	OverviewOwnGoal OverviewStatName = "ownGoal"
	OverviewRedCard OverviewStatName = "redCard"
)

// goalscorerStat resolves a raw live-feed stat key to an OverviewStatName. ok is
// false for anything that does not belong in a goalscorer display — bonus, bps,
// saves, yellow cards, penalties, and the whole defensive_contribution family.
// It is an allowlist, not a blocklist, so it stays correct as the Draft API
// adds new stat families.
func goalscorerStat(raw string) (OverviewStatName, bool) {
	switch raw {
	case "goals_scored":
		return OverviewGoal, true
	case "assists":
		return OverviewAssist, true
	case "own_goals":
		return OverviewOwnGoal, true
	case "red_cards":
		return OverviewRedCard, true
	default:
		return "", false
	}
}

// OverviewStat is one goalscorer-display event within a fixture: the resolved
// event kind, the player it belongs to (web_name, accents intact, joined from
// bootstrap-static), the owning league member's first name ("" for an unowned
// player / free agent), and the count (2 for a brace).
type OverviewStat struct {
	StatName   OverviewStatName
	Element    ElementID
	PlayerName string
	OwnerName  string
	Value      int
}

// FixtureOverview is one Premier League match in a gameweek with its
// goalscorer-display events resolved — the derived view behind /overview. Team
// names are the full bootstrap-static names.
//
// The three /overview windows (Today's / Gameweek's / Live) are a pure filter
// over Started / FinishedProvisional / Today, so the command handler needs no
// wall clock: Today is computed here against the snapshot's own BuiltAt — the
// same clock ProcessedGWs uses — not time.Now().
type FixtureOverview struct {
	FixtureID           int
	TeamHome            string
	TeamAway            string
	HomeScore           int
	AwayScore           int
	Kickoff             time.Time
	Started             bool
	Finished            bool
	FinishedProvisional bool
	Today               bool // kicks off on the snapshot's build (UTC) date
	Scorers             []OverviewStat
}

// Overview returns every fixture in gameweek gw with its goalscorer-display
// events (goals, assists, own goals, red cards) resolved and joined to player
// and owner names; bonus, bps and the defensive_contribution family are left
// out. Fixtures come back in kickoff order; the events within a fixture keep the
// live feed's order (home side before away).
//
// gw is a gameweek id (0 means the current GW). The MVP snapshot carries only
// the current GW's live feed, so any other gw returns nil. The joins are built
// from the exported Snapshot fields, so the view is a pure function of a
// hand-built *Snapshot — including from the bot package at the seam-4 boundary.
func (s *Snapshot) Overview(gw int) []FixtureOverview {
	if gw == 0 {
		gw = s.CurrentGW
	}
	live, ok := s.Live[gw]
	if !ok {
		return nil
	}

	teamName := make(map[int]string, len(s.Bootstrap.Teams))
	for i := range s.Bootstrap.Teams {
		t := &s.Bootstrap.Teams[i]
		teamName[t.ID] = t.Name
	}
	playerName := make(map[ElementID]string, len(s.Bootstrap.Elements))
	for i := range s.Bootstrap.Elements {
		e := &s.Bootstrap.Elements[i]
		playerName[e.ID] = e.WebName
	}
	ownerFirst := make(map[EntryID]string, len(s.LeagueDetails.LeagueEntries))
	for i := range s.LeagueDetails.LeagueEntries {
		le := &s.LeagueDetails.LeagueEntries[i]
		ownerFirst[le.EntryID] = le.PlayerFirstName
	}
	ownerByElement := make(map[ElementID]EntryID, len(s.ElementStatus))
	for i := range s.ElementStatus {
		es := &s.ElementStatus[i]
		if es.Owner != nil {
			ownerByElement[es.Element] = *es.Owner
		}
	}

	buildDate := s.BuiltAt.UTC().Format("2006-01-02")

	out := make([]FixtureOverview, 0, len(live.Fixtures))
	for i := range live.Fixtures {
		f := &live.Fixtures[i]
		if f.Event != 0 && f.Event != gw {
			continue
		}

		ko := f.KickoffTime.Time()
		fo := FixtureOverview{
			FixtureID:           f.ID,
			TeamHome:            teamName[f.TeamH],
			TeamAway:            teamName[f.TeamA],
			HomeScore:           derefInt(f.TeamHScore),
			AwayScore:           derefInt(f.TeamAScore),
			Kickoff:             ko,
			Started:             f.Started,
			Finished:            f.Finished,
			FinishedProvisional: f.FinishedProvisional,
			Today:               !ko.IsZero() && ko.UTC().Format("2006-01-02") == buildDate,
		}

		for _, st := range f.Stats {
			name, ok := goalscorerStat(st.S)
			if !ok {
				continue
			}
			for _, side := range [][]StatValue{st.H, st.A} {
				for _, sv := range side {
					if sv.Value == 0 {
						continue
					}
					fo.Scorers = append(fo.Scorers, OverviewStat{
						StatName:   name,
						Element:    sv.Element,
						PlayerName: playerName[sv.Element],
						OwnerName:  ownerFirst[ownerByElement[sv.Element]],
						Value:      sv.Value,
					})
				}
			}
		}

		out = append(out, fo)
	}

	sort.SliceStable(out, func(i, j int) bool { return out[i].Kickoff.Before(out[j].Kickoff) })
	return out
}

// derefInt is *int -> int with a nil guard; a Draft fixture carries nil scores
// until kickoff.
func derefInt(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}
