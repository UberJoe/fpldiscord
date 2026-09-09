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

**Status:** ready-for-agent

- [ ] `packCodeBlockMessages` and any helper functions that existed only to serve
      it are removed.
- [ ] No production code references the removed symbols; `go vet` / the build is
      clean.
- [ ] `go test ./...` is green.
