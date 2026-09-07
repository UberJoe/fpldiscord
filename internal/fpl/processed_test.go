package fpl

import (
	"reflect"
	"testing"
	"time"
)

func TestProcessedGWs_FromWaiverTransactionsAscendingUnique(t *testing.T) {
	snap := &Snapshot{
		CurrentGW: 5,
		Transactions: []Transaction{
			{Kind: "w", Event: 3},
			{Kind: "w", Event: 2},
			{Kind: "w", Event: 3}, // duplicate GW
			{Kind: "f", Event: 4}, // free agent — ignored
			{Kind: "t", Event: 1}, // trade — ignored
			{Kind: "w", Event: 0}, // no event — ignored
		},
	}

	got := snap.ProcessedGWs()
	want := []int{2, 3}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ProcessedGWs() = %v, want %v", got, want)
	}
}

func TestProcessedGWs_IncludesCurrentGWWhenWaiversProcessedFlagSet(t *testing.T) {
	snap := &Snapshot{
		CurrentGW:    5,
		Game:         Game{WaiversProcessed: true},
		Transactions: []Transaction{{Kind: "w", Event: 4}},
	}

	got := snap.ProcessedGWs()
	want := []int{4, 5}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ProcessedGWs() = %v, want %v", got, want)
	}
}

func TestProcessedGWs_FromEventsWhoseWaiverTimeHasPassed(t *testing.T) {
	built := time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC)
	snap := &Snapshot{
		BuiltAt:   built,
		CurrentGW: 4,
		Bootstrap: Bootstrap{Events: []Event{
			{ID: 1, WaiversTime: apiTime(built.AddDate(0, 0, -18))}, // past, no transaction
			{ID: 2, WaiversTime: apiTime(built.AddDate(0, 0, -11))}, // past, no transaction
			{ID: 3, WaiversTime: apiTime(built.AddDate(0, 0, -4))},  // past
			{ID: 4, WaiversTime: apiTime(built.AddDate(0, 0, 4))},   // future — not yet
			{ID: 5, WaiversTime: apiTime(time.Time{})},              // unscheduled
		}},
		Transactions: []Transaction{{Kind: "w", Event: 3}},
	}

	got := snap.ProcessedGWs()
	want := []int{1, 2, 3}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ProcessedGWs() = %v, want %v", got, want)
	}
}

func TestProcessedGWs_EmptyWhenNoWaiversYet(t *testing.T) {
	snap := &Snapshot{
		CurrentGW:    2,
		Transactions: []Transaction{{Kind: "f", Event: 1}},
	}

	if got := snap.ProcessedGWs(); len(got) != 0 {
		t.Fatalf("ProcessedGWs() = %v, want empty", got)
	}
}
