package bot

import (
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/UberJoe/fpldiscord/internal/fpl"
	"github.com/bwmarrin/discordgo"
)

// standingsNameCap bounds the team-name column so a table line stays within the
// ~30–34 characters a Discord mobile client shows before it wraps or
// side-scrolls. With the rank, Official-total and gameweek columns and their
// two-space gutters, that leaves roughly half the budget for the name; 14 runes
// fits the league names seen so far and truncates the rest with an ellipsis.
//
// Provisional: set from the research note's 12–14 estimate, not yet eyeballed on
// a real mobile build (see .scratch/discord-embeds/issues/02-standings-embed.md
// comments). Widen or narrow once that check happens.
const standingsNameCap = 14

// handleStandings renders the classic total-points league table as a single
// framed embed: a neutral colour bar, the league name on the author line, the
// aligned rank / team / total / gameweek table in a fenced code block, and a
// footer carrying the gameweek context plus a client-localised "last updated"
// from the snapshot build time.
//
// It is a classic-only path: an h2h league gets a short plain-text "not
// available" reply rather than the h2h standings variant, which this season does
// not build. The still-starting-up reply also stays plain text.
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

	e := renderStandings(in.snap.LeagueName, in.snap.BuiltAt, in.snap.CurrentGW, rows)
	return in.resp.RespondEmbeds([]*discordgo.MessageEmbed{e})
}

// renderStandings builds the /standings embed: the shared scaffold (league name
// on the author line, neutral colour bar, gameweek-context footer, snapshot-time
// Timestamp) with the aligned table as a fenced code block in the description.
func renderStandings(leagueName string, builtAt time.Time, gw int, rows []fpl.StandingRow) *discordgo.MessageEmbed {
	e := dataEmbed(
		leagueName,
		builtAt,
		"Standings",
		colorNeutral,
		fmt.Sprintf("GW%d · total points league", gw),
	)
	e.Description = codeBlock(standingsTable(rows))
	return e
}

// standingsTable lays the classic table out as space-padded fixed-width columns
// for a monospace code block: rank, capped team name, the Draft Official total,
// then that gameweek's official event total. Rank and the two numeric columns
// are right-aligned; the team column is left-aligned and capped at
// standingsNameCap. It returns the bare body with no code fence — handleStandings
// wraps it via codeBlock so the fence lives in exactly one place.
//
// A hand-rolled column layout rather than text/tabwriter (which the plain-text
// version used and which ADR 0002 calls "unchanged"): tabwriter measures width
// in bytes, so a multi-byte capped name misaligns, and it cannot right-align the
// numeric columns while left-aligning the name. See the ticket comments.
//
// No movement-arrow column: Discord /standings passes the Draft rank straight
// through, and arrows are a web-only indicator (see
// docs/adr/0001-standings-movement-arrow-week-over-week.md).
func standingsTable(rows []fpl.StandingRow) string {
	rankCol := make([]string, len(rows))
	teamCol := make([]string, len(rows))
	totCol := make([]string, len(rows))
	gwCol := make([]string, len(rows))
	for i, r := range rows {
		rankCol[i] = strconv.Itoa(r.Rank)
		teamCol[i] = capRunes(r.EntryName, standingsNameCap)
		totCol[i] = strconv.Itoa(r.Total)
		gwCol[i] = strconv.Itoa(r.EventTotal)
	}

	rankW := colWidth("#", rankCol)
	teamW := colWidth("Team", teamCol)
	totW := colWidth("Tot", totCol)
	gwW := colWidth("GW", gwCol)

	var b strings.Builder
	// fmt string widths count runes, so a capped multi-byte name still aligns.
	fmt.Fprintf(&b, "%*s  %-*s  %*s  %*s", rankW, "#", teamW, "Team", totW, "Tot", gwW, "GW")
	for i := range rows {
		fmt.Fprintf(&b, "\n%*s  %-*s  %*s  %*s",
			rankW, rankCol[i], teamW, teamCol[i], totW, totCol[i], gwW, gwCol[i])
	}
	return b.String()
}

// colWidth is the display width, in runes, of the widest of a column header and
// its cells.
func colWidth(header string, cells []string) int {
	w := utf8.RuneCountInString(header)
	for _, c := range cells {
		w = max(w, utf8.RuneCountInString(c))
	}
	return w
}

// capRunes returns s unchanged when it is at most n runes wide, otherwise its
// first n-1 runes followed by a single-character ellipsis.
func capRunes(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n-1]) + "…"
}
