package bot

import (
	"strings"
	"testing"

	"github.com/UberJoe/fpldiscord/internal/fpl"
)

func TestHandleTeamlist_GroupsSquadByPosition(t *testing.T) {
	r := &recordingResponder{}
	in := &cmdInput{snap: ownerTeamlistSnap(), opts: cmdOptions{"owner": "Ian"}, resp: r}
	if err := handleTeamlist(in); err != nil {
		t.Fatalf("handleTeamlist: %v", err)
	}
	if len(r.messages) != 1 {
		t.Fatalf("messages = %v, want one", r.messages)
	}
	msg := r.messages[0]

	// Ian owns Sánchez (GK/CRY), Muñoz (DEF/CRY) and Gabriel (DEF/ARS).
	for _, want := range []string{"Ian's squad", "GK", "DEF", "MID", "FWD", "Sánchez (CRY)", "Muñoz (CRY)", "Gabriel (ARS)"} {
		if !strings.Contains(msg, want) {
			t.Errorf("teamlist missing %q:\n%s", want, msg)
		}
	}
	// Fixed position order.
	gk, def, mid, fwd := strings.Index(msg, "GK"), strings.Index(msg, "DEF"), strings.Index(msg, "MID"), strings.Index(msg, "FWD")
	if !(gk < def && def < mid && mid < fwd) {
		t.Errorf("positions out of order (GK %d, DEF %d, MID %d, FWD %d):\n%s", gk, def, mid, fwd, msg)
	}
	// Ian has no midfielders or forwards — those rows show the placeholder.
	if !strings.Contains(msg, "—") {
		t.Errorf("empty positions should show a placeholder:\n%s", msg)
	}
	// A player he does not own must not appear.
	if strings.Contains(msg, "Højlund") {
		t.Errorf("teamlist leaked a player Ian doesn't own:\n%s", msg)
	}
}

func TestHandleTeamlist_AccentAndCaseInsensitiveOwner(t *testing.T) {
	r := &recordingResponder{}
	in := &cmdInput{snap: ownerTeamlistSnap(), opts: cmdOptions{"owner": " ian "}, resp: r}
	if err := handleTeamlist(in); err != nil {
		t.Fatalf("handleTeamlist: %v", err)
	}
	if !strings.Contains(r.messages[0], "Ian's squad") {
		t.Errorf("lower/whitespace owner should resolve; reply = %q", r.messages[0])
	}
}

func TestHandleTeamlist_SharedFirstNamePicksTheFirstManager(t *testing.T) {
	snap := ownerTeamlistSnap()
	// A second "Ian" enters the league and owns Højlund (element 11).
	otherIan := fpl.EntryID(777)
	snap.LeagueDetails.LeagueEntries = append(snap.LeagueDetails.LeagueEntries,
		fpl.LeagueEntry{ID: 777, EntryID: otherIan, EntryName: "Ian's Other XI", PlayerFirstName: "Ian"})
	snap.ElementStatus[0] = fpl.ElementStatus{Element: 11, Owner: &otherIan}

	r := &recordingResponder{}
	in := &cmdInput{snap: snap, opts: cmdOptions{"owner": "Ian"}, resp: r}
	if err := handleTeamlist(in); err != nil {
		t.Fatalf("handleTeamlist: %v", err)
	}
	msg := r.messages[0]
	// Højlund (element 11) is first in bootstrap order and now belongs to the
	// new Ian, so his one-player squad is shown — not a merge that also carries
	// the original Ian's Sánchez / Muñoz / Gabriel.
	if !strings.Contains(msg, "Højlund (ARS)") {
		t.Errorf("expected the first-seen Ian's squad:\n%s", msg)
	}
	for _, leaked := range []string{"Sánchez", "Muñoz", "Gabriel"} {
		if strings.Contains(msg, leaked) {
			t.Errorf("shared first name merged two squads (found %q):\n%s", leaked, msg)
		}
	}
}

func TestHandleTeamlist_UnknownOwner(t *testing.T) {
	r := &recordingResponder{}
	in := &cmdInput{snap: ownerTeamlistSnap(), opts: cmdOptions{"owner": "Zoe"}, resp: r}
	if err := handleTeamlist(in); err != nil {
		t.Fatalf("handleTeamlist: %v", err)
	}
	if !strings.Contains(strings.ToLower(r.messages[0]), "no manager") {
		t.Errorf("unknown owner reply = %q", r.messages[0])
	}
}

func TestHandleTeamlist_NoSnapshotReportsStartingUp(t *testing.T) {
	r := &recordingResponder{}
	if err := handleTeamlist(&cmdInput{snap: nil, opts: cmdOptions{"owner": "Ian"}, resp: r}); err != nil {
		t.Fatalf("handleTeamlist: %v", err)
	}
	if !strings.Contains(strings.ToLower(r.messages[0]), "starting up") {
		t.Errorf("nil-snapshot reply = %q", r.messages[0])
	}
}
