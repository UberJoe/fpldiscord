# Spec — Deployment & local-dev DX

**Status:** ready-for-agent

**Feature slug:** `deployment-dx`
**Grilling:** this spec is the synthesis of a `/grilling` session (three rounds).
**Related:** `docs/runbook.md`, `.scratch/go-rewrite/issues/09-deployment-build-spec.md`
(T09 §7 "local dev parity", §8 "GitHub Actions auto-deploy — out of scope for MVP").

## Problem Statement

Two friction points, both the developer's own words:

1. **Local dev takes remembered ceremony.** Running the bot locally means recalling
   a two-terminal dance — `cd web && npm run build -- --watch` in one, `go run
   ./cmd/fpldiscord` in another — and having copied `.env.example` to `.env` at some
   point in the past. There is no single entry point and nothing that tells a
   returning developer (or a fresh checkout) what the commands are. "I want
   something easy to run so I don't have to remember the commands every time."

2. **Production deploys are a manual step that only lives in one person's head and
   one runbook.** Today shipping is `fly deploy` run by hand from `main` after a
   feature branch merges (`docs/runbook.md` §"Routine deploy"). It is easy to
   forget, easy to run from the wrong branch, and there is no gate — a merge that
   breaks `go test` or `tsc` is only caught if someone runs them locally first.
   "Maybe I can have deployment when changes are merged to main."

Production on fly already works and is not in question: app `fpldiscord`, one
always-on machine in `lhr`, one volume, secrets set through the fly web UI, the
operational `[env]` block committed in `fly.toml`. This spec does not touch that
infrastructure — only how a developer drives it.

## Solution

Two independent deliverables sharing one theme: **make the common paths a single
named command, and make the deploy path automatic.**

1. **A `Taskfile.yml` (go-task) at the repo root** becomes the one place every
   routine command lives. `task setup` scaffolds `.env`. `task dev` runs the whole
   local loop — Vite watch build plus the Go binary with restart-on-save — from one
   terminal, cleaned up together on Ctrl-C. `task test` runs the same checks CI
   will. `task docker` builds and runs the real production image locally against
   `.env`. `task deploy` still exists as a break-glass manual deploy, documented as
   *not the normal path*. A bare `task` prints the catalogue, so nothing has to be
   remembered.

2. **A GitHub Actions workflow** runs `go test` + `go vet` + `tsc --noEmit` +
   `actionlint` on every pull request to `main` and on every push to `main`, and —
   only on push to `main`, only if those checks pass — deploys to fly with
   `flyctl deploy --remote-only`, then polls `/healthz` until it returns `200`.
   Merging a feature branch to `main` is the deploy trigger. A concurrency guard
   stops two merges deploying over each other. No auto-rollback — a failed release
   halts on fly as it does today, and rollback stays the manual runbook procedure.

Supporting the two: a new top-level `README.md` with a Quickstart, an edit to
`docs/runbook.md` reframing routine deploys, a new ADR recording the automation
decision, and a one-time wizard for minting the fly deploy token the workflow
needs.

## User Stories

1. As a developer on a fresh checkout, I want one command that creates my `.env`
   from the example, so that I don't have to know the file exists or what goes in
   it.
2. As a developer, I want `task setup` to be a no-op when `.env` already exists, so
   that re-running it is safe and never clobbers my real secrets.
3. As a developer, I want a single `task dev` that starts both the frontend watch
   build and the Go bot, so that I don't run two terminals or remember two
   commands.
4. As a developer, I want `task dev` to rebuild and restart the Go binary when I
   save a `.go` file, so that I see my change without manual restarts.
5. As a developer, I want a frontend edit picked up by the Vite watch to flow
   through into the running bot, so that web changes are visible in `task dev`
   without a manual rebuild despite the compile-time embed.
6. As a developer, I want Ctrl-C on `task dev` to stop both processes, so that I
   don't leave an orphaned Vite or bot process holding a port.
7. As a developer, I want `task dev` to tell me to run `task setup` when `.env` is
   missing, so that I get a clear instruction instead of a confusing boot failure.
8. As a developer, I want `task test` to run exactly the checks CI runs — Go tests,
   `go vet`, the web typecheck — so that "green locally" means "green in CI".
9. As a developer, I want a bare `task` with no arguments to list every available
   task with a one-line description, so that the command set is self-documenting.
10. As a developer, I want `task build` to produce the release binary the way the
    image does (web assets built, then `go build`), so that I can sanity-check a
    production build locally.
11. As a developer, I want `task web:build` to do a one-shot frontend build, so
    that I can refresh the embedded assets without starting the whole dev loop.
