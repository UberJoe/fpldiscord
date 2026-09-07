# 11 — `bet` command + `/api/bet` + web Bet view

**Spec:** [../to-spec.md](../to-spec.md)

**What to build:** The full season-long bet game. `/bet` shows the current live
leaderboard (each bettor's 4 players + season goals, running total, status marker,
leader); `/bet <season>` shows an archived season statically; `/bet set` and
`/bet archive` (admin only) enter current picks and record past seasons. The web Bet
view shows the current season's live leaderboard sorted closest-to-21, busts last.

**Blocked by:** 05, 10

**Status:** ready-for-agent

- [ ] `/bet` with no season shows the current season live-computed: per bettor 4
      players + goals, total, status (`in` / `🕓` / `💥`), and the leader; `🏆` only
      once GW38 is finished
- [ ] `/bet <season>` shows the archived record statically
- [ ] `/bet set` (admin) takes a bettor (Discord user) + 4 autocompleting player args
      and replaces that bettor's current-season picks, stored by Discord user id
- [ ] `/bet archive` (admin) takes a season, a free-text bettor name, and a
      `Name:goals` list and stores a past-season record
- [ ] Non-allowlisted callers are refused `/bet set` and `/bet archive`; the allowlist
      is read from `ADMIN_IDS`
- [ ] `web` declares a `MemberNamer` interface satisfied by `bot` (mutex-guarded
      id→name cache, lazy `GuildMember`, ~1 h TTL, `StateEnabled` false); `app` injects
      it; `web` does not import `bot`
- [ ] `GET /api/bet` returns `season` + pre-sorted `bettors[]` (`displayName`,
      `picks[]` of 4 in slot order with `elementId`/`webName`/`goals`, `total`,
      `status`, `leader`); non-bust by total desc then bust last; `displayName` via
      `MemberNamer` with raw-id fallback, raw id not shipped; the ETag folds
      `storeGen`
- [ ] The web Bet view shows the current season live, sorted closest-to-21 from below
      with busts last, and polls at live cadence
- [ ] seam-3 + seam-4 coverage
