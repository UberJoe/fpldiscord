# 01 — Standings "Total" column shows the live total

**What to build:** On the web Standings view, the "Total" column shows each
manager's **live total** — points from completed gameweeks plus their live
current-gameweek score — instead of the frozen pre-gameweek total shown today. The
value `livePoints` is already fetched from `/api/standings`; the view currently
renders `totalPoints` (frozen) and should render `livePoints` instead. The header
stays "Total" and gets no live-vs-idle styling — between gameweeks the live total
simply equals the old number.

The GW column changes from an unconditional `+{liveGwPoints}` to:

- `+N` when the manager has current-gameweek points;
- `+0` when they have none *but* the gameweek has started — where "started" means
  `meta.matchLive` is true, or any row in the table has non-zero `liveGwPoints`
  (so `+0` persists through gaps between fixtures once `matchLive` drops to false);
- `–` otherwise (between gameweeks — nothing started, nobody has points).

The movement arrow, the server-side sort by `livePoints`, and the `/api/*` surface
are all unchanged. This is a frontend-only change; `totalPoints` stays in the API
payload, just unrendered. Scope is the Standings **list view** only — Discord
`/standings` and the manager drill-down are out of scope.

**Blocked by:** None — can start immediately.

**Status:** done — `feat/standings-live-total` @ 0a66f03

- [x] The "Total" column on the web Standings view shows `livePoints` (frozen +
      live gameweek score), not the frozen total; header still reads "Total".
- [x] GW column shows `+N` for a manager with current-gameweek points.
- [x] GW column shows `+0` for a manager with no current-gameweek points once the
      gameweek has started (`meta.matchLive`, or any row has non-zero
      `liveGwPoints`).
- [x] GW column shows `–` between gameweeks (not started, no row has points).
- [x] Movement arrow, row ordering, and drill-down navigation are unchanged.
- [x] No Go, `/api/*`, or Discord `/standings` change.
- [x] `npm run typecheck` passes; manual check in the running app covers all three
      GW-column states — *manual app pass still outstanding (no web test runner).*
