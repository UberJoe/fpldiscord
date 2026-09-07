# T02 — Go Discord library choice — findings

Parent: [Wayfinder map: Go rewrite](../map.md)
Ticket: [issues/02-go-discord-library.md](../issues/02-go-discord-library.md)
Researched: 2026-09-06
Primary sources: GitHub repos, `pkg.go.dev`, each project's `_examples/` and wiki.

## TL;DR

Use **`bwmarrin/discordgo`**. It is the boring, ubiquitous choice: one external
dependency, a stable API that almost never breaks on upgrade, builds on any Go
toolchain, and a huge body of examples/tutorials/StackOverflow answers. For a
13-command bot with one daily loop, the "manual" parts (a handler map, one
`ApplicationCommandBulkOverwrite` call on boot) are ~40 lines of boilerplate you
write once.

Pick **`disgoorg/disgo`** instead only if the team actively wants the nicer
developer experience: fluent `EmbedBuilder`/`MessageCreateBuilder`, a real
`handler` router with path-style routing + middleware, and a `handler.SyncCommands`
helper. The cost is a heavier dependency tree, a Go 1.24+ requirement, a much
smaller community, and a live-until-v1 breaking-change history (the v0.19.0 release
in Feb 2026 was large).

`diamondburned/arikawa` is a credible third option (clean design, `cmdroute`
router, tiny dep set) but has the smallest community of the three and slower
maintenance; no reason to prefer it here.

Neither slash-command ergonomics story in Go gets close to py-cord's decorator +
type-annotation terseness — Go has no decorators, so all three libraries make you
declare command schemas as data separately from handler funcs.

---

## Candidate summary

| | discordgo | disgo | arikawa |
|---|---|---|---|
| Repo | `bwmarrin/discordgo` | `disgoorg/disgo` | `diamondburned/arikawa` (v3) |
| Stars | ~6.0k | ~605 | ~600 |
| Latest release | v0.29.0 (2025-05-24) | v0.19.6 (2026-06-07) | v3.6.0 |
| Last commit (at research) | 2026-02-14 | 2026-09-05 | 2026-05-18 (mostly dependabot) |
| Open issues | ~158 | ~3 | ~29 |
| `go.mod` go version | 1.13 | 1.24.0 | 1.25.0 |
| Direct deps | 1 (`gorilla/websocket`) | ~8 | 4 |
| Versioning | v0.x, pre-1.0 but very stable | v0.x, "mostly stable, small breaks until v1" | v3.x, stable within v3 |
| Command router in core | no (write your own map) | yes — `handler` package | yes — `cmdroute` package |
| License | BSD-3 | Apache-2.0 | ISC |

---

## 1. Slash-command ergonomics

### py-cord baseline (what we're replacing)

```python
@bot.slash_command(description="Show a manager's squad")
async def team(ctx, manager: Option(str, "manager name", required=True)):
    await ctx.respond(f"...{manager}...")
```

Decorator registers it, the function signature *is* the schema, type annotations +
`Option(...)` drive parsing, and py-cord auto-syncs on connect. Very terse. No Go
library matches this because Go has no decorators or runtime signature
introspection worth using.

### discordgo

Command schema and handler are separate. Schema is a plain struct tree:

```go
var commands = []*discordgo.ApplicationCommand{
    {
        Name:        "team",
        Description: "Show a manager's squad",
        Options: []*discordgo.ApplicationCommandOption{
            {
                Type:        discordgo.ApplicationCommandOptionString,
                Name:        "manager",
                Description: "manager name",
                Required:    true,
            },
        },
    },
}
```

Routing is a name→func map plus one dispatcher you register with `AddHandler`:

```go
var handlers = map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate){
    "team": teamHandler,
}
s.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
    if h, ok := handlers[i.ApplicationCommandData().Name]; ok { h(s, i) }
})
```

Reading options inside the handler is manual: iterate
`i.ApplicationCommandData().Options`, or build a
`map[string]*ApplicationCommandInteractionDataOption` first, then `opt.StringValue()`
/ `opt.IntValue()` / `opt.BoolValue()` / `opt.UserValue(s)`. Subcommands are nested
option trees you walk yourself. Verbose but completely explicit; the pattern is in
`examples/slash_commands/main.go` and copied across the ecosystem.

