package bot

import (
	"strings"
	"testing"
	"time"

	"github.com/UberJoe/fpldiscord/internal/fpl"
	"github.com/bwmarrin/discordgo"
)

// classicSnapBuiltAt is the build time every classic snapshot in this file
// carries, so the standings embed's Timestamp is deterministic.
var classicSnapBuiltAt = time.Date(2026, 9, 7, 14, 30, 0, 0, time.UTC)

// standingsElements returns 11 same-position bootstrap elements per base id,
// enough for ManagerScore to resolve a submitted XI with no bench. None of
// them ever carries a finished-match Explain link, so ApplyAutoSubs never
// looks for a substitute — mirrors internal/fpl/standings_test.go's
// midElements, which bot cannot import directly (unexported, different
// package).
func standingsElements(bases ...fpl.ElementID) []fpl.Element {
	els := make([]fpl.Element, 0, 11*len(bases))
	for _, base := range bases {
		for i := fpl.ElementID(0); i < 11; i++ {
			els = append(els, fpl.Element{ID: base + i, ElementType: int(fpl.PosMID)})
		}
	}
	return els
}

// standingsXI returns a submitted 11-pick XI (slots 1..11) starting at base.
func standingsXI(base fpl.ElementID) []fpl.Pick {
	picks := make([]fpl.Pick, 0, 11)
	for i := 0; i < 11; i++ {
		picks = append(picks, fpl.Pick{Element: base + fpl.ElementID(i), Position: i + 1, Multiplier: 1})
	}
	return picks
}

// classicSnap is a hand-built classic-mode snapshot, GW5, with a live match in
// progress. Coq au Vin (LE 89) sits at official/last-week rank 1 — frozen 110,
// no live points this GW. Bruno Dos Tres (LE 39936) sits at official/last-week
// rank 2 — frozen 95 — but scores 25 live, overtaking for a live total of 120.
// The live table must therefore rank Bruno above Coq au Vin despite the Draft
// order disagreeing, with a ▲1/▼1 arrow pair for the swap.
func classicSnap() *fpl.Snapshot {
	return &fpl.Snapshot{
		BuiltAt:    classicSnapBuiltAt,
		LeagueName: "Coq au Ian",
		LeagueMode: fpl.ModeClassic,
		CurrentGW:  5,
		Bootstrap: fpl.Bootstrap{
			Elements: standingsElements(100, 200),
		},
		Live: map[int]fpl.LiveGW{
			5: {
				Fixtures: []fpl.LiveFixture{{ID: 1, Started: true, FinishedProvisional: false}},
				Elements: map[fpl.ElementID]fpl.LiveElement{
					200: {Stats: fpl.LiveStats{TotalPoints: 25, Minutes: 90}},
				},
			},
		},
		Entries: map[fpl.EntryID]fpl.EntryEvent{
			8900:   {Picks: standingsXI(100)},
			399360: {Picks: standingsXI(200)},
		},
		LeagueDetails: fpl.LeagueDetails{
			LeagueEntries: []fpl.LeagueEntry{
				{ID: 89, EntryID: 8900, EntryName: "Coq au Vin"},
				{ID: 39936, EntryID: 399360, EntryName: "Bruno Dos Tres"},
			},
			Standings: []fpl.Standing{
				{Rank: 2, LastRank: 2, RankSort: 2, LeagueEntry: 39936, Total: 130, EventTotal: 35},
				{Rank: 1, LastRank: 1, RankSort: 1, LeagueEntry: 89, Total: 150, EventTotal: 40},
			},
		},
	}
}

// deadWeekSnap is a classic snapshot between gameweeks: no live feed and no
// picks submitted, so every manager's live points default to 0 and MatchLive
// is false — the GW column must read "–", never "+0".
func deadWeekSnap() *fpl.Snapshot {
	return &fpl.Snapshot{
		BuiltAt:    classicSnapBuiltAt,
		LeagueName: "Coq au Ian",
		LeagueMode: fpl.ModeClassic,
		CurrentGW:  5,
		LeagueDetails: fpl.LeagueDetails{
			LeagueEntries: []fpl.LeagueEntry{
				{ID: 89, EntryID: 8900, EntryName: "Coq au Vin"},
				{ID: 39936, EntryID: 399360, EntryName: "Bruno Dos Tres"},
			},
			Standings: []fpl.Standing{
				{Rank: 1, LastRank: 1, RankSort: 1, LeagueEntry: 89, Total: 110, EventTotal: 0},
				{Rank: 2, LastRank: 2, RankSort: 2, LeagueEntry: 39936, Total: 95, EventTotal: 0},
			},
		},
	}
}

