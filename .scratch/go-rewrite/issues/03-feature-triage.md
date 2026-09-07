# T03 — Feature triage

Parent: [Wayfinder map: Go rewrite](../map.md)
Type: grilling
Status: resolved
Blocked by: 01

## Question

Walk all **14 behaviours** of the current bot and lock, for each: **keep / fix / cut**,
and — for everything kept — the **target behaviour** in the Go rewrite (inputs,
output shape, edge cases). Uses T01's API findings to judge what is still fixable.

Behaviours (from `draft/cogs/fplcommands.py` + `draft/cogs/waiverstasks.py`):

| # | Command | Charting-session starting position |
|---|---|---|
| 1 | `owner` — who owns a player | keep, fix |
| 2 | `fixtures` — H2H fixtures for a GW | keep, **behind `h2h` league-mode toggle** |
| 3 | `teamlist` — a team as a text list | keep, fix |
| 4 | `waivers` — this week's waiver results | keep, fix |
| 5 | `dave` — joke reply | keep as-is |
| 6 | `team` — pitch image of a team | **cut** (removes `teamImg.py`, PIL, font, shirt scraping) |
| 7 | `scores` — live gameweek scores | keep, fix |
| 8 | `bet` — goal totals for a side-bet | **keep, reimplement** — picks in SQLite, not hardcoded IDs; needs an add/edit path via Discord command |
| 9 | `update` — force data refresh | keep (or replace with automatic refresh — decide here) |
| 10 | `overview` — GW fixture overview w/ goalscorers | keep, fix |
| 11 | `standings` — league standings (total-points + h2h variants) | keep; h2h variant behind toggle |
| 12 | `h2h` — head-to-head record between two owners | keep, **behind `h2h` league-mode toggle** |
| 13 | (13th `application_command` entry — reconcile the list; `fixtures` is registered once) | verify actual count |
| 14 | waiver-reminder daily loop (`waiverstasks.py`) | keep, fix (currently fragile `datetime` math, `utcnow`) |

Also decide:
- The `bet` add/edit UX: one command with subcommands? who can edit (allowlist)?
  where does the bettor->players mapping live?
- `classic` vs `h2h` league-mode toggle: config value name, and exactly which
  commands/variants it gates.
- Whether `update` survives as a manual command or becomes purely a background
  refresh + gameweek-change trigger.
- Any behaviour worth **adding** to the bot MVP (not the webpage — that is T04).

## Answer

Count reconciled: the code registers **12 slash commands** (not 13 — `fixtures` is
registered once at `fplcommands.py:22`) + **1 daily task** = 13 behaviours.

### Per-behaviour verdicts

