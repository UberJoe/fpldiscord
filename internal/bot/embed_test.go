package bot

import (
	"fmt"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/bwmarrin/discordgo"
)

func TestDataEmbed_StampsAuthorTitleColourFooterAndTimestamp(t *testing.T) {
	built := time.Date(2026, 9, 7, 14, 30, 0, 0, time.UTC)

	e := dataEmbed("Coq au Ian", built, "Standings", colorNeutral, "GW5 · total points league")

	if e.Author == nil || e.Author.Name != "Coq au Ian" {
		t.Errorf("author = %+v, want league name on the author line", e.Author)
	}
	if e.Title != "Standings" {
		t.Errorf("title = %q, want %q", e.Title, "Standings")
	}
	if e.Color != colorNeutral {
		t.Errorf("color = %#x, want %#x", e.Color, colorNeutral)
	}
	if e.Footer == nil || e.Footer.Text != "GW5 · total points league" {
		t.Errorf("footer = %+v, want the gameweek context text", e.Footer)
	}
	if want := built.Format(time.RFC3339); e.Timestamp != want {
		t.Errorf("timestamp = %q, want RFC 3339 of BuiltAt %q", e.Timestamp, want)
	}
}

func TestDataEmbed_OmitsAuthorWithoutLeagueNameAndFooterWhenEmpty(t *testing.T) {
	e := dataEmbed("", time.Now(), "GW5 scores", colorProvisional, "")

	if e.Author != nil {
		t.Errorf("author = %+v, want nil when the snapshot has no league name", e.Author)
	}
	if e.Footer != nil {
		t.Errorf("footer = %+v, want nil when the footer text is empty", e.Footer)
	}
}

func TestDataEmbed_OmitsTimestampWhenBuiltAtIsZero(t *testing.T) {
	e := dataEmbed("Coq au Ian", time.Time{}, "Standings", colorNeutral, "")
	if e.Timestamp != "" {
		t.Errorf("timestamp = %q, want empty for a zero BuiltAt", e.Timestamp)
	}
}

func TestEmbedStateColours_AreDistinct(t *testing.T) {
	if colorNeutral == colorProvisional || colorProvisional == colorFinal || colorNeutral == colorFinal {
		t.Fatalf("state colours must differ: neutral=%#x provisional=%#x final=%#x",
			colorNeutral, colorProvisional, colorFinal)
	}
}

// chunkN feeds n trivial fields ("f0","b0" …) through a chunker built from the
// given scaffold and returns the per-message slices.
func chunkN(s embedFieldScaffold, n int) [][]*discordgo.MessageEmbed {
	c := newEmbedFieldChunker(s)
	for i := 0; i < n; i++ {
		c.add(fmt.Sprintf("f%d", i), fmt.Sprintf("b%d", i))
	}
	c.flushMessage()
	return c.messages
}

func TestEmbedFieldChunker_SplitsPastTwentyFiveFieldsIntoAnotherEmbed(t *testing.T) {
	s := embedFieldScaffold{title: "T", color: colorNeutral, footer: "F", titleFirstOnly: true}

	if msgs := chunkN(s, maxEmbedFields); len(msgs) != 1 || len(msgs[0]) != 1 {
		t.Fatalf("%d fields = %v, want a single embed", maxEmbedFields, msgs)
	}

	msgs := chunkN(s, maxEmbedFields+1)
	if len(msgs) != 1 {
		t.Fatalf("messages = %d, want 26 fields to still fit one message", len(msgs))
	}
	if len(msgs[0]) != 2 {
		t.Fatalf("embeds = %d, want a split into two past %d fields", len(msgs[0]), maxEmbedFields)
	}
	if len(msgs[0][0].Fields) != maxEmbedFields || len(msgs[0][1].Fields) != 1 {
		t.Errorf("field split = %d + %d, want %d + 1",
			len(msgs[0][0].Fields), len(msgs[0][1].Fields), maxEmbedFields)
	}
	if countFields(msgs) != maxEmbedFields+1 {
		t.Errorf("total fields = %d, want %d (none dropped)", countFields(msgs), maxEmbedFields+1)
	}
}

