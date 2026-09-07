package bot

import (
	"fmt"
	"strings"

	"github.com/UberJoe/fpldiscord/internal/fpl"
)

// handleTeamlist lists a manager's whole squad grouped GK / DEF / MID / FWD.
// The owner arg is autocompleted from player_first_name; the squad and the
// per-player club come from fpl.TeamPlayers (live element-status).
//
// The owner is matched on first name. Should two managers share one (the
// autocomplete menu de-dupes them to a single entry), the first manager seen
// in bootstrap-static order wins — as the Python bot's get_team_id did with
// .iloc[0] — rather than merging both squads into one over-long list.
func handleTeamlist(in *cmdInput) error {
	if in.snap == nil {
		return in.resp.Respond("Team lists aren't available yet — the bot is still starting up.")
	}
	owner, _ := in.opts.String("owner")
	owner = strings.TrimSpace(owner)
	if owner == "" {
		return in.resp.Respond("Give me a manager's first name.")
	}

	want := fold(owner)
	display := owner
	var chosen fpl.EntryID
	var squad []fpl.TeamPlayer
	for _, p := range in.snap.TeamPlayers() {
		if p.OwnerName == "" || fold(p.OwnerName) != want {
			continue
		}
		if chosen == 0 {
			chosen = p.OwnerEntryID
			display = p.OwnerName
		}
		if p.OwnerEntryID == chosen {
			squad = append(squad, p)
		}
	}
	if len(squad) == 0 {
		return in.resp.Respond(fmt.Sprintf("No manager called %q, or they own no players.", owner))
	}
	return in.resp.Respond(renderTeamlist(display, squad))
}

// teamlistGroups is the fixed GK → DEF → MID → FWD order the squad is laid out
// in.
var teamlistGroups = []struct {
	label string
	pos   fpl.Pos
}{
	{"GK", fpl.PosGK},
	{"DEF", fpl.PosDEF},
	{"MID", fpl.PosMID},
	{"FWD", fpl.PosFWD},
}

// renderTeamlist prints one line per position — "LABEL  Name (CLUB), …" in
// bootstrap-static order within the group — inside a monospace block. A
// position with no players shows "—" so the full squad shape is visible. One
// manager's squad is at most 15 players, so the output never needs splitting
// across messages.
func renderTeamlist(owner string, squad []fpl.TeamPlayer) string {
	byPos := map[fpl.Pos][]string{}
	for _, p := range squad {
		byPos[p.Pos] = append(byPos[p.Pos], playerLabel(p))
	}

	var b strings.Builder
	fmt.Fprintf(&b, "**%s's squad**\n```\n", owner)
	for _, g := range teamlistGroups {
		names := "—"
		if len(byPos[g.pos]) > 0 {
			names = strings.Join(byPos[g.pos], ", ")
		}
		fmt.Fprintf(&b, "%-4s %s\n", g.label, names)
	}
	b.WriteString("```")
	return b.String()
}
