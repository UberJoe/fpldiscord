package fpl

import (
	"testing"
	"time"
)

// overviewSnap is a hand-built GW4 snapshot with three fixtures:
//
//   - fixture 31 (ARS v CHE): kicks off on the snapshot's build date, in play.
//     Saka scores a brace (owned by Sam), Palmer scores once (a free agent),
//     Palmer also puts through his own net and is sent off. Rice records a big
//     defensive_contribution and top bps — neither is a goalscorer event.
//   - fixture 32 (TOT v WHU): kicked off the day before, finished-provisional.
//     Son scores once (owned by Joe).
//   - fixture 33 (CHE v TOT): kicks off the next day, not started, no stats.
//
// Fixtures are listed out of kickoff order to prove the sort.
func overviewSnap() *Snapshot {
	built := time.Date(2026, 9, 12, 18, 0, 0, 0, time.UTC)
	at := func(d time.Duration) apiTime { return apiTime(built.Add(d)) }
	score := func(n int) *int { return &n }

	sam := EntryID(500)
	joe := EntryID(600)

	return &Snapshot{
		BuiltAt:   built,
		CurrentGW: 4,
		Bootstrap: Bootstrap{
			Teams: []Team{
				{ID: 1, Name: "Arsenal", ShortName: "ARS"},
				{ID: 2, Name: "Chelsea", ShortName: "CHE"},
				{ID: 3, Name: "Tottenham", ShortName: "TOT"},
				{ID: 4, Name: "West Ham", ShortName: "WHU"},
			},
			Elements: []Element{
				{ID: 11, WebName: "Saka", ElementType: 3, Team: 1},
				{ID: 12, WebName: "Palmer", ElementType: 3, Team: 2},
				{ID: 13, WebName: "Son", ElementType: 3, Team: 3},
				{ID: 14, WebName: "Rice", ElementType: 3, Team: 1},
			},
		},
		LeagueDetails: LeagueDetails{LeagueEntries: []LeagueEntry{
			{EntryID: sam, PlayerFirstName: "Sam"},
			{EntryID: joe, PlayerFirstName: "Joe"},
		}},
		ElementStatus: []ElementStatus{
			{Element: 11, Owner: &sam},
			{Element: 12, Owner: nil},
			{Element: 13, Owner: &joe},
			{Element: 14, Owner: &sam},
		},
		Live: map[int]LiveGW{
			4: {Fixtures: []LiveFixture{
				{
					ID: 33, Event: 4, TeamH: 2, TeamA: 3,
					KickoffTime: at(24 * time.Hour), Started: false,
				},
				{
					ID: 31, Event: 4, TeamH: 1, TeamA: 2,
					TeamHScore: score(2), TeamAScore: score(1),
					KickoffTime: at(-3 * time.Hour), Started: true,
					Stats: []FixtureStat{
						{S: "goals_scored", H: []StatValue{{Element: 11, Value: 2}}, A: []StatValue{{Element: 12, Value: 1}}},
						{S: "own_goals", A: []StatValue{{Element: 12, Value: 1}}},
						{S: "red_cards", A: []StatValue{{Element: 12, Value: 1}}},
						{S: "defensive_contribution", H: []StatValue{{Element: 14, Value: 14}}},
						{S: "clearances_blocks_interceptions", H: []StatValue{{Element: 14, Value: 9}}},
						{S: "recoveries", H: []StatValue{{Element: 14, Value: 7}}},
						{S: "tackles", H: []StatValue{{Element: 14, Value: 5}}},
						{S: "bps", H: []StatValue{{Element: 14, Value: 41}}},
						{S: "bonus", H: []StatValue{{Element: 14, Value: 3}}},
						{S: "saves", A: []StatValue{{Element: 12, Value: 4}}},
						{S: "yellow_cards", H: []StatValue{{Element: 11, Value: 1}}},
					},
				},
				{
					ID: 32, Event: 4, TeamH: 3, TeamA: 4,
					TeamHScore: score(1), TeamAScore: score(0),
					KickoffTime: at(-27 * time.Hour), Started: true,
					Finished: true, FinishedProvisional: true,
					Stats: []FixtureStat{
						{S: "goals_scored", H: []StatValue{{Element: 13, Value: 1}}},
					},
				},
			}},
		},
	}
}

