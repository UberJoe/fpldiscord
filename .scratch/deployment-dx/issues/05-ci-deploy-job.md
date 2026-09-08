# 05 — CI deploy job

**What to build:** A `deploy` job in `ci.yml` that ships to fly on push to `main`
after the check job passes, then confirms the release is healthy.

- Job `deploy` in `.github/workflows/ci.yml`:
  - `needs: test`.
  - Guard: run only when `github.event_name == 'push' && github.ref ==
    'refs/heads/main'` — never on a pull request, never on `workflow_dispatch`
    from a non-`main` ref.
  - `concurrency: { group: deploy-production, cancel-in-progress: false }` — close
    merges queue, they don't race the single machine / volume.
  - `superfly/flyctl-actions/setup-flyctl` pinned to a released version; then
    `flyctl deploy --remote-only` (fly's remote builder; runner needs no Docker).
  - `FLY_API_TOKEN` from repository secrets, exported for `flyctl`.
  - Post-deploy step: poll `https://fpldiscord.fly.dev/healthz` with
    retry/backoff, ~90 s total budget, tolerate initial connection-refused
    (`strategy = "immediate"` ⇒ ~40 s downtime mid-deploy); fail the job if it
    never returns `200`.
  - No rollback step — a failed deploy or failed poll leaves the job red and the
    release halted (fly's own single-machine behaviour); rollback stays the
    manual runbook procedure.

**Blocked by:** 04 (same workflow file, `needs: test`) and 06 — the deploy job
cannot authenticate to fly or reach a green run without the `FLY_API_TOKEN`
repository secret that 06 provisions.

**Status:** in-review

- [x] `deploy` job has `needs: test` and the `push` + `refs/heads/main` guard. —
      `if: github.event_name == 'push' && github.ref == 'refs/heads/main'`.
- [x] `concurrency` group `deploy-production` with `cancel-in-progress: false`. —
      job-level `concurrency:` block.
- [x] Deploy uses `flyctl deploy --remote-only` authenticated by `FLY_API_TOKEN`.
      — job-level `env: FLY_API_TOKEN: ${{ secrets.FLY_API_TOKEN }}`; `flyctl`
      reads it automatically. `superfly/flyctl-actions/setup-flyctl@1.5` (newest
      GitHub Release of that action).
- [x] Post-deploy step polls `/healthz`, passes on `200`, fails the job otherwise
      within ~90 s. — `curl` loop, 90 s budget, 3→10 s widening interval,
      `code="000"` on connection-refused so the `strategy = "immediate"` window
      is tolerated; `exit 0` on first `200`, `::error::` + `exit 1` otherwise.
- [x] No rollback logic anywhere in the job. — three steps only: checkout,
      setup-flyctl, deploy, health check. Nothing catches failure.
- [~] A pull request against `main` runs `test` only — `deploy` does not appear.
      — enforced by the `if:` guard (`pull_request` ⇒ `event_name != 'push'`);
      confirmed by reading, proven on the introducing PR (Seam 3).
- [~] First push to `main` after merge: `deploy` runs green and the `/healthz`
      poll reaches `200`. — Seam-3 acceptance for the whole feature; needs the
      `FLY_API_TOKEN` secret from ticket 06's wizard to be set first. Not
      exercisable pre-merge. `actionlint` (which runs `shellcheck` on the
      `run:` script) is green on the workflow.

## Comments

### 2026-09-08 — implemented

`deploy` job added to `.github/workflows/ci.yml` after `test`:

- `needs: test`; `if: github.event_name == 'push' && github.ref ==
  'refs/heads/main'` — never on a PR, never on `workflow_dispatch` from another
  ref.
- `concurrency: { group: deploy-production, cancel-in-progress: false }` at job
  scope — overlapping merges queue, an in-flight deploy is never cancelled.
- `env: FLY_API_TOKEN: ${{ secrets.FLY_API_TOKEN }}` at job scope; steps:
  `actions/checkout@v4`, `superfly/flyctl-actions/setup-flyctl@1.5`,
  `flyctl deploy --remote-only` (fly's remote builder — no Docker on the
  runner; `fly.toml` at the repo root supplies app / region / volume /
  `strategy = "immediate"`).
- Health-check step: `curl -sS -o /dev/null -w '%{http_code}'` against
  `https://fpldiscord.fly.dev/healthz` in a loop with a 90 s budget and a
  3→10 s widening retry interval. `|| code="000"` absorbs the ~35-40 s
  connection-refused window mid-replace. First `200` ⇒ `exit 0`; budget
  exhausted ⇒ a `::error::` annotation and `exit 1`. No rollback step anywhere.

The top-of-file comment is rewritten to describe both jobs.

`actionlint` (with its embedded `shellcheck` pass over the `run:` block) is
green; `go test ./...` unaffected. The first real deploy run is Seam-3 and
depends on ticket 06 having populated `FLY_API_TOKEN`.
