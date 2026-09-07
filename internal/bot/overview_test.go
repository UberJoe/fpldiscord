package bot

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/UberJoe/fpldiscord/internal/fpl"
	"github.com/bwmarrin/discordgo"
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

// overviewMessages runs handleOverview against a recordingResponder and returns
// the embed payloads it emitted — one []*discordgo.MessageEmbed per Discord
// message — failing the test if the handler sent any plain-text message.
func overviewMessages(t *testing.T, in *cmdInput) [][]*discordgo.MessageEmbed {
	t.Helper()
	r := &recordingResponder{}
	in.resp = r
	if err := handleOverview(in); err != nil {
		t.Fatalf("handleOverview: %v", err)
	}
	if len(r.messages) != 0 {
		t.Fatalf("plain messages = %v, want none — /overview replies with embeds", r.messages)
	}
	return r.embeds
}

// allFields flattens every field of every embed across every message into a
// single "name\nvalue" string for coarse substring assertions.
func allFields(msgs [][]*discordgo.MessageEmbed) string {
	var b strings.Builder
	for _, msg := range msgs {
		for _, e := range msg {
			for _, f := range e.Fields {
				b.WriteString(f.Name)
				b.WriteByte('\n')
				b.WriteString(f.Value)
				b.WriteByte('\n')
			}
		}
	}
	return b.String()
}

func countFields(msgs [][]*discordgo.MessageEmbed) int {
	n := 0
	for _, msg := range msgs {
		for _, e := range msg {
			n += len(e.Fields)
		}
	}
	return n
}

func TestHandleOverview_DefaultModeShowsWholeGameweekAsEmbedFields(t *testing.T) {
	msgs := overviewMessages(t, &cmdInput{snap: overviewSnap()})

	if len(msgs) != 1 || len(msgs[0]) != 1 {
		t.Fatalf("embeds = %v, want one message carrying one embed", msgs)
	}
	e := msgs[0][0]

	if e.Title != "GW4 gameweek fixtures" {
		t.Errorf("title = %q, want the gameweek mode wording", e.Title)
	}
	if e.Footer == nil || e.Footer.Text != "Coq au Ian" {
		t.Errorf("footer = %+v, want the league name", e.Footer)
	}
	if want := overviewSnap().BuiltAt.Format(time.RFC3339); e.Timestamp != want {
		t.Errorf("timestamp = %q, want the snapshot build time %q", e.Timestamp, want)
	}
	// A live fixture is in the window, so the colour bar is provisional.
	if e.Color != colorProvisional {
		t.Errorf("color = %#x, want provisional %#x (a fixture is live)", e.Color, colorProvisional)
	}

	// One field per fixture, in the snapshot's order.
	if len(e.Fields) != 3 {
		t.Fatalf("fields = %d, want one per fixture", len(e.Fields))
	}
	wantNames := []string{"Tottenham 1 - 0 West Ham (FT)", "Arsenal 2 - 1 Chelsea (live)", "Chelsea vs Tottenham"}
	for i, want := range wantNames {
		if e.Fields[i].Name != want {
			t.Errorf("field %d name = %q, want %q", i, e.Fields[i].Name, want)
		}
		if e.Fields[i].Inline {
			t.Errorf("field %d is inline, want a non-inline block per fixture", i)
		}
	}

	body := allFields(msgs)
	for _, want := range []string{"Saka", "Sam"} {
		if !strings.Contains(body, want) {
			t.Errorf("field bodies missing %q:\n%s", want, body)
		}
	}
	// Rice only has a defensive_contribution — it must not be rendered.
	if strings.Contains(body, "Rice") {
		t.Errorf("defensive_contribution player leaked into the goalscorer display:\n%s", body)
	}
	// The upcoming fixture has no goals and carries the note.
	if e.Fields[2].Value != "_no goals_" {
		t.Errorf("upcoming fixture value = %q, want the no-goals note", e.Fields[2].Value)
	}
}

func TestHandleOverview_LiveModeDropsFinishedAndUpcoming(t *testing.T) {
	in := &cmdInput{snap: overviewSnap(), opts: cmdOptions{"mode": "live"}}
	msgs := overviewMessages(t, in)

	if len(msgs) != 1 || len(msgs[0]) != 1 {
		t.Fatalf("embeds = %v, want one message carrying one embed", msgs)
	}
	e := msgs[0][0]

	if len(e.Fields) != 1 || e.Fields[0].Name != "Arsenal 2 - 1 Chelsea (live)" {
		t.Errorf("live window fields = %+v, want just the in-play fixture", e.Fields)
	}
	if e.Color != colorProvisional {
		t.Errorf("color = %#x, want provisional %#x", e.Color, colorProvisional)
	}
	body := allFields(msgs)
	if strings.Contains(body, "West Ham") || strings.Contains(body, "Chelsea vs Tottenham") {
		t.Errorf("live window still shows a finished or upcoming fixture:\n%s", body)
	}
}

