package bot

import (
	"fmt"
	"strings"

	"github.com/UberJoe/fpldiscord/internal/fpl"
)

// handleOwner names the league member who owns a player, or reports a free
// agent. Ownership comes from fpl.TeamPlayers, which reads live element-status,
// so the answer is correct the moment the GW20 mid-season redraft lands.
//
// The player arg is autocompleted from web_name, so an exact (accent- and
// case-insensitive) match is the norm; a web_name shared by two players lists
// every match.
func handleOwner(in *cmdInput) error {
	if in.snap == nil {
		return in.resp.Respond("Ownership isn't available yet — the bot is still starting up.")
	}
	name, _ := in.opts.String("player")
	name = strings.TrimSpace(name)
	if name == "" {
		return in.resp.Respond("Give me a player name.")
	}

	want := fold(name)
	var matches []fpl.TeamPlayer
	for _, p := range in.snap.TeamPlayers() {
		if fold(p.WebName) == want {
			matches = append(matches, p)
		}
	}
	if len(matches) == 0 {
		return in.resp.Respond(fmt.Sprintf("No player called %q.", name))
	}

	var b strings.Builder
	for i, p := range matches {
		if i > 0 {
			b.WriteByte('\n')
		}
		if p.OwnerName == "" {
			fmt.Fprintf(&b, "**%s** is a free agent.", playerLabel(p))
		} else {
			fmt.Fprintf(&b, "**%s** is owned by **%s** (%s).", playerLabel(p), p.OwnerName, p.OwnerTeam)
		}
	}
	return in.resp.Respond(b.String())
}
