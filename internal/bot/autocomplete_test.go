package bot

import (
	"strconv"
	"testing"
	"time"

	"github.com/UberJoe/fpldiscord/internal/fpl"
	"github.com/bwmarrin/discordgo"
)

// teamlistSnapBuiltAt is the build time ownerTeamlistSnap carries, so the
// /teamlist embed's Timestamp is deterministic.
var teamlistSnapBuiltAt = time.Date(2026, 9, 9, 12, 0, 0, 0, time.UTC)

// ownerTeamlistSnap is the shared seam-4 fixture for /owner, /teamlist and
// their autocomplete: five players across two clubs, owned by Bruno / Ian /
// nobody, with the accent-stripped autocomplete slices pre-populated the way
// the refresher builds them.
func ownerTeamlistSnap() *fpl.Snapshot {
	bruno := fpl.EntryID(39880)
	ian := fpl.EntryID(89)
	return &fpl.Snapshot{
		BuiltAt:    teamlistSnapBuiltAt,
		LeagueName: "Coq au Ian",
		CurrentGW:  7,
		Bootstrap: fpl.Bootstrap{
			Elements: []fpl.Element{
				{ID: 11, WebName: "Højlund", ElementType: int(fpl.PosFWD), Team: 1},
				{ID: 12, WebName: "Muñoz", ElementType: int(fpl.PosDEF), Team: 2},
				{ID: 13, WebName: "Raya", ElementType: int(fpl.PosGK), Team: 1},
				{ID: 14, WebName: "Sánchez", ElementType: int(fpl.PosGK), Team: 2},
				{ID: 15, WebName: "Gabriel", ElementType: int(fpl.PosDEF), Team: 1},
			},
			Teams: []fpl.Team{{ID: 1, ShortName: "ARS"}, {ID: 2, ShortName: "CRY"}},
		},
		LeagueDetails: fpl.LeagueDetails{
			League: fpl.League{Scoring: "c"},
			LeagueEntries: []fpl.LeagueEntry{
				{ID: 39936, EntryID: bruno, EntryName: "Bruno Dos Tres", PlayerFirstName: "Bruno"},
				{ID: 89, EntryID: ian, EntryName: "Coq au Vin", PlayerFirstName: "Ian"},
			},
		},
		ElementStatus: []fpl.ElementStatus{
			{Element: 11, Owner: &bruno},
			{Element: 12, Owner: &ian},
			{Element: 13, Owner: nil},
			{Element: 14, Owner: &ian},
			{Element: 15, Owner: &ian},
		},
		PlayerNames: []fpl.PlayerName{
			{ID: 11, WebName: "Højlund", Stripped: "Hojlund"},
			{ID: 12, WebName: "Muñoz", Stripped: "Munoz"},
			{ID: 13, WebName: "Raya", Stripped: "Raya"},
			{ID: 14, WebName: "Sánchez", Stripped: "Sanchez"},
			{ID: 15, WebName: "Gabriel", Stripped: "Gabriel"},
		},
		OwnerNames: []fpl.OwnerName{
			{EntryID: bruno, Name: "Bruno", Stripped: "Bruno"},
			{EntryID: ian, Name: "Ian", Stripped: "Ian"},
		},
	}
}

func choiceLabels(choices []*discordgo.ApplicationCommandOptionChoice) []string {
	out := make([]string, len(choices))
	for i, c := range choices {
		out[i] = c.Name
	}
	return out
}

func TestResolveAutocomplete_AccentAndCaseInsensitiveSubstring(t *testing.T) {
	cands := playerCandidates(ownerTeamlistSnap())

	// "hoj" (ASCII, lower) must reach "Højlund".
	got := choiceLabels(resolveAutocomplete(cands, "hoj"))
	if len(got) != 1 || got[0] != "Højlund" {
		t.Errorf("resolveAutocomplete(_, \"hoj\") = %v, want [Højlund]", got)
	}

	// A mid-string, differently-cased fragment still matches.
	got = choiceLabels(resolveAutocomplete(cands, "NCH"))
	if len(got) != 1 || got[0] != "Sánchez" {
		t.Errorf("resolveAutocomplete(_, \"NCH\") = %v, want [Sánchez]", got)
	}
}

func TestResolveAutocomplete_BlankReturnsEveryCandidate(t *testing.T) {
	cands := playerCandidates(ownerTeamlistSnap())
	got := resolveAutocomplete(cands, "  ")
	if len(got) != len(cands) {
		t.Errorf("blank partial returned %d choices, want all %d", len(got), len(cands))
	}
}

func TestResolveAutocomplete_PrefixMatchesRankAboveMidString(t *testing.T) {
	cands := []acCandidate{
		{label: "McRae", stripped: "McRae"},       // "ra" is mid-string
		{label: "Rashford", stripped: "Rashford"}, // "ra" is a prefix
	}
	got := choiceLabels(resolveAutocomplete(cands, "ra"))
	if len(got) != 2 || got[0] != "Rashford" || got[1] != "McRae" {
		t.Errorf("prefix should rank first: got %v", got)
	}
}

func TestResolveAutocomplete_CapsAtDiscordLimit(t *testing.T) {
	cands := make([]acCandidate, 0, 40)
	for i := 0; i < 40; i++ {
		s := "Player" + strconv.Itoa(i)
		cands = append(cands, acCandidate{label: s, stripped: s})
	}
	if got := resolveAutocomplete(cands, "player"); len(got) != maxACChoices {
		t.Errorf("got %d choices, want the %d cap", len(got), maxACChoices)
	}
}

func TestOwnerCandidates_DedupesOwnerFirstNames(t *testing.T) {
	snap := ownerTeamlistSnap()
	snap.OwnerNames = append(snap.OwnerNames, fpl.OwnerName{EntryID: 7, Name: "Ian", Stripped: "Ian"})
	got := choiceLabels(resolveAutocomplete(ownerCandidates(snap), ""))
	if len(got) != 2 {
		t.Errorf("owner candidates = %v, want Bruno + Ian with no duplicate", got)
	}
}

func TestAutocompleteHandlers_DrawFromTheRightSlice(t *testing.T) {
	snap := ownerTeamlistSnap()

	players := choiceLabels(autocompletePlayer(snap, "player", "a"))
	if len(players) == 0 {
		t.Fatal("autocompletePlayer returned nothing for \"a\"")
	}
	for _, p := range players {
		if p == "Bruno" || p == "Ian" {
			t.Errorf("autocompletePlayer leaked an owner name: %v", players)
		}
	}

	owners := choiceLabels(autocompleteOwner(snap, "owner", "i"))
	if len(owners) != 1 || owners[0] != "Ian" {
		t.Errorf("autocompleteOwner(_, \"i\") = %v, want [Ian]", owners)
	}
}