func TestHandleOverview_TodayModeEmptyIsExplainedAsPlainText(t *testing.T) {
	// No fixture in overviewSnap carries a kickoff on the build date, so Today
	// mode comes back empty — the handler must say so in plain text rather than
	// erroring or falling back to the full list.
	r := &recordingResponder{}
	in := &cmdInput{snap: overviewSnap(), opts: cmdOptions{"mode": "today"}, resp: r}
	if err := handleOverview(in); err != nil {
		t.Fatalf("handleOverview: %v", err)
	}
	if len(r.embeds) != 0 {
		t.Errorf("today-mode empty path emitted an embed %v, want plain text only", r.embeds)
	}
	if len(r.messages) != 1 || !strings.Contains(strings.ToLower(r.messages[0]), "today") {
		t.Errorf("today-mode empty reply = %v", r.messages)
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

	msgs := renderOverview("Coq au Ian", time.Now(), 4, overviewToday, fixtures)
	if len(msgs) != 1 || len(msgs[0]) != 1 {
		t.Fatalf("msgs = %v, want one message carrying one embed", msgs)
	}
	e := msgs[0][0]

	if e.Title != "GW4 today's fixtures" {
		t.Errorf("title = %q, want the today mode wording", e.Title)
	}
	if len(e.Fields) != 1 {
		t.Fatalf("fields = %d, want one for the single fixture", len(e.Fields))
	}
	if e.Fields[0].Name != "Arsenal 2 - 1 Chelsea (live)" {
		t.Errorf("field name = %q", e.Fields[0].Name)
	}
	for _, want := range []string{"⚽⚽ Saka — Sam", "Gabriel (OG)", "🟥 Palmer"} {
		if !strings.Contains(e.Fields[0].Value, want) {
			t.Errorf("field value missing %q:\n%s", want, e.Fields[0].Value)
		}
	}
}

// bigOverview builds n minimal no-goals fixtures for chunking-boundary tests.
func bigOverview(n int) []fpl.FixtureOverview {
	out := make([]fpl.FixtureOverview, n)
	for i := range out {
		out[i] = fpl.FixtureOverview{TeamHome: "Home", TeamAway: "Away"}
	}
	return out
}

func TestRenderOverview_OneFieldPerFixture(t *testing.T) {
	msgs := renderOverview("A League", time.Now(), 4, overviewGameweek, bigOverview(7))
	if got := countFields(msgs); got != 7 {
		t.Errorf("total fields = %d, want one per fixture", got)
	}
}

func TestRenderOverview_SplitsPastTwentyFiveFixturesIntoMultipleEmbeds(t *testing.T) {
	msgs := renderOverview("A League", time.Now(), 4, overviewGameweek, bigOverview(26))

	if len(msgs) != 1 {
		t.Fatalf("messages = %d, want 26 fixtures to still fit one message", len(msgs))
	}
	if len(msgs[0]) != 2 {
		t.Fatalf("embeds in the message = %d, want a split into two past 25 fields", len(msgs[0]))
	}
	if len(msgs[0][0].Fields) != maxEmbedFields || len(msgs[0][1].Fields) != 1 {
		t.Errorf("field split = %d + %d, want %d + 1", len(msgs[0][0].Fields), len(msgs[0][1].Fields), maxEmbedFields)
	}
	// Only the first embed carries the title.
	if msgs[0][0].Title == "" || msgs[0][1].Title != "" {
		t.Errorf("titles = %q / %q, want the title on the first embed only", msgs[0][0].Title, msgs[0][1].Title)
	}
	if countFields(msgs) != 26 {
		t.Errorf("total fields = %d, want 26", countFields(msgs))
	}
}

func TestRenderOverview_SplitsPastTenEmbedsIntoASecondMessage(t *testing.T) {
	// 251 fixtures = ten full 25-field embeds (one message's worth) plus one.
	msgs := renderOverview("A League", time.Now(), 4, overviewGameweek, bigOverview(251))

	if len(msgs) != 2 {
		t.Fatalf("messages = %d, want a spill into a second message past ten embeds", len(msgs))
	}
	if len(msgs[0]) != maxEmbedsPerMessage {
		t.Errorf("first message embeds = %d, want %d", len(msgs[0]), maxEmbedsPerMessage)
	}
	if len(msgs[1]) != 1 || len(msgs[1][0].Fields) != 1 {
		t.Errorf("second message = %+v, want one embed with the last fixture", msgs[1])
	}
	if countFields(msgs) != 251 {
		t.Errorf("total fields = %d, want 251", countFields(msgs))
	}
}

func TestRenderOverview_SplitsAMessageBeforeTheCharacterCeiling(t *testing.T) {
	// Twelve fixtures, each with a ~700-char scorer list: well under the 25-field
	// and 10-embed walls, so any message split here is the character budget
	// firing.
	scorers := make([]fpl.OverviewStat, 10)
	for i := range scorers {
		scorers[i] = fpl.OverviewStat{
			StatName:   fpl.OverviewGoal,
			PlayerName: strings.Repeat("x", 44),
			OwnerName:  strings.Repeat("y", 18),
			Value:      1,
		}
	}
	fixtures := make([]fpl.FixtureOverview, 12)
	for i := range fixtures {
		fixtures[i] = fpl.FixtureOverview{TeamHome: "Home", TeamAway: "Away", Started: true, Scorers: scorers}
	}

	msgs := renderOverview("A League", time.Now(), 4, overviewGameweek, fixtures)

	if len(msgs) < 2 {
		t.Fatalf("messages = %d, want a character-budget split", len(msgs))
	}
	for i, m := range msgs {
		if len(m) > maxEmbedsPerMessage {
			t.Errorf("message %d has %d embeds, over the limit", i, len(m))
		}
		chars := 0
		for _, e := range m {
			chars += utf8.RuneCountInString(e.Title)
			if e.Footer != nil {
				chars += utf8.RuneCountInString(e.Footer.Text)
			}
			for _, f := range e.Fields {
				chars += utf8.RuneCountInString(f.Name) + utf8.RuneCountInString(f.Value)
			}
		}
		if chars > maxMessageEmbedChars {
			t.Errorf("message %d totals %d chars, over Discord's %d ceiling", i, chars, maxMessageEmbedChars)
		}
	}
	if countFields(msgs) != 12 {
		t.Errorf("total fields = %d, want 12 (no fixture dropped in the split)", countFields(msgs))
	}
}

func TestRenderOverview_TruncatesAFixtureValueOverTheFieldLimit(t *testing.T) {
	scorers := make([]fpl.OverviewStat, 40)
	for i := range scorers {
		scorers[i] = fpl.OverviewStat{
			StatName:   fpl.OverviewGoal,
			PlayerName: "A Player With A Fairly Long Web Name Here",
			OwnerName:  "A Manager With A Long Team Name",
			Value:      1,
		}
	}
	fixtures := []fpl.FixtureOverview{{TeamHome: "Home", TeamAway: "Away", Started: true, Scorers: scorers}}

	msgs := renderOverview("A League", time.Now(), 4, overviewGameweek, fixtures)
	v := msgs[0][0].Fields[0].Value

	if utf8.RuneCountInString(v) > maxEmbedFieldValue {
		t.Errorf("field value is %d runes, over the %d limit", utf8.RuneCountInString(v), maxEmbedFieldValue)
	}
	if !strings.HasSuffix(v, "…") {
		t.Errorf("over-long field value was not truncated with an ellipsis: ...%q", v[len(v)-10:])
	}
}

func TestHandleOverview_RejectsUnknownModeAsPlainText(t *testing.T) {
	r := &recordingResponder{}
	in := &cmdInput{snap: overviewSnap(), opts: cmdOptions{"mode": "yesterday"}, resp: r}
	if err := handleOverview(in); err != nil {
		t.Fatalf("handleOverview: %v", err)
	}
	if len(r.embeds) != 0 {
		t.Errorf("unknown-mode path emitted an embed %v, want plain text only", r.embeds)
	}
	if len(r.messages) != 1 || !strings.Contains(r.messages[0], "mode") {
		t.Errorf("unknown-mode reply = %v", r.messages)
	}
}

func TestHandleOverview_NoSnapshotReportsStartingUpAsPlainText(t *testing.T) {
	r := &recordingResponder{}
	if err := handleOverview(&cmdInput{snap: nil, resp: r}); err != nil {
		t.Fatalf("handleOverview: %v", err)
	}
	if len(r.embeds) != 0 {
		t.Errorf("nil-snapshot path emitted an embed %v, want plain text only", r.embeds)
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
