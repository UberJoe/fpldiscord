# 2. Rich-data Discord replies are embeds

Date: 2026-09-07

## Status

Accepted

## Context

The bot's data commands — `/standings`, `/scores`, `/overview` — reply with a
bold markdown header line followed by a `text/tabwriter` table wrapped in a
triple-backtick code block. In the Discord client the header and the block do
not read as one unit: there is no visual framing, no colour, no "last updated"
cue, and on mobile the long code-block lines wrap or scroll sideways. The
league's words: "they don't look great, spacing is not good and visually it's
not nice."

Discord's only reliable fixed-width primitive is a fenced code block, and it
renders no colour or emphasis inside. Everything else Discord offers for
framing — a coloured left bar, an author line, a title, a footer, a
client-localised timestamp — lives on the **embed** object. Markdown pipe tables
render nowhere in Discord on any client, so they are not an option (see
`.scratch/discord-embeds/research/01-embed-presentation.md`).

## Decision

Rich-data replies use a Discord **embed**. The aligned `tabwriter` table stays,
moved unchanged into the embed `description` as a fenced code block —
`/overview`, whose records are visually distinct fixtures, uses one non-inline
`field` per fixture instead. The embed adds the framing the plain text lacked:

- **Colour bar encodes state**, via three shared named constants:
  - *neutral* (blurple) — informational, no "can this still move" signal:
    `/standings`, `/overview` before kickoff.
  - *provisional* (amber) — live / still-moving: `/scores` while the gameweek is
    in progress, `/overview` when any shown fixture is live.
  - *final* (green) — settled: `/scores` once the gameweek is finished,
    `/overview` when every shown fixture is finished.
- **Author line** carries the league name (when the snapshot has one), so a
  screenshot is self-identifying.
- **Title** names the table's subject ("Standings", "GW5 scores").
- **Footer** carries the gameweek context (e.g. "GW5 · total points league",
  or the `/scores` provisional / final phrase) — the state wording moves off the
  header line and into the footer, consistent with the colour bar.
- **Timestamp** is set from `Snapshot.BuiltAt` formatted RFC 3339, so each
  client shows its own localised "last updated".

On desktop the colour bar is the primary state cue; on mobile the footer text
carries the same information, so neither platform loses the signal.

The output seam is the existing `Responder` interface, widened with a second
method that carries embeds. Command handlers stay pure functions of the snapshot
and options: render helpers return `*discordgo.MessageEmbed` values (or a slice,
for `/overview` chunks) rather than strings, so tests assert on structured
fields. The real responder sends embeds through the same first-response /
deferred-edit / follow-up branching the string path already uses.

Short plain-text replies — startup-not-ready, h2h-mode, bad option value, empty
window, "couldn't find" — stay on the string method. Trivial responses are not
decorated.

Discord's embed limits are respected in code: description 4096, field value
1024, 25 fields per embed, 10 embeds per message, 6000 characters total across a
message's embeds.

This ADR mirrors ADR 0001's treatment of a user-visible indicator change: the
convention is recorded here so the fast-follow tickets (`/teamlist`, `/waivers`,
`/bet`, `/owner`) and future commands follow the same pattern.

## Consequences

- A colour or palette change is a one-line edit to the shared constants; the
  three commands cannot drift because they share one scaffold helper.
- Handler tests assert on embed structure (colour, footer, field count,
  `Timestamp`) instead of scraping formatted text. The `tabwriter` column math
  stays a cheap string-returning helper, unit-tested without a Discord gateway.
- `interactionResponder`'s new embed branching has no gateway in the unit suite,
  so it is covered by inspection only — consistent with the existing string
  path.
- The change is presentation-only: same rows, same numbers, same ordering, same
  gameweek and provisional / final semantics. The web view and `/api/*` payloads
  are untouched.
- Discord `/standings` still passes the Draft rank straight through with no
  movement-arrow column; arrows remain web-only per ADR 0001.
- Amendment (ticket 02): "moved unchanged" holds for the plain-text `tabwriter`
  bodies in principle, but `/standings` re-rolls its table body by hand
  (`standingsTable`) instead of reusing `tabwriter`. `tabwriter` measures column
  width in bytes, so a capped multi-byte team name misaligns, and it cannot
  right-align the numeric columns while left-aligning the name — both of which
  the ticket requires. `/scores` and `/overview` keep `tabwriter` unless they
  hit the same constraint.
- Amendment (ticket 04): `/overview` carries no table body at all — it renders
  one non-inline embed `field` per fixture (score line as the name, goalscorer
  lines as the value), so no `tabwriter` and no code fence. `renderOverview`
  returns one `[]*discordgo.MessageEmbed` per Discord message; an
  `overviewChunker` packs fields at 25 per embed, 10 embeds per message, and a
  per-message character budget kept a little under the 6000-char ceiling, keeping
  the "spill to another message" behaviour a big or double gameweek needs. The field name is plain text (Discord renders no
  markdown there); the field value reuses the scorer-line formatting verbatim.
  The league name moves to the footer (not the author line) for `/overview`, and
  only the first embed carries the title.
- Amendment (`/teamlist`): like `/overview`, `/teamlist` carries no table body —
  it renders one non-inline `field` per position in fixed `GK` → `DEF` → `MID` →
  `FWD` order, the position label as the field name and that position's players
  (`playerLabel`, bootstrap-static order, comma-joined) as the value, so a squad
  reads as four labelled blocks that never wrap or side-scroll on mobile. A
  position with no players shows a `"—"` placeholder so the full four-row shape
  is always visible. The colour bar is `colorNeutral` — a squad has no
  live / settled axis — with the league name on the author line, the title
  `"{Manager}'s squad"`, a `"GW{n}"` footer and the snapshot-time `Timestamp`.
  `renderTeamlist` returns a single `*discordgo.MessageEmbed`: one squad is at
  most 15 players across four fields, comfortably inside every limit, so there is
  no chunking. The startup-not-ready and "no manager called X" replies stay
  plain text.
- Amendment (`/waivers`): `/waivers` renders **two shapes keyed on the `result`
  option**. `accepted` (the default) is a flat homogeneous list, so it stays a
  fenced code-block table in one embed's `description` — but the columns now get
  real alignment the loose `fmt.Sprintf` version lacked: the owner (left, capped
  at `waiverOwnerCap`, mirroring `standingsNameCap`), the roster move and the
  claim-kind tag each in their own column, rows in `Index` order. The table body
  is a pure string helper (`waiverAcceptedTable`) wrapped by `codeBlock`, the
  same split as `standingsTable`; it is guarded against the 4096-character
  description limit by keeping whole rows under ~4000 characters and appending a
  `"…and N more claims"` line rather than chunking (a pathological free-agent
  week only). `failed` / `all` are contested-player groups, so each contested
  incoming player becomes one non-inline `field` — name = the incoming player,
  value = the bid chain in `Priority` order (winner first, `waiverBidLine`
  shape) — packed by the shared `embedFieldChunker` (now used by `/overview` and
  `/waivers`), which spills a long round across further messages with no
  truncation; `failed` drops a group with no failed claim. Both shapes use
  `colorNeutral` (a processed round is settled history; `colorFinal` green is
  reserved for "the gameweek has finished"), the title `"GW{n} waivers"`, the
  league name on the author line, a footer naming the resolved `result` mode and
  the snapshot-time `Timestamp`. The row-resolution logic in `internal/fpl`
  (`LeagueTransactions`) is untouched. The startup-not-ready, "no waiver rounds
  processed", `result`-validation, "couldn't find any waivers" and "no {result}
  claims" replies stay plain text.
