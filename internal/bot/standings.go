package bot

import (
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/UberJoe/fpldiscord/internal/fpl"
)

// handleStandings renders the classic total-points league table. It is a
// classic-only path: an h2h league gets a short "not available" reply rather
// than the h2h standings variant, which this season does not build.
func handleStandings(in *cmdInput) error {
	if in.snap == nil {
		return in.resp.Respond("Standings aren't available yet — the bot is still starting up.")
	}
	if in.snap.LeagueMode != fpl.ModeClassic {
		return in.resp.Respond("This league runs in head-to-head mode; the classic standings table isn't available.")
	}
	rows := in.snap.Standings()
	if len(rows) == 0 {
		return in.resp.Respond("No standings available yet.")
	}

	var b strings.Builder
	if name := in.snap.LeagueName; name != "" {
		fmt.Fprintf(&b, "**%s — Standings**\n", name)
	}
	b.WriteString(renderStandings(rows))
	return in.resp.Respond(b.String())
}

// renderStandings lays the table out in a monospace code block: rank, team
// name, season total, then this gameweek's total.
func renderStandings(rows []fpl.StandingRow) string {
	var b strings.Builder
	b.WriteString("```\n")
	tw := tabwriter.NewWriter(&b, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "#\tTeam\tTot\tGW")
	for _, r := range rows {
		fmt.Fprintf(tw, "%d\t%s\t%d\t%d\n", r.Rank, r.EntryName, r.Total, r.EventTotal)
	}
	tw.Flush()
	b.WriteString("```")
	return b.String()
}
