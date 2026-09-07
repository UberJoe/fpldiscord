# 03 — Auto-subs + ManagerScore (seam 1)

**Spec:** [../to-spec.md](../to-spec.md)

**What to build:** Two pure functions over the snapshot that fix the long-standing
mis-scoring bug. `ApplyAutoSubs` chooses a manager's actual scoring XI — applying the
`subs[]` array verbatim once a gameweek is final, or provisionally substituting
starters who played zero minutes in finished fixtures while respecting squad position
limits. `ManagerScore` sums that XI's live points. This is the keystone seam: captured
real Draft responses drive an exhaustive table-test suite.

**Blocked by:** 02

**Status:** done

- [x] `ApplyAutoSubs(picks, subs, live, squad)` returns the scoring XI; for a finalised
      GW with a populated `subs[]` it applies that array verbatim (authoritative)
- [x] Live/provisional: a starter (pos 1–11) on 0 minutes whose fixture is
      finished-provisional is replaced by the first bench player (order 12→15) that
      keeps per-position min/max valid
- [x] The pos-12 locked backup GK only ever replaces the starting GK; a starter whose
      match is unfinished is never subbed out
- [x] `ManagerScore(entryId, gw)` sums the scoring XI's live `stats.total_points`
      as-is and includes `stats.bonus` as-is — no bonus recompute, no scoring engine
- [x] `settings.squad` is the only part of `settings` parsed, and only for auto-sub
      validity
- [x] `internal/fpl/testdata/*.json` fixtures cover: verbatim `subs[]` apply,
      provisional 0-minute sub-in, unfinished-match no-sub, the GK / backup-GK rule,
      and a case where the naive first-bench sub would violate position limits
- [x] The seam-1 table suite is green

## Answer

Two pure functions in `internal/fpl`:

- `ApplyAutoSubs(picks []Pick, subs []Sub, live LiveGW, squad SquadSettings) []ScoredPlayer`
  in `autosubs.go`. `len(subs) > 0` is the "GW finalised" signal (the Draft API
  only populates `subs[]` once final) → verbatim `ElementOut→ElementIn` swaps,
  applied regardless of minutes. Otherwise provisional: a starter on 0 minutes
  whose every linked fixture (via `live.Elements[el].explain[].fixture` →
  `LiveFixture.FinishedProvisional`) is finished-provisional is replaced by the
  first bench player, in slot order, that keeps `formationOK` (play count +
  per-position minimums + GK max). A keeper only ever swaps with a keeper, which
  is what keeps the slot-12 backup GK off an outfield blank.
- `(*Snapshot).ManagerScore(id EntryID, gw int) (ManagerScore, error)` in
  `score.go`. Resolves each pick's `Pos` from `bootstrap-static`, calls
  `ApplyAutoSubs`, sums the XI's live `total_points` verbatim (bonus already
  folded in, never re-added). Errors on unknown entry / a GW the snapshot does
  not carry / missing live feed / empty picks.

New wire fields: `Pick.Pos` (`json:"-"`, filled by `ManagerScore`),
`LiveElement.Explain []LiveExplain`. New `Pos` type + `PosGK/DEF/MID/FWD` in
`fpl.go`.

Tests: `autosubs_test.go` is the standalone table suite (verbatim-authoritative,
provisional sub-in, unfinished-match no-sub, no-fixture-link no-sub,
backup-GK-only-for-GK, first-bench-breaks-formation skip, multiple subs).
`score_test.go` drives `ManagerScore` end-to-end off `testdata/score/*.json` — a
fixture per named scenario (default provisional sub-in + unfinished no-sub,
verbatim, backup-GK, formation-break) plus bonus-as-is and error paths. Full
`go test ./...` green; `go vet` / `gofmt` clean.

Post-review adjustments (from `/code-review`): the slot-12 rule now reads
`settings.squad.position_type_locks` explicitly (`lockedPos`) rather than
relying on GK-position parity; `formationOK` reads `squad.Play` /
`squad.MaxPlayGKP` directly (dropped the zero-value fallbacks); added the three
missing JSON-driven scenario fixtures. `Pick.Pos` stays a `json:"-"` derived
field (the `ApplyAutoSubs` signature is pinned by the spec) and `len(subs) > 0`
stays the verbatim trigger (`to-spec.md` accepts it as the "finalised" proxy).
