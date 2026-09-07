# 11 — `bet` command + `/api/bet` + web Bet view

**Spec:** [../to-spec.md](../to-spec.md)

**What to build:** The full season-long bet game. `/bet` shows the current live
leaderboard (each bettor's 4 players + season goals, running total, status marker,
leader); `/bet <season>` shows an archived season statically; `/bet set` and
`/bet archive` (admin only) enter current picks and record past seasons. The web Bet
view shows the current season's live leaderboard sorted closest-to-21, busts last.

**Blocked by:** 05, 10

**Status:** done

- [x] `/bet` (subcommand `show`) with no season shows the current season
      live-computed: per bettor 4 players + goals, total, status (`in` / `🕓` /
      `💥`), and the leader; `🏆` only once GW38 is finished
- [x] `/bet show <season>` shows the archived record statically
- [x] `/bet set` (admin) takes a bettor (Discord user) + 4 autocompleting player args
      and replaces that bettor's current-season picks, stored by Discord user id
- [x] `/bet archive` (admin) takes a season, a free-text bettor name, and a
      `Name:goals` list and stores a past-season record
- [x] Non-allowlisted callers are refused `/bet set` and `/bet archive`; the allowlist
      is read from `ADMIN_IDS`
- [x] `web` declares a `MemberNamer` interface satisfied by `bot` (mutex-guarded
      id→name cache, lazy `GuildMember`, ~1 h TTL, `StateEnabled` false); `app` injects
      it; `web` does not import `bot`
- [x] `GET /api/bet` returns `season` + pre-sorted `bettors[]` (`displayName`,
      `picks[]` of 4 in slot order with `elementId`/`webName`/`goals`, `total`,
      `status`, `leader`); non-bust by total desc then bust last; `displayName` via
      `MemberNamer` with raw-id fallback, raw id not shipped; the ETag folds
      `storeGen`
- [x] The web Bet view shows the current season live, sorted closest-to-21 from below
      with busts last, and polls at live cadence
- [x] seam-3 + seam-4 coverage

**Notes:**

- Discord forbids mixing a bare top-level arg with subcommands, so `bet` is
  three subcommands — `show` (optional `season`), `set`, `archive`. `/bet show`
  is the leaderboard; the dispatcher also treats a bare (subcommand-less) `bet`
  as `show`.
- `/bet` is ACK'd with a deferred response (`deferredCommands`), so rendering
  bettor names (which can trigger lazy `GuildMember` REST calls) can't miss
  Discord's 3-second window.
- `web` imports `store` for the `store.BettorPicks` type that `bet.Leaderboard`
  already takes — `store` is an import-graph leaf, no cycle; `web` still does
  not import `bot`.
- `bet.SnapshotGoals` adapts a hand-built snapshot to `bet.GoalSource` over the
  exported `Bootstrap.Elements` (the snapshot's own `Element` index is only
  built by `fpl.assemble`), matching the seam-safe pattern `handleManager` uses.
- `/api/bet` resolves names concurrently and folds a `-partial` marker into the
  ETag when any id fell back to its raw form, so a cold response never shares a
  validator with the warm one that replaces it.
