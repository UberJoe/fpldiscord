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
