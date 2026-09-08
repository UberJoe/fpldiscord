# 3. Deployment is automated on merge to `main`

Date: 2026-09-08

## Status

Accepted

## Context

Shipping was a manual step that lived in one person's head and one runbook line:
`fly deploy` run by hand from `main` after a feature branch merged. It was easy
to forget, easy to run from the wrong branch, and ungated — a merge that broke
`go test` or `tsc` only surfaced if someone ran them locally first. There is no
staging environment, so "caught in review" was the only safety net before
production.

The fly infrastructure is fixed and not in question (recorded in
`.scratch/go-rewrite/issues/09-deployment-build-spec.md`, "T09"): one always-on
`shared-cpu-1x` machine in `lhr`, one SQLite volume mounted at `/data`, secrets
set through the fly web UI, the operational `[env]` block committed in
`fly.toml`. Because the single machine is bound to the single volume, fly cannot
roll a deploy — `fly.toml` sets `strategy = "immediate"`, so every release
replaces the machine in place with roughly 35–40 s of downtime. Migrations are
forward-only and additive, applied on boot with fail-fast.

## Decision

Merging to `main` is the deploy trigger. A single GitHub Actions workflow,
`.github/workflows/ci.yml`, does both the gating and the shipping:

- **Triggers:** `pull_request` targeting `main`, `push` to `main`, and
  `workflow_dispatch`.
- **`test` job** (all triggers): `go build ./...`, `go vet ./...`,
  `go test ./...`; `npm ci` + `npm run typecheck` in `web/`; `actionlint` over
  the workflow files. The web bundle is not built here — the committed
  `internal/web/dist/index.html` placeholder satisfies the `//go:embed` and the
  typecheck covers the frontend.
- **`deploy` job**: `needs: test`, and guarded to
  `github.event_name == 'push' && github.ref == 'refs/heads/main'` — it never
  runs on a pull request or a dispatch from another ref, so a PR branch can
  never reach production. It carries
  `concurrency: { group: deploy-production, cancel-in-progress: false }` so two
  merges landing close together queue rather than race the single machine and
  volume. It authenticates with a deploy-scoped `FLY_API_TOKEN` repository
  secret, runs `flyctl deploy --remote-only` (fly's remote builder — the runner
  needs no Docker), then polls `https://fpldiscord.fly.dev/healthz` for up to
  ~90 s, tolerating the initial connection-refused window from the immediate
  replace, and fails the job if it never sees a `200`.

**No rollback automation.** A failed `flyctl deploy` or a failed health poll
leaves the job red and the release halted on fly — the same state a hand-run
`fly deploy` failure leaves today. Rollback stays the manual, documented
`docs/runbook.md` procedure (redeploy the previous release image, or
`fly releases rollback`). Against a single machine with forward-only,
additive-only migrations, a halted release that a human then inspects and
reverts deliberately is safer than an automatic revert racing the same machine:
the old binary is always schema-compatible, but an auto-revert triggered by a
flaky health poll would add a second unattended machine-replace on top of a
release that may already be half-applied.

**`task deploy` is retained as break-glass.** It prints a notice that merge-to-
`main` is the normal path, then runs `flyctl deploy` from the maintainer's
machine — for a hotfix or when GitHub Actions is unavailable. `docs/runbook.md`
frames it the same way.

**Toolchain versions are coupled to the image, one file per language.** CI reads
Go from `go.mod` (`1.24`); the `Dockerfile` pins `golang:1.24`. CI reads Node
from `web/.nvmrc` (`22`); the `Dockerfile` pins `node:22-alpine`. The two files
per language are kept equal by hand; a Node-major or Go bump changes both
together.

## Consequences

- Shipping is no longer a step anyone can forget or run from the wrong branch. A
  merge to `main` that passes `test` is in production a few minutes later.
- A broken change is caught on the pull request, before review finishes, and the
  same suite runs again on the `push` to `main` so a bad merge resolution or a
  race between two PRs cannot ship unchecked.
- Every deploy still carries ~35–40 s of downtime (`strategy = "immediate"`);
  the health poll must tolerate it, and does. The window is unchanged from
  manual deploys.
- The pipeline cannot be proven end to end until it runs once for real: the
  introducing pull request shows `test` green (and no `deploy`, since it is
  push-only), and the first push to `main` after merge shows `deploy` green with
  the `/healthz` poll at `200`. That first run is the acceptance test.
- A one-time setup remains manual and is scripted in
  `scripts/fly-deploy-token-wizard.sh`: minting the deploy-scoped `FLY_API_TOKEN`
  and, recommended, enabling branch protection on `main` that requires the
  `test` check before merge.
- Deploy notifications (a Discord webhook on success/failure) and a staging
  environment are deliberately out of scope; they can be added later without
  revisiting this decision.