12. As a developer, I want `task db:reset` to delete my local SQLite database and
    its WAL/SHM siblings, so that I can start from a clean store without
    remembering the file names.
13. As a developer, I want `task docker` to build and run the actual production
    `Dockerfile` image locally, so that I can catch image-only breakage before it
    reaches `main`.
14. As a developer, I want `task docker` to pass my `.env` into the container, so
    that the local image run uses the same config keys as `go run`.
15. As a developer, I want the containerised run to keep its database across
    `task docker` runs, so that bet rows and a warm snapshot survive a restart the
    way they do on fly's volume.
16. As a developer, I want `task db:reset` to also drop the local container's
    data volume, so that "reset" means reset for both run modes.
17. As a developer running `task docker` on Windows, I want it to work in
    PowerShell without WSL or a bash shim, so that I don't need a Unix environment.
18. As a maintainer, I want every push of a feature branch's pull request against
    `main` to run the full check suite, so that a broken change is caught before
    review finishes, not after merge.
19. As a maintainer, I want the same check suite to run again on the push to
    `main` itself, so that a bad merge resolution or a race between two PRs can't
    ship unchecked.
20. As a maintainer, I want the workflow to lint the workflow files themselves
    with `actionlint`, so that a malformed YAML or a bad expression is caught in CI
    rather than by a silently skipped job.
21. As a maintainer, I want merging to `main` to deploy to fly automatically, so
    that shipping is not a manual step I can forget or run from the wrong branch.
22. As a maintainer, I want the deploy job to run only after the check job passes,
    so that failing tests block the release.
23. As a maintainer, I want the deploy job to run only on push to `main` and never
    on a pull request, so that PR branches are never able to touch production.
24. As a maintainer, I want a concurrency guard on the deploy job, so that two
    merges landing close together can't run overlapping deploys against the single
    machine and volume.
25. As a maintainer, I want the deploy job to poll `/healthz` after `flyctl
    deploy` returns and fail if it doesn't get a `200` within about ninety
    seconds, so that a release that boots broken shows up as a red job.
26. As a maintainer, I want no automatic rollback, so that a failed release stays
    halted for me to inspect rather than being silently reverted.
27. As an operator during an incident, I want `task deploy` to still exist as a
    one-command manual deploy from my machine, so that I can ship a hotfix when CI
    is unavailable.
28. As an operator, I want `task deploy` and the runbook to state plainly that
    manual deploy is not the normal path, so that the automated pipeline stays the
    default and manual deploy stays exceptional.
29. As a maintainer setting this up, I want a short wizard that walks me through
    minting a fly deploy-scoped token and storing it as the `FLY_API_TOKEN`
    repository secret, so that I don't hand a broad personal token to CI.
30. As a new contributor, I want a top-level `README.md` with prerequisites and a
    Quickstart (`task setup`, `task dev`, `task test`), so that I can get the
    project running without reading the runbook or the scratch specs.
31. As a maintainer, I want `docs/runbook.md` updated so that "routine deploy"
    describes "merge to `main`" and the manual `fly deploy` is reframed as
    break-glass, so that the runbook matches reality after this lands.
32. As a maintainer, I want an ADR recording the auto-deploy-on-merge decision and
    its constraints (single machine, `strategy = "immediate"`, ~40 s downtime, no
    rollback automation), so that a future change to the pipeline has the context.
33. As a developer, I want the Node version CI uses pinned to match the
    `Dockerfile` (`22`), so that "builds in CI" and "builds in the image" can't
    diverge on a Node major.
34. As a developer, I want CI's Go version taken from `go.mod`, so that the
    toolchain tracks the module and isn't a second place to bump.
35. As a maintainer, I want the introducing pull request to show the check job
    green and the first post-merge push to show the deploy job green with
    `/healthz` at `200`, so that the pipeline is proven end to end on day one.
36. As a developer, I want the new tooling files (`Taskfile.yml`, `.air.toml`,
    `.nvmrc`) kept out of the Docker build context and the release binary kept out
    of git, so that the repo and image stay clean.

## Implementation Decisions

### Task runner — `Taskfile.yml` (go-task) at repo root

- go-task chosen over `make` (not present on Windows), a `run.ps1`/`run.sh` pair
  (no self-documenting list, two copies to maintain), and root `package.json`
  scripts (adds a JS manifest to a Go-primary repo). go-task is a single binary,
  cross-platform, and `task --list` gives the command catalogue for free.
