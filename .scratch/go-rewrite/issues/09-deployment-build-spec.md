# T09 — Deployment & build spec

Parent: [Wayfinder map: Go rewrite](../map.md)
Type: grilling
Status: resolved
Blocked by: 05, 07, 08

## Question

Produce the container + fly.io deployment spec for the single Go binary — detailed
enough to `fly deploy` from a clean checkout with nothing left to decide. Graduated
from the "Deployment & build spec" fog once [T08](08-go-module-layout.md) fixed the
module tree and the `web/` -> `internal/web/dist/` embed point.

Inputs are locked:

- **Tree** (T08): `go.mod` at root; `cmd/fpldiscord`; `internal/…`; `web/` Vite
  project with `build.outDir` = `../internal/web/dist/`; `//go:embed all:dist` in
  `internal/web`. Go 1.24. CGO-free (`modernc.org/sqlite`, `discordgo`).
- **Config** (T07): fly **secrets** `DISCORD_TOKEN`, `NOTIFICATION_CHANNEL_ID`,
  `ADMIN_IDS`, `DEV_GUILD_ID`; committed **`fly.toml` `[env]`** `LEAGUE_ID`,
  `SEASON`, `DB_PATH` (`/data/fpldiscord.db`), `PORT` (`8080`), `LOG_LEVEL`
  (`info`). `.env.example` at repo root. `internal_port` must equal `PORT`.
- **Storage** (T05): SQLite on a fly volume mounted at `/data`; WAL (`-wal` /
  `-shm` siblings). Migrations run on boot, fail-fast. Volume stays near-empty.
- **Boot** (T05/T08): `config.Load` -> open DB -> migrate -> snapshot #1 (<=30 s,
  exit 1 on failure -> fly restarts) -> HTTP -> Discord + `BulkOverwrite` on READY
  -> refresher + reminder goroutines -> block on signal.
- **Target** (map Notes): fly.io legacy free tier, low RAM (~256 MB), few machines,
  3 GB volume available, no custom domain.

Decide:

- **Dockerfile** — multi-stage: (1) `node:*` builds `web/` (`npm ci && npm run
  build`) emitting `internal/web/dist/`; (2) `golang:1.24` runs `go build` with
  that `dist/` present so the embed succeeds; (3) minimal final image
  (`gcr.io/distroless/static` — CGO-free static binary) with just the binary.
  Confirm the stage wiring, the build context / `.dockerignore`, and how stage 1's
  output reaches stage 2 (`COPY --from`).
- **`fly.toml`** — app name, primary region, `[build]`, `[env]` block (T07),
  `[mounts]` (`source` -> `/data`), `[http_service]` (`internal_port` = 8080,
  `force_https`, `auto_stop_machines` / `auto_start_machines` / `min_machines_running`
  — does a Discord gateway connection + a daily timer tolerate scale-to-zero? almost
  certainly `min_machines_running = 1` and no autostop), `[[vm]]` size
  (`shared-cpu-1x`, 256 MB — enough? the snapshot parse + indexes are the RAM
  high-water mark).
- **Health check** — fly needs a check. Add `GET /healthz` (200 once snapshot #1 is
  built, 503 before) and point an `[http_service.checks]` / `[checks]` entry at it?
  Interval / grace period vs the <=30 s cold-snapshot boot.
- **Volume** — `fly volumes create` name / size / region; single-machine
  implication (a volume binds to one machine, so this reinforces one machine);
  what happens on machine replacement (volume persists; cold start re-fetches all
  API data, DB has only bet rows).
- **Slash-command registration** — already decided (T02 `BulkOverwrite` on READY,
  guild vs global on `DEV_GUILD_ID`); just note where it sits in the deploy story
  (nothing to run manually).
- **Deploy runbook** — first-time setup (`fly launch` / `fly volumes create` /
  `fly secrets set …`) vs routine deploy; how `SEASON` rollover (bump + redeploy)
  and a schema migration land; rollback.
- **Local dev parity** — `docker compose` or just `go run ./cmd/fpldiscord` +
  `vite build --watch` + a `.env`? Keep it a documented two-command flow, no
  compose?

## Answer

`fly deploy`-able from a clean checkout. One always-on machine, one volume, hard-swap
deploys. All T05/T07/T08 inputs consumed unchanged; one T09-body assumption dropped
(see Health check).

### 1. Dockerfile (repo root as build context, 3 stages)

