package bot

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/UberJoe/fpldiscord/internal/fpl"
	"github.com/UberJoe/fpldiscord/internal/store"
	"github.com/bwmarrin/discordgo"
)

// --- fakes -----------------------------------------------------------------

type setCall struct {
	season string
	user   string
	ids    [4]int
}

type addCall struct {
	season string
	name   string
	picks  [4]store.ArchivePick
}

// fakeBetStore records writes and serves canned reads.
type fakeBetStore struct {
	current  []store.BettorPicks
	archives map[string][]store.ArchivedBettor
	seasons  []string
	sets     []setCall
	adds     []addCall
}

func (f *fakeBetStore) CurrentPicks(string) ([]store.BettorPicks, error) { return f.current, nil }
func (f *fakeBetStore) SetPicks(season, user string, ids [4]int) error {
	f.sets = append(f.sets, setCall{season, user, ids})
	return nil
}
func (f *fakeBetStore) ArchivedSeasons() ([]string, error) { return f.seasons, nil }
func (f *fakeBetStore) Archive(season string) ([]store.ArchivedBettor, error) {
	return f.archives[season], nil
}
func (f *fakeBetStore) AddArchive(season, name string, picks [4]store.ArchivePick) error {
	f.adds = append(f.adds, addCall{season, name, picks})
	return nil
}

type fakeNamer map[string]string

func (f fakeNamer) MemberName(id string) (string, bool) {
	n, ok := f[id]
	return n, ok
}

// betSnapBuiltAt is the build time betBotSnap carries, so a test can assert the
// live-board embed Timestamp is the snapshot build time (and the archived view
// carries none).
var betSnapBuiltAt = time.Date(2026, 9, 10, 8, 0, 0, 0, time.UTC)

// betBotSnap is a snapshot with ten players (fixed season goals) and the
// matching autocomplete slice, enough to drive the bet leaderboard and the
// name->id resolution in /bet set.
func betBotSnap() *fpl.Snapshot {
	els := []fpl.Element{
		{ID: 1, WebName: "Alpha", GoalsScored: 5},
		{ID: 2, WebName: "Bravo", GoalsScored: 5},
		{ID: 3, WebName: "Charlie", GoalsScored: 5},
		{ID: 4, WebName: "Delta", GoalsScored: 5},
		{ID: 5, WebName: "Echo", GoalsScored: 9},
		{ID: 6, WebName: "Foxtrot", GoalsScored: 9},
		{ID: 7, WebName: "Golf", GoalsScored: 9},
		{ID: 8, WebName: "Hotel", GoalsScored: 1},
		{ID: 9, WebName: "India", GoalsScored: 6},
		{ID: 10, WebName: "Juliet", GoalsScored: 0},
	}
	pn := make([]fpl.PlayerName, len(els))
	for i, e := range els {
		pn[i] = fpl.PlayerName{ID: e.ID, WebName: e.WebName, Stripped: e.WebName}
	}
	return &fpl.Snapshot{
		LeagueName:  "FPL Draft",
		LeagueMode:  fpl.ModeClassic,
		CurrentGW:   5,
		BuiltAt:     betSnapBuiltAt,
		Game:        fpl.Game{CurrentEvent: 5},
		Bootstrap:   fpl.Bootstrap{Elements: els},
		PlayerNames: pn,
	}
}

// betEmbeds runs handleBet against a recordingResponder and returns the
// per-message embed slices it emitted, failing if the handler sent any
// plain-text message or nothing at all.
func betEmbeds(t *testing.T, in *cmdInput) [][]*discordgo.MessageEmbed {
	t.Helper()
	r := &recordingResponder{}
	in.resp = r
	if err := handleBet(in); err != nil {
		t.Fatalf("handleBet: %v", err)
	}
	if len(r.messages) != 0 {
		t.Fatalf("plain messages = %v, want none — this /bet reply is an embed", r.messages)
	}
	if len(r.embeds) == 0 {
		t.Fatalf("handler emitted no embed messages")
	}
	return r.embeds
}

