package bot

import (
	"strings"
	"testing"

	"github.com/UberJoe/fpldiscord/internal/fpl"
)

// classicSnap is a hand-built classic-mode snapshot with two members whose
// standings rows are deliberately out of rank order.
func classicSnap() *fpl.Snapshot {
	return &fpl.Snapshot{
		LeagueName: "Coq au Ian",
		LeagueMode: fpl.ModeClassic,
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

func TestHandleStandings_RendersClassicTableInRankOrder(t *testing.T) {
	r := &recordingResponder{}
	if err := handleStandings(&cmdInput{snap: classicSnap(), resp: r}); err != nil {
		t.Fatalf("handleStandings: %v", err)
	}
	if len(r.messages) != 1 {
		t.Fatalf("messages = %v, want one", r.messages)
	}
	msg := r.messages[0]

	for _, want := range []string{"Coq au Ian", "Coq au Vin", "Bruno Dos Tres", "210", "180", "55", "40"} {
		if !strings.Contains(msg, want) {
			t.Errorf("rendered standings missing %q:\n%s", want, msg)
		}
	}
	// Rank 1 (Ian) renders above rank 2 (Bruno) despite the input order.
	if strings.Index(msg, "Coq au Vin") > strings.Index(msg, "Bruno Dos Tres") {
		t.Errorf("rows not in rank order:\n%s", msg)
	}
}

func TestHandleStandings_H2HModeReportsUnavailable(t *testing.T) {
	s := classicSnap()
	s.LeagueMode = fpl.ModeH2H

	r := &recordingResponder{}
	if err := handleStandings(&cmdInput{snap: s, resp: r}); err != nil {
		t.Fatalf("handleStandings: %v", err)
	}
	if len(r.messages) != 1 || !strings.Contains(strings.ToLower(r.messages[0]), "head-to-head") {
		t.Errorf("h2h reply = %v", r.messages)
	}
}

func TestHandleStandings_NoSnapshotReportsStartingUp(t *testing.T) {
	r := &recordingResponder{}
	if err := handleStandings(&cmdInput{snap: nil, resp: r}); err != nil {
		t.Fatalf("handleStandings: %v", err)
	}
	if len(r.messages) != 1 || !strings.Contains(strings.ToLower(r.messages[0]), "starting up") {
		t.Errorf("nil-snapshot reply = %v", r.messages)
	}
}
