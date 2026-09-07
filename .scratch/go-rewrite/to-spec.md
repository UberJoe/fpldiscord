# fpldiscord Go rewrite — spec

Status: ready-for-agent

Companion to the wayfinder build document [`spec.md`](spec.md) (assembled from tickets
T01–T11, [map](map.md)). This spec restates the same effort in problem / solution /
user-story / decision form, and pins the **test seams**. Where the two disagree,
`spec.md` and the underlying `issues/NN-*.md` tickets win on mechanical detail; this
document owns the seam analysis and the acceptance framing.

---

## Problem Statement

The Discord bot and companion webpage for our private Draft Fantasy Premier League run
on a Python `py-cord` codebase that has become expensive to keep alive:

- **It breaks every season on Draft FPL API drift.** `events` changed shape
  (`{current, data, next}`), `entry_history` is now `{}`, the bare `/entry/{id}`
  endpoint 403s, `transactions` moved behind a `/api/draft/league/...` prefix, and a
  new defensive-stat family leaked into goalscorer displays. Each change is a
  hand-patch against a singleton `Utils` class full of pandas joins.
- **Scores are wrong during live gameweeks.** The bot ignores the `subs[]` (auto-sub)
  array, so any manager whose starter played zero minutes is mis-scored until the
  gameweek finalises.
- **Operational drag.** Runtime depends on PIL, pandas, and a bundled font; the daily
  waiver-reminder uses fragile `datetime`/`utcnow()` math; the `bet` game is a
  hardcoded dict literal of last season's player ids with no storage; deploys carry a
  heavy image on a 256 MB fly.io machine.
- **The webpage is minimal.** There is no good phone view of live standings, the live
  gameweek squad, or waiver history.

The league owner wants a rewrite that is cheap to run, survives API drift with a
single well-tested data layer, scores gameweeks correctly, and gives league members a
usable phone-first webpage — without changing what the commands *do* from a member's
point of view (documented improvements aside).

## Solution

One Go binary, one process, deployed to fly.io's legacy free tier:

- A **Discord bot** goroutine (`bwmarrin/discordgo`) exposing the same slash commands
  members already use — `owner`, `teamlist`, `waivers`, `dave`, `scores`, `bet`,
  `overview`, `standings` always on, `fixtures` / `h2h` registered only in H2H league
  mode — plus the daily waiver-reminder task.
- An **HTTP server** (stdlib `net/http.ServeMux`) serving a `go:embed`'d
  TypeScript / React (Vite) single-page app plus a `/api/*` JSON surface that feeds
  it. The webpage is **public and read-only**: Standings (with live scores and a
  per-manager drill-down), Waiver History, and the Bet leaderboard.
- A single **`fpl` package** that holds one immutable in-memory **snapshot** of the
  Draft FPL API, rebuilt by one refresher goroutine and served last-good. Every
  command and every API endpoint is a pure function over the current snapshot. All API
  drift is absorbed in this one place.
- **SQLite** (`modernc.org/sqlite`, pure Go, no CGO) on a fly volume, persisting the
  `bet` picks only.

Members see: correct live scores (auto-subs applied), a `waivers` command that can
show failed claims and never truncates, a real season-long `bet` leaderboard backed by
storage, and a phone webpage that shows the league table updating live during a
gameweek. Operators see: a small static binary, a 3-stage Docker build, one always-on
machine, forward-only additive migrations, and a blind hard-swap cutover from the
(currently dormant) Python app.

League mode (`classic` / `h2h`) is **derived from the Draft API** (`league.scoring`
`"c"` / `"h"`), not configured. League 64 is `classic` for 2026/27, so `fixtures` and
`h2h` compile and gate but stay dormant.

## User Stories

### League member — Discord

1. As a league member, I want `/owner <player>`, so that I can see which manager owns a
   given player (or that they are a free agent).
2. As a league member, I want `/owner` to autocomplete player names, so that I don't
   have to spell "Højlund" exactly.
3. As a league member, I want `/owner` to be correct immediately after the GW20
   mid-season redraft, so that I can trust it once rosters are reshuffled.
4. As a league member, I want `/teamlist <owner>`, so that I can see a manager's full
   squad grouped by position (GK / DEF / MID / FWD).
5. As a league member, I want `/teamlist` to autocomplete owner names, so that I can
   pick a manager quickly.
6. As a league member, I want `/waivers`, so that I can see the results of the most
   recent processed waiver round without passing any arguments.
7. As a league member, I want `/waivers [gw]`, so that I can look up a specific
   gameweek's waiver round.
8. As a league member, I want `/waivers [result]` with `accepted` (default) / `failed`
   / `all`, so that I can see not just who got a player but who was out-bid.
9. As a league member, I want long `/waivers` output split across multiple messages,
   so that I never see a truncated waiver round.
10. As a league member, I want `/scores`, so that I can see every manager's live or
    provisional gameweek points.
11. As a league member, I want `/scores` to apply auto-subs from the `subs[]` array,
    so that a manager whose starter played zero minutes is scored on their actual
    scoring XI, not their submitted XI.
12. As a league member, I want `/scores` totals to reconcile with the Draft website
    once a gameweek is final, so that I can trust the number.
