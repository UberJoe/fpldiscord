# fpldiscord

A Discord bot and companion web view for a private Fantasy Premier League
**Draft** league: it mirrors the league's standings, live gameweek scores,
waivers, and a season-long side bet. The Go binary is the whole app — it serves
the Discord gateway session and the embedded web view from one process, backed
by a small SQLite database. See [`CONTEXT.md`](CONTEXT.md) for the domain
language and [`docs/runbook.md`](docs/runbook.md) for operations.

## Prerequisites

Install once:

| Tool | Why | Install |
| --- | --- | --- |
| Go (version from [`go.mod`](go.mod), currently 1.24) | builds and runs the bot | <https://go.dev/dl/> |
| Node 22 (pinned in [`web/.nvmrc`](web/.nvmrc)) | builds the web view | `nvm install 22` / `fnm install 22` / <https://nodejs.org> |
| [go-task](https://taskfile.dev) | the task runner every command below goes through | `winget install Task.Task` · `scoop install task` · `brew install go-task` · `go install github.com/go-task/task/v3/cmd/task@latest` |
| [air](https://github.com/air-verse/air) | restart-on-save for `task dev` | `go install github.com/air-verse/air@latest` |
| Docker Desktop | `task docker` only (runs the production image locally) | <https://www.docker.com/products/docker-desktop/> |
| [flyctl](https://fly.io/docs/flyctl/) | `task deploy` only (break-glass manual deploy) | <https://fly.io/docs/flyctl/install/> |

Run `task` with no arguments at any point for the full list of commands with
descriptions.

## Quickstart

```bash
task setup            # copy .env.example -> .env (no-op if .env exists)
# then edit .env: at minimum a real DISCORD_TOKEN, plus NOTIFICATION_CHANNEL_ID
# and ADMIN_IDS. Use a test bot, and set DEV_GUILD_ID for instant command
# registration in dev. See .env.example for every key.

npm --prefix web install   # one-time: install the web toolchain that task dev drives

task dev              # Vite watch build + the Go bot under air, from one terminal;
                      # Ctrl-C stops both. Saving a .go or web/ file reloads the bot.

task test             # the same checks CI runs: go build / go vet / go test + web typecheck
```

Other tasks: `task build` (release binary), `task web:build` (one-shot frontend
build), `task docker` (build and run the production image against `.env`),
`task db:reset` (drop the local SQLite DB and the `task docker` volume),
`task lint:ci` (actionlint over the workflows), `task deploy` (break-glass).

## Deploy

Merge the feature branch to `main`; GitHub Actions
([`.github/workflows/ci.yml`](.github/workflows/ci.yml)) runs the checks and, on
green, deploys to fly and health-checks `/healthz`. There is no manual step in
the normal path. See [`docs/runbook.md`](docs/runbook.md) for the pipeline, the
break-glass manual deploy, and rollback.
