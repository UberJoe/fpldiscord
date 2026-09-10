# Spec — Discord embeds for `/teamlist`, `/waivers` and `/bet`

**Status:** ready-for-agent

**Feature slug:** `discord-embed-followups`
**Parent:** [`../discord-embeds/spec.md`](../discord-embeds/spec.md), `docs/adr/0002-rich-data-discord-replies-use-embeds.md`

## Problem Statement

The `discord-embeds` feature reskinned `/standings`, `/scores` and `/overview` as
framed Discord embeds — a coloured state bar, the league name on the author line,
a title, a footer with gameweek context, and a client-localised "last updated"
timestamp — and explicitly parked `/teamlist`, `/waivers` and `/bet` as
"same pattern, separate fast-follow tickets once the seam lands".

Those three still reply the old way: a bold markdown header line followed by a
`text/tabwriter` (or hand-joined) body wrapped in a triple-backtick code block,
sent through the plain-string `Respond`. In the client they read as loose text,
carry no state colour, no "last updated" cue, and on mobile the code-block lines
wrap or side-scroll. `/waivers` and `/bet` additionally hand-roll their own
2000-character message pagination (`packCodeBlockMessages`) instead of chunking
against Discord's real embed limits.

## Solution

`/teamlist`, `/waivers` and `/bet` (the `show` and archived leaderboard views)
become Discord embeds, built on the seam and scaffold the parent feature already
landed: the widened `Responder`, the `dataEmbed` scaffold helper, the `codeBlock`
wrapper, and the shared `colorNeutral` / `colorProvisional` / `colorFinal`
constants.

Each command takes the embed primitive that fits its data shape:

- **`/teamlist`** — a fixed four-row squad. One embed, one non-inline **field per
  position** (`GK` / `DEF` / `MID` / `FWD`), the player list as the field value.
  No code-block table; nothing to chunk.
- **`/waivers`** — renders **two shapes keyed on the `result` option**.
  `accepted` (the default) is a flat homogeneous list, so it stays a
  **code-block table** in the embed `description` — but the columns get real
  alignment (owner / move / kind), which the current loose `fmt.Sprintf` never
  had. `failed` / `all` are contested-player groups with bid chains, so they
  become **one non-inline field per contested player**, the bid chain in the
  value.
- **`/bet`** — the leaderboard is one "record with detail" per bettor. **One
  non-inline field per bettor**: name = bettor, total, and the leader / status
  marker; value = the four `Name (goals)` picks. The archived view uses the
  identical shape.

The `overviewChunker` — which already packs a variable field list into embeds
against the 25-field / 10-embed / ~6000-character limits and spills to more
messages — is generalised to a shared `embedFieldChunker` and reused by
`/waivers` (`failed` / `all`) and `/bet`. `packCodeBlockMessages`, whose only
callers are `/waivers` and `/bet`, is removed.

The change is presentation-only: same rows, same numbers, same ordering, same
`result` filter, same closest-to-21 math, same provisional / final semantics.
Every non-data reply — startup-not-ready, "no manager called X", "no waiver
rounds processed yet", bad `result` value, empty result, and the `/bet set` /
`/bet archive` write acknowledgements — stays short plain text.

## User Stories

1. As a league member, I want `/teamlist` to come back as a framed embed rather
   than a bold line plus a loose code block, so that a manager's squad reads as
   one deliberate unit.
2. As a league member, I want each position in `/teamlist` (`GK`, `DEF`, `MID`,
   `FWD`) shown as its own labelled block, so that I can scan the squad shape at
   a glance.
3. As a league member reading `/teamlist` on my phone, I want the squad laid out
   as embed fields rather than a monospace block, so that nothing wraps or
   side-scrolls.
4. As a league member, I want a `/teamlist` position with no players to still
   show with a placeholder, so that the full squad shape (all four rows) is
   always visible.
5. As a league member, I want the league name shown on the `/teamlist` embed, so
   that a screenshot is self-identifying.
6. As a league member, I want the `/teamlist` embed titled with the manager's
   name, so that it is clear whose squad it is.
7. As a league member, I want a gameweek context and a "last updated" time on the
   `/teamlist` embed, so that I know how current the roster is.
8. As a league member, I want `/teamlist` to keep matching the manager on first
   name and keep the first-seen manager winning a shared first name, so that the
   command behaves exactly as it does today.