// betOneEmbed asserts the handler emitted exactly one message carrying one
// embed and returns it.
func betOneEmbed(t *testing.T, in *cmdInput) *discordgo.MessageEmbed {
	t.Helper()
	msgs := betEmbeds(t, in)
	if len(msgs) != 1 || len(msgs[0]) != 1 {
		t.Fatalf("embeds = %v, want exactly one message carrying one embed", msgs)
	}
	return msgs[0][0]
}

func liveBetPicks() []store.BettorPicks {
	return []store.BettorPicks{
		{DiscordUserID: "u-alice", Elements: [4]int{1, 2, 3, 4}},  // 20 -> in, leader
		{DiscordUserID: "u-bob", Elements: [4]int{5, 6, 7, 8}},    // 28 -> bust
		{DiscordUserID: "u-carol", Elements: [4]int{9, 1, 2, 10}}, // 16, one on zero -> provisionallyOut
	}
}

func betNamer() fakeNamer {
	return fakeNamer{"u-alice": "Alice", "u-bob": "Bob", "u-carol": "Carol"}
}

// --- /bet show (live) -----------------------------------------------------

func TestHandleBet_ShowLiveRendersStatusesAndLeader(t *testing.T) {
	e := betOneEmbed(t, &cmdInput{
		snap:     betBotSnap(),
		opts:     cmdOptions{},
		sub:      "show",
		betStore: &fakeBetStore{current: liveBetPicks()},
		season:   "2026/27",
		isAdmin:  func(string) bool { return false },
		namer:    betNamer(),
	})

	if e.Title != "Bet leaderboard — 2026/27" {
		t.Errorf("title = %q, want %q", e.Title, "Bet leaderboard — 2026/27")
	}
	if e.Author == nil || e.Author.Name != "FPL Draft" {
		t.Errorf("author = %+v, want the league name", e.Author)
	}
	if want := betSnapBuiltAt.Format(time.RFC3339); e.Timestamp != want {
		t.Errorf("timestamp = %q, want the snapshot build time %q", e.Timestamp, want)
	}
	// One non-inline field per bettor.
	if len(e.Fields) != 3 {
		t.Fatalf("fields = %d, want one per bettor (3)", len(e.Fields))
	}
	for _, f := range e.Fields {
		if f.Inline {
			t.Errorf("bettor field %q is inline, want a full-width block", f.Name)
		}
	}

	body := allEmbedFields([][]*discordgo.MessageEmbed{{e}})
	for _, want := range []string{"Alice", "Bob", "Carol", "Alpha (5)", "Juliet (0)", "🕓", "💥", "(leading)"} {
		if !strings.Contains(body, want) {
			t.Errorf("rendered board missing %q:\n%s", want, body)
		}
	}
	// Not settled yet, so no trophy.
	if strings.Contains(body, "🏆") {
		t.Errorf("trophy shown before season complete:\n%s", body)
	}
	// Field order: Alice (20) before Carol (16) before Bob (bust).
	if !(strings.Index(body, "Alice") < strings.Index(body, "Carol") && strings.Index(body, "Carol") < strings.Index(body, "Bob")) {
		t.Errorf("rows not closest-to-21 with bust last:\n%s", body)
	}
}

func TestHandleBet_ShowLiveColourAndFooterAreProvisionalWhileSeasonRuns(t *testing.T) {
	e := betOneEmbed(t, &cmdInput{
		snap: betBotSnap(), opts: cmdOptions{}, sub: "show",
		betStore: &fakeBetStore{current: liveBetPicks()},
		season:   "2026/27",
		isAdmin:  func(string) bool { return false },
		namer:    betNamer(),
	})
	if e.Color != colorProvisional {
		t.Errorf("color = %#x, want provisional %#x while the season runs", e.Color, colorProvisional)
	}
	if e.Footer == nil || !strings.Contains(e.Footer.Text, "2026/27") || !strings.Contains(e.Footer.Text, "provisional") {
		t.Errorf("footer = %+v, want the season and the provisional phase", e.Footer)
	}
}

