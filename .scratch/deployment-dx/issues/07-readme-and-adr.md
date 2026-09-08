# 07 — README + ADR 0003

**What to build:** The docs that make the new workflow discoverable: a top-level
`README.md` (there is none today) and an ADR recording the auto-deploy decision.

- `README.md` at repo root:
  - One-paragraph "what is this", adapted from `CONTEXT.md` (Discord bot + web
    view for a private FPL Draft league).
  - **Prerequisites:** Go (version per `go.mod`), Node 22, go-task, `air`, Docker
    Desktop (for `task docker`), `flyctl` (for `task deploy` only).
  - **Quickstart:** `task setup` → `task dev` → `task test`. Mention bare `task`
    lists everything.
  - **Deploy:** one line — "merge to `main`; GitHub Actions runs checks and
    deploys. See `docs/runbook.md`."
  - Links to `CONTEXT.md` and `docs/runbook.md`.
- `docs/adr/0003-deployment-automation.md`, following the style of
  `docs/adr/0001` / `0002`:
  - Decision: deploy on merge to `main` via GitHub Actions; a `test` job
    (`go build`/`vet`/`test`, web `tsc`, `actionlint`) gates a push-only `deploy`
    job (`flyctl deploy --remote-only`) that health-polls `/healthz`.
  - Constraints inherited from
    `.scratch/go-rewrite/issues/09-deployment-build-spec.md`: one always-on
    machine, one volume, `strategy = "immediate"`, ~40 s downtime per deploy.
  - Explicitly no rollback automation — rationale: a halted bad release plus a
    manual, documented rollback is safer than auto-revert against a single
    machine with forward-only migrations.
  - `task deploy` retained as break-glass.
  - Toolchain coupling noted: CI Node (`web/.nvmrc`) == `Dockerfile` Node; CI Go
    (`go.mod`) == `Dockerfile` Go.
- No `.dockerignore` change needed for these — `*.md` and `.github` are already
  excluded.

**Blocked by:** 01 (README documents its tasks). Deploy sections should match
tickets 04–05 wording.

**Status:** ready-for-agent

- [ ] `README.md` exists with overview, prerequisites, Quickstart, and the deploy
      pointer.
- [ ] A fresh reader can get to a running `task dev` from the README alone.
- [ ] `docs/adr/0003-deployment-automation.md` records the decision, the inherited
      constraints, and the no-rollback rationale, in the house ADR style.
- [ ] README and ADR name the same triggers/jobs as `ci.yml`.

## Comments

_(none)_
