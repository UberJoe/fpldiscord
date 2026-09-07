# Standings — live total points

## Problem

The web Standings view's "Total" column shows each manager's **frozen total**
(points from completed gameweeks only). During a gameweek it looks stale — it
doesn't move as players score. The number the league wants to see mid-match is the
**live total**: frozen total plus the live current-gameweek score.

The live total already exists. `LiveStandings()` computes `LivePoints = frozen +
live gameweek points`, the table is already server-sorted by it, and
`/api/standings` already ships `livePoints` on every row. The web view fetches it
and never renders it — it paints `totalPoints` (frozen) instead.

## Agreed design

Scope: **web Standings list view only** (`web/src/views/Standings.tsx`). Discord
`/standings` is unaffected — it uses a different code path and doesn't have this
problem. The manager drill-down is untouched.

Frontend-only. `livePoints` is already on the wire; `totalPoints` stays in the
payload, just unrendered. No Go, API, or sort/arrow change.

### Column change

- The "Total" column renders `row.livePoints` instead of `row.totalPoints`.
- Header stays "Total".
- No distinct styling whether or not a match is live — always just the live total.
  Between gameweeks it silently equals the old frozen number, which is correct.

### GW column — `+N` / `+0` / `–`

The GW column currently always shows `+{liveGwPoints}`. New rule so a genuine
mid-match `+0` reads differently from a dead week:

```
gwStarted = meta.matchLive || rows.some(r => r.liveGwPoints !== 0)
cell      = liveGwPoints !== 0 ? `+${liveGwPoints}` : (gwStarted ? "+0" : "–")
```

`meta.matchLive` catches the just-kicked-off / nobody-scored window; "any row has
points" carries `+0` through the rest of the gameweek once `matchLive` drops back
to false between fixtures. Both false → dead week → `–`. The movement arrow is
unchanged.

## Verification

No automated test — the `web/` project has no test runner and the repo has no CI;
the frontend is typecheck-only by design, with the scoring logic tested in Go.
Acceptance is `npm run typecheck` plus a manual pass in the running app across the
three GW-column states.

## Vocabulary

New glossary terms in [CONTEXT.md](../../CONTEXT.md): *live total*, *frozen total*,
*live gameweek points*, *official total (Draft)*.

## Not doing

- No ADR — the column choice is a low-stakes, reversible display tweak.
- No web test infrastructure — its own decision if wanted, not riding in on this.
- No change to Discord `/standings`, the manager drill-down, or the `/api/*`
  surface.