func TestHandleBet_ShowStampsTrophyOnceSeasonComplete(t *testing.T) {
	snap := betBotSnap()
	snap.Game = fpl.Game{CurrentEvent: 38, CurrentEventFinished: true}

	e := betOneEmbed(t, &cmdInput{
		snap: snap, opts: cmdOptions{}, sub: "show",
		betStore: &fakeBetStore{current: liveBetPicks()},
		season:   "2026/27",
		isAdmin:  func(string) bool { return false },
		namer:    betNamer(),
	})
	body := allEmbedFields([][]*discordgo.MessageEmbed{{e}})
	if !strings.Contains(body, "🏆") || strings.Contains(body, "(leading)") {
		t.Errorf("want trophy, not (leading), once complete:\n%s", body)
	}
	if e.Color != colorFinal {
		t.Errorf("color = %#x, want final %#x once complete", e.Color, colorFinal)
	}
	if e.Footer == nil || e.Footer.Text != "2026/27 · final" {
		t.Errorf("footer = %+v, want %q", e.Footer, "2026/27 · final")
	}
}

func TestHandleBet_ShowNoPicksExplains(t *testing.T) {
	r := &recordingResponder{}
	in := &cmdInput{
		snap: betBotSnap(), opts: cmdOptions{}, sub: "", resp: r,
		betStore: &fakeBetStore{},
		season:   "2026/27",
		isAdmin:  func(string) bool { return false },
	}
	if err := handleBet(in); err != nil {
		t.Fatalf("handleBet: %v", err)
	}
	if len(r.embeds) != 0 {
		t.Fatalf("emitted an embed, want plain text only: %v", r.embeds)
	}
	if len(r.messages) != 1 || !strings.Contains(r.messages[0], "/bet set") {
		t.Errorf("empty-board reply = %v", r.messages)
	}
}

func TestHandleBet_ShowNoSnapshotReportsStartingUp(t *testing.T) {
	r := &recordingResponder{}
	in := &cmdInput{
		snap: nil, opts: cmdOptions{}, sub: "show", resp: r,
		betStore: &fakeBetStore{current: liveBetPicks()},
		season:   "2026/27",
	}
	if err := handleBet(in); err != nil {
		t.Fatalf("handleBet: %v", err)
	}
	if len(r.embeds) != 0 {
		t.Fatalf("emitted an embed, want plain text only: %v", r.embeds)
	}
	if len(r.messages) != 1 || !strings.Contains(strings.ToLower(r.messages[0]), "starting up") {
		t.Errorf("no-snapshot reply = %v", r.messages)
	}
}

// --- /bet show <season> (archived) --------------------------------------

func TestHandleBet_ShowArchivedSeasonIsStatic(t *testing.T) {
	fbs := &fakeBetStore{
		seasons: []string{"2024/25", "2025/26"},
		archives: map[string][]store.ArchivedBettor{
			"2025/26": {
				{Name: "Zoe", Picks: [4]store.ArchivePick{
					{PlayerName: "Salah", FinalGoals: 5}, {PlayerName: "Isak", FinalGoals: 5},
					{PlayerName: "Palmer", FinalGoals: 5}, {PlayerName: "Saka", FinalGoals: 5},
				}},
			},
		},
	}
	e := betOneEmbed(t, &cmdInput{
		snap: betBotSnap(), opts: cmdOptions{"season": "2025/26"}, sub: "show",
		betStore: fbs, season: "2026/27",
	})

	if e.Title != "Bet — 2025/26 (archived)" {
		t.Errorf("title = %q, want %q", e.Title, "Bet — 2025/26 (archived)")
	}
	if e.Color != colorFinal {
		t.Errorf("color = %#x, want final %#x — a frozen record", e.Color, colorFinal)
	}
	if e.Footer == nil || e.Footer.Text != "2025/26 · archived" {
		t.Errorf("footer = %+v, want %q", e.Footer, "2025/26 · archived")
	}
	if e.Timestamp != "" {
		t.Errorf("timestamp = %q, want none — no snapshot bears on a frozen record", e.Timestamp)
	}
	body := allEmbedFields([][]*discordgo.MessageEmbed{{e}})
	for _, want := range []string{"Zoe", "Salah (5)", "20"} {
		if !strings.Contains(body, want) {
			t.Errorf("archived view missing %q:\n%s", want, body)
		}
	}
}

