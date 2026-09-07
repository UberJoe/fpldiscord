# T01 — Current Draft FPL API surface

Parent: [Wayfinder map: Go rewrite](../map.md)
Type: research
Status: resolved
Blocked by: —

## Question

What is the **current** shape of the Draft FPL API (`draft.premierleague.com/api/...`)
that the rewrite must build against? The Python code was written in ~2023 and has been
firefighting API drift ever since ("fixed /scores", "updated league id", "deactivated
h2h commands"). Establish ground truth before feature triage (T03) and the data model
(T05).

Answer these:

1. **Endpoint inventory.** For each endpoint the Python code uses, confirm it still
   exists, its current URL, and its current response shape (top-level keys, the fields
   the bot actually reads). The Python list, from `draft/fplutils.py`:
   - `GET /api/draft/league/{id}/transactions`
   - `GET /api/bootstrap-static` (elements, element_types, teams, events)
   - `GET /api/league/{id}/details` (league_entries, matches, standings)
   - `GET /api/league/{id}/element-status`
   - `GET /api/game` (current/next event, `waivers_processed`, `current_event_finished`)
   - `GET /api/event/{gw}/live` (elements[].stats, fixtures[].stats)
   - `GET /api/entry/{entry_id}/event/{gw}` (picks)
2. **Auth.** Which of the above (if any) now require an authenticated session? The
   Python `login()` (`users.premierleague.com/accounts/login/`) is dead code and the
   bot reads everything unauthenticated today — confirm that still holds, and document
   the login flow + which endpoints need it if it does not. (User has FPL credentials
   available if required.)
3. **Season / gameweek handling.** How is the active season identified in URLs or
   payloads now? How do you get "current gameweek", "is the current GW finished",
   "have waivers been processed", "next waivers time"? Note anything that changed.
4. **Bonus points.** `/event/{gw}/live` `fixtures[].stats` — confirm the `bps` /
   `bonus` stat structure the Python bonus calculation depends on
   (`calculate_team_bonus` in `fplutils.py`).
5. **Rate limits / etiquette.** Any observed rate limiting, required headers, or
   User-Agent expectations for polite polling from a bot.
6. **Gotchas.** Anything else a Go rewrite should know — pagination, nullable fields
   that used to be non-null, renamed keys, `element-status` vs `details` overlap.

Capture findings as a Markdown file at
`.scratch/go-rewrite/research/01-draft-fpl-api-surface.md` with example JSON snippets
(trimmed) for each endpoint.

## Answer

Probed all 7 endpoints live on 2026-09-06 (season 2026/27, GW3), league id `12`.
Full findings + trimmed real JSON per endpoint:
[research/01-draft-fpl-api-surface.md](../research/01-draft-fpl-api-surface.md).

**Headlines:**

1. **Everything the bot uses is still 100% public** — no login, cookie, or key.
   `Utils.login()` is dead code; drop it from the Go port. Scripted
   `users.premierleague.com` login is separately reported broken (bot-detection 403)
   since ~2024, so if an authed feature is ever needed, take a **pasted cookie
   string** from config — do not build around programmatic login.
2. **`bootstrap-static` reshaped**: `events` is now an object `{current, data, next}`;
   `data` is a 0-indexed array of 38 whose items carry a **1-indexed `id`** — look up
   by `id`, never index by GW number. New `element_stats` and `fixtures` keys
   (`fixtures` here is a map of *upcoming* events, **no stats**, different struct from
   `/event/{gw}/live`).
3. **New defensive stats** (`defensive_contribution`, `clearances_blocks_interceptions`,
   `recoveries`, `tackles`) in `elements[]` and live `stats`, with
   `defensive_contribution_*` scoring in `settings.scoring`.
4. **Bonus structure unchanged and confirmed**: `fixtures[].stats` = array of
   `{s,h,a}`; `s:"bps"` raw per player both sides, `s:"bonus"` empty until awarded.
   Gate provisional calc on `finished_provisional`. New public
   `/api/pl/event-status` has `bonus_added` — flip to official bonus once true.
5. **The two-id-spaces trap**: `league_entries.id` ≠ `entry_id`. `matches` /
   `standings` reference `id`; `element_status.owner` / `transactions.entry` /
   `/api/entry/{entry_id}/...` URLs use `entry_id`. Model as distinct typed ints.
6. **Other drift**: `transactions` needs the `/api/draft/league/...` prefix (un-prefixed
   404s); `entry_history` in `/entry/{id}/event/{gw}` now returns `{}` — use
   `/api/entry/{id}/history` instead; bare `/api/entry/{id}` now 403s — use `/public`;
   no rate-limit headers but a ~300 s Fastly edge cache, so set a real `User-Agent`
   and cache locally; error bodies may be HTML — gate `json.Unmarshal` on 200 +
   JSON content-type.
7. **Correctness bug to carry into T03/T05**: the Python bot ignores `subs[]`
   (auto-subs) and so mis-scores any manager who had a starter play 0 minutes. A
   correct Go client applies `subs` (finalised) or replicates auto-sub rules from
   `settings.squad` for live scoring. Note also `captains_disabled: true` — all
   `multiplier == 1`, no captain logic.

**Unblocks:** T03 (feature triage) — now on the frontier. T05 still also needs T03.

## Comments

**2026-09-06 (from [T05](05-data-model.md)):** §4's provisional-bonus recompute is
**no longer needed** — `stats.bonus` from `/event/{gw}/live` is now trusted directly,
so the Go port does not recompute bonus and does not poll `pl/event-status` for
`bonus_added`. The rest of §4 (structure of `fixtures[].stats`) still stands as
reference. The §7 auto-sub fix is unaffected and is carried into T05.
