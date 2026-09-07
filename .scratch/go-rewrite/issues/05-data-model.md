# T05 — Data model, SQLite schema & `fpl` cache/refresh design

Parent: [Wayfinder map: Go rewrite](../map.md)
Type: domain-modeling
Status: resolved
Blocked by: 01, 03

## Question

Two linked designs. **Inputs from [T03](03-feature-triage.md):** league mode is
API-derived (not stored); `update` is cut (no manual-refresh state); `bet` is the
only real persistence need and has **two shapes** — current season = bettor Discord
user id + 4 element ids (goals computed live), past season = season label + free-text
bettor name + 4 × `{player_name, final_goals}` static.

### A. Persistent state (SQLite via `modernc.org/sqlite`)

What does the rewrite store, and in what schema? Known:

- **`bet` picks** — model the two shapes above. Bettor keyed by Discord user id for
  the current season, free-text for archived seasons. Seasons retained (archive is a
  feature). Season identified by the `SEASON` config string. `/bet archive` writes
  static rows; `/bet set` writes live-id rows.
- **Discord user-id allowlist** — who may run mutating commands (`/bet set`,
  `/bet archive`). In DB (editable) or in config (redeploy to change)? Decide.
- **Waiver-reminder state** — does the daily task need to persist "last fired" to
  survive restarts, or is recompute-from-`waivers_time`-on-boot enough?
- **League-mode flag** — NOT stored (derived from `league.scoring` each refresh).
  Confirm nothing else wants runtime-config-in-DB vs boot-config.

Decide: migration tooling (plain versioned `.sql` + a tiny runner, or `goose` /
`golang-migrate`), where the DB file lives on the fly volume, and the boot sequence
(open DB -> run migrations -> start bot + server).

### B. The `fpl` package — live API data cache & refresh

Replaces the Python `Utils` singleton + its 1-hour / gameweek-change refresh logic.

- What is cached in memory (raw endpoint responses? parsed structs?) and for how long.
- Refresh triggers: time interval (per-endpoint TTLs from T01 §5), gameweek change,
  `waivers_processed` change, during-match faster polling of `/event/{gw}/live`.
  (`update` command is cut — no manual trigger.)
- Autocomplete needs fast in-memory lookups: `elements[].web_name` (player args) and
  `league_entries[].player_first_name` (owner args) — fold into the cache shape.
- Concurrency: bot goroutine and HTTP handlers both read the cache — mutex /
  `sync.RWMutex` / channel-owned? Stale-while-revalidate?
- Failure handling: serve last-good on API error; surface staleness to callers.
- No pandas — define the Go structs and the join/merge helpers that replace
  `get_team_players`, `get_readable_matches`, `get_transactions`, `get_scores`.

## Answer

Two rounds of grilling with Joe; every recommendation adopted, with **one correction
in round 2**: `stats.bonus` is now trusted directly — the provisional-bonus recompute
is dropped entirely (supersedes [T01](01-draft-fpl-api-surface.md) §4 and the
"recompute per T01 §4" clause in [T03](03-feature-triage.md)'s `scores` verdict;
`pl/event-status` / `bonus_added` polling is not needed).

---

### A. Persistent state — SQLite (`modernc.org/sqlite`)

**Two tables, matching the two real `bet` shapes — no nullable mega-table:**

```sql
-- store/migrations/0001_init.sql
CREATE TABLE schema_migrations (
    version    INTEGER PRIMARY KEY,
    applied_at TEXT    NOT NULL
);

CREATE TABLE bet_pick_current (
    season          TEXT    NOT NULL,
    discord_user_id TEXT    NOT NULL,
    slot            INTEGER NOT NULL CHECK (slot BETWEEN 1 AND 4),
    element_id      INTEGER NOT NULL,
    PRIMARY KEY (season, discord_user_id, slot)
);

CREATE TABLE bet_archive (
    season      TEXT    NOT NULL,
    bettor_name TEXT    NOT NULL,
    slot        INTEGER NOT NULL CHECK (slot BETWEEN 1 AND 4),
    player_name TEXT    NOT NULL,
    final_goals INTEGER NOT NULL,
    PRIMARY KEY (season, bettor_name, slot)
);
```

