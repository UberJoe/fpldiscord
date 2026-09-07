package bot

import (
	"fmt"
	"strings"

	"github.com/UberJoe/fpldiscord/internal/fpl"
)

// overviewMode is the resolved /overview window.
type overviewMode string

const (
	overviewToday    overviewMode = "today"
	overviewGameweek overviewMode = "gameweek"
	overviewLive     overviewMode = "live"
)

// handleOverview renders the current gameweek's fixtures and goalscorers in one
// of three windows: today's fixtures, the whole gameweek (the default), or just
// the matches in play. The goalscorer list is goals / assists / own goals / red
// cards only — fpl.Overview has already dropped bonus, bps and the
// defensive_contribution family.
//
// Mode selection is a pure filter over the snapshot's per-fixture flags
// (Today / Started / FinishedProvisional), so there is no wall-clock call here:
// the Python original's datetime.utcnow() comparisons are gone.
func handleOverview(in *cmdInput) error {
	if in.snap == nil {
		return in.resp.Respond("The overview isn't available yet — the bot is still starting up.")
	}
	snap := in.snap

	mode := overviewGameweek
	if v, ok := in.opts.String("mode"); ok && strings.TrimSpace(v) != "" {
		switch overviewMode(strings.ToLower(strings.TrimSpace(v))) {
		case overviewToday:
			mode = overviewToday
		case overviewLive:
			mode = overviewLive
		case overviewGameweek:
			mode = overviewGameweek
		default:
			return in.resp.Respond(`The "mode" option must be today, gameweek or live.`)
		}
	}

	fixtures := filterOverview(snap.Overview(snap.CurrentGW), mode)
	if len(fixtures) == 0 {
		return in.resp.Respond(overviewEmptyReply(mode, snap.CurrentGW))
	}
	for _, msg := range renderOverview(snap.LeagueName, snap.CurrentGW, mode, fixtures) {
		if err := in.resp.Respond(msg); err != nil {
			return err
		}
	}
	return nil
}

// filterOverview narrows the gameweek's fixtures to the chosen window. Gameweek
// keeps everything; Today keeps fixtures the fpl layer flagged as kicking off on
// the snapshot's build date; Live keeps fixtures that have started and are not
// yet finished-provisional (matching fpl.MatchLive).
func filterOverview(fixtures []fpl.FixtureOverview, mode overviewMode) []fpl.FixtureOverview {
	var out []fpl.FixtureOverview
	for _, f := range fixtures {
		switch mode {
		case overviewToday:
			if !f.Today {
				continue
			}
		case overviewLive:
			if !f.Started || f.Finished || f.FinishedProvisional {
				continue
			}
		}
		out = append(out, f)
	}
	return out
}

// overviewEmptyReply explains an empty window without implying the bot is
// broken: an early week may have no fixtures today, and outside a live window
// nothing is in play.
func overviewEmptyReply(mode overviewMode, gw int) string {
	switch mode {
	case overviewToday:
		return fmt.Sprintf("No GW%d fixtures kick off today.", gw)
	case overviewLive:
		return fmt.Sprintf("No GW%d matches are in play right now.", gw)
	default:
		return fmt.Sprintf("No fixtures found for GW%d.", gw)
	}
}

var overviewTitle = map[overviewMode]string{
	overviewToday:    "today's fixtures",
	overviewGameweek: "gameweek fixtures",
	overviewLive:     "live fixtures",
}

// renderOverview lays each fixture out as a bold score line followed by its
// goalscorer events, one per line, and packs the fixtures into as few messages
// as possible under the Discord length limit — a full (or double) gameweek with
// a lot of goals can exceed 2000 characters. Only the first message carries the
// title. Fixtures come pre-sorted by kickoff.
func renderOverview(leagueName string, gw int, mode overviewMode, fixtures []fpl.FixtureOverview) []string {
	title := fmt.Sprintf("**GW%d %s**", gw, overviewTitle[mode])
	if leagueName != "" {
		title = fmt.Sprintf("**%s — GW%d %s**", leagueName, gw, overviewTitle[mode])
	}

	blocks := make([]string, 0, len(fixtures))
	for _, f := range fixtures {
		var b strings.Builder
		b.WriteString(overviewFixtureHeader(f))
		if len(f.Scorers) == 0 {
			b.WriteString("\n_no goals_")
		}
		for _, sc := range f.Scorers {
			b.WriteByte('\n')
			b.WriteString(overviewScorerLine(sc))
		}
		blocks = append(blocks, b.String())
	}

	var msgs []string
	cur := title
	for _, blk := range blocks {
		if len(cur)+len("\n\n")+len(blk) > maxDiscordMessage && cur != "" {
			msgs = append(msgs, cur)
			cur = ""
		}
		if cur == "" {
			cur = blk
		} else {
			cur += "\n\n" + blk
		}
	}
	if cur != "" {
		msgs = append(msgs, cur)
	}
	return msgs
}

// overviewFixtureHeader is the bold match line: "Home H - A Away" once the match
// has started (with a live / FT tag), or "Home vs Away" beforehand.
func overviewFixtureHeader(f fpl.FixtureOverview) string {
	home, away := overviewTeam(f.TeamHome), overviewTeam(f.TeamAway)
	if !f.Started {
		return fmt.Sprintf("**%s vs %s**", home, away)
	}
	tag := " _(live)_"
	if f.Finished || f.FinishedProvisional {
		tag = " _(FT)_"
	}
	return fmt.Sprintf("**%s %d - %d %s**%s", home, f.HomeScore, f.AwayScore, away, tag)
}

// overviewScorerLine is one goalscorer event: a marker (repeated for a brace),
// the player, and the owning manager when the player is owned in this league.
func overviewScorerLine(sc fpl.OverviewStat) string {
	n := sc.Value
	if n < 1 {
		n = 1
	}
	var text string
	switch sc.StatName {
	case fpl.OverviewGoal:
		text = strings.Repeat("⚽", n) + " " + overviewPlayer(sc.PlayerName)
	case fpl.OverviewAssist:
		text = strings.Repeat("🅰️", n) + " " + overviewPlayer(sc.PlayerName) + " (assist)"
	case fpl.OverviewOwnGoal:
		text = strings.Repeat("⚽", n) + " " + overviewPlayer(sc.PlayerName) + " (OG)"
	case fpl.OverviewRedCard:
		text = "🟥 " + overviewPlayer(sc.PlayerName)
	default:
		text = overviewPlayer(sc.PlayerName)
	}
	if sc.OwnerName != "" {
		text += " — " + sc.OwnerName
	}
	return text
}

func overviewTeam(name string) string {
	if name == "" {
		return "?"
	}
	return name
}

func overviewPlayer(name string) string {
	if name == "" {
		return "Unknown player"
	}
	return name
}