9. As a league member, I want `/waivers` with no arguments to still show the
   latest processed round's accepted claims, now as a framed embed, so that the
   default view looks intentional.
10. As a league member, I want the accepted-claims view rendered as an aligned
    table — owner, the roster move, the claim kind in their own columns — so that
    the claims line up instead of running together.
11. As a league member, I want the accepted-claims table wrapped in one embed
    with a state colour bar, a title, and a footer, so that it matches
    `/standings` and `/scores`.
12. As a league member, I want `/waivers result:failed` and `/waivers result:all`
    to show each contested player as its own block with the full bid list in
    priority order, so that I can see who out-bid whom without hunting through a
    wall of text.
13. As a league member, I want each contested-player block to keep the winner
    first and the out-bid markers, so that the bid outcome is unchanged from
    today.
14. As a league member, I want `/waivers` to keep splitting a long round across
    more than one message when it has to, so that a busy week still comes through
    in full with no truncation.
15. As a league member, I want a `/waivers` round with an unusually long accepted
    list to still fit in the single accepted-view embed, with a clear
    "…and N more" line if it ever overflows, so that the reply never silently
    drops claims.
16. As a league member, I want the `/waivers` embed colour to be neutral, so that
    a settled historical round is not dressed up as something still live.
17. As a league member, I want the `/waivers` embed footer to say which claim set
    I am looking at (`accepted` / `failed` / `all`), so that the view is
    self-describing once the header line is gone.
18. As a league member, I want `/bet` (and `/bet show`) to return the leaderboard
    as a framed embed, so that the season-long bet standings look intentional.
19. As a league member, I want each bettor shown as their own block — name,
    running total, and the leader / provisionally-out / bust marker as the block
    heading; the four picks with their goal counts as the block body — so that a
    bettor's position is legible at a glance on any device.
20. As a league member, I want the `/bet` embed colour to tell me whether the
    season is still running or finished, so that I know if the totals can still
    move.
21. As a league member, I want the `/bet` footer to name the season and echo the
    provisional / final phase, so that the state cue is consistent with the
    colour bar and survives on mobile.
22. As a league member, I want the leader marked `(leading)` while the season
    runs and `🏆` only once it is complete — unchanged from today — carried in the
    bettor's block heading.
23. As a league member, I want `/bet show season:<past>` to render the archived
    season in the same block-per-bettor shape as the live board, so that a past
    season reads the same way.
24. As a league member, I want the archived `/bet` view coloured as final and
    footered as archived, so that it is clearly a frozen record.
25. As a league member, I want `/bet` to keep its closest-to-21 ordering
    (over-21 totals last) in both the live and archived views, so that the
    ranking is unchanged.
26. As a league member on desktop, I want the embed colour bar as my primary
    state cue; as a league member on mobile, I want the footer text to carry the
    same information, so that neither platform loses the signal.
27. As a league member, I want the "still starting up" / "not configured" /
    "no manager called X" / "no waiver rounds processed yet" / bad-`result` /
    empty-result replies to stay short plain text, so that trivial responses are
    not over-decorated.
28. As a league member, I want the `/bet set` and `/bet archive` confirmation and
    error replies to stay plain text, so that a write acknowledgement is not
    dressed up as a data view.
29. As a league member, I want `/owner` left exactly as it is (a one-line plain
    reply), so that a single-sentence answer is not wrapped in embed chrome.
30. As a developer, I want `/teamlist`, `/waivers` and `/bet` to go through the
    existing widened `Responder` seam, so that the handlers stay pure functions
    of the snapshot / store and options and remain testable without a Discord
    gateway.
31. As a developer, I want the render helpers to return `*discordgo.MessageEmbed`
    values (or a per-message slice for the chunked replies), so that tests assert
    on structured fields instead of scraping formatted text.
32. As a developer, I want the aligned accepted-claims table body kept as a pure
    string-returning helper, so that the column math is cheap to unit-test and
    the code-fence wrapping stays in one place.
33. As a developer, I want the `overviewChunker` generalised into one shared
    `embedFieldChunker`, so that the 25-field / 10-embed / ~6000-character
    packing logic exists once and cannot drift between `/overview`, `/waivers`
    and `/bet`.
