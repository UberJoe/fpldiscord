# 02 — `fpl` snapshot spine: parse + refresher + indexes

**Spec:** [../to-spec.md](../to-spec.md)

**What to build:** The `fpl` package holds one immutable snapshot of the Draft FPL API
covering every endpoint the MVP reads, rebuilt on a schedule by a single refresher
goroutine and served last-good without locks. Every downstream reader gets a
consistent point-in-time view via `Current()`, with a `Stale` flag when the most
recent refresh failed. League mode (classic/h2h) and "is a match live right now" are
derived from the snapshot, not configured.

**Blocked by:** 01

**Status:** ready-for-agent

- [ ] `Current()` returns an immutable snapshot pointer — lock-free, always last-good,
      `nil` only until snapshot #1
- [ ] The snapshot parses `bootstrap-static`, `game`, `league/details`,
      `element-status`, `event/{currentGW}/live`, `entry/{entryId}/event/{gw}`, and
      `transactions` (with the `/api/draft/league/...` prefix) into typed structs
- [ ] `ElementID`, `EntryID`, `LeagueEntryID` are distinct types; the two id spaces
      are never conflated; joins go through snapshot indexes
- [ ] Events are looked up by their 1-indexed `id`, never by array position
- [ ] Numeric-text element fields (`form`, `points_per_game`, `expected_*`) parse to
      numbers on ingest
- [ ] One refresher goroutine loops ~30 s with per-endpoint TTLs — bootstrap 1 h;
      details / element-status / transactions 10 min; game 2 min; live 60 s while a
      match is live else 10 min
- [ ] A change to `current_event` or `waivers_processed` force-refetches bootstrap +
      details + element-status + transactions
- [ ] On fetch failure the previous snapshot is kept, the next served copy is marked
      `Stale`, with 1/2/4 s retries capped at 3 within the cycle
- [ ] Requests send a real `User-Agent` + `Accept: application/json`; non-200 or
      non-JSON responses never reach `json.Unmarshal`
- [ ] `MatchLive` = any fixture started and not finished-provisional in the current
      GW; `LeagueMode` is derived from `league.scoring`
- [ ] Accent-stripped player-name and owner-name slices are built for autocomplete
- [ ] A one-line startup summary (league name, current GW, `MatchLive`, element count)
      is logged after snapshot #1
