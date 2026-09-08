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

**Status:** ready-for-agent

- [ ] `deploy` job has `needs: test` and the `push` + `refs/heads/main` guard.
- [ ] `concurrency` group `deploy-production` with `cancel-in-progress: false`.
- [ ] Deploy uses `flyctl deploy --remote-only` authenticated by `FLY_API_TOKEN`.
- [ ] Post-deploy step polls `/healthz`, passes on `200`, fails the job otherwise
      within ~90 s.
- [ ] No rollback logic anywhere in the job.
- [ ] A pull request against `main` runs `test` only — `deploy` does not appear.
- [ ] First push to `main` after merge: `deploy` runs green and the `/healthz`
      poll reaches `200`. (This run is the acceptance test for the whole feature.)

## Comments

_(none)_