13. As a league member, I want `/scores [gw]`, so that I can review a past gameweek.
14. As a league member, I want `/standings`, so that I can see the classic
    total-points league table (rank, team name, total, gameweek total).
15. As a league member, I want `/overview` in its Today's / Gameweek's / Live modes,
    so that I can see fixtures and goalscorers for the current window.
16. As a league member, I want the new defensive-contribution stat family excluded
    from `/overview` goalscorer lists, so that the display stays a goalscorer display.
17. As a league member, I want `/dave` to always reply with the joke text, so that the
    running gag still works regardless of who runs it.
18. As a league member, I want the daily waiver-reminder posted to our notification
    channel ahead of the next relevant gameweek's waiver deadline, so that I don't
    forget to set my claims.
19. As a league member, I want the waiver-reminder to fire once (not repeatedly, not
    skipped by a clock-math bug) at the scheduled times, so that it stays trustworthy.

### Bettor — the `bet` game

20. As a bettor, I want `/bet` with no arguments, so that I can see the current
    season's live leaderboard.
21. As a bettor, I want each entry to show my 4 picked players and each player's
    cumulative Premier League goals for the season, so that I can see how my bet is
    doing.
22. As a bettor, I want a running total and a status marker (`in` / `🕓`
    provisionally-out / `💥` bust), so that I know where I stand at a glance.
23. As a bettor, I want to be marked provisionally out while any of my 4 picks is still
    on zero goals, and flipped back in automatically when that player scores, so that
    the "every pick must score" rule is enforced live.
24. As a bettor, I want a total over 21 to be a bust, so that the closest-to-21
    ceiling is enforced.
25. As a bettor, I want the leader to be whoever is closest to 21 without going over
    (exactly 21 wins outright), with genuine ties shown joint, so that the winner is
    unambiguous.
26. As a bettor, I want the `🏆` stamped on the leader only once gameweek 38 is
    finished, so that the trophy means the season is actually over.
27. As a bettor, I want `/bet <season>` to show a past season's archived record
    statically, so that I can look back at previous years.
28. As a bettor, I want my picks stored by my Discord user id and my display name
    resolved at render time, so that a name change doesn't detach me from my bet.

### League admin

29. As the league admin, I want `/bet set` (admin-only) with a bettor and four player
    args, so that I can enter or replace a bettor's current-season picks.
30. As the league admin, I want `/bet set` player args to autocomplete, so that I
    enter the right players.
31. As the league admin, I want `/bet archive` (admin-only) with a season, a free-text
    bettor name, and a list of `Name:goals` entries, so that I can record a completed
    season even for bettors who have left the server.
32. As the league admin, I want `/bet set` and `/bet archive` gated by an admin
    allowlist, so that ordinary members can't rewrite the bet.
33. As the league admin, I want the admin allowlist to be an environment variable, so
    that I don't need a database write or a redeploy of code to change who is admin.
34. As the league admin, I want a season/league rollover to be: run `/bet archive`
    first, then bump `SEASON` / `LEAGUE_ID` in `fly.toml` and redeploy, so that the
    outgoing season is frozen while the Draft API still serves it live.

### Webpage visitor

35. As a webpage visitor on my phone, I want a single-column, hamburger-nav SPA
    landing on Standings, so that the site is usable without pinching and zooming.
36. As a webpage visitor, I want the Standings view to list every manager in live
    order (season total plus live gameweek points, classic only), so that I see the
    table as it stands right now.
37. As a webpage visitor, I want each Standings row to show the season total, the
    gameweek points so far, and a green/red arrow for live movement within the
    gameweek, so that I can see who is climbing or falling.
38. As a webpage visitor, I want the Standings view to refresh itself roughly every
    20 seconds while a match is live and roughly every 60 seconds otherwise, so that
    it stays current without me reloading.
39. As a webpage visitor, I want to tap a manager and land on a `/manager/:id` route
    with a back button and a shareable URL, so that I can look at one manager's
    gameweek in detail.
40. As a webpage visitor, I want the manager drill-down to show the starting XI and
    bench with auto-subs already applied, points per player, the gameweek total, and
    sub in/out markers, so that I see the real scoring picture.
41. As a webpage visitor, I want the Waiver History view to show one gameweek at a
    time with a selector defaulting to the latest processed gameweek, so that I can
    browse waiver rounds.
42. As a webpage visitor, I want accepted and failed waiver claims in one table with
    bid order, so that I can see who out-bid whom.
43. As a webpage visitor, I want the Bet view to show the current season's live
    leaderboard (4 players plus goals per bettor, total, status, leader), sorted
    closest to 21 from below with busts last, so that I can follow the bet without
    Discord.
44. As a webpage visitor, I want the site to keep working (showing the last good data
    with a staleness indicator) when the Draft API is temporarily failing, so that a
    brief upstream outage doesn't blank the page.

### Operator

45. As the operator, I want one static binary with no CGO and no Node at runtime, so
    that the image is small enough for a 256 MB machine.
46. As the operator, I want `config.Load()` to run before anything else and aggregate
    every missing or invalid required key into one non-zero-exit message, so that a
    misconfigured deploy fails fast and tells me everything wrong at once.
