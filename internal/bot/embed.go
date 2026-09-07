package bot

import (
	"time"

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
//   - maxEmbedFieldValue   characters in one field value.
//   - maxEmbedFields        fields per embed.
//   - maxEmbedsPerMessage   embeds carried by one message.
//   - maxMessageEmbedChars  combined character total across a message's embeds
//     (Discord's hard ceiling; the /overview chunker keeps a margin below it).
const (
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