- go-task executes `cmds` through its embedded POSIX `sh` interpreter
  (`mvdan.cc/sh`), **not** the system shell, so task command bodies are written in
  portable `sh` and run identically in PowerShell, git-bash, and CI without WSL.
- `version: '3'`. `default` task runs `task --list` so a bare `task` prints the
  menu. Every task carries a `desc:` line.
- Task catalogue: `setup`, `dev`, `test`, `build`, `web:build`, `docker`,
  `db:reset`, `deploy`, `lint:ci`.
- go-task is a prerequisite the developer installs once (`winget install
  Task.Task`, `scoop install task`, `brew install go-task`, or `go install
  github.com/go-task/task/v3/cmd/task@latest`). The README lists it.

### Local dev loop — `task dev`

- `task dev` runs two long-lived processes concurrently and ties their lifetimes
  together: the Vite watch build and the Go binary under a file watcher.
- Orchestration is `npx concurrently` (added to `web/package.json`
  devDependencies), invoked with named prefixes and kill-others semantics so a
  Ctrl-C or a crash in either process tears down both. `concurrently` is chosen
  over go-task parallel `deps:` because go-task's dep parallelism does not give
  reliable process-tree teardown on Windows PowerShell, which is the primary dev
  environment. Node/`npx` is already a hard dependency.
- The Go side runs under **`air`** (`github.com/air-verse/air`), configured by a
  committed `.air.toml`:
  - build command builds `./cmd/fpldiscord`; run command runs the built binary.
  - watched extensions include `.go`; watched paths include
    `internal/web/dist/` so a frontend rebuild emitted by the Vite watch triggers
    a Go rebuild and the `//go:embed` copy refreshes (this is how story 5 is
    satisfied without a dev-server proxy).
  - excluded: `web/`, `legacy/`, `.scratch/`, `tmp/`, `bin/`, test files.
  - `air` is a developer-installed prerequisite; the README lists it alongside
    go-task.
- `task dev` first checks for `.env` and, if absent, exits with a message telling
  the developer to run `task setup` — it does not start either process.
- The Vite watch command is `npm run build -- --watch` run in `web/` (the
  existing `build` script already targets `../internal/web/dist`).

### `.env` bootstrap — `task setup`

- Copies `.env.example` to `.env` only when `.env` does not exist; prints a
  one-line "created .env — fill in your secrets" on copy and "keeping existing
  .env" otherwise. Never overwrites.
- No other setup steps (no dependency install, no DB init — `go run`/`air` and
  `npm` handle those on first use, and migrations run on boot).

### Local container run — `task docker`

- `task docker` does two steps: `docker build -t fpldiscord:local .` (repo root
  context, the existing 3-stage `Dockerfile`), then a `docker run`.
- The run command:
  - `--rm -it`
  - `--env-file .env` to supply config
  - `-e DB_PATH=/data/fpldiscord.db` to override the `.env` value
    (`./fpldiscord.db`, which is correct for `go run` but wrong inside the image)
  - `-v fpldiscord_local_data:/data` — a named volume so the SQLite DB and a warm
    snapshot persist across runs, mirroring fly's `/data` mount
  - `-p 8080:8080`
  - `fpldiscord:local`
- The image already excludes `.env` via `.dockerignore`, so nothing secret is
  baked into the layer; the token only enters at `docker run` time.

### Housekeeping tasks

- `task build` — `npm run build` in `web/`, then `go build -o ./bin/fpldiscord
  ./cmd/fpldiscord`. `/bin/` is added to `.gitignore`.
- `task web:build` — the `web/` one-shot build alone.
- `task db:reset` — deletes `fpldiscord.db`, `fpldiscord.db-wal`,
  `fpldiscord.db-shm` at repo root if present, and runs `docker volume rm -f
  fpldiscord_local_data` (ignoring "no such volume"). Prints what it removed.
- `task lint:ci` — runs `actionlint` against `.github/workflows/` for local
  pre-flight; `actionlint` is optional locally (the README notes it) and always
  present in CI.

### Break-glass deploy — `task deploy`

- `task deploy` is `flyctl deploy` (no `--remote-only` needed locally; the
  developer has Docker). It first prints a short notice that automated deploy on
  merge to `main` is the normal path and this command is for incidents / CI
  outages, then proceeds.
- `docs/runbook.md` §"Routine deploy" is rewritten: the routine path is "merge the
  feature branch to `main`; GitHub Actions runs the checks and deploys". The
  existing `fly deploy` block moves under a "Manual deploy (break-glass)" heading
  with the same "not normally needed" framing. Rollback, rollover, and
  first-time-cutover sections are unchanged.

