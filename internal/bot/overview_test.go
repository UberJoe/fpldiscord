package bot

import (
	"strings"
	"testing"
	"time"

	"github.com/UberJoe/fpldiscord/internal/fpl"
)

// overviewSnap is a GW4 snapshot with a finished fixture, an in-play fixture,
// and an upcoming fixture. Saka (owned by Sam) scores in the live game; Rice
// only records a defensive_contribution there and must never appear in the
// goalscorer list.
//
// fpl.LiveFixture.KickoffTime is an unexported apiTime, so these fixtures carry
// zero kickoffs — which the fpl layer never flags as Today. The Today-mode test
// relies on that (an empty window, reported as such).
func overviewSnap() *fpl.Snapshot {
	built := time.Date(2026, 9, 12, 18, 0, 0, 0, time.UTC)
	score := func(n int) *int { return &n }
	sam := fpl.EntryID(500)

	return &fpl.Snapshot{
		BuiltAt:    built,
		LeagueName: "Coq au Ian",
		CurrentGW:  4,
		Bootstrap: fpl.Bootstrap{
			Teams: []fpl.Team{
				{ID: 1, Name: "Arsenal"}, {ID: 2, Name: "Chelsea"},
				{ID: 3, Name: "Tottenham"}, {ID: 4, Name: "West Ham"},
			},
			Elements: []fpl.Element{
				{ID: 11, WebName: "Saka", Team: 1},
				{ID: 14, WebName: "Rice", Team: 1},
				{ID: 21, WebName: "Palmer", Team: 2},
			},
		},
		LeagueDetails: fpl.LeagueDetails{LeagueEntries: []fpl.LeagueEntry{
			{EntryID: sam, PlayerFirstName: "Sam"},
		}},
		ElementStatus: []fpl.ElementStatus{{Element: 11, Owner: &sam}},
		Live: map[int]fpl.LiveGW{4: {Fixtures: []fpl.LiveFixture{
			{
				ID: 32, Event: 4, TeamH: 3, TeamA: 4,
				TeamHScore: score(1), TeamAScore: score(0),
				Started: true, Finished: true, FinishedProvisional: true,
			},
			{
				ID: 31, Event: 4, TeamH: 1, TeamA: 2,
				TeamHScore: score(2), TeamAScore: score(1),
				Started: true,
				Stats: []fpl.FixtureStat{
					{S: "goals_scored", H: []fpl.StatValue{{Element: 11, Value: 1}}, A: []fpl.StatValue{{Element: 21, Value: 1}}},
					{S: "defensive_contribution", H: []fpl.StatValue{{Element: 14, Value: 15}}},
					{S: "bps", H: []fpl.StatValue{{Element: 14, Value: 40}}},
				},
			},
			{ID: 33, Event: 4, TeamH: 2, TeamA: 3},
		}}},
	}
}

func TestHandleOverview_DefaultModeShowsWholeGameweek(t *testing.T) {
	r := &recordingResponder{}
	if err := handleOverview(&cmdInput{snap: overviewSnap(), resp: r}); err != nil {
		t.Fatalf("handleOverview: %v", err)
	}
	if len(r.messages) != 1 {
		t.Fatalf("messages = %v, want one", r.messages)
	}
	msg := r.messages[0]

	for _, want := range []string{"Coq au Ian", "GW4", "gameweek", "Saka", "Sam"} {
		if !strings.Contains(msg, want) {
			t.Errorf("overview output missing %q:\n%s", want, msg)
		}
	}
	// Rice only has a defensive_contribution — it must not be rendered.
	if strings.Contains(msg, "Rice") {
		t.Errorf("defensive_contribution player leaked into the goalscorer display:\n%s", msg)
	}
	// All three fixtures present in the default (whole-gameweek) window.
	for _, want := range []string{"Tottenham 1 - 0 West Ham", "Arsenal 2 - 1 Chelsea", "Chelsea vs Tottenham"} {
		if !strings.Contains(msg, want) {
			t.Errorf("whole-gameweek window missing fixture line %q:\n%s", want, msg)
		}
	}
}

func TestHandleOverview_LiveModeDropsFinishedAndUpcoming(t *testing.T) {
	r := &recordingResponder{}
	in := &cmdInput{snap: overviewSnap(), opts: cmdOptions{"mode": "live"}, resp: r}
	if err := handleOverview(in); err != nil {
		t.Fatalf("handleOverview: %v", err)
	}
	msg := r.messages[0]

	if !strings.Contains(msg, "Arsenal 2 - 1 Chelsea") {
		t.Errorf("live window missing the in-play fixture:\n%s", msg)
	}
	if strings.Contains(msg, "West Ham") {
		t.Errorf("live window still shows the finished fixture:\n%s", msg)
	}
	if strings.Contains(msg, "Chelsea vs Tottenham") {
		t.Errorf("live window still shows the upcoming fixture:\n%s", msg)
	}
}

func TestHandleOverview_TodayModeEmptyIsExplained(t *testing.T) {
	// No fixture in overviewSnap carries a kickoff on the build date, so Today
	// mode comes back empty — the handler must say so rather than erroring or
	// falling back to the full list.
	r := &recordingResponder{}
	in := &cmdInput{snap: overviewSnap(), opts: cmdOptions{"mode": "today"}, resp: r}
	if err := handleOverview(in); err != nil {
		t.Fatalf("handleOverview: %v", err)
	}
	if len(r.messages) != 1 || !strings.Contains(strings.ToLower(r.messages[0]), "today") {
		t.Errorf("today-mode empty reply = %v", r.messages)
	}
	if strings.Contains(r.messages[0], "Arsenal") {
		t.Errorf("today mode leaked a non-today fixture:\n%s", r.messages[0])
	}
}

