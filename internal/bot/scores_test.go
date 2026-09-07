package bot

import (
	"strings"
	"testing"
	"time"

	"github.com/UberJoe/fpldiscord/internal/fpl"
	"github.com/bwmarrin/discordgo"
)

// scoresSnapBuiltAt is the build time every scoresSnap carries, so the /scores
// embed's Timestamp is deterministic.
var scoresSnapBuiltAt = time.Date(2026, 9, 12, 17, 45, 0, 0, time.UTC)

// scoresSnap is a two-manager classic snapshot for GW5 sharing one live feed:
//
//   - Tess XI (entry 500): starts the slot-5 DEF (element 5) who blanked in a
//     finished fixture, so the auto-sub brings on the slot-13 bench DEF
//     (element 13, 6 pts). Scoring XI total 27 — it would be 21 without the sub.
//   - Bruno Dos Tres (entry 600): starts the low-scoring MID (element 14, 4 pts)
//     instead, no blanks, total 25.
func scoresSnap() *fpl.Snapshot {
	elem := func(id, typ int) fpl.Element {
		return fpl.Element{ID: fpl.ElementID(id), ElementType: typ, Team: 1}
	}
	stat := func(pts, mins int) fpl.LiveElement {
		return fpl.LiveElement{Stats: fpl.LiveStats{TotalPoints: pts, Minutes: mins}}
	}
	pick := func(el, pos int) fpl.Pick {
		return fpl.Pick{Element: fpl.ElementID(el), Position: pos, Multiplier: 1}
	}

	return &fpl.Snapshot{
		BuiltAt:    scoresSnapBuiltAt,
		LeagueName: "Coq au Ian",
		LeagueMode: fpl.ModeClassic,
		CurrentGW:  5,
		Game:       fpl.Game{CurrentEvent: 5},
		Bootstrap: fpl.Bootstrap{
			Settings: fpl.Settings{Squad: fpl.SquadSettings{
				Size: 15, Play: 11, MinPlayGKP: 1, MaxPlayGKP: 1,
				MinPlayDEF: 3, MinPlayMID: 2, MinPlayFWD: 1,
				PositionTypeLocks: map[string]string{"12": "GKP"},
			}},
			Elements: []fpl.Element{
				elem(1, 1), elem(2, 2), elem(3, 2), elem(4, 2), elem(5, 2),
				elem(6, 3), elem(7, 3), elem(8, 3), elem(9, 3),
				elem(10, 4), elem(11, 4),
				elem(12, 1), elem(13, 2), elem(14, 3), elem(15, 4),
			},
		},
		LeagueDetails: fpl.LeagueDetails{
			League: fpl.League{Name: "Coq au Ian", Scoring: "c"},
			LeagueEntries: []fpl.LeagueEntry{
				{ID: 1, EntryID: 500, EntryName: "Tess XI"},
				{ID: 2, EntryID: 600, EntryName: "Bruno Dos Tres"},
			},
			Standings: []fpl.Standing{
				{Rank: 1, LeagueEntry: 1, Total: 100},
				{Rank: 2, LeagueEntry: 2, Total: 90},
			},
		},
		Live: map[int]fpl.LiveGW{
			5: {
				Fixtures: []fpl.LiveFixture{{ID: 1, Started: true, Finished: true, FinishedProvisional: true}},
				Elements: map[fpl.ElementID]fpl.LiveElement{
					1: stat(3, 90), 2: stat(2, 90), 3: stat(2, 90), 4: stat(2, 90),
					5: {Stats: fpl.LiveStats{TotalPoints: 0, Minutes: 0}, Explain: []fpl.LiveExplain{{Fixture: 1}}},
					6: stat(2, 90), 7: stat(2, 90), 8: stat(2, 90), 9: stat(2, 90),
					10: stat(2, 90), 11: stat(2, 90),
					13: stat(6, 90), 14: stat(4, 90),
				},
			},
		},
		Entries: map[fpl.EntryID]fpl.EntryEvent{
			500: {Picks: []fpl.Pick{
				pick(1, 1), pick(2, 2), pick(3, 3), pick(4, 4), pick(5, 5),
				pick(6, 6), pick(7, 7), pick(8, 8), pick(9, 9),
				pick(10, 10), pick(11, 11),
				pick(12, 12), pick(13, 13), pick(14, 14), pick(15, 15),
			}},
			600: {Picks: []fpl.Pick{
				pick(1, 1), pick(2, 2), pick(3, 3), pick(4, 4), pick(14, 5),
				pick(6, 6), pick(7, 7), pick(8, 8), pick(9, 9),
				pick(10, 10), pick(11, 11),
				pick(12, 12), pick(13, 13), pick(5, 14), pick(15, 15),
			}},
		},
	}
}

