# 05 — Retire `packCodeBlockMessages`

**What to build:** With `/waivers` and `/bet` migrated to embeds (tickets 03 and
04), the hand-rolled `packCodeBlockMessages` string paginator — which packed
display blocks into as few sub-2000-character code-block messages as possible —
has no callers left. Delete it and its now-unused helpers.

This is the contract half of the paginator removal: pure dead-code cleanup, no
behaviour change. If tickets 03 and 04 both merge cleanly and one of them ends up
being the last to touch the shared file, the deletion can fold into that ticket
instead — but as a standalone step it stays blocked by both.

**Blocked by:** 03 — `/waivers` as embeds; 04 — `/bet` leaderboard as embeds.

**Status:** done (folded into ticket 04)

- [x] `packCodeBlockMessages` and any helper functions that existed only to serve
      it are removed.
- [x] No production code references the removed symbols; `go vet` / the build is
      clean.
- [x] `go test ./...` is green.

## Comments

Folded into ticket 04, the last ticket to remove a `packCodeBlockMessages`
caller. `packCodeBlockMessages` and the `maxDiscordMessage` const it used were
deleted from `internal/bot/waivers.go`; no helpers existed solely to serve it.
`bet_test.go`'s one reference (`TestHandleBet_ShowSplitsALongBoardAcrossMessages`)
was retargeted at the embed chunker boundary in ticket 04.
