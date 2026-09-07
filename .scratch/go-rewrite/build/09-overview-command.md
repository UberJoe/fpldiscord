# 09 — `overview` command

**Spec:** [../to-spec.md](../to-spec.md)

**What to build:** `/overview` shows fixtures and goalscorers for the current window
in its three modes (Today's / Gameweek's / Live), with the new defensive-contribution
stat family kept out of the goalscorer display.

**Blocked by:** 02

**Status:** done

- [x] `Overview(gw)` derived view produces fixtures + goalscorers per mode
      (`internal/fpl/overview.go`: `FixtureOverview` with `Today` / `Started` /
      `FinishedProvisional` flags; the bot's `filterOverview` selects the window)
- [x] `defensive_contribution`, `clearances_blocks_interceptions`, `recoveries`,
      `tackles`, `bps`, and `bonus` are filtered out of the goalscorer display
      (`goalscorerStat` is an allowlist of goals / assists / own goals / red
      cards, so new stat families are excluded by default)
- [x] All three modes (Today's / Gameweek's / Live) render
      (`internal/bot/overview.go`; `mode` choice option, whole gameweek default)
- [x] The logic has no `utcnow()`-style wall-clock dependency — `Today` is
      derived from the snapshot's `BuiltAt` (as `ProcessedGWs` does); the handler
      filters on booleans only
- [x] seam-4 handler test against a fake snapshot (`internal/bot/overview_test.go`),
      plus seam-1 coverage of the derived view (`internal/fpl/overview_test.go`)