func TestHandleBet_ShowArchivedKeepsClosestTo21WithBustLast(t *testing.T) {
	fbs := &fakeBetStore{
		archives: map[string][]store.ArchivedBettor{
			"2025/26": {
				{Name: "Busty", Picks: [4]store.ArchivePick{
					{PlayerName: "A", FinalGoals: 10}, {PlayerName: "B", FinalGoals: 10},
					{PlayerName: "C", FinalGoals: 10}, {PlayerName: "D", FinalGoals: 10}, // 40 -> bust
				}},
				{Name: "Faraway", Picks: [4]store.ArchivePick{
					{PlayerName: "A", FinalGoals: 2}, {PlayerName: "B", FinalGoals: 2},
					{PlayerName: "C", FinalGoals: 2}, {PlayerName: "D", FinalGoals: 2}, // 8
				}},
				{Name: "Closest", Picks: [4]store.ArchivePick{
					{PlayerName: "A", FinalGoals: 5}, {PlayerName: "B", FinalGoals: 5},
					{PlayerName: "C", FinalGoals: 5}, {PlayerName: "D", FinalGoals: 5}, // 20
				}},
			},
		},
	}
	e := betOneEmbed(t, &cmdInput{
		snap: betBotSnap(), opts: cmdOptions{"season": "2025/26"}, sub: "show",
		betStore: fbs, season: "2026/27",
	})
	body := allEmbedFields([][]*discordgo.MessageEmbed{{e}})
	// Closest (20) before Faraway (8) before Busty (over 21, last).
	if !(strings.Index(body, "Closest") < strings.Index(body, "Faraway") && strings.Index(body, "Faraway") < strings.Index(body, "Busty")) {
		t.Errorf("archived rows not closest-to-21 with bust last:\n%s", body)
	}
	// The bust total keeps its marker.
	if !strings.Contains(e.Fields[2].Name, "💥") {
		t.Errorf("bust row lost its 💥 marker: %q", e.Fields[2].Name)
	}
}

func TestHandleBet_ShowUnknownArchivedSeasonListsAvailable(t *testing.T) {
	fbs := &fakeBetStore{seasons: []string{"2024/25", "2025/26"}}
	r := &recordingResponder{}
	in := &cmdInput{
		snap: betBotSnap(), opts: cmdOptions{"season": "1999/00"}, sub: "show", resp: r,
		betStore: fbs, season: "2026/27",
	}
	if err := handleBet(in); err != nil {
		t.Fatalf("handleBet: %v", err)
	}
	if len(r.embeds) != 0 {
		t.Fatalf("emitted an embed, want plain text only: %v", r.embeds)
	}
	msg := r.messages[0]
	if !strings.Contains(msg, "No archived record") || !strings.Contains(msg, "2024/25") {
		t.Errorf("unknown-season reply = %q", msg)
	}
}

// --- /bet set ----------------------------------------------------------

func setInput(r Responder, fbs *fakeBetStore, caller string, admin bool, opts cmdOptions) *cmdInput {
	return &cmdInput{
		snap: betBotSnap(), opts: opts, sub: "set", caller: caller, resp: r,
		betStore: fbs, season: "2026/27",
		isAdmin: func(id string) bool { return admin && id == caller },
	}
}

func TestHandleBet_SetRejectsNonAdmin(t *testing.T) {
	fbs := &fakeBetStore{}
	r := &recordingResponder{}
	in := setInput(r, fbs, "rando", false, cmdOptions{
		"bettor": "target-9", "p1": "Alpha", "p2": "Bravo", "p3": "Charlie", "p4": "Delta",
	})
	if err := handleBet(in); err != nil {
		t.Fatalf("handleBet: %v", err)
	}
	if len(r.messages) != 1 || !strings.Contains(strings.ToLower(r.messages[0]), "admin") {
		t.Errorf("non-admin reply = %v", r.messages)
	}
	if len(fbs.sets) != 0 {
		t.Errorf("SetPicks called for a non-admin: %+v", fbs.sets)
	}
}

