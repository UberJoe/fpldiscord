package fpl

import (
	"testing"
	"time"
)

func TestNextWaiverDeadline_EarliestFutureRoundByEventID(t *testing.T) {
	now := time.Date(2026, 9, 12, 12, 0, 0, 0, time.UTC)
	// Events deliberately out of array order and mixing past / future / unset,
	// to prove the lookup is by id and takes the soonest future round.
	snap := &Snapshot{
		Bootstrap: Bootstrap{Events: []Event{
			{ID: 4, WaiversTime: apiTime(now.AddDate(0, 0, 7))},  // future, later
			{ID: 2, WaiversTime: apiTime(now.AddDate(0, 0, -7))}, // past
			{ID: 3, WaiversTime: apiTime(now.AddDate(0, 0, 2))},  // future, sooner
			{ID: 5, WaiversTime: apiTime(time.Time{})},           // unscheduled
		}},
	}

	gw, deadline, ok := snap.NextWaiverDeadline(now)
	if !ok {
		t.Fatal("NextWaiverDeadline ok = false, want a future round")
	}
	if gw != 3 {
		t.Errorf("gw = %d, want 3", gw)
	}
	if want := now.AddDate(0, 0, 2); !deadline.Equal(want) {
		t.Errorf("deadline = %s, want %s", deadline, want)
	}
}

func TestNextWaiverDeadline_ExactNowIsNotFuture(t *testing.T) {
	now := time.Date(2026, 9, 12, 12, 0, 0, 0, time.UTC)
	snap := &Snapshot{
		Bootstrap: Bootstrap{Events: []Event{
			{ID: 3, WaiversTime: apiTime(now)},                // exactly now — excluded
			{ID: 4, WaiversTime: apiTime(now.Add(time.Hour))}, // future
		}},
	}

	gw, _, ok := snap.NextWaiverDeadline(now)
	if !ok || gw != 4 {
		t.Fatalf("gw, ok = %d, %v; want 4, true", gw, ok)
	}
}

func TestNextWaiverDeadline_NoFutureRound(t *testing.T) {
	now := time.Date(2026, 9, 12, 12, 0, 0, 0, time.UTC)
	snap := &Snapshot{
		Bootstrap: Bootstrap{Events: []Event{
			{ID: 1, WaiversTime: apiTime(now.AddDate(0, 0, -14))},
			{ID: 2, WaiversTime: apiTime(time.Time{})},
		}},
	}

	if _, _, ok := snap.NextWaiverDeadline(now); ok {
		t.Fatal("NextWaiverDeadline ok = true, want false when every round is in the past")
	}
}

func TestNextWaiverDeadline_NoEvents(t *testing.T) {
	if _, _, ok := (&Snapshot{}).NextWaiverDeadline(time.Now()); ok {
		t.Fatal("NextWaiverDeadline ok = true on an empty snapshot")
	}
}
