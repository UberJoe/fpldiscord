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
// side-scrolls. With the rank, live-total, GW and arrow columns and their
// two-space gutters, that leaves roughly half the budget for the name; 14 runes
// fits the league names seen so far and truncates the rest with an ellipsis.
//
// Provisional: set from the research note's 12–14 estimate, not yet eyeballed on
// a real mobile build (see .scratch/discord-embeds/issues/02-standings-embed.md
// comments). Widen or narrow once that check happens.
const standingsNameCap = 14

// handleStandings renders the classic live-standings table as a single framed
// embed: a neutral colour bar, the league name on the author line, the aligned
// rank / team / total / gameweek / arrow table in a fenced code block, and a
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
	rows := in.snap.LiveStandings()
	if len(rows) == 0 {
		return in.resp.Respond("No standings available yet.")
	}

	e := renderStandings(in.snap.LeagueName, in.snap.BuiltAt, in.snap.CurrentGW, in.snap.MatchLive(), rows)
	return in.resp.RespondEmbeds([]*discordgo.MessageEmbed{e})
}

// renderStandings builds the /standings embed: the shared scaffold (league name
// on the author line, neutral colour bar, gameweek-context footer, snapshot-time
// Timestamp) with the aligned live-standings table as a fenced code block in the
// description.
func renderStandings(leagueName string, builtAt time.Time, gw int, matchLive bool, rows []fpl.LiveStandingRow) *discordgo.MessageEmbed {
	e := dataEmbed(
		leagueName,
		builtAt,
		"Standings",
		colorNeutral,
		fmt.Sprintf("GW%d · total points league", gw),
	)
	e.Description = codeBlock(standingsTable(rows, matchLive))
	return e
}

// standingsTable lays the live-ordered classic table out as space-padded
// fixed-width columns for a monospace code block: live rank, capped team name,
// the live total (frozen total plus live gameweek score, auto-subs applied),
// this gameweek's points in the `+N` / `+0` / `–` convention, and the
// week-over-week movement arrow (▲/▼/–). Rank, the numeric columns and the
// arrow are right-aligned; the team column is left-aligned and capped at
// standingsNameCap. It returns the bare body with no code fence — handleStandings
// wraps it via codeBlock so the fence lives in exactly one place.
//
// A hand-rolled column layout rather than text/tabwriter (which the plain-text
// version used and which ADR 0002 calls "unchanged"): tabwriter measures width
// in bytes, so a multi-byte capped name misaligns, and it cannot right-align the
// numeric columns while left-aligning the name. See the ticket comments.
//
// Row order and rank come from LiveRank (joint ranking, ties broken by the
// Draft strict-sort field), not the Draft official rank — see
// docs/adr/0001-standings-movement-arrow-week-over-week.md, which this command
// now shares with the web view.
func standingsTable(rows []fpl.LiveStandingRow, matchLive bool) string {
	gwStarted := matchLive
	if !gwStarted {
		for _, r := range rows {
			if r.LiveGwPoints != 0 {
				gwStarted = true
				break
			}
		}
	}

	rankCol := make([]string, len(rows))
	teamCol := make([]string, len(rows))
	totCol := make([]string, len(rows))
	gwCol := make([]string, len(rows))
	arrowCol := make([]string, len(rows))
	for i, r := range rows {
		rankCol[i] = strconv.Itoa(r.LiveRank)
		teamCol[i] = capRunes(r.EntryName, standingsNameCap)
		totCol[i] = strconv.Itoa(r.LivePoints)
		gwCol[i] = gwCell(r.LiveGwPoints, gwStarted)
		arrowCol[i] = arrowCell(r.Arrow)
	}

	rankW := colWidth("#", rankCol)
	teamW := colWidth("Team", teamCol)
	totW := colWidth("Tot", totCol)
	gwW := colWidth("GW", gwCol)
	arrowW := colWidth("", arrowCol)

	var b strings.Builder
	// fmt string widths count runes, so a capped multi-byte name still aligns.
	fmt.Fprintf(&b, "%*s  %-*s  %*s  %*s  %*s", rankW, "#", teamW, "Team", totW, "Tot", gwW, "GW", arrowW, "")
	for i := range rows {
		fmt.Fprintf(&b, "\n%*s  %-*s  %*s  %*s  %*s",
			rankW, rankCol[i], teamW, teamCol[i], totW, totCol[i], gwW, gwCol[i], arrowW, arrowCol[i])
	}
	return b.String()
}

// gwCell renders a gameweek points value the same way the web view does: `+N`
// when the manager has current-gameweek points, `+0` once the gameweek has
// started but they don't, and `–` between gameweeks (see
// .scratch/standings-live-total/spec.md). `%+d` (not a hand-rolled "+" prefix)
// so a genuinely negative gameweek — a scoring XI dragged below zero by red
// cards/own goals — renders "-3", not "+-3".
func gwCell(liveGwPoints int, gwStarted bool) string {
	if liveGwPoints != 0 {
		return fmt.Sprintf("%+d", liveGwPoints)
	}
	if gwStarted {
		return "+0"
	}
	return "–"
}

// arrowCell renders the week-over-week movement arrow: ▲N climbed, ▼N dropped,
// – flat (including no previous position — see ADR 0001).
func arrowCell(n int) string {
	switch {
	case n > 0:
		return fmt.Sprintf("▲%d", n)
	case n < 0:
		return fmt.Sprintf("▼%d", -n)
	default:
		return "–"
	}
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
