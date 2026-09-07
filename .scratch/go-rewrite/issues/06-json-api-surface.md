# T06 — JSON API surface (`/api/*`)

Parent: [Wayfinder map: Go rewrite](../map.md)
Type: grilling
Status: resolved
Blocked by: 04, 05

## Question

Design the `/api/*` JSON endpoints and their response payloads — the contract
between the Go binary and the embedded React app. Serves **only** the bundled app
(a public third-party API is out of scope, per the map).

Inputs are locked:

- **Views** from [T04](04-web-mvp-scope.md): Standings (live-ordered, with
  `officialRank` / `liveRank` / `liveGwPoints` / `totalPoints` / `arrow` per manager,
  pre-sorted server-side), manager drill-down (`/manager/:id` — live GW squad, XI +
  bench, auto-subs applied, points per player, auto-sub markers), Waiver History
  (one GW at a time + a GW selector), Bet Leaderboard (current season, live).
- **Refresh model** from T04: client polling; every payload carries a top-level
  `matchLive` flag + a poll-interval hint. Live cadence ~15–20 s, idle ~60 s.
- **Data + cache structs** from T05 (part B) — payload field types should reference
  the real `fpl` package structs, not invent parallel shapes.

Decide:

- The endpoint list. Candidates: `GET /api/standings`, `GET /api/manager/{id}/live`,
  `GET /api/waivers?gw=N` (+ how the GW selector learns the list of processed GWs —
  same endpoint? `/api/meta`?), `GET /api/bet`, `GET /api/meta` (league name,
  current GW `id`, `matchLive`, `leagueMode`).
- Exact response schema for each — field names, nesting, how `matchLive` + the
  poll-interval hint are carried (per-payload vs a shared envelope).
- The two id spaces from T01 (`league_entries.id` vs `entry_id`) — which one appears
  in `/api/*` URLs and payloads, and where the mapping is resolved.
- Error / staleness behaviour: what the client gets on an upstream draft-API failure
  (serve last-good + a `stale` flag? HTTP status?), consistent with T05's
  serve-last-good design.
- Whether the drill-down while open polls `/api/manager/{id}/live` independently of
  the standings poll (T04 says yes) — confirm no combined endpoint is worth it.
- Caching headers / `ETag` on `/api/*` responses given the ~300 s upstream edge
  cache and in-memory `fpl` cache.

## Answer

Two rounds of grilling with Joe; all recommendations adopted with three
clarifications from him — see Q7 (real-life football context is fine; T04's "no
fixtures" meant inter-manager H2H fixtures), Q8 (trades excluded from `/waivers`,
deferred to their own view), Q9 (exact `bet` status definitions). One fact was
checked against the [T01 research](../research/01-draft-fpl-api-surface.md) rather
than asked — bid `priority` is on the public feed (below).

---

### Envelope — shared `{ meta, data }` on every `/api/*` 200

```json
{
  "meta": {
    "matchLive": true,
    "pollAfterMs": 20000,
    "stale": false,
    "builtAt": "2026-09-06T14:03:00Z",
    "leagueName": "FPL Draft 26/27",
    "leagueMode": "classic",
    "currentGw": 4,
    "gwFinished": false,
    "processedGws": [1, 2, 3]
  },
  "data": { }
}
```

- `meta` is identical across all four endpoints, built once per request from
  `fpl.Current()`. The landing view's first fetch bootstraps the whole app, so
  **there is no `GET /api/meta`** (dropped from the T04 candidate list).
- `pollAfterMs`: **20000** when `MatchLive()`, else **60000**. Single source in Go.
  The Waiver view ignores it (refetch-on-mount only, T04); Standings, Bet and the
  manager drill-down honour it.
- `stale` mirrors `Snapshot.Stale` (T05 serve-last-good).
- `leagueMode` from `LeagueDetails.League.Scoring` (`"c"`/`"h"`), per map Notes / T03.
- `processedGws` = ascending GW ids with `waivers_processed` — drives the Waiver
  selector; sourced from `bootstrap-static.events` + transaction history.

### Endpoints

| Method + path | View | Live-polled |
|---|---|---|
| `GET /api/standings` | Standings | yes |
| `GET /api/manager/{entryId}` | Manager drill-down (`/manager/:id`) | yes, while route mounted |
| `GET /api/waivers?gw=N` | Waiver History | no (refetch on mount) |
| `GET /api/bet` | Bet Leaderboard | yes |

Exactly these four. The manager endpoint is **standalone** — no
`/api/standings?expand={id}` combined form (T04 already has the drill-down polling
independently; folding it in would couple two cadences and bloat the common-case
standings payload). The `/live` suffix from the T04 candidate name is dropped —
there is only one representation of a manager.

### ID space on the wire — `EntryID` only

