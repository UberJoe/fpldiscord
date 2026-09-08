# 02 — `task docker` — run the production image locally

**What to build:** A `task docker` target that builds the real 3-stage
`Dockerfile` and runs it locally against `.env`, so image-only breakage is caught
before it reaches `main`.

- `task docker` step 1: `docker build -t fpldiscord:local .` (repo root context,
  existing `Dockerfile`).
- `task docker` step 2: `docker run --rm -it --env-file .env -e
  DB_PATH=/data/fpldiscord.db -v fpldiscord_local_data:/data -p 8080:8080
  fpldiscord:local`.
  - `--env-file .env` supplies config; `-e DB_PATH=/data/fpldiscord.db` overrides
    the `.env` value (`./fpldiscord.db`, right for `go run`, wrong in the image).
  - `-v fpldiscord_local_data:/data` — named volume so the SQLite DB and warm
    snapshot survive across runs, mirroring fly's `/data` mount.
- Extend `task db:reset` (defined in ticket 01) to also
  `docker volume rm -f fpldiscord_local_data` (ignore "no such volume"), so a
  reset clears both the `go run` and the containerised store.
- No compose file. Nothing baked into the image — `.env` is already in
  `.dockerignore`, the token enters only at `docker run` time.
- README prerequisites (ticket 07) gains "Docker Desktop, for `task docker`".

**Blocked by:** 01 (adds the target to the Taskfile created there).

**Status:** in-review

- [~] `task docker` builds `fpldiscord:local` from the repo-root `Dockerfile`. —
      `docker build -t fpldiscord:local .` (repo-root context) is step 1 of the
      target; full build is Seam-2 manual acceptance (Docker daemon needed).
- [~] The container boots, loads config from `.env`, serves `GET /healthz` → `200`
      on `localhost:8080`, and the web page loads. — Seam-2 manual acceptance;
      run command wires `--env-file .env`, `-p 8080:8080`, `-e DB_PATH=/data/…`.
- [~] Writes persist: run `task docker`, let a snapshot build / a bet row be set,
      stop it, run `task docker` again — the data is still there. — Seam-2 manual
      acceptance; `-v fpldiscord_local_data:/data` named volume is wired.
- [~] This ticket extends `task db:reset` to drop `fpldiscord_local_data`; after
      it, the next `task docker` starts from an empty store. — `db:reset` now runs
      `docker volume rm -f fpldiscord_local_data` and reports it in the removed
      list; verified it degrades to a clean no-op when Docker is absent/stopped
      and still removes the DB files (exit 0). Empty-store-on-next-run is Seam-2.
- [x] `docker history fpldiscord:local` shows no `.env` / secret layer. —
      guaranteed by the existing `.dockerignore` (`.env`, `.env.*` excluded, only
      `!.env.example` kept); the token enters only at `docker run` time via
      `--env-file`.
- [~] Works on Windows PowerShell with Docker Desktop. — `Taskfile.yml` parses
      and `task --list` shows `docker` on the box; command bodies are portable
      `sh`; full containerised run is Seam-2 manual acceptance.

## Comments

### 2026-09-08 — implemented

Added the `docker` task to `Taskfile.yml` (build `fpldiscord:local` from the
repo-root `Dockerfile`, then `docker run --rm -it --env-file .env -e
DB_PATH=/data/fpldiscord.db -v fpldiscord_local_data:/data -p 8080:8080`). It
carries the same `.env`-missing guard as `dev` so `--env-file` failures are
legible. Extended `db:reset` to also `docker volume rm -f fpldiscord_local_data`
— `-f` makes a missing volume a no-op, `|| true` covers Docker being
absent/stopped, and the volume is named in the "removed:" line only on a real
removal. Taskfile header prerequisites gain "Docker Desktop for `task docker`".

No compose file, no image change. `.env` / `Taskfile.yml` / `.air.toml` /
`.nvmrc` / `bin/` are already in `.dockerignore`, so the build context stays
clean. `go test ./...` green (no app code touched).

Not done here: the README "Docker Desktop, for `task docker`" prerequisite line
belongs to ticket 07 (no README exists yet). The full containerised run
(build → boot → `/healthz` 200 → persistence across two runs) is Seam-2 manual
acceptance per the spec and needs the Docker daemon running.
