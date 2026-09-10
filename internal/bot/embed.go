package bot

import (
	"time"
	"unicode/utf8"

	"github.com/bwmarrin/discordgo"
)

// Embed colour-bar semantics, shared by every rich-data reply so /standings,
// /scores and /overview stay consistent and a palette change is a one-line
// edit. discordgo renders Color as a 24-bit RGB int; these are the Discord
// brand colours.
//
//   - colorNeutral      informational, carries no "can this still move" signal:
//     /standings, /overview before kickoff.
//   - colorProvisional  live / still-moving: /scores while the gameweek is in
//     progress, /overview when any shown fixture is live.
//   - colorFinal        settled: /scores once the gameweek is finished,
//     /overview when every shown fixture is finished.
const (
	colorNeutral     = 0x5865F2 // blurple
	colorProvisional = 0xFAA61A // amber
	colorFinal       = 0x3BA55D // green
)

// Discord's embed limits, enforced in code so a large league or a double
// gameweek never produces a reply Discord rejects. Values from the Discord
// developer docs (see .scratch/discord-embeds/research/01-embed-presentation.md).
//
//   - maxEmbedFieldName    characters in one field name.
//   - maxEmbedFieldValue   characters in one field value.
//   - maxEmbedFields        fields per embed.
//   - maxEmbedsPerMessage   embeds carried by one message.
//   - maxMessageEmbedChars  combined character total across a message's embeds
//     (Discord's hard ceiling; the /overview chunker keeps a margin below it).
const (
	maxEmbedFieldName    = 256
	maxEmbedFieldValue   = 1024
	maxEmbedFields       = 25
	maxEmbedsPerMessage  = 10
	maxMessageEmbedChars = 6000
)

// dataEmbed builds the shared scaffold every /standings, /scores and /overview
// reply starts from: the league name on the author line (when leagueName is
// non-empty), the given title and colour bar, the caller-supplied footer line
// (gameweek context for /standings and /scores; the league name for /overview,
// whose fields carry the fixtures instead), and the snapshot build time as the
// embed Timestamp so each client renders its own localised "last updated".
// Callers fill in Description or Fields.
//
// It takes the two snapshot-derived scalars directly rather than the whole
// *fpl.Snapshot, matching renderStandings / renderScores in this package and
// keeping the helper nil-safe.
func dataEmbed(leagueName string, builtAt time.Time, title string, color int, footer string) *discordgo.MessageEmbed {
	e := &discordgo.MessageEmbed{
		Title: title,
		Color: color,
	}
	if !builtAt.IsZero() {
		e.Timestamp = builtAt.UTC().Format(time.RFC3339)
	}
	if leagueName != "" {
		e.Author = &discordgo.MessageEmbedAuthor{Name: leagueName}
	}
	if footer != "" {
		e.Footer = &discordgo.MessageEmbedFooter{Text: footer}
	}
	return e
}

// codeBlock wraps an aligned table body in a triple-backtick fence — the one
// Discord primitive that renders fixed-width on every client. Every rich-data
// reply that carries a tabwriter / space-padded table goes through here so the
// fence lives in exactly one place.
func codeBlock(body string) string {
	return "```\n" + body + "\n```"
}

// colWidth is the display width, in runes, of the widest of a column header and
// its cells. Shared by every hand-rolled fixed-width table body (/standings,
// /waivers) so a capped multi-byte cell still aligns — fmt string widths count
// runes, not bytes.
func colWidth(header string, cells []string) int {
	w := utf8.RuneCountInString(header)
	for _, c := range cells {
		w = max(w, utf8.RuneCountInString(c))
	}
	return w
}

// capRunes returns s unchanged when it is at most n runes wide, otherwise its
// first n-1 runes followed by a single-character ellipsis. It is the shared
// mobile-width cap for a code-block column (/standings team name, /waivers owner)
// and for an embed field value pre-truncated to maxEmbedFieldValue.
func capRunes(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n-1]) + "…"
}

