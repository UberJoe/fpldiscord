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

**Status:** ready-for-agent

- [ ] `/overview` replies with one field per fixture: score line as the name,
      goalscorer lines as the value, "no goals" note when the fixture has none;
      fixtures in the same order as today.
- [ ] The embed colour is neutral before kickoff, provisional if any shown
      fixture is live, and final if every shown fixture is finished.
- [ ] The title keeps the mode wording, the footer shows the league name, and the
      `Timestamp` equals the snapshot's build time.
- [ ] More than 25 fixtures split across multiple embeds; more than 10 embeds'
      worth split across multiple messages; a fixture whose scorer list would
      exceed the per-field limit is truncated with an ellipsis.
- [ ] The empty-window replies are still plain text.
- [ ] The overview handler tests assert on the embed structure and cover the
      25-field and multi-message chunking boundaries.
- [ ] `go test ./...` is green.
