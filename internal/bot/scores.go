package bot

import (
	"fmt"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/bwmarrin/discordgo"
)

// scoreRow is one manager's line in the /scores table.
type scoreRow struct {
	name   string
	points int
	scored bool // false when the snapshot cannot score this manager yet
}

// handleScores renders every league member's live / provisional gameweek points
// with auto-subs applied (fpl.ManagerScore trusts stats.total_points and
// stats.bonus as-is) as a single framed embed: the league name on the author
// line, the team / live-gameweek-points table in a fenced code block, and a colour
// bar plus footer that say whether the gameweek is still provisional or final.
//
// The optional gw option defaults to the current gameweek; the MVP snapshot only
// carries the current GW, so any other value gets a short plain-text explanation
// rather than stale or empty numbers. The still-starting-up and empty-league
// replies also stay plain text.
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

	e := renderScores(snap.LeagueName, snap.BuiltAt, gw, snap.GWFinished, rows)
	return in.resp.RespondEmbeds([]*discordgo.MessageEmbed{e})
}

// renderScores builds the /scores embed on the shared dataEmbed scaffold. The
// colour bar and footer encode phase off the finished flag: amber /
// "provisional — scores can still move" while the gameweek is unfinished, green /
// "final" once it is. The table body goes in the description as a fenced code
// block.
func renderScores(leagueName string, builtAt time.Time, gw int, finished bool, rows []scoreRow) *discordgo.MessageEmbed {
	color := colorProvisional
	footer := "provisional — scores can still move"
	if finished {
		color = colorFinal
		footer = "final"
	}

	e := dataEmbed(leagueName, builtAt, fmt.Sprintf("GW%d scores", gw), color, footer)
	e.Description = codeBlock(scoresTable(rows))
	return e
}

// scoresTable lays the scores out as a monospace code-block body: team then the
// manager's live gameweek points (auto-subs applied), highest first, unscored
// managers shown as a dash. It returns the bare body with no code fence —
// renderScores wraps it via codeBlock so the fence lives in exactly one place.
//
// tabwriter here (not the hand-rolled layout /standings uses) because the two
// columns are simple: no capped multi-byte name and no mixed per-column
// alignment, the cases the ADR 0002 amendment carves out. tabwriter needs a
// trailing newline per row, so the whole body is right-trimmed before return.
func scoresTable(rows []scoreRow) string {
	var b strings.Builder
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
	return strings.TrimRight(b.String(), "\n")
}