func TestFilterOverview_EachModeSelectsTheRightFixtures(t *testing.T) {
	fixtures := []fpl.FixtureOverview{
		{FixtureID: 1, TeamHome: "A", TeamAway: "B", Started: true, Finished: true, FinishedProvisional: true, Today: true},
		{FixtureID: 2, TeamHome: "C", TeamAway: "D", Started: true, Today: true},
		{FixtureID: 3, TeamHome: "E", TeamAway: "F", Started: false, Today: false},
	}

	ids := func(fs []fpl.FixtureOverview) []int {
		out := make([]int, len(fs))
		for i, f := range fs {
			out[i] = f.FixtureID
		}
		return out
	}

	if got := ids(filterOverview(fixtures, overviewGameweek)); len(got) != 3 {
		t.Errorf("gameweek window = %v, want all three", got)
	}
	if got := ids(filterOverview(fixtures, overviewToday)); len(got) != 2 || got[0] != 1 || got[1] != 2 {
		t.Errorf("today window = %v, want [1 2]", got)
	}
	if got := ids(filterOverview(fixtures, overviewLive)); len(got) != 1 || got[0] != 2 {
		t.Errorf("live window = %v, want [2] (started, not finished-provisional)", got)
	}
}

func TestRenderOverview_TodayModeRendersFixturesAndScorers(t *testing.T) {
	fixtures := []fpl.FixtureOverview{{
		TeamHome: "Arsenal", TeamAway: "Chelsea", HomeScore: 2, AwayScore: 1,
		Started: true, Today: true,
		Scorers: []fpl.OverviewStat{
			{StatName: fpl.OverviewGoal, PlayerName: "Saka", OwnerName: "Sam", Value: 2},
			{StatName: fpl.OverviewOwnGoal, PlayerName: "Gabriel", Value: 1},
			{StatName: fpl.OverviewRedCard, PlayerName: "Palmer", Value: 1},
		},
	}}

	msgs := renderOverview("Coq au Ian", 4, overviewToday, fixtures)
	if len(msgs) != 1 {
		t.Fatalf("len(msgs) = %d, want 1 for a single fixture", len(msgs))
	}
	msg := msgs[0]

	for _, want := range []string{"today's fixtures", "Arsenal 2 - 1 Chelsea", "⚽⚽ Saka — Sam", "Gabriel (OG)", "🟥 Palmer"} {
		if !strings.Contains(msg, want) {
			t.Errorf("today-mode render missing %q:\n%s", want, msg)
		}
	}
}

func TestRenderOverview_SplitsLongOutputAcrossMessages(t *testing.T) {
	scorers := make([]fpl.OverviewStat, 12)
	for i := range scorers {
		scorers[i] = fpl.OverviewStat{StatName: fpl.OverviewGoal, PlayerName: "A Long Player Name Here", OwnerName: "Some Owner", Value: 1}
	}
	fixtures := make([]fpl.FixtureOverview, 20)
	for i := range fixtures {
		fixtures[i] = fpl.FixtureOverview{
			TeamHome: "Manchester United", TeamAway: "Wolverhampton Wanderers",
			HomeScore: 3, AwayScore: 3, Started: true, Scorers: scorers,
		}
	}

	msgs := renderOverview("A League", 4, overviewGameweek, fixtures)
	if len(msgs) < 2 {
		t.Fatalf("len(msgs) = %d, want a split for a 20-fixture high-scoring gameweek", len(msgs))
	}
	for i, m := range msgs {
		if len(m) > maxDiscordMessage {
			t.Errorf("message %d is %d chars, over the %d limit", i, len(m), maxDiscordMessage)
		}
	}
	if !strings.HasPrefix(msgs[0], "**A League — GW4") {
		t.Errorf("first message missing the title: %q", msgs[0][:40])
	}
	if strings.Contains(msgs[1], "GW4 gameweek fixtures**") {
		t.Errorf("continuation message repeated the title:\n%s", msgs[1])
	}
}

func TestHandleOverview_RejectsUnknownMode(t *testing.T) {
	r := &recordingResponder{}
	in := &cmdInput{snap: overviewSnap(), opts: cmdOptions{"mode": "yesterday"}, resp: r}
	if err := handleOverview(in); err != nil {
		t.Fatalf("handleOverview: %v", err)
	}
	if len(r.messages) != 1 || !strings.Contains(r.messages[0], "mode") {
		t.Errorf("unknown-mode reply = %v", r.messages)
	}
}

func TestHandleOverview_NoSnapshotReportsStartingUp(t *testing.T) {
	r := &recordingResponder{}
	if err := handleOverview(&cmdInput{snap: nil, resp: r}); err != nil {
		t.Fatalf("handleOverview: %v", err)
	}
	if len(r.messages) != 1 || !strings.Contains(strings.ToLower(r.messages[0]), "starting up") {
		t.Errorf("nil-snapshot reply = %v", r.messages)
	}
}

func TestHandleOverview_RegisteredWithHandlerAndSpec(t *testing.T) {
	b, err := newTestBot()
	if err != nil {
		t.Fatalf("newTestBot: %v", err)
	}
	if _, ok := b.handlers["overview"]; !ok {
		t.Error("no handler registered for /overview")
	}
	var found bool
	for _, s := range commandSpecs() {
		if s.Name == "overview" {
			found = true
			if len(s.Options) == 0 || s.Options[0].Name != "mode" {
				t.Errorf("/overview spec missing the mode option: %+v", s.Options)
			}
		}
	}
	if !found {
		t.Error("/overview missing from commandSpecs()")
	}
}
