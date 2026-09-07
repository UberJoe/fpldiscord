# 04 — `standings` command (classic)

**Spec:** [../to-spec.md](../to-spec.md)

**What to build:** `/standings` in Discord returns the classic league table — each
manager's rank, team name, season total, and latest gameweek total — read from the
classic `standings[]` shape, on a code path distinct from the (absent) h2h variant.

**Blocked by:** 02

**Status:** done

- [x] `Standings()` derived view produces the classic rows (`rank`, `entry_name`,
      `total`, `event_total`) from `league/details`
- [x] `/standings` renders them as a readable table matching the Draft site
- [x] The classic path is separate from any h2h standings handling; the h2h variant is
      not implemented in this ticket
- [x] A handler test with a fake snapshot covers the rendered output

## Answer

`internal/fpl/standings.go` — `(*Snapshot).Standings() []StandingRow`. `StandingRow`
is exactly the four ticket fields (`Rank`, `EntryName`, `Total`, `EventTotal`).
Classic-only: returns `nil` when `LeagueMode != ModeClassic` (h2h `standings[]` rows
carry the `matches_*`/`points_*` family instead, and that variant is not built this
season). The `league_entries` join for the team name is built locally, keyed on the
typed `LeagueEntryID`, so the view stays a pure function of the *exported* `Snapshot`
fields and the seam-4 bot test can construct a `Snapshot` literal and call it. Rows
are returned ascending by `Rank`.

`internal/bot/standings.go` — `/standings` command, no options. `handleStandings`
guards nil snapshot ("starting up") and h2h mode ("not available"), then renders a
monospace code-block table via `text/tabwriter` (`#`, `Team`, `Tot`, `GW`) under a
`**<league> — Standings**` header. One message — the multi-message overflow rule is a
`/waivers` concern.

Handler plumbing widened once (ticket 01 predicted it): `handlerFunc` now takes
`*cmdInput{snap, resp}`; `bot.New` takes a `SnapshotSource` (`Current() *fpl.Snapshot`,
satisfied by `*fpl.Store`); `app.New` passes the existing store. `/dave` adapted.

Tests: `internal/fpl/standings_test.go` (classic rows off the shared `buildFromStub`
fixture; rank-sort with out-of-order input; h2h → nil). `internal/bot/standings_test.go`
(classic table in rank order off a hand-built snapshot; h2h reply; nil-snapshot reply);
`commandSpecs` test updated for dave + standings. Full `go test ./...` green;
`go vet` / `gofmt` clean.

Post-review adjustments (from `/code-review`, two axes): dropped `EntryID` /
`OwnerName` from `StandingRow` (scope creep — ticket 05's `/api/standings` adds its own
richer row); `Standings()` no longer hand-rolls the snapshot's `entryByLeagueEntryID`
index (kept a local name-only map with the rationale documented); renamed
`cmdContext`→`cmdInput` and dropped its unused `opts` field (ticket 06 re-adds options
at the single dispatch site); column order is `Tot` then `GW` to match the spec's
"total, gameweek total" wording.
