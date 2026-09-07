# 01 — Embed output seam, shared scaffold, and ADR

**What to build:** The bot can send a Discord embed as a reply, not just a
string, and does so through the same output seam command handlers already use.
This ticket lays the plumbing the three feature tickets build on; it changes no
command's behaviour on its own.

- The `Responder` interface gains a second method that carries embeds. One call
  sends one Discord message that may carry more than one embed.
- The real responder implements the new method through the **same three-way
  branching** the string path already has: first call is the interaction
  response (or an edit of the deferred-ACK placeholder when one was sent), every
  later call in the same handler is a follow-up message. Respect the discordgo
  shape differences between the immediate, deferred-edit, and follow-up paths.
- The test fake records embed payloads alongside the string messages it already
  records, so handler tests can assert on embed structure.
- Shared, named colour constants for the three states the feature tickets need:
  neutral / informational, live / provisional, final / settled.
- One shared embed-scaffold helper that stamps the common parts every data
  command wants: author = league name (when present), footer text, and
  `Timestamp` from the snapshot build time formatted RFC 3339.
- An ADR at `docs/adr/0002-*.md` recording the convention: rich-data Discord
  replies are embeds with a fenced-code-block table body; colour encodes
  provisional / final / neutral; the footer carries gameweek context and a
  snapshot-time "last updated". Mirrors ADR 0001's treatment of a user-visible
  indicator change.

**Blocked by:** None — can start immediately.

**Status:** done

- [x] `Responder` exposes an embeds-sending method next to `Respond(string)`; the
      string method is unchanged. (`RespondEmbeds([]*discordgo.MessageEmbed)`)
- [x] The real responder sends embeds on the first call as the interaction
      response, edits the deferred placeholder with embeds when a deferred ACK
      was sent, and sends later calls as follow-up messages — matching the
      existing string-path branching. (`interactionResponder.RespondEmbeds`,
      shared `answered` flag; deferred-edit path uses the pointer-to-slice shape)
- [x] The test fake records embed payloads; existing string-path assertions in
      the suite still pass. (`recordingResponder.embeds [][]*discordgo.MessageEmbed`)
- [x] Colour constants for neutral, provisional and final states exist in one
      place. (`internal/bot/embed.go`: `colorNeutral` / `colorProvisional` /
      `colorFinal`)
- [x] An embed-scaffold helper sets author, footer and RFC 3339 `Timestamp` from
      `Snapshot.BuiltAt`. (`dataEmbed`; Timestamp omitted for a zero `BuiltAt`)
- [x] `docs/adr/0002-*.md` records the embeds convention and colour semantics.
- [x] No command's reply changes in this ticket.
- [x] `go test ./...` is green.
