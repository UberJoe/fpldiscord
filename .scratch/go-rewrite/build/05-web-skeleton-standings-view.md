# 05 — Web skeleton + `/api/*` envelope + Standings web view (seam 3)

**Spec:** [../to-spec.md](../to-spec.md)

**What to build:** The deployed URL serves a phone-first React SPA (embedded in the
binary, no Node at runtime) that lands on a live-ordered league table and refreshes
itself — roughly every 20 s while a match is live, every 60 s otherwise. Each row
shows the manager's season total, gameweek points so far, and a green/red arrow for
live movement within the gameweek; all sorting and rank maths happen server-side.

**Blocked by:** 03, 04

**Status:** done

- [x] A Vite React project builds to an embedded `dist/` served with SPA fallback to
      `index.html`; the production image has no Node dependency; dev is
      `vite build --watch` + `go run`
- [x] Every 200 carries the shared `{meta, data}` envelope; `meta` is identical across
      endpoints and built once per request from `Current()` — `matchLive`,
      `pollAfterMs` (20000 live / 60000 idle), `stale`, `builtAt`, `leagueName`,
      `leagueMode`, `currentGw`, `gwFinished`, `processedGws`
- [x] `GET /api/standings` returns pre-sorted rows (`entryId`, `ownerName`,
      `entryName`, `officialRank`, `liveRank`, `arrow`, `totalPoints`, `liveGwPoints`
      from `ManagerScore`, `livePoints`), sorted by `livePoints` desc; `{ "rows": [] }`
      in h2h mode
- [x] Responses carry `Cache-Control: no-cache` and an `ETag`; `If-None-Match` yields
      304 with no body
- [x] Before snapshot #1 `/api/*` returns 503 with
      `{ "error": "starting up", "retryAfterMs": 3000 }` + `Retry-After: 3`; after,
      always 200 with `meta.stale` reflecting upstream trouble
- [x] JSON keys are camelCase, `webName` accent-stripped, ids/points/ranks are numbers
- [x] The SPA is single-column with hamburger nav, lands on Standings, and polls at
      `meta.pollAfterMs`
- [x] Rows render total, GW-points-so-far, and the live arrow; the client renders the
      server's array order
- [x] seam-3 httptest coverage: envelope shape, `pollAfterMs` flip, `stale`
      passthrough, pre-snapshot 503, 304, h2h empty rows, sort order

## Notes

- The live-ordered projection lives in `fpl.LiveStandings()` (a derived view over the
  snapshot, sorted with `LiveRank`/`arrow` assigned); `internal/web` only re-tags it as
  camelCase JSON, mirroring how `/standings` (Discord) consumes `fpl.Standings()`.
- `ManagerScore` now resolves pick positions from the exported `Bootstrap.Elements`
  slice instead of the unexported index, so seam-3 tests build a `*fpl.Snapshot`
  literal and drive the handlers directly (behaviour unchanged).
- `Snapshot.ProcessedGWs()` unions three sources named by the API ticket: events whose
  `waivers_time` has passed, waiver transactions, and `game.waivers_processed` for the
  current GW.
- The committed `internal/web/dist/index.html` stays the ticket-01 placeholder; a real
  `vite build` (local `--watch` or Docker stage 1) replaces the directory. The web
  Standings row shows the frozen season total; live order is conveyed by row position
  and the arrow.
- Waiver History / Bet nav entries and the `/manager/:id` route are skeleton
  placeholders — their views land in tickets 06/08/11.
