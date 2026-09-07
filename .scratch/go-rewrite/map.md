# Wayfinder map: Go rewrite

Label: `wayfinder:map`

## Destination

A written **rewrite spec** for `fpldiscord` as a **single Go binary** — Discord bot
goroutine + HTTP server + a shared `fpl` data package — with a **TypeScript / React
(Vite)** frontend embedded via `go:embed` and served alongside a `/api/*` JSON API.
**SQLite** (`modernc.org/sqlite`, pure Go) on a fly.io volume for persistence. Single
league via config, with a `classic` / `h2h` league-mode toggle. Feature set triaged
(`team` cut, `bet` reimplemented with storage, `h2h` / `fixtures` kept behind the
toggle, everything else fixed for API drift). New Go module lives in this repo; the
Python code moves to `legacy/` and is deleted once the Go version reaches parity.

The spec is done when it is detailed enough to hand to a weekend build session with
nothing left to decide.

## Notes

- Domain: a Discord bot + companion webpage for a private Draft Fantasy Premier
  League (single league, league id currently hardcoded as `12`).
- Current code (to be replaced): Python `py-cord` bot — `draft/cogs/fplcommands.py`
  (12 slash commands), `draft/cogs/waiverstasks.py` (daily waiver-reminder loop),
  `draft/fplutils.py` (singleton `Utils`, all Draft API calls + pandas joins),
  `draft/teamImg.py` (PIL pitch image — being cut).
- Deploy target: fly.io **legacy free tier** (no running cost on eligible machines),
  low RAM, few machines, 3GB volume available. No custom domain.
- Skills every session should consult: `/grilling` and `/domain-modeling`. Research
  tickets use a `/research` subagent.
- This is a **planning** map — produce decisions, not code. A later session builds.
- Locked decisions from charting (do not relitigate without cause): one Go binary /
  one process; TS + React + Vite, embedded via `go:embed`, `/api/*` JSON API;
  SQLite via `modernc.org/sqlite` with versioned `.sql` migrations; fresh Go module
  in this repo, Python -> `legacy/`; public **read-only** webpage, all mutations via
  Discord slash commands gated by a Discord user-id allowlist.
- League mode (`classic` / `h2h`) is **derived from the API** — `league.scoring` on
  `GET /api/league/{LEAGUE_ID}/details` (`"h"` / `"c"`), read at startup + refresh
  (T03; supersedes the earlier "config toggle" idea). Gates `fixtures`, `h2h`, and
  the `standings` h2h variant.

## Decisions so far

<!-- one line per closed ticket: gist of the answer, then the link for the detail -->

- [T01 — Current Draft FPL API surface](issues/01-draft-fpl-api-surface.md) — all 7
  endpoints still **public, no login** (`login()` is dead code). Drift: `events` is
  now `{current,data,next}` — look up by `id` not GW index; new defensive stats +
  scoring; `transactions` needs `/api/draft/league/...` prefix; `entry_history` now
  `{}` (use `/entry/{id}/history`); bare `/entry/{id}` 403s (use `/public`); two id
  spaces `league_entries.id` ≠ `entry_id`; ~300 s edge cache, set a real UA. Bonus
  recompute confirmed unchanged. **Bug to fix in the port:** ignoring `subs[]`
  (auto-subs) mis-scores managers; captains are disabled.
- [T02 — Go Discord library choice](issues/02-go-discord-library.md) — use
  **`bwmarrin/discordgo`** (1 dep, leanest for the 256 MB box, biggest ecosystem,
  stable); hand-rolled handler map + `ApplicationCommandBulkOverwrite` on READY;
  daily reminder is a `time.Timer` goroutine (no lib scheduler).
- [T03 — Feature triage](issues/03-feature-triage.md) — 8 always-on commands
  (`owner`, `teamlist`, `waivers`, `dave`, `scores`, `bet`, `overview`, `standings`)
  + 2 h2h-gated (`fixtures`, `h2h`) + the daily waiver-reminder task. **Cut:** `team`
  (already OOS) and `update`. `scores` gains auto-sub correctness; `waivers` gains an
  `accepted`/`failed`/`all` flag + multi-message overflow; `standings` gets a
  distinct classic path. **`bet` reimplemented**: 4 picks/bettor, cumulative PL
  goals, closest-to-21-without-bust, every pick must score; `/bet [season]` open to
  all, `/bet set` + `/bet archive` admin-only (current = live element ids, past =
  static totals). MVP addition: autocomplete on player/owner name args only. New config:
  `LEAGUE_ID`, `SEASON`, admin allowlist, dave user-id, notification channel.
