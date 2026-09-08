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

**Status:** ready-for-agent

- [ ] `task docker` builds `fpldiscord:local` from the repo-root `Dockerfile`.
- [ ] The container boots, loads config from `.env`, serves `GET /healthz` → `200`
      on `localhost:8080`, and the web page loads.
- [ ] Writes persist: run `task docker`, let a snapshot build / a bet row be set,
      stop it, run `task docker` again — the data is still there.
- [ ] This ticket extends `task db:reset` to drop `fpldiscord_local_data`; after
      it, the next `task docker` starts from an empty store.
- [ ] `docker history fpldiscord:local` shows no `.env` / secret layer.
- [ ] Works on Windows PowerShell with Docker Desktop.

## Comments

_(none)_
