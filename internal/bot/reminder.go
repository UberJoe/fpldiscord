package bot

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/bwmarrin/discordgo"
)

// Waiver-reminder schedule. Hardcoded here, never configuration (see to-spec
// "Config & secrets"). Every instant computed in this file is UTC, so there is
// no daylight-saving arithmetic anywhere in the task.
const (
	reminderWakeHour = 5           // 05:00 UTC — the morning "today" post
	reminderLeadTime = time.Hour   // the "one hour" post lands this far ahead
	reminderMinSleep = time.Minute // floor on the recomputed timer
)

// reminderKind identifies one of the two posts a waiver round gets. With the
// gameweek id it keys the in-memory sent guard.
type reminderKind string

const (
	reminderNone    reminderKind = ""
	reminderToday   reminderKind = "today"
	reminderOneHour reminderKind = "hour"
)

// channelPoster is the subset of *discordgo.Session the reminder loop needs:
// send one message to a channel.
type channelPoster interface {
	ChannelMessageSend(channelID, content string, options ...discordgo.RequestOption) (*discordgo.Message, error)
}

// reminder is the waiver-reminder background task: a self-recomputing time.Timer
// that posts a "today" reminder at the 05:00 UTC wake on a waiver round's day
// and a "one hour" reminder at T−1h, into the notification channel. The next
// fire is recomputed from an absolute clock reading every iteration (no
// accumulated drift), and fired-state is the in-memory sent map only — nothing
// is persisted, so a restart re-derives what is still outstanding from the
// snapshot.
type reminder struct {
	snap      SnapshotSource
	poster    channelPoster
	channelID string
	log       *slog.Logger
	now       func() time.Time
	sent      map[reminderSentKey]bool
}

type reminderSentKey struct {
	gw   int
	kind reminderKind
}

func newReminder(snap SnapshotSource, poster channelPoster, channelID string, log *slog.Logger) *reminder {
	return &reminder{
		snap:      snap,
		poster:    poster,
		channelID: channelID,
		log:       log,
		now:       func() time.Time { return time.Now().UTC() },
		sent:      map[reminderSentKey]bool{},
	}
}

// Run drives the task until ctx is cancelled. Unlike Refresher.Run's ticker,
// each iteration acts *before* it sleeps: it posts whatever is due now, then
// sleeps on a freshly computed timer until the next wake. Acting first is what
// lets a restart mid-run-up recover a reminder the previous process would have
// posted (fired-state is not persisted). The loop itself is left to the parity
// checklist (see to-spec testing table); its decision helpers — dueReminder,
// nextReminderFire, reminderMessage — and the sent guard are unit-tested.
func (r *reminder) Run(ctx context.Context) {
	for {
		now := r.now()
		gw, deadline, ok := r.nextDeadline(now)
		r.postIfDue(now, gw, deadline, ok)

		fire := nextReminderFire(now, deadline, ok)
		timer := time.NewTimer(fire.Sub(now))
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

// nextDeadline reads the next relevant waiver round from the current snapshot,
// or ok=false before snapshot #1.
func (r *reminder) nextDeadline(now time.Time) (gw int, deadline time.Time, ok bool) {
	if r.snap == nil {
		return 0, time.Time{}, false
	}
	snap := r.snap.Current()
	if snap == nil {
		return 0, time.Time{}, false
	}
	return snap.NextWaiverDeadline(now)
}

// postIfDue posts the single reminder due at now, if any, unless it has already
// been sent for this round. A send failure is logged and the round left
// unmarked, so it is re-attempted at the next scheduled wake if still due.
func (r *reminder) postIfDue(now time.Time, gw int, deadline time.Time, ok bool) {
	if !ok {
		return
	}
	kind := dueReminder(deadline, now)
	if kind == reminderNone {
		return
	}
	key := reminderSentKey{gw: gw, kind: kind}
	if r.sent[key] {
		return
	}
	if _, err := r.poster.ChannelMessageSend(r.channelID, reminderMessage(kind, deadline)); err != nil {
		r.log.Error("waiver reminder post failed", "gw", gw, "kind", string(kind), "err", err)
		return
	}
	r.sent[key] = true
	r.log.Info("waiver reminder posted", "gw", gw, "kind", string(kind), "deadline", deadline.Format(time.RFC3339))
}

// dueReminder decides which post, if any, is due for a waiver deadline as of
// now. The "today" post runs from the 05:00 UTC wake on the deadline's calendar
// day until the one-hour mark; the "one hour" post runs through the final hour.
// At or after the deadline nothing is due.
func dueReminder(deadline, now time.Time) reminderKind {
	if !now.Before(deadline) {
		return reminderNone
	}
	if !now.Before(deadline.Add(-reminderLeadTime)) {
		return reminderOneHour
	}
	if wake := atHourUTC(deadline, reminderWakeHour); !now.Before(wake) && sameUTCDay(now, deadline) {
		return reminderToday
	}
	return reminderNone
}

// nextReminderFire is the absolute instant the loop should next wake: the
// soonest of the next 05:00 UTC wake, the deadline's own 05:00 wake, and the
// deadline's one-hour mark — floored by reminderMinSleep so a just-passed
// instant cannot spin the loop.
func nextReminderFire(now, deadline time.Time, haveDeadline bool) time.Time {
	next := nextDailyWake(now)
	if haveDeadline {
		for _, cand := range []time.Time{atHourUTC(deadline, reminderWakeHour), deadline.Add(-reminderLeadTime)} {
			if cand.After(now) && cand.Before(next) {
				next = cand
			}
		}
	}
	if floor := now.Add(reminderMinSleep); next.Before(floor) {
		next = floor
	}
	return next
}

// nextDailyWake is the next 05:00 UTC — today if now is before it, else
// tomorrow.
func nextDailyWake(now time.Time) time.Time {
	wake := atHourUTC(now, reminderWakeHour)
	if !wake.After(now) {
		wake = wake.Add(24 * time.Hour)
	}
	return wake
}

// reminderMessage renders the notification-channel text. The deadline is shown
// with Discord's <t:unix:t> markup so every reader sees it in their own zone.
func reminderMessage(kind reminderKind, deadline time.Time) string {
	ts := fmt.Sprintf("<t:%d:t>", deadline.Unix())
	switch kind {
	case reminderToday:
		return "Waivers are happening today at: " + ts
	case reminderOneHour:
		return "Waivers are in ONE HOUR at: " + ts
	default:
		return ""
	}
}

// atHourUTC returns hour h (UTC) on t's calendar day.
func atHourUTC(t time.Time, h int) time.Time {
	y, m, d := t.UTC().Date()
	return time.Date(y, m, d, h, 0, 0, 0, time.UTC)
}

// sameUTCDay reports whether a and b fall on the same UTC calendar day.
func sameUTCDay(a, b time.Time) bool {
	ay, am, ad := a.UTC().Date()
	by, bm, bd := b.UTC().Date()
	return ay == by && am == bm && ad == bd
}