func TestHandleBet_SetRoundTripsForAdmin(t *testing.T) {
	fbs := &fakeBetStore{}
	r := &recordingResponder{}
	in := setInput(r, fbs, "admin-1", true, cmdOptions{
		"bettor": "target-9", "p1": "Alpha", "p2": "bravo", "p3": "Charlie", "p4": "Delta",
	})
	if err := handleBet(in); err != nil {
		t.Fatalf("handleBet: %v", err)
	}
	if len(fbs.sets) != 1 {
		t.Fatalf("SetPicks calls = %d, want 1", len(fbs.sets))
	}
	got := fbs.sets[0]
	if got.season != "2026/27" || got.user != "target-9" || got.ids != [4]int{1, 2, 3, 4} {
		t.Errorf("SetPicks(%+v), want season 2026/27 / target-9 / [1 2 3 4]", got)
	}
	if !strings.Contains(r.messages[0], "<@target-9>") {
		t.Errorf("confirmation missing bettor mention: %q", r.messages[0])
	}
}

func TestHandleBet_SetReportsUnmatchedPlayers(t *testing.T) {
	fbs := &fakeBetStore{}
	r := &recordingResponder{}
	in := setInput(r, fbs, "admin-1", true, cmdOptions{
		"bettor": "target-9", "p1": "Alpha", "p2": "Bravo", "p3": "Charlie", "p4": "Nobody",
	})
	if err := handleBet(in); err != nil {
		t.Fatalf("handleBet: %v", err)
	}
	if len(fbs.sets) != 0 {
		t.Errorf("SetPicks called despite an unmatched player")
	}
	if !strings.Contains(r.messages[0], "Nobody") {
		t.Errorf("reply should name the unmatched player: %q", r.messages[0])
	}
}

func TestHandleBet_SetRejectsDuplicatePicks(t *testing.T) {
	fbs := &fakeBetStore{}
	r := &recordingResponder{}
	in := setInput(r, fbs, "admin-1", true, cmdOptions{
		"bettor": "t", "p1": "Alpha", "p2": "Alpha", "p3": "Bravo", "p4": "Charlie",
	})
	if err := handleBet(in); err != nil {
		t.Fatalf("handleBet: %v", err)
	}
	if len(fbs.sets) != 0 || !strings.Contains(strings.ToLower(r.messages[0]), "different") {
		t.Errorf("duplicate picks not rejected: sets=%+v reply=%q", fbs.sets, r.messages[0])
	}
}

// --- /bet archive ----------------------------------------------------

func archiveInput(r Responder, fbs *fakeBetStore, caller string, admin bool, opts cmdOptions) *cmdInput {
	return &cmdInput{
		opts: opts, sub: "archive", caller: caller, resp: r,
		betStore: fbs, season: "2026/27",
		isAdmin: func(id string) bool { return admin && id == caller },
	}
}

func TestHandleBet_ArchiveRejectsNonAdmin(t *testing.T) {
	fbs := &fakeBetStore{}
	r := &recordingResponder{}
	in := archiveInput(r, fbs, "rando", false, cmdOptions{
		"season": "2024/25", "bettor_name": "Old Mate", "entries": "A:1, B:2, C:3, D:4",
	})
	if err := handleBet(in); err != nil {
		t.Fatalf("handleBet: %v", err)
	}
	if len(fbs.adds) != 0 || !strings.Contains(strings.ToLower(r.messages[0]), "admin") {
		t.Errorf("non-admin archive not refused: adds=%+v reply=%q", fbs.adds, r.messages[0])
	}
}