### disgo

Schema uses typed constructors instead of a tagged-union struct, which is easier to
get right (no "set `Type` to the matching enum" foot-gun):

```go
var commands = []discord.ApplicationCommandCreate{
    discord.SlashCommandCreate{
        Name:        "team",
        Description: "Show a manager's squad",
        Options: []discord.ApplicationCommandOption{
            discord.ApplicationCommandOptionString{
                Name: "manager", Description: "manager name", Required: true,
            },
        },
    },
}
```

Two routing styles:

* **Plain event listener** — `bot.WithEventListenerFunc(func(e *events.ApplicationCommandInteractionCreate){...})`, then `switch e.SlashCommandInteractionData().CommandName()`. Option access is typed and terse: `data.String("manager")`, `data.OptBool("ephemeral")`.
* **`handler` package (recommended for 13 commands)** — a `handler.Mux` router:

  ```go
  r := handler.New()
  r.SlashCommand("/team", teamHandler)
  r.SlashCommand("/bet/place", betPlaceHandler) // subcommand routing by path
  r.Use(middleware.Logger)
  client, _ := disgo.New(token, bot.WithEventListeners(r))
  ```

  Handlers get `(data discord.SlashCommandInteractionData, e *handler.CommandEvent)`.
  Path-style patterns cover subcommands and component/modal custom-IDs, and
  middleware (`Use`/`With`) works like `chi`/`net/http`.

### arikawa

`cmdroute.Router` with `AddFunc("team", handler)`; command data defined as
`api.CreateCommandData`. Handlers return `*api.InteractionResponseData` (or nil),
which is a clean functional style. Option parsing via
`cmdroute.Router` + `data.Options` helpers, roughly on par with discordgo.

**Verdict:** disgo's `handler` package is the closest thing to py-cord's ergonomics
and the least error-prone options API. discordgo is the most manual but the pattern
is well-trodden and 13 commands is not a lot of boilerplate.

---

## 2. Command registration / sync model

**None of the three do a local hash-diff to skip the sync call.** All three rely on
Discord's server-side bulk-overwrite endpoint, which itself diffs: you send the full
desired set, Discord adds/updates/removes to match, and unchanged commands are not
touched or rate-limited. For 13 commands that is one HTTP call on boot.

* **discordgo:** `s.ApplicationCommandBulkOverwrite(appID, guildID, commands)`.
  `guildID == ""` → global (propagation historically up to ~1h, usually fast now);
  `guildID` set → that guild, instant (use for dev). The `examples/` show a
  one-by-one `ApplicationCommandCreate` loop; ignore that and use bulk-overwrite —
  it is the de-facto "sync". You call it yourself from your startup code.
* **disgo:** `client.Rest.SetGlobalCommands(appID, commands)` /
  `SetGuildCommands(appID, guildID, commands)`, or the helper
  `handler.SyncCommands(client, commands, guildIDs)` — "sets the given commands for
  the given guilds, or globally if `guildIDs` is empty", looping guilds
  sequentially. Slightly more batteries-included but functionally identical.
* **arikawa:** `s.BulkOverwriteCommands` / `cmdroute` `OverwriteCommands`.

For this bot: register a `[]ApplicationCommand` slice as data, call bulk-overwrite
once after the gateway is READY, gate global-vs-guild on an env var
(`DEV_GUILD_ID`). Same amount of work in any of the three.

---

## 3. Embeds & responses

### discordgo

* **Embeds:** plain struct, no builder.
  ```go
  e := &discordgo.MessageEmbed{
      Title:       "Standings",
      Description: "...",
      Color:       0x00ff88,
      Fields: []*discordgo.MessageEmbedField{
          {Name: "1. Alice", Value: "82 pts", Inline: true},
      },
  }
  ```