| # | Behaviour | Verdict | Target behaviour in the Go rewrite |
|---|---|---|---|
| 1 | `owner` | **keep, fix** | Port as-is. Autocomplete on the player-name arg (from `elements[].web_name`). Shows current owner or "free agent"; reads live `element-status` so it self-heals across the GW20 redraft. |
| 2 | `fixtures` | **keep, h2h-gated** | Only registered when `league.scoring == "h"`. Port as-is (reads `matches` for the GW, home vs away). Dormant this season. |
| 3 | `teamlist` | **keep, fix** | Port as-is. Autocomplete on the owner-name arg (from `league_entries[].player_first_name`). GK/DEF/MID/FWD grouped list. |
| 4 | `waivers` | **keep, fix** | New `result` option: `accepted` (default) / `failed` / `all`. On `failed`/`all` show who out-bid whom. **Split across multiple messages** when rows exceed the embed limit (no truncation). Keep the smart-default `gameweek` arg. |
| 5 | `dave` | **keep as-is** | Replace the dead `str(ctx.user) == "…#0"` discriminator check with a hardcoded Discord **user-id** constant (Steve → "fuck you Steve", else "fuck you Dave"). |
| 6 | `team` | **cut** | Removes `teamImg.py`, Pillow, the bundled font, and shirt-image scraping. (Already in the map's Out of scope.) |
| 7 | `scores` | **keep, fix** | **Apply `subs[]` auto-subs** (fixes the T01 mis-scoring bug); for live/provisional scoring replicate auto-sub rules from `settings.squad`. Keep the optional `gameweek` arg. Provisional bonus recompute per T01 §4; switch to official bonus once `pl/event-status[].bonus_added` is true. |
| 8 | `bet` | **keep, reimplement** | SQLite-backed. Rules + surface below. |
| 9 | `update` | **cut** | Reads already refresh their data; automatic TTL + event-driven refresh (T05) replaces it. No admin escape hatch. |
| 10 | `overview` | **keep, fix** | Keep all three modes (Today's / Gameweek's / Live). Filter the new `defensive_contribution` family out of the goalscorer display (same as `bonus`/`bps` are already filtered). Replace `datetime.utcnow()`. |
| 11 | `standings` | **keep, fix, mode-aware** | **Classic**: one total-points table from the classic `standings[]` shape (`rank`, `entry_name`, `total`, `event_total`) — no `matches` key exists, so a distinct code path. **H2h**: keep the `normal` bool letting the user choose the total-points table or the h2h table. |
| 12 | `h2h` | **keep, h2h-gated** | Only registered when `league.scoring == "h"`. Port as-is (record between two owners from `matches`). Autocomplete on both owner-name args. Dormant this season. |
| 13 | waiver-reminder daily task | **keep, fix** | Replace the fragile `datetime`/`utcnow` math with a `time.Timer` goroutine that recomputes the next run each iteration (T02). Reads `waivers_time` for the next relevant GW (look up by event `id`, not array index — T01). Posts to the notification channel. |

**Result:** 8 always-on commands (`owner`, `teamlist`, `waivers`, `dave`, `scores`,
`bet`, `overview`, `standings`) + 2 h2h-gated (`fixtures`, `h2h`) + 1 daily task.
`team` and `update` cut.

### League mode — API-derived (supersedes the map's "config toggle")

Read `league.scoring` from `GET /api/league/{LEAGUE_ID}/details` at startup and on
each refresh: `"h"` = head-to-head, `"c"` = classic (confirmed against league 12 =
`h`, league 64 = `c`; see research §7). It gates: the `fixtures` and `h2h` commands
(not registered in classic) and the h2h variant of `standings`. Corroborating: the
`matches` key is absent entirely in classic. **No `LEAGUE_MODE` env var.** Next
season flipping the league to h2h lights these up with no code change.

### `bet` — rules

- Each bettor picks **4 players**. Scored on those players' **cumulative Premier
  League goals for the whole season** (`bootstrap-static.elements[].goals_scored`);
  FPL/draft ownership is irrelevant.
- **Every one of the 4 must have scored at least once.** A bettor with a pick still
  on zero is **provisionally out** (`🕓`), and flips back in automatically if that
  player scores. At season end, any pick on zero = **out**, whatever the total.
- Total **> 21 = bust** (`💥`).
- Winner: among bettors who are in and not bust, **exactly 21 wins outright**,
  otherwise **closest to 21 from below**. Genuine ties are shown **joint**, no
  tiebreak.
- No "close" action, no end-date. `/bet` is a live leaderboard all season; the bot
  stamps `🏆` on the leader once `game.current_event == 38 && current_event_finished`.
- No stake/prize tracking — just the tally.

### `bet` — Discord surface

`/bet [season]` is a **read, open to all users**. `/bet set` and `/bet archive` are
**admin-only**, gated by the Discord user-id allowlist.

| Command | Access | Args | Behaviour |
|---|---|---|---|
| `/bet [season]` | **everyone** | optional `season` string | Standings. No season → current season, live-computed. Season given → archive view, static. Per bettor: 4 players + each player's goals, total, status (`in` / `🕓` / `💥`), and the leader. |
| `/bet set` | admin | `bettor` (Discord user), `p1 p2 p3 p4` (player, autocomplete) | Set/replace the bettor's **current-season** picks. Bettor stored as Discord user id; display name resolved at render. |
| `/bet archive` | admin | `season` (string), `bettor_name` (free text), `entries` (text) | Add a **past-season** record. `entries` parsed as `Name:goals, Name:goals, …` (e.g. `Haaland:27, Palmer:15, Saka:12, Watkins:19`). Bettor is free text — they may have left the server. |

**Two storage shapes (feeds T05):** current season = bettor Discord user id + 4
element ids, goals computed live; past season = season label + free-text bettor name +
4 × `{player_name, final_goals}` static.

### Config keys surfaced (full inventory still fog, pending T05)

`LEAGUE_ID` (`64` this season), `SEASON` (string, e.g. `2026/27` — the API has no
season identifier so this is explicit; rollover = bump both + redeploy), Discord
user-id **allowlist** for admin commands, `dave` target user-id, notification
channel id.

### Additions to MVP

**Autocomplete only** — player-name args (`owner`, `bet set`) from
`elements[].web_name`; owner-name args (`teamlist`, `h2h`) from
`league_entries[].player_first_name`. Auto-posting live score changes to a channel is
deferred to the T04 neighbourhood.

### Not blockers, noted for build

- **GW20 mid-season redraft** on league 64 (`drafts[]`, `draft_dt` 2027-01-03):
  roster-reading commands self-heal off live `element-status`; confirm at build time
  what `transactions` `kind` codes redraft picks use so `waivers` doesn't
  misrepresent them.
- Classic `standings[]` carries `event_total` (current-GW score) for free — useful
  for a "this week" column or a future web view.

## Comments

**2026-09-06 (from [T05](05-data-model.md)):** the `scores` verdict's "Provisional
bonus recompute per T01 §4; switch to official bonus once `pl/event-status[].bonus_added`
is true" clause is **superseded** — `stats.bonus` is now trusted as-is, with no
recompute and no `pl/event-status` polling. Everything else in the `scores` row
(apply `subs[]` auto-subs, replicate from `settings.squad` for live scoring, keep the
optional `gameweek` arg) still stands.