34. As a developer, I want `/overview`'s embed output unchanged after the chunker
    extraction, so that its existing chunk-boundary tests are the regression net
    for the move.
35. As a developer, I want `packCodeBlockMessages` removed once its callers are
    migrated, so that the codebase carries one field-chunking mechanism, not a
    second string-message paginator.
36. As a developer, I want the embed limits (description 4096, field value 1024,
    25 fields per embed, 10 embeds per message, 6000 characters across a
    message's embeds) respected in code, so that a busy waiver round or a large
    bet field never produces a reply Discord rejects.
37. As a developer, I want the `/bet` deferred-ACK path to keep working with
    embeds — first chunk edits the placeholder, later chunks are follow-ups — so
    that the member-name lookups still cannot miss Discord's 3-second window.
38. As a developer, I want ADR 0002 amended with one paragraph per command
    recording how each lands the embeds convention, so that the deviations
    (fields vs table, two shapes for `/waivers`, the shared chunker, the
    season-long provisional/final axis) are discoverable.
39. As a maintainer, I want the existing `/teamlist`, `/waivers` and `/bet`
    handler tests retargeted at the embed structure, so that the suite keeps
    describing real user-visible behaviour.
40. As a maintainer, I want new tests for the `/bet` state colour and the
    `embedFieldChunker` boundary under `/waivers` and `/bet`, so that the parts
    most likely to regress are pinned.
41. As a league member, I want the change to be presentation-only — same rows,
    same numbers, same ordering, same `result` filter and closest-to-21 and
    provisional / final semantics — so that nothing about what the bot reports
    changes, only how it looks.

## Implementation Decisions

### Scope

- **In:** `/teamlist`, `/waivers`, `/bet` (`show` — no subcommand — and the
  archived `season:<past>` view).
- **Out:** `/owner` (a one-line answer — stays plain text per ADR 0002's "trivial
  responses are not decorated"); `/bet set` and `/bet archive` confirmation and
  error replies (write acknowledgements, not rich data).
- Presentation-only. No change to command names, options, autocomplete, dispatch
  wiring, the `result` filter, the closest-to-21 math, `internal/fpl`,
  `internal/bet`, `internal/store`, the web views, or `/api/*`.

### Shared seam and scaffold (unchanged from the parent feature)

- The widened `Responder` (`Respond(string)` + `RespondEmbeds([]*discordgo.MessageEmbed)`).
  A handler may call the responder more than once; each call is one Discord
  message. The real `interactionResponder` already routes first-call vs
  deferred-edit vs follow-up for both methods.
- `dataEmbed(leagueName, builtAt, title, color, footer)` stamps author line,
  footer, and the RFC 3339 `Timestamp` from `Snapshot.BuiltAt` (omitted for a
  zero time).
- `codeBlock(body)` is the single fenced-block wrapper.
- Colour constants: `colorNeutral` (blurple), `colorProvisional` (amber),
  `colorFinal` (green).

### `embedFieldChunker` (generalised from `overviewChunker`)

- Move `overviewChunker` into the shared embed helper as `embedFieldChunker`
  (name indicative, not binding). It packs a stream of non-inline
  `(name, value)` fields into embeds and embeds into messages, honouring: at most
  25 fields per embed, at most 10 embeds per message, and a per-message
  character budget kept a little under the 6000-character ceiling. It returns one
  `[]*discordgo.MessageEmbed` per Discord message.
- It must stay configurable for the behaviours `/overview` already relies on:
  title stamped on the first embed of the reply only; a single colour across all
  embeds; league name carried in the footer *or* the author line depending on
  caller. The cleanest shape is: the caller supplies the scaffold parameters
  (`leagueName`, `builtAt`, `title`, `color`, `footer`) and a flag for
  first-embed-only titling; the chunker calls `dataEmbed` per embed.
- `/overview` is migrated to call the generalised chunker with its current
  parameters and must emit byte-identical embed output. Its existing
  chunk-boundary tests are the regression net.
- Field values are expected pre-truncated to the 1024-character limit by the
  caller (as `/overview` does today with `capRunes`).

### `/teamlist` embed

- One embed. Author = league name (when the snapshot has one). Title =
  `"{Manager}'s squad"` using the resolved display name (first-name match,
  first-seen manager wins a shared first name — unchanged). Colour =
  `colorNeutral` (a squad has no live / settled axis). Footer = `"GW{n}"` from
  `Snapshot.CurrentGW`. `Timestamp` = snapshot build time.
- Four non-inline fields in fixed order `GK` → `DEF` → `MID` → `FWD`. Field name
  = the position label. Field value = the players in that position, in
  bootstrap-static order, each formatted by the existing `playerLabel`
  (`WebName` + club short name), joined by `", "`. A position with no players
  shows a single placeholder (`"—"`).
- No chunking — a squad is at most 15 players across four fields, comfortably
  inside every limit.
- `renderTeamlist` changes from returning `string` to returning
  `*discordgo.MessageEmbed`.
- The startup-not-ready reply and the "no manager called X, or they own no
  players" reply stay `Respond` strings.

### `/waivers` embed

- **`result: accepted` (default)** → one embed, no pagination.
  - Description = a fenced code block containing an **aligned** table: `owner`
    (left, capped to a mobile-friendly rune width with an ellipsis, mirroring
    `standingsNameCap` / `capRunes`), the roster move (`Out → In`, or just `In`
    when nothing was dropped), and the claim-kind tag (`(free agent)` for a
    free-agent pickup, blank for a plain waiver) as their own columns. Rows stay
    in `Index` order. This replaces the current loose
    `fmt.Sprintf("%s  %s%s", …)` one-liner.
  - The table body is a pure string-returning helper (no fences), wrapped by
    `codeBlock` in the render helper — same split as `standingsTable`.
  - Guard the body against the 4096-character `description` limit: if it would
    overflow (a pathological free-agent week — not expected in practice), keep as
    many whole rows as fit under ~4000 characters and append a final
    `"…and N more claims"` line. No description chunker — the same call
    `/standings` made ("a classic league fits well under 4096").
- **`result: failed` and `result: all`** → the `embedFieldChunker`.
  - One non-inline field per contested incoming player, groups in first-seen
    `ElementIn` order. `failed` drops groups with no failed claim (unchanged).
  - Field name = the incoming player's name (`waiverName(g[0].In)`; plain text —
    Discord renders no markdown in a field name).
  - Field value = the bid chain, one line per claim in `Priority` order (winner
    first), each line the existing `waiverBidLine` shape
    (`{bid slot} {owner}  {move}  [{won|out-bid}{kind}]`).
  - Chunks spill to further messages exactly as `/overview` does now.
- Common to both: Title = `"GW{n} waivers"`. Author = league name. Colour =
  `colorNeutral` (a processed round is settled and historical; `colorFinal`
  green is reserved for "the gameweek has finished"). Footer = the resolved
  `result` mode. `Timestamp` = snapshot build time.
- `waiverBlocks` and the render layer change to return embed values / the
  per-message slices. The row-resolution logic in `internal/fpl`
  (`LeagueTransactions`) is untouched.
- All the short replies stay `Respond` strings: startup-not-ready, "No waiver
  rounds have been processed yet", the `result` validation error, "Couldn't find
  any waivers for GW{n}", "No {result} waiver claims in GW{n}".

### `/bet` embed (`show` + archived)

- **`/bet` / `/bet show`** (live board) → the `embedFieldChunker`.
  - One non-inline field per bettor, board order unchanged
    (`bet.Leaderboard`, closest-to-21, over-21 last).
  - Field name = `"{bettor}  ·  {total}  {marker}"` where `marker` is the
    existing `betStatusGlyph` output (`in` / `🕓` / `💥`) plus the leader tag —
    `(leading)` while the season runs, `🏆` once
    `bet.SeasonComplete(snap)` is true. Bettor name via the existing
    `betDisplayName` (member-namer with the raw id as fallback).
  - Field value = the four picks, each `"{WebName} ({goals})"` (existing
    `pickCells`, raw `#id` fallback), joined by `"  ·  "` or newlines
    (implementer's choice — must stay within 1024 characters, which four picks
    always do).
  - Title = `"Bet leaderboard — {season}"`. Author = league name.
  - Colour = `colorProvisional` while `!bet.SeasonComplete(snap)`,
    `colorFinal` once complete. Footer = `"{season} · provisional — goals can
    still move"` / `"{season} · final"`, mirroring `/scores`. `Timestamp` =
    snapshot build time.
- **`/bet show season:<past>`** (archived) → the same `embedFieldChunker` and
  the same field shape, fed from `betStore.Archive(season)` and sorted by the
  existing `archivedBlocks` ordering. Bust totals keep the `💥` marker. Title =
  `"Bet — {season} (archived)"`. Colour = `colorFinal` always. Footer =
  `"{season} · archived"`. No `Timestamp` (no snapshot bears on a frozen
  record).
- `/bet` stays in `deferredCommands`. `RespondEmbeds` already handles the
  deferred-edit path (pointer-to-slice) and follow-ups, so a multi-message board
  edits the placeholder with the first message and follows up with the rest.
- `betBoardBlocks` / `archivedBlocks` change from returning `[]string` to
  producing the chunker's field stream (or an intermediate the render helper
  turns into fields). `betShow` / `betShowArchived` call `RespondEmbeds` per
  message instead of `packCodeBlockMessages` + `Respond`.
