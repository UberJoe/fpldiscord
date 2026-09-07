# Draft Fantasy Premier League API — Current Surface (research)

Investigated live on **2026-09-06** by hitting the public endpoints directly (unauthenticated,
`User-Agent: Mozilla/5.0`). Season in the responses is **2026/27**, currently **GW3** (in progress).
Sample league id used: **12** (the bot's league — public/readable). Base URL: `https://draft.premierleague.com/api`.

All endpoints the bot uses **still exist and still return unauthenticated JSON**. Details, drift and gotchas below.

Secondary sources consulted (all older than the live probes, treat as background):
- Medium "FPL API Endpoints: A Detailed Guide" (frenzelts) — classic FPL, ~2019, still the common reference.
- oliverlooney.com "FPL APIs Explained" (~2024).
- Bram Vanherle "FPL API authentication guide" (Medium, ~2022) and `amosbastian/fpl` — login flow.
- `leej11/fpl_draft_league` (GitHub) — draft-specific, but README documents no endpoints.

---

## 0. Quick drift summary vs ~2023

1. `bootstrap-static` now also carries `element_stats` and `fixtures`; `events` is an **object**
   `{current, data, next}` (not a bare list). Draft `bootstrap-static` has **no** `phases`, `total_players`,
   `chips`, `game_settings` (those are classic-FPL only).
2. Every `elements[]` / live `stats` object gained defensive metrics: `defensive_contribution`,
   `clearances_blocks_interceptions`, `recoveries`, `tackles` (plus the older `starts`, `expected_*`).
   `settings.scoring` now has `defensive_contribution_*` scoring + threshold limits.
3. `/api/game` returns two extra keys not in old docs: `processing_status`, `trades_time_for_approval`.
4. `/api/entry/{id}` (bare) now returns **403** unauthenticated. Use `/api/entry/{id}/public` and
   `/api/entry/{id}/history` (both still public). `/api/me` → 404 on the draft host.
5. No rate-limit headers; responses sit behind Fastly/Varnish with `Edge-Control: max-age=300`.
6. The `users.premierleague.com/accounts/login/` script login is widely reported broken (bot-detection
   403) since ~2024 — but **none of the endpoints this bot uses need it**.
7. `id` vs `entry_id` in `league_entries` are different number spaces (see Gotchas). Unchanged, but the
   #1 source of join bugs for a fresh client.

---

## 1. Endpoint inventory

### `GET /api/game`
Status: **200, public.** Top-level keys:
```json
{"current_event":3,"current_event_finished":false,"next_event":4,
 "processing_status":"n","trades_time_for_approval":true,"waivers_processed":false}
```
- `current_event` int|null (null before season start → fall back to `next_event`).
- `current_event_finished` bool.
- `waivers_processed` bool — have this GW's waivers been run.
- `processing_status` string — `"n"` seen (not processing); other values likely during waiver/score runs.
- `trades_time_for_approval` bool — new; whether manager-to-manager trades are in their approval window.

### `GET /api/bootstrap-static`
Status: **200, public.** ~1 MB. Top-level keys (current):
`elements`, `element_types`, `element_stats`, `events`, `fixtures`, `settings`, `teams`.

**`elements`** — array, 653 items this season. One item (trimmed):
```json
{"id":1,"web_name":"Raya","first_name":"David","second_name":"Raya Martín",
 "element_type":1,"team":1,"code":154561,"status":"a","draft_rank":62,
 "total_points":12,"event_points":0,"points_per_game":"6.0","form":"4.0",
 "minutes":180,"goals_scored":0,"assists":0,"clean_sheets":2,"goals_conceded":0,
 "bonus":0,"bps":45,"saves":1,"penalties_saved":0,"penalties_missed":0,
 "yellow_cards":0,"red_cards":0,"own_goals":0,"starts":2,
 "expected_goals":"0.00","expected_assists":"0.00","expected_goal_involvements":"0.00",
 "expected_goals_conceded":"0.53","clearances_blocks_interceptions":2,"recoveries":19,
 "tackles":0,"defensive_contribution":0,
 "influence":"17.4","creativity":"0.0","threat":"0.0","ict_index":"1.8",
 "influence_rank":194,"creativity_rank":634,"threat_rank":629,"ict_index_rank":267,
 "form_rank":null,"points_per_game_rank":null,
 "chance_of_playing_this_round":null,"chance_of_playing_next_round":null,
 "news":"","news_added":null,"news_return":null,"news_updated":null,
 "ep_this":null,"ep_next":null,"squad_number":null,"in_dreamteam":false,"dreamteam_count":0,
 "added":"2026-07-22T14:42:46.276828Z",
 "corners_and_indirect_freekicks_order":null,"direct_freekicks_order":null,"penalties_order":null}
```
Notable: `id` is the element id used everywhere else. `status` `a`/`d`/`i`/`s`/`u` (available/doubtful/injured/suspended/unavailable). `element_type` → `element_types.id`. `team` → `teams.id`. Numeric-looking `form`/`creativity`/`ep_*` come back as **strings**; ranks and `ep_*`/`chance_of_playing_*` are frequently **null**.

**`element_types`** — full value:
```json
[{"id":1,"plural":"Goalkeepers","singular":"Goalkeeper","plural_name":"goalkeepers","singular_name":"goalkeeper"},
 {"id":2,"plural":"Defenders",...},{"id":3,"plural":"Midfielders",...},{"id":4,"plural":"Forwards",...}]
```
(Draft's `element_types` has **no** `squad_min_play`/`squad_select`/`element_count` fields — those live in `settings.squad` instead.)

**`element_stats`** — new. Array describing every stat name, e.g.:
```json
{"name":"bps","label":"Bonus Points System","abbreviation":"BPS","is_match_stat":true,"match_stat_order":13,"sort":"desc"}
{"name":"defensive_contribution","label":"Defensive Contribution","abbreviation":"DC","is_match_stat":true,"match_stat_order":26,"sort":"desc"}
```
Useful as a schema/enumeration for the Go structs; not strictly required.

**`teams`** — array, 20 items:
```json
{"code":3,"id":1,"name":"Arsenal","short_name":"ARS","pulse_id":1}
```
(Leaner than classic FPL — no `strength*`, `form`, `played`, `points` on the draft host.)

**`events`** — **object**, not array:
```json
{"current":3,"next":4,"data":[ {event objects, 38 of them} ]}
```
Each `data[]` item:
```json
{"id":1,"name":"Gameweek 1","deadline_time":"2026-08-21T17:30:00Z","finished":true,
 "average_entry_score":null,"highest_scoring_entry":null,
 "trades_time":"2026-08-19T17:30:00Z","waivers_time":"2026-08-20T17:30:00Z"}
```
`data` is a plain 0-indexed array of 38; **look up by `id`, do not index by GW number.**
(The bot's `events['data'][gw]` works only because it deliberately wants the *next* GW's `waivers_time` —
`data[gw]` where gw is 1-based lands on `id == gw+1`. Preserve that intent explicitly in Go.)

**`fixtures`** — new in bootstrap-static, and a **different shape** from the live endpoint: an object keyed by
event number as string, containing only **upcoming** events (GW1–3 finished were absent; keys started at `"4"`):
```json
{"4":[{"id":31,"event":4,"team_h":2,"team_a":18,"kickoff_time":"2026-09-12T14:00:00Z",
       "started":false,"finished":false,"finished_provisional":false,"minutes":0,
       "provisional_start_time":false,"team_h_score":null,"team_a_score":null,"code":2645226,"pulse_id":0}],
 "5":[...], "6":[...]}
```
No per-fixture `stats` here (that's live-only). Fine to ignore for the bot; use `/event/{gw}/live` for stats.

**`settings`** — object with `league`, `scoring`, `squad`, `transactions`, `ui`. Key bits:
```json
"scoring":{"long_play_limit":60,"short_play":1,"long_play":2,"bonus":1,
  "goals_scored_GKP":10,"goals_scored_DEF":6,"goals_scored_MID":5,"goals_scored_FWD":4,
  "assists":3,"clean_sheets_GKP":4,"clean_sheets_DEF":4,"clean_sheets_MID":1,"clean_sheets_FWD":0,
  "goals_conceded_GKP":-1,"goals_conceded_DEF":-1,"concede_limit":2,"saves":1,"saves_limit":3,
  "penalties_saved":5,"penalties_missed":-2,"yellow_cards":-1,"red_cards":-3,"own_goals":-2,
  "defensive_contribution_limit_DEF":10,"defensive_contribution_limit_MID":12,"defensive_contribution_limit_FWD":12,
  "defensive_contribution_DEF":2,"defensive_contribution_MID":2,"defensive_contribution_FWD":2}
"squad":{"size":15,"select_GKP":2,"select_DEF":5,"select_MID":5,"select_FWD":3,"play":11,
  "min_play_GKP":1,"max_play_GKP":1,"min_play_DEF":3,"min_play_MID":2,"min_play_FWD":1,
  "position_type_locks":{"12":"GKP"},"captains_disabled":true}
"transactions":{"new_element_locked_hours":24,"trade_veto_minimum":50,"trade_veto_hours":24,
  "waivers_before_start_min_hours":48,"waivers_before_deadline_hours":24}
```
Note `squad.captains_disabled: true` and `position_type_locks {"12":"GKP"}` — position 12 is always the
backup GK, which is exactly why the bot filters `position < 12` for the active XI.

### `GET /api/league/{id}/details`
Status: **200, public** (league 12). Top-level keys: `league`, `league_entries`, `matches`, `standings`.

`league`:
```json
{"id":12,"name":"FPL Draft 26/27","admin_entry":12,"closed":true,"scoring":"h",
 "start_event":1,"stop_event":38,"trades":"m","transaction_mode":"waivers","variety":"x",
 "draft_status":"pre","draft_dt":"2026-08-17T19:00:00Z","draft_pick_time_limit":30,
 "draft_tz_show":"Europe/London","ko_rounds":0,"max_entries":16,"min_entries":2,
 "make_code_public":false,"drafts":[...],"is_renewed":false}
```
(`drafts` array and `is_renewed` are newer additions.)

`league_entries[]`:
```json
{"id":39936,"entry_id":39880,"entry_name":"Bruno Dos Tres","short_name":"mr",
 "player_first_name":"...","player_last_name":"...","joined_time":"2026-08-04T12:03:25Z","waiver_pick":6}
```
**`id` != `entry_id`.** See Gotchas §6.

`matches[]` (152 rows for 8 entries — full double round-robin + extra rounds):
```json
{"event":1,"finished":true,"started":true,
 "league_entry_1":96301,"league_entry_1_points":28,
 "league_entry_2":12,"league_entry_2_points":35,
 "winning_league_entry":null,"winning_method":null}
```
`league_entry_1/2` reference `league_entries[].id` (**not** `entry_id`). `winning_league_entry` null on a draw
(or before finished). No null-entry "Average" rows seen in this league, but the bot still guards for them.

`standings[]`:
```json
{"rank":1,"last_rank":1,"rank_sort":1,"league_entry":93793,"total":6,
 "matches_played":38,"matches_won":2,"matches_drawn":0,"matches_lost":0,
 "points_for":116,"points_against":68}
```
`league_entry` → `league_entries[].id`. `total` is league points (h2h: 3/1/0).

### `GET /api/league/{id}/element-status`
Status: **200, public.** Single key `element_status`, one row per element (653):
```json
{"element":124,"owner":76691,"status":"o","in_accepted_trade":false}
```
- `element` → `elements[].id`.
- `owner` → `league_entries[].entry_id` (**global entry id, not `id`**), or `null` if unowned (free agent).
- `status`: `"o"` owned / `"a"` available. Only those two seen.
- `in_accepted_trade`: bool (all `false` here).

Overlap with `details`: `element-status` is the element→owner map; `details.league_entries` is the
`id`↔`entry_id`↔name map. To get "which manager owns player X" you must join
`element_status.owner == league_entries.entry_id`. (The bot does exactly this.)

### `GET /api/draft/league/{id}/transactions`
Status: **200, public.** Note the `draft/` prefix — `GET /api/league/{id}/transactions` (no prefix) is **404**.
Single key `transactions`:
```json
{"id":284930,"entry":76691,"event":1,"element_in":230,"element_out":448,
 "kind":"w","result":"a","priority":1,"index":3,"added":"2026-08-18T06:05:30.061476Z"}
```
- `entry` → `league_entries[].entry_id`.
- `element_in` / `element_out` → `elements[].id`.
- `kind`: `"w"` waiver, `"f"` free agent, (trade kinds exist too).
- `result`: `"a"` accepted, `"do"` denied-other (a higher-priority claim for the same player-out won),
  other codes for denied/pending.
- `priority` = the manager's waiver order for that claim; `index` = ordering within the batch.
- Not paginated; returns the whole league history. Sort by `event` then `index`.

### `GET /api/event/{gw}/live`
Status: **200, public.** Top-level keys: `elements` (object keyed by element id **as string**), `fixtures` (array).

`elements["1"]`:
```json
{"stats":{"minutes":0,"goals_scored":0,"assists":0,"clean_sheets":0,"goals_conceded":0,
 "own_goals":0,"penalties_saved":0,"penalties_missed":0,"yellow_cards":0,"red_cards":0,"saves":0,
 "bonus":0,"bps":0,"influence":0.0,"creativity":0.0,"threat":0.0,"ict_index":0.0,"starts":0,
 "expected_goals":0.0,"expected_assists":0.0,"expected_goal_involvements":0.0,"expected_goals_conceded":0.0,
 "clearances_blocks_interceptions":0,"recoveries":0,"tackles":0,"defensive_contribution":0,
 "total_points":0,"in_dreamteam":false},
 "explain":[ [ [ {"name":"Minutes played","points":0,"value":0,"stat":"minutes"} ], 29 ] ]}
```
- `stats.total_points` and `stats.bonus` — the bot subtracts `bonus` from `total_points` to get the
  no-bonus score, then re-adds its own provisional bonus. Still valid.
- In live, numeric stats are **numbers** (0.0), unlike bootstrap where they're strings.
- `explain` is `[[ [stat-detail objects...], fixture_id ], ...]` — one `[details, fixture_id]` pair per
  fixture the player featured in (DGW → 2 pairs). Each detail: `{name, points, value, stat}`.

`fixtures[]` (array, one per PL match in the GW):
```json
{"id":21,"event":3,"team_h":12,"team_a":14,"team_h_score":0,"team_a_score":2,
 "started":true,"finished":false,"finished_provisional":true,"minutes":0,
 "provisional_start_time":false,"kickoff_time":"2026-09-04T19:00:00Z","code":2645221,"pulse_id":0,
 "stats":[ ... see §4 ... ]}
```

### `GET /api/entry/{entry_id}/event/{gw}`
Status: **200, public.** `{entry_id}` is `league_entries[].entry_id`. Top-level keys: `picks`, `entry_history`, `subs`.
```json
{"picks":[{"element":28,"position":1,"is_captain":false,"is_vice_captain":false,"multiplier":1}, ... 15 rows],
 "entry_history":{},
 "subs":[]}
```
- `picks`: 15 rows, `position` 1–15. Positions 1–11 = starting XI, 12–15 = bench (12 = backup GK).
  `multiplier` is 1 for all (captains disabled in draft). Filter `position < 12` for the scoring XI —
  but see Gotchas §7 re: auto-subs.
- **`entry_history` came back as `{}` (empty)** this season. Historically an object with
  `points`, `total_points`, `rank`, `event_transfers`, `points_on_bench`, `event`. Treat as
  nullable/possibly-empty; get the same numbers from `/api/entry/{id}/history` instead (see §6).
- `subs`: array of applied auto-subs, `{element_in, element_out, event}` shape when populated; `[]` here.

---

## Additional public endpoints worth knowing (not currently used)

| Endpoint | Status | Use |
|---|---|---|
| `GET /api/entry/{entry_id}/public` | 200 public | `{entry:{id,name,player_first_name,player_last_name,favourite_team,started_event,overall_points,event_points,transactions_total,league_set:[...]}}` — safe replacement for the now-403 bare `/api/entry/{id}`. |
| `GET /api/entry/{entry_id}/history` | 200 public | `{history:[{event,points,total_points,rank,event_transfers,points_on_bench,...}], entry:{...}}` — per-GW history; fills the gap left by the empty `entry_history`. |
| `GET /api/pl/event-status` | 200 public | `{"status":[{"event":3,"date":"2026-09-04","bonus_added":false,"leagues_updated":true,"points":"p"}, ...],"leagues":""}` — per-day flags for the current event. `bonus_added` tells you when official bonus has landed (stop computing provisional); `points`: `"p"` provisional / `"r"` (ready/final) / `""`. |
| `GET /api/bootstrap-dynamic` | 200 public | Session/user state: `{player:{},entries:[],leagues:[],active:{...},time:<epoch>}`. All empty when unauthenticated. Only useful with a logged-in session. |
| `GET /api/entry/{id}` (bare) | **403** | `{"detail":"Authentication credentials were not provided."}` — needs auth now; use `/public`. |
| `GET /api/me` | **404** | Exists on classic FPL, not on the draft host. |

---

## 2. Auth

**Every endpoint the bot uses is readable with no session, no cookie, no API key.** Confirmed live
2026-09-06 for `game`, `bootstrap-static`, `league/12/details`, `league/12/element-status`,
`draft/league/12/transactions`, `event/3/live`, `entry/12/event/3`. Also public: `.../public`,
`.../history`, `pl/event-status`, `bootstrap-dynamic`.

Only **private leagues you're not a member of** and **personal/session endpoints** (`/api/entry/{id}` bare,
`/api/bootstrap-dynamic` with data, `/api/watchlist`, draft-room/pick submission, waiver/trade submission)
require an authenticated session. League 12 is readable anonymously, so the bot needs **no login**.
The bot's `Utils.login()` method is dead code for the read paths and can be dropped from the Go port.

### If a future feature ever needs login
Historical flow (`amosbastian/fpl`, Bram Vanherle guide):
1. `POST https://users.premierleague.com/accounts/login/` (form-encoded), body:
   `login=<email>&password=<pwd>&app=plfpl-web&redirect_uri=https://fantasy.premierleague.com/a/login`
2. On success the response sets cookies (`pl_profile`, plus a session cookie); reuse that cookie jar on
   `draft.premierleague.com/api/...` requests.

**Caveat (drift):** this scripted login is widely reported to fail with bot-detection **403 / challenge
pages** since ~2024 (DataDome/Cloudflare on `users.premierleague.com`). Current community workaround is to
log in with a real browser and copy the session cookie into the client
(`FPL_COOKIE` env-var pattern). Do **not** build the Go bot around programmatic login; if an authed feature
is ever required, take a pasted cookie string from config.

---

## 3. Season / gameweek handling

- **No season identifier anywhere.** The API only ever serves the *current* season; `bootstrap-static`,
  `game`, and `league/{id}/details` implicitly describe it. To "detect a new season", watch for
  `game.current_event` resetting to null/1 and `events.data` deadlines jumping to next year, or just
  re-fetch `bootstrap-static` and trust it. `league/{id}/details.league` has `start_event`/`stop_event`
  (1/38) and `draft_dt`, but not a year.
- **Current GW:** `GET /api/game` → `current_event` (int, or `null` before the season's first deadline →
  use `next_event`).
- **Is current GW finished:** `game.current_event_finished` (bool). For finer state also see
  `events.data[<id-1>].finished` and `pl/event-status[].points` (`"r"` when finalised).
- **Waivers processed for this GW:** `game.waivers_processed` (bool).
- **Next waivers time:** `bootstrap-static.events.data`, find the entry whose `id` == the next relevant GW,
  read `waivers_time` (ISO-8601 Z). Same array also has `trades_time` and `deadline_time`.
  The bot's rule: if `waivers_processed` is true, jump to `current_event + 1`'s `waivers_time`.
  **Index `events.data` by matching `id`, not by array position** (array is 0-based, `id` is 1-based).
- `game.trades_time_for_approval` (bool) and `game.processing_status` (`"n"` idle) give live processing state.

---

## 4. Bonus points — `/event/{gw}/live` `fixtures[].stats`

`fixtures[].stats` is an **array** of `{s, h, a}` objects. `s` is the stat name; `h`/`a` are arrays of
`{element, value}` for the home / away side. Relevant entries for bonus:

```json
{"s":"bps",
 "h":[{"element":619,"value":24},{"element":591,"value":16},{"element":315,"value":14}, ...],
 "a":[{"element":379,"value":61},{"element":367,"value":43},{"element":350,"value":35}, ...]},
{"s":"bonus",
 "h":[],
 "a":[{"element":379,"value":3},{"element":367,"value":2},{"element":350,"value":1}]}
```

- `s:"bps"` — raw Bonus Points System score per player, **both squads, all players who featured**.
  In the observed response the lists were already sorted by `value` desc, but **do not rely on ordering** —
  sort yourself.
- `s:"bonus"` — the **awarded** bonus (3/2/1). **Empty until bonus is computed/added.** Once
  `pl/event-status[].bonus_added` flips true for the day (and after the GW fully finishes) this list is
  populated and you should prefer it over your own calc.
- Also note each `elements[<id>].stats` has scalar `bps` and `bonus` totals for the player across the GW
  (summed over their fixtures) — handy for DGWs.

**Recomputing provisional bonus (what the bot does, still correct):**
1. Only consider fixtures where `finished_provisional == true` (BPS stable). `finished` means official.
2. For each such fixture, concat `bps.h + bps.a`, sort by `value` desc.
3. Award `3, 2, 1` to positions 1, 2, 3.
4. **Ties:** all players sharing a `value` get the same bonus, and each consumes a rank slot.
   e.g. two players tie on the top BPS → both get 3, next distinct BPS gets 1 (2 is skipped).
   Three tie for 2nd → all get 2, 3rd place bonus skipped. (The bot's tie loop implements this,
   though it's fiddly — a clean Go re-impl: group by value, walk groups, assign `points[rankIndex]`
   to every member, advance rankIndex by group size, stop when rankIndex >= 3.)
5. `bonus_given` beyond 3 places is 0.

Watch `fixture.minutes` / `started` too: BPS of 0 with `started:false` = match not begun (skip).
(The GW3 sample had `finished_provisional:true` with `minutes:0` — preseason/simulated data on the live
host right now; real in-play data will have non-zero `minutes`.)

---

## 5. Rate limits / headers / User-Agent

- **No published rate limit and no rate-limit headers.** Response headers on `/api/game`:
  `Server: openresty`, `Via: 1.1 google, 1.1 varnish`, `X-Served-By: cache-...`, `X-Cache: HIT`,
  `Cache-Control: max-age=0, no-cache, no-store, must-revalidate`, `Edge-Control: max-age=300`,
  `Vary: Accept-Encoding`, `Allow: GET, HEAD, OPTIONS`. No `X-RateLimit-*`, no `Retry-After`, no auth
  challenge. CORS is `same-origin` only (server-to-server is fine; browser fetch is not).
- Despite `no-cache`, there's a **Fastly/Varnish edge cache (~300 s)** in front of most endpoints, so
  polling faster than ~5 min mostly returns cached bytes anyway. `event/{gw}/live` updates more often
  during matches but is still edge-cached for a short TTL.
- **Politeness for the Go client:**
  - Set a real, identifying `User-Agent`, e.g. `fpldiscord-bot/2.0 (+https://github.com/<you>/fpldiscord)`.
    A blank/`Go-http-client` UA occasionally trips WAFs on the PL estate.
  - `Accept: application/json`, `Accept-Encoding: gzip`.
  - One shared `http.Client` with keep-alive; sequential requests, not a burst of 20 goroutines.
  - Local cache with TTLs: `bootstrap-static` 1 h, `details`/`element-status`/`transactions` 5–15 min
    (or event-driven on GW / `waivers_processed` change, like the current bot), `game` 1–5 min,
    `event/{gw}/live` 60–120 s while matches are live, else 10 min.
  - Retry on `>=500` and on transient network errors with exponential backoff (e.g. 1s, 2s, 4s, cap 3
    tries); treat `403`/`404` as terminal (don't hammer).
  - Expect occasional HTML error bodies (Fastly/So — `content-type: text/html`) instead of JSON when the
    origin is down or the path is wrong — check `Content-Type` / status before `json.Unmarshal`.

---

## 6. Gotchas for a fresh Go client

1. **`league_entries[].id` ≠ `league_entries[].entry_id`.** Two distinct id spaces:
   - `id` = league-membership id. Used by `matches.league_entry_1/2`, `matches.winning_league_entry`,
     `standings.league_entry`.
   - `entry_id` = global team id. Used by `element_status.owner`, `transactions.entry`, and the URL
     path of `/api/entry/{entry_id}/...` and `/api/entry/{entry_id}/event/{gw}`.
   They happen to be equal for some managers (e.g. the league admin, id 12/entry_id 12) which masks the
   bug in small tests. Model them as separate typed ints (`LeagueEntryID`, `EntryID`) in Go.
2. **`transactions` lives under `/api/draft/league/{id}/transactions`** — the `draft/` segment is required;
   the un-prefixed path 404s. Every *other* league endpoint is `/api/league/{id}/...` with no prefix.
3. **`bootstrap-static.events` is an object** `{current, data, next}`, and `events.data` is a 0-indexed
   array of 38 whose items carry a 1-indexed `id`. Never do `events.data[gw]` expecting GW `gw`; find by
   `id`. (The existing Python relies on the off-by-one on purpose to get *next* GW waiver time — port that
   intent as `findEvent(currentGW + 1)`, not as an array index trick.)
4. **`bootstrap-static.fixtures`** is a map `{"<event>": [fixture,...]}` containing only *upcoming* events,
   and its fixture objects have **no `stats`**. Completely different shape from `/api/event/{gw}/live`'s
   `fixtures` (a flat array *with* `stats`). Don't share a struct between them.
5. **String vs number stat fields.** In `bootstrap-static.elements`, `form`, `points_per_game`,
   `influence`, `creativity`, `threat`, `ict_index`, `expected_*`, `ep_this`, `ep_next` are JSON
   **strings** (`"4.0"`, `"0.00"`, or `null`). In `/event/{gw}/live` `stats` the same metrics are JSON
   **numbers** (`0.0`). Use `json.Number` or explicit `string`→`float` parsing, and expect `null`.
6. **Newly-nullable / empty:** `entry_history` from `/api/entry/{id}/event/{gw}` came back `{}` this
   season — don't assume `points`/`points_on_bench` are present there; use `/api/entry/{id}/history`
   (`history[]` per GW) instead. `average_entry_score` and `highest_scoring_entry` in `events.data` are
   `null` until a GW is scored. `elements[].chance_of_playing_*`, `ep_*`, `*_rank` are routinely `null`.
   `matches.winning_league_entry`/`winning_method` are `null` for draws and unplayed matches.
7. **Active XI is not just `position < 12`.** That gives the *submitted* XI. When a starter plays 0
   minutes the draft engine applies auto-subs from the bench; the applied swaps appear in
   `/api/entry/{id}/event/{gw}` `subs[]` (`{element_in, element_out, event}`) once the GW is finalised.
   The current bot ignores `subs` and can therefore mis-score managers who had an auto-sub. A correct Go
   client should apply `subs` (or, for live/provisional scoring, replicate the auto-sub rules using
   `settings.squad` min/max-per-position constraints).
8. **`element-status` vs `details` are complementary, not overlapping.** `element-status` = 653 rows of
   `element → owner(entry_id) / status`. `details.league_entries` = the id↔entry_id↔name directory.
   "Who owns player X" = join `element_status.owner == league_entries.entry_id`. `details` alone can't
   tell you rosters; `element-status` alone can't tell you names.
9. **No pagination anywhere** on the endpoints the bot uses (`transactions` returns the full league
   history in one blob; `standings`/`matches` are full-season). Classic-FPL-style `?page=` params are
   not used on these draft routes.
10. **Draft `elements`/`teams` are leaner than classic FPL.** No `now_cost`/price, no `selected_by_percent`,
    no team `strength_*`. Don't port code that expects those. `draft_rank` replaces price as the
    "player value" signal.
11. **`settings.squad.captains_disabled: true`** and every `picks[].multiplier == 1` — there is no
    captaincy in this draft league. Any captain/vice logic from a classic-FPL reference is dead weight.
12. **DGW handling:** a player's `/event/{gw}/live` `stats` are already summed across both fixtures, but
    `explain` and `fixtures[].stats` are per-fixture — iterate all fixtures when recomputing bonus, and
    a player can receive bonus in each fixture of a DGW.
13. **Error bodies may be HTML** (`content-type: text/html`, ~179 bytes) on 404 and when the origin is
    unhealthy. Gate `json.Unmarshal` on status==200 && JSON content-type.
14. `league/{id}/details.league` gained `drafts` (array) and `is_renewed` (bool); `draft_status`
    (`"pre"`/`"post"`/...) tells you whether the draft has happened.

---

## 7. Follow-up: league-type detection (added 2026-09-06)

Probed live 2026-09-06, unauthenticated (`User-Agent: Mozilla/5.0`). **League 64 (`Coq au Ian`, classic)
is publicly readable — `GET /api/league/64/details` → HTTP 200, no auth.** Compared against league 12
(h2h) from §1.

**1. League 64 `league` object (trimmed):**
```json
{"id":64,"name":"Coq au Ian","admin_entry":89,"closed":true,
 "scoring":"c","transaction_mode":"waivers","trades":"a","variety":"x","ko_rounds":0,
 "start_event":1,"stop_event":38,"draft_status":"pre",
 "draft_dt":"2027-01-03T16:00:00Z","draft_pick_time_limit":90,
 "max_entries":9,"min_entries":2,"make_code_public":false,"is_renewed":false,
 "drafts":[{"id":66,"event":1,"draft_started":true,"draft_completed":"2026-08-16T16:55:04Z","order_method":"random"},
           {"id":44546,"event":20,"draft_started":false,"draft_completed":null,"order_method":"rank"}]}
```

**2. `scoring`:** league 12 = `"h"`, league 64 = `"c"`. Classic (total-points) mode produces **`"c"`**,
head-to-head produces **`"h"`**. `scoring` **alone is a reliable h2h-vs-classic signal** — it is a
direct two-value discriminator on every league object, present for both. (Third value `"m"` exists in
older docs for a mixed/manual mode; not seen here.)

**3. `matches` and `standings` shape:**
- `matches`: h2h league 12 has top-level `matches` populated (152 rows). Classic league 64 has **no
  `matches` key at all** — top-level keys are just `league`, `league_entries`, `standings`.
- `standings[]` rows differ. h2h (12): `rank, rank_sort, last_rank, league_entry, total, matches_played,
  matches_won, matches_drawn, matches_lost, points_for, points_against` (`total` = league pts, 3/1/0).
  Classic (64): `rank, rank_sort, last_rank, league_entry, total, event_total` — **no `matches_*`, no
  `points_for`/`points_against`**; `total` = cumulative score, `event_total` = this GW's score.

**4. Corroborating signals if you don't want to trust `scoring`:** presence of top-level `matches`
(present+non-empty → h2h; absent → classic) and `standings[]` row shape (`points_for`/`matches_won`
present → h2h; `event_total` present instead → classic). **`ko_rounds` is NOT usable** — it's `0` in
both leagues; it's only non-zero for h2h leagues that add a knockout cup phase, so it can't distinguish
a plain h2h league from a classic one.

**5. Other differences between the two `league` objects:** all either league-specific config or
mode-correlated but weaker than `scoring` — `trades` (`"m"` manual vs `"a"` auto), `admin_entry`, `name`,
`id`, `max_entries`, `draft_pick_time_limit` (30 vs 90). League 64 also has a **mid-season redraft**:
`drafts[]` has two entries (completed GW1 random draft + pending GW20 rank-order draft) and top-level
`draft_dt` points at the GW20 redraft; league 12 has a single `drafts[]` entry. `variety` (`"x"`),
`transaction_mode` (`"waivers"`), `ko_rounds` (`0`), `start_event`/`stop_event` (1/38), `draft_status`
(`"pre"`), `is_renewed` (`false`) are identical.

**Verdict:** to detect h2h vs classic, read `league.scoring` — h2h = `"h"`, classic = `"c"`.
(Optional sanity check: h2h has a populated top-level `matches` array; classic omits it.)
