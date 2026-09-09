# 03 — `/waivers` as embeds

**What to build:** `/waivers` replies with framed embeds instead of a bold
header line plus hand-paginated code-block messages. The command renders two
shapes, keyed on the `result` option:

- **`result: accepted`** (the default) is a flat list, so it stays a fenced
  code-block table in the embed `description` — but the columns get real
  alignment they never had: the owner (left-aligned, capped to a
  mobile-friendly rune width with an ellipsis), the roster move (`Out → In`, or
  just `In` when nothing was dropped), and the claim-kind tag (`(free agent)` /
  blank) each in their own column. Rows stay in `Index` order. One embed, no
  pagination — but guard the body against the 4096-character `description` limit:
  if it would overflow, keep as many whole rows as fit under ~4000 characters and
  append a final `"…and N more claims"` line.
- **`result: failed` and `result: all`** are contested-player groups, so they
  become one non-inline field per contested incoming player (groups in
  first-seen `ElementIn` order; `failed` still drops groups with no failed
  claim). The field name is the incoming player's name; the field value is the
  bid chain, one line per claim in `Priority` order (winner first), each line the
  existing `waiverBidLine` shape. These are packed by the shared
  `embedFieldChunker` and still spill across multiple messages for a long round,
  with no truncation.

Common to both: title `"GW{n} waivers"`, league name on the author line, a
neutral colour bar (a processed round is settled and historical — green is
reserved for "the gameweek has finished"), a footer naming the resolved `result`
mode, and the snapshot build time as the `Timestamp`. Every short reply stays
plain text: startup-not-ready, "No waiver rounds have been processed yet", the
`result` validation error, "Couldn't find any waivers for GW{n}", and
"No {result} waiver claims in GW{n}".

Hoist the shared rune-width helpers (`capRunes` / `colWidth`, currently in the
`/standings` command file) next to `codeBlock` in the shared embed helper so the
accepted-view table can reuse them — the parent feature's ticket-02 amendment
already flagged this move.

The row-resolution logic in `internal/fpl` (`LeagueTransactions` and the
`WaiverType` / `WaiverStatus` resolution) is untouched. ADR 0002 gets a
`/waivers` amendment paragraph: two shapes keyed on `result`, aligned
code-block table for `accepted`, fields for `failed` / `all`, and the column
alignment the plain-text version lacked.

**Blocked by:** 01 — Extract shared `embedFieldChunker`.

**Status:** ready-for-agent

- [ ] `/waivers` with no `gw` still shows the latest processed round; `gw`
      selects a round; the `result` validation error is unchanged.
- [ ] `result: accepted` replies with one embed whose description is a fenced
      block containing an aligned table — owner (capped, left), the move, and the
      kind tag as distinct columns — rows in `Index` order.
- [ ] An accepted list that would exceed the description limit keeps whole rows
      under ~4000 characters and ends with a `"…and N more claims"` line.
- [ ] `result: failed` / `all` reply with one non-inline field per contested
      player, groups in first-seen order, winner first within each group,
      `failed` dropping groups with no failed claim.
- [ ] A `failed` / `all` round with more than 25 contested groups splits into a
      second embed; past ten embeds' worth it splits into a second message (a
      second responder call) — no truncation.
- [ ] Both shapes use the neutral colour, the title `"GW{n} waivers"`, the
      league name on the author line, a footer naming the `result` mode, and a
      `Timestamp` equal to the snapshot build time.
- [ ] All the short / empty / error replies stay plain text.
- [ ] `capRunes` / `colWidth` live with the shared embed helpers and both
      `/standings` and `/waivers` use them.
- [ ] `/waivers` handler tests assert on embed structure; the `result`-filter,
      group-order and winner-first cases are retained, retargeted at the embed;
      the accepted-table alignment and the overflow line are covered on the pure
      table-body helper.
- [ ] ADR 0002 has a `/waivers` amendment paragraph.
- [ ] `go test ./...` is green.