### CI — workflow structure

- One workflow file under `.github/workflows/` (e.g. `ci.yml`).
- Triggers: `pull_request` targeting `main`, `push` to `main`, and
  `workflow_dispatch`.
- Two jobs: `test` (all triggers) and `deploy` (`needs: test`, guarded to
  `github.event_name == 'push' && github.ref == 'refs/heads/main'`).
- `deploy` carries `concurrency: { group: deploy-production, cancel-in-progress:
  false }` so overlapping merges queue rather than race.

### CI — `test` job

- `actions/setup-go` with `go-version-file: go.mod`; module and build cache
  enabled.
- `actions/setup-node` with `node-version-file: web/.nvmrc`; a new `web/.nvmrc`
  pins `22` to match the `Dockerfile`'s `node:22-alpine`. npm cache keyed on
  `web/package-lock.json`.
- Steps: `go build ./...`, `go vet ./...`, `go test ./...`; `npm ci` + `npm run
  typecheck` in `web/`; `actionlint` (via the reviewdog action or the raw binary)
  over `.github/workflows/`.
- The web build is **not** required for `go test` (the repo commits a placeholder
  `internal/web/dist/index.html`, so the embed compiles); the typecheck covers the
  frontend.

### CI — `deploy` job

- `superfly/flyctl-actions/setup-flyctl` (pinned to a released version), then
  `flyctl deploy --remote-only` — fly's remote builder builds the image, so the
  runner needs no Docker.
- Auth via `FLY_API_TOKEN` from repository secrets, exported as `FLY_API_TOKEN`
  for `flyctl`.
- Post-deploy step polls `https://fpldiscord.fly.dev/healthz` (the always-`200`
  check from T09 §4) with retry/backoff, ~90 s budget, and fails the job if it
  never sees `200`. `strategy = "immediate"` means ~40 s of downtime mid-deploy,
  so the poll must tolerate initial connection-refused.
