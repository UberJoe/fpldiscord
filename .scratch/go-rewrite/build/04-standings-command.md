# 04 — `standings` command (classic)

**Spec:** [../to-spec.md](../to-spec.md)

**What to build:** `/standings` in Discord returns the classic league table — each
manager's rank, team name, season total, and latest gameweek total — read from the
classic `standings[]` shape, on a code path distinct from the (absent) h2h variant.

**Blocked by:** 02

**Status:** ready-for-agent

- [ ] `Standings()` derived view produces the classic rows (`rank`, `entry_name`,
      `total`, `event_total`) from `league/details`
- [ ] `/standings` renders them as a readable table matching the Draft site
- [ ] The classic path is separate from any h2h standings handling; the h2h variant is
      not implemented in this ticket
- [ ] A handler test with a fake snapshot covers the rendered output
