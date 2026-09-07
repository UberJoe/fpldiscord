# 08 — `waivers` command + `/api/waivers` + web Waiver History view

**Spec:** [../to-spec.md](../to-spec.md)

**What to build:** A league member can review any processed waiver round — in Discord
via `/waivers [gw] [result]` (accepted by default, or `failed` / `all`, showing who
out-bid whom, split across multiple messages when long) and on the web in a Waiver
History view showing one gameweek at a time with a selector, accepted and failed
claims in one table with bid order.

**Blocked by:** 02, 05

**Status:** done

- [x] `LeagueTransactions(gw)` resolves raw `type`/`status` codes to
      `waiver`/`freeAgent` and `accepted`/`failed`, exposes `priority`, and excludes
      trades (`internal/fpl/transactions.go`; groups on `ElementIn` id, not name)
- [x] `/waivers` with no args shows the latest processed GW's accepted claims; `[gw]`
      selects a GW; `result` = `accepted` (default) / `failed` / `all`
- [x] `failed`/`all` output shows who out-bid whom in bid order; long output splits
      across multiple messages with no truncation (`packWaiverMessages`;
      `interactionResponder` first call → response, later calls → follow-ups)
- [x] `GET /api/waivers?gw=N` returns `gw` + `rows[]` (`ownerName`, `entryId`, `in`,
      `out`, `type`, `status`, `priority`, `index`), sorted by `index`; omitted `gw` →
      `max(processedGws)`; out-of-range clamps (no 400) and the resolved value is
      echoed in `data.gw`
- [x] `priority` is present on the unauthenticated feed so bid order works without
      login
- [x] The web view has a GW selector defaulting to the latest processed GW, one table
      for accepted + failed with bid order, and refetches on mount only (no live poll)
      (`web/src/views/Waivers.tsx`; list-styled-as-table, matching Standings/Manager)
- [x] seam-3 + seam-4 coverage (`internal/web/waivers_test.go`,
      `internal/bot/waivers_test.go`, `internal/fpl/transactions_test.go`)