```dockerfile
# --- stage 1: build the SPA ---
FROM node:22-alpine AS web
WORKDIR /app/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npx vite build --outDir /web-dist --emptyOutDir

# --- stage 2: build the Go binary ---
FROM golang:1.24 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /web-dist ./internal/web/dist
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /fpldiscord ./cmd/fpldiscord

# --- stage 3: runtime ---
FROM gcr.io/distroless/static-debian12
COPY --from=build /fpldiscord /fpldiscord
EXPOSE 8080
ENTRYPOINT ["/fpldiscord"]
```

- **Why `--outDir /web-dist` (CLI override, not T08's `../internal/web/dist`):** keeps
  the image build independent of the local-dev outDir. Stage 2 copies it into the
  embed point *after* `COPY . .` so `//go:embed all:dist` in `internal/web` resolves.
- **`golang:1.24` Debian, not alpine:** CGO is already off (`modernc.org/sqlite`,
  `discordgo` are pure Go); Debian avoids musl edge cases for zero size cost — the
  final image is stage 3 only.
- **`distroless/static-debian12`:** ships CA certs + tzdata + `/etc/passwd`; smallest
  base that runs a static binary with working TLS to Discord and the Draft API.
- **Runs as root.** The fly volume at `/data` is root-owned; a `nonroot` image would
  need a chown init step. Single-tenant box — not worth it. (Hardening note: switch to
  `distroless/static-debian12:nonroot` + an entrypoint `chown` if ever multi-tenant.)
- **`-trimpath` added** for reproducible builds; `-s -w` strips debug info.
- Build via fly's **remote builder** (default `fly deploy`) — no local Docker needed.
- If `go.sum` does not exist yet on the very first build, use `COPY go.mod go.sum* ./`.

### 2. `.dockerignore` (rewritten for Go) + `.gitignore` change

`.dockerignore` — replace the Python-era contents with:

```
.git
.scratch
legacy/
draft/
fonts/
web/node_modules
internal/web/dist
*.db
*.db-wal
*.db-shm
.env
.env.*
!.env.example
*.md
.github
tmp/
```

**Remove the `fly.toml` line** from both `.dockerignore` *and* `.gitignore` — T07
commits `fly.toml`. The dev `.env` stays gitignored (it replaces `config.env`, which
can be deleted).

### 3. `fly.toml` (committed at repo root)

```toml
app = "fpldiscord"
primary_region = "lhr"

[build]

[env]
  LEAGUE_ID = "64"
  SEASON = "2026/27"
  DB_PATH = "/data/fpldiscord.db"
  PORT = "8080"
  LOG_LEVEL = "info"

[[mounts]]
  source = "fpldiscord_data"
  destination = "/data"

[http_service]
  internal_port = 8080
  force_https = true
  auto_stop_machines = "off"
  auto_start_machines = false
  min_machines_running = 1

  [[http_service.checks]]
    method = "get"
    path = "/healthz"
    grace_period = "45s"
    interval = "15s"
    timeout = "5s"

[deploy]
  strategy = "immediate"

[[vm]]
  size = "shared-cpu-1x"
  memory = "256mb"
```

- **`internal_port` == `PORT`** (8080), per T07.
- **No scale-to-zero:** the Discord gateway websocket and the 05:00 UTC reminder timer
  need the process alive continuously — `auto_stop_machines = "off"`,
  `min_machines_running = 1`.
- **`strategy = "immediate"`:** one machine bound to one volume ⇒ fly cannot roll (no
  second machine can mount the volume). Every deploy is stop-old / start-new, ~40 s
  downtime. Acceptable for a private league bot; documented, not worked around.
- **256 MB:** the snapshot parse (~700 Draft elements + ~16 rosters + indexes) is a
  few MB of structs — well under. Set `GOMEMLIMIT` (e.g. `230MiB`, via `[env]` or in
  code) as a soft GC guard. Bump to `512mb` only if OOM is observed in practice.
- **`[build]` empty:** fly auto-detects the `Dockerfile`.

### 4. Health check — `/healthz`

`GET /healthz` **always returns `200`** with a small JSON body (`{"status":"ok",
"builtAt":<snapshot RFC3339>}`).

**T09-body assumption dropped:** the body proposed "200 once snapshot #1 is built, 503
before". Per T05 the HTTP listener only binds *after* snapshot #1 succeeds (fail-fast,
`exit 1` before HTTP), so there is no in-process "warming up" window to serve 503 for —
it's connection-refused for ~35-40 s, then 200. Making `/healthz` unconditionally 200
matches T05 as written and needs no reordering. The `grace_period = "45s"` covers the
cold-boot window (config → open DB → migrate → snapshot #1 ≤ 30 s → HTTP).

### 5. Volume

```bash
fly volumes create fpldiscord_data --size 1 --region lhr
```

- **1 GB** (fly minimum). The DB holds only `bet_pick_current` + `bet_archive` +
  `schema_migrations` — kilobytes, plus WAL/`-shm` siblings. Stays near-empty.
- **Binds to one machine** — reinforces the single-machine design (§3).
- **Machine replacement:** volume persists, so bet rows survive; a cold start
  re-fetches all Draft API data (nothing else is persisted).
- **No automated backups for MVP** — bet rows are re-enterable via `/bet set`. fly's
  daily volume snapshots are available if wanted later (`fly volumes snapshots`).

### 6. Deploy runbook

**First-time setup (cutover from the current Python app on the same `fpldiscord`
fly app):**

```bash
# 1. volume
fly volumes create fpldiscord_data --size 1 --region lhr

# 2. secrets (T07 fly-secret keys; repo is public so snowflakes are secrets too)
fly secrets set \
  DISCORD_TOKEN=xxx \
  NOTIFICATION_CHANNEL_ID=xxx \
  ADMIN_IDS=id1,id2
#   DEV_GUILD_ID omitted -> global command registration (set it for a dev guild)

# 3. drop the Python-era secret names (renamed in T07)
fly secrets unset TOKEN DEBUG_GUILDS   # + any others the old config.env used

# 4. deploy
fly deploy
```

`LEAGUE_ID` / `SEASON` / `DB_PATH` / `PORT` / `LOG_LEVEL` are in the committed
`fly.toml [env]` — **not** `fly secrets set`.

**Routine deploy:** `fly deploy` (remote builder; no local Docker). On READY the bot
runs `ApplicationCommandBulkOverwrite` (T02) — no manual command-registration step.

**Schema migrations:** forward-only, additive `.sql`, run on boot, fail-fast. A
migration that fails in prod = machine exits non-zero = with `strategy = "immediate"`
the release fails and the machine stays down (an outage, not a silent skip). Mitigate:
migrations are additive-only (only the two bet tables exist), and each is tested
locally against a copy of the prod DB before deploy. If one fails in prod, roll back
the image (below) and fix forward.

**Rollback:**

```bash
fly releases list
fly deploy --image registry.fly.io/fpldiscord@<digest-of-last-good>
#   or: fly releases rollback
```

~40 s downtime like any deploy. Safe against a schema that is newer than the
rolled-back binary *because* migrations are additive-only — older code ignores unknown
tables/columns.

**SEASON / LEAGUE_ID rollover:**

1. Run `/bet archive` in Discord **first** — freezes the outgoing season's totals into
   `bet_archive` while the Draft API still serves that season's live data.
2. Edit `fly.toml [env]`: bump `SEASON` (`2026/27` -> `2027/28`), set the new
   `LEAGUE_ID`.
3. Commit.
4. `fly deploy`.

### 7. Local dev parity

No `docker compose`. Two terminals:

```bash
# terminal 1 — keeps internal/web/dist/ fresh
cd web && npm run build -- --watch

# terminal 2 — the binary; loads ./.env when present
go run ./cmd/fpldiscord
```

- `.env` at repo root (gitignored), loaded only in dev by `config.Load` (T07).
- `go:embed` is compile-time, so a frontend change needs a `go run` restart. Optional:
  `wgo run ./cmd/fpldiscord` (or `air`) for Go live-reload on save.
- Matches T08's "dev = `vite build --watch`, no proxy, no build tag" exactly.

### 8. Out of scope (-> map fog)

**GitHub Actions auto-deploy** — no `.github` today; deploys are manual `fly deploy`
for the MVP. A deploy-on-push workflow is a later nicety, noted in the map's
Not-yet-specified.

### Follow-on

This resolution makes the **parity / cutover plan** specifiable (one fly app, one
machine ⇒ the Python→Go switch is a hard swap, not parallel running). Graduated to
[T10 — Parity & cutover plan](10-parity-cutover-plan.md).

## Comments

_(none)_
