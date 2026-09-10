# 01 — Extract shared `embedFieldChunker`

**What to build:** The field-packing logic that `/overview` uses to lay a
variable list of embed fields across Discord messages — respecting 25 fields per
embed, 10 embeds per message, and a per-message character budget kept under the
6000-character ceiling, spilling to further messages when it has to — becomes a
single shared helper, `embedFieldChunker` (name indicative), so that `/waivers`
and `/bet` can reuse it instead of each copying it.

`/overview` is migrated onto the shared helper and its embed output is
byte-identical to today. This ticket changes no command's behaviour; it is the
prefactor that tickets 03 and 04 build on.

The helper stays configurable for the behaviours `/overview` already relies on:
the title stamped on the first embed of the whole reply only; one colour across
every embed; the league name carried in the footer *or* the author line at the
caller's choice. The clean shape is: the caller supplies the scaffold parameters
(league name, build time, title, colour, footer) plus a first-embed-only-title
flag, and the chunker builds each embed through the existing `dataEmbed`
scaffold. Field values are expected pre-truncated to the 1024-character limit by
the caller.

**Blocked by:** None — can start immediately.

**Status:** done

- [x] The chunker type lives with the shared embed helpers, not inside the
      `/overview` command file, and carries no `/overview`-specific naming.
- [x] It packs non-inline `(name, value)` fields into embeds at ≤ 25 fields per
      embed, ≤ 10 embeds per message, and a per-message character total kept a
      margin under 6000; it returns one `[]*discordgo.MessageEmbed` per Discord
      message.
- [x] Title-on-first-embed-only, a single colour across all embeds, and
      league-name-in-footer-vs-author are all caller-configurable.
- [x] `/overview` is migrated to the shared chunker and emits byte-identical
      embeds — same field count, order, titling, colour, footer and `Timestamp`
      as before.
- [x] `/overview`'s existing chunk-boundary tests (one field per fixture; split
      into a second embed past 25 fixtures; split into a second message past ten
      embeds' worth) pass unchanged.
- [x] The chunker has direct unit coverage for the 25-field, 10-embed and
      character-budget boundaries, independent of `/overview`.
- [x] No other command's reply changes in this ticket.
- [x] `go test ./...` is green.

## Comments

**Implemented** — `embedFieldChunker` + `embedFieldScaffold` + `newEmbedFieldChunker`
now live in `internal/bot/embed.go` alongside `dataEmbed` / `codeBlock`, with the
per-message budget renamed `overviewMessageCharBudget` → `embedMessageCharBudget`
(value unchanged, `maxMessageEmbedChars - 200`). `renderOverview` builds the
chunker from a scaffold (`footer: leagueName`, `titleFirstOnly: true`) — the
league name still rides the footer with no author line, so output is
byte-identical and the existing `overview_test.go` chunk-boundary tests pass
untouched. New direct coverage in `embed_test.go`: 25-field split, 10-embed
spill, character-budget split, `titleFirstOnly` on/off, footer-vs-author
carriage, colour bar on every embed. `packCodeBlockMessages` is left for
ticket 05. `go test ./...` green.
