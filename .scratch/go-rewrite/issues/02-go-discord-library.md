# T02 — Go Discord library choice

Parent: [Wayfinder map: Go rewrite](../map.md)
Type: research
Status: resolved
Blocked by: —

## Question

Which Go library should the rewrite use for the Discord side — gateway connection
plus **application (slash) commands** — running on one goroutine inside the single
binary?

Compare at least `bwmarrin/discordgo` and `disgoorg/disgo`. For each:

1. **Slash-command ergonomics.** How are commands defined, registered (global vs
   guild), and routed to handlers? How painful is the options/parameters API compared
   to `py-cord`'s `Option(...)` decorators the Python bot uses?
2. **Command registration model.** Does it diff/sync registered commands on startup,
   or must that be managed by hand? The Python bot registers 13 commands via
   `client.application_command(...)`.
3. **Embeds & responses.** Building `Embed`s, deferring long responses
   (`ctx.defer()` / followups — the Python `bet` command needs this), ephemeral
   replies, editing responses.
4. **Maintenance & adoption.** Release cadence, last commit, open-issue health,
   community size, breaking-change history.
5. **Footprint.** Dependency weight and rough memory profile — matters on fly.io
   legacy free tier.
6. **Background tasks.** Fit for the daily waiver-reminder loop
   (`draft/cogs/waiverstasks.py`) — just a `time.Ticker` goroutine, or does the lib
   offer scheduling helpers?

Give a recommendation with the trade-off stated plainly. If a third library is
clearly better, include it.

Capture findings as a Markdown file at
`.scratch/go-rewrite/research/02-go-discord-library.md` with a minimal slash-command
"hello world" sketch for the recommended library.

## Answer

**Use `bwmarrin/discordgo`.** Full findings:
[research/02-go-discord-library.md](../research/02-go-discord-library.md).

- **discordgo** — 1 dependency (`gorilla/websocket`), leanest binary/RAM for the
  256 MB fly box, builds on any Go toolchain, near-zero upgrade churn, by far the
  biggest example/Q&A ecosystem. Cost: no built-in command router (write a
  `map[string]handlerFunc` + one dispatcher — trivial at ~13 commands), plain embed
  structs (no builder), manual option access (`opt.StringValue()` etc.).
- **disgo** — nicer DX (fluent `EmbedBuilder`, `handler.Mux` router with middleware +
  path-style subcommand routing, typed `data.String("x")`, `handler.SyncCommands`
  helper, fastest new-feature support). Cost: ~8 deps incl. voice-E2EE dead weight,
  needs Go 1.24+, small community, active pre-v1 breaking-change history (v0.19.0
  Feb 2026 was large). The most py-cord-like option if DX is valued over leanness.
- **arikawa** — credible (`cmdroute` router, tiny deps) but smallest community +
  effectively maintenance mode. Not recommended here.

Key facts that apply whichever library:
- **No library diffs commands locally** — all rely on Discord's server-side
  bulk-overwrite (`ApplicationCommandBulkOverwrite`), one HTTP call on READY; gate
  global-vs-guild on a `DEV_GUILD_ID` env var.
- **No library has a scheduler** — the daily waiver-reminder loop is a hand-rolled
  goroutine using `time.Timer` recomputed each iteration (not `time.Ticker`) so it
  doesn't drift and handles DST. Add `robfig/cron/v3` only if cron-expression config
  is wanted later.
- discordgo: set `s.StateEnabled = false` and request only
  `IntentsGuilds | IntentsGuildMessages` to keep RAM in the tens of MB.
- `s.Open()` returns once connected (gateway runs on background goroutines), so it
  composes cleanly with an `http.Server` in the same process — start the HTTP server
  before blocking on the signal channel.
- All three are CGO-free single binaries → compose fine with `modernc.org/sqlite` +
  `go:embed`.

A minimal discordgo hello-world (gateway goroutine + bulk-sync on READY +
name-routed handler + deferred/ephemeral/response-edit flow for `bet` + daily
reminder goroutine) is in the research file.

New config keys implied: `DISCORD_TOKEN`, `DEV_GUILD_ID` (optional),
`REMINDER_CHANNEL_ID` — folded into the "Config & secrets inventory" fog.