func TestHandleBet_ArchiveRoundTripsForAdmin(t *testing.T) {
	fbs := &fakeBetStore{}
	r := &recordingResponder{}
	in := archiveInput(r, fbs, "admin-1", true, cmdOptions{
		"season": "2024/25", "bettor_name": "Old Mate",
		"entries": "Salah:19, Isak:23, Haaland:27, Palmer:15",
	})
	if err := handleBet(in); err != nil {
		t.Fatalf("handleBet: %v", err)
	}
	if len(fbs.adds) != 1 {
		t.Fatalf("AddArchive calls = %d, want 1", len(fbs.adds))
	}
	got := fbs.adds[0]
	if got.season != "2024/25" || got.name != "Old Mate" {
		t.Errorf("AddArchive key = %q / %q", got.season, got.name)
	}
	want := [4]store.ArchivePick{
		{PlayerName: "Salah", FinalGoals: 19}, {PlayerName: "Isak", FinalGoals: 23},
		{PlayerName: "Haaland", FinalGoals: 27}, {PlayerName: "Palmer", FinalGoals: 15},
	}
	if got.picks != want {
		t.Errorf("AddArchive picks = %+v, want %+v", got.picks, want)
	}
	if !strings.Contains(r.messages[0], "84") {
		t.Errorf("confirmation should carry the total 84: %q", r.messages[0])
	}
}

func TestHandleBet_ArchiveRejectsMalformedEntries(t *testing.T) {
	fbs := &fakeBetStore{}
	for _, entries := range []string{
		"Salah:19, Isak:23",                      // too few
		"Salah:19, Isak:23, Haaland:27, Palmer",  // missing :goals
		"Salah:19, Isak:x, Haaland:27, Palmer:1", // non-numeric
	} {
		r := &recordingResponder{}
		in := archiveInput(r, fbs, "admin-1", true, cmdOptions{
			"season": "2024/25", "bettor_name": "Old Mate", "entries": entries,
		})
		if err := handleBet(in); err != nil {
			t.Fatalf("handleBet(%q): %v", entries, err)
		}
		if len(fbs.adds) != 0 {
			t.Fatalf("AddArchive called for malformed entries %q", entries)
		}
		if len(r.messages) != 1 {
			t.Fatalf("entries %q: messages = %v", entries, r.messages)
		}
	}
}

// --- wiring ---------------------------------------------------------

func TestHandleBet_IsRegistered(t *testing.T) {
	b, err := newTestBot()
	if err != nil {
		t.Fatalf("newTestBot: %v", err)
	}
	if _, ok := b.handlers["bet"]; !ok {
		t.Error("no handler registered for /bet")
	}
	if _, ok := b.autocomplete["bet"]; !ok {
		t.Error("no autocomplete resolver registered for /bet")
	}
}

func TestCommandSpecs_BetSubcommandsAndAutocompletingPicks(t *testing.T) {
	var betCmd *discordgo.ApplicationCommand
	for _, s := range commandSpecs() {
		if s.Name == "bet" {
			betCmd = s
		}
	}
	if betCmd == nil {
		t.Fatal("commandSpecs() has no bet command")
	}

	subs := map[string]*discordgo.ApplicationCommandOption{}
	for _, o := range betCmd.Options {
		if o.Type != discordgo.ApplicationCommandOptionSubCommand {
			t.Fatalf("bet option %q is not a subcommand", o.Name)
		}
		subs[o.Name] = o
	}
	for _, want := range []string{"show", "set", "archive"} {
		if subs[want] == nil {
			t.Errorf("bet is missing the %q subcommand", want)
		}
	}

	set := subs["set"]
	var bettorType discordgo.ApplicationCommandOptionType
	picks := 0
	for _, o := range set.Options {
		switch o.Name {
		case "bettor":
			bettorType = o.Type
		case "p1", "p2", "p3", "p4":
			picks++
			if !o.Autocomplete {
				t.Errorf("bet set.%s is missing Autocomplete=true", o.Name)
			}
		}
	}
	if bettorType != discordgo.ApplicationCommandOptionUser {
		t.Errorf("bet set.bettor type = %v, want User", bettorType)
	}
	if picks != 4 {
		t.Errorf("bet set has %d player args, want 4", picks)
	}
}

