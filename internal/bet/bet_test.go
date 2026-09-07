package bet

import (
	"testing"

	"github.com/UberJoe/fpldiscord/internal/fpl"
	"github.com/UberJoe/fpldiscord/internal/store"
)

// fakeGoals is a GoalSource backed by a literal id -> (name, goals) table.
type fakeGoals map[fpl.ElementID]fakePlayer

type fakePlayer struct {
	name  string
	goals int
}

func (f fakeGoals) Element(id fpl.ElementID) (*fpl.Element, bool) {
	p, ok := f[id]
	if !ok {
		return nil, false
	}
	return &fpl.Element{ID: id, WebName: p.name, GoalsScored: p.goals}, true
}

// entry is one bettor's test input: a Discord user id and their four pick ids.
type entry struct {
	user string
	ids  [4]int
}

func bettor(user string, ids [4]int) entry { return entry{user: user, ids: ids} }

// picksOf turns test entries into the []store.BettorPicks that Leaderboard takes.
func picksOf(entries ...entry) []store.BettorPicks {
	out := make([]store.BettorPicks, 0, len(entries))
	for _, e := range entries {
		out = append(out, store.BettorPicks{DiscordUserID: e.user, Elements: e.ids})
	}
	return out
}

func TestStatus(t *testing.T) {
	goals := fakeGoals{
		1: {"a", 5}, 2: {"b", 5}, 3: {"c", 5}, 4: {"d", 6}, // total 21, all scored -> in
		10: {"e", 8}, 11: {"f", 7}, 12: {"g", 6}, 13: {"h", 0}, // one on zero -> provisionallyOut
		20: {"i", 9}, 21: {"j", 8}, 22: {"k", 7}, 23: {"l", 1}, // total 25 -> bust
		30: {"m", 10}, 31: {"n", 10}, 32: {"o", 5}, 33: {"p", 0}, // over 21 AND a zero -> bust wins
	}
	board := Leaderboard(picksOf(
		bettor("in-exactly-21", [4]int{1, 2, 3, 4}),
		bettor("prov-out", [4]int{10, 11, 12, 13}),
		bettor("bust", [4]int{20, 21, 22, 23}),
		bettor("bust-with-zero", [4]int{30, 31, 32, 33}),
	), goals)

	got := map[string]Bettor{}
	for _, b := range board {
		got[b.DiscordUserID] = b
	}

	if b := got["in-exactly-21"]; b.Status != StatusIn || b.Total != 21 {
		t.Errorf("in-exactly-21: status=%q total=%d, want in / 21", b.Status, b.Total)
	}
	if b := got["prov-out"]; b.Status != StatusProvisionallyOut || b.Total != 21 {
		t.Errorf("prov-out: status=%q total=%d, want provisionallyOut / 21", b.Status, b.Total)
	}
	if b := got["bust"]; b.Status != StatusBust || b.Total != 25 {
		t.Errorf("bust: status=%q total=%d, want bust / 25", b.Status, b.Total)
	}
	if b := got["bust-with-zero"]; b.Status != StatusBust {
		t.Errorf("bust-with-zero: status=%q, want bust (over 21 beats the zero rule)", b.Status)
	}
}

func TestStatusUnknownElementCountsAsZeroGoals(t *testing.T) {
	goals := fakeGoals{1: {"a", 5}, 2: {"b", 5}, 3: {"c", 5}} // id 4 missing
	board := Leaderboard(picksOf(bettor("u", [4]int{1, 2, 3, 4})), goals)

	if len(board) != 1 {
		t.Fatalf("board has %d rows, want 1", len(board))
	}
	b := board[0]
	if b.Total != 15 {
		t.Errorf("total = %d, want 15 (missing pick contributes 0)", b.Total)
	}
	if b.Status != StatusProvisionallyOut {
		t.Errorf("status = %q, want provisionallyOut (a pick resolves to 0 goals)", b.Status)
	}
	if b.Picks[3].WebName != "" || b.Picks[3].Goals != 0 {
		t.Errorf("unknown pick = %+v, want zero-valued name/goals", b.Picks[3])
	}
}

func TestLeaderClosestToTargetFromBelow(t *testing.T) {
	goals := fakeGoals{
		1: {"", 5}, 2: {"", 5}, 3: {"", 5}, 4: {"", 3}, // 18, in
		10: {"", 5}, 11: {"", 5}, 12: {"", 5}, 13: {"", 5}, // 20, in  <- leader
		20: {"", 9}, 21: {"", 9}, 22: {"", 9}, 23: {"", 1}, // 28, bust
	}
	board := Leaderboard(picksOf(
		bettor("eighteen", [4]int{1, 2, 3, 4}),
		bettor("twenty", [4]int{10, 11, 12, 13}),
		bettor("bust", [4]int{20, 21, 22, 23}),
	), goals)

	for _, b := range board {
		want := b.DiscordUserID == "twenty"
		if b.Leader != want {
			t.Errorf("%s: Leader=%v, want %v", b.DiscordUserID, b.Leader, want)
		}
	}
}

func TestLeaderExactly21WinsOutright(t *testing.T) {
	goals := fakeGoals{
		1: {"", 6}, 2: {"", 5}, 3: {"", 5}, 4: {"", 5}, // 21, in
		10: {"", 5}, 11: {"", 5}, 12: {"", 5}, 13: {"", 5}, // 20, in
	}
	board := Leaderboard(picksOf(
		bettor("twenty", [4]int{10, 11, 12, 13}),
		bettor("twentyone", [4]int{1, 2, 3, 4}),
	), goals)

	for _, b := range board {
		want := b.DiscordUserID == "twentyone"
		if b.Leader != want {
			t.Errorf("%s: Leader=%v, want %v", b.DiscordUserID, b.Leader, want)
		}
	}
}

