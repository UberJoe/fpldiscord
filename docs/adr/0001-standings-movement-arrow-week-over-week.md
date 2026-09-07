# 1. Standings movement arrow tracks week-over-week position

Date: 2026-09-07

## Status

Accepted

## Context

The web Standings view shows a movement arrow next to each manager. As first
built, the arrow was `officialRank − liveRank`: the difference between the
manager's position in the Draft standings (which already folds in the current
gameweek's official event total) and their position once our own live scoring is
applied (live provisional bonus, auto-subs before the gameweek finalises). This
measured *within-gameweek* drift between official and live scoring. It had two
problems:

- It collapses toward flat as a gameweek finishes and official scoring catches up
  to live, so for the stretch between gameweeks it conveys nothing.
- It assumed the Draft official rank was a strict `1..N` sequence. Draft actually
  reports standard competition ranking, where level managers share a rank and the
  next rank skips. Subtracting a dense live rank from a joint official rank
  produced phantom arrows whenever two managers were tied.

The league reads the arrow as "did my team go up or down" in the ordinary
league-table sense — movement since the last completed gameweek — not as a
live-vs-official scoring delta.

## Decision

The movement arrow shows movement since **last week's final standings**: the
Draft `last_rank` field on each standings row, compared to the manager's current
live rank.

The live table adopts the same joint-ranking convention as the Draft standings —
managers level on live points share a rank — so both ends of the arrow
subtraction use the same ranking scheme. Row order within a tie is settled by the
Draft strict-sort field.

A manager with no previous position (`last_rank == 0`: gameweek 1, or a
mid-season entrant) shows a flat arrow.

## Consequences

- The arrow is stable across a gameweek in progress: `last_rank` is fixed, only
  the live rank moves as matches play out, so the arrow reflects real net
  movement rather than scoring-method noise.
- *Official total (Draft)* in the glossary keeps its meaning; a new *movement
  arrow* entry is added alongside it.
- The `officialRank` field is no longer used to compute the arrow. It stays on
  the `/api/standings` payload for now (the web view does not render it) and can
  be removed later if nothing else needs it.
- Discord `/standings` is unaffected — it never had a movement arrow and passes
  the Draft rank through directly.