// scoresEmbed runs handleScores against a recordingResponder and returns the
// single embed it emitted, failing the test if the handler sent anything else.
func scoresEmbed(t *testing.T, in *cmdInput) *discordgo.MessageEmbed {
	t.Helper()
	r := &recordingResponder{}
	in.resp = r
	if err := handleScores(in); err != nil {
		t.Fatalf("handleScores: %v", err)
	}
	if len(r.messages) != 0 {
		t.Fatalf("plain messages = %v, want none — /scores replies with an embed", r.messages)
	}
	if len(r.embeds) != 1 || len(r.embeds[0]) != 1 {
		t.Fatalf("embeds = %v, want exactly one message carrying one embed", r.embeds)
	}
	return r.embeds[0][0]
}

func TestHandleScores_RepliesWithOneEmbedTitledForTheGameweek(t *testing.T) {
	e := scoresEmbed(t, &cmdInput{snap: scoresSnap()})

	if e.Title != "GW5 scores" {
		t.Errorf("title = %q, want %q", e.Title, "GW5 scores")
	}
	if e.Author == nil || e.Author.Name != "Coq au Ian" {
		t.Errorf("author = %+v, want the league name", e.Author)
	}
	if want := scoresSnapBuiltAt.Format(time.RFC3339); e.Timestamp != want {
		t.Errorf("timestamp = %q, want the snapshot build time %q", e.Timestamp, want)
	}
}

func TestHandleScores_DescriptionListsLiveGameweekPointsHighestFirstWithAutoSubs(t *testing.T) {
	e := scoresEmbed(t, &cmdInput{snap: scoresSnap()})
	d := e.Description

	if !strings.HasPrefix(d, "```") || !strings.HasSuffix(d, "```") {
		t.Errorf("description is not a fenced code block:\n%s", d)
	}
	for _, want := range []string{"Tess XI", "Bruno Dos Tres", "27", "25"} {
		if !strings.Contains(d, want) {
			t.Errorf("description missing %q:\n%s", want, d)
		}
	}
	// The auto-sub fired for Tess XI: the raw submitted XI sums to 21.
	if strings.Contains(d, "21") {
		t.Errorf("auto-subs not applied — found the un-subbed total 21:\n%s", d)
	}
	// Higher gameweek total is listed first.
	if strings.Index(d, "Tess XI") > strings.Index(d, "Bruno Dos Tres") {
		t.Errorf("rows not ordered by gameweek points desc:\n%s", d)
	}
	// The provisional/final wording is not on the title/description any more.
	if strings.Contains(d, "provisional") || strings.Contains(d, "final") {
		t.Errorf("phase wording leaked into the description:\n%s", d)
	}
}

func TestHandleScores_ProvisionalColourAndFooterWhileGameweekLive(t *testing.T) {
	snap := scoresSnap() // GWFinished is false
	e := scoresEmbed(t, &cmdInput{snap: snap})

	if e.Color != colorProvisional {
		t.Errorf("color = %#x, want provisional %#x", e.Color, colorProvisional)
	}
	if e.Footer == nil || !strings.Contains(e.Footer.Text, "provisional") {
		t.Errorf("footer = %+v, want the provisional phrase", e.Footer)
	}
}

