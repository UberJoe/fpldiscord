# 04 — `/bet` leaderboard as embeds

**What to build:** `/bet` / `/bet show` (the live leaderboard) and
`/bet show season:<past>` (an archived season) reply with framed embeds instead
of a bold header line plus hand-paginated code-block messages. The leaderboard is
one "record with detail" per bettor, so each bettor becomes one non-inline embed
field:

- Field name = `"{bettor}  ·  {total}  {marker}"`, where `bettor` is resolved
  via the existing member-namer (raw id as fallback), `total` is the running
  goal total, and `marker` is the existing status glyph (`in` / `🕓` / `💥`) plus
  the leader tag — `(leading)` while the season runs, `🏆` once
  `bet.SeasonComplete` is true.
- Field value = the four picks, each `"{web name} ({goals})"` (existing pick-cell
  formatting, raw `#id` fallback), joined so it stays within the 1024-character
  field limit (four picks always do).

Board order is unchanged — closest-to-21, over-21 totals last — in both the live
and archived views. Fields are packed by the shared `embedFieldChunker`.

- **Live view:** title `"Bet leaderboard — {season}"`, league name on the author
  line, colour = provisional while `!bet.SeasonComplete`, final once complete,
  footer `"{season} · provisional — goals can still move"` / `"{season} · final"`
  mirroring `/scores`, `Timestamp` = snapshot build time.
- **Archived view:** title `"Bet — {season} (archived)"`, colour = final always,
  footer `"{season} · archived"`, no `Timestamp` (no snapshot bears on a frozen
  record).

`/bet` stays in the deferred-ACK set: the first chunk edits the "thinking…"
placeholder and later chunks are follow-up messages — the embed responder path
already supports both. The "not configured" / "still starting up" / "no bets
entered yet" / "no archived record for X" / "no seasons archived yet" replies
stay plain text. `/bet set` and `/bet archive` are entirely untouched —
`internal/bet` and `internal/store` are not changed.

ADR 0002 gets a `/bet` amendment paragraph: one non-inline field per bettor
rather than a code-block table, and the provisional / final colour semantics
extended to a season-long axis via `bet.SeasonComplete`.

**Blocked by:** 01 — Extract shared `embedFieldChunker`.

**Status:** done

- [x] `/bet` / `/bet show` replies with embeds carrying one non-inline field per
      bettor, board order unchanged (closest-to-21, over-21 last).
- [x] The field name carries the bettor, total, status glyph and the leader tag
      — `(leading)` pre-completion, `🏆` post-completion; the field value carries
      the four `"{web name} ({goals})"` picks.
- [x] Live-view colour is the provisional value while `bet.SeasonComplete` is
      false and the final value once true; the footer names the season and the
      matching phase; `Timestamp` equals the snapshot build time.
- [x] `/bet show season:<past>` renders the same field-per-bettor shape from the
      archive, sorted the same way, bust totals keeping the `💥` marker; colour
      is final, footer is `"{season} · archived"`, and there is no `Timestamp`.
- [x] A board with more than 25 bettors splits across embeds at the chunker
      boundary.
- [x] The deferred-ACK flow still works — first chunk edits the placeholder,
      later chunks are follow-ups.
- [x] The not-configured / still-starting-up / no-bets / no-archive replies stay
      plain text; `/bet set` and `/bet archive` are unchanged.
- [x] `/bet` handler tests assert on embed structure; the closest-to-21 order,
      the leader / status markers, and the empty / no-archive plain-text paths
      are retained, retargeted at the embed; the state-colour cases are new.
- [x] ADR 0002 has a `/bet` amendment paragraph.
- [x] `go test ./...` is green.

## Comments

**Implemented** — `betShow` / `betShowArchived` now build embeds through the
shared `embedFieldChunker`. `betBoardBlocks` / `archivedBlocks` (returning
`[]string`) are replaced by `betBoardFields` / `archivedFields` (returning
`[]betField`, a name/value pair) plus `renderBetBoard` / `renderBetArchived`,
which feed the chunker from an `embedFieldScaffold` (`leagueName` on the author
line, `titleFirstOnly: true`). Live view: `colorProvisional` +
`"{season} · provisional — goals can still move"` while `!bet.SeasonComplete`,
`colorFinal` + `"{season} · final"` once complete, `Timestamp` = `Snapshot.BuiltAt`.
Archived view: `colorFinal` always, `"{season} · archived"` footer, zero `builtAt`
so `dataEmbed` omits the `Timestamp`. `/bet` stays in `deferredCommands`;
`RespondEmbeds` already routes first-message-edits-placeholder / rest-are-follow-ups.

`bet_test.go` retargeted at embed structure via new `betEmbeds` / `betOneEmbed` /
`betText` helpers (mirroring `waivers_test.go`); `betBotSnap` gained a `BuiltAt`.
New cases: live colour/footer provisional while the season runs; final + trophy
once complete; archived is `colorFinal` with no `Timestamp`; archived keeps
closest-to-21 with bust last and the `💥` marker; a 26-bettor board splits 25 + 1
across two embeds in one message; a long board spills across messages.

**Ticket 05 folded in** — with `/waivers` (03) and `/bet` (04) both migrated,
`packCodeBlockMessages` and `maxDiscordMessage` had no callers and were deleted
from `waivers.go` (ticket 05 permits folding the deletion into the last ticket to
touch the shared file). `go build`, `go vet`, `go test ./...` green.
