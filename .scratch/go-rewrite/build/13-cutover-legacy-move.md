# 13 — Cutover: `legacy/` move + runbook

**Spec:** [../to-spec.md](../to-spec.md) · gate: [../parity-checklist.md](../parity-checklist.md)

**What to build:** The repo root becomes Go-only with the Python tree quarantined
under `legacy/`, and a documented one-shot runbook takes the fly app from the dormant
Python deploy to the Go binary. The `rm -rf legacy/` cleanup is explicitly deferred
until the parity checklist is ticked and is **not** part of this ticket.

**Blocked by:** 04, 06, 07, 08, 09, 11, 12

**Status:** ready-for-agent

- [ ] `draft/`, `fonts/`, `requirements.txt`, and the old Python `Dockerfile` move to
      `legacy/` (or `fonts/` is deleted outright); the root `Dockerfile` is the Go
      one; `fly.toml` is committed at repo root
- [ ] Nothing in the Go module imports `legacy/`
- [ ] A runbook documents the first-time / cutover steps in order:
      `fly volumes create` → `fly secrets set DISCORD_TOKEN … NOTIFICATION_CHANNEL_ID
      … ADMIN_IDS …` → `fly secrets unset TOKEN DEBUG_GUILDS` → `fly deploy`
- [ ] After deploy, the global command set no longer contains `team` or `update`
      (allow ~1 h propagation)
- [ ] `parity-checklist.md` is referenced as the acceptance gate for the later
      `legacy/` deletion
- [ ] The rollback procedure (`fly deploy --image @<digest>` of the previous Go
      release) is documented