- The "not configured" / "still starting up" / "no bets entered yet" /
  "no archived record for X" / "no seasons archived yet" replies stay `Respond`
  strings. `/bet set` and `/bet archive` are entirely untouched.

### Removals

- Delete `packCodeBlockMessages` — after `/waivers` and `/bet` migrate it has no
  callers.
- If `/overview` no longer needs any command-local chunker type after the
  extraction, remove the now-empty `overviewChunker` name (folded into
  `embedFieldChunker`).

### ADR

- Amend `docs/adr/0002-rich-data-discord-replies-use-embeds.md` — append to
  Consequences, one short paragraph per command, matching the existing
  ticket-02 / ticket-04 amendment style. Record: `/teamlist` and `/bet` use
  `fields` rather than a code-block table; `/waivers` renders two shapes keyed on
  `result` (aligned code-block table for `accepted`, fields for `failed` / `all`)
  and gains column alignment the plain-text version lacked; `overviewChunker`
  generalised to `embedFieldChunker` shared by three commands;
  `packCodeBlockMessages` retired; `/bet` extends the provisional / final colour
  semantics to a season-long axis via `bet.SeasonComplete`.
- No new ADR — the decision ("rich-data Discord replies are embeds") is
  unchanged.

### Not changed

- Command names, options, choices, autocomplete, dispatch and deferral wiring.
- What the bot reports: same rows, numbers, ordering, `result` filter,
  closest-to-21 math, leader / status markers, provisional / final and gameweek
  semantics.
