# 01 — Taskfile and local dev loop

**What to build:** A `Taskfile.yml` at the repo root that is the single home for
every routine command, plus the `air` config that powers restart-on-save. After
this ticket a developer runs `task setup` then `task dev` and has the whole local
loop from one terminal.

- `Taskfile.yml` (go-task, `version: '3'`) at repo root. `default` runs `task
  --list`; every task has a `desc:`.
- `task setup` — copy `.env.example` → `.env` only if `.env` is absent; print
  "created .env …" on copy, "keeping existing .env" otherwise. Never overwrite.
- `task dev` — refuse with "run `task setup` first" if `.env` is missing;
  otherwise run the Vite watch build (`npm run build -- --watch` in `web/`) and
  the Go binary under `air` concurrently via `npx concurrently`, named prefixes,
  kill-others semantics so Ctrl-C or a crash in either stops both.
- `.air.toml` (committed) — build `./cmd/fpldiscord`, run the built binary; watch
  `.go` and the `internal/web/dist/` tree (so a Vite rebuild flows into the embed);
  exclude `web/`, `legacy/`, `.scratch/`, `tmp/`, `bin/`, test files.
- `task test` — `go build ./...`, `go vet ./...`, `go test ./...`, and `npm run
  typecheck` in `web/`. Same command set the CI check job will run.
- `task build` — `npm run build` in `web/`, then `go build -o ./bin/fpldiscord
  ./cmd/fpldiscord`.
- `task web:build` — the `web/` one-shot build alone.
- `task db:reset` — remove `fpldiscord.db{,-wal,-shm}` at repo root if present;
  print what was removed. (Ticket 02 extends this to also drop the local
  container's data volume.)
- `web/package.json` — add `concurrently` to `devDependencies`; commit the
  `package-lock.json` change.
- `.gitignore` — add `/bin/`. `.dockerignore` — add `Taskfile.yml`, `.air.toml`,
  `.nvmrc`.
- Command bodies use portable `sh` syntax (go-task runs them through
  `mvdan.cc/sh`), so `task setup` / `task db:reset` conditionals must work in
  PowerShell with no bash installed — verify on the Windows box.

**Blocked by:** None — can start immediately.

**Status:** done

- [x] `Taskfile.yml` at repo root; bare `task` prints the task list with
      descriptions.
- [x] `task setup` on a checkout with no `.env` creates it from `.env.example`;
      re-running with `.env` present changes nothing and says so.
- [x] `task dev` with no `.env` exits telling the developer to run `task setup`
      and starts neither process.
- [x] `task dev` with `.env` present starts both the Vite watch and the bot;
      Ctrl-C stops both with no orphan process on the port. — Seam-2 manual
      acceptance done on the Windows box 2026-09-08: both `[web]`/`[bot]` streams
      start, the bot shuts down gracefully on Ctrl-C, `concurrently --kill-others`
      tears down the survivor, and no LISTEN socket remains on 8080 (only
      transient `TIME_WAIT`). The Windows `Terminate batch job (Y/N)?` prompt on
      Ctrl-C is a cosmetic `cmd.exe` artifact of interrupting `npm.cmd`, not a
      hang — both processes have already stopped.
- [x] Saving a `.go` file under `task dev` rebuilds and restarts the binary. —
      Seam-2 manual acceptance done 2026-09-08.
- [x] Saving a `web/` source file under `task dev` results in the running bot
      serving the updated asset (Vite rebuild → `air` rebuild → embed refresh).
      — Seam-2 manual acceptance done 2026-09-08.
- [x] `task test` runs `go build`/`go vet`/`go test` and the web typecheck; exit
      code reflects failure in any of them.
- [x] `task build` produces `./bin/fpldiscord` with web assets embedded.
- [x] `task db:reset` removes the local DB files, printing what it removed, and is
      a clean no-op when nothing exists.
- [x] `task setup` and `task db:reset` work in PowerShell on Windows with no bash
      installed — verified on the box (`task setup` both paths, `task db:reset`
      both paths). go-task runs command bodies through its embedded shell; `cp`
      and `rm` resolve as bundled coreutils builtins (go-task >= 3.41), not from
      PATH — confirmed with `command -v cp` finding nothing while `cp` still ran.
      The Taskfile header records the >= 3.41 floor.
- [x] `/bin/` is gitignored; `Taskfile.yml`, `.air.toml`, `.nvmrc` are in
      `.dockerignore`.
- [x] `go test ./...` is green.

## Comments

### 2026-09-08 — implemented

`Taskfile.yml` (`setup`, `dev`, `test`, `build`, `web:build`, `db:reset` +
`default`) and `.air.toml` added at repo root; `concurrently@^10` added to
`web/package.json` devDependencies with the `package-lock.json` change;
`/bin/` added to `.gitignore`; `Taskfile.yml`/`.air.toml`/`.nvmrc` added to
`.dockerignore`.

`task docker`, `task deploy`, `task lint:ci` are intentionally not here — they
land with tickets 02/03.

Verified on the Windows/PowerShell box: bare `task`, `setup` (both paths),
`dev` no-`.env` guard, `test` (green), `build` (`./bin/fpldiscord` with the real
Vite bundle embedded), `db:reset` (both paths). The three `air`-driven
restart-on-save behaviours are Seam-2 manual acceptance per the spec.

### 2026-09-08 — code-review pass

`/code-review` (Standards + Spec axes) run against the diff. Changes made in
response:

- `.air.toml`: added `include_dir = ["cmd", "internal"]` so the watch is
  actually scoped to those trees rather than matching `.html/.js/.css`
  repo-wide; verified air still rebuilds on an `internal/web/dist/` edit.
  Dropped the no-op `#:schema` line; header comment corrected to match.
- `.dockerignore`: also ignore `bin/` (the `Dockerfile` does `COPY . .`, so
  build output would otherwise enter the context; mirrors `tmp/`, already in
  both ignore files).
- `Taskfile.yml`: `build` now calls `web:build` as a subtask instead of a
  duplicated `cd web && npm run build`; `web:build` uses `dir: web`. Header
  notes the go-task `>= 3.41` floor for the bundled `cp`/`rm` builtins.

Both axes' remaining notes (Seam-2 manual acceptance still pending; `.nvmrc` in
`.dockerignore` ahead of ticket 04 creating it, which ticket 01 mandates) are
expected and not acted on.

### 2026-09-08 — Seam-2 acceptance

Ran on the Windows/PowerShell box. `task dev` starts the Vite watch and the bot
under `air` from one terminal; a `.go` edit and a `web/` edit each flow through
to a bot restart; Ctrl-C stops both with no listener left on 8080. Closed.
(`.nvmrc` → `web/.nvmrc` in `.dockerignore` was corrected in ticket 04.)
