# T04 — Web MVP scope

Parent: [Wayfinder map: Go rewrite](../map.md)
Type: grilling
Status: resolved
Blocked by: 03

## Question

Which **views** ship in the weekend version of the webpage, and what does each show?
The webpage is a public read-only TS/React app served by the Go binary; its whole job
is the things Discord chat renders badly.

Starting position from charting: **live scores** and **waiver history** are the two
named candidates. For the MVP set, decide per view:

- Exact content and layout intent (a table of what, sorted how).
- Refresh model — htmx-style poll interval / React `setInterval` / SSE — and how
  "live" it needs to be during matches.
- Which `fpl` data + which stored data it needs (feeds the `/api/*` surface, which
  graduates from fog once this is settled).
- Mobile-first? (league checks it on phones during games — assume yes unless told
  otherwise.)

Then split: **MVP views** vs **season fog** (things to add across the season —
standings trends, bet leaderboard, player-ownership browser, GW overview, etc.).

Depends on T03 so we only build web views for behaviours that survived triage.

## Answer

Phone-first TS / React (Vite) SPA, `go:embed`'d into the Go binary, served alongside
`/api/*`. Public read-only (locked in charting). **Hamburger** nav, **3 views**,
landing on **Standings**.

**Refresh model (all views):** client polling of `/api/*` JSON — no SSE/websockets on
the 256 MB box. The server returns a top-level `matchLive` flag + a poll-interval
hint; the client polls **~15–20 s when a match is live, ~60 s idle**. Views that
aren't match-sensitive poll slower or not at all (below).

### View 1 — Standings (also the live-scores view)

Live Scores is **not** a separate view — it is folded into Standings and its
manager drill-down.

- All league managers shown in **live order**: sorted by `total + live GW points`.
  **Classic mode only** this season.
- Per row: manager name, **total points**, **points gained so far this GW**, and a
  green/red **position arrow**.
- **Arrow baseline = live movement within the current GW** (Q10a): baseline is the
  official `standings[].rank` (frozen until the GW finalises); "current" is the live
  rank recomputed from `total + live GW points`. Arrow = `officialRank - liveRank`.
  `standings[].last_rank` is left available for a future subtle "since last GW"
  marker, but MVP renders live-within-GW only.
- **Sorting + rank math happen in Go** (Q11): `/api/standings` returns the array
  **pre-sorted** with `officialRank`, `liveRank`, `liveGwPoints`, `totalPoints`,
  `arrow` per manager. The React client renders only — no league-wide computation on
  the phone, "live" logic lives in one place.
- Poll: live cadence during a GW (`liveGwPoints` moves), ~60 s idle.
- **Manager drill-down** — tapping a row navigates to its **own route
  `/manager/:id`** (not an in-page modal): back button works, links are shareable in
  Discord. Content: **starting XI + bench, auto-subs already applied** (fixes the T01
  mis-scoring bug), points per player, GW total, and markers for which players were
  auto-subbed in/out. **No** fixtures, goalscorer lists, or bonus breakdown in MVP —
  just the number and how it was built. This route polls its own endpoint at the
  live cadence while open; the standings list polls independently.
- **`h2h` standings table is shelved** — a build-time branch on `league.scoring`
  (per T03), not designed here. Real H2H features remain out of scope (map).

### View 2 — Waiver History

- **One gameweek at a time with a selector** (default = latest processed GW).
- Accepted **and** failed claims in **one table**: manager, player in, player out,
  status. For failed rows, show who out-bid whom (bid order). Web has the room — no
  `accepted`/`failed`/`all` flag needed (that flag stays on the `/waivers` Discord
  command only, per T03).
- **No live poll** — refetch on mount only; waivers settle once a week.

### View 3 — Bet Leaderboard

- **Current season only**, live-computed. Mirrors `/bet` (T03 rules).
- Per bettor: 4 players + each player's season goals, running total, status
  (`in` / `🕓` provisionally out / `💥` bust), and the leader `🏆`.
- Sorted **closest to 21 from below**, bust last.
- Poll: only tracks match-time goal changes — live cadence during a GW, otherwise
  static. **Archive / past seasons are season fog** (static rows, cheap to add
  later — not in MVP).

### Mobile-first

Yes (Q2). Single column, thumb-reachable. Desktop is the same layout widened
naturally — no separate desktop design in MVP.

### MVP vs season fog

**MVP:** the 3 views above.
**Season fog** (to the map's Not-yet-specified): archive bet seasons, standings
trend chart, player-ownership browser, GW overview / goalscorer ticker, `h2h`
standings table when `league.scoring` flips.

### Graduates from fog

The **`/api/*` JSON surface** is now sharp — design the endpoints + payloads that
feed these 3 views (candidates: `/api/standings`, `/api/manager/{id}/live`,
`/api/waivers?gw=N`, `/api/bet`, `/api/meta` for league name / current GW /
`matchLive` / league mode). Created as a new ticket, **blocked by T04 + T05** so
payload field types can reference the real `fpl` cache structs from the data-model
ticket instead of being reworked.

### Terms coined this session

- **Live rank** — a manager's standings position recomputed from
  `total + live GW points`, changes minute-to-minute during matches.
- **Official rank** — `standings[].rank` from the draft API; only changes when a GW
  is finalised. The arrow baseline.
- **Drill-down** — the `/manager/:id` route showing one manager's live GW squad.