47. As the operator, I want the effective config logged once at boot with the Discord
    token redacted, so that I can confirm what the process actually loaded.
48. As the operator, I want identifying values (`DISCORD_TOKEN`,
    `NOTIFICATION_CHANNEL_ID`, `ADMIN_IDS`, `DEV_GUILD_ID`) as fly secrets and
    operational values (`LEAGUE_ID`, `SEASON`, `DB_PATH`, `PORT`, `LOG_LEVEL`) in
    committed `fly.toml [env]`, so that the public repo leaks nothing while ordinary
    config stays in version control.
49. As the operator, I want database migrations to be forward-only additive `.sql`
    files run on boot with fail-fast, so that a bad migration stops the release
    instead of corrupting data, and a rolled-back binary still works against a newer
    schema.
50. As the operator, I want the HTTP listener to bind only after the first snapshot
    succeeds and `/healthz` to always return 200 once bound, so that fly's health
    check has a clean signal and there is no partial-serving window.
51. As the operator, I want `/api/*` to return 503 with `Retry-After` only during the
    pre-first-snapshot boot window and otherwise always 200 with a `meta.stale` flag,
    so that clients can distinguish "starting" from "degraded".
52. As the operator, I want one always-on machine with scale-to-zero disabled, so that
    the Discord gateway websocket and the reminder timer stay alive.
53. As the operator, I want `strategy = "immediate"` deploys, so that the single
    machine bound to the single volume can deploy at all (accepting ~40 s downtime).
54. As the operator, I want the bot to run `ApplicationCommandBulkOverwrite` on
    `READY`, so that `team` and `update` disappear and new commands register with no
    manual step.
55. As the operator, I want rollback to be `fly deploy --image @<digest>` of the
    previous Go release, so that recovery does not depend on the `legacy/` tree still
    existing.
56. As the operator, I want local dev to be `vite build --watch` plus `go run` with a
    repo-root `.env`, so that I don't need Docker or compose to work on it.

### Maintainer

57. As a maintainer, I want all Draft FPL API drift and quirks handled in one `fpl`
    package (two id spaces as distinct Go types, look-up-by-`id` for events, prefixed
    `transactions` URL, `/public` and `/history` endpoints, numeric-text fields parsed
    on ingest, a real `User-Agent`), so that next season's drift is one package to
    fix.
58. As a maintainer, I want auto-sub selection in one pure function
    (`ApplyAutoSubs(picks, subs, live, squad)`), so that the highest-risk piece of
    scoring logic is unit-tested in isolation.
59. As a maintainer, I want the six pandas-join helpers replaced by six pure derived
    views over the snapshot (`TeamPlayers`, `ReadableMatches`, `LeagueTransactions`,
    `Standings`, `ManagerScore`, `Overview`), so that each is independently testable
    against captured fixtures.
60. As a maintainer, I want a flat `internal/{config,fpl,store,bet,bot,web,app}`
    layout with an import graph that is a DAG rooted at `app` and `web` not importing
    `bot` (consumer-side `MemberNamer` interface, injected by `app`), so that every
    package builds and tests in isolation.
61. As a maintainer, I want scoring to trust `stats.total_points` and `stats.bonus`
    as-is with no bonus recompute and no scoring engine, so that there is far less
    surface to keep in sync with Draft's rules.
62. As a maintainer, I want the Python `draft/` tree moved to `legacy/` at cutover and
    deleted only once the parity checklist is fully ticked, so that the old behaviour
    stays diff-able during bring-up.

## Implementation Decisions

### Shape and process

- **One Go binary, one process.** Discord bot goroutine + HTTP server + shared `fpl`
  package. New Go module `github.com/UberJoe/fpldiscord` at repo root, Go 1.24. No
  module imports `legacy/`.
- **Package layout:** flat `internal/{config,fpl,store,bet,bot,web,app}`;
  `cmd/fpldiscord/main.go` is a ~15-line shell (`config.Load()` → `app.New(cfg)` →
  `app.Run(ctx)`). Import graph is a DAG rooted at `app`: `config` / `fpl` / `store`
  are leaves; `bet` → `fpl`, `store`; `bot` → `config`, `fpl`, `store`, `bet`; `web`
  → `config`, `fpl`, `bet`.
- **Id types (`ElementID`, `EntryID`, `LeagueEntryID`, `Pos`) live in `fpl`.** No
  `domain` / `types` package. `matches` / `standings` use `LeagueEntryID`;
  `element-status.owner` / `transactions.entry` / `/entry/{id}/...` URLs use `EntryID`;
  joins go through snapshot indexes.
- **`web` does not import `bot`.** `web` declares
  `type MemberNamer interface { MemberName(discordUserID string) (string, bool) }`;
  `app` injects `*bot.Bot`, which keeps a mutex-guarded id→name map (lazy REST
  `GuildMember`, ~1 h TTL). `s.StateEnabled = false`.
- **Boot sequence (owned by `internal/app`):** `config.Load()` → open DB (`PRAGMA
  journal_mode=WAL, foreign_keys=ON, busy_timeout=5000`) → run migrations (exit 1 on
  failure) → build snapshot #1 (block ≤ 30 s, exit 1 if never succeeds) → start
  `http.Server` → `discordgo.Open()` + `ApplicationCommandBulkOverwrite` on `READY` →
  start refresher + waiver-reminder goroutines → block on `SIGINT`/`SIGTERM` →
  graceful shutdown HTTP → Discord → DB.

