# 06 — `scores` command + `/api/manager/{entryId}` + web manager drill-down

**Spec:** [../to-spec.md](../to-spec.md)

**What to build:** A league member can see any manager's live gameweek squad with
auto-subs already applied — in Discord via `/scores [gw]` (every manager's provisional
total) and on the web by tapping a Standings row to reach a shareable `/manager/:id`
route showing XI + bench, points per player, gameweek total, and sub in/out markers.

**Blocked by:** 03, 05

**Status:** done

- [x] `/scores [gw]` lists every manager's live/provisional GW points with `subs[]`
      auto-subs applied; `total_points` + `bonus` as-is; `gw` optional with a sensible
      default
- [ ] `/scores` totals reconcile with the Draft website once the GW is final
      (parity-checklist row — real live data)
- [x] `GET /api/manager/{entryId}` is standalone; returns `entryId`, `ownerName`,
      `gw`, `provisional`, `total`, and `players[]` with `elementId`, `webName`,
      `teamShort`, `pos`, `squadSlot`, `points`, `minutes`, `inScoringXI`,
      `autoSubbedIn`, `autoSubbedOut`
- [x] Only `EntryID` crosses the wire; `LeagueEntryID` is resolved server-side; an
      unknown id returns 404 `{ "error": ... }`
- [x] The `/manager/:id` route has its own history entry (back button works, URL
      shareable), groups players XI-then-bench each ordered by `pos` then `squadSlot`,
      and polls its endpoint at live cadence while mounted
- [x] No fixtures, goalscorer lists, or bonus breakdown on the drill-down
- [x] seam-3 + seam-4 coverage for the endpoint and the command renderer

## Notes

- `fpl.ManagerSquad(id, gw)` is the new derived view: the full 15-man squad in
  squad-slot order, each pick tagged `InScoringXI` / `AutoSubbedIn` /
  `AutoSubbedOut` after `ApplyAutoSubs`, plus `Total` and `Provisional`
  (`!GWFinished`). `ManagerScore` is now a thin projection of it (scoring XI +
  total), so the two can't drift. Both stay pure functions of the exported
  `Snapshot` fields — element/team/name lookups are built locally from
  `Bootstrap.Elements` / `Bootstrap.Teams`, not the unexported indexes, so
  seam-3 tests drive a hand-built `*fpl.Snapshot`.
- `web.handleManager` resolves the membership row by scanning
  `LeagueDetails.LeagueEntries` (same seam-safety reason). ETag
  `manager-<entryId>-<builtAtUnix>`; 404 body `{ "error": ... }` with no
  envelope, for a non-numeric id, an id absent from `league_entries`, or a known
  id the snapshot can't score yet.
- The bot gained a minimal `cmdOptions` map on `cmdInput` (populated from the
  interaction's options in `onInteraction`); `/scores` reads the optional `gw`
  integer (1–38). A non-current `gw` gets a one-line explanation — the MVP
  snapshot only carries the current GW.
- `internal/web/dist/` stays the ticket-01 placeholder in git; `Manager.tsx` +
  `api.ts` (`fetchManager`, `NotFoundError`) + the drill-down CSS are the real
  view, picked up by `vite build`.
