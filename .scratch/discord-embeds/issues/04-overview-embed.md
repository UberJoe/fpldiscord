# 04 — `/overview` as embed(s)

**What to build:** `/overview` replies with an embed whose fields are the
fixtures — one non-inline field per fixture, the score line as the field name and
the goalscorer lines as the field value, with a "no goals" note when a fixture
has none. The existing scorer-line formatting and its emoji are reused verbatim.
The embed's colour reflects the shown fixtures: neutral before kickoff,
provisional if any is live, final if all are finished. The title keeps the
existing mode wording (today's fixtures / gameweek fixtures / live fixtures), the
footer carries the league name, and the `Timestamp` is the snapshot build time.

Fixtures are chunked to respect Discord's limits — at most 25 fields per embed,
at most 10 embeds per message, and a new message before the combined field text
approaches the per-message character ceiling. This replaces the current raw
2000-character text pagination with limit-aware chunking, but keeps the
"spill to another message" behaviour for a big or double gameweek. Each field
value is guarded against the per-field character limit and truncated with an
ellipsis if a single fixture's scorer list would exceed it.

The empty-window replies ("no fixtures kick off today", "nothing in play", "no
fixtures for this gameweek") stay short plain text.

**Blocked by:** 01 — Embed output seam, shared scaffold, and ADR.

**Status:** done

- [x] `/overview` replies with one field per fixture: score line as the name,
      goalscorer lines as the value, "no goals" note when the fixture has none;
      fixtures in the same order as today. (`renderOverview` → one non-inline
      `discordgo.MessageEmbedField` per fixture; `overviewFixtureHeader` /
      `overviewFixtureBody`)
- [x] The embed colour is neutral before kickoff, provisional if any shown
      fixture is live, and final if every shown fixture is finished.
      (`overviewColor`)
- [x] The title keeps the mode wording, the footer shows the league name, and the
      `Timestamp` equals the snapshot's build time. (title
      `"GW{n} {mode wording}"` on the first embed only; `dataEmbed("", BuiltAt,
      title, colour, leagueName)` — league name in the footer, not the author
      line, per the spec)
- [x] More than 25 fixtures split across multiple embeds; more than 10 embeds'
      worth split across multiple messages; a fixture whose scorer list would
      exceed the per-field limit is truncated with an ellipsis.
      (`overviewChunker.add` closes an embed at `maxEmbedFields`, a message at
      `maxEmbedsPerMessage` or `overviewMessageCharBudget` — a small margin under
      `maxMessageEmbedChars`; per-field guard is `capRunes(body,
      maxEmbedFieldValue)`)
- [x] The empty-window replies are still plain text. (`overviewEmptyReply` via
      `Respond`, unchanged; nil-snapshot and bad-mode too)
- [x] The overview handler tests assert on the embed structure and cover the
      25-field and multi-message chunking boundaries.
      (`TestRenderOverview_SplitsPastTwentyFiveFixturesIntoMultipleEmbeds`,
      `TestRenderOverview_SplitsPastTenEmbedsIntoASecondMessage`,
      `TestRenderOverview_TruncatesAFixtureValueOverTheFieldLimit`)
- [x] `go test ./...` is green.

## Comments

**Return type: `[][]*discordgo.MessageEmbed`, not `[]*discordgo.MessageEmbed`.**
The spec's Implementation Decisions line hints `renderOverview` returns
`[]*discordgo.MessageEmbed` "(the per-message chunks)". A message can carry up to
ten embeds and the ticket wants both a >25-fixture split (into embeds) *and* a
>10-embed split (into messages), so the helper returns one
`[]*discordgo.MessageEmbed` per Discord message and the handler sends each with a
single `RespondEmbeds` call. Read the spec's phrase as "one chunk per message".

**Field name is plain text.** `overviewFixtureHeader` dropped the `**bold**` /
`_italic_` markdown it carried in the plain-text version — Discord renders no
markdown in an embed field name, so the tag is now `(live)` / `(FT)` rather than
`_(live)_`. `overviewScorerLine` (the field *value*) is unchanged; markdown does
render there, so the `_no goals_` note keeps its italics.

**`maxDiscordMessage` no longer used by `/overview`.** The old raw 2000-char
pagination is gone; the constant still backs `/waivers` and `/bet`.
