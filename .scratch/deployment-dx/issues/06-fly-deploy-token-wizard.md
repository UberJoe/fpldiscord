# 06 — Fly deploy token provisioning wizard

**What to build:** A short interactive wizard (per the `wizard` skill) that walks
the maintainer through the one-time steps CI can't do itself: minting a fly
deploy token and registering it as a repository secret.

- Wizard steps:
  1. Confirm `flyctl` is installed and `fly auth whoami` is the right account.
  2. `fly tokens create deploy -x 8760h` — deploy-scoped, one-year expiry. NOT
     `fly tokens create org` / a personal access token.
  3. Copy the printed `FlyV1 …` value.
  4. GitHub → repo Settings → Secrets and variables → Actions → New repository
     secret → name `FLY_API_TOKEN`, paste the value.
  5. (Recommended, manual) Settings → Branches → branch protection rule for
     `main` → require the `test` status check to pass before merge. Documented as
     a recommendation, not applied by any ticket.
  6. Verify: re-run the latest `main` workflow (or push a trivial commit) and
     confirm the `deploy` job authenticates and the `/healthz` poll passes.
- The wizard is a script the maintainer runs; it prints instructions and pauses
  for the dashboard steps it can't perform. It does not store or echo the token.

**Blocked by:** None. Pairs with 05 — 05's first green deploy run needs this done.

**Status:** in-review

- [x] Wizard script exists and runs on the maintainer's machine (PowerShell OK).
      — `scripts/fly-deploy-token-wizard.sh` (from the `/wizard` template lib);
      `bash -n` clean, `chmod +x`. Bash script — the maintainer runs it from
      git-bash (or `bash scripts/…` from a PowerShell prompt); URL-open uses
      `explorer.exe` on Windows via the template's cross-platform `open_url`.
- [x] It mints a **deploy-scoped** token and never prints it back to the terminal
      or a file. — Stage 2 runs `fly tokens create deploy -x 8760h`; the value is
      never captured into a variable, `echo`-ed, or `write_env`-ed. Stage 3 uses
      `gh secret set FLY_API_TOKEN --repo <repo>` with **no value argument**, so
      the token flows from the maintainer's paste straight into `gh`'s hidden
      prompt. `set_secret` (which takes a value) is deliberately not used.
- [x] It gives exact click-path instructions for the `FLY_API_TOKEN` repo secret
      and the branch-protection recommendation. — Stage 3 has both the `gh` path
      and the dashboard path (`…/settings/secrets/actions/new`, name
      `FLY_API_TOKEN`, paste, Add secret). Stage 4 opens `…/settings/branches`
      with the full rule steps and states plainly it is a recommendation no
      ticket applies.
- [~] After the wizard, the `deploy` job in `ci.yml` authenticates to fly. —
      Stage 5 is the verify step (trigger a `main` push / re-run, watch the
      `deploy` job auth + `/healthz` poll). Not exercisable until ticket 05 adds
      the `deploy` job; the wizard notes this.

## Comments

### 2026-09-08 — implemented

`scripts/fly-deploy-token-wizard.sh` — 5 stages on the `/wizard` template
library (library section verbatim, not hand-edited):

1. **Fly CLI — confirm the right account.** Guards on `flyctl`/`fly` presence
   (opens the install docs and exits if missing), runs `fly auth whoami`,
   `confirm`s it owns the `fpldiscord` app.
2. **Mint the deploy token.** `fly tokens create deploy -x 8760h -n
   "github-actions-deploy"` — deploy-scoped, one-year; the `-n` name is a small
   add so the token is findable in `fly tokens list` for later revocation.
   Warns that `fly` shows the value once and to copy the whole `FlyV1 …` string.
3. **Store it as `FLY_API_TOKEN`.** Prefers `gh secret set FLY_API_TOKEN
   --repo <repo>` (gh's own hidden prompt; wizard never sees the value);
   dashboard click-path as the fallback and for when `gh` is absent.
4. **Recommended branch protection on `main`.** Opens `…/settings/branches`,
   full steps to require the `test` check; explicitly "a recommendation, no
   ticket applies it".
5. **Verify.** Push to `main` / re-run the `CI` workflow; confirm the `deploy`
   job authenticates and the `/healthz` poll returns `200`. Notes the `deploy`
   job arrives with ticket 05.

Repo owner/name for the URLs is resolved via `gh repo view`, falling back to the
`origin` remote, then a literal `UberJoe/fpldiscord`.

`.dockerignore` gains `scripts/` — the `Dockerfile`'s `COPY . .` would otherwise
pull the wizard into the build context (same housekeeping as `Taskfile.yml` /
`.air.toml`).

`go test ./...` green (no app code touched). Not run end-to-end (it opens a
browser and blocks on input) — traced statically per the `/wizard` skill:
every stage traces to concrete instructions, and the `FLY_API_TOKEN` name
matches the `secrets.FLY_API_TOKEN` reference ticket 05's `deploy` job will use.
README linkage is ticket 07's.
