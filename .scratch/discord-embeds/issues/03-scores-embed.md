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

**Status:** ready-for-agent

- [ ] `/scores` replies with one embed whose description is a fenced code block
      containing the team / live-gameweek-points table, highest first, unscored
      managers as a dash — same rows and order as today.
- [ ] The embed colour is the provisional value while the gameweek is unfinished
      and the final value once it is finished.
- [ ] The provisional / final wording is in the footer, not the title; the league
      name is on the author line.
- [ ] The embed's `Timestamp` equals the snapshot's build time.
- [ ] The startup-not-ready, non-current-gameweek and empty-league replies are
      still plain text.
- [ ] The scores handler tests assert on the embed structure, including a case
      for the provisional colour/footer and a case for the final colour/footer.
- [ ] `go test ./...` is green.
