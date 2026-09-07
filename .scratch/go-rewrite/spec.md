# fpldiscord — Go rewrite spec

Assembled from wayfinder tickets T01–T10 ([map](map.md)). This is the build
document: a weekend session follows it top to bottom, nothing left to decide. Where a
later ticket corrected an earlier one, only the final position is stated here; the
per-ticket detail lives in `issues/NN-*.md`.

Resolved corrections folded in:

- **Scoring trusts the API totals** — `stats.total_points` *and* `stats.bonus` are
  used as-is. No provisional bonus recompute, no `pl/event-status` polling (T05
  supersedes T01 §4 and the "recompute" clause in T03's `scores` verdict).
- **`/healthz` is always 200** once reachable (T09 drops its own draft's "503 before
  snapshot" idea — the HTTP listener only binds after snapshot #1, so there is no
  pre-snapshot window on that port).
- **No `bet` data migration** — cutover coincides with a fresh `/bet set` round (T10).

---

## 1. Overview & target

One Go binary, one process, deployed to fly.io:

- **Discord bot** goroutine (`bwmarrin/discordgo`) — 10 slash commands + a daily
  waiver-reminder timer.
- **HTTP server** (stdlib `net/http.ServeMux`) — serves a `go:embed`'d React/Vite SPA
  and a `/api/*` JSON surface that feeds it.
- **`fpl` package** — one in-memory immutable snapshot of the Draft FPL API, swapped
  by a single refresher goroutine; every read is lock-free and served last-good.
- **SQLite** (`modernc.org/sqlite`, pure Go, CGO-free) on a fly volume — persists
  `bet` picks only.

Domain: a private Draft Fantasy Premier League Discord bot + companion webpage.
**Single league** via config (`LEAGUE_ID`, `64` for 2026/27). League mode
(`classic` / `h2h`) is **derived from the API** (`league.scoring` `"c"` / `"h"`), not
configured; league 64 is `classic` this season. The webpage is **public read-only**;
all mutations are Discord slash commands gated by an admin allowlist.

New Go module lives in this repo at root; the Python `draft/` tree moves to `legacy/`
at cutover and is deleted once the Go version reaches parity.

Deploy target: fly.io **legacy free tier** — one always-on `shared-cpu-1x` / 256 MB
machine in `lhr`, a 1 GB volume, no custom domain.

---

## 2. Draft FPL API — ground truth

Full findings with trimmed real JSON per endpoint:
[`research/01-draft-fpl-api-surface.md`](research/01-draft-fpl-api-surface.md).
Probed live 2026-09-06 (season 2026/27).

**Endpoints used** (all `https://draft.premierleague.com/api/...`, all **public — no
login, cookie, or key**; `Utils.login()` is dead code, drop it):

| Endpoint | Provides |
|---|---|
| `GET /bootstrap-static` | `elements`, `element_types`, `teams`, `events` (`{current,data,next}`), `settings` |
| `GET /game` | `current_event`, `current_event_finished`, `waivers_processed`, next `waivers_time` |
| `GET /league/{LEAGUE_ID}/details` | `league` (incl. `scoring`), `league_entries`, `matches` (h2h only), `standings` |
| `GET /league/{LEAGUE_ID}/element-status` | player ownership (`owner` = `entry_id`) |
| `GET /draft/league/{LEAGUE_ID}/transactions` | waiver / free-agent / trade history, with `priority` + `result` on every row, **unauthenticated** |
| `GET /event/{gw}/live` | `elements[].stats` (incl. `total_points`, `bonus`), `fixtures[].stats` |
| `GET /entry/{entry_id}/event/{gw}` | `picks`, `subs` |
| `GET /entry/{entry_id}/history` | replaces the now-`{}` `entry_history` key |
| `GET /entry/{entry_id}/public` | replaces the now-403 bare `/entry/{entry_id}` |

**Drift / traps the port must handle:**

1. `events` is `{current, data, next}`; `data` is a 0-indexed array of 38 whose items
   carry a **1-indexed `id`** — always look up by `id`, never by GW index.
2. **Two id spaces:** `league_entries.id` (`LeagueEntryID`) ≠ `entry_id` (`EntryID`).
   `matches` / `standings` use `LeagueEntryID`; `element_status.owner` /
   `transactions.entry` / `/entry/{id}/...` URLs use `EntryID`. Model as distinct Go
   types.
3. New defensive stat family (`defensive_contribution`,
   `clearances_blocks_interceptions`, `recoveries`, `tackles`) in `elements[]` and
   live `stats` — filter it out of `overview` goalscorer display alongside `bps` /
   `bonus`.
4. `transactions` needs the `/api/draft/league/...` prefix (un-prefixed 404s).
5. No rate-limit headers, but a ~300 s Fastly edge cache — set a real `User-Agent`
   (`fpldiscord/2.0 (+https://github.com/UberJoe/fpldiscord)`), cache locally, and
   gate `json.Unmarshal` on `200` + JSON content-type (error bodies may be HTML).
6. Numeric-text `element` fields (`form`, `points_per_game`, `expected_*`) parse to
   `float64` on ingest; live `stats` are already numbers.
7. **Auto-sub bug to fix:** the Python bot ignores `subs[]` and mis-scores any manager
   whose starter played 0 minutes. `captains_disabled: true` — every `multiplier == 1`,
   no captain logic.
8. **Scoring stance:** per-player points = live `stats.total_points` as-is;
   `stats.bonus` **also** as-is. No `settings.scoring` engine. Only `settings.squad`
   (position min/max, `position_type_locks`, `captains_disabled`) is parsed, and only
   for auto-subs.

If an authed feature is ever needed, take a **pasted cookie string** from config —
programmatic `users.premierleague.com` login is bot-blocked (403) and must not be
built around.

---

## 3. Module & package layout

- **One `go.mod`** at repo root, module `github.com/UberJoe/fpldiscord`, Go **1.24**.
  Single module — nothing imports `legacy/`.

```
go.mod  go.sum
cmd/fpldiscord/main.go          ~15-line shell: config.Load() → app.New(cfg) → app.Run(ctx) → exit code
internal/config/                Config struct + Load() (aggregate-then-fail, redacted effective-config log)
internal/fpl/                   *Snapshot, refresher goroutine, Current(), derived views, ApplyAutoSubs; OWNS id types
internal/store/                 SQLite + go:embed'd .sql migrations + ~30-line runner
internal/bet/                   bet rule logic (in / provisionallyOut / bust / leader, sort order)
internal/bot/                   discordgo session, hand-rolled handler map, 10 commands, time.Timer reminder
internal/web/                   http.Server, /api/* handlers, embedded-SPA serving
internal/app/                   boot sequence, wiring, graceful shutdown
web/                            React + Vite project (own package.json, node_modules, src/)
internal/web/dist/              Vite build.outDir target (gitignored), //go:embed all:dist
legacy/                         draft/ + fonts?/ + requirements.txt + old Dockerfile, moved at cutover
Dockerfile  fly.toml  .env.example  .dockerignore  .gitignore
```

**Import graph — a DAG rooted at `app`:**

| Package | Internal imports |
|---|---|
| `config` | — |
| `fpl` | — |
| `store` | — |
| `bet` | `fpl`, `store` |
| `bot` | `config`, `fpl`, `store`, `bet` |
| `web` | `config`, `fpl`, `bet` |
| `app` | all of the above |

- **No `internal/domain` / `internal/types`.** `ElementID`, `EntryID`,
  `LeagueEntryID`, `Pos` live in `fpl`, which everything imports.
- **`internal/web` does NOT import `internal/bot`.** `web` declares
  `type MemberNamer interface { MemberName(discordUserID string) (string, bool) }`
  and takes one in its constructor; `app` constructs `*bot.Bot` (which satisfies it)
  and injects it. Both packages build and test in isolation.
- `bot` keeps a tiny member-name cache — `map[string]string` (id→name), mutex-guarded,
  filled lazily with one REST `GuildMember` call per unknown id, ~1 h TTL.
  `s.StateEnabled` stays `false`.

**`internal/app` owns the boot sequence:**

- `New(cfg) (*App, error)` — steps 1–3: open DB (`PRAGMA journal_mode=WAL,
  foreign_keys=ON, busy_timeout=5000`), run migrations (**exit 1** on failure), build
  snapshot #1 (block ≤ 30 s, **exit 1** if it never succeeds → fly restarts).
- `Run(ctx) error` — steps 4–7: start `http.Server`; `discordgo` `Open()` +
  `ApplicationCommandBulkOverwrite` on `READY` (guild if `DEV_GUILD_ID` set, else
  global); start refresher + waiver-reminder goroutines; block on `SIGINT` /
  `SIGTERM`; graceful shutdown **HTTP → Discord → DB**.

**HTTP router:** stdlib `net/http.ServeMux` (1.22+ method+pattern routing). One
hand-written logging + panic-recover wrapper. No `chi`.

**Test layout:** `_test.go` alongside each package. `internal/fpl/testdata/*.json`
(captured real Draft responses) drive `ApplyAutoSubs` table tests + one test per
derived view. `internal/bet` gets rule tests. `internal/store` gets a migration +
round-trip test against a temp-file DB. `bot` / `web` get thin handler tests with a
fake `*fpl.Snapshot` and a fake `MemberNamer`.

**Build order for the session:**
`config` → `store` → `fpl` (with fixtures) → `bet` → `web` → `bot` → `app`. `fpl` is
the keystone.

---

## 4. Config & secrets

Full detail: [T07](issues/07-config-secrets-inventory.md). Naming: unprefixed
`SCREAMING_SNAKE_CASE`; `_ID` / `_IDS` on Discord snowflakes; `_MS` / `_SECONDS` on
any duration knob. **9 keys.**

**fly secrets** (`fly secrets set`, never committed — repo is public, so snowflakes
count as secrets):

| Key | Req | Missing → | Notes |
|---|---|---|---|
| `DISCORD_TOKEN` | yes | fail fast | bot token (was `TOKEN`) |
| `NOTIFICATION_CHANNEL_ID` | yes | fail fast | waiver-reminder target channel |
| `ADMIN_IDS` | yes, non-empty | fail fast | comma-separated Discord user ids; gates `/bet set` + `/bet archive` |
| `DEV_GUILD_ID` | no | unset → **global** command registration; set → guild-scoped, instant | was `DEBUG_GUILDS` |

**`[env]` in committed `fly.toml`** (operational, non-identifying):

| Key | Req | Default | Notes |
|---|---|---|---|
| `LEAGUE_ID` | yes | — | Draft league id (`64` this season) |
| `SEASON` | yes | — | string `2026/27`; no API season id exists; rollover = bump + redeploy |
| `DB_PATH` | no | `/data/fpldiscord.db` | local dev `./fpldiscord.db` |
| `PORT` | no | `8080` | must equal fly `internal_port` |
| `LOG_LEVEL` | no | `info` | `debug` \| `info` \| `warn` \| `error` — the only ops knob |

**Hardcoded constants, NOT env:** per-endpoint refresh TTLs; waiver-reminder schedule
(05:00 UTC wake / same-day ping / T−1h ping; timezone UTC); Draft `User-Agent`; bet
bettors/picks (they live in SQLite).

**Dropped:** `PWD` / `EMAIL` (dead login); `IMG_FONT` (`team` cut);
`DAVE_STEVE_USER_ID` (`/dave` becomes unconditional — no user check, no key).

**`config.Load()`** runs at the very top of boot, before DB and Discord. Validates
every required key, **aggregates** all missing/invalid into one non-zero-exit message.
On success, logs the effective config once at `info` with `DISCORD_TOKEN` redacted to
`***` (snowflakes printed in full).

**`.env.example`** at repo root, checked in, placeholder values, one comment line per
key. Local dev copies it to `.env` (gitignored); `.env` is loaded **only in local
dev** — on fly the real env is injected. Replaces Python's `config.env`.

---

## 5. Data layer

Full detail: [T05](issues/05-data-model.md).

### 5.1 SQLite (`internal/store`)

Two tables + a migrations table. Migration tooling is plain versioned `.sql` files,
`go:embed`'d, applied by a ~30-line runner (`SELECT MAX(version)` from
`schema_migrations`, apply newer files in order, each in its own tx). No `goose` /
`golang-migrate`. Migrations are **forward-only and additive**.

