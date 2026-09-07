# Deploy runbook — fpldiscord (Go)

Operational steps for the fly.io deploy. The Go binary is the only thing that ships;
the Python tree is quarantined under [`legacy/`](../legacy/) and is not built or run.

- App: `fpldiscord` · region: `lhr` · one always-on `shared-cpu-1x` / 256 MB machine.
- Config split: identifying values are **fly secrets**; operational values live in
  committed `fly.toml [env]`. See [`.env.example`](../.env.example) for the full key list.
- Image: 3-stage `Dockerfile` at repo root, built by fly's remote builder. No local
  Docker needed.

---

## First-time cutover (Python → Go)

The Python app on fly is dormant, so this is a blind hard swap — no parallel run, no
shakedown. Run these **in order** from the repo root:

```bash
# 1. Create the SQLite volume (1 GB is the fly minimum; the DB is kilobytes).
fly volumes create fpldiscord_data --size 1 --region lhr

# 2. Set the Go app's secrets. ADMIN_IDS is comma-separated and must be non-empty
#    (it gates /bet set and /bet archive). DEV_GUILD_ID is intentionally omitted
#    so commands register globally.
fly secrets set \
  DISCORD_TOKEN="<bot token>" \
  NOTIFICATION_CHANNEL_ID="<channel id>" \
  ADMIN_IDS="<discord user id>,<discord user id>"

# 3. Remove the old Python app's secrets so nothing stale lingers.
fly secrets unset TOKEN DEBUG_GUILDS

# 4. Ship the Go image.
fly deploy
```

`fly deploy` uses `strategy = "immediate"` (single machine bound to a single volume),
so expect ~40 s of downtime while the machine replaces.

### Post-deploy verification

- `fly logs` shows `config.Load()` output once (with `DISCORD_TOKEN` redacted), then
  migrations applied, then snapshot #1 built, then the HTTP listener bound.
- `curl https://fpldiscord.fly.dev/healthz` → `200` `{"status":"ok",...}`.
- `curl https://fpldiscord.fly.dev/api/standings` → `200` with a `{meta, data}` body
  (503 `starting up` only in the ~35–40 s before snapshot #1).
- The webpage loads and lands on Standings.
- **The global command set no longer contains `team` or `update`.** The bot runs
  `ApplicationCommandBulkOverwrite` on `READY`; global propagation takes up to ~1 h.
  Check in Discord (type `/` and look at the fpldiscord command list) or via the API
  once propagation settles.
- `/scores` and `/standings` return sane numbers against the live league.

---

## Routine deploy

```bash
fly deploy
```

Migrations are forward-only and additive, applied on boot with fail-fast: a bad
migration stops the release rather than corrupting data.

---

## Rollback

Roll back to the **previous Go release image** — this does not depend on the
`legacy/` tree still existing:

```bash
# Option A — redeploy a specific prior image digest.
fly releases --image                       # find the previous Go release's digest
fly deploy --image registry.fly.io/fpldiscord@<digest>

# Option B — let fly pick the previous release.
fly releases rollback
```

Rolling back to an older binary is safe against a newer schema because migrations are
additive-only (an old binary never references columns it didn't ship with).

---

## Season / league rollover

The Draft API keeps serving the outgoing season live for a while, so freeze it in the
DB **before** pointing the app at the new season:

```bash
# 1. Archive the finished season while the Draft API still has its data.
#    (Discord, admin-only) /bet archive season:<old> ...

# 2. Bump the committed operational config.
#    Edit fly.toml [env]: SEASON (and LEAGUE_ID if the league id changes).
git commit -am "Roll over to season <new>"

# 3. Redeploy.
fly deploy
```

Both `bet` tables filter on `season`, so the new season starts empty and the archived
season stays queryable via `/bet <old-season>`.

---

## Retiring `legacy/`

`legacy/` (the Python `draft/` tree, `requirements.txt`, and the old Python
`Dockerfile`) stays in the repo, diff-able, until behavioural parity is signed off.

**Gate:** every row in
[`.scratch/go-rewrite/parity-checklist.md`](../.scratch/go-rewrite/parity-checklist.md)
is ticked against real data in the live guild — roughly one gameweek with live matches
plus one processed waiver run. Sign-off is Joe's.

Once ticked, a separate cleanup commit does `rm -rf legacy/` (and drops the now-unused
`legacy/` entry from `.dockerignore`). No deadline; there is no cost to leaving it in
place beyond repo noise.
