package fpl

import (
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

// accentStripper decomposes accented runes (NFD) and drops the combining marks,
// then recomposes. This handles the Latin-accent case (é -> e, ü -> u) but not
// letters with no canonical decomposition (ø, ł, æ, …) — those go through
// extraFolds first.
var accentStripper = transform.Chain(
	norm.NFD,
	runes.Remove(runes.In(unicode.Mn)),
	norm.NFC,
)

// extraFolds covers the letters NFD leaves intact — the ones that actually turn
// up in Premier League player names (Nordic ø/å-family, Polish ł, ligatures).
var extraFolds = strings.NewReplacer(
	"Ø", "O", "ø", "o",
	"Ł", "L", "ł", "l",
	"Æ", "AE", "æ", "ae",
	"Œ", "OE", "œ", "oe",
	"Ð", "D", "ð", "d",
	"Þ", "Th", "þ", "th",
	"ß", "ss",
	"İ", "I", "ı", "i",
	"Đ", "D", "đ", "d",
)

// StripAccents removes diacritics from s (é→e, ø→o, ł→l, …) while preserving
// case. It is the shared name-normalisation primitive: the snapshot builds the
// autocomplete candidate slices with it, and the bot folds a user's partial
// argument through it at match time so a name typed in plain ASCII resolves
// against its accented form.
func StripAccents(s string) string { return stripAccents(s) }

// stripAccents returns s with diacritics removed, matching the intent of the
// Python bot's unidecode() call so that autocomplete matches "Hojlund" against
// "Højlund". Case is preserved; callers lowercase at match time.
func stripAccents(s string) string {
	s = extraFolds.Replace(s)
	out, _, err := transform.String(accentStripper, s)
	if err != nil {
		return s
	}
	return out
}
