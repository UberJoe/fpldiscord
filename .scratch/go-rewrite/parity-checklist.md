# Parity checklist — Python → Go cutover

Asset of [T10 — Parity & cutover plan](issues/10-parity-cutover-plan.md).

Gate for deleting `legacy/`. Standard is **behavioural equivalence**: same inputs →
correct information in the T03 / T06-documented shape. Listed divergences are expected
improvements, not failures. Tick each row against **real data** in the live guild —
needs roughly one gameweek with live matches + one processed waiver run.

Sign-off: Joe.

## Always-on commands

| # | Behaviour | Passes when… | Expected divergence from Python |
|---|---|---|---|
| 1 | `owner <player>` | returns the current owner (or "free agent") for an autocompleted player; correct immediately after the GW20 redraft (reads live `element-status`) | autocomplete added |
| 2 | `teamlist <owner>` | returns the owner's squad grouped GK/DEF/MID/FWD, autocompleted owner arg | autocomplete added |
| 3 | `waivers [gw] [result]` | `accepted` (default) matches the processed waiver results for the GW; `failed` / `all` show who out-bid whom; output splits across multiple messages when long, no truncation | `result` flag + multi-message overflow are new |
| 4 | `dave` | replies with the joke text; Steve → "fuck you Steve", everyone else → "fuck you Dave" (hardcoded user-id, not discriminator) | dead discriminator check replaced |
| 5 | `scores [gw]` | live/provisional GW scores per manager **with `subs[]` auto-subs applied**; totals reconcile with the Draft site once the GW is final | auto-sub fix (was mis-scoring); bonus taken from `stats.bonus` as-is, no recompute |
| 6 | `bet [season]` | no season → live leaderboard: 8 bettors, 4 players each with goals, total, status (`in` / `🕓` provisionally-out / `💥` bust), leader = closest to 21 from below; `/bet set` (admin) round-trips a bettor's picks; `/bet archive` (admin) stores a past season | fully reimplemented — SQLite-backed, new rules, autocomplete on player args |
| 7 | `overview` | all three modes (Today's / Gameweek's / Live) render fixtures + goalscorers; `defensive_contribution` family filtered out of the goalscorer display | new stat family filtered; `utcnow()` replaced |
| 8 | `standings` | classic total-points table from the classic `standings[]` shape (`rank`, `entry_name`, `total`, `event_total`); distinct code path from the (absent) h2h variant | dedicated classic path |

## Task

| # | Behaviour | Passes when… | Expected divergence |
|---|---|---|---|
| 9 | waiver-reminder daily loop | fires once into the notification channel at the scheduled time (05:00 UTC wake / same-day / T−1h) ahead of the next relevant GW's `waivers_time`, looked up by event `id` | fragile `datetime`/`utcnow` math replaced by a self-recomputing `time.Timer` goroutine |

## Compile / register-gate only (not exercised — dormant in classic mode)

| # | Behaviour | Passes when… |
|---|---|---|
| 10 | `fixtures` | compiles; **not registered** while `league.scoring == "c"`; would read `matches` for the GW if h2h |
| 11 | `h2h` | compiles; **not registered** while `league.scoring == "c"`; would read the record between two owners from `matches` if h2h |

## Not part of this gate

- The 3 web views (Standings + live scores, Waiver History, Bet) — no Python
  equivalent; "done" is a T04 judgement, tracked separately.
- `team`, `update` — cut (T03). Confirm they are **absent** from the command list
  after the global `ApplicationCommandBulkOverwrite` (allow ~1 h propagation).
