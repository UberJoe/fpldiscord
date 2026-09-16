package bot

import (
	"strings"
	"testing"
	"time"

	"github.com/UberJoe/fpldiscord/internal/fpl"
	"github.com/bwmarrin/discordgo"
)

// teamlistEmbed runs handleTeamlist against a recordingResponder and returns the
// single embed it emitted, failing the test if the handler sent anything else.
func teamlistEmbed(t *testing.T, in *cmdInput) *discordgo.MessageEmbed {
	t.Helper()
	r := &recordingResponder{}
	in.resp = r
	if err := handleTeamlist(in); err != nil {
		t.Fatalf("handleTeamlist: %v", err)
	}
	if len(r.messages) != 0 {
		t.Fatalf("plain messages = %v, want none — /teamlist replies with an embed", r.messages)
	}
	if len(r.embeds) != 1 || len(r.embeds[0]) != 1 {
		t.Fatalf("embeds = %v, want exactly one message carrying one embed", r.embeds)
	}
	return r.embeds[0][0]
}

// fieldByName returns the value of the named embed field, or "" if absent.
func fieldByName(e *discordgo.MessageEmbed, name string) string {
	for _, f := range e.Fields {
		if f.Name == name {
			return f.Value
		}
	}
	return ""
}

func TestHandleTeamlist_OneEmbedWithFourPositionFieldsInOrder(t *testing.T) {
	e := teamlistEmbed(t, &cmdInput{snap: ownerTeamlistSnap(), opts: cmdOptions{"owner": "Ian"}})

	if e.Title != "Ian's squad" {
		t.Errorf("title = %q, want %q", e.Title, "Ian's squad")
	}
	if e.Color != colorNeutral {
		t.Errorf("color = %#x, want neutral %#x — a squad has no live/settled axis", e.Color, colorNeutral)
	}
	if e.Author == nil || e.Author.Name != "Coq au Ian" {
		t.Errorf("author = %+v, want the league name", e.Author)
	}
	if e.Footer == nil || e.Footer.Text != "GW7" {
		t.Errorf("footer = %+v, want %q", e.Footer, "GW7")
	}
	if want := teamlistSnapBuiltAt.Format(time.RFC3339); e.Timestamp != want {
		t.Errorf("timestamp = %q, want the snapshot build time %q", e.Timestamp, want)
	}

	if len(e.Fields) != 4 {
		t.Fatalf("fields = %d, want one per position", len(e.Fields))
	}
	for i, want := range []string{"GK", "DEF", "MID", "FWD"} {
		if e.Fields[i].Name != want {
			t.Errorf("field %d name = %q, want %q", i, e.Fields[i].Name, want)
		}
		if e.Fields[i].Inline {
			t.Errorf("field %d (%s) is inline, want a full-width block", i, want)
		}
	}
}

func TestHandleTeamlist_PositionFieldsCarryThatPositionsPlayers(t *testing.T) {
	e := teamlistEmbed(t, &cmdInput{snap: ownerTeamlistSnap(), opts: cmdOptions{"owner": "Ian"}})

	// Ian owns Sánchez (GK/CRY), Muñoz (DEF/CRY) and Gabriel (DEF/ARS).
	if gk := fieldByName(e, "GK"); !strings.Contains(gk, "Sánchez (CRY)") {
		t.Errorf("GK field = %q, want Sánchez", gk)
	}
	def := fieldByName(e, "DEF")
	for _, want := range []string{"Muñoz (CRY)", "Gabriel (ARS)"} {
		if !strings.Contains(def, want) {
			t.Errorf("DEF field = %q, missing %q", def, want)
		}
	}
	// Ian has no midfielders or forwards — those fields show the placeholder.
	if mid := fieldByName(e, "MID"); mid != teamlistPlaceholder {
		t.Errorf("MID field = %q, want the %q placeholder", mid, teamlistPlaceholder)
	}
	if fwd := fieldByName(e, "FWD"); fwd != teamlistPlaceholder {
		t.Errorf("FWD field = %q, want the %q placeholder", fwd, teamlistPlaceholder)
	}
	// A player he does not own must not appear anywhere in the embed.
	for _, f := range e.Fields {
		if strings.Contains(f.Value, "Højlund") {
			t.Errorf("teamlist leaked a player Ian doesn't own: %s = %q", f.Name, f.Value)
		}
	}
}

func TestHandleTeamlist_AccentAndCaseInsensitiveOwner(t *testing.T) {
	e := teamlistEmbed(t, &cmdInput{snap: ownerTeamlistSnap(), opts: cmdOptions{"owner": " ian "}})
	if e.Title != "Ian's squad" {
		t.Errorf("lower/whitespace owner should resolve; title = %q", e.Title)
	}
}

func TestHandleTeamlist_SharedFirstNamePicksTheFirstManager(t *testing.T) {
	snap := ownerTeamlistSnap()
	// A second "Ian" enters the league and owns Højlund (element 11).
	otherIan := fpl.EntryID(777)
	snap.LeagueDetails.LeagueEntries = append(snap.LeagueDetails.LeagueEntries,
		fpl.LeagueEntry{ID: 777, EntryID: otherIan, EntryName: "Ian's Other XI", PlayerFirstName: "Ian"})
	snap.ElementStatus[0] = fpl.ElementStatus{Element: 11, Owner: &otherIan}

	e := teamlistEmbed(t, &cmdInput{snap: snap, opts: cmdOptions{"owner": "Ian"}})

	// Højlund (element 11) is first in bootstrap order and now belongs to the
	// new Ian, so his one-player squad is shown — not a merge that also carries
	// the original Ian's Sánchez / Muñoz / Gabriel.
	if fwd := fieldByName(e, "FWD"); !strings.Contains(fwd, "Højlund (ARS)") {
		t.Errorf("expected the first-seen Ian's squad; FWD = %q", fwd)
	}
	for _, f := range e.Fields {
		for _, leaked := range []string{"Sánchez", "Muñoz", "Gabriel"} {
			if strings.Contains(f.Value, leaked) {
				t.Errorf("shared first name merged two squads (found %q in %s)", leaked, f.Name)
			}
		}
	}
}

func TestHandleTeamlist_UnknownOwnerStaysPlainText(t *testing.T) {
	r := &recordingResponder{}
	in := &cmdInput{snap: ownerTeamlistSnap(), opts: cmdOptions{"owner": "Zoe"}, resp: r}
	if err := handleTeamlist(in); err != nil {
		t.Fatalf("handleTeamlist: %v", err)
	}
	if len(r.embeds) != 0 {
		t.Fatalf("unknown owner should not produce an embed: %v", r.embeds)
	}
	if !strings.Contains(strings.ToLower(r.messages[0]), "no manager") {
		t.Errorf("unknown owner reply = %q", r.messages[0])
	}
}

func TestHandleTeamlist_NoSnapshotReportsStartingUpAsPlainText(t *testing.T) {
	r := &recordingResponder{}
	if err := handleTeamlist(&cmdInput{snap: nil, opts: cmdOptions{"owner": "Ian"}, resp: r}); err != nil {
		t.Fatalf("handleTeamlist: %v", err)
	}
	if len(r.embeds) != 0 {
		t.Fatalf("nil snapshot should not produce an embed: %v", r.embeds)
	}
	if !strings.Contains(strings.ToLower(r.messages[0]), "starting up") {
		t.Errorf("nil-snapshot reply = %q", r.messages[0])
	}
}
