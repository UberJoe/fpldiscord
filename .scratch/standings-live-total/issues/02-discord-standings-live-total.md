# 02 — Discord /standings shows live totals and movement arrows

**What to build:** Discord's `/standings` command matches the web Standings
view during a live gameweek. It renders each manager's **live total** (frozen
total plus live gameweek score, auto-subs applied) instead of the Draft API's
raw **official total**, which today lags live scoring and only reflects
auto-subs once the gameweek has finished. The table is ordered and ranked by
live standing (joint rank, ties broken the same way as the web view) rather
than the Draft official rank, so the row order and the numbers shown in it
never disagree during a gameweek in progress. The GW column adopts the same
`+N` / `+0` / `–` convention as the web view, so a genuine mid-match zero reads
differently from a gameweek that hasn't started. A movement-arrow column
(▲/▼/–) is added, showing movement since last week's final standings — the
same measure the web view already shows.

**Blocked by:** None — can start immediately. `LiveStandings()` already
computes every value this ticket needs (live rank, live points, live GW
points, arrow); this is a Discord-side rendering change only.

**Status:** done — `main` @ (pending commit)

- [x] `/standings` Tot column shows the live total (frozen total + live
      gameweek score, auto-subs applied) instead of the Draft API's raw
      official total.
- [x] `/standings` GW column shows `+N` when the manager has current-gameweek
      points, `+0` once the gameweek has started but they don't (mirroring the
      web's `gwStarted` rule: match live, or any row nonzero), and `–` between
      gameweeks.
- [x] `/standings` row order and rank column (#) use live rank (joint
      ranking, ties broken by the Draft strict-sort field) instead of the
      Draft official rank.
- [x] `/standings` gains a movement-arrow column (▲/▼/–) showing movement
      since last week's final standings, matching the web view's meaning and
      tie handling (docs/adr/0001-standings-movement-arrow-week-over-week.md).
- [x] The h2h "not available" reply and the still-starting-up reply are
      unchanged.
- [x] ADR 0001's "Discord /standings is unaffected" consequence, and the
      `standings-live-total` spec's "Discord ... is unaffected / doesn't have
      this problem" notes, are corrected to reflect that Discord now shows
      both the live total and the arrow. (CONTEXT.md's "Movement arrow"
      glossary entry, found stale by the same review, corrected too.)
- [x] Go test coverage (`internal/bot/standings_test.go`,
      `internal/fpl/standings_test.go`) updated for the new columns/ordering;
      a new case covers a manager with an auto-sub applied rendering
      correctly, and the three GW-column states (`+N` / `+0` / `–`).

## Comments

Code review (high effort, 4 parallel finder angles + verify) surfaced and this
pass fixed: a `gwCell` formatting bug that rendered a genuinely negative
gameweek total as `+-3` instead of `-3` (now `%+d`, with a direct `TestGwCell`
covering all four cases); a same-package duplicate `SquadSettings` test fixture
in `internal/fpl/standings_test.go` (now reuses `stdSquad` from
`autosubs_test.go`); and the stale CONTEXT.md glossary line noted above. Left
open, out of this ticket's scope: `gwStarted` is computed independently in Go
(bot) and TypeScript (web) with nothing forcing them to agree; the auto-sub
test fixtures in the bot and fpl packages are near-duplicates; possible
misalignment of the ▲/▼ glyphs on some Discord clients (unverifiable without a
real device); and `LiveStandings()`'s per-manager `ManagerScore` calls redo
bootstrap lookup-map construction (pre-existing cost, now also paid by
Discord, not just the web poll loop).
