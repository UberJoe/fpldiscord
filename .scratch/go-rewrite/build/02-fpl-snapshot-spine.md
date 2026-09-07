# 02 — `fpl` snapshot spine: parse + refresher + indexes

**Spec:** [../to-spec.md](../to-spec.md)

**What to build:** The `fpl` package holds one immutable snapshot of the Draft FPL API
covering every endpoint the MVP reads, rebuilt on a schedule by a single refresher
goroutine and served last-good without locks. Every downstream reader gets a
consistent point-in-time view via `Current()`, with a `Stale` flag when the most
recent refresh failed. League mode (classic/h2h) and "is a match live right now" are
derived from the snapshot, not configured.

**Blocked by:** 01

**Status:** resolved

- [x] `Current()` returns an immutable snapshot pointer — lock-free, always last-good,
      `nil` only until snapshot #1
- [x] The snapshot parses `bootstrap-static`, `game`, `league/details`,
      `element-status`, `event/{currentGW}/live`, `entry/{entryId}/event/{gw}`, and
      `transactions` (with the `/api/draft/league/...` prefix) into typed structs
- [x] `ElementID`, `EntryID`, `LeagueEntryID` are distinct types; the two id spaces
      are never conflated; joins go through snapshot indexes
- [x] Events are looked up by their 1-indexed `id`, never by array position
- [x] Numeric-text element fields (`form`, `points_per_game`, `expected_*`) parse to
      numbers on ingest
- [x] One refresher goroutine loops ~30 s with per-endpoint TTLs — bootstrap 1 h;
      details / element-status / transactions 10 min; game 2 min; live 60 s while a
      match is live else 10 min
- [x] A change to `current_event` or `waivers_processed` force-refetches bootstrap +
      details + element-status + transactions
- [x] On fetch failure the previous snapshot is kept, the next served copy is marked
      `Stale`, with 1/2/4 s retries capped at 3 within the cycle
- [x] Requests send a real `User-Agent` + `Accept: application/json`; non-200 or
      non-JSON responses never reach `json.Unmarshal`
- [x] `MatchLive` = any fixture started and not finished-provisional in the current
      GW; `LeagueMode` is derived from `league.scoring`
- [x] Accent-stripped player-name and owner-name slices are built for autocomplete
- [x] A one-line startup summary (league name, current GW, `MatchLive`, element count)
      is logged after snapshot #1

## Answer

Built in `internal/fpl`:

- **`types.go`** — typed structs for every endpoint (`Game`, `Bootstrap`,
  `LeagueDetails`, `ElementStatus`, `Transaction`, `LiveGW`, `EntryEvent`). `num`
  parses the string-or-number-or-null numeric-text fields on ingest; `apiTime`
  tolerates null timestamps. `bootstrapWire` / `liveGWWire` handle the
  events-object and string-keyed-elements shapes.
- **`client.go`** — `Client` with one keep-alive `http.Client`, per-endpoint fetch
  methods (transactions under the `/draft/league/...` prefix), `getJSON` gating
  `json.Unmarshal` on 200 + JSON content-type, and `fetchAll` doing the full pass
  plus one `/entry/{id}/event/{gw}` per league member.
- **`fpl.go`** — id types, `LeagueMode`, `Snapshot` with unexported join indexes
  (`elementByID`, `eventByID`, `entryByEntryID`, `entryByLeagueEntryID`,
  `ownerByElement`) and accessor methods, `assemble()` (pure), `Event(id)`
  by-id lookup, `OwnerOf` joining `element_status.owner`→`entry_id`, `MatchLive()`.
- **`accent.go`** — `stripAccents` (NFD + strip Mn + ligature/ø-family fold) via
  `golang.org/x/text`, feeding `PlayerNames` / `OwnerNames`.
- **`refresher.go`** — single goroutine, 30 s tick, per-endpoint TTLs, forced
  season-refetch on `current_event` / `waivers_processed` change, 1/2/4 s retries
  keeping the previous piece and flagging `Stale`, one-line startup summary.
- **`internal/app/app.go`** — `Refresher.Bootstrap` replaces the old free
  `BuildFirst`; `Run` starts/stops the refresher goroutine.

Seam-1 fixtures live at `internal/fpl/testdata/*.json`. `go test ./... && go vet
./... && gofmt -l` clean. Note: `GOTMPDIR=./.gotmp` needed locally — a Windows
Application Control policy blocks exec from `%TEMP%`.
