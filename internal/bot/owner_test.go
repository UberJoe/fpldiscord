package bot

import (
	"strings"
	"testing"
)

func TestHandleOwner_NamesTheOwningManager(t *testing.T) {
	r := &recordingResponder{}
	in := &cmdInput{snap: ownerTeamlistSnap(), opts: cmdOptions{"player": "Højlund"}, resp: r}
	if err := handleOwner(in); err != nil {
		t.Fatalf("handleOwner: %v", err)
	}
	if len(r.messages) != 1 {
		t.Fatalf("messages = %v, want one", r.messages)
	}
	msg := r.messages[0]
	for _, want := range []string{"Højlund", "Bruno", "Bruno Dos Tres", "ARS"} {
		if !strings.Contains(msg, want) {
			t.Errorf("owner reply missing %q:\n%s", want, msg)
		}
	}
}

func TestHandleOwner_FreeAgent(t *testing.T) {
	r := &recordingResponder{}
	in := &cmdInput{snap: ownerTeamlistSnap(), opts: cmdOptions{"player": "Raya"}, resp: r}
	if err := handleOwner(in); err != nil {
		t.Fatalf("handleOwner: %v", err)
	}
	if !strings.Contains(strings.ToLower(r.messages[0]), "free agent") {
		t.Errorf("Raya is unowned; reply = %q", r.messages[0])
	}
}

func TestHandleOwner_AccentAndCaseInsensitiveLookup(t *testing.T) {
	r := &recordingResponder{}
	in := &cmdInput{snap: ownerTeamlistSnap(), opts: cmdOptions{"player": "  hojlund "}, resp: r}
	if err := handleOwner(in); err != nil {
		t.Fatalf("handleOwner: %v", err)
	}
	if !strings.Contains(r.messages[0], "Bruno") {
		t.Errorf("ASCII/lower/whitespace input should still resolve Højlund; reply = %q", r.messages[0])
	}
}

func TestHandleOwner_UnknownPlayer(t *testing.T) {
	r := &recordingResponder{}
	in := &cmdInput{snap: ownerTeamlistSnap(), opts: cmdOptions{"player": "Nobody"}, resp: r}
	if err := handleOwner(in); err != nil {
		t.Fatalf("handleOwner: %v", err)
	}
	if !strings.Contains(strings.ToLower(r.messages[0]), "no player") {
		t.Errorf("unknown player reply = %q", r.messages[0])
	}
}

func TestHandleOwner_MissingArg(t *testing.T) {
	r := &recordingResponder{}
	if err := handleOwner(&cmdInput{snap: ownerTeamlistSnap(), resp: r}); err != nil {
		t.Fatalf("handleOwner: %v", err)
	}
	if len(r.messages) != 1 || r.messages[0] == "" {
		t.Errorf("missing-arg reply = %v", r.messages)
	}
}

func TestHandleOwner_NoSnapshotReportsStartingUp(t *testing.T) {
	r := &recordingResponder{}
	if err := handleOwner(&cmdInput{snap: nil, opts: cmdOptions{"player": "Højlund"}, resp: r}); err != nil {
		t.Fatalf("handleOwner: %v", err)
	}
	if !strings.Contains(strings.ToLower(r.messages[0]), "starting up") {
		t.Errorf("nil-snapshot reply = %q", r.messages[0])
	}
}