* **Immediate reply:** `s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{Type: discordgo.InteractionResponseChannelMessageWithSource, Data: &discordgo.InteractionResponseData{Embeds: []*discordgo.MessageEmbed{e}}})`.
* **Defer (the `bet` command):** respond with
  `InteractionResponseDeferredChannelMessageWithSource`, do the slow work, then
  either `s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{...})` to
  fill in the "thinking" message, or `s.FollowupMessageCreate(i.Interaction, true,
  &discordgo.WebhookParams{...})` for additional messages.
* **Ephemeral:** `Flags: discordgo.MessageFlagsEphemeral` on the response data (also
  works on a deferred response to make the eventual message ephemeral).
* **Edit / delete:** `InteractionResponseEdit`, `InteractionResponseDelete`,
  `FollowupMessageEdit/Delete`.

### disgo

* **Embeds:** fluent builder —
  `discord.NewEmbedBuilder().SetTitle("Standings").SetColor(0x00ff88).AddField("1. Alice", "82 pts", true).Build()`.
* **Reply:** `event.CreateMessage(discord.NewMessageCreateBuilder().SetEmbeds(e).SetEphemeral(true).Build())`.
* **Defer:** `event.DeferCreateMessage(ephemeralBool)`, then
  `event.CreateFollowupMessage(...)` or
  `client.Rest.UpdateInteractionResponse(appID, token, discord.NewMessageUpdateBuilder()...)`.
* **Edit / delete:** `event.UpdateInteractionResponse`, `DeleteInteractionResponse`,
  followup edit/delete equivalents.

The builder pattern is the main day-to-day quality-of-life win over discordgo.

### arikawa

Plain `discord.Embed` structs (like discordgo). Deferral via
`s.RespondInteraction` with `DeferredMessageInteractionWithSource` then
`s.EditInteractionResponse`; ephemeral via `discord.EphemeralMessage` flag; with
`cmdroute` you can also just return `Ephemeral: true` in the response data.

**Verdict:** functionally equal. disgo's builders make embed-heavy code (this bot is
embed-heavy) noticeably tidier; discordgo/arikawa struct literals are fine and
arguably easier to `diff` in review.

---

## 4. Maintenance & adoption

### discordgo

* ~6.0k stars, by far the largest Go Discord community; most blog posts, tutorials,
  and Q&A target it.
* Releases are infrequent and chunky: v0.27 (2023), v0.28 (2024-04), v0.29
  (2025-05). Between releases, `master` is generally usable.
* Last commit around 2026-02; development is slow but not dead — maintainers land
  API-coverage PRs and security/handshake fixes (e.g. the March 2023 voice
  handshake emergency releases).
* ~158 open issues, ~73 open PRs — a real backlog, typical of a large old project.
* Breaking-change history is mild: v0.28 turned several struct fields into pointers;
  v0.29 changed two field types. Upgrades are usually a recompile plus a few
  mechanical fixes. Because it is pre-1.0 there is no SemVer guarantee, but in
  practice it is one of the more stable libraries in the ecosystem.

### disgo

* ~605 stars. Small but genuinely active community (Discord Gophers). Maintainer
  (`disgoorg`/`sebm253`) ships frequently — releases roughly monthly through 2026
  (v0.19.1–v0.19.6 Feb–Jun 2026), last commit within a day of this research.
* Tracks new Discord features fast (Components V2, DAVE E2EE voice, polls,
  entitlements, new component types) — often ahead of discordgo.
* ~3 open issues — either very healthy or low reporting volume; PR count is modest.
* Breaking changes are explicit and not rare while pre-v1. v0.19.0 (2026-02-12) was
  a big one: `Client` became a struct, global builders/command helpers removed, JSON
  library swapped, cache `ForEach` replaced with `iter.Seq`, REST errors became
  pointers. The changelog is good and migrations are documented, but you should
  expect to do real work on major-ish bumps until v1.

### arikawa

* ~600 stars. Design-led (clean API/gateway split, pluggable cache). v3 API is
  stable. Recent commit history is mostly dependabot bumps — maintenance mode more
  than active development. ~29 open issues. Fine to depend on, smallest support
  network of the three.

---

## 5. Footprint