func TestLeaderGenuineTiesAreJoint(t *testing.T) {
	goals := fakeGoals{
		1: {"", 5}, 2: {"", 5}, 3: {"", 5}, 4: {"", 5}, // 20, in
		10: {"", 4}, 11: {"", 6}, 12: {"", 5}, 13: {"", 5}, // 20, in
		20: {"", 1}, 21: {"", 1}, 22: {"", 1}, 23: {"", 1}, // 4, in but behind
	}
	board := Leaderboard(picksOf(
		bettor("a", [4]int{1, 2, 3, 4}),
		bettor("b", [4]int{10, 11, 12, 13}),
		bettor("c", [4]int{20, 21, 22, 23}),
	), goals)

	leaders := map[string]bool{}
	for _, b := range board {
		if b.Leader {
			leaders[b.DiscordUserID] = true
		}
	}
	if !leaders["a"] || !leaders["b"] || leaders["c"] {
		t.Errorf("leaders = %v, want a and b joint", leaders)
	}
}

func TestNoLeaderWhenNobodyIsIn(t *testing.T) {
	goals := fakeGoals{
		1: {"", 9}, 2: {"", 9}, 3: {"", 9}, 4: {"", 1}, // 28, bust
		10: {"", 5}, 11: {"", 5}, 12: {"", 5}, 13: {"", 0}, // 15, provisionallyOut
	}
	board := Leaderboard(picksOf(
		bettor("bust", [4]int{1, 2, 3, 4}),
		bettor("provout", [4]int{10, 11, 12, 13}),
	), goals)

	for _, b := range board {
		if b.Leader {
			t.Errorf("%s marked leader; nobody is 'in' so there is no leader", b.DiscordUserID)
		}
	}
}

func TestSortOrderClosestToTargetThenBustsLast(t *testing.T) {
	goals := fakeGoals{
		1: {"", 5}, 2: {"", 5}, 3: {"", 5}, 4: {"", 5}, // 20, in
		10: {"", 9}, 11: {"", 9}, 12: {"", 9}, 13: {"", 1}, // 28, bust
		20: {"", 3}, 21: {"", 3}, 22: {"", 3}, 23: {"", 0}, // 9, provisionallyOut
		30: {"", 6}, 31: {"", 5}, 32: {"", 5}, 33: {"", 5}, // 21, in
	}
	board := Leaderboard(picksOf(
		bettor("bust", [4]int{10, 11, 12, 13}),
		bettor("nine", [4]int{20, 21, 22, 23}),
		bettor("twenty", [4]int{1, 2, 3, 4}),
		bettor("twentyone", [4]int{30, 31, 32, 33}),
	), goals)

	order := make([]string, len(board))
	for i, b := range board {
		order[i] = b.DiscordUserID
	}
	want := []string{"twentyone", "twenty", "nine", "bust"}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("sort order = %v, want %v", order, want)
		}
	}
}

func TestSortOrderTieBreaksByUserID(t *testing.T) {
	goals := fakeGoals{
		1: {"", 5}, 2: {"", 5}, 3: {"", 5}, 4: {"", 5}, // 20
		10: {"", 5}, 11: {"", 5}, 12: {"", 5}, 13: {"", 5}, // 20
	}
	board := Leaderboard(picksOf(
		bettor("zeb", [4]int{1, 2, 3, 4}),
		bettor("amy", [4]int{10, 11, 12, 13}),
	), goals)

	if board[0].DiscordUserID != "amy" || board[1].DiscordUserID != "zeb" {
		t.Fatalf("tie order = [%s %s], want [amy zeb]", board[0].DiscordUserID, board[1].DiscordUserID)
	}
}

func TestPicksResolvedInSlotOrder(t *testing.T) {
	goals := fakeGoals{7: {"Haaland", 9}, 8: {"Salah", 5}, 9: {"Isak", 4}, 6: {"Watkins", 3}}
	board := Leaderboard(picksOf(bettor("u", [4]int{7, 8, 9, 6})), goals)

	b := board[0]
	wantNames := []string{"Haaland", "Salah", "Isak", "Watkins"}
	wantGoals := []int{9, 5, 4, 3}
	for i := range wantNames {
		if b.Picks[i].WebName != wantNames[i] || b.Picks[i].Goals != wantGoals[i] {
			t.Errorf("slot %d = %+v, want %s / %d", i, b.Picks[i], wantNames[i], wantGoals[i])
		}
		if b.Picks[i].ElementID != fpl.ElementID([]int{7, 8, 9, 6}[i]) {
			t.Errorf("slot %d element id = %d, want %d", i, b.Picks[i].ElementID, []int{7, 8, 9, 6}[i])
		}
	}
	if b.Total != 21 {
		t.Errorf("total = %d, want 21", b.Total)
	}
}

func TestSeasonComplete(t *testing.T) {
	cases := []struct {
		name string
		snap *fpl.Snapshot
		want bool
	}{
		{"nil snapshot", nil, false},
		{"mid-season", &fpl.Snapshot{Game: fpl.Game{CurrentEvent: 20, CurrentEventFinished: true}}, false},
		{"gw38 in progress", &fpl.Snapshot{Game: fpl.Game{CurrentEvent: 38, CurrentEventFinished: false}}, false},
		{"gw38 finished", &fpl.Snapshot{Game: fpl.Game{CurrentEvent: 38, CurrentEventFinished: true}}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := SeasonComplete(c.snap); got != c.want {
				t.Errorf("SeasonComplete = %v, want %v", got, c.want)
			}
		})
	}
}

// *fpl.Snapshot must satisfy GoalSource so production wiring can pass it directly.
var _ GoalSource = (*fpl.Snapshot)(nil)
