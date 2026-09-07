# 09 — `overview` command

**Spec:** [../to-spec.md](../to-spec.md)

**What to build:** `/overview` shows fixtures and goalscorers for the current window
in its three modes (Today's / Gameweek's / Live), with the new defensive-contribution
stat family kept out of the goalscorer display.

**Blocked by:** 02

**Status:** ready-for-agent

- [ ] `Overview(gw)` derived view produces fixtures + goalscorers per mode
- [ ] `defensive_contribution`, `clearances_blocks_interceptions`, `recoveries`,
      `tackles`, `bps`, and `bonus` are filtered out of the goalscorer display
- [ ] All three modes (Today's / Gameweek's / Live) render
- [ ] The logic has no `utcnow()`-style wall-clock dependency
- [ ] seam-4 handler test against a fake snapshot