- Current-season goals are computed live from `bootstrap-static.elements[].goals_scored`;
  archive rows are frozen totals. Every query filters `season = $SEASON`.
- **Season rollover** = bump `SEASON` + redeploy. Old `bet_pick_current` rows are left
  as dead history, never auto-converted; moving a season into the archive view is an
  explicit `/bet archive` admin action (goal totals are only stable once GW38 is
  finalised, which `/bet archive` captures by hand).
- **Admin allowlist is NOT in the DB** — `ADMIN_IDS` env var (comma-separated Discord
  user ids). No `/bet admin` command surface, no first-admin bootstrap problem.
- **Waiver-reminder fired-state is NOT persisted** — recomputed from
  `bootstrap-static.events` + `game.waivers_processed` each timer iteration; an
  in-memory `lastSent{gw, kind}` guard stops the today/one-hour ping pair
  double-firing. Restart tolerance here is cosmetic.
- **Nothing else is persisted** — no on-disk API cache (cold start re-fetches), no
  command audit log. Schema stays at two tables; the volume stays essentially empty.

**Migration tooling:** plain versioned `.sql` files, `go:embed`'d, applied by a
~30-line runner (`SELECT MAX(version)` from `schema_migrations`, apply newer files in
order, each in its own tx). No `goose` / `golang-migrate` dependency.

**DB location:** fly volume mounted at `/data`; DB at `/data/fpldiscord.db`. `DB_PATH`
env overrides (default `/data/fpldiscord.db`; local dev `./fpldiscord.db`). WAL mode →
`-wal` / `-shm` siblings live next to it on the volume.

**Boot sequence (single process):**

1. open DB (`DB_PATH`); `PRAGMA journal_mode=WAL, foreign_keys=ON, busy_timeout=5000`
2. run migrations — **exit 1 on failure**
3. build the first `fpl` snapshot — block up to 30 s; **exit 1 if it never succeeds**
   (fly restarts the machine)
