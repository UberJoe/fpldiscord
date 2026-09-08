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

**Status:** ready-for-agent

- [ ] Wizard script exists and runs on the maintainer's machine (PowerShell OK).
- [ ] It mints a **deploy-scoped** token and never prints it back to the terminal
      or a file.
- [ ] It gives exact click-path instructions for the `FLY_API_TOKEN` repo secret
      and the branch-protection recommendation.
- [ ] After the wizard, the `deploy` job in `ci.yml` authenticates to fly.

## Comments

_(none)_
