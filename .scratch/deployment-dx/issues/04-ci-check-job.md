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

**Status:** in-review

- [x] `.github/workflows/ci.yml` exists with `pull_request`→`main`, `push`→`main`,
      and `workflow_dispatch` triggers.
- [x] `web/.nvmrc` contains `22`.
- [x] The `test` job runs `go build`, `go vet`, `go test`, `npm run typecheck`,
      and `actionlint`, and fails if any fails. — `fail_level: error` on the
      reviewdog actionlint step; the `run:` steps fail the job by exit code.
- [x] Go version comes from `go.mod`; Node version from `web/.nvmrc` — no version
      literal in the workflow. — `go-version-file: go.mod`,
      `node-version-file: web/.nvmrc`; grep for a digit-only version in the file
      finds none.
- [x] `actionlint` passes on the workflow file itself. — `actionlint v1.7.12`
      (installed via `go install`) exits 0 on `.github/workflows/ci.yml`. It
      caught `fail_on_error` as deprecated during implementation; switched to
      `fail_level: error`.
- [x] `task lint:ci` runs `actionlint` over `.github/workflows/` locally and
      passes on the new workflow. — `task lint:ci` exits 0 with actionlint on
      PATH; skips cleanly (exit 0, install hint printed) when it is absent.
- [~] On a scratch PR against `main`, the `test` job runs and is green; it also
      runs on a push to `main`. — Seam-3 acceptance: the workflow's first real
      run. Locally the full command set (`go build`/`go vet`/`go test` +
      `web` `npm run typecheck` + `actionlint`) is green.

## Comments

### 2026-09-08 — implemented

`.github/workflows/ci.yml` added: `name: CI`, triggers `pull_request` →`main` /
`push` →`main` / `workflow_dispatch`, `permissions: contents: read`, one `test`
job on `ubuntu-latest`. Steps: `actions/checkout@v4`; `actions/setup-go@v5`
(`go-version-file: go.mod`, `cache: true`); `actions/setup-node@v4`
(`node-version-file: web/.nvmrc`, `cache: npm`,
`cache-dependency-path: web/package-lock.json`); `go build ./...`;
`go vet ./...`; `go test ./...`; `npm ci` + `npm run typecheck` in `web/`;
`reviewdog/action-actionlint@v1` (`fail_level: error`). No web bundle build —
the committed `internal/web/dist/index.html` placeholder satisfies the embed.

`web/.nvmrc` created with `22` (matches `Dockerfile` `node:22-alpine`).

`task lint:ci` added to `Taskfile.yml` (after `deploy`): runs bare `actionlint`
(auto-discovers `.github/workflows/`); if `actionlint` is not on PATH it prints
an install hint and exits 0 so a local check run is never blocked.

`.dockerignore`: the `.nvmrc` entry (added in ticket 01 at repo-root scope) is
now `web/.nvmrc` — the file lives under `web/`, which the `Dockerfile` copies in
stage 1, so the root-scoped pattern would not have excluded it.

Action versions pinned at `@vX` (`@v4`/`@v5`/`@v1`), per the ticket; SHA-pinning
is left as a later repo-wide choice.