### `fpl` data layer

- **One immutable `*Snapshot`, pointer-swapped** by a single refresher goroutine.
  `Current() *Snapshot` is lock-free, returns last-good, `nil` only until build #1.
- **Snapshot holds** parsed `Game`, `Bootstrap`, `LeagueDetails`, `ElementStatus`,
  `Transactions`, `Live map[int]LiveGW` (keyed by GW `id`), plus unexported indexes
  (`elementByID`, `ownerByEntryID`, `ownerByLeagueEntryID`, `eventByID`) and
  accent-stripped `playerNames` / `ownerNames` slices for autocomplete.
- **`LeagueMode`** derived from `LeagueDetails.League.Scoring`, not stored, read at
  startup and every refresh. Gates `fixtures`, `h2h`, and the h2h `standings` variant.
- **Refresher:** loop ~30 s, stale-while-revalidate. Per-endpoint TTLs —
  `bootstrap-static` 1 h; `details` / `element-status` / `transactions` 10 min; `game`
  2 min; `event/{currentGW}/live` 60 s while `MatchLive` else 10 min. Any change to
  `game.current_event` or `game.waivers_processed` force-refetches `bootstrap-static`
  + `details` + `element-status` + `transactions`. On fetch failure: keep previous
  snapshot, mark next served copy `Stale=true`, retry 1 s / 2 s / 4 s (cap 3) within
  the cycle. Shared keep-alive `http.Client`, `User-Agent` +
  `Accept: application/json` + gzip. `json.Unmarshal` gated on `200` + JSON
  content-type.
- **`MatchLive`** = `any(f.Started && !f.FinishedProvisional)` over
  `Live[currentGW].Fixtures` — single source for refresher cadence and the client
  poll-interval hint.
- **Scoring stance:** `ManagerScore(id, gw)` picks the scoring XI via `ApplyAutoSubs`,
  then sums those 11 players' live `stats.total_points` as-is; `stats.bonus` included
  as-is. No provisional bonus recompute, no `pl/event-status` polling, no scoring
  engine. Only `settings.squad` (position min/max, `position_type_locks`,
  `captains_disabled`) is parsed, and only for auto-subs.
- **`ApplyAutoSubs(picks []Pick, subs []Sub, live LiveGW, squad SquadSettings)
  []ScoredPlayer`:** GW finalised & `subs[]` populated → apply `subs` verbatim
  (authoritative). Live / provisional → for each starter (pos 1–11) with
  `minutes == 0` **and** whose fixture is `finished_provisional`, replace with the
  first bench player (order 12→15) that keeps `squad` min/max-per-position valid; pos
  12 (locked backup GK) only ever replaces the starting GK; never sub out a starter
  whose match is unfinished.
- **Six derived views** (pure functions over `Current()`), replacing the `fplutils.py`
  pandas helpers: `TeamPlayers` (`owner`, `teamlist`), `ReadableMatches` (`fixtures`,
  `h2h`, h2h `standings`), `LeagueTransactions` (`waivers`, web Waiver History),
  `Standings` (classic `standings`, web Standings), `ManagerScore` / `ScoredPlayer`
  (`scores`, web `/manager/:id`), `Overview` (`overview`).

### `store` (SQLite)

- **Two tables + `schema_migrations`.** `bet_pick_current(season, discord_user_id,
  slot 1–4, element_id)` — live picks, goals computed from `bootstrap-static`.
  `bet_archive(season, bettor_name, slot 1–4, player_name, final_goals)` — frozen
  end-of-season totals. Every query filters `season = $SEASON`.
- **Migration tooling:** plain versioned `.sql` files, `go:embed`'d, applied by a
  ~30-line runner (`SELECT MAX(version)` from `schema_migrations`, apply newer files
  in order, each in its own tx). No `goose` / `golang-migrate`. Forward-only,
  additive.
- **`store` public API:** `CurrentPicks(season)`, `SetPicks(season, discordUserID,
  [4]ElementID)` (tx: delete then insert exactly 4), `ArchivedSeasons()`,
  `Archive(season)`, `AddArchive(season, bettorName, [4]ArchivePick)`.
- **`store` increments an in-memory `storeGen` counter** on every `SetPicks` /
  `AddArchive`; feeds the `/api/bet` ETag.
- **Not in the DB:** admin allowlist (`ADMIN_IDS` env), waiver-reminder fired-state
  (recomputed each timer iteration + in-memory `lastSent{gw, kind}` guard), any API
  cache or audit log. DB at `/data/fpldiscord.db`, WAL.

### `bet` logic

- `internal/bet` owns bust / `in` / `provisionallyOut` / `bust` / leader computation
  and sort order, reading picks from `store` and `elements[].goals_scored` from the
  snapshot.
- Rules: 4 players per bettor, scored on cumulative season PL goals; every pick must
  have scored ≥ 1 (else `provisionallyOut`, or `out` at season end); total > 21 =
  `bust`; leader = closest to 21 from below among in-and-not-bust, exactly 21 wins
  outright, genuine ties joint; `🏆` once `game.current_event == 38 &&
  current_event_finished`. No stake/prize/close/end-date.