Target: fly.io legacy free tier (`shared-cpu-1x`, 256 MB RAM), single static binary
that also runs the HTTP server + embedded React app.

* **discordgo:** one direct dependency, `github.com/gorilla/websocket`. Smallest
  build (bot layer adds ~a few MB to the binary). Idle RAM is low; the in-memory
  `State` cache is the main variable — set `dg.State.TrackMembers/TrackPresences =
  false` (or `dg.StateEnabled = false`) and request only the intents you need
  (`IntentsGuilds | IntentsGuildMessages`) and it sits comfortably in tens of MB.
  Best fit for a 256 MB box.
* **disgo:** ~8 direct deps incl. `klauspost/compress` (zstd gateway transport),
  `golang.org/x/crypto`, and `disgoorg/godave` (voice E2EE — dead weight for this
  bot but in the module graph). Binary is somewhat larger (~mid-teens MB). Caches
  are per-entity configurable and can be disabled (`cache.FlagsNone`). Still fits
  256 MB fine; just not as lean, and pulls more supply-chain surface.
* **arikawa:** 4 deps, no compression/crypto-heavy extras beyond `x/crypto`.
  Roughly discordgo-class footprint.

All three cross-compile to a single CGO-free binary, so they compose cleanly with
`modernc.org/sqlite` (pure Go) and `go:embed`. Go toolchain note: disgo needs Go
1.24+, arikawa Go 1.25+, discordgo builds on anything — you control the Dockerfile
so this is minor, but discordgo has zero toolchain friction.

---

## 6. Background tasks (daily waiver-reminder loop)

`draft/cogs/waiverstasks.py` is a `tasks.loop` that fires once a day. **No Go
Discord library ships a scheduler** — this is out of their scope by design.

The idiom in all three is a goroutine you start after the gateway is ready:

```go
func startWaiverReminder(ctx context.Context, s *discordgo.Session, channelID string) {
    go func() {
        for {
            next := nextRunAt(11, 0) // 11:00 local, tomorrow if already past
            timer := time.NewTimer(time.Until(next))
            select {
            case <-ctx.Done():
                timer.Stop()
                return
            case <-timer.C:
                sendWaiverReminder(s, channelID)
            }
        }
    }()
}
```

Use `time.Timer` recomputed each iteration (not a fixed `time.Ticker`) so the fire
time doesn't drift and DST is handled. If you want cron-expression config later, add
`github.com/robfig/cron/v3` — one small, stable dependency — but for a single daily
reminder the hand-rolled timer above is enough and has no deps. This is identical
work regardless of which Discord library you choose.

---

## Recommendation

**Go with `bwmarrin/discordgo`.**

Reasoning for this specific bot:

* **Smallest footprint / least supply chain** — one dependency, leanest binary,
  easiest to keep a 256 MB fly machine happy.
* **Stability & upgrade cost** — breaking changes are rare and mechanical; you will
  not be doing migration work mid-rewrite or on a lazy maintenance schedule later.
* **Ecosystem** — the most examples, tutorials, and answered questions by a wide
  margin; matters for a solo/weekend build and future you.
* **The gaps are small at this size** — 13 commands means the handler map +
  `ApplicationCommandBulkOverwrite` boilerplate is trivial and lives in one file;
  plain embed structs are fine (and review-friendly); the daily loop is a goroutine
  either way.

**The trade-off, plainly:** you give up disgo's nicer DX — fluent
embed/message builders, a real routing package with middleware, typed
`data.String("x")` option access, and faster support for brand-new Discord
features. If the team values that polish more than the smaller dep tree and the
larger community, disgo is a reasonable pick and its `handler` + `SyncCommands`
combo is the most py-cord-like option; just budget for occasional breaking bumps
until it reaches v1 and require a modern Go toolchain in the build image.

`arikawa` is not recommended here: it splits the difference on ergonomics and
footprint but has the thinnest community and is effectively in maintenance mode.

---

## Hello-world sketch — discordgo

Minimal single-file bot: connects the gateway on a goroutine, bulk-syncs one slash
command with a string option on READY, routes it, and demonstrates a deferred +
ephemeral followup (the pattern the `bet` command needs). HTTP server / SQLite /
embed are elsewhere in the real binary.