- `internal/fpl`, `internal/bet`, `internal/store`, the web views, `/api/*`.
- `/owner`, `/dave`, `/standings`, `/scores`, `/overview` behaviour (`/overview`
  is refactored onto the shared chunker but its output is byte-identical).

## Testing Decisions

- **What a good test asserts here:** given a constructed `*fpl.Snapshot` (and a
  fake `BetStore` for `/bet`), the handler emits the right *kind* of reply
  (embed vs plain string) with the right structured fields — colour matching the
  state, title and footer text present, `Timestamp` set (or absent, for the
  archived `/bet` view), the expected number of fields, and a description / field
  body containing the expected managers, claims or bettors in the expected
  order. Tests must **not** assert on exact whitespace, padding widths or column
  math in the embed-structure tests — that is presentation detail. Assert on row
  / field presence, relative order, and key substrings. The one place exact
  layout is checked is the pure table-body helper for `/waivers accepted`, unit
  tested directly (as `standingsTable` is) including a line-width bound and the
  overflow "…and N more" behaviour.
- **Modules under test:** `internal/bot` — `handleTeamlist`, `handleWaivers`,
  `handleBet` (`betShow`, `betShowArchived`) — through the fake `Responder` at
  the existing seam. Plus the generalised `embedFieldChunker` exercised at the
  same seam via the three commands, and `/overview` re-run against its current
  chunk-boundary tests unchanged.
- **Prior art:** `internal/bot/standings_test.go`, `scores_test.go`,
  `overview_test.go` — each drives its handler through `recordingResponder`
  (`dave_test.go`), which already records `embeds [][]*discordgo.MessageEmbed`
  alongside string messages, and asserts on embed structure. `teamlist_test.go`,
  `waivers_test.go`, `bet_test.go` already exist against the string output and
  are retargeted.