### Discord bot

- **`bwmarrin/discordgo`.** Hand-rolled `map[string]handlerFunc` + one dispatcher, no
  router library. Intents `IntentsGuilds | IntentsGuildMessages`.
  `ApplicationCommandBulkOverwrite` once on `READY` (guild if `DEV_GUILD_ID` set, else
  global — ~1 h propagation). Deferred / ephemeral / response-edit flow for `bet`.
- **8 always-on commands** (`owner`, `teamlist`, `waivers`, `dave`, `scores`, `bet`,
  `overview`, `standings`) + **2 h2h-gated** (`fixtures`, `h2h`, registered only when
  `league.scoring == "h"`) + the daily waiver-reminder task. `team` and `update` are
  **cut**.
- **Divergences from Python (intended):** `/scores` applies `subs[]` auto-subs;
  `/waivers` gains an `accepted`/`failed`/`all` flag and multi-message overflow;
  `/standings` has a distinct classic path; `/dave` drops the user check (unconditional
  reply); `/overview` filters the `defensive_contribution` stat family and drops
  `utcnow()`; `bet` is fully reimplemented against SQLite.
- **Autocomplete** is the only new feature — player-name args from
  `elements[].web_name`, owner-name args from `league_entries[].player_first_name`.
- **Waiver-reminder task:** `time.Timer` goroutine, recomputed each iteration (no
  drift / DST issue), reads `waivers_time` for the next relevant GW by event `id`,
  posts to `NOTIFICATION_CHANNEL_ID` at 05:00 UTC wake / same-day / T−1h. Schedule is
  a hardcoded constant, not env.

### `/api/*` JSON surface

- **Shared `{meta, data}` envelope on every 200.** `meta` is identical across
  endpoints, built once per request from `fpl.Current()`:
  `matchLive`, `pollAfterMs` (20000 when `MatchLive()` else 60000), `stale`,
  `builtAt` (RFC3339 UTC — the only timestamp in any payload), `leagueName`,
  `leagueMode`, `currentGw`, `gwFinished`, `processedGws`. **No `GET /api/meta`** — the
  landing view's first fetch bootstraps the app.
- **Exactly four endpoints:**
  - `GET /api/standings` → `{ rows: [...] }`, each row `entryId`, `ownerName`,
    `entryName`, `officialRank`, `liveRank`, `arrow` (`officialRank - liveRank`,
    positive = up), `totalPoints` (frozen, excludes live GW), `liveGwPoints`
    (`ManagerScore(entryId, currentGw).Total`, auto-subs applied), `livePoints`.
    Pre-sorted by `livePoints` desc. Classic only — H2H returns `{ rows: [] }`.
  - `GET /api/manager/{entryId}` → standalone (no `?expand=`). `entryId`, `ownerName`,
    `gw`, `provisional`, `total`, `players[]` with `elementId`, `webName`, `teamShort`,
    `pos` (1=GK…4=FWD), `squadSlot` (1–15), `points`, `minutes`, `inScoringXI`,
    `autoSubbedIn`, `autoSubbedOut`. Unknown id → **404** `{ error }`. Only `EntryID`
    crosses the wire.
  - `GET /api/waivers?gw=N` → `gw`, `rows[]` with `ownerName`, `entryId`, `in`, `out`,
    `type` (`waiver` / `freeAgent`), `status` (`accepted` / `failed` — Go resolves raw
    codes), `priority`, `index`. Trades excluded. Sorted by `index`. `gw` omitted →
    `max(processedGws)`; out-of-range → clamp (no 400); echo resolved value in
    `data.gw`.
  - `GET /api/bet` → `season`, `bettors[]` with `displayName`, `picks[]` (always 4,
    slot order, `elementId` / `webName` / `goals`), `total`, `status`
    (`in` / `provisionallyOut` / `bust`), `leader`. Pre-sorted: non-bust by `total`
    desc, bust last. `displayName` via `MemberNamer.MemberName(discordUserId)`, raw id
    fallback, raw id not shipped. Current season only.
- **Errors / staleness / caching:** snapshot exists → always 200, upstream failure
  surfaces only as `meta.stale: true`; before snapshot #1 → **503**
  `{ error: "starting up", retryAfterMs: 3000 }` + `Retry-After: 3`. Every 200 carries
  `Cache-Control: no-cache` (no `max-age`) and an `ETag`
  (`standings-<builtAtUnix>`, `manager-<entryId>-<builtAtUnix>`,
  `waivers-<resolvedGw>-<builtAtUnix>`, `bet-<storeGen>-<builtAtUnix>`); handler
  honours `If-None-Match` → **304** no body. camelCase keys via hand-written struct
  tags, accent-stripped `webName`, no `fullName`, all ids / points / goals / ranks are
  JSON numbers, error bodies `{ error: "<string>" }`.
- **Router:** stdlib `net/http.ServeMux` (1.22+ method+pattern routing) with one
  hand-written logging + panic-recover wrapper. No `chi`.

### Web MVP

