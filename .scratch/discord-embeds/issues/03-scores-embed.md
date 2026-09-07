# 03 — `/scores` as an embed

**What to build:** `/scores` replies with a single framed embed. The team /
live-gameweek-points table — highest first, managers who can't be scored yet
shown as a dash — moves unchanged into the embed's `description` as a fenced code
block. The embed's colour tells the reader at a glance whether the gameweek is
still in progress (provisional) or finished (final), and the provisional / final
wording moves out of the header line into the footer alongside the "last updated"
time from the snapshot build. The league name goes on the author line.

State comes from whether the gameweek is finished — the same signal the current
`(provisional)` / `(final)` tag uses.

The "still starting up", non-current-gameweek, and empty-league replies stay
short plain text.

**Blocked by:** 01 — Embed output seam, shared scaffold, and ADR.

**Status:** done

- [x] `/scores` replies with one embed whose description is a fenced code block
      containing the team / live-gameweek-points table, highest first, unscored
      managers as a dash — same rows and order as today. (`renderScores` →
      `dataEmbed` + `codeBlock(scoresTable(rows))`; sort + dash logic unchanged
      in `handleScores`)
- [x] The embed colour is the provisional value while the gameweek is unfinished
      and the final value once it is finished. (`colorProvisional` / `colorFinal`
      keyed on `snap.GWFinished`)
- [x] The provisional / final wording is in the footer, not the title; the league
      name is on the author line. (title `"GW{n} scores"`; footer
      `"provisional — scores can still move"` / `"final"`; author via `dataEmbed`)
- [x] The embed's `Timestamp` equals the snapshot's build time. (via `dataEmbed`,
      `Snapshot.BuiltAt` formatted RFC 3339)
- [x] The startup-not-ready, non-current-gameweek and empty-league replies are
      still plain text. (each asserted with `len(r.embeds) == 0`)
- [x] The scores handler tests assert on the embed structure, including a case
      for the provisional colour/footer
      (`TestHandleScores_ProvisionalColourAndFooterWhileGameweekLive`) and a case
      for the final colour/footer
      (`TestHandleScores_FinalColourAndFooterOnceGameweekFinished`).
- [x] `go test ./...` is green.

## Comments

`scoresTable` keeps `text/tabwriter` (only two columns, both simple; no capped
multi-byte name and no mixed per-column alignment), unlike `/standings`'s
hand-rolled `standingsTable` — consistent with the ADR 0002 amendment from
ticket 02.
