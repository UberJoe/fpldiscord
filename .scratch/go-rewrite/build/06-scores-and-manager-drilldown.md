# 06 — `scores` command + `/api/manager/{entryId}` + web manager drill-down

**Spec:** [../to-spec.md](../to-spec.md)

**What to build:** A league member can see any manager's live gameweek squad with
auto-subs already applied — in Discord via `/scores [gw]` (every manager's provisional
total) and on the web by tapping a Standings row to reach a shareable `/manager/:id`
route showing XI + bench, points per player, gameweek total, and sub in/out markers.

**Blocked by:** 03, 05

**Status:** ready-for-agent

- [ ] `/scores [gw]` lists every manager's live/provisional GW points with `subs[]`
      auto-subs applied; `total_points` + `bonus` as-is; `gw` optional with a sensible
      default
- [ ] `/scores` totals reconcile with the Draft website once the GW is final
- [ ] `GET /api/manager/{entryId}` is standalone; returns `entryId`, `ownerName`,
      `gw`, `provisional`, `total`, and `players[]` with `elementId`, `webName`,
      `teamShort`, `pos`, `squadSlot`, `points`, `minutes`, `inScoringXI`,
      `autoSubbedIn`, `autoSubbedOut`
- [ ] Only `EntryID` crosses the wire; `LeagueEntryID` is resolved server-side; an
      unknown id returns 404 `{ "error": ... }`
- [ ] The `/manager/:id` route has its own history entry (back button works, URL
      shareable), groups players XI-then-bench each ordered by `pos` then `squadSlot`,
      and polls its endpoint at live cadence while mounted
- [ ] No fixtures, goalscorer lists, or bonus breakdown on the drill-down
- [ ] seam-3 + seam-4 coverage for the endpoint and the command renderer
