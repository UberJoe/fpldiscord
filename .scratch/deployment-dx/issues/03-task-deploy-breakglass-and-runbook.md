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

**Status:** in-review

- [x] `task deploy` prints the break-glass notice, then runs `flyctl deploy`. —
      `task deploy --summary` shows the two commands in order (echo notice, then
      `flyctl deploy`); not executed (would deploy production). Seam-2 manual
      acceptance for the live run.
- [x] `docs/runbook.md` "routine deploy" == "merge to `main`", with the CI
      workflow linked. — §"Routine deploy" rewritten; links
      `.github/workflows/ci.yml` (path per spec §"CI — workflow structure";
      reconcile if ticket 04 names it differently).
- [x] The manual `fly deploy` steps survive under a clearly-labelled break-glass
      heading. — new §"Manual deploy (break-glass)" holds the `fly deploy` fenced
      block with "not normally needed" framing and a `task deploy` mention.
- [x] Rollback / rollover / cutover / legacy sections read correctly after the
      edit (no dangling references). — those sections are untouched; the new
      Routine-deploy text links `#rollback` and the break-glass text links
      `#post-deploy-verification`, both existing anchors. No other doc references
      the "Routine deploy" anchor.

## Comments

### 2026-09-08 — implemented

`task deploy` added to `Taskfile.yml` (after `db:reset`): a two-line `echo`
notice — "Automated deploy on merge to 'main' is the normal path. Use this only
for a hotfix or when CI is unavailable." — then `flyctl deploy` (no
`--remote-only`).

`docs/runbook.md` §"Routine deploy" rewritten: the routine path is now "merge
the feature branch to `main`", describing the CI check suite, the push-only
`flyctl deploy --remote-only`, the `/healthz` poll, the concurrency guard, and
no-auto-rollback; links `.github/workflows/ci.yml` and `#rollback`. The old
`fly deploy` block moved verbatim under a new §"Manual deploy (break-glass)"
with the "not normally needed" framing, a `task deploy` mention, and the
existing `strategy = "immediate"` / ~40 s downtime note. First-time cutover,
Rollback, Season/league rollover, and Retiring `legacy/` are unchanged.

`go test ./...` green (no app code touched). The live `task deploy` run and a
real CI-deploy are Seam-2 / Seam-3 acceptance (tickets 04–06).
