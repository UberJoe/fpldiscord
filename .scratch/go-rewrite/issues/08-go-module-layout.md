# T08 — Go project & module layout

Parent: [Wayfinder map: Go rewrite](../map.md)
Type: grilling
Status: resolved
Blocked by: 02, 05, 06, 07

## Question

Fix the complete Go module + directory tree for the rewrite — the last structural
decision before a weekend build session can start laying down files. Graduated from
the "Go project / module layout" fog item once T06 (the first `web -> bot` edge)
and T07 (the `config` package) landed.

Inputs are locked:

- **One binary, one process** (map Notes). Fresh Go module in this repo; Python
  moves to `legacy/` and is deleted at parity.
- **Packages already named by resolved tickets:**
  - `internal/fpl` — immutable `*Snapshot`, single refresher goroutine, lock-free
    `Current()`, six derived-view pure functions (T05).
  - `internal/store` — SQLite, `go:embed`'d `.sql` migrations + a tiny runner, the
    `bet_pick_current` / `bet_archive` / `schema_migrations` tables (T05).
  - `internal/bet` — bet rule logic (`in` / `provisionallyOut` / `bust` / `leader`,
    sort order), called by **both** the `/bet` command and `GET /api/bet` (T05, T06).
  - a `config` package — typed config struct + `config.Load()`, run before every
    other subsystem at boot (T07).
- **Confirmed cross-package edges:**
  - `internal/web` -> `internal/bot`: `GET /api/bet` display names call
    `MemberName(discordUserID)` on the bot's `discordgo` state cache (T06) — the
    first `web -> bot` dependency.
  - everything reads `internal/fpl.Current()`; both `internal/web` and
    `internal/bot` call `internal/bet`; `internal/store` is used by `internal/bet`
    and the `/bet` command path.
- **Boot sequence** (T05 step list + T07 prepend): `config.Load()` -> open DB ->
  migrate (fail-fast) -> first snapshot (<=30 s, fail-fast) -> HTTP server ->
  Discord session -> refresher + waiver-reminder goroutines.
- **discordgo**, hand-rolled handler map, `ApplicationCommandBulkOverwrite` on
  READY, `time.Timer` reminder goroutine (T02).

Decide:

- The full tree: `cmd/` layout (single `main` — it is one binary), and where each
  of `internal/bot`, `internal/web`, `internal/fpl`, `internal/store`,
  `internal/bet`, and `config` sits (top-level `config` vs `internal/config`).
- `web/` — the React (Vite) app location; which package owns the `go:embed`
  directive + the `embed.FS` -> `http.FileServer` wiring; how the
  dev-server-vs-embedded-assets switch works (build tag? env? always embed and
  proxy in dev?).
- Where the boot sequence lives — a fat `cmd/fpldiscord/main.go` that wires
  everything, or a thin `main` over an `internal/app` (or `internal/server`)
  package that owns the sequence and shutdown.
- How the `web -> bot` edge is expressed without an import cycle and without
  making `bot` un-startable on its own — `bot` exposes a narrow `MemberNamer`
  interface, `app` injects a func, or a shared interface package.
- Module path — `github.com/UberJoe/fpldiscord` at repo root (with `legacy/`
  holding untouched Python), or something else.
- Whether any cross-cutting types need a home beyond `internal/fpl` (an
  `internal/domain` / `internal/types`), or the `fpl` structs + per-package types
  are enough.
- Test layout and which package a build session should stub or build first.

## Answer

Two rounds of grilling with Joe; all recommendations adopted. Round 2 was one
clarification — Q5's dev-asset workflow — resolved toward the simpler option
(`vite build --watch`, no dev proxy, no build tag).

---

### Module & repo layout

- **One `go.mod`** at repo root, module `github.com/UberJoe/fpldiscord`, Go **1.24**
  (for `net/http.ServeMux` method+pattern routing and current stdlib). Single
  module — nothing imports `legacy/`, so no submodule.
- Target root tree:

  ```
  go.mod  go.sum
  cmd/fpldiscord/main.go
  internal/{config,fpl,store,bet,bot,web,app}/
  web/                     React + Vite project (own package.json, node_modules, src/)
  legacy/                  the current draft/ + requirements.txt, moved wholesale
  Dockerfile  fly.toml  .env.example
  ```

- `git mv draft/ legacy/` (+ `requirements.txt`) happens at **build / cutover
  time**, not now — T08 only fixes the target shape. `fonts/` is **deleted** with
  the `team` command, not moved.

### The binary — `cmd/` + `internal/app`

- `cmd/fpldiscord/main.go` is a ~15-line shell: `config.Load()` -> `app.New(cfg)`
  -> `app.Run(ctx)` -> map the returned error to an exit code. Nothing testable
  lives here.
- **`internal/app`** owns the `App` struct and the T05 boot sequence:
  - `New(cfg) (*App, error)` — boot steps 1–3: open DB (`PRAGMA` per T05),
    migrate (**exit 1** on failure), build snapshot #1 (block <=30 s, **exit 1**
    if it never succeeds).
  - `Run(ctx) error` — boot steps 4–7: start `http.Server`; `discordgo` `Open()` +
    `ApplicationCommandBulkOverwrite` on `READY` (guild if `DEV_GUILD_ID` set, else
    global); start the refresher + waiver-reminder goroutines; block on
    `SIGINT`/`SIGTERM`; graceful shutdown **HTTP -> Discord -> DB**.

### `internal/` package set & dependency direction

Flat under `internal/`. The import graph is a DAG rooted at `app`:

| Package | Role | Internal imports |
|---|---|---|
| `config` | T07 `Config` struct + `Load()` (aggregate-then-fail, redacted effective-config log) | — |
| `fpl` | immutable `*Snapshot`, refresher goroutine, `Current()`, six derived views, `ApplyAutoSubs`; **owns the id types** `ElementID` / `EntryID` / `LeagueEntryID` and `Pos` (T05) | — |
| `store` | SQLite, `go:embed`'d `.sql` migrations + ~30-line runner, `bet_pick_current` / `bet_archive` / `schema_migrations` (T05) | — |
| `bet` | bet rule logic — `in` / `provisionallyOut` / `bust` / `leader`, sort order (T05/T06) | `fpl`, `store` |
| `bot` | `discordgo` session, hand-rolled handler map, 10 commands, `time.Timer` reminder goroutine (T02/T03) | `config`, `fpl`, `store`, `bet` |
| `web` | `http.Server`, `/api/*` handlers, embedded-SPA serving (T04/T06) | `config`, `fpl`, `bet` |
| `app` | boot sequence, wiring, shutdown | all of the above |

- **No `internal/domain` / `internal/types`.** The id types and `Pos` stay in
  `fpl`, which every other package already imports. `fpl`, `store`, `config` are
  leaves.
- **`internal/web` does not import `internal/bot`** — see the next section.

### The `web -> bot` edge (T06) — runtime wiring, not a compile dependency

T06 needs a Discord user-id -> display-name lookup for `/api/bet`, sourced from the
bot. Resolved in two parts:

1. **Consumer-side interface.** `internal/web` declares
   `type MemberNamer interface { MemberName(discordUserID string) (string, bool) }`
   and takes one in its constructor. `internal/bot`'s `*Bot` satisfies it;
   `internal/app` constructs the bot and passes it to `web`. Net result: **no
   `web -> bot` import edge** — the T06 "first `web -> bot` dependency" collapses to
   runtime injection, and both packages still build and test in isolation.
2. **`bot` keeps a tiny member-name cache** instead of enabling `discordgo` state.
   A `map[string]string` (id -> name), mutex-guarded, filled lazily with **one REST
   `GuildMember` call per unknown id**, ~1 h TTL. `s.StateEnabled` stays `false`
   (T02 — keeps RAM in the tens of MB). `MemberName` returns `(id, false)` on a
   miss, so `/api/bet` uses its T06-specified raw-id fallback.

### `web/` — location, `go:embed` point, dev vs prod

- The Vite project lives at **`web/`** (own `package.json`, `node_modules`, `src/`).
- `go:embed` can't reach outside its package dir, so **Vite `build.outDir` is set
  to `../internal/web/dist/`** (gitignored). `internal/web/embed.go` carries
  `//go:embed all:dist` and serves it via `http.FileServerFS` with an SPA fallback
  to `index.html`.
- **Dev workflow: `vite build --watch`** in a second terminal — each `.tsx` change
  re-emits `internal/web/dist/` in ~1–2 s, refresh the page manually. **No dev
  proxy, no `dev` build tag** (rejected as an unneeded code path for three
  utilitarian, data-rendering views).
- **Production is identical regardless** — the normal build embeds `dist/`; the
  Docker image has **zero Node runtime dependency**.

### HTTP router

**Stdlib `net/http.ServeMux`** (Go 1.22+ patterns). `GET /api/manager/{entryId}`
and the other three T06 routes + the SPA handler are all expressible; a
hand-written logging + panic-recover wrapper covers the only middleware need. No
`chi` — not worth a dependency on the 256 MB box.

### Test layout & build order

- Standard `_test.go` alongside each package.
- **`internal/fpl/testdata/*.json`** — captured real Draft API responses — drive
  `ApplyAutoSubs` table tests (the T01 §7 mis-scoring fix) and one test per derived
  view. `internal/bet` gets rule tests (`in`/`provisionallyOut`/`bust`/`leader`,
  sort). `internal/store` gets a migration + round-trip test against a temp-file
  DB. `bot` / `web` get thin handler tests with a fake `*fpl.Snapshot` and a fake
  `MemberNamer`.
- **Recommended build order for the weekend session:**
  `config` -> `store` -> `fpl` (with fixtures) -> `bet` -> `web` -> `bot` -> `app`.
  `fpl` is the keystone — nothing downstream is testable with real data until the
  snapshot + fixtures exist.

### Cross-references

- T02 — `discordgo`, `StateEnabled=false`, `BulkOverwrite` on READY, `time.Timer`
  reminder: all placed in `internal/bot`; `s.Open()` composing with `http.Server`
  is the `internal/app` wiring.
- T05 — `internal/{fpl,store,bet}` and the boot sequence are lifted verbatim into
  the table above; `internal/app` owns the numbered steps.
- T06 — the `web -> bot` edge is resolved here (consumer-side interface + REST
  member cache); `/api/*` routing is stdlib `ServeMux`.
- T07 — `internal/config` is the `config.Load()` home; `.env.example` sits at repo
  root.

### Graduated from fog

- **Deployment & build spec** — now unblocked (it was explicitly gated on T08 for
  the tree + `web/` location). Graduated into
  [T09 — Deployment & build spec](09-deployment-build-spec.md): multi-stage
  Dockerfile (Node builds `web/` into `internal/web/dist/` -> `go build` -> distroless),
  `fly.toml` (T07 `[env]`, `/data` volume, `internal_port` = `PORT`, legacy-free-tier
  sizing), health check, volume creation, deploy runbook.

## Comments

_(none)_