- Phone-first TS / React (Vite) SPA, single column, hamburger nav, **3 views**,
  landing on Standings; desktop is the same layout widened.
- **Refresh:** client polling of `/api/*` only (no SSE / websockets). Client polls at
  `meta.pollAfterMs`. Waiver view ignores it (refetch-on-mount only); Standings, Bet,
  and the manager drill-down honour it.
- **Standings view** doubles as the live-scores view (managers in live order, per-row
  total + GW points + green/red arrow; all sorting/rank math in Go, client renders
  array order). Tapping a row → `/manager/:id` route (own route, back button,
  shareable) showing XI + bench with auto-subs applied, points per player, GW total,
  sub markers; no fixtures / goalscorers / bonus.
- **Waiver History view:** one GW + selector (default latest processed), accepted +
  failed in one table with bid order.
- **Bet view:** current season only, live-computed, sorted closest to 21 from below,
  bust last.
- **Build wiring:** Vite `build.outDir` → `internal/web/dist/` (gitignored);
  `internal/web/embed.go` carries `//go:embed all:dist`, served via
  `http.FileServerFS` with SPA fallback to `index.html`. Dev = `vite build --watch` in
  a second terminal, no dev proxy, no `dev` build tag. Production embeds `dist/`; the
  Docker image has zero Node runtime dependency.

### Config & secrets

- **9 keys**, unprefixed `SCREAMING_SNAKE_CASE`; `_ID` / `_IDS` on Discord snowflakes;
  `_MS` / `_SECONDS` on duration knobs.
- **fly secrets:** `DISCORD_TOKEN` (req), `NOTIFICATION_CHANNEL_ID` (req), `ADMIN_IDS`
  (req, non-empty — gates `/bet set` + `/bet archive`), `DEV_GUILD_ID` (optional;
  unset → global command registration).
- **`fly.toml [env]` (committed):** `LEAGUE_ID` (req, `64`), `SEASON` (req, `2026/27`,
  rollover = bump + redeploy), `DB_PATH` (`/data/fpldiscord.db`), `PORT` (`8080`, must
  equal fly `internal_port`), `LOG_LEVEL` (`info`).
- **Hardcoded, not env:** refresh TTLs, waiver-reminder schedule, Draft `User-Agent`.
- **Dropped:** `PWD` / `EMAIL` (dead login), `IMG_FONT` (`team` cut), `DAVE_*`
  (`/dave` unconditional).
- **`config.Load()`** runs at the top of boot, validates every required key,
  aggregates all missing/invalid into one non-zero-exit message, logs the effective
  config once at `info` with `DISCORD_TOKEN` redacted. `.env.example` checked in
  (placeholders); dev `.env` gitignored, loaded only in local dev, replaces
  `config.env`.

### Deployment & build

- **3-stage Dockerfile**, repo root as context: `node:22-alpine` → `vite build
  --outDir /web-dist --emptyOutDir`; `golang:1.24` Debian → `go mod download` before
  `COPY . .`, then `COPY --from=web /web-dist ./internal/web/dist`, then
  `CGO_ENABLED=0 go build -trimpath -ldflags="-s -w"`; `gcr.io/distroless/static-
  debian12`, runs as **root** (fly volume at `/data` is root-owned). Built via fly's
  remote builder (`fly deploy`), no local Docker.
- **`.dockerignore` rewritten for Go** (`.git`, `.scratch`, `legacy/`, `draft/`,
  `fonts/`, `web/node_modules`, `internal/web/dist`, `*.db*`, `.env`, `.env.*` except
  `!.env.example`, `*.md`, `.github`, `tmp/`). **`fly.toml` line removed from both
  `.dockerignore` and `.gitignore`** (T07 commits it). Dev `.env` stays gitignored.
- **`fly.toml`:** `app = "fpldiscord"`, `primary_region = "lhr"`, one always-on
  `shared-cpu-1x` / `256mb` machine (`auto_stop_machines = "off"`,
  `auto_start_machines = false`, `min_machines_running = 1`), `GOMEMLIMIT` ~`230MiB`
  soft guard, `[deploy] strategy = "immediate"` (~40 s downtime/deploy), `[[mounts]]`
  `fpldiscord_data` → `/data`, `[[http_service.checks]]` `GET /healthz`,
  `grace_period = "45s"`, `interval = "15s"`, `timeout = "5s"`.
- **`/healthz`** always returns 200 (`{"status":"ok","builtAt":<RFC3339>}`) once the
  listener binds; the listener binds only after snapshot #1, so it is
  connection-refused for ~35–40 s then 200 (grace covers it). Distinct from `/api/*`,
  which returns 503 in that window.
- **Volume:** `fly volumes create fpldiscord_data --size 1 --region lhr` (1 GB fly
  minimum; DB is kilobytes). Persists across machine replacement; no automated backups
  for MVP (bet rows re-enterable via `/bet set`).
- **Runbook:** first-time = `volumes create` + `secrets set` + `secrets unset TOKEN
  DEBUG_GUILDS` + `fly deploy`. Routine = `fly deploy`. Rollback = `fly deploy --image
  registry.fly.io/fpldiscord@<digest>` or `fly releases rollback` (safe against a
  newer schema because migrations are additive-only). Rollover = `/bet archive` first,
  then bump `fly.toml [env]`, commit, `fly deploy`.
