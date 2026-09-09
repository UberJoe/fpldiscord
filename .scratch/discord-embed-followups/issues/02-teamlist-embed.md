# 02 — `/teamlist` as an embed

**What to build:** `/teamlist` replies with a single framed embed instead of a
bold `**X's squad**` line plus a loose four-line code block. The squad is laid
out as four non-inline embed fields — one per position, `GK` → `DEF` → `MID` →
`FWD` — so each line of the squad reads as its own labelled block and nothing
wraps or side-scrolls on mobile.

Each field's value is the players in that position, in bootstrap-static order,
each formatted by the existing `playerLabel` (web name + club short name), joined
by `", "`. A position with no players shows a single placeholder so the full
squad shape (all four rows) is always visible.

The embed carries the league name on the author line, the title
`"{Manager}'s squad"` using the resolved display name, a neutral colour bar (a
squad has no live / settled axis), a footer with the gameweek context, and the
snapshot build time as the `Timestamp`. Manager matching is unchanged — first
name, first-seen manager wins a shared first name. The startup-not-ready reply
and the "no manager called X, or they own no players" reply stay short plain
text.

ADR 0002 gets a `/teamlist` amendment paragraph in its Consequences section,
matching the existing ticket-02 / ticket-04 amendment style: `/teamlist` renders
one non-inline field per position rather than a code-block table.

**Blocked by:** None — can start immediately.

**Status:** done

- [x] `/teamlist` for a resolved manager replies with exactly one embed carrying
      four non-inline fields in `GK`, `DEF`, `MID`, `FWD` order.
- [x] Each field value lists that position's players via `playerLabel`, in
      bootstrap-static order, comma-joined; an empty position shows the `"—"`
      placeholder.
- [x] The embed uses the neutral colour, shows the league name on the author
      line (when present), is titled `"{Manager}'s squad"` with the resolved
      display name, and has a footer with the gameweek context.
- [x] The embed's `Timestamp` equals the snapshot's build time.
- [x] First-name matching and the first-seen-manager-wins tiebreak for a shared
      first name are unchanged.
- [x] The startup-not-ready and "no manager called X" replies are still plain
      text.
- [x] The `/teamlist` handler tests assert on embed structure (field count and
      order, per-position player presence, colour, footer, `Timestamp`) rather
      than on formatted text; the tiebreak and empty-position cases are retained,
      retargeted at the embed.
- [x] ADR 0002 has a `/teamlist` amendment paragraph.
- [x] `go test ./...` is green.

## Comments

**Implemented** — `renderTeamlist` now returns `*discordgo.MessageEmbed` instead
of `string`, built from `dataEmbed(leagueName, builtAt, "{owner}'s squad",
colorNeutral, "GW{n}")` with four non-inline fields appended in `teamlistGroups`
order; the empty-position placeholder moved to a `teamlistPlaceholder` const
(`"—"`). `handleTeamlist` sends it via `RespondEmbeds`; the two short replies are
untouched `Respond` strings. No chunker — a squad is ≤ 15 players.

The shared `ownerTeamlistSnap` fixture gained `BuiltAt` / `LeagueName` /
`CurrentGW` (additive; `/owner` and autocomplete tests unaffected).
`teamlist_test.go` retargeted at embed structure via a new `teamlistEmbed`
helper (mirrors `scoresEmbed`): field count/order/non-inline, per-position
player presence, the `"—"` placeholder for Ian's empty MID/FWD, colour, author,
footer, `Timestamp`, the shared-first-name tiebreak, and plain-text startup /
unknown-owner paths. ADR 0002 Consequences gains a `/teamlist` amendment
paragraph. `go test ./...` green; `gofmt` clean.
