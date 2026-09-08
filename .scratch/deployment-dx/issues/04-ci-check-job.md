# 04 — CI check job

**What to build:** A GitHub Actions workflow whose `test` job runs the full check
suite on every pull request to `main` and every push to `main`. No deploy in this
ticket — that is 05.

- One workflow file, `.github/workflows/ci.yml`.
- Triggers: `pull_request` targeting `main`, `push` to `main`,
  `workflow_dispatch`.
- Job `test`, runs on all three triggers:
  - `actions/setup-go` with `go-version-file: go.mod`; Go build + module cache on.
  - `actions/setup-node` with `node-version-file: web/.nvmrc`; new `web/.nvmrc`
    containing `22` (matches `Dockerfile` `node:22-alpine`); npm cache on
    `web/package-lock.json`.
  - Steps: `go build ./...`; `go vet ./...`; `go test ./...`; `npm ci` then `npm
    run typecheck` in `web/`; `actionlint` over `.github/workflows/`.
  - The web bundle is not built here — the committed placeholder
    `internal/web/dist/index.html` lets the embed compile; the typecheck covers
    the frontend.
- Pin action versions (`@vX` at least; SHA-pin if that's the repo's taste later).
- Add a `task lint:ci` target to the Taskfile — run `actionlint` over
  `.github/workflows/` — so the same lint CI runs is available locally as a
  pre-flight. (`actionlint` itself is only meaningfully exercised once this
  ticket's workflow exists, which is why the target lands here, not in 01.)

**Blocked by:** 01 (adds the `task lint:ci` target to the Taskfile created there).
The workflow file itself has no dependency; if 01 is not yet done, land the
workflow and add the target in a follow-up.

**Status:** ready-for-agent

- [ ] `.github/workflows/ci.yml` exists with `pull_request`→`main`, `push`→`main`,
      and `workflow_dispatch` triggers.
- [ ] `web/.nvmrc` contains `22`.
- [ ] The `test` job runs `go build`, `go vet`, `go test`, `npm run typecheck`,
      and `actionlint`, and fails if any fails.
- [ ] Go version comes from `go.mod`; Node version from `web/.nvmrc` — no version
      literal in the workflow.
- [ ] `actionlint` passes on the workflow file itself.
- [ ] `task lint:ci` runs `actionlint` over `.github/workflows/` locally and
      passes on the new workflow.
- [ ] On a scratch PR against `main`, the `test` job runs and is green; it also
      runs on a push to `main`.

## Comments

_(none)_