func TestOverview_FixturesSortedByKickoffWithModeFlags(t *testing.T) {
	fx := overviewSnap().Overview(4)

	if len(fx) != 3 {
		t.Fatalf("len(fx) = %d, want 3", len(fx))
	}
	// Kickoff order: 32 (day before), 31 (same day), 33 (next day).
	if fx[0].FixtureID != 32 || fx[1].FixtureID != 31 || fx[2].FixtureID != 33 {
		t.Fatalf("fixture order = %d/%d/%d, want 32/31/33", fx[0].FixtureID, fx[1].FixtureID, fx[2].FixtureID)
	}

	live := fx[1]
	if live.TeamHome != "Arsenal" || live.TeamAway != "Chelsea" {
		t.Errorf("fixture 31 teams = %q v %q, want Arsenal v Chelsea", live.TeamHome, live.TeamAway)
	}
	if live.HomeScore != 2 || live.AwayScore != 1 {
		t.Errorf("fixture 31 score = %d-%d, want 2-1", live.HomeScore, live.AwayScore)
	}
	if !live.Started || live.FinishedProvisional || !live.Today {
		t.Errorf("fixture 31 flags = started %v finishedProv %v today %v, want true/false/true",
			live.Started, live.FinishedProvisional, live.Today)
	}

	done := fx[0]
	if !done.Finished || !done.FinishedProvisional || done.Today {
		t.Errorf("fixture 32 flags = finished %v finishedProv %v today %v, want true/true/false",
			done.Finished, done.FinishedProvisional, done.Today)
	}

	up := fx[2]
	if up.Started || up.Today || len(up.Scorers) != 0 {
		t.Errorf("fixture 33 = started %v today %v scorers %d, want false/false/0",
			up.Started, up.Today, len(up.Scorers))
	}
}

func TestOverview_KeepsOnlyGoalscorerEventsWithOwners(t *testing.T) {
	fx := overviewSnap().Overview(0) // 0 -> current GW (4)

	var live FixtureOverview
	for _, f := range fx {
		if f.FixtureID == 31 {
			live = f
		}
	}

	// Saka brace + Palmer goal + Palmer OG + Palmer red = 4 events. Everything
	// else in the fixture (defensive_contribution family, bps, bonus, saves,
	// yellow_cards) is not a goalscorer event and must be dropped.
	if len(live.Scorers) != 4 {
		t.Fatalf("scorers = %+v, want 4 goalscorer events", live.Scorers)
	}
	for _, sc := range live.Scorers {
		switch sc.StatName {
		case OverviewGoal, OverviewAssist, OverviewOwnGoal, OverviewRedCard:
		default:
			t.Errorf("scorer %+v has a non-goalscorer StatName", sc)
		}
		if sc.Element == 14 {
			t.Errorf("Rice (defensive_contribution / bps only) leaked into the goalscorer list: %+v", sc)
		}
	}

	find := func(kind OverviewStatName, el ElementID) (OverviewStat, bool) {
		for _, sc := range live.Scorers {
			if sc.StatName == kind && sc.Element == el {
				return sc, true
			}
		}
		return OverviewStat{}, false
	}

	if g, ok := find(OverviewGoal, 11); !ok || g.PlayerName != "Saka" || g.Value != 2 || g.OwnerName != "Sam" {
		t.Errorf("Saka goal event = %+v (found %v), want Saka x2 owned by Sam", g, ok)
	}
	if g, ok := find(OverviewGoal, 12); !ok || g.PlayerName != "Palmer" || g.Value != 1 || g.OwnerName != "" {
		t.Errorf("Palmer goal event = %+v (found %v), want Palmer x1, no owner", g, ok)
	}
	if og, ok := find(OverviewOwnGoal, 12); !ok || og.PlayerName != "Palmer" || og.OwnerName != "" {
		t.Errorf("own-goal event = %+v (found %v), want Palmer, no owner (free agent)", og, ok)
	}
	if rc, ok := find(OverviewRedCard, 12); !ok || rc.Value != 1 {
		t.Errorf("red-card event = %+v (found %v), want Palmer x1", rc, ok)
	}

	// The away-side goal keeps feed order after the home-side brace.
	if live.Scorers[0].Element != 11 || live.Scorers[1].Element != 12 {
		t.Errorf("goal order = %d then %d, want home (11) before away (12)",
			live.Scorers[0].Element, live.Scorers[1].Element)
	}
}

func TestOverview_DefensiveOnlyFixtureHasNoScorers(t *testing.T) {
	snap := &Snapshot{
		CurrentGW: 4,
		Bootstrap: Bootstrap{
			Teams:    []Team{{ID: 1, Name: "Arsenal"}, {ID: 2, Name: "Chelsea"}},
			Elements: []Element{{ID: 14, WebName: "Rice", Team: 1}},
		},
		Live: map[int]LiveGW{4: {Fixtures: []LiveFixture{{
			ID: 31, Event: 4, TeamH: 1, TeamA: 2, Started: true,
			Stats: []FixtureStat{
				{S: "defensive_contribution", H: []StatValue{{Element: 14, Value: 20}}},
				{S: "bps", H: []StatValue{{Element: 14, Value: 44}}},
				{S: "bonus", H: []StatValue{{Element: 14, Value: 3}}},
			},
		}}}},
	}

	fx := snap.Overview(4)
	if len(fx) != 1 {
		t.Fatalf("len(fx) = %d, want 1", len(fx))
	}
	if len(fx[0].Scorers) != 0 {
		t.Errorf("scorers = %+v, want none (defensive stats only)", fx[0].Scorers)
	}
}

func TestOverview_UnknownGameweekIsNil(t *testing.T) {
	if fx := overviewSnap().Overview(99); fx != nil {
		t.Fatalf("Overview(99) = %v, want nil (snapshot carries only GW4)", fx)
	}
}