- **Existing cases to keep, retargeted at the embed:**
  - `/teamlist`: the first-name match, the first-seen-manager-wins tiebreak for a
    shared first name, and the `"—"` placeholder for an empty position.
  - `/waivers`: `accepted` default when no `gw`; `gw` selects a round; the
    `result` validation error; `failed` drops groups with no failed claim;
    contested groups in first-seen order with the winner first.
  - `/bet`: closest-to-21 order with over-21 last in both live and archived
    views; the leader / provisionally-out / bust markers; the "no bets entered"
    and "no archived record" plain-text paths.
- **New cases:**
  - `/teamlist` emits one embed with four fields in `GK`/`DEF`/`MID`/`FWD` order;
    `colorNeutral`; `Timestamp == Snapshot.BuiltAt`.
  - `/waivers accepted` emits one embed, description is a fenced block, columns
    align (checked on the pure helper); an oversized accepted list keeps whole
    rows and ends with `"…and N more claims"`.
  - `/waivers failed` / `all` emits one field per contested player; a round with
    more than 25 contested groups splits into a second embed; past ten embeds'
    worth it splits into a second message (a second responder call).
  - `/bet` colour is `colorProvisional` while `bet.SeasonComplete` is false and
    `colorFinal` once true; footer phrasing follows. Archived view is
    `colorFinal` with no `Timestamp`.
  - `/bet` board of more than 25 bettors splits across embeds at the chunker
    boundary (retargeted from the `/overview` chunk test).
  - Each command's startup / not-configured / empty replies are plain strings,
    not embeds.
- **`interactionResponder` stays structurally untested** — no gateway in the unit
  suite, unchanged from the parent feature. The embed branching is covered by
  inspection.
- `go test ./...` green. No web or TypeScript changes.

## Out of Scope

- `/owner` — stays a one-line plain-text reply.
- `/bet set` and `/bet archive` — confirmation and error strings stay plain
  text; the handlers are untouched.
- Any error / startup / bad-option / empty-result reply becoming an embed.
- Any change to what the bot reports: rows, numbers, ordering, the `result`
  filter, the closest-to-21 math, the leader / status markers, or the
  provisional / final and gameweek semantics.
- `ansi` colour code blocks (desktop-only, monochrome on mobile) — the embed
  colour bar covers state signalling, consistent with the parent feature.
- Per-league or configurable embed colours.
- Markdown pipe tables — Discord renders them nowhere (parent research).
- Movement-arrow column anywhere in Discord — web-only per ADR 0001.
- Medal / coloured-circle emoji embellishment of rows — optional polish a
  reviewer may fold in, not required.
- Resolving the latent `/teamlist` ambiguity where two managers sharing a first
  name are de-duped to one autocomplete entry and one squad — unchanged
  behaviour, tracked separately if it matters.
- `internal/fpl`, `internal/bet`, `internal/store`, the web views, `/api/*`.

## Further Notes

- Build order: the `embedFieldChunker` extraction is a prerequisite for the
  `/waivers` and `/bet` tickets and is shared between them — whichever lands
  first carries the extraction and the `/overview` migration; the other depends
  on it. `/teamlist` depends on nothing and can land first or in parallel.
- discordgo specifics (from the parent research, still current): no embed
  builder — plain `&discordgo.MessageEmbed{}`; `Color` is a plain `int` (hex
  literal); `Timestamp` is an RFC 3339 `string` with no helper;
  `WebhookEdit.Embeds` is `*[]*discordgo.MessageEmbed` (pointer-to-slice) on the
  deferred-edit path, which the `/bet` flow exercises.
- Limits to respect in code: description 4096, field name 256, field value 1024,
  25 fields per embed, 10 embeds per message, 6000 characters total across a
  message's embeds.
- Mobile is the width constraint: a fenced code block renders fixed-width but a
  phone shows ~30–34 characters before wrapping. The `/waivers accepted` owner
  column cap is set with that budget in mind; `standingsNameCap` (14 runes) is
  the reference point and is itself flagged provisional pending a real-device
  check in the parent feature.
- Confirm nothing outside the handlers consumes `renderTeamlist`, `waiverBlocks`
  / the `/waivers` render layer, or `betBoardBlocks` / `archivedBlocks` before
  changing their signatures — a grep should show only the handlers and their
  tests.
- `packCodeBlockMessages` currently lives in `waivers.go` and is imported by
  `bet.go`; its removal is part of this feature, not a separate cleanup.
