# 12 — Waiver-reminder daily task

**Spec:** [../to-spec.md](../to-spec.md)

**What to build:** A background task posts a waiver reminder into the notification
channel ahead of the next relevant gameweek's waiver deadline — once at a morning
wake, once on the day, once an hour before — using a self-recomputing timer rather
than fragile wall-clock arithmetic.

**Blocked by:** 02

**Status:** ready-for-agent

- [ ] A `time.Timer` goroutine recomputes its next fire each iteration (no accumulated
      drift, no DST bug)
- [ ] It reads `waivers_time` for the next relevant GW looked up by event `id`
- [ ] It posts to `NOTIFICATION_CHANNEL_ID` at 05:00 UTC wake, same-day, and T−1h
- [ ] An in-memory `lastSent{gw, kind}` guard prevents the same reminder firing twice;
      fired-state is not persisted
- [ ] The goroutine is started in the run phase and stopped on shutdown
