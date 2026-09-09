package bot

import (
	"fmt"
	"strings"
	"time"

	"github.com/UberJoe/fpldiscord/internal/fpl"
	"github.com/bwmarrin/discordgo"
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
	e := renderTeamlist(in.snap.LeagueName, in.snap.BuiltAt, in.snap.CurrentGW, display, squad)
	return in.resp.RespondEmbeds([]*discordgo.MessageEmbed{e})
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

// teamlistPlaceholder fills a position with no players so the full four-row
// squad shape is always visible in the embed.
const teamlistPlaceholder = "—"

// renderTeamlist frames a manager's squad as one embed built on the shared
// scaffold: the league name on the author line (when the snapshot has one), a
// neutral colour bar (a squad carries no live / settled axis), a "GW{n}" footer
// and the snapshot build time as the Timestamp. The squad is laid out as four
// non-inline fields in fixed GK → DEF → MID → FWD order — one labelled block per
// position — so nothing wraps or side-scrolls on mobile.
//
// Each field value is that position's players in bootstrap-static order, each
// formatted by playerLabel (web name + club short name) and joined by ", "; a
// position with no players shows teamlistPlaceholder so all four rows are always
// visible. One manager's squad is at most 15 players across four fields,
// comfortably inside every embed limit, so there is nothing to chunk.
func renderTeamlist(leagueName string, builtAt time.Time, gw int, owner string, squad []fpl.TeamPlayer) *discordgo.MessageEmbed {
	byPos := map[fpl.Pos][]string{}
	for _, p := range squad {
		byPos[p.Pos] = append(byPos[p.Pos], playerLabel(p))
	}

	e := dataEmbed(
		leagueName,
		builtAt,
		fmt.Sprintf("%s's squad", owner),
		colorNeutral,
		fmt.Sprintf("GW%d", gw),
	)
	for _, g := range teamlistGroups {
		names := teamlistPlaceholder
		if len(byPos[g.pos]) > 0 {
			names = strings.Join(byPos[g.pos], ", ")
		}
		e.Fields = append(e.Fields, &discordgo.MessageEmbedField{Name: g.label, Value: names})
	}
	return e
}
