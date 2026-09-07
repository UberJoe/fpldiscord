# 05 — Web skeleton + `/api/*` envelope + Standings web view (seam 3)

**Spec:** [../to-spec.md](../to-spec.md)

**What to build:** The deployed URL serves a phone-first React SPA (embedded in the
binary, no Node at runtime) that lands on a live-ordered league table and refreshes
itself — roughly every 20 s while a match is live, every 60 s otherwise. Each row
shows the manager's season total, gameweek points so far, and a green/red arrow for
live movement within the gameweek; all sorting and rank maths happen server-side.

**Blocked by:** 03, 04

**Status:** ready-for-agent

- [ ] A Vite React project builds to an embedded `dist/` served with SPA fallback to
      `index.html`; the production image has no Node dependency; dev is
      `vite build --watch` + `go run`
- [ ] Every 200 carries the shared `{meta, data}` envelope; `meta` is identical across
      endpoints and built once per request from `Current()` — `matchLive`,
      `pollAfterMs` (20000 live / 60000 idle), `stale`, `builtAt`, `leagueName`,
      `leagueMode`, `currentGw`, `gwFinished`, `processedGws`
- [ ] `GET /api/standings` returns pre-sorted rows (`entryId`, `ownerName`,
      `entryName`, `officialRank`, `liveRank`, `arrow`, `totalPoints`, `liveGwPoints`
      from `ManagerScore`, `livePoints`), sorted by `livePoints` desc; `{ "rows": [] }`
      in h2h mode
- [ ] Responses carry `Cache-Control: no-cache` and an `ETag`; `If-None-Match` yields
      304 with no body
- [ ] Before snapshot #1 `/api/*` returns 503 with
      `{ "error": "starting up", "retryAfterMs": 3000 }` + `Retry-After: 3`; after,
      always 200 with `meta.stale` reflecting upstream trouble
- [ ] JSON keys are camelCase, `webName` accent-stripped, ids/points/ranks are numbers
- [ ] The SPA is single-column with hamburger nav, lands on Standings, and polls at
      `meta.pollAfterMs`
- [ ] Rows render total, GW-points-so-far, and the live arrow; the client renders the
      server's array order
- [ ] seam-3 httptest coverage: envelope shape, `pollAfterMs` flip, `stale`
      passthrough, pre-snapshot 503, 304, h2h empty rows, sort order