- [T04 — Web MVP scope](issues/04-web-mvp-scope.md) — phone-first React (Vite) SPA,
  `go:embed`'d, hamburger nav, **3 views**, landing on Standings; all refresh via
  client polling of `/api/*` (server sends a `matchLive` flag + poll-interval hint,
  ~15–20 s live / ~60 s idle). **Standings** absorbs live scores: managers in live
  order (`total + live GW pts`, classic only), per row total + GW-points-so-far + a
  green/red **live-within-GW** position arrow; Go returns the array **pre-sorted**
  with `officialRank`/`liveRank`/`liveGwPoints`/`arrow`. Tapping a manager →
  **`/manager/:id`** route: live GW squad, XI + bench, **auto-subs applied** (T01
  fix), points per player, sub markers — no fixtures/goalscorers/bonus. **Waivers**:
  one GW + selector, accepted+failed in one table with bid order, refetch-on-mount
  only. **Bet**: current season only, live, `/bet` rules. `h2h` standings + archive
  bet seasons + goalscorer ticker = season fog. The `/api/*` surface graduates as
  [T06 — JSON API surface](issues/06-json-api-surface.md).
- [T05 — Data model, SQLite schema & `fpl` cache/refresh design](issues/05-data-model.md)
  — **DB**: two tables (`bet_pick_current` = live element ids, `bet_archive` = frozen
  totals) + `schema_migrations`; plain `go:embed`'d `.sql` + a tiny runner; DB at
  `/data/fpldiscord.db` (`DB_PATH`), WAL. Admin allowlist = `ADMIN_IDS` env, not DB;
  waiver-reminder fired-state not persisted (recompute + in-memory dedupe); nothing
  else stored. Boot: open DB → migrate (fail-fast) → first snapshot (30 s, fail-fast)
  → HTTP → Discord → refresher + reminder goroutines. **`fpl`**: one immutable
  `*Snapshot` (parsed structs + indexes) pointer-swapped by a single refresher
  goroutine; lock-free `Current()`; per-endpoint TTLs + event-driven force on GW /
  `waivers_processed` change; serve-last-good with a `Stale` flag. `MatchLive` =
  `any(started && !finished_provisional)`. **Scoring: trust `stats.total_points` AND
  `stats.bonus` as-is** — no bonus recompute, no scoring engine; only `settings.squad`
  is parsed, for `ApplyAutoSubs` (the T01 §7 fix). Six derived views become pure
  functions over the snapshot; full structs + `store` API + the
  `internal/{fpl,store,bet}` packages are specced in the ticket. Bonus call
  supersedes T01 §4 / T03 `scores`.
- [T06 — JSON API surface (`/api/*`)](issues/06-json-api-surface.md) — shared
  `{meta,data}` envelope on every response (`matchLive`, `pollAfterMs` 20 s live /
  60 s idle, `stale`, `builtAt`, `leagueName`, `leagueMode`, `currentGw`,
  `processedGws`); **no `/api/meta`**. Four endpoints: `GET /api/standings`
  (pre-sorted rows, `officialRank`/`liveRank`/`arrow`/`totalPoints`/`liveGwPoints`/
  `livePoints`; classic only, `{rows:[]}` in h2h), `GET /api/manager/{entryId}`
  (standalone, not combined; `pos`+`squadSlot` split, `teamShort` in, auto-sub
  markers, `provisional`; 404 on bad id), `GET /api/waivers?gw=N` (Go resolves
  `type`/`status` codes, `priority` on the **public** feed so bid order works
  unauthenticated, trades excluded, `gw` clamps), `GET /api/bet` (`status`
  `in`/`provisionallyOut`/`bust`, `leader` = closest to 21 from below, pre-sorted,
  `displayName` via bot state-cache). **`EntryID` only** on the wire (`LeagueEntryID`
  resolved server-side). Errors: 200 + `meta.stale` for upstream trouble, 503 only
  pre-first-snapshot. `ETag` + `304` on unchanged snapshot (bet tag folds a
  `storeGen` counter); `Cache-Control: no-cache`, no `max-age`. camelCase tags,
  accent-stripped `webName`, no `fullName`, numeric ids, RFC3339 `builtAt`,
  `{error}` bodies.
