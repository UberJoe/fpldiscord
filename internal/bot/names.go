package bot

import (
	"fmt"
	"strings"

	"github.com/UberJoe/fpldiscord/internal/fpl"
)

// fold is the shared name-match normalisation for the name-oriented commands
// (/owner, /teamlist) and the autocomplete resolver: accent-stripped, trimmed,
// and lower-cased, matching how the snapshot's PlayerNames / OwnerNames slices
// are compared.
func fold(s string) string { return strings.ToLower(fpl.StripAccents(strings.TrimSpace(s))) }

// playerLabel is the display form both /owner and /teamlist use for a player:
// the web name, suffixed with the club short name when one is known.
func playerLabel(p fpl.TeamPlayer) string {
	if p.TeamShort == "" {
		return p.WebName
	}
	return fmt.Sprintf("%s (%s)", p.WebName, p.TeamShort)
}
