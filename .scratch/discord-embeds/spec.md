# Spec — Discord data replies as embeds

**Status:** ready-for-agent

**Feature slug:** `discord-embeds`
**Research:** [research/01-embed-presentation.md](research/01-embed-presentation.md)

## Problem Statement

The bot's data commands — `/standings`, `/scores`, `/overview` — reply with a bold
markdown header line followed by a `text/tabwriter` table wrapped in a triple
backtick code block. In the Discord client this looks poor: the header and the
block don't feel like one unit, there is no visual framing, no colour, no "last
updated" cue, and on mobile the long code-block lines wrap or scroll sideways.
The user's words: "they don't look great, spacing is not good and visually it's
not nice."

## Solution

Every rich-data reply becomes a Discord **embed**. The aligned table stays — it
moves, unchanged, into the embed's `description` as a fenced code block, which is
the only Discord primitive that renders fixed-width on every client. The embed
adds the framing the plain text lacks: a coloured left bar that encodes state
(neutral / provisional / final), an author line for the league name, a title for
the table's subject, and a footer carrying the gameweek context plus a
client-localised "last updated" timestamp from the snapshot build time.

`/standings`, `/scores` and `/overview` are done in this spec. The other
message-heavy commands (`/teamlist`, `/waivers`, `/bet`, `/owner`) get the same
treatment in fast-follow tickets once the pattern and the widened output seam are
in place.

## User Stories

1. As a league member, I want `/standings` to come back as a framed embed rather
   than loose text, so that the table reads as one deliberate unit.
2. As a league member, I want the standings table itself to keep its column
   alignment, so that ranks, teams and totals still line up.
3. As a league member reading `/standings` on my phone, I want the table lines
   short enough not to wrap or side-scroll, so that I can read the whole row at a
   glance.
4. As a league member, I want the league name shown on the `/standings` embed, so
   that a screenshot is self-identifying.
5. As a league member, I want the `/standings` embed to show which gameweek the
   numbers are through, so that I know how current the table is.
6. As a league member, I want a "last updated" time on the `/standings` embed, so
   that I can tell whether it reflects the latest snapshot.
7. As a league member, I want `/standings` to keep showing the Draft official
   table order with no movement arrows, so that the Discord table stays a plain
   mirror of the official standings (arrows remain a web-only feature).
8. As a league member, I want `/scores` to come back as a framed embed, so that
   the live gameweek points table looks intentional.
9. As a league member, I want the `/scores` embed's colour to tell me at a glance
   whether the gameweek is still in progress or finished, so that I know if the
   numbers can still move.
10. As a league member, I want the provisional / final wording on `/scores` moved
    into the footer, so that the header isn't cluttered and the state cue is
    consistent with the colour bar.
11. As a league member, I want managers who can't be scored yet to still appear
    in the `/scores` table marked with a dash, so that the league roster stays
    complete.
12. As a league member, I want `/scores` rows ordered by live gameweek points,
    highest first, so that the leader is on top — unchanged from today.
13. As a league member, I want `/overview` fixtures rendered as embed fields — one
    field per fixture, score line as the field name, goalscorer lines as the
    value — so that each match reads as its own block with clear separation.
14. As a league member, I want the `/overview` embed's colour to reflect whether
    the shown fixtures are pre-kickoff, live, or all finished, so that I get the
    state at a glance.
15. As a league member, I want a fixture with no goals to still show in
    `/overview` with a "no goals" note, so that the fixture list is complete.
16. As a league member, I want `/overview` to keep working when a gameweek has
    many fixtures (including a double gameweek), so that a big week doesn't break
    the reply.
17. As a league member, I want `/overview` to keep splitting across more than one
    message when it has to, so that a huge week still comes through in full.
18. As a league member on desktop, I want the embed colour bar to be my primary
    state cue; as a league member on mobile, I want the footer text to carry the
    same information, so that neither platform loses the signal.
19. As a league member, I want the "not available yet" / "still starting up" /
    bad-option / empty-window replies to stay short plain text, so that trivial
    responses aren't over-decorated.
20. As a league member, I want `/standings` to keep telling me plainly when the
    league is in head-to-head mode, so that that path is unchanged.
21. As a developer, I want a single output seam that can carry either plain text
    or embeds, so that command handlers stay pure functions of the snapshot and
    options and remain testable without a Discord gateway.
22. As a developer, I want the embed-sending path to reuse the existing
    first-response / deferred-edit / follow-up branching, so that deferred
    interactions and multi-message replies keep working.
23. As a developer, I want the render helpers to return embed values rather than
    strings, so that tests can assert on structured fields (colour, footer, field
    list) instead of scraping formatted text.
24. As a developer, I want the pure table-body formatting kept as a
    string-returning helper, so that column layout stays cheap to unit-test and
    the code-fence wrapping lives in exactly one place.
25. As a developer, I want shared colour constants and one embed-scaffold helper,
    so that the three commands stay visually consistent and a colour change is a
    one-line edit.