`{entryId}` in the path and **every** manager identifier in **every** payload is
`EntryID` (`league_entries[].entry_id`). `LeagueEntryID` (`league_entries[].id`)
never crosses the wire: the `/api/standings` builder resolves
`LeagueEntryID -> EntryID` server-side via the snapshot's `ownerByLeagueEntryID`
index (T05). The React `/manager/:id` route then passes its param straight to
`ManagerScore(id EntryID, gw int)` with no client-side mapping.

### `GET /api/standings` — `data`

```json
{
  "rows": [
    {
      "entryId": 39880,
      "ownerName": "Joe",
      "entryName": "Bruno Dos Tres",
      "officialRank": 3,
      "liveRank": 1,
      "arrow": 2,
      "totalPoints": 365,
      "liveGwPoints": 47,
      "livePoints": 412
    }
  ]
}
```

- `totalPoints` = **frozen** season total (`standings[].total`; excludes the live GW).
- `liveGwPoints` = `ManagerScore(entryId, currentGw).Total` — auto-subs applied
  (the T01 §7 fix, via T05's `ApplyAutoSubs`).
- `livePoints` = `totalPoints + liveGwPoints` — returned so the client does no
  arithmetic.
- `arrow` = `officialRank - liveRank`; **positive = moved up** within the GW.
- `rows` is **pre-sorted** by `livePoints` desc; the client renders in array order
  and does no league-wide computation (T04 Q11).
- Classic only this season. In H2H mode the endpoint returns `{ "rows": [] }` and
  the client shows "not available in H2H" (the real H2H standings table is season
  fog / OOS).

### `GET /api/manager/{entryId}` — `data`

```json
{
  "entryId": 39880,
  "ownerName": "Joe",
  "gw": 4,
  "provisional": true,
  "total": 47,
  "players": [
    {
      "elementId": 88,
      "webName": "Saka",
      "teamShort": "ARS",
      "pos": 3,
      "squadSlot": 7,
      "points": 6,
      "minutes": 90,
      "inScoringXI": true,
      "autoSubbedIn": false,
      "autoSubbedOut": false
    }
  ]
}
```

- `pos`: 1=GK 2=DEF 3=MID 4=FWD (element type). `squadSlot`: 1–15 as submitted —
  this is T05's `ScoredPlayer.Position`, **renamed** in the payload to avoid
  colliding with `pos`.
- `teamShort`: player's real-life club 3-letter code — **included** (a squad list
  reads badly without it). Real-life opponent / kickoff are **not** in the MVP
  payload but are fine to add if a later view needs them — T04's "no fixtures"
  ruled out *inter-manager H2H fixtures*, not real-life football context.
- `provisional` = `ManagerScore.Provisional` (target GW not finalised — score still
  moving).
- `total` = sum of the scoring XI's `points` (T05 `ManagerScore.Total`).
- Rendering: the client makes **two groups keyed on `inScoringXI`** (scoring XI,
  then bench), each ordered by `pos` then `squadSlot`; `autoSubbedIn` / `autoSubbedOut`
  annotate the swapped pair.
- Unknown `entryId` -> **404**, body `{ "error": "..." }`.

### `GET /api/waivers?gw=N` — `data`

```json
{
  "gw": 3,
  "rows": [
    {
      "ownerName": "Joe",
      "entryId": 39880,
      "in": "Saka",
      "out": "Foden",
      "type": "waiver",
      "status": "accepted",
      "priority": 2,
      "index": 5
    }
  ]
}
```

- Go **resolves the raw codes** — the client has no mapping table:
  - `type`: `"waiver"` | `"freeAgent"` (from `kind` `w` / `f`).
  - `status`: `"accepted"` | `"failed"` (from `result` `a` / `do` / other denied
    codes).
  - Raw `kind` / `result` are **not** shipped.
