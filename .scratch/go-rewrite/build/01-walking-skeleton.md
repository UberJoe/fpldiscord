# 01 — Walking skeleton: deployable binary

**Spec:** [../to-spec.md](../to-spec.md)

**What to build:** A `fly deploy` from a clean checkout brings up one always-on
machine running a single Go binary. `GET /healthz` returns 200 once the HTTP listener
is bound. The Discord bot connects, registers its command set on `READY`, and `/dave`
replies with the joke text unconditionally for any caller. Starting the binary with a
required config key missing or invalid exits non-zero with one message listing every
problem at once; a valid load logs the effective configuration once with the Discord
token redacted. The process shuts down gracefully on `SIGINT`/`SIGTERM` (HTTP first,
then Discord).

**Blocked by:** None — can start immediately.

**Status:** ready-for-agent

- [ ] `fly deploy` via the remote builder (3-stage Docker build: web → Go → distroless
      static) produces a running machine in `lhr` with the `fpldiscord_data` volume
      mounted at `/data`
- [ ] `GET /healthz` returns 200 with a small JSON body once bound; before snapshot #1
      the port is connection-refused, never 5xx
- [ ] The bot shows online in the guild; `/dave` returns the joke text for any caller
      (no user check)
- [ ] `ApplicationCommandBulkOverwrite` runs once on `READY` — guild-scoped if
      `DEV_GUILD_ID` is set, else global
- [ ] `config.Load()` runs before anything else, validates the 9-key inventory, and
      aggregates every missing/invalid required key into a single non-zero exit
- [ ] On success the effective config is logged once at `info` with `DISCORD_TOKEN`
      redacted (snowflakes printed in full)
- [ ] Boot attempts snapshot #1 with a ≤30 s budget; permanent failure exits non-zero
      so fly restarts the machine
- [ ] `.env.example` is checked in with placeholder values; a local `.env` is loaded
      only in dev; the `fly.toml` line is removed from `.dockerignore` and `.gitignore`
- [ ] `SIGINT`/`SIGTERM` triggers graceful shutdown in HTTP → Discord order
