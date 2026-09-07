package bot

import (
	"strings"
	"testing"

	"github.com/UberJoe/fpldiscord/internal/fpl"
)

// waiverBotSnap builds a classic snapshot whose processed gameweeks are 3 and
// 4. GW4 holds a contested waiver (Sam wins Salah on priority 1, Joe is
// out-bid), a free-agent pickup, and a trade that must never render. GW3 holds
// one uncontested accepted waiver.
func waiverBotSnap() *fpl.Snapshot {
	return &fpl.Snapshot{
		LeagueName: "FPL Draft 26/27",
		LeagueMode: fpl.ModeClassic,
		CurrentGW:  5,
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

func TestHandleWaivers_NoArgsShowsLatestProcessedAcceptedClaims(t *testing.T) {
	r := &recordingResponder{}
	if err := handleWaivers(&cmdInput{snap: waiverBotSnap(), resp: r}); err != nil {
		t.Fatalf("handleWaivers: %v", err)
	}
	if len(r.messages) != 1 {
		t.Fatalf("messages = %d, want 1", len(r.messages))
	}
	msg := r.messages[0]
	if !strings.Contains(msg, "GW4") || !strings.Contains(strings.ToLower(msg), "accepted") {
		t.Errorf("header missing GW4/accepted:\n%s", msg)
	}
	// The accepted waiver and the free-agent pickup show; the out-bid claim and
	// the trade do not.
	if !strings.Contains(msg, "Sam") || !strings.Contains(msg, "Salah") {
		t.Errorf("accepted claim missing:\n%s", msg)
	}
	if !strings.Contains(msg, "Watkins") || !strings.Contains(strings.ToLower(msg), "free agent") {
		t.Errorf("free-agent pickup missing:\n%s", msg)
	}
	if strings.Contains(msg, "Joe") {
		t.Errorf("out-bid (failed) claim should not appear under accepted:\n%s", msg)
	}
}

func TestHandleWaivers_GWArgSelectsRound(t *testing.T) {
	r := &recordingResponder{}
	err := handleWaivers(&cmdInput{
		snap: waiverBotSnap(),
		opts: cmdOptions{"gw": float64(3)},
		resp: r,
	})
	if err != nil {
		t.Fatalf("handleWaivers: %v", err)
	}
	msg := r.messages[0]
	if !strings.Contains(msg, "GW3") || !strings.Contains(msg, "Saka") {
		t.Errorf("GW3 round not rendered:\n%s", msg)
	}
}

func TestHandleWaivers_FailedShowsOutbidChain(t *testing.T) {
	r := &recordingResponder{}
	err := handleWaivers(&cmdInput{
		snap: waiverBotSnap(),
		opts: cmdOptions{"result": "failed"},
		resp: r,
	})
	if err != nil {
		t.Fatalf("handleWaivers: %v", err)
	}
	all := strings.Join(r.messages, "\n")
	// The contested group lists the winner and the out-bid manager together.
	if !strings.Contains(all, "Sam") || !strings.Contains(all, "Joe") {
		t.Errorf("out-bid chain missing a party:\n%s", all)
	}
	if !strings.Contains(all, "won") || !strings.Contains(all, "out-bid") {
		t.Errorf("out-bid markers missing:\n%s", all)
	}
	if !strings.Contains(all, "#1") || !strings.Contains(all, "#4") {
		t.Errorf("priority order not shown:\n%s", all)
	}
	// The uncontested free-agent pickup has no failed row -> not in failed view.
	if strings.Contains(all, "Watkins") {
		t.Errorf("uncontested claim leaked into failed view:\n%s", all)
	}
}

func TestHandleWaivers_LongRoundSplitsAcrossMessagesNoTruncation(t *testing.T) {
	snap := waiverBotSnap()

	// 60 contested players, each with a winner and an out-bid claim: far more
	// than one Discord message can hold.
	var txns []fpl.Transaction
	var elems []fpl.Element
	var owners []fpl.LeagueEntry
	owners = append(owners,
		fpl.LeagueEntry{ID: 1, EntryID: 500, PlayerFirstName: "Sam"},
		fpl.LeagueEntry{ID: 2, EntryID: 600, PlayerFirstName: "Joe"},
	)
	const n = 60
	wantNames := make([]string, 0, n)
	for i := 0; i < n; i++ {
		inID := fpl.ElementID(1000 + i)
		name := "Contested" + string(rune('A'+i%26)) + strings.Repeat("x", 1+i/26)
		wantNames = append(wantNames, name)
		elems = append(elems, fpl.Element{ID: inID, WebName: name})
		txns = append(txns,
			fpl.Transaction{Entry: 500, Event: 4, ElementIn: inID, Kind: "w", Result: "a", Priority: 1, Index: 2 * i},
			fpl.Transaction{Entry: 600, Event: 4, ElementIn: inID, Kind: "w", Result: "do", Priority: 2, Index: 2*i + 1},
		)
	}
	snap.Bootstrap.Elements = elems
	snap.LeagueDetails.LeagueEntries = owners
	snap.Transactions = txns

	r := &recordingResponder{}
	if err := handleWaivers(&cmdInput{snap: snap, opts: cmdOptions{"result": "all"}, resp: r}); err != nil {
		t.Fatalf("handleWaivers: %v", err)
	}
	if len(r.messages) < 2 {
		t.Fatalf("messages = %d, want the round split across at least 2", len(r.messages))
	}
	joined := strings.Join(r.messages, "\n")
	for _, name := range wantNames {
		if !strings.Contains(joined, name) {
			t.Fatalf("player %q dropped — output was truncated", name)
		}
	}
	for i, m := range r.messages {
		if len(m) > 2000 {
			t.Errorf("message %d is %d chars, over the 2000 Discord limit", i, len(m))
		}
		if strings.Count(m, "```")%2 != 0 {
			t.Errorf("message %d has an unbalanced code fence:\n%s", i, m)
		}
	}
}

// Two different players can share a web_name in one round. Grouping the out-bid
// chain must key on the element id, not the name, or the two collapse into one
// bogus chain.
func TestHandleWaivers_SameNamedPlayersDoNotMergeIntoOneChain(t *testing.T) {
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

	r := &recordingResponder{}
	if err := handleWaivers(&cmdInput{snap: snap, opts: cmdOptions{"result": "all"}, resp: r}); err != nil {
		t.Fatalf("handleWaivers: %v", err)
	}
	all := strings.Join(r.messages, "\n")
	// Two separate contested groups, each a "Silva:" header, each with its own
	// winner + out-bid pair.
	if got := strings.Count(all, "Silva:"); got != 2 {
		t.Fatalf("Silva group headers = %d, want 2 (one per element id)\n%s", got, all)
	}
	if strings.Count(all, "won") != 2 || strings.Count(all, "out-bid") != 2 {
		t.Errorf("each Silva group should have one winner and one out-bid claim:\n%s", all)
	}
}

func TestHandleWaivers_NoProcessedRoundsIsExplained(t *testing.T) {
	snap := waiverBotSnap()
	snap.Transactions = nil
	snap.Game = fpl.Game{CurrentEvent: 5}

	r := &recordingResponder{}
	if err := handleWaivers(&cmdInput{snap: snap, resp: r}); err != nil {
		t.Fatalf("handleWaivers: %v", err)
	}
	if len(r.messages) != 1 || !strings.Contains(strings.ToLower(r.messages[0]), "no waiver") {
		t.Errorf("reply = %v", r.messages)
	}
}

func TestHandleWaivers_NoSnapshotReportsStartingUp(t *testing.T) {
	r := &recordingResponder{}
	if err := handleWaivers(&cmdInput{snap: nil, resp: r}); err != nil {
		t.Fatalf("handleWaivers: %v", err)
	}
	if len(r.messages) != 1 || !strings.Contains(strings.ToLower(r.messages[0]), "starting up") {
		t.Errorf("reply = %v", r.messages)
	}
}

func TestHandleWaivers_UnknownGWReportsNoWaivers(t *testing.T) {
	r := &recordingResponder{}
	err := handleWaivers(&cmdInput{
		snap: waiverBotSnap(),
		opts: cmdOptions{"gw": float64(20)},
		resp: r,
	})
	if err != nil {
		t.Fatalf("handleWaivers: %v", err)
	}
	if len(r.messages) != 1 || !strings.Contains(r.messages[0], "GW20") {
		t.Errorf("reply = %v", r.messages)
	}
}
