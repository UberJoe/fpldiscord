package bot

import (
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
)

// fakePoster records what the reminder loop tried to post instead of talking to
// Discord. err, when set, fails every send.
type fakePoster struct {
	sent []string
	err  error
}

func (f *fakePoster) ChannelMessageSend(_, content string, _ ...discordgo.RequestOption) (*discordgo.Message, error) {
	if f.err != nil {
		return nil, f.err
	}
	f.sent = append(f.sent, content)
	return &discordgo.Message{Content: content}, nil
}

func testReminder(p channelPoster) *reminder {
	return &reminder{
		poster:    p,
		channelID: "chan-1",
		log:       slog.New(slog.NewTextHandler(discardWriter{}, nil)),
		now:       time.Now,
		sent:      map[reminderSentKey]bool{},
	}
}

func utc(y int, mo time.Month, d, h, mi int) time.Time {
	return time.Date(y, mo, d, h, mi, 0, 0, time.UTC)
}

func TestDueReminder(t *testing.T) {
	noon := utc(2026, 9, 12, 12, 0) // a waiver deadline
	early := utc(2026, 9, 12, 3, 0) // a pre-dawn deadline

	cases := []struct {
		name     string
		deadline time.Time
		now      time.Time
		want     reminderKind
	}{
		{"day before", noon, utc(2026, 9, 11, 20, 0), reminderNone},
		{"same day before the wake", noon, utc(2026, 9, 12, 4, 59), reminderNone},
		{"at the 05:00 wake", noon, utc(2026, 9, 12, 5, 0), reminderToday},
		{"mid-morning", noon, utc(2026, 9, 12, 9, 30), reminderToday},
		{"exactly one hour out", noon, utc(2026, 9, 12, 11, 0), reminderOneHour},
		{"inside the final hour", noon, utc(2026, 9, 12, 11, 45), reminderOneHour},
		{"at the deadline", noon, utc(2026, 9, 12, 12, 0), reminderNone},
		{"past the deadline", noon, utc(2026, 9, 12, 13, 0), reminderNone},
		{"pre-dawn deadline, inside the hour", early, utc(2026, 9, 12, 2, 30), reminderOneHour},
		{"pre-dawn deadline, hours out", early, utc(2026, 9, 12, 0, 30), reminderNone},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := dueReminder(c.deadline, c.now); got != c.want {
				t.Errorf("dueReminder(%s, %s) = %q, want %q", c.deadline, c.now, got, c.want)
			}
		})
	}
}

func TestNextReminderFire(t *testing.T) {
	noon := utc(2026, 9, 12, 12, 0)

	cases := []struct {
		name         string
		now          time.Time
		deadline     time.Time
		haveDeadline bool
		want         time.Time
	}{
		{"no deadline, after today's wake", utc(2026, 9, 10, 10, 0), time.Time{}, false, utc(2026, 9, 11, 5, 0)},
		{"no deadline, before today's wake", utc(2026, 9, 10, 3, 0), time.Time{}, false, utc(2026, 9, 10, 5, 0)},
		{"deadline today, wake done", utc(2026, 9, 12, 6, 0), noon, true, utc(2026, 9, 12, 11, 0)},
		{"deadline tomorrow, wake for the today post", utc(2026, 9, 11, 22, 0), noon, true, utc(2026, 9, 12, 5, 0)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := nextReminderFire(c.now, c.deadline, c.haveDeadline)
			if !got.Equal(c.want) {
				t.Errorf("nextReminderFire = %s, want %s", got, c.want)
			}
		})
	}
}

func TestNextReminderFire_FlooredSoAJustMissedInstantCannotSpin(t *testing.T) {
	now := time.Date(2026, 9, 10, 4, 59, 40, 0, time.UTC) // 20s before the 05:00 wake
	got := nextReminderFire(now, time.Time{}, false)
	if want := now.Add(reminderMinSleep); !got.Equal(want) {
		t.Errorf("nextReminderFire = %s, want %s (floored)", got, want)
	}
}

func TestReminderMessage(t *testing.T) {
	deadline := utc(2026, 9, 12, 12, 0)
	ts := fmt.Sprintf("<t:%d:t>", deadline.Unix())

	if got, want := reminderMessage(reminderToday, deadline), "Waivers are happening today at: "+ts; got != want {
		t.Errorf("today message = %q, want %q", got, want)
	}
	if got, want := reminderMessage(reminderOneHour, deadline), "Waivers are in ONE HOUR at: "+ts; got != want {
		t.Errorf("one-hour message = %q, want %q", got, want)
	}
	if got := reminderMessage(reminderNone, deadline); got != "" {
		t.Errorf("none message = %q, want empty", got)
	}
}

func TestReminderPostIfDue_PostsEachReminderOncePerRound(t *testing.T) {
	f := &fakePoster{}
	r := testReminder(f)
	deadline := utc(2026, 9, 12, 12, 0)

	r.postIfDue(utc(2026, 9, 12, 9, 0), 7, deadline, true)   // today
	r.postIfDue(utc(2026, 9, 12, 10, 0), 7, deadline, true)  // today again — suppressed
	r.postIfDue(utc(2026, 9, 12, 11, 15), 7, deadline, true) // one hour
	r.postIfDue(utc(2026, 9, 12, 11, 30), 7, deadline, true) // one hour again — suppressed

	want := []string{
		"Waivers are happening today at: " + fmt.Sprintf("<t:%d:t>", deadline.Unix()),
		"Waivers are in ONE HOUR at: " + fmt.Sprintf("<t:%d:t>", deadline.Unix()),
	}
	if !reflect.DeepEqual(f.sent, want) {
		t.Fatalf("sent = %#v, want %#v (each reminder posted exactly once, in order)", f.sent, want)
	}
}

func TestReminderPostIfDue_RetriesAfterAFailedSend(t *testing.T) {
	f := &fakePoster{err: errors.New("discord down")}
	r := testReminder(f)
	deadline := utc(2026, 9, 12, 12, 0)

	r.postIfDue(utc(2026, 9, 12, 9, 0), 7, deadline, true)
	if len(f.sent) != 0 {
		t.Fatalf("sent = %v, want nothing after a failed send", f.sent)
	}
	f.err = nil
	r.postIfDue(utc(2026, 9, 12, 9, 30), 7, deadline, true)
	if len(f.sent) != 1 {
		t.Fatalf("sent = %v, want the reminder retried on the next wake", f.sent)
	}
}

func TestReminderPostIfDue_NoopBeforeTheFirstSnapshot(t *testing.T) {
	f := &fakePoster{}
	r := testReminder(f)
	r.postIfDue(utc(2026, 9, 12, 9, 0), 0, time.Time{}, false) // ok=false: no snapshot yet

	if len(f.sent) != 0 {
		t.Fatalf("sent = %v, want nothing", f.sent)
	}
}