26. As a developer, I want the embed limits (description 4096, field value 1024,
    25 fields per embed, 10 embeds per message, 6000 chars per message) respected
    in code, so that a large league or a double gameweek never produces a reply
    Discord rejects.
27. As a developer, I want an ADR recording the "rich data replies use embeds"
    convention and the colour semantics, so that the follow-up tickets and future
    commands follow the same pattern.
28. As a maintainer, I want the existing handler tests updated to assert on the
    new embed output, so that the suite keeps describing real user-visible
    behaviour.
29. As a maintainer, I want new tests for the state-colour logic and the
    `/overview` chunking boundaries, so that the parts most likely to regress are
    pinned.
30. As a league member, I want the change to be presentation-only — same rows,
    same numbers, same ordering, same gameweek semantics — so that nothing about
    what the bot reports changes, only how it looks.

## Implementation Decisions

### Output seam

- **Widen the existing `Responder` interface** (`internal/bot`, today
  `Respond(content string) error`). Add a second method that carries embeds —
  one call sends one message that may carry up to 10 embeds. This keeps the test
  seam exactly where it is today ("seam 4": handlers driven through a fake
  responder), just able to carry embeds as well as strings.
- The embeds method takes discordgo embed values directly. `internal/bot`
  already imports `discordgo`, so a discordgo-free `Message` value type is not
  worth the churn across every handler.
- The real implementation (`interactionResponder`) implements the new method
  through the **same three-way branching** the string path already has: first
  call is the interaction response (or, when a deferred ACK was sent, an edit of
  that placeholder); every later call in the same handler is a follow-up message
  on the interaction. Note the discordgo shape differences: the immediate and
  follow-up paths take `[]*discordgo.MessageEmbed`, the deferred-edit path takes
  `*[]*discordgo.MessageEmbed` (pointer to slice).
- A handler may still call the responder more than once (`/overview` when it
  needs more than one message). Each call maps to one Discord message.
- Plain-text replies (startup-not-ready, h2h-mode, bad option value, empty
  window, "couldn't find") stay on the string method — no embed.

### Render helpers

- `renderStandings` and `renderScores` change from returning `string` to
  returning one `*discordgo.MessageEmbed`.
- `renderOverview` changes from returning `[]string` to returning
  `[]*discordgo.MessageEmbed` (the per-message chunks).
- The pure table body — the current `tabwriter` loop — is split into its own
  helper that still returns a plain `string` (no code fences). The embed builder
  wraps that in a fenced block in one place.
- A shared scaffold helper stamps the common parts (author = league name, footer
  text, `Timestamp` from `Snapshot.BuiltAt` formatted RFC 3339, colour) so the
  three commands don't drift.

### Colour semantics (shared constants)

- Neutral / informational — `/standings`, `/overview` before kickoff.
- Live / provisional — `/scores` while the gameweek is in progress, `/overview`
  when any shown fixture is live.
- Final / settled — `/scores` once the gameweek is finished, `/overview` when
  every shown fixture is finished.
- Exact hex values are an implementation detail; the research note proposes the
  standard Discord brand green / amber / blurple. Define them once as named
  constants.

### `/standings` embed

- One embed. Author = league name (when present). Title = "Standings". Colour =
  neutral. Description = fenced code block containing the existing rank / team /
  official total / live-gameweek-points table. Footer = gameweek context (e.g.
  "GW{n} · total points league"). `Timestamp` = snapshot build time.
- Team-name column capped so lines stay within a mobile-friendly width; numeric
  columns right-aligned. The exact cap is set after checking real wrap width on
  the current client (research open question).
- **No movement-arrow column.** Discord `/standings` passes the Draft rank
  straight through; arrows are a web-only indicator (see
  `docs/adr/0001-standings-movement-arrow-week-over-week.md`).
- No pagination: a classic league fits well under the 4096-char description
  limit.

### `/scores` embed

