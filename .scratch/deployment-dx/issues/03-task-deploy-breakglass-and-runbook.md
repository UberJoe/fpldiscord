# 03 — `task deploy` break-glass + runbook rewrite

**What to build:** A manual deploy target kept as an explicit break-glass path,
and a `docs/runbook.md` that describes the automated pipeline as the normal way to
ship.

- `task deploy` — print a short notice first ("Automated deploy on merge to `main`
  is the normal path. Use this only for a hotfix or when CI is unavailable."),
  then run `flyctl deploy` (no `--remote-only`; the developer has Docker).
- `docs/runbook.md` §"Routine deploy" — rewrite so the routine path is: merge the
  feature branch to `main`; GitHub Actions runs the checks and, on green, deploys
  and health-checks. Link to the CI workflow.
- Move the existing `fly deploy` block under a new "Manual deploy (break-glass)"
  heading with the same "not normally needed" framing; mention `task deploy` as
  the shortcut.
- Leave §"First-time cutover", §"Rollback", §"Season / league rollover", and
  §"Retiring `legacy/`" unchanged in substance (fix cross-references only if the
  section order shifts).

**Blocked by:** 01 (adds the target to the Taskfile). Runbook wording for the
automated path should match tickets 04–05; safe to draft in parallel and reconcile
terms before merge.

**Status:** ready-for-agent

- [ ] `task deploy` prints the break-glass notice, then runs `flyctl deploy`.
- [ ] `docs/runbook.md` "routine deploy" == "merge to `main`", with the CI
      workflow linked.
- [ ] The manual `fly deploy` steps survive under a clearly-labelled break-glass
      heading.
- [ ] Rollback / rollover / cutover / legacy sections read correctly after the
      edit (no dangling references).

## Comments

_(none)_
