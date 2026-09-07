# T01 — Discord embeds for tabular / structured data — findings

Researched: 2026-09-07
Primary sources: Discord developer docs (`docs.discord.com/developers/...`, formerly
`discord.com/developers/docs/...`), `discordgo` source on
`raw.githubusercontent.com/bwmarrin/discordgo/master/*.go` and `pkg.go.dev`,
Discord `discord-api-docs` GitHub issues/discussions, the widely-cited
`kkrypt0nn` ANSI-on-Discord gist. Client-behaviour claims that are not in the docs
are flagged as **observed**.

## TL;DR

- **Markdown tables do not render in Discord** — not in message content, not in an
  embed `description`, on neither desktop nor mobile. Pipes come out as literal
  text. It is still an open feature request. Do not build on them.
  (`github.com/discord/discord-api-docs` Discussion #4098;
  `support.discord.com/hc/en-us/community/posts/16131946321815`.)
- The **only reliable aligned-columns primitive in an embed is a fenced code
  block inside `description`** (max 4096 chars). Code blocks render in a
  fixed-width font on every client, so `text/tabwriter` output stays aligned;
  outside a code block Discord uses a proportional font and the padding collapses.
  So: keep the current `tabwriter` table, just move it into an embed `description`
  and gain a colour bar, title, footer and "last updated" timestamp.
- **`fields`** are a good pseudo-table only for a *small* fixed set of cells:
  25-field hard cap, and **inline fields do not sit side-by-side on mobile**
  (they stack to one column — observed). Column-per-field works for ≤3 short
  columns; row-per-field (one non-inline field per fixture) is the right shape
  for `/overview`.
- **`ansi` code blocks** give 8 fg / 8 bg colours on desktop + web only; mobile
  shows a plain uncoloured block (observed, historically; some 2025–26 patches
  narrow the gap but it is still inconsistent). Nice-to-have for
  up/down/provisional cues, not worth depending on.
- **Limits that matter:** description 4096, field value 1024, 25 fields, **6000
  chars total across all embeds in a message**, **max 10 embeds per message**.
- **discordgo** has no embed builder — plain `&discordgo.MessageEmbed{...}`
  structs. `Color` is a plain `int` (decimal RGB); `Timestamp` is an ISO-8601
  `string` (use `time.Now().Format(time.RFC3339)`); no helper.
- Recommendation in one line each:
  - **/standings** → one embed, `tabwriter` table in `description` code block,
    colour bar + footer timestamp.
  - **/scores** → one embed, same code-block table, `Color` encodes
    provisional (amber) vs final (green), phase tag + timestamp in the footer.
  - **/overview** → one embed per message, **one non-inline `field` per fixture**
    (name = score line, value = scorer lines); spill to more messages past 25
    fixtures instead of the current char-count pagination.

---

## 1. Embed object structure & limits

### 1.1 Structure (Discord docs — Embed Object)

Source: `docs.discord.com/developers/resources/message` ("Embed Object").

| Field | Type | Notes |
|---|---|---|
| `title` | string | |
| `type` | string | "always `rich` for webhook [and bot] embeds" |
| `description` | string | markdown is rendered here |
| `url` | string | makes the title a link |
| `timestamp` | ISO-8601 timestamp | rendered in the footer line, localised by the client |
| `color` | integer | "color code of the embed" — decimal RGB |
| `footer` | footer object | `text`, `icon_url` |
| `image` | image object | large image at the bottom |
| `thumbnail` | image object | small image, top-right |
| `video`, `provider` | objects | set by Discord for link unfurls, not by bots |
| `author` | author object | `name`, `url`, `icon_url` — small line above the title |
| `fields` | array of field objects | "max of 25"; each has `name`, `value`, `inline` |
| `flags` | integer | bitfield |

`color` is a decimal integer, i.e. the 24-bit RGB value. Hex → decimal:
`0x5865F2` = `5793266` (Discord blurple). Examples from
`discord-webhook.com/en/blog/discord-embed-builder-guide/`: red `15548997`,
green `5763719`, blue `3447003`, blurple `5793266`. In Go just write the hex
literal: `Color: 0x5865F2`.