```sql
-- store/migrations/0001_init.sql
CREATE TABLE schema_migrations (
    version    INTEGER PRIMARY KEY,
    applied_at TEXT    NOT NULL
);

CREATE TABLE bet_pick_current (        -- live: goals computed from bootstrap-static
    season          TEXT    NOT NULL,
    discord_user_id TEXT    NOT NULL,
    slot            INTEGER NOT NULL CHECK (slot BETWEEN 1 AND 4),
    element_id      INTEGER NOT NULL,
    PRIMARY KEY (season, discord_user_id, slot)
);

CREATE TABLE bet_archive (             -- frozen: static end-of-season totals
    season      TEXT    NOT NULL,
    bettor_name TEXT    NOT NULL,
    slot        INTEGER NOT NULL CHECK (slot BETWEEN 1 AND 4),
    player_name TEXT    NOT NULL,
    final_goals INTEGER NOT NULL,
    PRIMARY KEY (season, bettor_name, slot)
);
```

- Every query filters `season = $SEASON`. Season rollover = bump `SEASON` + redeploy;
  old `bet_pick_current` rows are left as dead history, never auto-converted. Moving a
  season to the archive is an explicit `/bet archive` admin action.
- **Admin allowlist is NOT in the DB** — `ADMIN_IDS` env.
- **Waiver-reminder fired-state is NOT persisted** — recomputed from
  `bootstrap-static.events` + `game.waivers_processed` each timer iteration; an
  in-memory `lastSent{gw, kind}` guard stops the today/one-hour pair double-firing.