- **Trades are excluded** from this endpoint. A dedicated Trades view is deferred
  (added to the map's Not-yet-specified).
- `priority` = the manager's waiver order for that claim. **Confirmed retrievable
  for all entries without auth**: the T01 research probed
  `/api/draft/league/{id}/transactions` unauthenticated on 2026-09-06 and it
  returned the whole league history with `priority` + `result` on every row (there
  is no session to be "logged in as" — `login()` is dead code). So the T04
  "who out-bid whom (bid order)" rendering works on fully public data.
- `rows` sorted by `index`. The client groups rows on the `in` player and orders
  each contested group by `priority` to show the bid order — no server-side
  nesting.
- No timestamp on rows (`priority` / `index` carry the ordering; `added` is omitted).
- `gw` omitted -> `max(processedGws)`. The resolved value is echoed in `data.gw`.
  Out-of-range / malformed `gw` **clamps** to the nearest valid processed GW (no
  400).
- No transactions that GW -> `{ "gw": N, "rows": [] }`.

### `GET /api/bet` — `data`

```json
{
  "season": "2026/27",
  "bettors": [
    {
      "displayName": "Joe",
      "picks": [
        { "elementId": 88, "webName": "Saka", "goals": 5 }
      ],
      "total": 18,
      "status": "in",
      "leader": true
    }
  ]
}
```

- `picks`: always 4, in slot order. `goals` per pick = live
  `bootstrap-static.elements[].goals_scored` (T05); `total` = their sum.
- `status` (exact definitions from Joe):
  - `"in"` — all 4 picks have scored **at least one goal**.
  - `"provisionallyOut"` — `total <= 21` but **not** all 4 picks have scored yet
    (>= 1 pick still on 0).
  - `"bust"` — `total > 21`.
- `leader` — the bettor with the highest `total <= 21` (**closest to 21 from
  below**). On a tie, **both** rows get `leader: true`. If every bettor is `bust`,
  no row has `leader: true`.
- `bettors` is **pre-sorted** by Go: non-bust by `total` desc (closest to 21
  first), then all `bust` rows last.
- `displayName` — bettors are keyed by Discord user id in `store` (T05). Go
  resolves a human name via `internal/web` calling `MemberName(discordUserId)` on
  the bot's `discordgo` state cache, falling back to the raw id string when the
  member isn't cached. The raw `discordUserId` is **not** shipped in the payload.
  **This is the first `internal/web -> internal/bot` dependency** — flagged for the
  module-layout ticket.
- Current season only (`SEASON` config), echoed as `season`. Archive / past seasons
  remain season fog.
- Bet rule logic (`in` / `provisionallyOut` / `bust` / `leader`, sort order) lives
  in `internal/bet` (T05), which this endpoint calls; `internal/web` only shapes
  the JSON.

### Error / staleness behaviour

- A snapshot exists -> **always 200** with a full envelope. An upstream draft-API
  failure surfaces **only** as `meta.stale: true`, never as an HTTP error status
  (consistent with T05's serve-last-good).
- Before snapshot #1 (`fpl.Current() == nil` — only during the <=30 s boot window,
  T05 boot step 3) -> **503**, body `{ "error": "starting up", "retryAfterMs": 3000 }`,
  with a `Retry-After: 3` header.
- Malformed / out-of-range `?gw=` -> **clamp** to the nearest valid processed GW,
  do not 400.
- `/api/manager/{id}` with an unknown id -> **404**, `{ "error": "..." }`.

### Caching

- Every 200 response carries `Cache-Control: no-cache` (always revalidate) and an
  `ETag`:
  - `/api/standings` -> `"standings-<builtAt-unix>"`
  - `/api/manager/{id}` -> `"manager-<entryId>-<builtAt-unix>"`
  - `/api/waivers?gw=N` -> `"waivers-<resolvedGw>-<builtAt-unix>"`
  - `/api/bet` -> `"bet-<storeGen>-<builtAt-unix>"`, where `storeGen` is an
    in-memory counter incremented on every `store.SetPicks` / `store.AddArchive`,
    so a `/bet set` **between** snapshots still invalidates the tag.
- The handler honours `If-None-Match` and returns **304** with no body when the tag
  matches — idle 60 s polls cost a bare 304.
- **No `max-age`** — the client owns cadence via `meta.pollAfterMs`, and a caching
  intermediary would defeat "live". The upstream ~300 s Fastly edge cache stays
  purely between Go and the draft API, invisible to the browser.

### Conventions

- **camelCase** JSON keys throughout, via hand-written struct tags (not Go field
  names). T04 already set the precedent (`officialRank`, `liveGwPoints`).
- `webName` ships **accent-stripped** (as the snapshot stores it). **No `fullName`**
  anywhere in `/api/*` — no MVP view renders it.
- All ids / points / goals / ranks are JSON **numbers**, never strings (T05 already
  parses numeric-text fields on ingest).
- `meta.builtAt` is **RFC3339 UTC** (`2026-09-06T14:03:00Z`). It is the **only**
  timestamp in any payload.
- Every error body is `{ "error": "<human-readable string>" }`.

### Consumed / cross-references

- `meta` fields map to T05's `fpl` public API: `CurrentGW()`, `MatchLive()`,
  `BuiltAt()`, `Stale()`, `LeagueMode` (derived).
- `data` field types reference the real T05 structs (`StandingRow`, `ManagerScore`
  / `ScoredPlayer`, `LeagueTransaction`, and `internal/bet`'s bettor result) — the
  JSON is a re-tagged / renamed projection, not a parallel model.
- Two id spaces (T01 §6): only `EntryID` is exposed.

### Graduated from fog

- **Web — Trades view** — new item in the map's Not-yet-specified (deferred from
  the `/api/waivers` scope decision).
- The **Go project / module layout** fog item gains a confirmed constraint:
  `internal/web` depends on `internal/bot` (the `MemberName` state-cache lookup for
  `/api/bet` display names) — the first such edge.

## Comments

_(none)_
