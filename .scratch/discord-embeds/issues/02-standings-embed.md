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

**Status:** done

- [x] `/standings` in classic mode replies with one embed whose description is a
      fenced code block containing the existing table, rows in the same order as
      today. (`renderStandings` → `dataEmbed` + `codeBlock(standingsTable(rows))`)
- [x] The embed uses the neutral colour, shows the league name (when present) on
      the author line, and has a footer with the gameweek context.
      (`colorNeutral`; footer `"GW{n} · total points league"`)
- [x] The embed's `Timestamp` equals the snapshot's build time. (via `dataEmbed`,
      `Snapshot.BuiltAt` formatted RFC 3339)
- [x] Table lines fit the mobile client without wrapping or horizontal scroll;
      the team-name column is capped and numerics are right-aligned.
      (`standingsNameCap = 14` runes + ellipsis; rank/Tot/GW right-aligned, Team
      left-aligned; test asserts every line ≤ 34 runes. Cap value is provisional
      — from the research 12–14 estimate, not yet eyeballed on a real mobile
      build; see comment below.)
- [x] The description contains no movement-arrow glyphs. (no arrow column;
      pinned by `TestHandleStandings_DescriptionHasNoMovementArrowGlyphs`)
- [x] The head-to-head-mode and startup-not-ready replies are still plain text.
- [x] The standings handler tests assert on the embed structure (colour, footer,
      `Timestamp`, row presence and order) rather than on formatted text.
- [x] `go test ./...` is green.

## Comments

**Implementation deviation — hand-rolled table body instead of `text/tabwriter`.**
The ticket and ADR 0002 say the table "moves unchanged". The plain-text
`renderStandings` used `text/tabwriter`; the embed version re-rolls the column
layout by hand (`standingsTable` + `colWidth` / `capRunes`). Two reasons
`tabwriter` can't do the job the ticket now asks for:

- `tabwriter` measures column width in bytes, so a capped multi-byte team name
  (e.g. `Ødegaard…`) throws the alignment off.
- The ticket wants the numeric columns right-aligned while the team column stays
  left-aligned; `tabwriter`'s `AlignRight` is all-or-nothing per writer.

Recorded as an amendment on ADR 0002. `/scores` and `/overview` (tickets 03/04)
keep `tabwriter` unless they hit the same constraint; if they don't, `colWidth`
and `capRunes` are candidates to hoist next to `codeBlock` in `embed.go`.

**Team-name cap not yet verified on a real mobile client.** `standingsNameCap`
is `14`, taken from the research note's 12–14 estimate. The "check real wrap
width on the current mobile build" step in the ticket body has not been done —
no device available in this environment. Realistic lines land near 29 chars
(well inside the 30–34 target), and the width test guards ≤ 34, so this is a
follow-up refinement, not a blocker. Revisit the constant after eyeballing the
live client.