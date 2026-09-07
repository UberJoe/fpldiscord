# T10 — Parity & cutover plan

Parent: [Wayfinder map: Go rewrite](../map.md)
Type: grilling
Status: resolved
Blocked by: 03, 09

## Question

Define how the Python bot is retired in favour of the Go binary, and the checklist
that gates deleting `legacy/`. Graduated from the "Parity / cutover plan" fog once
[T09](09-deployment-build-spec.md) fixed the deploy story — one `fpldiscord` fly app,
one always-on machine bound to one volume, `strategy = "immediate"` — which means the
Python → Go switch is a **hard swap**, not parallel running on the same app.

Inputs are locked:

- **Deploy** (T09): single machine, hard-swap deploys (~40 s downtime), fly secrets
  renamed from the Python names, `fly secrets unset TOKEN DEBUG_GUILDS …` on cutover,
  volume `fpldiscord_data` created first.
- **Feature set** (T03): 8 always-on commands + 2 h2h-gated + the daily
  waiver-reminder task define "parity". `team` and `update` are cut — not parity
  gaps. `bet` is *reimplemented* (new rules, new storage) — parity means "the new
  `bet` works", not "matches the old `bet`".
- **Repo move** (destination): Python code moves to `legacy/`; deleted once the Go
  version reaches parity. New Go module already lives in the repo (T08).

Decide:

- **Cutover mechanism** — is there any parallel-run at all (e.g. Go bot on a
  throwaway second fly app / a dev guild via `DEV_GUILD_ID`, pointed at the real
  league read-only, while Python still serves the live guild), or is it a single
  cutover deploy with a rollback plan? How long is the shakedown, and who signs off?
- **Parity checklist** — the concrete per-command acceptance list (the 10 commands +
  the reminder task + the 3 web views) that must pass before Python is gone. What
  counts as "passes" for each — exact output match vs. behavioural equivalence, given
  the known intentional divergences (auto-sub fix, `waivers` flag, `standings`
  classic path, reimplemented `bet`, no `scores` bonus recompute).
- **`bet` data carry-over** — the old Python `bet` had its own storage/format. Does
  any existing bet state need migrating into `bet_pick_current` / `bet_archive`, or
  does cutover coincide with a fresh `/bet` round and old data is abandoned?
- **`legacy/` move** — one commit moving `draft/` → `legacy/` (plus `fonts/`,
  `requirements.txt`, the Python `Dockerfile`), or delete outright and rely on git
  history? When does the move happen relative to the cutover deploy — before (so the
  cutover commit is Go-only) or after parity is signed off?
- **Rollback window** — how long the Python image stays deployable (`fly deploy
  --image` of the last Python release) before `legacy/` deletion makes rollback a
  code-restore job. What observability confirms the Go bot is healthy enough to burn
  that bridge (healthz, command error rate, the reminder task firing once).
- **Discord command cleanup** — Python registered its command set; the Go bot does
  `ApplicationCommandBulkOverwrite` on READY (T02), which should replace it wholesale.
  Confirm no stale commands linger (global vs guild scope), and whether the cut
  commands (`team`, `update`) need an explicit dev note.

## Answer

The Python app on fly is **dormant** (not in real use), which collapses most of the
risk: cutover is a blind hard swap, no shakedown, no "keep Python warm" plan.

### 1. Cutover mechanism — blind hard swap

No parallel run, no dev-guild smoke pass, no second fly app. A single `fly deploy` of
the Go image to the existing `fpldiscord` app replaces the dormant Python bot. ~40 s
downtime is irrelevant — nothing is using it. First-time-only steps from T09 run once,
in order, immediately before the cutover deploy:

```bash
fly volumes create fpldiscord_data --size 1 --region lhr
fly secrets set DISCORD_TOKEN=… NOTIFICATION_CHANNEL_ID=… ADMIN_IDS=…
fly secrets unset TOKEN DEBUG_GUILDS          # Python-era names
fly deploy
```

Sign-off is Joe's, against the parity checklist (§2).

### 2. Parity checklist — behavioural equivalence

Match standard is **behavioural equivalence**, not exact output: each kept behaviour,
given the same inputs, returns correct information in its T03/T06-documented shape.
The known divergences are *expected improvements*, not failures — auto-sub correctness
(`scores`, web `/manager`), the `waivers` `result` flag + multi-message overflow, the
`standings` classic code path, reimplemented `bet`, and no bonus recompute in
`scores`.

Scope of the gate: the **8 always-on commands** (`owner`, `teamlist`, `waivers`,
`dave`, `scores`, `bet`, `overview`, `standings`) + the **waiver-reminder task**.
Excluded: `fixtures` and `h2h` (dormant in classic mode — untestable without an h2h
league; they must *compile and register-gate correctly*, nothing more). The **3 web
views are a separate gate** — no Python equivalent exists, so "web MVP done" is a T04
judgement and is **not** a `legacy/`-deletion gate.

Recorded as [`parity-checklist.md`](../parity-checklist.md) in the effort dir — one
row per behaviour with a "passes when…" line. Ticking it requires real data: roughly
**one gameweek with live matches + one waiver run** to exercise `scores` / auto-subs /
`waivers` / `standings` / the reminder. No calendar deadline.

### 3. `bet` data carry-over — none; fresh round

The Python `bet` picks are a **hardcoded dict literal** (`fplcommands.py`, 8 bettors ×
4 element ids) and were never stored; the ids are last season's and are stale for
2026/27. So there is nothing to migrate. At cutover the admin runs `/bet set` once per
bettor with this season's 4 picks — `bet_pick_current` starts empty and fills via the
command. `bet_archive` also starts empty (no machine-readable past-season data
exists). The old dict survives in git history as the roster of who plays.

### 4. `legacy/` move — quarantine at cutover, delete after sign-off

Two commits:

1. **Cutover commit/PR** — add the Go tree at repo root; move `draft/` → `legacy/`
   plus `fonts/`, `requirements.txt`, and the old Python `Dockerfile` → `legacy/`; the
   new root `Dockerfile` is the Go one (T09); commit `fly.toml`. Root is Go-only,
   Python quarantined and still diff-able.
2. **Cleanup commit** — `rm -rf legacy/` once the parity checklist is fully ticked.

### 5. Rollback window

Post-cutover rollback is `fly releases list` → `fly deploy --image
registry.fly.io/fpldiscord@<digest>`. After the first Go deploy the realistic target
is the **previous Go release**, not Python (dormant). Image-based, so it keeps working
after `legacy/` is deleted — fly retains recent release images with no retention
action needed. `legacy/` is never required to roll back; it exists only for reading
and diffing during the parity window. The window is gated solely on the checklist
(≈ one live GW + one waiver run) — no fixed deadline, and no pressure to rush the
`legacy/` delete since nothing depends on the old code.

### 6. Discord command cleanup — automatic

Prod commands are **global** (`main.py` sets `debug_guilds` only when `DEBUG_GUILDS`
is set; it is not). The Go bot's `ApplicationCommandBulkOverwrite` on READY (T02),
global scope (`DEV_GUILD_ID` unset — T07), replaces the entire global set, so `team`
and `update` disappear with no manual deregister. Note in the runbook: global command
propagation can take up to ~1 h after the first Go boot. No dev guild is used (blind
swap), so no guild-scoped cleanup applies.

### Follow-on

All decision tickets (T01–T10) are now resolved — nothing left to decide. The only
remaining step to the destination is stitching the resolved tickets into the single
rewrite-spec document. Graduated to [T11 — Assemble the rewrite spec](11-assemble-rewrite-spec.md)
(a `task`, not a decision).

## Comments

_(none)_
