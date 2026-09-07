package bot

import (
	"testing"
	"time"
)

func TestDataEmbed_StampsAuthorTitleColourFooterAndTimestamp(t *testing.T) {
	built := time.Date(2026, 9, 7, 14, 30, 0, 0, time.UTC)

	e := dataEmbed("Coq au Ian", built, "Standings", colorNeutral, "GW5 · total points league")

	if e.Author == nil || e.Author.Name != "Coq au Ian" {
		t.Errorf("author = %+v, want league name on the author line", e.Author)
	}
	if e.Title != "Standings" {
		t.Errorf("title = %q, want %q", e.Title, "Standings")
	}
	if e.Color != colorNeutral {
		t.Errorf("color = %#x, want %#x", e.Color, colorNeutral)
	}
	if e.Footer == nil || e.Footer.Text != "GW5 · total points league" {
		t.Errorf("footer = %+v, want the gameweek context text", e.Footer)
	}
	if want := built.Format(time.RFC3339); e.Timestamp != want {
		t.Errorf("timestamp = %q, want RFC 3339 of BuiltAt %q", e.Timestamp, want)
	}
}

func TestDataEmbed_OmitsAuthorWithoutLeagueNameAndFooterWhenEmpty(t *testing.T) {
	e := dataEmbed("", time.Now(), "GW5 scores", colorProvisional, "")

	if e.Author != nil {
		t.Errorf("author = %+v, want nil when the snapshot has no league name", e.Author)
	}
	if e.Footer != nil {
		t.Errorf("footer = %+v, want nil when the footer text is empty", e.Footer)
	}
}

func TestDataEmbed_OmitsTimestampWhenBuiltAtIsZero(t *testing.T) {
	e := dataEmbed("Coq au Ian", time.Time{}, "Standings", colorNeutral, "")
	if e.Timestamp != "" {
		t.Errorf("timestamp = %q, want empty for a zero BuiltAt", e.Timestamp)
	}
}

func TestEmbedStateColours_AreDistinct(t *testing.T) {
	if colorNeutral == colorProvisional || colorProvisional == colorFinal || colorNeutral == colorFinal {
		t.Fatalf("state colours must differ: neutral=%#x provisional=%#x final=%#x",
			colorNeutral, colorProvisional, colorFinal)
	}
}