4. start `http.Server` (serves the `go:embed`'d React app + `/api/*`)
5. `discordgo` session `Open()`; on `READY` → `ApplicationCommandBulkOverwrite`
   (guild if `DEV_GUILD_ID` set, else global)
6. start the `fpl` refresher goroutine and the waiver-reminder `time.Timer` goroutine
7. block on `SIGINT` / `SIGTERM`; graceful shutdown: HTTP, then Discord, then DB

### B. The `fpl` package — in-memory cache & refresh

**One immutable `*Snapshot`, pointer-swapped (replaces the Python `Utils` singleton):**

```go
type Snapshot struct {
    BuiltAt time.Time
    Stale   bool // last refresh failed; this is the previous good data

    // parsed endpoint responses — top-level structs, not raw bytes
    Game          GameState       // /api/game
    Bootstrap     Bootstrap       // /api/bootstrap-static (elements, element_types,
                                  //   teams, events, settings)
    LeagueDetails LeagueDetails   // /api/league/{id}/details (league, league_entries,
                                  //   matches?, standings)
    ElementStatus []ElementStatus // /api/league/{id}/element-status
    Transactions  []Transaction   // /api/draft/league/{id}/transactions
    Live          map[int]LiveGW  // /api/event/{gw}/live, keyed by GW id

    // indexes, rebuilt each refresh
    elementByID          map[ElementID]*Element
    ownerByEntryID       map[EntryID]*Owner
    ownerByLeagueEntryID map[LeagueEntryID]*Owner
    eventByID            map[int]*Event
    playerNames          []string // elements[].web_name, accent-stripped (autocomplete)
    ownerNames           []string // league_entries[].player_first_name (autocomplete)
}
```

- `LeagueMode` (`classic` / `h2h`) is derived from `LeagueDetails.League.Scoring`
  (`"c"` / `"h"`) per the map Notes and T03 — not stored.
- The three id spaces are distinct Go types: `ElementID`, `EntryID`, `LeagueEntryID`
  (T01 §6). Joins go through the indexes above.
- Numeric-text `Element` fields (`form`, `points_per_game`, `expected_*`) are parsed
  to `float64` on ingest (T01 §5); live `stats` are already numbers.

**Refresher — one goroutine, stale-while-revalidate by construction:**

- Loop every ~30 s. For each endpoint whose TTL expired, fetch. On any change to
  `game.current_event` or `game.waivers_processed`, force-refetch `bootstrap-static` +
  `details` + `element-status` + `transactions`.
- TTLs: `bootstrap-static` 1 h; `details` / `element-status` / `transactions` 10 min
  (or the event-driven force above); `game` 2 min; `event/{currentGW}/live` 60 s while
  `MatchLive`, else 10 min.
- Build a fresh `*Snapshot` (parse + indexes), then atomically swap the pointer.
  Readers call `Current() *Snapshot` → lock-free, no torn reads, always last-good.
- Fetch failure: keep the previous snapshot, mark the next served copy `Stale=true`,
  log, retry 1 s / 2 s / 4 s (cap 3) within the cycle. Shared keep-alive `http.Client`;
  `User-Agent: fpldiscord-bot/2.0 (+https://github.com/…/fpldiscord)`,
  `Accept: application/json`, gzip; gate `json.Unmarshal` on `200` + JSON
  content-type (T01 §5). Cold start: `Current()` returns `nil` until build #1
  (boot step 3 blocks on it).

**`MatchLive`:** `any(f.Started && !f.FinishedProvisional)` over
`Live[currentGW].Fixtures`. Single source for both the refresher's fast/slow cadence
and T04's client poll-interval hint.

**Scoring — trust the API totals (round-2 correction):**

- Per-player points = `Live[gw].Elements[id].Stats.TotalPoints` **as-is**.
  `Stats.Bonus` is **also trusted as-is** — no provisional recompute, no
  `pl/event-status` polling.
- **No `settings.scoring` engine.** Only `settings.squad` is parsed (position
  min/max, `position_type_locks`, `captains_disabled`) — and only for auto-subs.
- `ManagerScore(id, gw)` = choose the scoring XI via `ApplyAutoSubs`, then sum those
  11 players' `TotalPoints`.

**Auto-subs — one pure function in `fpl` (the T01 §7 mis-scoring fix):**

`ApplyAutoSubs(picks []Pick, subs []Sub, live LiveGW, squad SquadSettings) []ScoredPlayer`

- GW finalised & `subs[]` populated → apply `subs` verbatim (authoritative).
- Live / provisional (no `subs` yet) → for each starter (pos 1–11) with
  `minutes == 0` **and** whose fixture is `finished_provisional`, replace with the
  first bench player (order 12→15) that keeps `squad` min/max-per-position valid;
  pos 12 (locked backup GK) only ever replaces the starting GK. Never sub out a
  starter whose match is unfinished.

Backs `/scores` and the web `/manager/:id` drill-down. `bet` is unaffected (it
ignores rosters).

**Public API** — all pure over `Current()`:

```go
// snapshot access
Current() *Snapshot                 // immutable; nil until the first build
CurrentGW() (id int, finished bool)
MatchLive() bool
BuiltAt() time.Time
Stale() bool

// autocomplete (from prebuilt indexes)
PlayerNames() []string
OwnerNames()  []string

// id directory
OwnerByEntryID(EntryID) (Owner, bool)
OwnerByLeagueEntryID(LeagueEntryID) (Owner, bool)
ElementByID(ElementID) (Element, bool)

// derived views — pure functions over *Snapshot
TeamPlayers() []TeamPlayer
OwnerOfPlayer(name string) (Owner, bool)
ReadableMatches(gw int) []ReadableMatch
LeagueTransactions(gw int) []LeagueTransaction
Standings() []StandingRow
ManagerScore(id EntryID, gw int) (ManagerScore, error)
Overview(gw int) []FixtureOverview
```

**Derived structs, what they replace, and their consumers:**

| Struct | Replaces (`fplutils.py`) | Consumers |
|---|---|---|
| `TeamPlayer` | `get_team_players` | `owner`, `teamlist` |
| `ReadableMatch` | `get_readable_matches` | `fixtures`, `h2h`, `standings` (h2h) |
| `LeagueTransaction` | `get_transactions` | `waivers`, web Waiver History |
| `StandingRow` | `get_standings` | `standings` (classic), web Standings |
| `ManagerScore` / `ScoredPlayer` | `get_scores` + `get_active_team` | `scores`, web `/manager/:id` |
| `FixtureOverview` | `get_overview` | `overview` |

```go
type Pos int // 1=GK 2=DEF 3=MID 4=FWD

type TeamPlayer struct {
    OwnerName    string
    OwnerEntryID EntryID
    ElementID    ElementID
    WebName      string // accent-stripped
    FullName     string
    Position     Pos
    TeamName     string
    TotalPoints  int
    GoalsScored  int
    Assists      int
    CleanSheets  int
    DraftRank    int
}

type MatchSide struct {
    OwnerName     string // "Average" when the league entry is nil
    LeagueEntryID LeagueEntryID
    EntryID       EntryID
    Points        int
}
type ReadableMatch struct {
    GW       int
    Finished bool
    Home     MatchSide
    Away     MatchSide
    Winner   *LeagueEntryID // nil on a draw or unplayed
}

type LeagueTransaction struct {
    GW        int
    Index     int // ordering within the batch; sort by (GW, Index)
    OwnerName string
    EntryID   EntryID
    In        string // web_name, or "" 
    Out       string // web_name, or ""
    Kind      string // "w" waiver, "f" free agent, trade kinds
    Result    string // "a" accepted, "do" denied-other, …
    Priority  int
    Added     time.Time
}

type StandingRow struct {
    Rank          int
    OwnerName     string
    EntryName     string
    LeagueEntryID LeagueEntryID
    Total         int
    EventTotal    int // classic standings[] carries this for free
}

type ScoredPlayer struct {
    ElementID     ElementID
    WebName       string
    Position      int  // 1–15 as submitted
    Points        int  // Live stats.total_points, as-is
    Minutes       int
    AutoSubbedIn  bool
    AutoSubbedOut bool
    InScoringXI   bool
}
type ManagerScore struct {
    EntryID     EntryID
    GW          int
    Total       int
    Provisional bool // target GW not finalised — score still moving
    Players     []ScoredPlayer
}

type OverviewStat struct {
    StatName   string
    PlayerName string
    OwnerName  string
    Value      int
}
type FixtureOverview struct {
    TeamHome, TeamAway   string
    HomeScore, AwayScore int
    Started, Finished    bool
    Kickoff              time.Time
    Scorers              []OverviewStat // render filters out bps / bonus /
                                        //   defensive_contribution family (T03 §10)
}
```

### `store` package — public API

```go
type CurrentBet struct {
    DiscordUserID string
    Elements      [4]ElementID
}
type ArchivePick struct {
    PlayerName string
    Goals      int
}
type ArchiveBet struct {
    BettorName string
    Picks      [4]ArchivePick
}

CurrentPicks(season string) ([]CurrentBet, error)
SetPicks(season, discordUserID string, els [4]ElementID) error // tx: delete then insert exactly 4
ArchivedSeasons() ([]string, error)
Archive(season string) ([]ArchiveBet, error)
AddArchive(season, bettorName string, picks [4]ArchivePick) error
```

Bet business logic (bust / `in` / `🕓` provisionally-out / `💥` bust / leader `🏆`)
lives in **`internal/bet`**, not `store` and not `fpl`: it reads picks from `store`
and `elements[].goals_scored` from the `fpl` snapshot.

### Packages this ticket introduces

`internal/fpl` (HTTP client + refresher + `Snapshot` + derived views + `ApplyAutoSubs`),
`internal/store` (SQLite + embedded `.sql` migrations + runner), `internal/bet` (bet
rules over `store` + `fpl`). **No `scoring` package.** The full `cmd/` + `internal/bot`
+ `internal/web` + `web/` tree stays fog → the "Go project / module layout" item
graduates next.

### Config keys surfaced

`DB_PATH` (default `/data/fpldiscord.db`), `ADMIN_IDS` (comma-separated Discord user
ids). Refresh TTLs and reminder offsets are hardcoded constants unless T07 rules
otherwise. No FPL credentials anywhere (T01 §2).

### Graduated from fog

- **Config & secrets inventory** → new ticket
  [T07 — Config & secrets inventory](07-config-secrets-inventory.md); unblocked
  (T02, T03, T05 all resolved). Cleared from the map's Not-yet-specified.