func TestEmbedFieldChunker_SplitsPastTenEmbedsIntoASecondMessage(t *testing.T) {
	s := embedFieldScaffold{title: "T", color: colorNeutral, footer: "F", titleFirstOnly: true}

	// Ten full embeds' worth of fields plus one more field.
	n := maxEmbedFields*maxEmbedsPerMessage + 1
	msgs := chunkN(s, n)

	if len(msgs) != 2 {
		t.Fatalf("messages = %d, want a spill into a second message past %d embeds", len(msgs), maxEmbedsPerMessage)
	}
	if len(msgs[0]) != maxEmbedsPerMessage {
		t.Errorf("first message embeds = %d, want %d", len(msgs[0]), maxEmbedsPerMessage)
	}
	if len(msgs[1]) != 1 || len(msgs[1][0].Fields) != 1 {
		t.Errorf("second message = %+v, want one embed carrying the last field", msgs[1])
	}
	if countFields(msgs) != n {
		t.Errorf("total fields = %d, want %d", countFields(msgs), n)
	}
}

func TestEmbedFieldChunker_SplitsAMessageBeforeTheCharacterCeiling(t *testing.T) {
	// Eight fields, each ~1000 chars: nowhere near the 25-field or 10-embed walls,
	// so any message split is the character budget firing.
	s := embedFieldScaffold{title: "T", color: colorNeutral, footer: "F", titleFirstOnly: true}
	big := strings.Repeat("x", 1000)

	c := newEmbedFieldChunker(s)
	for i := 0; i < 8; i++ {
		c.add(fmt.Sprintf("f%d", i), big)
	}
	c.flushMessage()
	msgs := c.messages

	if len(msgs) < 2 {
		t.Fatalf("messages = %d, want a character-budget split", len(msgs))
	}
	for i, m := range msgs {
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
	if countFields(msgs) != 8 {
		t.Errorf("total fields = %d, want 8 (none dropped in the split)", countFields(msgs))
	}
}

func TestEmbedFieldChunker_TitleFirstOnlyStampsOnlyTheOpeningEmbed(t *testing.T) {
	s := embedFieldScaffold{title: "The Title", color: colorNeutral, footer: "F", titleFirstOnly: true}
	msgs := chunkN(s, maxEmbedFields+1) // two embeds

	if msgs[0][0].Title != "The Title" {
		t.Errorf("first embed title = %q, want it stamped", msgs[0][0].Title)
	}
	if msgs[0][1].Title != "" {
		t.Errorf("second embed title = %q, want it left blank", msgs[0][1].Title)
	}
}

func TestEmbedFieldChunker_TitleOnEveryEmbedWhenNotFirstOnly(t *testing.T) {
	s := embedFieldScaffold{title: "The Title", color: colorNeutral, footer: "F"}
	msgs := chunkN(s, maxEmbedFields+1) // two embeds

	if msgs[0][0].Title != "The Title" || msgs[0][1].Title != "The Title" {
		t.Errorf("titles = %q / %q, want the title on every embed when titleFirstOnly is false",
			msgs[0][0].Title, msgs[0][1].Title)
	}
}

func TestEmbedFieldChunker_CarriesLeagueNameInFooterOrOnAuthorPerScaffold(t *testing.T) {
	// /overview style: league name rides the footer, no author line.
	footerStyle := chunkN(embedFieldScaffold{title: "T", color: colorNeutral, footer: "Coq au Ian", titleFirstOnly: true}, 1)
	e := footerStyle[0][0]
	if e.Footer == nil || e.Footer.Text != "Coq au Ian" {
		t.Errorf("footer = %+v, want the league name", e.Footer)
	}
	if e.Author != nil {
		t.Errorf("author = %+v, want none in the footer-carried style", e.Author)
	}

	// /waivers, /bet style: league name on the author line, view cue in the footer.
	authorStyle := chunkN(embedFieldScaffold{leagueName: "Coq au Ian", title: "T", color: colorNeutral, footer: "accepted", titleFirstOnly: true}, 1)
	e = authorStyle[0][0]
	if e.Author == nil || e.Author.Name != "Coq au Ian" {
		t.Errorf("author = %+v, want the league name", e.Author)
	}
	if e.Footer == nil || e.Footer.Text != "accepted" {
		t.Errorf("footer = %+v, want the view cue", e.Footer)
	}
}

func TestEmbedFieldChunker_EveryEmbedCarriesTheColourBar(t *testing.T) {
	s := embedFieldScaffold{title: "T", color: colorProvisional, footer: "F", titleFirstOnly: true}
	msgs := chunkN(s, maxEmbedFields*2+1) // three embeds

	for _, m := range msgs {
		for _, e := range m {
			if e.Color != colorProvisional {
				t.Errorf("embed colour = %#x, want %#x on every embed", e.Color, colorProvisional)
			}
		}
	}
}
