package bot

import (
	"fmt"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/UberJoe/fpldiscord/internal/fpl"
	"github.com/bwmarrin/discordgo"
)

// waiverSnapBuiltAt is the build time waiverBotSnap carries, so a test can assert
// the embed Timestamp is the snapshot build time.
var waiverSnapBuiltAt = time.Date(2026, 9, 9, 11, 30, 0, 0, time.UTC)

// waiverBotSnap builds a classic snapshot whose processed gameweeks are 3 and
// 4. GW4 holds a contested waiver (Sam wins Salah on priority 1, Joe is
// out-bid), a free-agent pickup, and a trade that must never render. GW3 holds
// one uncontested accepted waiver.
func waiverBotSnap() *fpl.Snapshot {
	return &fpl.Snapshot{
		LeagueName: "FPL Draft 26/27",
		LeagueMode: fpl.ModeClassic,
		CurrentGW:  5,
		BuiltAt:    waiverSnapBuiltAt,
		Game:       fpl.Game{CurrentEvent: 5},
		Bootstrap: fpl.Bootstrap{Elements: []fpl.Element{
			{ID: 20, WebName: "Salah"},
			{ID: 21, WebName: "Núñez"},
			{ID: 22, WebName: "Højlund"},
			{ID: 23, WebName: "Watkins"},
			{ID: 24, WebName: "Saka"},
		}},
		LeagueDetails: fpl.LeagueDetails{
			League: fpl.League{Name: "FPL Draft 26/27", Scoring: "c"},
			LeagueEntries: []fpl.LeagueEntry{
				{ID: 1, EntryID: 500, PlayerFirstName: "Sam"},
				{ID: 2, EntryID: 600, PlayerFirstName: "Joe"},
			},
		},
		Transactions: []fpl.Transaction{
			{Entry: 600, Event: 4, ElementIn: 20, ElementOut: 21, Kind: "w", Result: "do", Priority: 4, Index: 2},
			{Entry: 500, Event: 4, ElementIn: 20, ElementOut: 22, Kind: "w", Result: "a", Priority: 1, Index: 1},
			{Entry: 500, Event: 4, ElementIn: 23, ElementOut: 0, Kind: "f", Result: "a", Priority: 0, Index: 3},
			{Entry: 600, Event: 4, ElementIn: 21, ElementOut: 23, Kind: "t", Result: "a", Priority: 0, Index: 4},
			{Entry: 500, Event: 3, ElementIn: 24, ElementOut: 20, Kind: "w", Result: "a", Priority: 2, Index: 1},
		},
	}
}

// waiversEmbeds runs handleWaivers against a recordingResponder and returns the
// per-message embed slices it emitted, failing the test if the handler sent any
// plain-text message or nothing at all.
func waiversEmbeds(t *testing.T, in *cmdInput) [][]*discordgo.MessageEmbed {
	t.Helper()
	r := &recordingResponder{}
	in.resp = r
	if err := handleWaivers(in); err != nil {
		t.Fatalf("handleWaivers: %v", err)
	}
	if len(r.messages) != 0 {
		t.Fatalf("plain messages = %v, want none — this /waivers reply is an embed", r.messages)
	}
	if len(r.embeds) == 0 {
		t.Fatalf("handler emitted no embed messages")
	}
	return r.embeds
}

// waiversOneEmbed asserts the handler emitted exactly one message carrying one
// embed and returns it — the shape of the accepted view.
func waiversOneEmbed(t *testing.T, in *cmdInput) *discordgo.MessageEmbed {
	t.Helper()
	msgs := waiversEmbeds(t, in)
	if len(msgs) != 1 || len(msgs[0]) != 1 {
		t.Fatalf("embeds = %v, want exactly one message carrying one embed", msgs)
	}
	return msgs[0][0]
}