- No rollback step. A failed `flyctl deploy` or a failed health poll leaves the
  job red and the release halted (fly's own behaviour with a single machine).

### CI — auth / token provisioning

- Delivered as a wizard (see the tickets): mint a **deploy-scoped** token, not a
  personal org token — `fly tokens create deploy -x 8760h` — and store it as the
  `FLY_API_TOKEN` repository secret (Settings → Secrets and variables → Actions).
- The wizard also notes enabling branch protection on `main` with the `test` job
  as a required status check, so a red check blocks merge. That toggle is a
  GitHub setting the maintainer applies; it is documented, not automated.

### Docs — README, runbook, ADR

- New top-level `README.md`: one-paragraph "what is this" adapted from
  `CONTEXT.md`; a Prerequisites list (Go via `go.mod`, Node 22, go-task, `air`,
  Docker for `task docker`, `flyctl` for `task deploy`); a Quickstart (`task
  setup` → `task dev` → `task test`); a "Deploy" line pointing at "merge to
  `main`" and `docs/runbook.md`; links to `CONTEXT.md` and the runbook.
- `docs/runbook.md` edit as described under "Break-glass deploy".
- New `docs/adr/0003-deployment-automation.md` (next number after 0001/0002),
  recording: deploy-on-merge-to-`main` via GitHub Actions; the `test`-gates-
  `deploy` shape; the health-poll acceptance check; explicitly no rollback
  automation and why; the single-machine / single-volume / `strategy =
  "immediate"` / ~40 s downtime constraints it inherits from T09; `task deploy`
  retained as break-glass. Follows the ADR style of 0001/0002.

### Repo housekeeping

- `.gitignore`: add `/bin/`. `tmp/` (air's scratch dir) is already ignored.
- `.dockerignore`: add `Taskfile.yml`, `.air.toml`, `.nvmrc` (`.github`, `*.md`,
  `.env*` are already excluded) so the tooling files don't enter the build
  context.
- `.env.example` is unchanged — its key list and comments already match what
  `task setup` copies and what `task docker` passes through.

## Testing Decisions

This feature changes no application code (no Go, no TypeScript; `config.Load`
already tolerates a missing `.env`). There is therefore **no new automated test
seam and no new fake**. Verification rests on three seams:

- **Seam 1 — the existing check suite, reused: `go build ./... && go vet ./... &&
  go test ./...` plus `web` `tsc --noEmit`.** This is the only real test seam in
  play. The feature's job is to invoke it consistently from three call sites —
  `task test` locally, the CI `test` job on pull requests, and the same job
  gating `deploy` on push to `main`. A good outcome is that all three run the
  identical command set and a failure in any of them is visible before code
  ships. Nothing about the suite's contents changes.
- **Seam 2 — the Taskfile targets: one-time manual acceptance, recorded as
  acceptance criteria on the tickets, not automated.** A `Taskfile.yml` is not
  unit-testable and stubbing `docker` / `flyctl` to assert a wrapper forwards the
  right flags is churn with no payoff. Each target (`setup` on a checkout with and
  without `.env`; `dev` starting and Ctrl-C stopping both processes; `dev` on a
  frontend edit; `build`; `docker` including volume persistence across two runs;
  `db:reset` clearing both the files and the volume; `deploy` printing its
  break-glass notice) is exercised once by hand and its effect eyeballed. The
  ticket for each carries the checklist.
- **Seam 3 — the workflow's own first run is the integration test.** Acceptance is
  concrete and external: the pull request that introduces `.github/workflows/`
  shows the `test` job green (and, because `deploy` is push-only, no deploy runs
  on the PR); the first push to `main` after merge shows the `deploy` job green
  with the `/healthz` poll reaching `200`. There is no cheaper honest proof that
  CI-deploy works than watching it deploy once. `actionlint` in the `test` job is
  the static guard that the workflow YAML and its expressions are well-formed
  before that first run.

No prior art in the repo for CI (there is no `.github/` today). Prior art for the
runbook/ADR prose is `docs/runbook.md` and `docs/adr/0001`/`0002`.

## Out of Scope

- **A Vite dev server + Go reverse-proxy for HMR.** The frontend stays
  embed-only; `air` restart-on-save covers web edits (T09 §7, reaffirmed in
  grilling).
- **`docker compose`.** `task docker` is a plain `docker build` + `docker run`;
  no compose file.
- **Automatic rollback on a failed deploy.** A failed release halts on fly;
  rollback stays the manual `docs/runbook.md` procedure.
- **Deploy notifications** (Discord webhook on success/failure). Deferred — noted
  in grilling as a later nicety, not built now.
- **Any change to fly infrastructure**: VM size, region, the volume, the
  always-on / no-autostop settings, `strategy = "immediate"`, the ~40 s deploy
  downtime. All unchanged.
- **Fly secret management changes.** Production secrets are still set through the
  fly web UI / `fly secrets set`; the workflow never reads or writes them.
- **A staging / preview environment on fly.** Single production app only.
- **Enforcing branch protection.** The required-status-check toggle on `main` is
  documented in the token wizard as a recommended manual step, not applied by
  this work.
- **Retiring `legacy/`.** Still gated on the parity checklist per
  `docs/runbook.md`; untouched here.
- **Migrating the Node major** in the `Dockerfile`. CI is pinned to match the
  image at `22`; bumping both is a separate change.

## Further Notes

- **The `.env` holds a live test-bot `DISCORD_TOKEN`.** It is gitignored, so it
  does not leak, but both `task dev` and `task docker` start a real Discord
  gateway session against whatever `DISCORD_TOKEN` `.env` contains. Use a test
  bot, and set `DEV_GUILD_ID` for instant guild-scoped command registration in
  dev. `task docker` passing `--env-file .env` means the container runs that same
  bot session.
- **Production parity of toolchains after this lands:** Go — CI reads `go.mod`
  (`1.24`), the image pins `golang:1.24`. Node — CI reads `web/.nvmrc` (`22`), the
  image pins `node:22-alpine`. Two files per language, deliberately kept equal;
  the ADR notes the coupling.
- **The introducing PR cannot exercise the `deploy` job** (push-only). Sequence:
  open the PR → `test` job proves the checks → merge → first push to `main` proves
  `deploy` + the health poll. Ticket 05's acceptance is that first post-merge run.
- **Fly deploy token wizard, in brief:** `fly tokens create deploy -x 8760h`
  (deploy-scoped, one-year expiry) → copy the `FlyV1 …` string → repo Settings →
  Secrets and variables → Actions → New repository secret `FLY_API_TOKEN`. Full
  steps in ticket 06.
- **go-task shell portability:** because go-task runs `cmds` through `mvdan.cc/sh`
  rather than the OS shell, the Taskfile can use `sh`-style conditionals and glob
  removal for `task setup` / `task db:reset` and they work in PowerShell without a
  bash install. Verify on the primary Windows box as part of ticket 01.
- **CI minutes:** `test` on every PR push plus every `main` push; feature-branch
  pushes without an open PR run nothing. Deliberate — feature work always goes
  through a PR (grilling Q6).
