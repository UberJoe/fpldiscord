package bot

import (
	"fmt"
	"sort"
	"strings"
	"text/tabwriter"
)

// scoreRow is one manager's line in the /scores table.
type scoreRow struct {
	name   string
	points int
	scored bool // false when the snapshot cannot score this manager yet
}

// handleScores renders every league member's live / provisional gameweek points
// with auto-subs applied (fpl.ManagerScore trusts stats.total_points and
// stats.bonus as-is). The optional gw option defaults to the current gameweek;
// the MVP snapshot only carries the current GW, so any other value gets a short
// explanation rather than stale or empty numbers.
func handleScores(in *cmdInput) error {
	if in.snap == nil {
		return in.resp.Respond("Scores aren't available yet — the bot is still starting up.")
	}
	snap := in.snap

	gw := snap.CurrentGW
	if v, ok := in.opts.Int("gw"); ok && v != 0 {
		gw = v
	}
	if gw != snap.CurrentGW {
		return in.resp.Respond(fmt.Sprintf(
			"Live scores are only available for the current gameweek (GW %d).", snap.CurrentGW))
	}

	rows := make([]scoreRow, 0, len(snap.LeagueDetails.LeagueEntries))
	for _, le := range snap.LeagueDetails.LeagueEntries {
		r := scoreRow{name: le.EntryName}
		if ms, err := snap.ManagerScore(le.EntryID, gw); err == nil {
			r.points, r.scored = ms.Total, true
		}
		rows = append(rows, r)
	}
	if len(rows) == 0 {
		return in.resp.Respond("No managers in this league yet.")
	}

	// Highest gameweek total first; unscored managers sink to the bottom in
	// membership order.
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].scored != rows[j].scored {
			return rows[i].scored
		}
		return rows[i].points > rows[j].points
	})

	return in.resp.Respond(renderScores(snap.LeagueName, gw, snap.GWFinished, rows))
}

// renderScores lays the scores out in a monospace code block: team then this
// gameweek's points (auto-subs applied), highest first. The header carries a
// provisional / final tag so a reader knows whether the numbers can still move.
func renderScores(leagueName string, gw int, finished bool, rows []scoreRow) string {
	phase := "provisional"
	if finished {
		phase = "final"
	}

	var b strings.Builder
	if leagueName != "" {
		fmt.Fprintf(&b, "**%s — GW%d scores (%s)**\n", leagueName, gw, phase)
	} else {
		fmt.Fprintf(&b, "**GW%d scores (%s)**\n", gw, phase)
	}
	b.WriteString("```\n")
	tw := tabwriter.NewWriter(&b, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "Team\tGW")
	for _, r := range rows {
		if r.scored {
			fmt.Fprintf(tw, "%s\t%d\n", r.name, r.points)
		} else {
			fmt.Fprintf(tw, "%s\t-\n", r.name)
		}
	}
	tw.Flush()
	b.WriteString("```")
	return b.String()
}
