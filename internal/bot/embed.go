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

// dataEmbed builds the shared scaffold every /standings, /scores and /overview
// reply starts from: the league name on the author line (when there is one), the
// given title and colour bar, an optional footer line for gameweek context, and
// the snapshot build time as the embed Timestamp so each client renders its own
// localised "last updated". Callers fill in Description or Fields.
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
