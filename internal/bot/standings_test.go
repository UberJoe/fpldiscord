package bot

import (
	"strings"
	"testing"
	"time"

	"github.com/UberJoe/fpldiscord/internal/fpl"
	"github.com/bwmarrin/discordgo"
)

// classicSnapBuiltAt is the build time every classicSnap carries, so the
// standings embed's Timestamp is deterministic.
var classicSnapBuiltAt = time.Date(2026, 9, 7, 14, 30, 0, 0, time.UTC)

// classicSnap is a hand-built classic-mode snapshot with two members whose
// standings rows are deliberately out of rank order.
func classicSnap() *fpl.Snapshot {
	return &fpl.Snapshot{
		BuiltAt:    classicSnapBuiltAt,
		LeagueName: "Coq au Ian",
		LeagueMode: fpl.ModeClassic,
		CurrentGW:  5,
		LeagueDetails: fpl.LeagueDetails{
			LeagueEntries: []fpl.LeagueEntry{
				{ID: 89, EntryName: "Coq au Vin"},
				{ID: 39936, EntryName: "Bruno Dos Tres"},
			},
			Standings: []fpl.Standing{
				{Rank: 2, LeagueEntry: 39936, Total: 180, EventTotal: 40},
				{Rank: 1, LeagueEntry: 89, Total: 210, EventTotal: 55},
			},
		},
	}
}

// standingsEmbed runs handleStandings against a recordingResponder and returns
// the single embed it emitted, failing the test if the handler sent anything
// else.
func standingsEmbed(t *testing.T, snap *fpl.Snapshot) *discordgo.MessageEmbed {
	t.Helper()
	r := &recordingResponder{}
	if err := handleStandings(&cmdInput{snap: snap, resp: r}); err != nil {
		t.Fatalf("handleStandings: %v", err)
	}
	if len(r.messages) != 0 {
		t.Fatalf("plain messages = %v, want none — /standings replies with an embed", r.messages)
	}
	if len(r.embeds) != 1 || len(r.embeds[0]) != 1 {
		t.Fatalf("embeds = %v, want exactly one message carrying one embed", r.embeds)
	}
	return r.embeds[0][0]
}

func TestHandleStandings_RepliesWithOneNeutralEmbed(t *testing.T) {
	e := standingsEmbed(t, classicSnap())

	if e.Color != colorNeutral {
		t.Errorf("color = %#x, want neutral %#x", e.Color, colorNeutral)
	}
	if e.Title != "Standings" {
		t.Errorf("title = %q, want %q", e.Title, "Standings")
	}
	if e.Author == nil || e.Author.Name != "Coq au Ian" {
		t.Errorf("author = %+v, want the league name", e.Author)
	}
	if e.Footer == nil || !strings.Contains(e.Footer.Text, "GW5") {
		t.Errorf("footer = %+v, want the gameweek context", e.Footer)
	}
	if want := classicSnapBuiltAt.Format(time.RFC3339); e.Timestamp != want {
		t.Errorf("timestamp = %q, want the snapshot build time %q", e.Timestamp, want)
	}
}

func TestHandleStandings_DescriptionIsAFencedTableInRankOrder(t *testing.T) {
	e := standingsEmbed(t, classicSnap())
	d := e.Description

	if !strings.HasPrefix(d, "```") || !strings.HasSuffix(d, "```") {
		t.Errorf("description is not a fenced code block:\n%s", d)
	}
	for _, want := range []string{"Coq au Vin", "Bruno Dos Tres", "210", "180", "55", "40"} {
		if !strings.Contains(d, want) {
			t.Errorf("description missing %q:\n%s", want, d)
		}
	}
	// Rank 1 (Coq au Vin) renders above rank 2 (Bruno) despite the input order.
	if strings.Index(d, "Coq au Vin") > strings.Index(d, "Bruno Dos Tres") {
		t.Errorf("rows not in rank order:\n%s", d)
	}
}

func TestHandleStandings_DescriptionHasNoMovementArrowGlyphs(t *testing.T) {
	e := standingsEmbed(t, classicSnap())
	for _, glyph := range []string{"▲", "▼", "▬", "↑", "↓", "→"} {
		if strings.Contains(e.Description, glyph) {
			t.Errorf("description contains movement-arrow glyph %q — arrows are web-only:\n%s", glyph, e.Description)
		}
	}
}

func TestHandleStandings_LongTeamNameIsCappedForMobileWidth(t *testing.T) {
	s := classicSnap()
	s.LeagueDetails.LeagueEntries[0].EntryName = "The Wandering Albatrosses of Anfield"

	e := standingsEmbed(t, s)
	for _, line := range strings.Split(strings.Trim(e.Description, "`\n"), "\n") {
		if n := len([]rune(line)); n > 34 {
			t.Errorf("table line is %d runes wide, want <= 34 for mobile:\n%q", n, line)
		}
	}
	if strings.Contains(e.Description, "Wandering Albatrosses of Anfield") {
		t.Errorf("long team name was not capped:\n%s", e.Description)
	}
}

func TestHandleStandings_H2HModeReportsUnavailableAsPlainText(t *testing.T) {
	s := classicSnap()
	s.LeagueMode = fpl.ModeH2H

	r := &recordingResponder{}
	if err := handleStandings(&cmdInput{snap: s, resp: r}); err != nil {
		t.Fatalf("handleStandings: %v", err)
	}
	if len(r.embeds) != 0 {
		t.Errorf("h2h path emitted an embed %v, want plain text only", r.embeds)
	}
	if len(r.messages) != 1 || !strings.Contains(strings.ToLower(r.messages[0]), "head-to-head") {
		t.Errorf("h2h reply = %v", r.messages)
	}
}

func TestHandleStandings_NoSnapshotReportsStartingUpAsPlainText(t *testing.T) {
	r := &recordingResponder{}
	if err := handleStandings(&cmdInput{snap: nil, resp: r}); err != nil {
		t.Fatalf("handleStandings: %v", err)
	}
	if len(r.embeds) != 0 {
		t.Errorf("nil-snapshot path emitted an embed %v, want plain text only", r.embeds)
	}
	if len(r.messages) != 1 || !strings.Contains(strings.ToLower(r.messages[0]), "starting up") {
		t.Errorf("nil-snapshot reply = %v", r.messages)
	}
}