```go
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"
)

// --- command schema (data) ---------------------------------------------------

var commands = []*discordgo.ApplicationCommand{
	{
		Name:        "standings",
		Description: "Show the current league table",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "week",
				Description: "gameweek number (default: latest)",
				Required:    false,
			},
		},
	},
}

// --- handlers --------------------------------------------------------------

var handlers = map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate){
	"standings": handleStandings,
}

func handleStandings(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// options -> map for easy lookup
	opts := map[string]*discordgo.ApplicationCommandInteractionDataOption{}
	for _, o := range i.ApplicationCommandData().Options {
		opts[o.Name] = o
	}
	week := "latest"
	if o, ok := opts["week"]; ok {
		week = o.StringValue()
	}

	// defer: tells Discord "thinking...", buys ~15 min; Ephemeral => only caller sees it
	if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{Flags: discordgo.MessageFlagsEphemeral},
	}); err != nil {
		log.Printf("defer failed: %v", err)
		return
	}

	// ... slow work here (FPL API call, table build) ...
	time.Sleep(500 * time.Millisecond)

	embed := &discordgo.MessageEmbed{
		Title:       "League table — GW " + week,
		Color:       0x00ff88,
		Description: "1. Alice — 82\n2. Bob — 79\n3. Carol — 75",
	}

	// fill in the deferred response
	if _, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Embeds: &[]*discordgo.MessageEmbed{embed},
	}); err != nil {
		log.Printf("response edit failed: %v", err)
	}
}

// --- wiring --------------------------------------------------------------

func main() {
	token := os.Getenv("DISCORD_TOKEN")
	devGuild := os.Getenv("DEV_GUILD_ID") // "" => register globally

	s, err := discordgo.New("Bot " + token)
	if err != nil {
		log.Fatal(err)
	}
	s.Identify.Intents = discordgo.IntentsGuilds | discordgo.IntentsGuildMessages
	s.StateEnabled = false // trim memory: no member/presence cache

	s.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		if i.Type != discordgo.InteractionApplicationCommand {
			return
		}
		if h, ok := handlers[i.ApplicationCommandData().Name]; ok {
			h(s, i)
		}
	})

	s.AddHandler(func(s *discordgo.Session, r *discordgo.Ready) {
		// one bulk-overwrite call = "sync"; Discord diffs server-side.
		if _, err := s.ApplicationCommandBulkOverwrite(r.User.ID, devGuild, commands); err != nil {
			log.Fatalf("command sync failed: %v", err)
		}
		log.Printf("synced %d commands (guild=%q)", len(commands), devGuild)
	})

	if err := s.Open(); err != nil { // gateway runs on its own goroutines
		log.Fatal(err)
	}
	defer s.Close()

	// daily reminder loop — plain timer, no library helper
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go dailyReminder(ctx, s, os.Getenv("REMINDER_CHANNEL_ID"))

	log.Println("running; Ctrl-C to stop")
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
}

func dailyReminder(ctx context.Context, s *discordgo.Session, channelID string) {
	for {
		timer := time.NewTimer(time.Until(nextRunAt(11, 0)))
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
			if channelID != "" {
				_, _ = s.ChannelMessageSend(channelID, "Waiver reminder: claims lock tonight.")
			}
		}
	}
}

func nextRunAt(hour, min int) time.Time {
	now := time.Now()
	next := time.Date(now.Year(), now.Month(), now.Day(), hour, min, 0, 0, now.Location())
	if !next.After(now) {
		next = next.Add(24 * time.Hour)
	}
	return next
}
```

Notes:

* `s.Open()` returns once connected; discordgo services the gateway on background
  goroutines, so this composes with an `http.Server` in the same process — start the
  HTTP server before the `<-stop` block.
* `WebhookEdit.Embeds` is `*[]*MessageEmbed` (pointer to slice) as of v0.28+ — the
  pointer-field change mentioned above.
* For non-deferred fast commands, reply directly with
  `InteractionResponseChannelMessageWithSource` instead of the defer/edit dance.
