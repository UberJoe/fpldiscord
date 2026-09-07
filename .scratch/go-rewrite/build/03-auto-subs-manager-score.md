# 03 — Auto-subs + ManagerScore (seam 1)

**Spec:** [../to-spec.md](../to-spec.md)

**What to build:** Two pure functions over the snapshot that fix the long-standing
mis-scoring bug. `ApplyAutoSubs` chooses a manager's actual scoring XI — applying the
`subs[]` array verbatim once a gameweek is final, or provisionally substituting
starters who played zero minutes in finished fixtures while respecting squad position
limits. `ManagerScore` sums that XI's live points. This is the keystone seam: captured
real Draft responses drive an exhaustive table-test suite.

**Blocked by:** 02

**Status:** ready-for-agent

- [ ] `ApplyAutoSubs(picks, subs, live, squad)` returns the scoring XI; for a finalised
      GW with a populated `subs[]` it applies that array verbatim (authoritative)
- [ ] Live/provisional: a starter (pos 1–11) on 0 minutes whose fixture is
      finished-provisional is replaced by the first bench player (order 12→15) that
      keeps per-position min/max valid
- [ ] The pos-12 locked backup GK only ever replaces the starting GK; a starter whose
      match is unfinished is never subbed out
- [ ] `ManagerScore(entryId, gw)` sums the scoring XI's live `stats.total_points`
      as-is and includes `stats.bonus` as-is — no bonus recompute, no scoring engine
- [ ] `settings.squad` is the only part of `settings` parsed, and only for auto-sub
      validity
- [ ] `internal/fpl/testdata/*.json` fixtures cover: verbatim `subs[]` apply,
      provisional 0-minute sub-in, unfinished-match no-sub, the GK / backup-GK rule,
      and a case where the naive first-bench sub would violate position limits
- [ ] The seam-1 table suite is green