// embedMessageCharBudget is the per-message character total an embedFieldChunker
// stops adding fields at. It sits a little below Discord's hard
// maxMessageEmbedChars ceiling: the running count tracks field names, field
// values, the title and the footer / author line — almost the whole of what
// Discord sums — and the margin absorbs the rest (newline and grapheme-vs-rune
// counting differences) so a real message can never land over.
const embedMessageCharBudget = maxMessageEmbedChars - 200

// embedFieldScaffold carries the dataEmbed parameters an embedFieldChunker
// stamps on the embeds it builds. The league name goes on the author line, the
// footer line, or both, purely by how the caller fills these in:
//
//   - /overview passes leagueName:"" and footer:<league name> — the fields carry
//     the fixtures, so the league name rides the footer and there is no author.
//   - /waivers and /bet pass leagueName:<league name> and a footer that names
//     the view (the result mode, the season) — league name on the author line,
//     state cue in the footer.
//
// titleFirstOnly stamps title on the first embed of the whole reply only (every
// later embed is untitled); with it false the title repeats on every embed.
type embedFieldScaffold struct {
	leagueName     string
	builtAt        time.Time
	title          string
	color          int
	footer         string
	titleFirstOnly bool
}

// embedFieldChunker packs a stream of non-inline (name, value) fields into
// Discord embeds and embeds into messages, respecting three limits at once: at
// most maxEmbedFields fields per embed, at most maxEmbedsPerMessage embeds per
// message, and a per-message character budget kept clear of the hard ceiling.
// Every embed carries the state colour bar and the scaffold's author / footer;
// titling follows embedFieldScaffold.titleFirstOnly. Field values are expected
// pre-truncated to maxEmbedFieldValue by the caller (as /overview does with
// capRunes).
//
// It is the one field-chunking mechanism shared by /overview, /waivers and /bet
// so the packing logic cannot drift between them. Build it with
// newEmbedFieldChunker, feed it with add, close it with flushMessage, then read
// messages — one []*discordgo.MessageEmbed per Discord message.
type embedFieldChunker struct {
	scaffold embedFieldScaffold
	title    string // working copy; cleared after the first embed when titleFirstOnly

	messages [][]*discordgo.MessageEmbed
	msg      []*discordgo.MessageEmbed // embeds accumulated for the current message
	cur      *discordgo.MessageEmbed   // embed accumulating fields
	msgChars int                       // approx character total across msg + cur
}

// newEmbedFieldChunker returns a chunker that will stamp every embed from the
// given scaffold.
func newEmbedFieldChunker(s embedFieldScaffold) *embedFieldChunker {
	return &embedFieldChunker{scaffold: s, title: s.title}
}

// add appends one field, opening a new embed or message first whenever this
// field would breach a limit.
func (c *embedFieldChunker) add(name, value string) {
	fieldChars := utf8.RuneCountInString(name) + utf8.RuneCountInString(value)

	if c.cur != nil && c.msgChars+fieldChars > embedMessageCharBudget {
		c.flushMessage()
	}
	if c.cur == nil {
		c.cur = dataEmbed(c.scaffold.leagueName, c.scaffold.builtAt, c.title, c.scaffold.color, c.scaffold.footer)
		c.msgChars += utf8.RuneCountInString(c.title) +
			utf8.RuneCountInString(c.scaffold.footer) +
			utf8.RuneCountInString(c.scaffold.leagueName)
		if c.scaffold.titleFirstOnly {
			c.title = "" // the rest of the reply's embeds are untitled
		}
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
func (c *embedFieldChunker) flushEmbed() {
	if c.cur != nil {
		c.msg = append(c.msg, c.cur)
		c.cur = nil
	}
}

// flushMessage closes the current message, folding in any in-progress embed
// first, and resets the character budget for the next one.
func (c *embedFieldChunker) flushMessage() {
	c.flushEmbed()
	if len(c.msg) > 0 {
		c.messages = append(c.messages, c.msg)
		c.msg, c.msgChars = nil, 0
	}
}
