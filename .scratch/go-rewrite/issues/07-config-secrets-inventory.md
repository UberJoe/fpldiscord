# T07 — Config & secrets inventory

Parent: [Wayfinder map: Go rewrite](../map.md)
Type: grilling
Status: resolved
Blocked by: —

## Question

Produce the **complete** config / secrets inventory for the Go binary — every env
var, its source, its default, whether it is a secret (fly secret) or plain config,
and what breaks if it is missing. Graduated from fog once [T05](05-data-model.md)
pinned the DB path and the refresh model.

Known so far, to be assembled and gap-checked:

- **Discord** (T02): `DISCORD_TOKEN` (secret), `DEV_GUILD_ID` (optional — guild-scoped
  command registration when set, else global).
- **League / season** (T03): `LEAGUE_ID` (`64` this season), `SEASON` (string, e.g.
  `2026/27` — the API has no season identifier; rollover = bump + redeploy).
- **Admin / behaviour** (T03): `ADMIN_IDS` (comma-separated Discord user ids, gates
  `/bet set` + `/bet archive`), `dave` target user-id, notification channel id.
- **Storage** (T05): `DB_PATH` (default `/data/fpldiscord.db`; local dev
  `./fpldiscord.db`).
- **Confirmed absent** (T01 §2): no FPL / `users.premierleague.com` credentials —
  every endpoint the bot uses is public.

Decide:

- Canonical name for every key (the Python code mixes `NOTIFICATION_CHANNEL`,
  `PWD`, `EMAIL`, etc. — settle a consistent scheme).