// autoSubSquadSettings mirrors league 64's settings.squad — the shape
// ApplyAutoSubs needs to resolve a bench substitution: 15-man squad, 11 play,
// slot 12 locked to a keeper.
var autoSubSquadSettings = fpl.SquadSettings{
	Size: 15, SelectGKP: 2, SelectDEF: 5, SelectMID: 5, SelectFWD: 3, Play: 11,
	MinPlayGKP: 1, MaxPlayGKP: 1, MinPlayDEF: 3, MinPlayMID: 2, MinPlayFWD: 1,
	PositionTypeLocks: map[string]string{"12": "GKP"},
}

// autoSubSnap is a one-manager classic snapshot whose slot-5 DEF (element 704)
// blanks in a finished match; the bench DEF (element 712, slot 13) comes on
// and scores 8. The rendered Tot/GW columns must reflect the scoring XI after
// the auto-sub (live 37, live total 147), not the raw picks (which would give
// 29 / 139).
func autoSubSnap() *fpl.Snapshot {
	return &fpl.Snapshot{
		BuiltAt:    classicSnapBuiltAt,
		LeagueName: "Coq au Ian",
		LeagueMode: fpl.ModeClassic,
		CurrentGW:  5,
		Bootstrap: fpl.Bootstrap{
			Elements: []fpl.Element{
				{ID: 700, ElementType: int(fpl.PosGK)},
				{ID: 701, ElementType: int(fpl.PosDEF)}, {ID: 702, ElementType: int(fpl.PosDEF)},
				{ID: 703, ElementType: int(fpl.PosDEF)}, {ID: 704, ElementType: int(fpl.PosDEF)},
				{ID: 705, ElementType: int(fpl.PosMID)}, {ID: 706, ElementType: int(fpl.PosMID)},
				{ID: 707, ElementType: int(fpl.PosMID)}, {ID: 708, ElementType: int(fpl.PosMID)},
				{ID: 709, ElementType: int(fpl.PosFWD)}, {ID: 710, ElementType: int(fpl.PosFWD)},
				{ID: 711, ElementType: int(fpl.PosGK)}, {ID: 712, ElementType: int(fpl.PosDEF)},
				{ID: 713, ElementType: int(fpl.PosMID)}, {ID: 714, ElementType: int(fpl.PosFWD)},
			},
			Settings: fpl.Settings{Squad: autoSubSquadSettings},
		},
		Live: map[int]fpl.LiveGW{
			5: {
				Fixtures: []fpl.LiveFixture{{ID: 1, Started: true, Finished: true, FinishedProvisional: true}},
				Elements: map[fpl.ElementID]fpl.LiveElement{
					700: {Stats: fpl.LiveStats{TotalPoints: 2, Minutes: 90}},
					701: {Stats: fpl.LiveStats{TotalPoints: 1, Minutes: 90}},
					702: {Stats: fpl.LiveStats{TotalPoints: 3, Minutes: 90}},
					703: {Stats: fpl.LiveStats{TotalPoints: 2, Minutes: 90}},
					704: {Stats: fpl.LiveStats{Minutes: 0}, Explain: []fpl.LiveExplain{{Fixture: 1}}},
					705: {Stats: fpl.LiveStats{TotalPoints: 5, Minutes: 90}},
					706: {Stats: fpl.LiveStats{TotalPoints: 4, Minutes: 90}},
					707: {Stats: fpl.LiveStats{TotalPoints: 3, Minutes: 90}},
					708: {Stats: fpl.LiveStats{TotalPoints: 6, Minutes: 90}},
					709: {Stats: fpl.LiveStats{TotalPoints: 2, Minutes: 90}},
					710: {Stats: fpl.LiveStats{TotalPoints: 1, Minutes: 90}},
					712: {Stats: fpl.LiveStats{TotalPoints: 8, Minutes: 90}},
				},
			},
		},
		Entries: map[fpl.EntryID]fpl.EntryEvent{
			700: {Picks: []fpl.Pick{
				{Element: 700, Position: 1, Multiplier: 1},
				{Element: 701, Position: 2, Multiplier: 1}, {Element: 702, Position: 3, Multiplier: 1},
				{Element: 703, Position: 4, Multiplier: 1}, {Element: 704, Position: 5, Multiplier: 1},
				{Element: 705, Position: 6, Multiplier: 1}, {Element: 706, Position: 7, Multiplier: 1},
				{Element: 707, Position: 8, Multiplier: 1}, {Element: 708, Position: 9, Multiplier: 1},
				{Element: 709, Position: 10, Multiplier: 1}, {Element: 710, Position: 11, Multiplier: 1},
				{Element: 711, Position: 12, Multiplier: 1}, {Element: 712, Position: 13, Multiplier: 1},
				{Element: 713, Position: 14, Multiplier: 1}, {Element: 714, Position: 15, Multiplier: 1},
			}},
		},
		LeagueDetails: fpl.LeagueDetails{
			LeagueEntries: []fpl.LeagueEntry{
				{ID: 1, EntryID: 700, EntryName: "Auto Subs FC"},
			},
			Standings: []fpl.Standing{
				{Rank: 1, LastRank: 1, RankSort: 1, LeagueEntry: 1, Total: 150, EventTotal: 40},
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

func TestHandleStandings_DescriptionShowsLiveTotalsInLiveRankOrder(t *testing.T) {
	e := standingsEmbed(t, classicSnap())
	d := e.Description

	if !strings.HasPrefix(d, "```") || !strings.HasSuffix(d, "```") {
		t.Errorf("description is not a fenced code block:\n%s", d)
	}
	for _, want := range []string{"Coq au Vin", "Bruno Dos Tres", "120", "110", "+25", "+0"} {
		if !strings.Contains(d, want) {
			t.Errorf("description missing %q:\n%s", want, d)
		}
	}
	// Bruno Dos Tres leads on live points (120) despite official rank 2 — the
	// table orders by live rank, not the Draft official rank.
	if strings.Index(d, "Bruno Dos Tres") > strings.Index(d, "Coq au Vin") {
		t.Errorf("rows not in live-rank order:\n%s", d)
	}
}

func TestHandleStandings_DescriptionHasMovementArrowsForLiveRankChanges(t *testing.T) {
	e := standingsEmbed(t, classicSnap())
	d := e.Description

	// Bruno Dos Tres climbed from last week's rank 2 to live rank 1 (▲1); Coq
	// au Vin dropped from rank 1 to live rank 2 (▼1).
	for _, want := range []string{"▲1", "▼1"} {
		if !strings.Contains(d, want) {
			t.Errorf("description missing movement arrow %q:\n%s", want, d)
		}
	}
	// Only the ▲/▼/– convention arrowCell actually emits — no other glyph a
	// future change might reach for instead.
	for _, glyph := range []string{"▬", "↑", "↓", "→"} {
		if strings.Contains(d, glyph) {
			t.Errorf("description contains unexpected glyph %q — arrowCell only emits ▲/▼/–:\n%s", glyph, d)
		}
	}
}

func TestGwCell(t *testing.T) {
	tests := []struct {
		name         string
		liveGwPoints int
		gwStarted    bool
		want         string
	}{
		{"positive points", 15, true, "+15"},
		{"zero points, gameweek started", 0, true, "+0"},
		{"zero points, gameweek not started", 0, false, "–"},
		// A scoring XI can net a genuinely negative gameweek (red cards, own
		// goals outweighing appearance points); the cell must show "-3", not
		// the "+-3" a hand-rolled "+" prefix would produce.
		{"negative points", -3, true, "-3"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := gwCell(tt.liveGwPoints, tt.gwStarted); got != tt.want {
				t.Errorf("gwCell(%d, %v) = %q, want %q", tt.liveGwPoints, tt.gwStarted, got, tt.want)
			}
		})
	}
}

func TestHandleStandings_GWColumnReadsDashBetweenGameweeks(t *testing.T) {
	e := standingsEmbed(t, deadWeekSnap())
	d := e.Description

	if strings.Contains(d, "+") {
		t.Errorf("description shows a +N/+0 GW value between gameweeks, want – for every row:\n%s", d)
	}
}

func TestHandleStandings_AutoSubbedManagerRendersPostSubTotal(t *testing.T) {
	e := standingsEmbed(t, autoSubSnap())
	d := e.Description

	// Frozen 110 (Total 150 - EventTotal 40) + live 37 (auto-sub applied,
	// element 712's 8 replacing blanking element 704) = 147.
	if !strings.Contains(d, "147") {
		t.Errorf("description missing live total 147 (auto-sub applied):\n%s", d)
	}
	if !strings.Contains(d, "+37") {
		t.Errorf("description missing GW cell +37 (auto-sub applied):\n%s", d)
	}
	// Summing the raw picks instead (704's blank counted, 712 never brought on)
	// would give 29 live / 139 total — neither must appear.
	for _, notWant := range []string{"139", "+29"} {
		if strings.Contains(d, notWant) {
			t.Errorf("description shows the pre-auto-sub value %q:\n%s", notWant, d)
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
