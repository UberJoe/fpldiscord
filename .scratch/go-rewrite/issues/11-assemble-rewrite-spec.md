# T11 — Assemble the rewrite spec

Parent: [Wayfinder map: Go rewrite](../map.md)
Type: task
Status: resolved
Blocked by: 01, 02, 03, 04, 05, 06, 07, 08, 09, 10

## Question

Every decision ticket (T01–T10) is resolved — nothing left to decide. This is the
terminal assembly step: stitch the resolved tickets into the single **rewrite-spec
document** the map's Destination names — "detailed enough to hand to a weekend build
session with nothing left to decide."

Not a decision. If a genuine open question surfaces while assembling, stop and raise
it as a new decision ticket rather than resolving it here.

### Deliverable

`.scratch/go-rewrite/spec.md` (the local-tracker spec path), a self-contained read
that a build session follows top to bottom. Suggested shape — adjust as the material
dictates:

1. **Overview & target** — one Go binary (bot goroutine + HTTP + `fpl` package),
   embedded React/Vite SPA, `/api/*`, SQLite on a fly volume, single league, classic
   mode this season. From the map Destination + Notes.
2. **API surface we build against** — the T01 Draft API findings + drift + the
   auto-sub bug. Link the research doc.
3. **Module & package layout** — T08 tree, import DAG, build order.
4. **Config & secrets** — T07 inventory, `.env.example`, boot-time `config.Load`.
5. **Data layer** — T05 SQLite schema + migrations runner + the `fpl` snapshot /
   refresher / serve-last-good design + the scoring stance.
6. **Discord bot** — T02 library choice + T03 per-command target behaviours + the
   reminder task + `bet` rules and surface.
7. **JSON API** — T06 envelope + the four endpoints.
8. **Web MVP** — T04 three views + polling model.
9. **Deployment & build** — T09 Dockerfile, `fly.toml`, volume, health check,
   runbook.
10. **Cutover** — T10 blind hard swap + `parity-checklist.md` + `legacy/` timing.
11. **Known post-MVP fog** — the map's Not-yet-specified list, so the build session
    knows what was deliberately deferred.

### Method

- Pull the detail from each ticket's `## Answer`; do not re-derive.
- Reconcile the superseding notes (T05 supersedes the T03 `scores` bonus clause and
  T01 §4; T09 drops the T09-body 503 idea) so the spec states only the final position.
- Keep it a spec, not a narrative — tables and concrete values over prose.
- Link the two research docs and `parity-checklist.md` as appendices rather than
  inlining them.

## Answer

Assembled as [`.scratch/go-rewrite/spec.md`](../spec.md) — 11 sections + appendices,
self-contained, spec-shaped (tables and concrete values over prose):

1. Overview & target · 2. Draft FPL API ground truth · 3. Module & package layout ·
4. Config & secrets · 5. Data layer (SQLite + `fpl`) · 6. Discord bot (commands +
`bet` + reminder) · 7. JSON API (`/api/*`) · 8. Web MVP (3 views) · 9. Deployment &
build · 10. Cutover & parity · 11. Known post-MVP fog + Out of scope. Appendices link
the two research docs and `parity-checklist.md`.

**Corrections reconciled so the spec states only the final position:**

- Scoring trusts `stats.total_points` **and** `stats.bonus` as-is — no provisional
  bonus recompute, no `pl/event-status` polling (T05 supersedes T01 §4 and the
  "recompute" clause in T03's `scores` verdict). Called out in the spec preamble and
  §2/§5.
- `/healthz` is always 200 once reachable — the T09-body "503 before snapshot" idea is
  dropped, because the listener binds only after snapshot #1 (§9.4). Distinct from
  `/api/*`, which does return 503 pre-first-snapshot (§7.3).
- No `bet` data migration — cutover coincides with a fresh `/bet set` round (§10.3).
- `NOTIFICATION_CHANNEL_ID` is the settled name (T02's provisional
  `REMINDER_CHANNEL_ID` did not survive T07).

No new open questions surfaced during assembly — nothing had to be kicked back as a
fresh decision ticket.

**This document is the map's destination.** All eleven tickets are resolved; the
frontier is empty. What remains (§11 fog) is post-destination and does not block the
weekend build.

## Comments

_(none)_
