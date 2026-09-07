package bot

import (
	"strings"
	"testing"

	"github.com/UberJoe/fpldiscord/internal/fpl"
)

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

func TestHandleScores_ListsGwTotalsWithAutoSubsApplied(t *testing.T) {
	r := &recordingResponder{}
	if err := handleScores(&cmdInput{snap: scoresSnap(), resp: r}); err != nil {
		t.Fatalf("handleScores: %v", err)
	}
	if len(r.messages) != 1 {
		t.Fatalf("messages = %v, want one", r.messages)
	}
	msg := r.messages[0]

	for _, want := range []string{"Coq au Ian", "GW5", "Tess XI", "Bruno Dos Tres", "27", "25"} {
		if !strings.Contains(msg, want) {
			t.Errorf("scores output missing %q:\n%s", want, msg)
		}
	}
	// The auto-sub fired for Tess XI: the raw submitted XI sums to 21.
	if strings.Contains(msg, "21") {
		t.Errorf("auto-subs not applied — found the un-subbed total 21:\n%s", msg)
	}
	// Higher gameweek total is listed first.
	if strings.Index(msg, "Tess XI") > strings.Index(msg, "Bruno Dos Tres") {
		t.Errorf("rows not ordered by gameweek points desc:\n%s", msg)
	}
}

func TestHandleScores_HeaderShowsProvisionalThenFinal(t *testing.T) {
	snap := scoresSnap() // game.current_event_finished is false

	r := &recordingResponder{}
	if err := handleScores(&cmdInput{snap: snap, resp: r}); err != nil {
		t.Fatalf("handleScores: %v", err)
	}
	if !strings.Contains(r.messages[0], "provisional") || strings.Contains(r.messages[0], "final") {
		t.Errorf("header should say provisional while the GW is live:\n%s", r.messages[0])
	}

	snap.GWFinished = true
	r = &recordingResponder{}
	if err := handleScores(&cmdInput{snap: snap, resp: r}); err != nil {
		t.Fatalf("handleScores: %v", err)
	}
	if !strings.Contains(r.messages[0], "final") {
		t.Errorf("header should say final once the GW is finished:\n%s", r.messages[0])
	}
}

func TestHandleScores_DefaultsToCurrentGameweek(t *testing.T) {
	r := &recordingResponder{}
	if err := handleScores(&cmdInput{snap: scoresSnap(), resp: r}); err != nil {
		t.Fatalf("handleScores: %v", err)
	}
	if !strings.Contains(r.messages[0], "GW5") {
		t.Errorf("default gameweek is not the current one:\n%s", r.messages[0])
	}
}

func TestHandleScores_PastGameweekIsExplained(t *testing.T) {
	r := &recordingResponder{}
	in := &cmdInput{snap: scoresSnap(), opts: cmdOptions{"gw": float64(3)}, resp: r}
	if err := handleScores(in); err != nil {
		t.Fatalf("handleScores: %v", err)
	}
	if len(r.messages) != 1 || !strings.Contains(strings.ToLower(r.messages[0]), "current gameweek") {
		t.Errorf("past-gameweek reply = %v", r.messages)
	}
}

func TestHandleScores_NoSnapshotReportsStartingUp(t *testing.T) {
	r := &recordingResponder{}
	if err := handleScores(&cmdInput{snap: nil, resp: r}); err != nil {
		t.Fatalf("handleScores: %v", err)
	}
	if len(r.messages) != 1 || !strings.Contains(strings.ToLower(r.messages[0]), "starting up") {
		t.Errorf("nil-snapshot reply = %v", r.messages)
	}
}
