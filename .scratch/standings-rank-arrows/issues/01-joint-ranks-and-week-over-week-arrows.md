# 01 — Standings ranks and movement arrows handle ties and track week-over-week

**What to build:** The web Standings live table ranks managers correctly when
they are level on points, and its movement arrow shows where each manager sits
versus **last week's final table** rather than versus the current gameweek's
official scoring.

Two coupled defects, fixed together because a correct arrow needs correct ranking
on both ends of its comparison:

1. **Joint ranks.** The Draft API reports standard competition ranking — tied
   managers share a rank and the next rank skips (`1, 2, 2, 4`) — and also
   provides a strict tiebroken ordering. The live table currently assigns itself
   a dense `1..N` rank, so a genuine tie on live points renders as two different
   ranks, and row order within a tie depends on the order the API happened to
   return rows in. After this change the live table shares a rank between managers
   level on live points (matching the official Draft table and the Discord
   `/standings` list, which already render ties correctly), and breaks ties for
   row order deterministically using the Draft strict-sort field.

2. **Arrow basis.** The arrow currently shows within-gameweek movement: the gap
   between a manager's official Draft position (which already includes this
   gameweek's official event total) and where live scoring — live provisional
   bonus, auto-subs before the gameweek finalises — puts them. It collapses toward
   flat once a gameweek finishes and official scoring catches up. After this
   change the arrow shows movement since last week's final standings: the Draft
   `last_rank` on each standings row compared to the manager's current live rank.
   A manager with no previous position — gameweek 1, or an entry that joined
   mid-season — shows a flat arrow, not a spurious climb.

Scope is the web Standings live table and the `/api/standings` payload that feeds
it. Discord `/standings` is out of scope — it passes the Draft rank straight
through and already renders ties correctly. The manager drill-down is out of
scope.

Also in this ticket:

- A *movement arrow* glossary entry in `CONTEXT.md`, using the
  frozen / live / official vocabulary already defined there.
- An ADR recording the arrow's semantic change (within-gameweek → week-over-week),
  since the arrow is a user-visible indicator whose meaning changes. Draft
  ADR started at [docs/adr/0001-standings-movement-arrow-week-over-week.md](../../../docs/adr/0001-standings-movement-arrow-week-over-week.md).
- The two existing live-standings Go tests assert the old `official − live` arrow
  arithmetic and need reworking; add coverage for a live-points tie and for the
  `last_rank == 0` (no previous position) case.

**Blocked by:** None — can start immediately.

**Status:** implemented

- [x] Managers level on live points share a live rank (`1, 2, 2, 4`), matching the
      official Draft table; the rank column no longer shows a dense `1..N`
      sequence through a tie.
- [x] Row order within a tie is deterministic, using the Draft strict-sort field
      rather than API response order.
- [x] The movement arrow shows places moved since last week's final standings
      (`last_rank` → current live rank), not within-gameweek official-vs-live
      movement.
- [x] A manager with no previous position (`last_rank == 0`: gameweek 1,
      mid-season entrant) shows a flat arrow.
- [x] Two managers tied on live points, with nothing moved since last week, both
      show a flat arrow — no phantom `▲` / `▼`.
- [x] `/api/standings` carries `last_rank`; the web `Arrow` component's shape and
      up / down labels are unchanged, only their meaning.
- [x] Discord `/standings` and the manager drill-down are unchanged.
- [x] `CONTEXT.md` has a *movement arrow* glossary entry.
- [x] An ADR records the within-gameweek → week-over-week arrow change.
- [x] Go and API tests updated: the existing live-standings arrow assertions
      reworked, plus new cases for a live-points tie and for `last_rank == 0`.
- [x] `npm run typecheck` passes.

## Comments

**Implementation (2026-09-07):** `LiveStandings` in `internal/fpl/standings.go`
now sorts by live points with a `rank_sort` tie-break, assigns a joint
`LiveRank`, and computes `Arrow` as `LastRank - LiveRank` (flat when
`LastRank == 0`). `OfficialRank` stays on the row and the `/api/standings`
payload but no longer feeds the arrow. `standingsRow` / `api.ts` gained
`lastRank`. Reworked `TestLiveStandings_SortedByLivePointsWithRanksAndArrows`
and the API pre-sorted test; added `TestLiveStandings_JointRankOnLivePointsTie`
and `TestLiveStandings_NoPreviousPositionShowsFlatArrow`. Full Go suite +
`npm run typecheck` green.
