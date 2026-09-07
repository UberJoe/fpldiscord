# 14 — Decode the Draft live-feed `explain` tuple

**Spec:** [../to-spec.md](../to-spec.md)

**What to build:** Snapshot #1 builds against the real league instead of failing the
30s boot budget. `/event/{gw}/live` on the Draft API serialises each `explain` entry
as a `[statsArray, fixtureId]` tuple, not the main-site `{"fixture": N}` object the
`fpl` package currently models — so the live-feed decode aborts (`cannot unmarshal
array into ... explain.0 of type fpl.LiveExplain`) and the bot never reaches
gateway-open. The `fpl` package reads the real tuple shape, and the hand-built score
fixtures that encode the wrong shape are corrected. This lands before ticket 09
resumes: it unblocks all manual and live testing.

**Blocked by:** None — can start immediately

**Status:** done

- [x] `LiveExplain` unmarshals the Draft tuple `[[{stat…}, …], fixtureId]` — `Fixture`
      resolves from the trailing id, the stats half is ignored
      (`LiveExplain.UnmarshalJSON` in `internal/fpl/types.go`)
- [x] Empty and `null` `explain` still decode to an empty slice, so a starter with no
      linked fixture yet (kickoff to come) does not error the whole snapshot
      (a `null` tuple entry also leaves `Fixture` zero rather than failing)
- [x] The three hand-authored `explain` blocks under `internal/fpl/testdata/score/`
      (`live.json`, `live-gk-blank.json`, `live-352-defblank.json`) are rewritten to
      the real tuple shape
- [x] Existing auto-subs coverage passes unchanged — starter blanked with every
      linked fixture finished-provisional is subbed; a starter whose fixture is still
      live is left alone (`autosubs_test.go` untouched, still green)
- [x] `go test ./...` is green.
      `go run ./cmd/fpldiscord` against league 64 (live gateway + BulkOverwrite —
      outward-facing) left for the operator to run; snapshot-#1 decode is now covered
      by `internal/fpl/types_test.go`.