// allEmbedFields flattens every field of every embed across every message into a
// single "name\nvalue" string for substring assertions.
func allEmbedFields(msgs [][]*discordgo.MessageEmbed) string {
	var b strings.Builder
	for _, m := range msgs {
		for _, e := range m {
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

func countEmbedFields(msgs [][]*discordgo.MessageEmbed) int {
	n := 0
	for _, m := range msgs {
		for _, e := range m {
			n += len(e.Fields)
		}
	}
	return n
}

func TestHandleWaivers_NoArgsShowsLatestProcessedRoundAsOneNeutralEmbed(t *testing.T) {
	e := waiversOneEmbed(t, &cmdInput{snap: waiverBotSnap()})

	if e.Color != colorNeutral {
		t.Errorf("color = %#x, want neutral %#x — a processed round is settled history", e.Color, colorNeutral)
	}
	if e.Title != "GW4 waivers" {
		t.Errorf("title = %q, want %q", e.Title, "GW4 waivers")
	}
	if e.Author == nil || e.Author.Name != "FPL Draft 26/27" {
		t.Errorf("author = %+v, want the league name", e.Author)
	}
	if e.Footer == nil || e.Footer.Text != "accepted" {
		t.Errorf("footer = %+v, want the resolved result mode %q", e.Footer, "accepted")
	}
	if want := waiverSnapBuiltAt.Format(time.RFC3339); e.Timestamp != want {
		t.Errorf("timestamp = %q, want the snapshot build time %q", e.Timestamp, want)
	}

	d := e.Description
	if !strings.HasPrefix(d, "```") || !strings.HasSuffix(d, "```") {
		t.Errorf("description is not a fenced code block:\n%s", d)
	}
	// The accepted waiver and the free-agent pickup show; the out-bid claim and
	// the trade do not.
	for _, want := range []string{"Sam", "Salah", "Watkins", "(free agent)"} {
		if !strings.Contains(d, want) {
			t.Errorf("description missing %q:\n%s", want, d)
		}
	}
	if strings.Contains(d, "Joe") {
		t.Errorf("out-bid (failed) claim should not appear under accepted:\n%s", d)
	}
}

func TestHandleWaivers_GWArgSelectsRound(t *testing.T) {
	e := waiversOneEmbed(t, &cmdInput{
		snap: waiverBotSnap(),
		opts: cmdOptions{"gw": float64(3)},
	})
	if e.Title != "GW3 waivers" {
		t.Errorf("title = %q, want %q", e.Title, "GW3 waivers")
	}
	if !strings.Contains(e.Description, "Saka") {
		t.Errorf("GW3 round not rendered:\n%s", e.Description)
	}
}

func TestHandleWaivers_FailedShowsOneFieldPerContestedPlayerWinnerFirst(t *testing.T) {
	msgs := waiversEmbeds(t, &cmdInput{
		snap: waiverBotSnap(),
		opts: cmdOptions{"result": "failed"},
	})

	first := msgs[0][0]
	if first.Color != colorNeutral || first.Title != "GW4 waivers" {
		t.Errorf("scaffold = color %#x / title %q, want neutral / %q", first.Color, first.Title, "GW4 waivers")
	}
	if first.Footer == nil || first.Footer.Text != "failed" {
		t.Errorf("footer = %+v, want %q", first.Footer, "failed")
	}

	// One contested group -> one field, named for the incoming player.
	if got := countEmbedFields(msgs); got != 1 {
		t.Fatalf("fields = %d, want 1 contested group", got)
	}
	f := msgs[0][0].Fields[0]
	if f.Inline {
		t.Errorf("contested field is inline, want a full-width block")
	}
	if f.Name != "Salah" {
		t.Errorf("field name = %q, want the incoming player %q", f.Name, "Salah")
	}
	// Winner (Sam, priority 1) leads the out-bid manager (Joe, priority 4).
	if strings.Index(f.Value, "Sam") > strings.Index(f.Value, "Joe") {
		t.Errorf("out-bid chain not winner-first:\n%s", f.Value)
	}
	for _, want := range []string{"#1", "#4", "won", "out-bid"} {
		if !strings.Contains(f.Value, want) {
			t.Errorf("bid chain missing %q:\n%s", want, f.Value)
		}
	}
	// The uncontested free-agent pickup has no failed row -> not in failed view.
	if strings.Contains(allEmbedFields(msgs), "Watkins") {
		t.Errorf("uncontested claim leaked into the failed view:\n%s", allEmbedFields(msgs))
	}
}

func TestHandleWaivers_AllKeepsUncontestedGroup(t *testing.T) {
	msgs := waiversEmbeds(t, &cmdInput{
		snap: waiverBotSnap(),
		opts: cmdOptions{"result": "all"},
	})
	body := allEmbedFields(msgs)
	// all mode keeps the single-claim free-agent group that failed mode drops.
	if !strings.Contains(body, "Watkins") {
		t.Errorf("all view dropped the uncontested group:\n%s", body)
	}
}

// Two different players can share a web_name in one round. Grouping the out-bid
// chain must key on the element id, not the name, or the two collapse into one
// bogus field.
func TestHandleWaivers_SameNamedPlayersDoNotMergeIntoOneField(t *testing.T) {
	snap := waiverBotSnap()
	snap.Bootstrap.Elements = []fpl.Element{
		{ID: 30, WebName: "Silva"},
		{ID: 31, WebName: "Silva"},
	}
	snap.Transactions = []fpl.Transaction{
		{Entry: 500, Event: 4, ElementIn: 30, Kind: "w", Result: "a", Priority: 1, Index: 1},
		{Entry: 600, Event: 4, ElementIn: 30, Kind: "w", Result: "do", Priority: 3, Index: 2},
		{Entry: 600, Event: 4, ElementIn: 31, Kind: "w", Result: "a", Priority: 2, Index: 3},
		{Entry: 500, Event: 4, ElementIn: 31, Kind: "w", Result: "do", Priority: 5, Index: 4},
	}

	msgs := waiversEmbeds(t, &cmdInput{snap: snap, opts: cmdOptions{"result": "all"}})

	// Two separate contested groups, each a "Silva" field, each with its own
	// winner + out-bid pair.
	if got := countEmbedFields(msgs); got != 2 {
		t.Fatalf("fields = %d, want 2 (one per element id)", got)
	}
	body := allEmbedFields(msgs)
	if strings.Count(body, "won") != 2 || strings.Count(body, "out-bid") != 2 {
		t.Errorf("each Silva group should have one winner and one out-bid claim:\n%s", body)
	}
}

func TestRenderWaiversContested_SplitsPastTwentyFiveGroupsIntoASecondEmbed(t *testing.T) {
	groups := manyContestedGroups(26)
	msgs := renderWaiversContested("FPL Draft 26/27", waiverSnapBuiltAt, 4, "all", groups)

	if len(msgs) != 1 {
		t.Fatalf("messages = %d, want the 26 groups to still fit one message", len(msgs))
	}
	if len(msgs[0]) != 2 || len(msgs[0][0].Fields) != maxEmbedFields || len(msgs[0][1].Fields) != 1 {
		t.Errorf("field split = %d embeds (%d + ...), want 25 + 1", len(msgs[0]), len(msgs[0][0].Fields))
	}
	if countEmbedFields(msgs) != 26 {
		t.Errorf("total fields = %d, want 26 — no group dropped in the split", countEmbedFields(msgs))
	}
	// Only the first embed of the reply is titled.
	if msgs[0][0].Title == "" || msgs[0][1].Title != "" {
		t.Errorf("titles = %q / %q, want the first embed titled and the rest bare", msgs[0][0].Title, msgs[0][1].Title)
	}
}

func TestRenderWaiversContested_SplitsPastTenEmbedsIntoASecondMessage(t *testing.T) {
	groups := manyContestedGroups(251)
	msgs := renderWaiversContested("FPL Draft 26/27", waiverSnapBuiltAt, 4, "all", groups)

	if len(msgs) < 2 {
		t.Fatalf("messages = %d, want 251 groups to spill past one message", len(msgs))
	}
	for i, m := range msgs {
		if len(m) > maxEmbedsPerMessage {
			t.Errorf("message %d carries %d embeds, over the %d cap", i, len(m), maxEmbedsPerMessage)
		}
	}
	if countEmbedFields(msgs) != 251 {
		t.Errorf("total fields = %d, want 251 — no group truncated", countEmbedFields(msgs))
	}
}

// manyContestedGroups builds n two-claim contested groups (a winner and an
// out-bid claim) with distinct incoming players, for the chunk-boundary tests.
func manyContestedGroups(n int) [][]fpl.WaiverRow {
	groups := make([][]fpl.WaiverRow, n)
	for i := range groups {
		name := "Contested" + string(rune('A'+i%26)) + strings.Repeat("x", 1+i/26)
		groups[i] = []fpl.WaiverRow{
			{OwnerName: "Sam", In: name, Type: fpl.WaiverTypeWaiver, Status: fpl.WaiverStatusAccepted, Priority: 1},
			{OwnerName: "Joe", In: name, Type: fpl.WaiverTypeWaiver, Status: fpl.WaiverStatusFailed, Priority: 2},
		}
	}
	return groups
}

// --- pure accepted-table helper -------------------------------------------------

func TestWaiverAcceptedTable_AlignsColumns(t *testing.T) {
	rows := []fpl.WaiverRow{
		{OwnerName: "Sam", In: "Salah", Out: "Højlund", Type: fpl.WaiverTypeWaiver, Status: fpl.WaiverStatusAccepted},
		{OwnerName: "Christopherlongname", In: "Watkins", Type: fpl.WaiverTypeFreeAgent, Status: fpl.WaiverStatusAccepted},
	}
	body := waiverAcceptedTable(rows)
	lines := strings.Split(body, "\n")
	if len(lines) != 3 {
		t.Fatalf("lines = %d (%q), want a header + 2 rows", len(lines), lines)
	}

	// The owner column is capped for mobile width — the long name is truncated.
	if strings.Contains(body, "Christopherlongname") {
		t.Errorf("owner column was not capped:\n%s", body)
	}
	ownerW := runeIndex(lines[0], "Move") - 2 // two-space gutter before the Move column
	for _, ln := range lines[1:] {
		owner := strings.TrimRight(string([]rune(ln)[:ownerW]), " ")
		if utf8.RuneCountInString(owner) > waiverOwnerCap {
			t.Errorf("owner cell %q is %d runes, over the %d cap", owner, utf8.RuneCountInString(owner), waiverOwnerCap)
		}
	}

	// The move column starts at the same rune offset on every line.
	want := runeIndex(lines[0], "Move")
	if got := runeIndex(lines[1], "Højlund"); got != want {
		t.Errorf("row 1 move column at rune %d, header has it at %d:\n%s", got, want, body)
	}
	if got := runeIndex(lines[2], "Watkins"); got != want {
		t.Errorf("row 2 move column at rune %d, header has it at %d:\n%s", got, want, body)
	}
	// The kind tag lands only against the free-agent pickup.
	if strings.Contains(lines[1], "(free agent)") || !strings.Contains(lines[2], "(free agent)") {
		t.Errorf("kind column misplaced:\n%s", body)
	}
}

func TestWaiverAcceptedTable_OverflowKeepsWholeRowsAndSummarises(t *testing.T) {
	const total = 400
	rows := make([]fpl.WaiverRow, total)
	for i := range rows {
		rows[i] = fpl.WaiverRow{
			OwnerName: "Owner",
			In:        "IncomingPlayer",
			Out:       "OutgoingPlayer",
			Type:      fpl.WaiverTypeWaiver,
			Status:    fpl.WaiverStatusAccepted,
		}
	}

	body := waiverAcceptedTable(rows)

	if n := utf8.RuneCountInString(body); n > 4096 {
		t.Errorf("table body is %d runes, over Discord's 4096 description limit", n)
	}
	lines := strings.Split(body, "\n")
	last := lines[len(lines)-1]
	if !strings.HasPrefix(last, "…and ") || !strings.HasSuffix(last, " more claims") {
		t.Fatalf("last line = %q, want a \"…and N more claims\" summary", last)
	}

	// header + kept rows + summary; every kept row is whole (carries the move).
	kept := len(lines) - 2
	for _, ln := range lines[1 : 1+kept] {
		if !strings.Contains(ln, "OutgoingPlayer -> IncomingPlayer") {
			t.Errorf("kept row is not whole: %q", ln)
		}
	}
	var more int
	if _, err := fmt.Sscanf(last, "…and %d more claims", &more); err != nil {
		t.Fatalf("cannot parse summary %q: %v", last, err)
	}
	if kept+more != total {
		t.Errorf("kept %d + summary %d = %d, want %d", kept, more, kept+more, total)
	}
}

func TestWaiverAcceptedTable_ShortRoundHasNoSummary(t *testing.T) {
	body := waiverAcceptedTable([]fpl.WaiverRow{
		{OwnerName: "Sam", In: "Salah", Type: fpl.WaiverTypeWaiver, Status: fpl.WaiverStatusAccepted},
	})
	if strings.Contains(body, "more claims") {
		t.Errorf("a one-row table should carry no overflow summary:\n%s", body)
	}
}

// --- plain-text replies -------------------------------------------------------

func TestHandleWaivers_ShortRepliesStayPlainText(t *testing.T) {
	cases := []struct {
		name string
		in   *cmdInput
		want string
	}{
		{"nil snapshot", &cmdInput{snap: nil}, "starting up"},
		{"no processed rounds", func() *cmdInput {
			s := waiverBotSnap()
			s.Transactions = nil
			s.Game = fpl.Game{CurrentEvent: 5}
			return &cmdInput{snap: s}
		}(), "No waiver rounds have been processed"},
		{"bad result value", &cmdInput{snap: waiverBotSnap(), opts: cmdOptions{"result": "sideways"}}, "must be accepted"},
		{"unknown gameweek", &cmdInput{snap: waiverBotSnap(), opts: cmdOptions{"gw": float64(20)}}, "GW20"},
		{"empty result set", func() *cmdInput {
			s := waiverBotSnap()
			// GW3 has only an accepted waiver, so failed mode yields nothing.
			return &cmdInput{snap: s, opts: cmdOptions{"gw": float64(3), "result": "failed"}}
		}(), "No failed waiver claims in GW3"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := &recordingResponder{}
			tc.in.resp = r
			if err := handleWaivers(tc.in); err != nil {
				t.Fatalf("handleWaivers: %v", err)
			}
			if len(r.embeds) != 0 {
				t.Fatalf("emitted an embed, want plain text only: %v", r.embeds)
			}
			if len(r.messages) != 1 || !strings.Contains(r.messages[0], tc.want) {
				t.Errorf("reply = %v, want a plain message containing %q", r.messages, tc.want)
			}
		})
	}
}

// runeIndex is the rune offset of the first occurrence of sub in s, or -1.
func runeIndex(s, sub string) int {
	b := strings.Index(s, sub)
	if b < 0 {
		return -1
	}
	return utf8.RuneCountInString(s[:b])
}
