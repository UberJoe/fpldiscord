# 13 — Cutover: `legacy/` move + runbook

**Spec:** [../to-spec.md](../to-spec.md) · gate: [../parity-checklist.md](../parity-checklist.md)

**What to build:** The repo root becomes Go-only with the Python tree quarantined
under `legacy/`, and a documented one-shot runbook takes the fly app from the dormant
Python deploy to the Go binary. The `rm -rf legacy/` cleanup is explicitly deferred
until the parity checklist is ticked and is **not** part of this ticket.

**Blocked by:** 04, 06, 07, 08, 09, 11, 12

**Status:** done

- [x] `draft/`, `fonts/`, `requirements.txt`, and the old Python `Dockerfile` move to
      `legacy/` (or `fonts/` is deleted outright); the root `Dockerfile` is the Go
      one; `fly.toml` is committed at repo root
      — `git mv draft/ requirements.txt → legacy/`; `fonts/` deleted outright (the
      `team` command is cut); the old Python `Dockerfile` (overwritten in place at
      ticket 01) restored from history as `legacy/Dockerfile`; root `Dockerfile` and
      `fly.toml` were already the Go ones (tickets 01/07)
- [x] Nothing in the Go module imports `legacy/` — `go build ./...` clean; the only
      `draft/` strings in `.go` files are Draft FPL API URL path segments
- [x] A runbook documents the first-time / cutover steps in order:
      `fly volumes create` → `fly secrets set DISCORD_TOKEN … NOTIFICATION_CHANNEL_ID
      … ADMIN_IDS …` → `fly secrets unset TOKEN DEBUG_GUILDS` → `fly deploy`
      — [`docs/runbook.md`](../../../docs/runbook.md)
- [x] After deploy, the global command set no longer contains `team` or `update`
      (allow ~1 h propagation) — documented as a post-deploy verification step in the
      runbook (live check, not exercisable here)
- [x] `parity-checklist.md` is referenced as the acceptance gate for the later
      `legacy/` deletion — "Retiring `legacy/`" section of the runbook
- [x] The rollback procedure (`fly deploy --image @<digest>` of the previous Go
      release) is documented — "Rollback" section of the runbook