- **Nothing else is persisted** — no on-disk API cache, no audit log. DB at
  `/data/fpldiscord.db`, WAL (`-wal` / `-shm` siblings on the volume).

**`store` public API:**

```go
type CurrentBet struct { DiscordUserID string; Elements [4]ElementID }
type ArchivePick struct { PlayerName string; Goals int }
type ArchiveBet  struct { BettorName string; Picks [4]ArchivePick }

CurrentPicks(season string) ([]CurrentBet, error)
SetPicks(season, discordUserID string, els [4]ElementID) error   // tx: delete then insert exactly 4
ArchivedSeasons() ([]string, error)
Archive(season string) ([]ArchiveBet, error)
AddArchive(season, bettorName string, picks [4]ArchivePick) error
```

`store` increments an in-memory `storeGen` counter on every `SetPicks` / `AddArchive`
(feeds the `/api/bet` ETag).

### 5.2 The `fpl` package

**One immutable `*Snapshot`, pointer-swapped** by a single refresher goroutine;
`Current() *Snapshot` is lock-free and always returns last-good (`nil` only until
build #1).

```go
type Snapshot struct {
    BuiltAt time.Time
    Stale   bool // last refresh failed; this is the previous good data

    Game          GameState
    Bootstrap     Bootstrap       // elements, element_types, teams, events, settings
    LeagueDetails LeagueDetails   // league, league_entries, matches?, standings
    ElementStatus []ElementStatus
    Transactions  []Transaction
    Live          map[int]LiveGW  // keyed by GW id

    elementByID          map[ElementID]*Element
    ownerByEntryID       map[EntryID]*Owner
    ownerByLeagueEntryID map[LeagueEntryID]*Owner
    eventByID            map[int]*Event
    playerNames          []string // web_name, accent-stripped (autocomplete)
    ownerNames           []string // league_entries[].player_first_name (autocomplete)
}
```

- `LeagueMode` derived from `LeagueDetails.League.Scoring` — not stored.
- Id spaces are distinct types: `ElementID`, `EntryID`, `LeagueEntryID`. Joins go
  through the indexes.

**Refresher** — one goroutine, loop every ~30 s, stale-while-revalidate:

| Endpoint | TTL |
|---|---|
| `bootstrap-static` | 1 h |
| `details` / `element-status` / `transactions` | 10 min (or event-driven force) |
| `game` | 2 min |
| `event/{currentGW}/live` | 60 s while `MatchLive`, else 10 min |

Any change to `game.current_event` or `game.waivers_processed` force-refetches
`bootstrap-static` + `details` + `element-status` + `transactions`. On fetch failure:
keep previous snapshot, mark next served copy `Stale=true`, retry 1 s / 2 s / 4 s (cap
3) within the cycle. Shared keep-alive `http.Client`; `User-Agent` +
`Accept: application/json` + gzip.

**`MatchLive`** = `any(f.Started && !f.FinishedProvisional)` over
`Live[currentGW].Fixtures` — single source for the refresher cadence and the client
poll-interval hint.

**Scoring:** `ManagerScore(id, gw)` chooses the scoring XI via `ApplyAutoSubs`, then
sums those 11 players' live `stats.total_points` (as-is). `stats.bonus` is included
as-is. No scoring engine.

**Auto-subs — one pure function** (`internal/fpl`):

```go
ApplyAutoSubs(picks []Pick, subs []Sub, live LiveGW, squad SquadSettings) []ScoredPlayer
```

- GW finalised & `subs[]` populated → apply `subs` verbatim (authoritative).
- Live / provisional → for each starter (pos 1–11) with `minutes == 0` **and** whose
  fixture is `finished_provisional`, replace with the first bench player (order
  12→15) that keeps `squad` min/max-per-position valid; pos 12 (locked backup GK)
  only ever replaces the starting GK. Never sub out a starter whose match is
  unfinished.

**`fpl` public API** (all pure over `Current()`):

```go
Current() *Snapshot
CurrentGW() (id int, finished bool)
MatchLive() bool
BuiltAt() time.Time
Stale() bool
PlayerNames() []string
OwnerNames()  []string
OwnerByEntryID(EntryID) (Owner, bool)
OwnerByLeagueEntryID(LeagueEntryID) (Owner, bool)
ElementByID(ElementID) (Element, bool)

TeamPlayers() []TeamPlayer
OwnerOfPlayer(name string) (Owner, bool)
ReadableMatches(gw int) []ReadableMatch
LeagueTransactions(gw int) []LeagueTransaction
Standings() []StandingRow
ManagerScore(id EntryID, gw int) (ManagerScore, error)
Overview(gw int) []FixtureOverview
```

**Derived structs** (replace the pandas helpers in `fplutils.py`) — full field lists
in [T05](issues/05-data-model.md):

| Struct | Replaces | Consumers |
|---|---|---|
| `TeamPlayer` | `get_team_players` | `owner`, `teamlist` |
| `ReadableMatch` | `get_readable_matches` | `fixtures`, `h2h`, `standings` (h2h) |
| `LeagueTransaction` | `get_transactions` | `waivers`, web Waiver History |
| `StandingRow` | `get_standings` | `standings` (classic), web Standings |
| `ManagerScore` / `ScoredPlayer` | `get_scores` + `get_active_team` | `scores`, web `/manager/:id` |
| `FixtureOverview` | `get_overview` | `overview` |

`internal/bet` holds bet business logic (bust / `in` / `provisionallyOut` / `bust` /
leader), reading picks from `store` and `elements[].goals_scored` from the snapshot.

---

## 6. Discord bot

Library: **`bwmarrin/discordgo`** ([T02](issues/02-go-discord-library.md)) — 1 dep,
leanest for the 256 MB box. Hand-rolled `map[string]handlerFunc` + one dispatcher; no
router library. `s.StateEnabled = false`; intents `IntentsGuilds |
IntentsGuildMessages`. `ApplicationCommandBulkOverwrite` once on `READY` (guild if
`DEV_GUILD_ID` set, else global — global propagation can take ~1 h). Deferred /
ephemeral / response-edit flow available for `bet`.

### 6.1 Command set — [T03](issues/03-feature-triage.md)

**8 always-on** + **2 h2h-gated** (registered only when `league.scoring == "h"`; dormant
this season) + **1 daily task**. `team` and `update` are **cut**.

| Command | Gate | Target behaviour |
|---|---|---|
| `owner <player>` | always | current owner or "free agent"; reads live `element-status` (self-heals across the GW20 redraft); autocomplete on player name |
| `teamlist <owner>` | always | squad grouped GK/DEF/MID/FWD; autocomplete on owner name |
| `waivers [gw] [result]` | always | `result` = `accepted` (default) / `failed` / `all`; `failed`/`all` show who out-bid whom; **splits across multiple messages** when long, no truncation; smart-default `gw` |
| `dave` | always | unconditional reply "fuck you Dave" (no user check — Steve branch deleted) |
| `scores [gw]` | always | live/provisional GW scores per manager **with `subs[]` auto-subs applied**; `total_points` + `bonus` as-is; optional `gw` |
| `bet [season]` / `bet set` / `bet archive` | always / admin / admin | see 6.2 |
| `overview` | always | three modes (Today's / Gameweek's / Live); filter `defensive_contribution` family out of goalscorer display; no `utcnow()` |
| `standings` | always | **classic**: total-points table from classic `standings[]` (`rank`, `entry_name`, `total`, `event_total`) — distinct code path from the (absent) h2h variant |
| `fixtures` | h2h only | H2H fixtures for a GW from `matches` |
| `h2h` | h2h only | record between two owners from `matches`; autocomplete on both owner names |
| waiver-reminder daily task | always | `time.Timer` goroutine, recomputed each iteration (no drift / DST issue); reads `waivers_time` for the next relevant GW by event `id`; posts to `NOTIFICATION_CHANNEL_ID` at 05:00 UTC wake / same-day / T−1h |

**Autocomplete** is the only MVP addition — player-name args from
`elements[].web_name`, owner-name args from `league_entries[].player_first_name`.

### 6.2 `bet` — rules & surface

- Each bettor picks **4 players**, scored on their **cumulative Premier League goals
  for the whole season** (`bootstrap-static.elements[].goals_scored`); draft ownership
  is irrelevant.
- **Every one of the 4 must have scored at least once.** A bettor with a pick still on
  zero is **provisionally out** (`🕓`), flipping back in automatically if that player
  scores. At season end any pick on zero = **out**, whatever the total.
- Total **> 21 = bust** (`💥`).
- Winner: among in-and-not-bust bettors, **exactly 21 wins outright**, else **closest
  to 21 from below**. Genuine ties shown joint, no tiebreak. `🏆` stamped on the
  leader once `game.current_event == 38 && current_event_finished`.
- No stake/prize tracking, no close action, no end-date. `/bet` is a live leaderboard
  all season.

| Command | Access | Args | Behaviour |
|---|---|---|---|
| `/bet [season]` | everyone | optional `season` | No season → current, live-computed. Season given → archive view, static. Per bettor: 4 players + goals, total, status (`in` / `🕓` / `💥`), leader |
| `/bet set` | admin | `bettor` (Discord user), `p1..p4` (player, autocomplete) | Set/replace the bettor's current-season picks. Stored by Discord user id; display name resolved at render |
| `/bet archive` | admin | `season` (string), `bettor_name` (free text), `entries` (`Name:goals, Name:goals, ...`) | Add a past-season record. Bettor is free text (may have left the server) |

---

## 7. JSON API (`/api/*`)

Full detail: [T06](issues/06-json-api-surface.md). Serves **only** the bundled React
app.

### 7.1 Shared envelope — every 200

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

- `meta` is identical across all endpoints, built once per request from
  `fpl.Current()`. **There is no `GET /api/meta`** — the landing view's first fetch
  bootstraps the app.
- `pollAfterMs`: **20000** when `MatchLive()`, else **60000**. Single source in Go.
  Waiver view ignores it (refetch-on-mount); Standings, Bet, manager drill-down honour
  it.
- `stale` mirrors `Snapshot.Stale`. `processedGws` = ascending GW ids with
  `waivers_processed` (drives the Waiver selector).

### 7.2 Endpoints — exactly four

| Method + path | View | Live-polled |
|---|---|---|
| `GET /api/standings` | Standings | yes |
| `GET /api/manager/{entryId}` | Manager drill-down | yes, while route mounted |
| `GET /api/waivers?gw=N` | Waiver History | no |
| `GET /api/bet` | Bet Leaderboard | yes |

The manager endpoint is **standalone** (no combined `?expand=` form). Only `EntryID`
crosses the wire — `LeagueEntryID` is resolved server-side via the snapshot's
`ownerByLeagueEntryID` index.

**`GET /api/standings` → `data`** — `{ "rows": [...] }`, each row:
`entryId`, `ownerName`, `entryName`, `officialRank`, `liveRank`,
`arrow` (`officialRank - liveRank`, positive = moved up), `totalPoints` (frozen season
total, excludes live GW), `liveGwPoints` (`ManagerScore(entryId, currentGw).Total`,
auto-subs applied), `livePoints` (`totalPoints + liveGwPoints`). **Pre-sorted** by
`livePoints` desc. Classic only — H2H mode returns `{ "rows": [] }`.

**`GET /api/manager/{entryId}` → `data`** — `entryId`, `ownerName`, `gw`,
`provisional`, `total`, `players[]` with `elementId`, `webName`, `teamShort`,
`pos` (1=GK 2=DEF 3=MID 4=FWD), `squadSlot` (1–15 as submitted), `points`, `minutes`,
`inScoringXI`, `autoSubbedIn`, `autoSubbedOut`. Client groups on `inScoringXI` (XI
then bench), each ordered by `pos` then `squadSlot`. Unknown id → **404**
`{ "error": "..." }`.

**`GET /api/waivers?gw=N` → `data`** — `gw`, `rows[]` with `ownerName`, `entryId`,
`in`, `out`, `type` (`"waiver"` / `"freeAgent"`), `status` (`"accepted"` / `"failed"`
— Go resolves the raw codes), `priority`, `index`. **Trades excluded.** `rows` sorted
by `index`; client groups on `in` and orders each contested group by `priority`. `gw`
omitted → `max(processedGws)`; out-of-range → **clamp** (no 400). Echo resolved value
in `data.gw`.

**`GET /api/bet` → `data`** — `season`, `bettors[]` with `displayName`, `picks[]`
(always 4, slot order, each `elementId` / `webName` / `goals`), `total`, `status`
(`"in"` = all 4 scored ≥ 1; `"provisionallyOut"` = `total ≤ 21` but ≥ 1 pick on 0;
`"bust"` = `total > 21`), `leader` (highest `total ≤ 21`; ties → both `true`; all bust
→ none). **Pre-sorted**: non-bust by `total` desc, then bust last. `displayName` via
`MemberNamer.MemberName(discordUserId)`, raw id fallback; raw id not shipped. Current
season only. Bet rule logic lives in `internal/bet`.

### 7.3 Errors, staleness, caching

- Snapshot exists → **always 200**; upstream failure surfaces only as
  `meta.stale: true`.
- Before snapshot #1 (`fpl.Current() == nil`, only during the ≤ 30 s boot window) →
  **503** `{ "error": "starting up", "retryAfterMs": 3000 }` + `Retry-After: 3`.
- Every 200 carries `Cache-Control: no-cache` (no `max-age`) and an `ETag`:
  `standings-<builtAtUnix>`, `manager-<entryId>-<builtAtUnix>`,
  `waivers-<resolvedGw>-<builtAtUnix>`, `bet-<storeGen>-<builtAtUnix>`. Handler honours
  `If-None-Match` → **304** no body.
- **camelCase** JSON keys via hand-written struct tags. `webName` accent-stripped, no
  `fullName` anywhere. All ids / points / goals / ranks are JSON numbers.
  `meta.builtAt` (RFC3339 UTC) is the only timestamp in any payload. Error bodies are
  `{ "error": "<string>" }`.

---

## 8. Web MVP

Full detail: [T04](issues/04-web-mvp-scope.md). Phone-first TS / React (Vite) SPA,
`go:embed`'d, served alongside `/api/*`. Public read-only. Hamburger nav, **3 views**,
landing on Standings. Single column; desktop is the same layout widened.

**Refresh:** client polling of `/api/*` only (no SSE/websockets). Client polls at
`meta.pollAfterMs` — ~20 s when a match is live, ~60 s idle.

**Vite `build.outDir` → `../internal/web/dist/`** (gitignored). `internal/web/embed.go`
carries `//go:embed all:dist`, served via `http.FileServerFS` with SPA fallback to
`index.html`. Dev workflow: `vite build --watch` in a second terminal — no dev proxy,
no `dev` build tag. Production embeds `dist/`; the Docker image has zero Node runtime
dependency.

### View 1 — Standings (also the live-scores view)

All league managers in **live order** (`totalPoints + liveGwPoints`, **classic
only**). Per row: manager name, total points, GW points so far, and a green/red
**live-within-GW** position arrow (`arrow = officialRank - liveRank`). All sorting +
rank math happen in Go; the client renders array order. Polls at live cadence during a
GW, ~60 s idle.

**Manager drill-down** — tapping a row navigates to **`/manager/:id`** (own route,
back button works, shareable). Shows starting XI + bench, **auto-subs already
applied**, points per player, GW total, auto-sub in/out markers. **No** fixtures,
goalscorer lists, or bonus breakdown. Polls its own endpoint at live cadence while
mounted.

### View 2 — Waiver History

**One gameweek at a time with a selector** (default = latest processed GW). Accepted
**and** failed claims in one table: manager, player in, player out, status; failed
rows show who out-bid whom (bid order via `priority`). **No live poll** — refetch on
mount only.

### View 3 — Bet Leaderboard

**Current season only**, live-computed. Per bettor: 4 players + each player's season
goals, running total, status (`in` / `🕓` / `💥`), leader `🏆`. Sorted closest to 21
from below, bust last. Polls at live cadence during a GW, otherwise static.

---

## 9. Deployment & build

Full detail: [T09](issues/09-deployment-build-spec.md). `fly deploy`-able from a clean
checkout. One always-on machine, one volume, hard-swap deploys.

### 9.1 Dockerfile — 3 stages, repo root as context

```dockerfile
# --- stage 1: build the SPA ---
FROM node:22-alpine AS web
WORKDIR /app/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npx vite build --outDir /web-dist --emptyOutDir

# --- stage 2: build the Go binary ---
FROM golang:1.24 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /web-dist ./internal/web/dist
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /fpldiscord ./cmd/fpldiscord

# --- stage 3: runtime ---
FROM gcr.io/distroless/static-debian12
COPY --from=build /fpldiscord /fpldiscord
EXPOSE 8080
ENTRYPOINT ["/fpldiscord"]
```

- `--outDir /web-dist` (CLI override, not T08's local-dev `../internal/web/dist`)
  keeps the image build independent of the local outDir; stage 2 copies it into the
  embed point after `COPY . .`.
- `golang:1.24` Debian (not alpine — CGO is off anyway, avoids musl edge cases).
- `distroless/static-debian12` — CA certs + tzdata + `/etc/passwd`, smallest base for
  a static binary with working TLS. **Runs as root** (the fly volume at `/data` is
  root-owned; a `nonroot` image would need a chown init — not worth it single-tenant).
- Build via fly's **remote builder** (default `fly deploy`) — no local Docker.
- If `go.sum` is absent on the very first build, use `COPY go.mod go.sum* ./`.

### 9.2 `.dockerignore` / `.gitignore`

Rewrite the Python-era `.dockerignore`:

```
.git
.scratch
legacy/
draft/
fonts/
web/node_modules
internal/web/dist
*.db
*.db-wal
*.db-shm
.env
.env.*
!.env.example
*.md
.github
tmp/
```

**Remove the `fly.toml` line from both `.dockerignore` and `.gitignore`** — T07
commits `fly.toml`. The dev `.env` stays gitignored (replaces `config.env`).

### 9.3 `fly.toml` (committed at repo root)

```toml
app = "fpldiscord"
primary_region = "lhr"

[build]

[env]
  LEAGUE_ID = "64"
  SEASON = "2026/27"
  DB_PATH = "/data/fpldiscord.db"
  PORT = "8080"
  LOG_LEVEL = "info"

[[mounts]]
  source = "fpldiscord_data"
  destination = "/data"

[http_service]
  internal_port = 8080
  force_https = true
  auto_stop_machines = "off"
  auto_start_machines = false
  min_machines_running = 1

  [[http_service.checks]]
    method = "get"
    path = "/healthz"
    grace_period = "45s"
    interval = "15s"
    timeout = "5s"

[deploy]
  strategy = "immediate"

[[vm]]
  size = "shared-cpu-1x"
  memory = "256mb"
```

- **No scale-to-zero** — the Discord gateway websocket and the 05:00 UTC reminder
  timer need the process alive continuously.
- **`strategy = "immediate"`** — one machine bound to one volume ⇒ fly cannot roll;
  every deploy is stop-old / start-new, ~40 s downtime. Acceptable for a private bot.
- **256 MB** — the snapshot parse (~700 elements + ~16 rosters + indexes) is a few MB
  of structs. Set `GOMEMLIMIT` (~`230MiB`) as a soft GC guard; bump to `512mb` only if
  OOM is observed.
- `internal_port` must equal `PORT` (8080).

### 9.4 Health check — `/healthz`

`GET /healthz` **always returns 200** with a small JSON body (`{"status":"ok",
"builtAt":<snapshot RFC3339>}`). The HTTP listener binds only after snapshot #1
succeeds, so there is no pre-snapshot 503 window on this port — it is
connection-refused for ~35–40 s, then 200. `grace_period = "45s"` covers the cold
boot (config → open DB → migrate → snapshot #1 ≤ 30 s → HTTP).

### 9.5 Volume

`fly volumes create fpldiscord_data --size 1 --region lhr`. 1 GB (fly minimum); the DB
holds only bet rows + WAL — kilobytes. Binds to one machine (reinforces the
single-machine design). On machine replacement the volume persists (bet rows survive);
cold start re-fetches all API data. No automated backups for MVP (bet rows are
re-enterable via `/bet set`); fly daily volume snapshots available if wanted.

### 9.6 Runbook

**First-time setup** (also the cutover — see §10):

```bash
fly volumes create fpldiscord_data --size 1 --region lhr
fly secrets set DISCORD_TOKEN=… NOTIFICATION_CHANNEL_ID=… ADMIN_IDS=…
#   DEV_GUILD_ID omitted → global command registration
fly secrets unset TOKEN DEBUG_GUILDS          # Python-era names
fly deploy
```

`LEAGUE_ID` / `SEASON` / `DB_PATH` / `PORT` / `LOG_LEVEL` are in committed
`fly.toml [env]` — not `fly secrets set`.

**Routine deploy:** `fly deploy` (remote builder, no local Docker). On READY the bot
runs `ApplicationCommandBulkOverwrite` — no manual command registration.

**Migrations:** forward-only, additive `.sql`, run on boot, fail-fast. A migration
that fails in prod = machine exits non-zero = with `strategy = "immediate"` the
release fails and the machine stays down until fixed. Test each against a copy of the
prod DB before deploy.

**Rollback:**

```bash
fly releases list
fly deploy --image registry.fly.io/fpldiscord@<digest-of-last-good>
#   or: fly releases rollback
```

~40 s downtime. Safe against a schema newer than the rolled-back binary *because*
migrations are additive-only.

**SEASON / LEAGUE_ID rollover:** (1) run `/bet archive` in Discord **first** — freezes
the outgoing season's totals while the Draft API still serves live data; (2) edit
`fly.toml [env]` — bump `SEASON`, set the new `LEAGUE_ID`; (3) commit; (4) `fly deploy`.

### 9.7 Local dev

No `docker compose`. Two terminals:

```bash
cd web && npm run build -- --watch      # keeps internal/web/dist/ fresh
go run ./cmd/fpldiscord                 # loads ./.env when present
```

`.env` at repo root (gitignored), loaded only in dev. `go:embed` is compile-time, so a
frontend change needs a `go run` restart — optional `wgo run ./cmd/fpldiscord` for Go
live-reload.

---

## 10. Cutover & parity

Full detail: [T10](issues/10-parity-cutover-plan.md). The Python app on fly is
**dormant** (not in real use), so cutover is a **blind hard swap** — no shakedown, no
parallel run.

### 10.1 Cutover mechanism

Run the §9.6 first-time steps once, in order, immediately before the cutover deploy:
`fly volumes create` → `fly secrets set` → `fly secrets unset TOKEN DEBUG_GUILDS` →
`fly deploy` of the Go image. ~40 s downtime is irrelevant. Sign-off is Joe's, against
the parity checklist.

### 10.2 `legacy/` move — two commits

1. **Cutover commit/PR** — add the Go tree at repo root; `git mv draft/` →
   `legacy/` plus `fonts/`, `requirements.txt`, and the old Python `Dockerfile` →
   `legacy/`; the new root `Dockerfile` is the Go one; commit `fly.toml`. Root is
   Go-only, Python quarantined and still diff-able. (`fonts/` may instead be deleted
   outright — it dies with the `team` command.)
2. **Cleanup commit** — `rm -rf legacy/` once the parity checklist is fully ticked.

### 10.3 `bet` data carry-over — none

The Python `bet` picks are a **hardcoded dict literal** (8 bettors × 4 element ids)
and were never stored; the ids are last season's and stale for 2026/27. Nothing to
migrate. At cutover the admin runs `/bet set` once per bettor with this season's 4
picks; `bet_pick_current` / `bet_archive` start empty.

### 10.4 Parity checklist — behavioural equivalence

Standard = **behavioural equivalence** (same inputs → correct information in the
documented shape); the listed divergences (auto-sub fix, `waivers` flag, `standings`
classic path, reimplemented `bet`, no bonus recompute) are expected improvements.
Gate scope: the **8 always-on commands + the waiver-reminder task**. `fixtures` /
`h2h` are compile-and-register-gate only (dormant in classic). The **3 web views are
a separate gate** — no Python equivalent, not a `legacy/`-deletion blocker.

Recorded as [`parity-checklist.md`](parity-checklist.md) — one row per behaviour with
a "passes when…" line. Ticking it needs real data: ≈ one gameweek with live matches +
one processed waiver run. No calendar deadline.

### 10.5 Rollback window

Post-cutover rollback is `fly deploy --image @<digest>` of the previous **Go** release
(Python is dormant, not the realistic target). Image-based — survives `legacy/`
deletion; fly retains recent release images with no retention action. `legacy/` is
reference/diff only, never required to roll back. The window is gated solely on the
checklist; no pressure to rush the `legacy/` delete.

### 10.6 Discord command cleanup — automatic

Prod commands are **global**. The Go bot's `ApplicationCommandBulkOverwrite` (global,
`DEV_GUILD_ID` unset) replaces the entire global set on READY — `team` and `update`
disappear with no manual deregister (allow ~1 h global propagation). No dev guild is
used (blind swap), so no guild-scoped cleanup applies.

---

## 11. Known post-MVP fog

Deliberately deferred — none blocks the weekend build. Live list on the
[map](map.md#not-yet-specified).

- **Web — post-MVP views** — archive / past-season bet view, standings trend chart,
  player-ownership browser, GW overview + goalscorer ticker, and the `h2h` standings
  table to light up when `league.scoring` flips.
- **Web — Trades view** — `/api/waivers` covers waivers + free agents only; league
  trades are their own transactions in the same feed. Dedicated view + endpoint when
  someone wants it.
- **CI / auto-deploy** — no `.github` today; MVP deploys are manual `fly deploy`. A
  deploy-on-push GitHub Actions workflow (build web + Go, `flyctl deploy` with a
  `FLY_API_TOKEN` secret) if/when wanted.
- **GW20 redraft handling** — league 64 has a mid-season rank-order redraft
  (`2027-01-03`). Roster-reading commands self-heal off live `element-status`; confirm
  at build time what `transactions` `kind` codes redraft picks use so `waivers`
  renders them sensibly. Low stakes.

### Out of scope (never graduates)

Multi-league / multi-tenant; web-side auth and browser editing of `bet` picks;
the `team` pitch-image feature; a public JSON API for third-party consumers; real
H2H-mode features beyond the toggle stub.

---

## Appendices

- [`research/01-draft-fpl-api-surface.md`](research/01-draft-fpl-api-surface.md) —
  per-endpoint findings with trimmed real JSON (T01).
- [`research/02-go-discord-library.md`](research/02-go-discord-library.md) —
  discordgo vs disgo vs arikawa comparison + a discordgo hello-world sketch (T02).
- [`parity-checklist.md`](parity-checklist.md) — the cutover acceptance list (T10).
- `issues/01-*.md` … `issues/10-*.md` — the decision tickets, each with its full
  `## Answer` and the grilling that produced it.
