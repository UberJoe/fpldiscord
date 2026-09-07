package bot

import (
	"sort"
	"strings"

	"github.com/UberJoe/fpldiscord/internal/fpl"
	"github.com/bwmarrin/discordgo"
)

// maxACChoices is Discord's hard cap on the number of choices an autocomplete
// response may carry.
const maxACChoices = 25

// acHandlerFunc resolves one command's focused autocomplete option to a set of
// choices. focused names the option the user is editing (so a command with
// several autocompleting args — /bet set later — can tell its slots apart);
// partial is the text typed so far. It is a pure function of the snapshot so it
// is exercised directly at the seam-4 boundary.
type acHandlerFunc func(snap *fpl.Snapshot, focused, partial string) []*discordgo.ApplicationCommandOptionChoice

// acCandidate is one thing an autocomplete argument can resolve to: the label
// the user sees and picks, plus the accent-stripped form it is matched against
// (folded to lower case at match time).
type acCandidate struct {
	label    string
	stripped string
}

// playerCandidates / ownerCandidates adapt the snapshot's pre-stripped
// autocomplete slices to the shared resolver. Player args come from
// elements[].web_name; owner args from league_entries[].player_first_name —
// the two argument kinds every autocompleting command uses.
func playerCandidates(snap *fpl.Snapshot) []acCandidate {
	out := make([]acCandidate, 0, len(snap.PlayerNames))
	for _, p := range snap.PlayerNames {
		out = append(out, acCandidate{label: p.WebName, stripped: p.Stripped})
	}
	return out
}

func ownerCandidates(snap *fpl.Snapshot) []acCandidate {
	seen := make(map[string]bool, len(snap.OwnerNames))
	out := make([]acCandidate, 0, len(snap.OwnerNames))
	for _, o := range snap.OwnerNames {
		if o.Name == "" || seen[o.Name] {
			continue
		}
		seen[o.Name] = true
		out = append(out, acCandidate{label: o.Name, stripped: o.Stripped})
	}
	return out
}

// resolveAutocomplete is the reusable arg-resolver: it returns the Discord
// choices for a partial argument value — every candidate whose accent-stripped
// label contains the accent-stripped, case-folded partial, prefix matches
// first, capped at Discord's 25. A blank partial yields the first 25 candidates
// so the menu is never empty.
func resolveAutocomplete(candidates []acCandidate, partial string) []*discordgo.ApplicationCommandOptionChoice {
	q := fold(partial)

	type hit struct {
		label string
		rank  int // 0 = prefix match, 1 = mid-string match
	}
	var hits []hit
	for _, c := range candidates {
		s := strings.ToLower(c.stripped)
		switch {
		case q == "":
			hits = append(hits, hit{c.label, 1})
		case strings.HasPrefix(s, q):
			hits = append(hits, hit{c.label, 0})
		case strings.Contains(s, q):
			hits = append(hits, hit{c.label, 1})
		}
	}
	sort.SliceStable(hits, func(i, j int) bool { return hits[i].rank < hits[j].rank })
	if len(hits) > maxACChoices {
		hits = hits[:maxACChoices]
	}

	out := make([]*discordgo.ApplicationCommandOptionChoice, len(hits))
	for i, h := range hits {
		out[i] = &discordgo.ApplicationCommandOptionChoice{Name: h.label, Value: h.label}
	}
	return out
}

// autocompletePlayer / autocompleteOwner are the acHandlerFuncs for ticket 07's
// two commands. Both defer to the shared resolver; the focused-option name is
// unused while each command has a single autocompleting arg.
func autocompletePlayer(snap *fpl.Snapshot, _, partial string) []*discordgo.ApplicationCommandOptionChoice {
	return resolveAutocomplete(playerCandidates(snap), partial)
}

func autocompleteOwner(snap *fpl.Snapshot, _, partial string) []*discordgo.ApplicationCommandOptionChoice {
	return resolveAutocomplete(ownerCandidates(snap), partial)
}