- **Local dev:** two terminals — `cd web && npm run build -- --watch`, then `go run
  ./cmd/fpldiscord` (loads `./.env`). No compose. Optional `wgo run` for Go
  live-reload.

### Cutover & parity

- The Python app on fly is **dormant** → cutover is a **blind hard swap**: run the
  first-time steps in order, then one `fly deploy` of the Go image. No shakedown, no
  parallel run. Sign-off is Joe's, against `parity-checklist.md`.
- **`legacy/` move — two commits:** (1) cutover commit adds the Go tree at root and
  `git mv`s `draft/` + `fonts/` + `requirements.txt` + old `Dockerfile` → `legacy/`,
  new root `Dockerfile` is the Go one, `fly.toml` committed; (2) cleanup commit
  `rm -rf legacy/` once the parity checklist is fully ticked (≈ one live GW + one
  waiver run; no deadline).
- **`bet` carry-over: none.** Old picks are a stale hardcoded dict; cutover = fresh
  `/bet set` round, both tables start empty.
- **Command cleanup is automatic** — global `ApplicationCommandBulkOverwrite` on READY
  drops `team` / `update` (~1 h propagation).
- **Rollback target is the previous Go release image** — image-based, survives
  `legacy/` deletion.

## Testing Decisions

### What makes a good test here

- **Test external behaviour, not implementation.** Feed a captured real Draft FPL API
  response (or a fake snapshot / fake responder) in at a seam, assert on the value
  that crosses back out (a derived struct, a JSON body, a Discord message payload, a
  DB row). Do not assert on private fields, call counts, goroutine scheduling, or the
  internal shape of an index map.
- **Fixtures are captured real responses**, trimmed, checked in under
  `internal/fpl/testdata/*.json`. New drift = update the fixture, watch the test move.
- **The refresher goroutine, the `time.Timer` reminder loop, and live HTTP/Discord
  connections are not unit-tested** — they are covered by the parity checklist against
  real data (one gameweek with live matches + one processed waiver run). Unit tests
  target the pure logic those loops drive.
- Prefer **table tests** for anything with rule branches (auto-subs, bet status,
  waiver code resolution, `gw` clamping).

### Seams (confirmed with the developer)

Four seams, each placed as high as it can go:

1. **`fpl` core — captured Draft JSON fixtures → derived views + `ApplyAutoSubs`.**
   The keystone seam and the one carrying the most rewrite risk (pandas→Go joins, the
   auto-sub bug fix, every API-drift trap). Construct a `*Snapshot` from
   `testdata/*.json`, then assert on the pure functions: `Standings()`,
   `ManagerScore(id, gw)`, `ReadableMatches(gw)`, `LeagueTransactions(gw)`,
   `Overview(gw)`, `TeamPlayers()`, and `ApplyAutoSubs(picks, subs, live, squad)` as a
   standalone table test. Fixture cases must include: a starter on 0 minutes in a
   `finished_provisional` fixture (auto-sub in), a starter whose match is unfinished
   (no sub), a finalised GW with a populated `subs[]` (verbatim apply), the GK / pos-12
   backup-GK rule, and a squad where the naive first-bench sub would break
   min/max-per-position. Also: event look-up by `id` not index, the two id spaces not
   being conflated, and the `defensive_contribution` family excluded from `Overview`
   goalscorers.

2. **`store` — temp-file SQLite → `store` public API round-trips.** Open a `store`
   against a throwaway DB file (fresh temp dir per test), run migrations, then exercise
   `SetPicks` / `CurrentPicks` / `ArchivedSeasons` / `Archive` / `AddArchive`. Assert:
   a `SetPicks` fully replaces a bettor's 4 rows (delete-then-insert, no orphan slot
   from a previous 4), `season` filtering isolates rows, the migration runner is
   idempotent (running twice is a no-op) and applies a second migration file in order,
   and `storeGen` increments on each mutating call. No mocking of `database/sql`.

3. **`/api/*` handlers — `httptest` + fake snapshot provider + fake `MemberNamer`.**
   Build the `web` handler with a hand-built `*fpl.Snapshot` (or a small provider
   interface returning one) and a fake `MemberNamer`, fire `httptest` requests, assert
   on: status code, the exact `{meta, data}` envelope shape and camelCase keys,
   `pollAfterMs` flipping with `matchLive`, `meta.stale` passthrough, pre-snapshot
   503 + `Retry-After`, `GET /api/manager/{bad-id}` → 404, `GET /api/waivers?gw=999`
   clamping to `max(processedGws)` with the resolved value echoed in `data.gw`,
   `/api/standings` returning `{ rows: [] }` in H2H mode, pre-sorted order on
   `standings` and `bet`, and `If-None-Match` → 304 with no body. `displayName`
   resolution and raw-id fallback via the fake `MemberNamer`.

