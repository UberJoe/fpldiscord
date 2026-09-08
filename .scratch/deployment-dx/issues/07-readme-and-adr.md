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

**Status:** in-review

- [x] `README.md` exists with overview, prerequisites, Quickstart, and the deploy
      pointer. — overview adapted from `CONTEXT.md`; prerequisites table (Go per
      `go.mod`, Node 22 per `web/.nvmrc`, go-task, air, Docker Desktop for
      `task docker`, flyctl for `task deploy`); Quickstart `task setup` →
      `npm install` in web/ → `task dev` → `task test`; a Deploy section pointing
      at merge-to-`main` and `docs/runbook.md`; links to `CONTEXT.md` and the
      runbook.
- [x] A fresh reader can get to a running `task dev` from the README alone. —
      every prerequisite has an install command/link; Quickstart calls out the
      `.env` edit (real `DISCORD_TOKEN`, `DEV_GUILD_ID` for dev) and the
      one-time `npm install` in `web/` that `task dev` needs; bare `task` for
      the full command list is mentioned.
- [x] `docs/adr/0003-deployment-automation.md` records the decision, the inherited
      constraints, and the no-rollback rationale, in the house ADR style. —
      `# 3.` title, `Date:`, `## Status` Accepted, `## Context` / `## Decision` /
      `## Consequences`, code-span file references (matching 0001/0002, no
      markdown links). Constraints (one machine, one volume,
      `strategy = "immediate"`, ~35–40 s downtime) attributed to T09; no-rollback
      rationale spelled out (halted release + manual revert vs auto-revert racing
      a single machine with forward-only migrations); `task deploy` break-glass
      and the Go/Node one-file-per-language image coupling both recorded.
- [x] README and ADR name the same triggers/jobs as `ci.yml`. — both name
      triggers `pull_request`→`main` / `push`→`main` / `workflow_dispatch`, jobs
      `test` and `deploy`, the `deploy` guard (`push` + `refs/heads/main`), the
      `deploy-production` concurrency group, and `flyctl deploy --remote-only` +
      the `/healthz` poll. Cross-checked against the committed workflow.

## Comments

### 2026-09-08 — implemented

`README.md` (new, repo root) and `docs/adr/0003-deployment-automation.md` (new)
added. No `.dockerignore` change needed — `*.md` and `.github` are already
excluded, and the ADR lives under `docs/` which `*.md` covers.

README: one-paragraph overview from `CONTEXT.md`; a prerequisites table with an
install route for each tool; a `bash` Quickstart (`task setup`, edit `.env`,
`npm install` in `web/`, `task dev`, `task test`) plus a one-line tour of the
other tasks; a Deploy section = "merge to `main`; GitHub Actions runs checks and
deploys, see `docs/runbook.md`".

ADR 0003 follows 0001/0002 prose style — Context (the manual-deploy problem +
the fixed fly constraints from T09), Decision (the `ci.yml` shape, verbatim
guard/concurrency, no rollback automation with its rationale, `task deploy`
retained, toolchain-file coupling), Consequences (including that the pipeline is
only proven on its first real run, and the token wizard as the remaining manual
step).

`go test ./...` green (no app code touched).
