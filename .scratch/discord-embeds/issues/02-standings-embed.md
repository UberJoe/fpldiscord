# 02 — `/standings` as an embed

**What to build:** `/standings` replies with a single framed embed instead of a
bold header line plus a loose code block. The rank / official-total /
live-gameweek-points table keeps its column alignment — it moves unchanged into
the embed's `description` as a fenced code block. The embed adds a neutral colour
bar, the league name on the author line, and a footer carrying the gameweek
context and a client-localised "last updated" time from the snapshot build.

The team-name column is capped to a width that does not wrap or side-scroll on
the Discord mobile client; numeric columns stay right-aligned. Set the cap after
checking real wrap width on the current mobile build.

No movement-arrow column — Discord `/standings` passes the Draft rank straight
through; arrows are a web-only indicator (see
`docs/adr/0001-standings-movement-arrow-week-over-week.md`).

The head-to-head-mode reply and the "still starting up" reply stay short plain
text.

**Blocked by:** 01 — Embed output seam, shared scaffold, and ADR.

**Status:** ready-for-agent

- [ ] `/standings` in classic mode replies with one embed whose description is a
      fenced code block containing the existing table, rows in the same order as
      today.
- [ ] The embed uses the neutral colour, shows the league name (when present) on
      the author line, and has a footer with the gameweek context.
- [ ] The embed's `Timestamp` equals the snapshot's build time.
- [ ] Table lines fit the mobile client without wrapping or horizontal scroll;
      the team-name column is capped and numerics are right-aligned.
- [ ] The description contains no movement-arrow glyphs.
- [ ] The head-to-head-mode and startup-not-ready replies are still plain text.
- [ ] The standings handler tests assert on the embed structure (colour, footer,
      `Timestamp`, row presence and order) rather than on formatted text.
- [ ] `go test ./...` is green.
