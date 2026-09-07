# 12 — Waiver-reminder daily task

**Spec:** [../to-spec.md](../to-spec.md)

**What to build:** A background task posts a waiver reminder into the notification
channel ahead of the next relevant gameweek's waiver deadline — once at a morning
wake, once on the day, once an hour before — using a self-recomputing timer rather
than fragile wall-clock arithmetic.

**Blocked by:** 02

**Status:** done

- [x] A `time.Timer` goroutine recomputes its next fire each iteration (no accumulated
      drift, no DST bug) — `reminder.Run` reads an absolute UTC clock every iteration
      and builds a fresh `time.NewTimer(nextReminderFire(...) - now)`; all instants are
      UTC so there is no DST arithmetic
- [x] It reads `waivers_time` for the next relevant GW looked up by event `id` —
      `fpl.Snapshot.NextWaiverDeadline` scans `Bootstrap.Events` by `ev.ID`, taking the
      soonest future `waivers_time`
- [x] It posts to `NOTIFICATION_CHANNEL_ID` at 05:00 UTC wake, same-day, and T−1h —
      `dueReminder` returns `today` from the 05:00 wake on deadline day and `hour`
      through the final hour; `reminderMessage` renders the `<t:unix:t>` posts
- [x] An in-memory `lastSent{gw, kind}` guard prevents the same reminder firing twice;
      fired-state is not persisted — `reminder.sent map[reminderSentKey]bool`, never
      written to the store; a restart re-derives outstanding reminders from the snapshot
- [x] The goroutine is started in the run phase and stopped on shutdown —
      `app.Run` starts `bot.RunReminder` once the gateway is open and cancels it
      (before `bot.Close`) on shutdown

## Implementation

- `internal/fpl/waiverdeadline.go` — `NextWaiverDeadline(now)` derived view (+ table test)
- `internal/bot/reminder.go` — schedule constants, `dueReminder` / `nextReminderFire` /
  `reminderMessage` pure helpers, the `reminder` struct with the `sent` guard, and the
  self-recomputing `Run` loop (+ `reminder_test.go`)
- `internal/bot/bot.go` — `notificationChannelID` field, `RunReminder(ctx)`
- `internal/app/app.go` — reminder goroutine lifecycle in `Run`

The `Run` loop / `time.Timer` firing itself is left to the parity checklist (per the
to-spec testing table); its decision helpers and the dedupe guard are unit-tested.