func TestFocusedOption_DescendsIntoSubcommand(t *testing.T) {
	opts := []*discordgo.ApplicationCommandInteractionDataOption{
		{
			Name: "set",
			Type: discordgo.ApplicationCommandOptionSubCommand,
			Options: []*discordgo.ApplicationCommandInteractionDataOption{
				{Name: "bettor", Value: "u1"},
				{Name: "p2", Value: "haa", Focused: true},
			},
		},
	}
	name, partial := focusedOption(opts)
	if name != "p2" || partial != "haa" {
		t.Errorf("focusedOption = %q / %q, want p2 / haa", name, partial)
	}
}

func TestAutocompleteBet_MatchesPlayers(t *testing.T) {
	choices := autocompletePlayer(betBotSnap(), "p1", "alp")
	if len(choices) == 0 || choices[0].Name != "Alpha" {
		t.Errorf("autocomplete for 'alp' = %v, want Alpha first", choices)
	}
}

func TestHandleBet_ShowSplitsPast25BettorsAcrossEmbeds(t *testing.T) {
	var picks []store.BettorPicks
	namer := fakeNamer{}
	for i := 0; i < 26; i++ {
		uid := fmt.Sprintf("u-%02d", i)
		picks = append(picks, store.BettorPicks{DiscordUserID: uid, Elements: [4]int{1, 2, 3, 4}})
		namer[uid] = fmt.Sprintf("Bettor%02d", i)
	}

	msgs := betEmbeds(t, &cmdInput{
		snap: betBotSnap(), opts: cmdOptions{}, sub: "show",
		betStore: &fakeBetStore{current: picks}, season: "2026/27",
		isAdmin: func(string) bool { return false },
		namer:   namer,
	})

	if countEmbedFields(msgs) != 26 {
		t.Fatalf("total fields = %d, want 26 — no bettor dropped in the split", countEmbedFields(msgs))
	}
	// 26 fields split 25 + 1 across two embeds carried by one message.
	if len(msgs) != 1 || len(msgs[0]) != 2 {
		t.Fatalf("embed layout = %d message(s), want one message with two embeds", len(msgs))
	}
	if len(msgs[0][0].Fields) != maxEmbedFields || len(msgs[0][1].Fields) != 1 {
		t.Errorf("field split = %d + %d, want 25 + 1", len(msgs[0][0].Fields), len(msgs[0][1].Fields))
	}
	// Only the first embed of the reply is titled.
	if msgs[0][0].Title == "" || msgs[0][1].Title != "" {
		t.Errorf("titles = %q / %q, want the first embed titled and the rest bare", msgs[0][0].Title, msgs[0][1].Title)
	}
}

func TestHandleBet_ShowSplitsALongBoardAcrossMessages(t *testing.T) {
	// Each bettor carries a long display name, repeated enough that the packed
	// fields cannot fit a single Discord message's character budget.
	long := strings.Repeat("verylongname", 12)
	var picks []store.BettorPicks
	namer := fakeNamer{}
	for i := 0; i < 40; i++ {
		uid := long + string(rune('A'+i))
		picks = append(picks, store.BettorPicks{DiscordUserID: uid, Elements: [4]int{1, 2, 3, 4}})
		namer[uid] = uid
	}

	msgs := betEmbeds(t, &cmdInput{
		snap: betBotSnap(), opts: cmdOptions{}, sub: "show",
		betStore: &fakeBetStore{current: picks}, season: "2026/27",
		isAdmin: func(string) bool { return false },
		namer:   namer,
	})

	if len(msgs) < 2 {
		t.Fatalf("long board sent in %d message(s), want it split", len(msgs))
	}
	for i, m := range msgs {
		if len(m) > maxEmbedsPerMessage {
			t.Errorf("message %d carries %d embeds, over the %d cap", i, len(m), maxEmbedsPerMessage)
		}
	}
	if countEmbedFields(msgs) != 40 {
		t.Errorf("total fields = %d, want 40 — no bettor truncated", countEmbedFields(msgs))
	}
}

func TestBet_IsDeferredCommand(t *testing.T) {
	if !deferredCommands["bet"] {
		t.Error("/bet should be ACK'd with a deferred response so name lookups can't miss the 3s window")
	}
}

// The real store must satisfy the consumer interface.
var _ BetStore = (*store.Store)(nil)