4. **Discord command handlers — fake snapshot + fake responder.** Each handler in the
   hand-rolled map is a function over (interaction options, `fpl` snapshot, `store` /
   `bet`, a responder). Call it directly with a fake responder that records what would
   be sent, assert on the rendered output: `/scores` reflects the auto-subbed XI,
   `/waivers result:failed` includes out-bid rows and splits into multiple messages
   past the length threshold (no truncation), `/standings` uses the classic shape,
   `/dave` replies unconditionally, `/overview` omits the `defensive_contribution`
   family, `/bet` renders `in` / `🕓` / `💥` / `🏆` per the rules, `/bet set` and
   `/bet archive` reject a non-allowlisted caller and round-trip through `store` for an
   allowlisted one, and autocomplete handlers return matches from the snapshot's
   `playerNames` / `ownerNames`.

### Modules tested vs. left to the parity checklist

| Tested at a seam | Left to `parity-checklist.md` (real data) |
|---|---|
| `internal/fpl` derived views + `ApplyAutoSubs` | the refresher goroutine cadence / force-refetch triggers |
| `internal/store` API + migration runner | the `time.Timer` waiver-reminder actually firing once at the scheduled time |
| `internal/bet` rule logic (via seam 1 or standalone) | live Discord command registration / `BulkOverwrite` dropping `team`/`update` |
| `internal/web` `/api/*` handlers | end-to-end totals reconciling with the Draft website once a GW is final |
| `internal/bot` command handlers | the 3 web views as a UX judgement (separate gate) |

### Prior art

There are **no existing Go tests** — this is a greenfield module. The nearest prior
art is the parity checklist itself ([`parity-checklist.md`](parity-checklist.md)),
which enumerates the behavioural-equivalence rows the seam tests should each nail down
before the checklist is exercised against live data. The Python code in `draft/` is the
behavioural reference for the six derived views (`fplutils.py` helpers
`get_team_players`, `get_readable_matches`, `get_transactions`, `get_standings`,
`get_scores` + `get_active_team`, `get_overview`) — read it to capture the intended
output shape, but the auto-sub handling is the one place the Go behaviour is
deliberately *not* a copy.

## Out of Scope

- **Multi-league / multi-tenant support.** Single league via `LEAGUE_ID` config only.
- **Web-side authentication and browser editing of `bet` picks.** All mutations are
  Discord slash commands gated by the admin allowlist.
- **The `team` pitch-image feature.** Cut — removes `teamImg.py`, PIL, the bundled
  font, and shirt-image scraping. `fonts/` may be deleted outright at cutover.
- **The `update` command.** Cut.
- **A public JSON API for third-party consumers.** `/api/*` serves only the bundled
  React app; no versioning or stability guarantee beyond that.
- **Real H2H-mode features beyond the compile-and-gate stub.** `fixtures` and `h2h`
  build and register-gate but are not exercised or polished this season; revisit when
  `league.scoring` flips to `"h"`.
- **Provisional bonus recompute / a scoring engine.** Scoring trusts
  `stats.total_points` and `stats.bonus` as-is.
- **Bonus recompute polling of `pl/event-status`.** Not built.
- **`bet` data migration from the Python dict.** Cutover is a fresh `/bet set` round.
- **CI / auto-deploy.** No `.github` today; MVP deploys are manual `fly deploy`. A
  deploy-on-push GitHub Actions workflow is post-MVP fog.
- **Automated volume backups.** Bet rows are re-enterable; fly daily volume snapshots
  are available if wanted but not configured for MVP.
- **Post-MVP web views** — archive / past-season bet view, standings trend chart,
  player-ownership browser, GW overview + goalscorer ticker, the h2h standings table,
  and a dedicated Trades view (`/api/waivers` covers waivers + free agents only).

## Further Notes

- **Superseding corrections already folded in** (from T11 assembly, so the build
  session sees only the final position): scoring trusts the API totals as-is (T05 over
  T01 §4 and T03's `scores` bonus clause); `/healthz` is always 200 once reachable
  (T09 dropped its own "503 before snapshot" draft — that behaviour lives only on
  `/api/*`); no `bet` data migration; `NOTIFICATION_CHANNEL_ID` is the settled name
  (not T02's provisional `REMINDER_CHANNEL_ID`).
- **Build order for the weekend session:** `config` → `store` → `fpl` (with fixtures)
  → `bet` → `web` → `bot` → `app`. `fpl` is the keystone; seam 1 should be green
  before `bet` / `web` / `bot` are started.
- **GW20 redraft (2027-01-03):** league 64 has a mid-season rank-order redraft.
  Roster-reading commands self-heal off live `element-status`, so this is expected to
  be a build-time check (confirm what `transactions` `kind` codes redraft picks use so
  `/waivers` renders them sensibly), not a design decision.
- **If an authed Draft feature is ever needed:** take a pasted cookie string from
  config. Programmatic `users.premierleague.com` login is bot-blocked (403) and must
  not be built around. Not in scope now — `Utils.login()` is dead code, dropped.
- **Appendices / detail sources:** [`spec.md`](spec.md) (the mechanical build doc),
  [`research/01-draft-fpl-api-surface.md`](research/01-draft-fpl-api-surface.md)
  (per-endpoint findings with trimmed real JSON),
  [`research/02-go-discord-library.md`](research/02-go-discord-library.md) (library
  comparison + hello-world sketch), [`parity-checklist.md`](parity-checklist.md), and
  the decision tickets `issues/01-*.md` … `issues/11-*.md`.