- [T07 — Config & secrets inventory](issues/07-config-secrets-inventory.md) — **9
  keys**, unprefixed `SCREAMING_SNAKE` with `_ID(S)` / `_MS` suffix rules. **fly
  secrets** (repo is public, so snowflakes too): `DISCORD_TOKEN`,
  `NOTIFICATION_CHANNEL_ID`, `ADMIN_IDS` (required, non-empty — gates `/bet set` +
  `/bet archive`), `DEV_GUILD_ID` (optional; unset = global command registration).
  **`[env]` in committed `fly.toml`**: `LEAGUE_ID`, `SEASON` (required; `2026/27`,
  bump + redeploy at rollover), `DB_PATH` (`/data/fpldiscord.db`), `PORT` (`8080`,
  must match fly `internal_port`), `LOG_LEVEL` (`info`). **Dropped:** `PWD` /
  `EMAIL` (dead login), `IMG_FONT` (`team` cut), and `/dave` loses its user check
  entirely — unconditional reply, no `DAVE_*` key. **Constants, not env:** refresh
  TTLs, waiver-reminder schedule (05:00 UTC wake / same-day / T−1h), Draft
  `User-Agent`. **Boot:** one `config.Load()` before DB + Discord, aggregates every
  missing required key into a single non-zero exit; effective config logged once
  with the token redacted. Lives as a checked-in `.env.example` (placeholders) + a
  spec table; dev `.env` gitignored, replaces `config.env`.
- [T08 — Go project & module layout](issues/08-go-module-layout.md) — one root
  `go.mod` (`github.com/UberJoe/fpldiscord`, Go 1.24); `cmd/fpldiscord/main.go` a
  ~15-line shell over **`internal/app`** which owns the T05 boot sequence +
  shutdown. Flat `internal/{config,fpl,store,bet,bot,web,app}`, import graph a DAG
  rooted at `app` (`config`/`fpl`/`store` are leaves; `bet`→`fpl`,`store`;
  `bot`→`config`,`fpl`,`store`,`bet`; `web`→`config`,`fpl`,`bet`). Id types stay in
  `fpl` — no `domain`/`types` package. **`web` does not import `bot`**: `web`
  declares a consumer-side `MemberNamer` interface, `app` injects `*bot.Bot`; `bot`
  keeps a tiny mutex-guarded id→name map (lazy REST `GuildMember`, ~1 h TTL),
  `StateEnabled` stays `false`. `web/` Vite `build.outDir` → `internal/web/dist/`
  (gitignored), `//go:embed all:dist`; dev = `vite build --watch` (no proxy, no
  build tag), prod embeds, zero Node at runtime. Router = stdlib `net/http.ServeMux`
  (1.22 patterns), no `chi`. Tests alongside; `fpl/testdata/*.json` fixtures drive
  `ApplyAutoSubs` + view tests. Build order: `config`→`store`→`fpl`→`bet`→`web`→
  `bot`→`app`.
