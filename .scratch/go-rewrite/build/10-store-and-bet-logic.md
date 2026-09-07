# 10 — `store` (SQLite) + migrations + `internal/bet` rule logic (seam 2)

**Spec:** [../to-spec.md](../to-spec.md)

**What to build:** A pure-Go SQLite store on the fly volume that persists bet picks,
with forward-only additive migrations run at boot (fail-fast), plus the `internal/bet`
package that computes each bettor's status and the leader from stored picks and the
snapshot's season goal totals.

**Blocked by:** 01, 02

**Status:** ready-for-agent

- [ ] `internal/store` opens the DB with WAL + foreign keys + busy timeout and applies
      `go:embed`'d versioned `.sql` migrations via a small runner (highest applied
      version tracked in `schema_migrations`, newer files applied in order, each in its
      own tx)
- [ ] The runner is idempotent (a second run is a no-op) and applies a second
      migration file in order
- [ ] Schema: `schema_migrations`; `bet_pick_current(season, discord_user_id,
      slot 1–4, element_id)`; `bet_archive(season, bettor_name, slot 1–4, player_name,
      final_goals)`
- [ ] Store API: `CurrentPicks`, `SetPicks` (tx: delete then insert exactly 4),
      `ArchivedSeasons`, `Archive`, `AddArchive`; every query filters `season =
      $SEASON`
- [ ] An in-memory `storeGen` counter increments on every `SetPicks` / `AddArchive`
- [ ] The boot sequence opens the DB and runs migrations before Discord connects; a
      failed migration exits non-zero
- [ ] `internal/bet` computes `in` (all 4 scored ≥ 1) / `provisionallyOut` (total ≤ 21
      but a pick on 0) / `bust` (total > 21) and the leader (closest to 21 from below,
      exactly 21 outright, genuine ties joint, none if all bust), with the specified
      sort order
- [ ] seam-2 tests: temp-file DB round-trips, `SetPicks` full replacement, season
      isolation, runner idempotency; `bet` rule table tests
