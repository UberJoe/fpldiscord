package bot

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/UberJoe/fpldiscord/internal/fpl"
	"github.com/bwmarrin/discordgo"
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
	for _, embeds := range renderOverview(snap.LeagueName, snap.BuiltAt, snap.CurrentGW, mode, fixtures) {
		if err := in.resp.RespondEmbeds(embeds); err != nil {
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

// renderOverview lays each fixture out as one non-inline embed field — the score
// line as the field name, the goalscorer events (or a "no goals" note) as the
// field value — and chunks those fields across Discord messages with an
// overviewChunker. This replaces the old raw 2000-character text pagination with
// limit-aware chunking while keeping the "spill to another message" behaviour a
// big or double gameweek needs.
//
// The return is one []*discordgo.MessageEmbed per Discord message; the caller
// sends each with a single RespondEmbeds call. Fixtures come pre-sorted by
// kickoff.
func renderOverview(leagueName string, builtAt time.Time, gw int, mode overviewMode, fixtures []fpl.FixtureOverview) [][]*discordgo.MessageEmbed {
	c := overviewChunker{
		leagueName: leagueName,
		builtAt:    builtAt,
		title:      fmt.Sprintf("GW%d %s", gw, overviewTitle[mode]),
		color:      overviewColor(fixtures),
	}
	for _, f := range fixtures {
		c.add(overviewFixtureHeader(f), capRunes(overviewFixtureBody(f), maxEmbedFieldValue))
	}
	c.flushMessage()
	return c.messages
}

// overviewMessageCharBudget is the per-message character total the chunker stops
// adding fields at. It sits a little below Discord's hard maxMessageEmbedChars
// ceiling: the running count tracks field names, field values, the title and the
// footers — almost the whole of what Discord sums — and the margin absorbs the
// rest (newline and grapheme-vs-rune counting differences) so a real message can
// never land over.
const overviewMessageCharBudget = maxMessageEmbedChars - 200

// overviewChunker packs one embed field per fixture into Discord messages,
// respecting three limits at once: at most maxEmbedFields fields per embed, at
// most maxEmbedsPerMessage embeds per message, and a per-message character
// budget kept clear of the hard ceiling. Only the first embed of the whole reply
// is titled; every embed carries the state colour bar and the league-name
// footer. Field values are expected pre-truncated to the per-field limit.
type overviewChunker struct {
	leagueName string
	builtAt    time.Time
	title      string // stamped on the first embed, then cleared
	color      int

	messages [][]*discordgo.MessageEmbed
	msg      []*discordgo.MessageEmbed // embeds accumulated for the current message
	cur      *discordgo.MessageEmbed   // embed accumulating fields
	msgChars int                       // approx character total across msg + cur
}

// add appends one fixture field, opening a new embed or message first whenever
// this field would breach a limit.
func (c *overviewChunker) add(name, value string) {
	fieldChars := utf8.RuneCountInString(name) + utf8.RuneCountInString(value)

	if c.cur != nil && c.msgChars+fieldChars > overviewMessageCharBudget {
		c.flushMessage()
	}
	if c.cur == nil {
		c.cur = dataEmbed("", c.builtAt, c.title, c.color, c.leagueName)
		c.msgChars += utf8.RuneCountInString(c.title) + utf8.RuneCountInString(c.leagueName)
		c.title = "" // the rest of the reply's embeds are untitled
	}

	c.cur.Fields = append(c.cur.Fields, &discordgo.MessageEmbedField{Name: name, Value: value})
	c.msgChars += fieldChars

	if len(c.cur.Fields) >= maxEmbedFields {
		c.flushEmbed()
		if len(c.msg) >= maxEmbedsPerMessage {
			c.flushMessage()
		}
	}
}

// flushEmbed folds the in-progress embed into the current message.
func (c *overviewChunker) flushEmbed() {
	if c.cur != nil {
		c.msg = append(c.msg, c.cur)
		c.cur = nil
	}
}

// flushMessage closes the current message, folding in any in-progress embed
// first, and resets the character budget for the next one.
func (c *overviewChunker) flushMessage() {
	c.flushEmbed()
	if len(c.msg) > 0 {
		c.messages = append(c.messages, c.msg)
		c.msg, c.msgChars = nil, 0
	}
}

// overviewColor encodes the state of the shown fixtures on the embed colour bar:
// provisional if any fixture is live, final if every fixture has finished,
// neutral otherwise (nothing has kicked off yet, or a mix with none in play).
func overviewColor(fixtures []fpl.FixtureOverview) int {
	anyLive, allFinished := false, len(fixtures) > 0
	for _, f := range fixtures {
		finished := f.Finished || f.FinishedProvisional
		if f.Started && !finished {
			anyLive = true
		}
		if !finished {
			allFinished = false
		}
	}
	switch {
	case anyLive:
		return colorProvisional
	case allFinished:
		return colorFinal
	default:
		return colorNeutral
	}
}

// overviewFixtureBody is the field value for one fixture: its goalscorer event
// lines joined by newlines, or a "no goals" note when the fixture has none. The
// scorer-line formatting (and its emoji) is reused verbatim from the plain-text
// version.
func overviewFixtureBody(f fpl.FixtureOverview) string {
	if len(f.Scorers) == 0 {
		return "_no goals_"
	}
	lines := make([]string, len(f.Scorers))
	for i, sc := range f.Scorers {
		lines[i] = overviewScorerLine(sc)
	}
	return strings.Join(lines, "\n")
}

// overviewFixtureHeader is the match line used as a field name: "Home H - A Away"
// once the match has started (with a live / FT tag), or "Home vs Away"
// beforehand. Plain text — Discord renders no markdown in a field name.
func overviewFixtureHeader(f fpl.FixtureOverview) string {
	home, away := overviewTeam(f.TeamHome), overviewTeam(f.TeamAway)
	if !f.Started {
		return fmt.Sprintf("%s vs %s", home, away)
	}
	tag := " (live)"
	if f.Finished || f.FinishedProvisional {
		tag = " (FT)"
	}
	return fmt.Sprintf("%s %d - %d %s%s", home, f.HomeScore, f.AwayScore, away, tag)
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