func TestHandleScores_FinalColourAndFooterOnceGameweekFinished(t *testing.T) {
	snap := scoresSnap()
	snap.GWFinished = true
	e := scoresEmbed(t, &cmdInput{snap: snap})

	if e.Color != colorFinal {
		t.Errorf("color = %#x, want final %#x", e.Color, colorFinal)
	}
	if e.Footer == nil || e.Footer.Text != "final" {
		t.Errorf("footer = %+v, want %q", e.Footer, "final")
	}
}

func TestHandleScores_UnscoredManagerShownAsDashAndSinksToBottom(t *testing.T) {
	snap := scoresSnap()
	snap.LeagueDetails.LeagueEntries = append(snap.LeagueDetails.LeagueEntries,
		fpl.LeagueEntry{ID: 3, EntryID: 700, EntryName: "Ghost XI"}) // no Entries picks -> cannot be scored

	e := scoresEmbed(t, &cmdInput{snap: snap})
	d := e.Description

	var ghostLine string
	for _, line := range strings.Split(d, "\n") {
		if strings.Contains(line, "Ghost XI") {
			ghostLine = line
		}
	}
	if ghostLine == "" {
		t.Fatalf("unscored manager dropped from the table:\n%s", d)
	}
	if !strings.HasSuffix(strings.TrimRight(ghostLine, " "), "-") || strings.ContainsAny(ghostLine, "0123456789") {
		t.Errorf("unscored manager not marked with a bare dash: %q", ghostLine)
	}
	if strings.Index(d, "Ghost XI") < strings.Index(d, "Tess XI") ||
		strings.Index(d, "Ghost XI") < strings.Index(d, "Bruno Dos Tres") {
		t.Errorf("unscored manager did not sink to the bottom:\n%s", d)
	}
}

func TestHandleScores_DefaultsToCurrentGameweek(t *testing.T) {
	e := scoresEmbed(t, &cmdInput{snap: scoresSnap()})
	if e.Title != "GW5 scores" {
		t.Errorf("default gameweek is not the current one: title = %q", e.Title)
	}
}

func TestHandleScores_PastGameweekIsExplainedAsPlainText(t *testing.T) {
	r := &recordingResponder{}
	in := &cmdInput{snap: scoresSnap(), opts: cmdOptions{"gw": float64(3)}, resp: r}
	if err := handleScores(in); err != nil {
		t.Fatalf("handleScores: %v", err)
	}
	if len(r.embeds) != 0 {
		t.Errorf("past-gameweek path emitted an embed %v, want plain text only", r.embeds)
	}
	if len(r.messages) != 1 || !strings.Contains(strings.ToLower(r.messages[0]), "current gameweek") {
		t.Errorf("past-gameweek reply = %v", r.messages)
	}
}

func TestHandleScores_NoSnapshotReportsStartingUpAsPlainText(t *testing.T) {
	r := &recordingResponder{}
	if err := handleScores(&cmdInput{snap: nil, resp: r}); err != nil {
		t.Fatalf("handleScores: %v", err)
	}
	if len(r.embeds) != 0 {
		t.Errorf("nil-snapshot path emitted an embed %v, want plain text only", r.embeds)
	}
	if len(r.messages) != 1 || !strings.Contains(strings.ToLower(r.messages[0]), "starting up") {
		t.Errorf("nil-snapshot reply = %v", r.messages)
	}
}

func TestHandleScores_EmptyLeagueIsPlainText(t *testing.T) {
	snap := scoresSnap()
	snap.LeagueDetails.LeagueEntries = nil

	r := &recordingResponder{}
	if err := handleScores(&cmdInput{snap: snap, resp: r}); err != nil {
		t.Fatalf("handleScores: %v", err)
	}
	if len(r.embeds) != 0 {
		t.Errorf("empty-league path emitted an embed %v, want plain text only", r.embeds)
	}
	if len(r.messages) != 1 {
		t.Errorf("empty-league reply = %v, want one plain message", r.messages)
	}
}