- Which keys are **fly secrets** vs plain `[env]` in `fly.toml`.
- Whether the refresh TTLs and the waiver-reminder offsets (today / one-hour pings)
  stay **hardcoded constants** (T05's assumption) or become env knobs.
- Defaults and required-vs-optional for each; the startup behaviour when a required
  one is missing (fail fast with a clear message).
- Where the inventory is written so the weekend build and the deploy spec can both
  consume it (a `.env.example` + a table in the assembled spec).

## Answer

One round of grilling with Joe; all recommendations adopted except the `dave`
easter egg — Joe cut it entirely (see "Dropped"). Repo visibility was checked, not
asked: `github.com/UberJoe/fpldiscord` is **public**, which drives the secret /
`[env]` split below.

---

### Naming scheme

Unprefixed `SCREAMING_SNAKE_CASE`. Two suffix rules: `_ID` / `_IDS` on every
Discord snowflake; `_MS` / `_SECONDS` on any duration that ever becomes a knob.
No `FPLD_` namespace — single-tenant app on its own fly machine, nothing to
collide with, and the bare names match fly/Docker convention.

### The inventory — 9 keys

**fly secrets** (set via `fly secrets set`, never committed — repo is public):

| Key | Req | Default | Missing → | Notes |
|---|---|---|---|---|
| `DISCORD_TOKEN` | yes | — | fail fast | bot token (was `TOKEN`) |
| `NOTIFICATION_CHANNEL_ID` | yes | — | fail fast | waiver-reminder target channel (was `NOTIFICATION_CHANNEL`) |
| `ADMIN_IDS` | yes, **non-empty** | — | fail fast | comma-separated Discord user ids; gates `/bet set` + `/bet archive` (T03). Empty is a fail, not "no admins" — a silently un-administrable bet feature is worse than a boot error |
| `DEV_GUILD_ID` | no | — | unset → **global** command registration; set → **guild-scoped**, instant | was `DEBUG_GUILDS` (always singular in practice) |

Snowflake ids are not credentials, but committing them publishes the server's
channel layout and exactly who holds admin — so they go in secrets, not `fly.toml`.

**`[env]` in committed `fly.toml`** (operational, non-identifying):

| Key | Req | Default | Missing → | Notes |
|---|---|---|---|---|
| `LEAGUE_ID` | yes | — | fail fast | Draft league id (`64` this season; was hardcoded `12`→`64` inside `fplutils.py` URL strings — the exact manual-edit pain this removes). No default: a wrong-league default that half-works is worse than a boot error |
| `SEASON` | yes | — | fail fast | string, format `2026/27`. No API season identifier exists (T01); rollover = bump + redeploy. Used for `bet_archive` labels + `/api/bet`'s `season` field (T05/T06) |
| `DB_PATH` | no | `/data/fpldiscord.db` | — | fly volume mount; local dev `./fpldiscord.db` (T05) |
| `PORT` | no | `8080` | — | HTTP listen port; matches `EXPOSE 8080`; fly.toml `internal_port` must agree |
| `LOG_LEVEL` | no | `info` | — | `debug` \| `info` \| `warn` \| `error`. The **only** ops-tunable knob exposed |

### Dropped from the candidate list

- `PWD`, `EMAIL` — dead FPL login (`login()` is dead code, T01 §2).
- `IMG_FONT` — dies with the `team` command (OOS).
- **`DAVE_STEVE_USER_ID`** — proposed, **rejected by Joe**. `/dave` becomes an
  unconditional `"fuck you Dave"`. The `str(ctx.user) == "bigsamspintofwine#0"`
  → `"fuck you Steve"` branch is deleted outright: no user check, no config key.

### Hardcoded constants — explicitly NOT env

Revisit only with a code change; tuning any of these needs a redeploy anyway
because the code understands their coupling.

- **Per-endpoint refresh TTLs** → constants in `internal/fpl` (coupled to the
  event-driven force-refresh on GW / `waivers_processed` change, T05).
- **Waiver-reminder schedule** — 05:00 UTC wake, same-day "today" ping, T−1h ping
  → constants; timezone **UTC hardcoded** (matches current `datetime.utcnow`).
- **Draft API `User-Agent`** → constant
  `fpldiscord/2.0 (+https://github.com/UberJoe/fpldiscord)` (T01: "set a real UA").
- **Bet bettors / picks** → SQLite (`bet_pick_current`, T05), never config.

### Startup validation

- A single `config.Load()` at the very top of boot — **before** DB open and
  Discord connect (prepends T05's fail-fast boot sequence).
- Validates every required key, **aggregates** all missing/invalid ones, and exits
  non-zero with **one** message listing all of them — not fail-on-first.
- Optional keys with defaults: applied silently, no warning.
- On success: log the effective config once at `info`, with `DISCORD_TOKEN`
  redacted to `***`; snowflake ids printed in full (not secret, and seeing them
  saves a fly-deploy debugging round-trip).

### Where the inventory lives

- **`.env.example`** at repo root, checked in, placeholder values, one comment line
  per key (required/optional · default · secret vs `[env]`). Replaces Python's
  gitignored `config.env`. Local dev copies it to `.env`; add `.env` to
  `.gitignore`. The three `load_dotenv(dotenv_path='config.env')` calls collapse to
  one `.env` load that runs **in local dev only** — on fly the real env is injected.
- The assembled rewrite spec carries the same content as a table.

### Cross-references

- T02 — `DISCORD_TOKEN`, `DEV_GUILD_ID`.
- T03 — `LEAGUE_ID`, `SEASON`, `ADMIN_IDS`, notification channel.
- T05 — `DB_PATH`; the fail-fast boot sequence this prepends to; the TTL constants.
- T06 — no config (the `/api/*` surface has none).
- **Deployment & build spec** (map fog) — consumes the secret / `[env]` split and
  `.env.example` verbatim.

### Graduated from fog

- Nothing new. This adds one input — a `config` package owning `config.Load()` +
  the typed config struct, consumed before every other subsystem at boot — to the
  **Go project / module layout** fog item, which is now itself graduated into
  [T08 — Go project & module layout](08-go-module-layout.md).

## Comments

_(none)_