`timestamp` — "timestamp of embed content", ISO-8601
(`docs.discord.com/developers/resources/message`). The client formats it to the
viewer's locale/timezone and shows it joined to the footer text with a `•`
(observed; `discord-webhook.com/.../discord-embed-builder-guide/`:
"Discord automatically formats the timestamp based on the user's locale and
timezone").

### 1.2 Limits (Discord docs — "Embed Limits")

Source: `docs.discord.com/developers/resources/message` ("Embed Limits").

| Thing | Limit |
|---|---|
| `title` | 256 characters |
| `description` | 4096 characters |
| `fields` | up to 25 field objects |
| `field.name` | 256 characters |
| `field.value` | 1024 characters |
| `footer.text` | 2048 characters |
| `author.name` | 256 characters |
| **Combined** `title` + `description` + `field.name` + `field.value` + `footer.text` + `author.name`, **across all embeds on a message** | **6000 characters** |

Max embeds per message: **10** — "array of up to 10 embed objects"
(`docs.discord.com/developers/resources/webhook`, Execute Webhook params; the
Create Message endpoint carries the same "up to 10" wording). A webhook/bot
message must supply at least one of `content`, `embeds`, `components`, `file`,
`poll` (same source).

Practical read for an FPL league (~12–24 managers): a 24-row `tabwriter` table at
~34 chars/line is ~820 chars — one `description` holds it ~5× over, and the 6000
total is a non-issue unless you attach several big embeds at once.

### 1.3 `inline` field layout

Docs say only that fields exist and `inline` is a bool
(`docs.discord.com/developers/resources/message`). Layout is **observed
client behaviour**, corroborated across:
`discord-webhook.com/en/blog/discord-embed-builder-guide/`,
`github.com/discord/discord-api-docs/issues/1128`,
`github.com/discord/discord-api-docs/discussions/3233`.

- **Desktop / web:** consecutive `inline: true` fields pack **up to 3 per row**;
  the 4th starts a new row, then groups of 3 after that.
- An `inline: false` (full-width) field **forces a row break**: fields after it
  start on a fresh row regardless of their own `inline` value.
- **Uneven counts:** a trailing row of 1 or 2 inline fields stretches those
  fields wider to fill the embed width (so a 4-field block renders 3 + 1, the
  lone field spanning full width). There is no way to pin a field to a column or
  force 2-per-row (open request: Discussion #3233).
- **Mobile (iOS/Android):** inline fields historically **do not** render
  side-by-side — every field takes its own full-width line, i.e. a single column
  (`github.com/discord/discord-api-docs/issues/1128`;
  `support.discord.com/hc/en-us/community/posts/1500001139621`). Treat any
  multi-column `fields` layout as "desktop-only nicety, degrades to a list on
  phones."
- **Blank spacer field:** `name` and `value` must be non-empty; the convention is
  a zero-width space `"​"` as the value (or name) to pad a row to 3
  (`discord-webhook.com/.../discord-embed-builder-guide/`).

---

## 2. Rendering a data table in an embed — the real options

### 2.1 Markdown table syntax — NOT supported

Discord does **not** render GFM pipe tables anywhere: not in message content, not
in an embed `description`, on any platform, as of 2026-09. Pasting
`| a | b |` / `|---|---|` shows the literal pipes and dashes.

- "Discord has not added native markdown table support to the platform"
  (`mdtidy.com/blog/markdown-in-discord`, 2026 guide).
- "Discord supports a strict subset of GitHub Flavored Markdown … a few
  surprising omissions (no tables, no images via Markdown)" (same).
- Open feature requests, never shipped:
  `github.com/discord/discord-api-docs/discussions/4098` ("Tables"),
  `support.discord.com/hc/en-us/community/posts/16131946321815`
  ("Request for markdown tables").
- The only things that render GFM tables in a Discord message are third-party
  browser extensions (e.g. the "Discord Markdown Table Renderer" Chrome
  extension) — not the official clients.

Conclusion: **do not** switch the tables to markdown pipe syntax. It would look
worse than today.

### 2.2 Monospace code block inside `description` — the workable option

`description` renders markdown, including fenced code blocks
(`docs.discord.com/developers/resources/message` says description is markdown;
code-block support is universal Discord markdown,
`support.discord.com/hc/en-us/articles/210298617`).

- Inside a ```` ``` ```` fence the client uses a **fixed-width font on every
  platform**, so space-padded columns from `text/tabwriter` line up. This is
  exactly today's `renderStandings` / `renderScores` output — it already aligns;
  the only change is to nest it in an embed.
- **Why proportional font breaks it:** outside a code block Discord renders text
  in its UI sans-serif (gg sans). `tabwriter` pads with U+0020 spaces computed
  for a monospace grid; in a proportional font a space is narrower than a digit
  and every glyph differs, so the columns drift. That is the user's "spacing is
  not good" — any non-code-block attempt to align with spaces will look wrong.
- **Mobile:** code blocks render with a smaller monospace face and, for lines
  wider than the viewport, either wrap or scroll horizontally (observed;
  `markdowntools.io/discord-code-block`). Keep lines **short** — aim ≤ ~34–38
  chars — by truncating long team names (e.g. to 12–14 chars + "…") and
  right-aligning the numeric columns so they stay narrow.
- **Caveat:** a code block disables other markdown inside it (no bold, no colour
  unless it is an `ansi` block). The colour/emphasis has to come from the embed
  chrome (colour bar, title, footer) instead.
- Fits the limits comfortably: `description` 4096 vs ~800 chars for a 24-row
  table.

### 2.3 `fields` as a pseudo-table

Two patterns people use (both observed in the wild; layout mechanics per §1.3):

- **Column-per-field:** one field per column, `value` = all cells in that column
  joined by `\n`, `inline: true`. E.g. three fields `#` / `Team` / `Pts`. Rows
  stay visually aligned because each field is its own block. Works well for **2–3
  narrow columns on desktop**; on mobile the three columns stack, so you get
  "all ranks, then all teams, then all points" — readable but not a table.
  1024-char `value` cap ⇒ ~40–60 rows per column max.
- **Row-per-field:** one field per row/record, `inline: false`, `name` = the
  headline (e.g. a fixture score), `value` = the detail (scorer lines). This is
  not a grid, it is a titled list — the natural fit for `/overview`. 25-field cap
  = 25 records per embed.
- Column-per-field for a 4-column, 24-row standings table would need either 4
  fields with long stacked values (loses per-row alignment between columns
  because field blocks don't share a baseline) or 96 fields (way over 25). Not
  worth it — §2.2 wins for `/standings` and `/scores`.

### 2.4 `ansi` code blocks

Source: `gist.github.com/kkrypt0nn/a02506f3712ff2d1c8ca7c9e0aed7c06`
("A guide to ANSI on Discord").

- Syntax: a fence tagged `ansi`, then SGR sequences with the real ESC char
  (U+001B): prefix `[{format};{color}m`, reset with `[0m`.
- **Format codes:** `0` normal, `1` bold, `4` underline.
- **Foreground 30–37:** 30 gray, 31 red, 32 green, 33 yellow, 34 blue, 35 pink,
  36 cyan, 37 white. **Background 40–47:** 40 dark/"firefly", 41 orange/red,
  42–47 various grays/blue/white (palette is theme-dependent and approximate).
- 8 colours only; no 256-colour, no hex, no truecolor.
- **Client support:** "brought to all stable desktop clients … ANSI is not
  supported on mobile" (kkrypt0nn gist). Mobile shows a plain uncoloured code
  block. Some Discord patch notes in late-2025/2026 touch mobile code-block
  rendering but multiple 2026 write-ups still say mobile ANSI is
  inconsistent/absent (`ultratextgen.com/guide/discord-colored-text-guide`,
  `support.discord.com/hc/en-us/community/posts/8128597744919`).
- Verdict: usable to tint a provisional table amber or colour movement arrows on
  desktop, but it must degrade gracefully to monochrome. Given the effort
  (embedding ESC bytes, per-cell sequences) vs the payoff, prefer using the
  **embed `Color` bar** for state and plain `▲`/`▼` glyphs for movement. Revisit
  `ansi` only if a desktop-only colour cue is specifically wanted.

---

## 3. discordgo specifics

All struct/const quotes below are verbatim from
`raw.githubusercontent.com/bwmarrin/discordgo/master/` at research time
(corresponds to released **v0.29.0**), cross-checked on
`pkg.go.dev/github.com/bwmarrin/discordgo`.

### 3.1 Embed structs — `message.go`

```go
type EmbedType string
const (
	EmbedTypeRich    EmbedType = "rich"
	EmbedTypeImage   EmbedType = "image"
	EmbedTypeVideo   EmbedType = "video"
	EmbedTypeGifv    EmbedType = "gifv"
	EmbedTypeArticle EmbedType = "article"
	EmbedTypeLink    EmbedType = "link"
)

type MessageEmbedFooter struct {
	Text         string `json:"text,omitempty"`
	IconURL      string `json:"icon_url,omitempty"`
	ProxyIconURL string `json:"proxy_icon_url,omitempty"`
}

type MessageEmbedImage struct {
	URL      string `json:"url"`
	ProxyURL string `json:"proxy_url,omitempty"`
	Width    int    `json:"width,omitempty"`
	Height   int    `json:"height,omitempty"`
}

type MessageEmbedThumbnail struct { // same shape as MessageEmbedImage
	URL      string `json:"url"`
	ProxyURL string `json:"proxy_url,omitempty"`
	Width    int    `json:"width,omitempty"`
	Height   int    `json:"height,omitempty"`
}

type MessageEmbedVideo struct {
	URL    string `json:"url,omitempty"`
	Width  int    `json:"width,omitempty"`
	Height int    `json:"height,omitempty"`
}

type MessageEmbedProvider struct {
	URL  string `json:"url,omitempty"`
	Name string `json:"name,omitempty"`
}

type MessageEmbedAuthor struct {
	URL          string `json:"url,omitempty"`
	Name         string `json:"name"`
	IconURL      string `json:"icon_url,omitempty"`
	ProxyIconURL string `json:"proxy_icon_url,omitempty"`
}

type MessageEmbedField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline,omitempty"`
}

type MessageEmbed struct {
	URL         string                 `json:"url,omitempty"`
	Type        EmbedType              `json:"type,omitempty"`
	Title       string                 `json:"title,omitempty"`
	Description string                 `json:"description,omitempty"`
	Timestamp   string                 `json:"timestamp,omitempty"`
	Color       int                    `json:"color,omitempty"`
	Footer      *MessageEmbedFooter    `json:"footer,omitempty"`
	Image       *MessageEmbedImage     `json:"image,omitempty"`
	Thumbnail   *MessageEmbedThumbnail `json:"thumbnail,omitempty"`
	Video       *MessageEmbedVideo     `json:"video,omitempty"`
	Provider    *MessageEmbedProvider  `json:"provider,omitempty"`
	Author      *MessageEmbedAuthor    `json:"author,omitempty"`
	Fields      []*MessageEmbedField   `json:"fields,omitempty"`
}
```

Notes:
- **No builder.** discordgo is plain struct literals (confirmed in the sibling
  note `../../go-rewrite/research/02-go-discord-library.md` §3: "Embeds: plain
  struct, no builder"). disgo has `NewEmbedBuilder()`; discordgo does not.
- **`Color` is `int`** — assign a hex literal directly: `Color: 0x3BA55D`.
- **`Timestamp` is a `string`** and must be RFC 3339 / ISO 8601. No helper in
  discordgo — write `Timestamp: time.Now().UTC().Format(time.RFC3339)` (or the
  snapshot's build time). An empty string omits it (`omitempty`).
- `Type` can be left zero — Discord treats bot/webhook embeds as `"rich"`
  regardless (`docs.discord.com/developers/resources/message`). Set
  `Type: discordgo.EmbedTypeRich` only if you want it explicit.
- There is a package-level `discordgo.MessageEmbed` size constant? No — there is
  no limit-checking helper; enforce §1.2 yourself before sending.

### 3.2 Sending an embed

Response containers — `interactions.go` / `webhook.go` (verbatim):

```go
type InteractionResponseData struct {
	// ...
	Content string          `json:"content"`
	Embeds  []*MessageEmbed  `json:"embeds"`
	Flags   MessageFlags     `json:"flags,omitempty"`
	// ...
}

type WebhookParams struct { // FollowupMessageCreate / ChannelMessageSendComplex path
	Content string          `json:"content,omitempty"`
	Embeds  []*MessageEmbed  `json:"embeds,omitempty"`
	// ...
}

type WebhookEdit struct {   // InteractionResponseEdit path
	Content *string          `json:"content,omitempty"`
	Embeds  *[]*MessageEmbed  `json:"embeds,omitempty"` // NOTE: pointer-to-slice
	// ...
}
```

`InteractionResponseType` constants (`interactions.go`):
`InteractionResponseChannelMessageWithSource = 4`,
`InteractionResponseDeferredChannelMessageWithSource = 5`, etc.

Immediate reply with an embed:

```go
s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
	Type: discordgo.InteractionResponseChannelMessageWithSource,
	Data: &discordgo.InteractionResponseData{
		Embeds: []*discordgo.MessageEmbed{embed},
		// Content: "optional text above the embed(s)",
		// Flags: discordgo.MessageFlagsEphemeral,
	},
})
```

Fill in a deferred ("thinking…") response — note `Embeds` is `*[]*MessageEmbed`:

```go
s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
	Embeds: &[]*discordgo.MessageEmbed{embed},
})
```

Follow-up message on the same interaction (this bot's multi-message path):

```go
s.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
	Embeds: []*discordgo.MessageEmbed{embed},
})
```

Plain channel message: `s.ChannelMessageSendEmbed(channelID, embed)` or
`s.ChannelMessageSendComplex(channelID, &discordgo.MessageSend{Embeds: ...,
Content: ...})`.

- **Multiple embeds in one message:** pass up to 10 in the slice
  (`Embeds: []*discordgo.MessageEmbed{e1, e2, e3}`); 11+ is rejected by Discord.
- **Embed + text:** set both `Content` and `Embeds` — the content renders above
  the embed stack.
- **Colour helper:** none. **Timestamp helper:** none — `time.RFC3339`.
- Relevant to this repo: `internal/bot/bot.go`'s `interactionResponder.Respond`
  only accepts a `string` and the `Responder` interface exposes just
  `Respond(string) error`. To send embeds you need a new method on that
  interface (e.g. `RespondEmbeds([]*discordgo.MessageEmbed) error`) threaded
  through the same first-call / deferred-edit / follow-up branching that
  `Respond` already implements (`internal/bot/bot.go:535`), and the test doubles
  (`dave_test.go`, `bet_test.go`) updated.

---

## 4. Design / best-practice guidance for readable embeds

- **One embed can hold a whole league table.** `description` 4096 chars ≫ any
  realistic FPL classic league in a code block. Don't paginate `/standings` or
  `/scores` unless the league exceeds ~90 rows.
- **Keep code-block lines short for mobile.** Truncate names, right-align
  numbers. Target ≤ ~34 chars/line so it doesn't horizontally scroll on a phone
  (observed; §2.2).
- **Use `Color` to signal state**, since a code block can't carry colour:
  - final / settled → green `0x3BA55D`
  - live / provisional → amber `0xFAA61A`
  - error / "not available" → red `0xED4245`
  - neutral / informational → blurple `0x5865F2`
  (Palette values are the commonly-used Discord brand colours;
  `discord-webhook.com/.../discord-embed-builder-guide/`.)
- **Footer + `Timestamp` for "last updated".** Put the data freshness there
  (`Footer.Text = "GW17 · provisional"`, `Timestamp = snapshot time`) instead of
  a bold header line. The client localises the timestamp for each viewer.
- **`Author` line** is a good place for the league name + a small icon; keeps the
  `Title` free for the table's subject ("Standings", "GW17 scores").
- **When to use multiple embeds vs paginate vs fields:**
  - Multiple embeds (≤10/msg) — when records are visually distinct cards
    (`/overview` fixtures). Watch the **6000-char message total**.
  - More messages — only when you'd blow 10 embeds or 6000 chars, or a single
    code block would exceed ~3500 chars (leave headroom under 4096).
  - `fields` — for ≤25 titled records, or ≤3 short desktop columns.
- **Stay well under 6000 total.** A standings code block (~800) + footer (~40) is
  trivial; the ceiling only bites if you stack many field-heavy embeds.
- **Don't mix an `ansi` block with reliance on colour** — design for the
  monochrome (mobile) case first, treat colour as enhancement.
- **Emoji as low-cost structure:** `▲`/`▼`/`▬` for movement, medal emoji for the
  top 3, a coloured circle (`🟢`/`🟡`) mirroring the embed `Color` so mobile users
  who don't see the bar still get the state cue.

---

## 5. Concrete recommendation per command

### 5.1 `/standings` — one embed, code-block table in `description`

Keep `renderStandings`'s `tabwriter` body; wrap it in an embed. Right-align the
numeric columns and cap the team name so lines stay phone-friendly. Move the
league name to `Author`, drop the bold header line.

```go
embed := &discordgo.MessageEmbed{
	Author: &discordgo.MessageEmbedAuthor{Name: snap.LeagueName},
	Title:  "Standings",
	Color:  0x5865F2, // neutral; or per-league colour
	Description: "```\n" + renderStandingsTable(rows) + "\n```",
	Footer: &discordgo.MessageEmbedFooter{
		Text: fmt.Sprintf("GW%d · total points league", snap.CurrentGW),
	},
	Timestamp: snap.BuiltAt.UTC().Format(time.RFC3339),
}
```

Where `renderStandingsTable` is today's `tabwriter` loop, minus the ```` ``` ````
fences (the embed adds them) and with columns like:

```
 #  Team            Tot   GW
 1  Lampard's XI    1204   62
 2  Klopp Off        1198   58
 3  Ten Hag Loose    1187   74
```

Notes: no pagination needed; `description` easily fits 20–30 rows. If you want a
movement arrow column, add a 5-char `↑`/`↓`/`–` field from the week-over-week
rank diff the repo already computes (commit `d112ba3`).

### 5.2 `/scores` — one embed, `Color` = phase, tag in the footer

Same code-block approach as `/standings`. Encode provisional/final in the colour
bar and the footer rather than a `(provisional)` header string.

```go
color := 0xFAA61A            // amber = provisional
phase := "provisional — scores can still move"
if finished {
	color = 0x3BA55D    // green = final
	phase = "final"
}
embed := &discordgo.MessageEmbed{
	Author:      &discordgo.MessageEmbedAuthor{Name: leagueName},
	Title:       fmt.Sprintf("GW%d scores", gw),
	Color:       color,
	Description:  "```\n" + renderScoresTable(rows) + "\n```",
	Footer:      &discordgo.MessageEmbedFooter{Text: phase},
	Timestamp:   snap.BuiltAt.UTC().Format(time.RFC3339),
}
```

Table body (unscored managers as `–`, highest first — unchanged logic):

```
Team              GW
Ten Hag Loose     74
Lampard's XI      62
Klopp Off         58
Pochettino Party   –
```