- [T09 — Deployment & build spec](issues/09-deployment-build-spec.md) — **3-stage
  Dockerfile** (`node:22-alpine` → `vite build --outDir /web-dist`; `golang:1.24`
  Debian, `go mod download` before `COPY . .`, then `COPY --from=web` into the embed
  point, `CGO_ENABLED=0 go build -trimpath -ldflags="-s -w"`; `distroless/static-
  debian12`, runs as **root**). `.dockerignore` rewritten for Go; **`fly.toml` line
  removed from `.dockerignore` + `.gitignore`** (T07 commits it). **`fly.toml`**:
  `app="fpldiscord"`, `primary_region="lhr"`, one always-on machine
  (`auto_stop_machines="off"`, `min_machines_running=1` — Discord gateway + reminder
  timer), `shared-cpu-1x`/`256mb` + `GOMEMLIMIT`, `[deploy] strategy="immediate"`
  (single machine + volume ⇒ no rolling, ~40 s downtime/deploy), volume
  `fpldiscord_data` 1 GB `lhr`. **`/healthz` always 200** (T05 binds HTTP only after
  snapshot #1, so no 503 window — drops the T09-body "503 before" idea), 45 s check
  grace. **Runbook**: first-time (`volumes create` + `secrets set` + `secrets unset`
  old Python names + `fly deploy`); routine `fly deploy` (remote builder); rollback
  via `fly deploy --image @digest` (safe ∵ additive-only migrations); SEASON/
  LEAGUE_ID rollover = `/bet archive` **first**, then bump `fly.toml [env]` + deploy.
  Local dev = `npm run build -- --watch` + `go run` + repo-root `.env`, no compose.
- [T10 — Parity & cutover plan](issues/10-parity-cutover-plan.md) — Python app on fly
  is **dormant**, so cutover is a **blind hard swap**: run T09's first-time steps
  (`volumes create` / `secrets set` / `secrets unset` old names) then one `fly deploy`
  of the Go image; no shakedown, no parallel run. Parity standard = **behavioural
  equivalence** (documented divergences are improvements), gated on
  [`parity-checklist.md`](parity-checklist.md) — 8 always-on commands + reminder task;
  `fixtures`/`h2h` compile-and-gate only; the 3 web views are a **separate** gate, not
  a `legacy/` blocker. **`bet` carry-over: none** — old picks are a stale hardcoded
  dict; cutover = fresh `/bet set` round, `bet_pick_current`/`bet_archive` start empty.
  **`legacy/`**: cutover commit moves `draft/` + `fonts/` + `requirements.txt` + old
  `Dockerfile` → `legacy/`; a later cleanup commit `rm -rf`s it once the checklist is
  ticked (≈ one live GW + one waiver run; no deadline). Rollback = `fly deploy --image
  @digest` of the prior **Go** release (image-based, survives `legacy/` deletion).
  `team`/`update` vanish automatically via the global `BulkOverwrite` (~1 h propagation).
- [T11 — Assemble the rewrite spec](issues/11-assemble-rewrite-spec.md) — **the
  destination.** [`spec.md`](spec.md) stitches T01–T10 into one self-contained build
  document (11 sections + appendices). Reconciled the superseding notes: scoring
  trusts `stats.total_points` + `stats.bonus` as-is (T05 over T01 §4 / T03); `/healthz`
  always 200 (T09 drops its own "503 before" draft); no `bet` migration;
  `NOTIFICATION_CHANNEL_ID` is the settled name. No new questions surfaced.

---

**🏁 Destination reached.** All tickets T01–T11 resolved; the frontier is empty. The
rewrite spec is [`spec.md`](spec.md). Everything in *Not yet specified* below is
post-destination — it does not block the weekend build and returns only if someone
picks it up as a fresh effort.

## Not yet specified

<!-- in-scope fog; graduates into tickets as the frontier advances -->

- **Web — post-MVP views** (season fog from T04) — archive / past-season bet view,
  standings trend chart, player-ownership browser, GW overview + goalscorer ticker,
  and the `h2h` standings table to light up when `league.scoring` flips. Each
  graduates on its own once someone wants it; none block the weekend build.
- **Web — Trades view** (from T06) — `/api/waivers` covers waivers + free agents
  only; league trades are their own transactions in the same feed (`kind` trade
  codes, two sides per trade). A dedicated view + endpoint when someone wants it;
  not MVP.
- **CI / auto-deploy** (from T09) — no `.github` today; MVP deploys are manual
  `fly deploy`. A deploy-on-push GitHub Actions workflow (build web + Go, `flyctl
  deploy` with a `FLY_API_TOKEN` secret) if/when someone wants it off the laptop.
- **GW20 redraft handling** — league 64 has a mid-season rank-order redraft
  (`2027-01-03`). Roster-reading commands self-heal off live `element-status`;
  confirm what `transactions` `kind` codes redraft picks use so `waivers` renders
  them sensibly. Low stakes — likely a build-time check, not a decision.

## Out of scope

<!-- ruled beyond the destination; never graduates -->

- Multi-league / multi-tenant support — single league via config only.
- Web-side authentication and browser editing of `bet` picks — mutations are
  Discord-only for this effort.
- The `team` pitch-image feature — cut; removes `teamImg.py`, PIL, the bundled font,
  and shirt-image scraping.
- A public JSON API for third-party consumers — `/api/*` serves only the bundled
  React app.
- Real H2H-mode features beyond the toggle stub — revisit next season when the league
  switches to H2H.