- One embed. Author = league name. Title = "GW{n} scores". Colour encodes state:
  provisional while `GWFinished` is false, final once true. Description = fenced
  code block with the team / live-gameweek-points table, highest first, unscored
  managers shown as a dash. Footer = phase text ("provisional — scores can still
  move" / "final"). `Timestamp` = snapshot build time.
- The `(provisional)` / `(final)` tag is removed from the header line; the colour
  bar and footer carry it.

### `/overview` embed(s)

- One non-inline field per fixture: field name = the fixture header line (teams,
  score, status tag), field value = the goalscorer lines joined by newlines, or a
  "no goals" note. Existing scorer-line formatting (with its emoji) is reused
  verbatim.
- Title = "GW{n} {today's fixtures | gameweek fixtures | live fixtures}" per the
  existing mode. Footer = league name. `Timestamp` = snapshot build time. Colour
  = neutral before kickoff, provisional if any shown fixture is live, final if
  all shown fixtures are finished.
- Chunk fixtures into embeds of at most 25 fields; at most 10 embeds per message;
  start a new message before the combined field text approaches the 6000-char
  message ceiling. This replaces the current raw 2000-char text pagination with
  limit-aware chunking, but keeps the "spill to another message" behaviour.
- Guard each field value against the 1024-char limit and truncate with an
  ellipsis if a single fixture's scorer list somehow exceeds it.
- The one-embed-per-fixture alternative is rejected: it hits the 10-embed wall at
  ten fixtures, and a double gameweek has roughly twice that.

### ADR

- Add `docs/adr/0002-*.md` recording: rich-data Discord replies are embeds with a
  code-block table body; colour encodes provisional / final / neutral; the footer
  carries gameweek context and a snapshot-time "last updated". Mirrors how the
  arrow semantic change got ADR 0001.

### Not changed

- Command names, options, autocomplete, dispatch wiring.
- What the bot reports: same rows, same numbers, same ordering, same
  provisional / final and gameweek semantics.
- The web view and `/api/*` payloads.
- `internal/fpl` — no snapshot or scoring change; `BuiltAt` already exists.

## Testing Decisions

- **What a good test asserts here:** given a constructed snapshot, the handler
  emits the right *kind* of reply (embed vs plain string), with the right
  structured fields — colour matching the gameweek state, footer text present,
  `Timestamp` set, the expected number of fields for `/overview` — and a
  description / field body that contains the expected managers or fixtures in the
  expected order. Tests must **not** assert on exact whitespace, padding widths
  or the tabwriter's internal column math; that is presentation detail. Assert on
  row presence, relative order, and key substrings.
- **Modules under test:** `internal/bot` — `handleStandings`, `handleScores`,
  `handleOverview` — exercised through the fake responder at the widened seam.
- **Prior art:** `internal/bot/standings_test.go`, `scores_test.go`,
  `overview_test.go` already drive each handler through `recordingResponder`
  (`internal/bot/dave_test.go`) and assert on emitted output. The fake gains a
  field that records embed payloads alongside the string messages; the existing
  string-scraping assertions are rewritten against the embed structure.
- **Existing cases to keep, retargeted at the embed:** `/scores` ordering
  highest-first and the unscored-manager dash; `/standings` rank order and the
  h2h-mode plain-text path and the startup-nil plain-text path; `/overview`
  empty-window plain-text replies and the "no goals" fixture note.
- **New cases:**
  - `/scores` colour is the provisional value while `GWFinished` is false and the
    final value once true; footer phrasing follows.
  - `/overview` produces one field per fixture; at 26 fixtures it splits into two
    embeds; past ten embeds' worth it splits into a second message (second
    responder call).
  - `/standings` description contains no arrow glyphs.
  - `Timestamp` on each embed equals the snapshot's `BuiltAt` (RFC 3339).
- **`interactionResponder` stays structurally untested** — there is no gateway in
  the unit suite today, and that is unchanged. The new method's branching is
  covered only by inspection; acceptable, consistent with the existing
  string-path treatment.
- `go test ./...` green. No TypeScript or web changes, so `npm run typecheck` is
  unaffected (not part of this ticket's gate).

## Out of Scope

- Re-skinning `/teamlist`, `/waivers`, `/bet` (`show` and archived), `/owner`,
  and the `/bet set` / `/bet archive` confirmations. Same pattern, separate
  fast-follow tickets once the seam lands.
- Turning any error / startup / bad-option / empty-result reply into an embed —
  those stay short plain text.
- `ansi` colour code blocks (desktop/web only, monochrome on mobile) — deferred;
  the embed colour bar covers state signalling.
- Per-league or configurable embed colours.
- Movement-arrow column in Discord `/standings` — explicitly excluded; arrows are
  web-only per ADR 0001 and the `standings-rank-arrows` spec.
- Markdown pipe tables — Discord does not render them in message body or embed
  description on any client (research TL;DR); not an option.
- Medal / coloured-circle emoji embellishment of rows — optional polish a
  reviewer may fold in, not required by this spec.
- Any change to the web Standings view or `/api/*`.

## Further Notes

- Full source review in [research/01-embed-presentation.md](research/01-embed-presentation.md),
  with every limit and rendering claim cited to the Discord developer docs or
  `discordgo` source.
- discordgo specifics that shape the code: no embed builder (plain
  `&discordgo.MessageEmbed{}` structs); `Color` is a plain `int` (use a hex
  literal); `Timestamp` is an ISO-8601 `string` with no helper — format
  `Snapshot.BuiltAt` with `time.RFC3339`; `WebhookEdit.Embeds` is
  `*[]*discordgo.MessageEmbed`.
- Limits to enforce in code: description 4096, field value 1024, 25 fields per
  embed, 10 embeds per message, 6000 characters total across a message's embeds.
- Mobile is the constraint on table width: a code block renders fixed-width but a
  phone shows roughly 30–34 characters before wrapping; keep table lines within
  that. Outside a code block Discord uses a proportional font, which is what makes
  the current output look ragged.
- Before setting the `/standings` team-name cap, eyeball the real wrap width on
  the current Discord mobile build (research open question).
- Confirm nothing outside the handlers consumes the render helpers' current
  string return before changing their signatures — a grep shows only the handlers
  call them today.