### 5.3 `/overview` — one embed per message, one non-inline `field` per fixture

`fields` row-per-record is the natural shape: `name` = the bold score line,
`value` = the scorer lines (emoji already in `overviewScorerLine`). `Color`
tracks the most "live" fixture in the batch. Replace the current
char-count pagination with a **25-field** cap per embed (Discord's limit); a
double gameweek is ~20 fixtures so it usually stays in one embed/one message.

```go
embed := &discordgo.MessageEmbed{
	Title: fmt.Sprintf("GW%d %s", gw, overviewTitle[mode]),
	Color: overviewColor(fixtures), // grey pre-KO, amber any live, green all FT
	Footer: &discordgo.MessageEmbedFooter{Text: leagueName},
	Timestamp: snap.BuiltAt.UTC().Format(time.RFC3339),
	Fields: make([]*discordgo.MessageEmbedField, 0, len(fixtures)),
}
for _, f := range fixtures {
	v := "_no goals_"
	if len(f.Scorers) > 0 {
		lines := make([]string, len(f.Scorers))
		for i, sc := range f.Scorers {
			lines[i] = overviewScorerLine(sc)
		}
		v = strings.Join(lines, "\n")
	}
	embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
		Name:   overviewFixtureHeader(f), // "Everton 2 - 1 Spurs  (FT)"
		Value:  v,                        // ≤1024 chars — fine for goal lists
		Inline: false,                    // one fixture per row, all platforms
	})
}
```

Rules to respect:
- Chunk into embeds of ≤25 fields; ≤10 embeds per message; if the combined
  `name`+`value` chars approach 6000, start a new message (mirrors the existing
  `renderOverview` splitting, just counting fields/chars against embed limits).
- `field.value` cap is 1024 — a single fixture's scorer list won't hit it, but
  guard anyway and truncate with "…".
- Keep the plain-text fallback path for the "no fixtures" case (`Respond`
  string) — an embed there is overkill.

Alternative if per-fixture cards are preferred visually: one **embed per
fixture**, `Title` = score line, `Description` = scorers, `Color` per state,
batched ≤10 per message. More vertical space per fixture and hits the 10-embed
wall at 10 fixtures, so the single-embed-with-fields form above is the better
default.

---

## 6. Open questions / things to verify against the live client

- Exact mobile wrapping of a ~34-char code-block line on the current Discord
  Android/iOS build — set the truncation width after eyeballing it.
- Whether the team wants a desktop-only `ansi` tint on provisional `/scores`
  (cheap win for desktop users, no-op on mobile) — deferred; `Color` bar covers
  the need for now.
- `snap.BuiltAt` (or equivalent snapshot timestamp) — confirm the field name in
  `internal/fpl` for the `Timestamp` values above.
