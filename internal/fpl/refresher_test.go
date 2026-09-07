package fpl

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"
)

// fakeFetcher is an in-memory fetcher. Each endpoint returns the stored value,
// counts its calls, and fails its first failN[endpoint] invocations.
type fakeFetcher struct {
	game    Game
	details LeagueDetails
	live    LiveGW
	entries map[EntryID]EntryEvent

	calls map[string]int
	failN map[string]int
}

func newFakeFetcher() *fakeFetcher {
	return &fakeFetcher{
		game: Game{CurrentEvent: 4, NextEvent: 5},
		details: LeagueDetails{
			League: League{ID: 64, Name: "Coq au Ian", Scoring: "c"},
			LeagueEntries: []LeagueEntry{
				{ID: 39936, EntryID: 39880, PlayerFirstName: "Bruno"},
				{ID: 89, EntryID: 89, PlayerFirstName: "Ian"},
			},
		},
		live: LiveGW{
			Elements: map[ElementID]LiveElement{11: {Stats: LiveStats{TotalPoints: 9}}},
			Fixtures: []LiveFixture{{ID: 31, Event: 4, Started: true, FinishedProvisional: false}},
		},
		entries: map[EntryID]EntryEvent{39880: {}, 89: {}},
		calls:   map[string]int{},
		failN:   map[string]int{},
	}
}

func (f *fakeFetcher) hit(name string) error {
	f.calls[name]++
	if f.failN[name] > 0 {
		f.failN[name]--
		return errors.New(name + ": injected failure")
	}
	return nil
}

func (f *fakeFetcher) fetchGame(context.Context) (Game, error) {
	return f.game, f.hit(keyGame)
}
func (f *fakeFetcher) fetchBootstrap(context.Context) (Bootstrap, error) {
	return Bootstrap{Elements: []Element{{ID: 11, WebName: "Højlund"}}}, f.hit(keyBootstrap)
}
func (f *fakeFetcher) fetchDetails(context.Context) (LeagueDetails, error) {
	return f.details, f.hit(keyDetails)
}
func (f *fakeFetcher) fetchElementStatus(context.Context) ([]ElementStatus, error) {
	return []ElementStatus{{Element: 11, Status: "a"}}, f.hit(keyElementStatus)
}
func (f *fakeFetcher) fetchTransactions(context.Context) ([]Transaction, error) {
	return []Transaction{{ID: 1, Event: 3}}, f.hit(keyTransactions)
}
func (f *fakeFetcher) fetchLive(_ context.Context, _ int) (LiveGW, error) {
	return f.live, f.hit(keyLive)
}
func (f *fakeFetcher) fetchEntries(_ context.Context, _ []LeagueEntry, _ int) (map[EntryID]EntryEvent, error) {
	return f.entries, f.hit(keyEntries)
}

func (f *fakeFetcher) fetchAll(ctx context.Context) (pieces, error) {
	var p pieces
	var err error
	if p.game, err = f.fetchGame(ctx); err != nil {
		return pieces{}, err
	}
	if p.bootstrap, err = f.fetchBootstrap(ctx); err != nil {
		return pieces{}, err
	}
	if p.details, err = f.fetchDetails(ctx); err != nil {
		return pieces{}, err
	}
	if p.status, err = f.fetchElementStatus(ctx); err != nil {
		return pieces{}, err
	}
	if p.txns, err = f.fetchTransactions(ctx); err != nil {
		return pieces{}, err
	}
	p.currentGW = resolveCurrentGW(p.game)
	if p.live, err = f.fetchLive(ctx, p.currentGW); err != nil {
		return pieces{}, err
	}
	if p.entries, err = f.fetchEntries(ctx, p.details.LeagueEntries, p.currentGW); err != nil {
		return pieces{}, err
	}
	return p, nil
}

func testRefresher(f fetcher, store *Store) *Refresher {
	r := newRefresher(f, store, slog.New(slog.NewTextHandler(io.Discard, nil)))
	r.sleep = func(context.Context, time.Duration) {} // no real waits
	r.retryDelays = []time.Duration{0, 0, 0}          // 3 retries, instant
	return r
}

func TestRefresher_BootstrapPublishesSnapshot(t *testing.T) {
	f := newFakeFetcher()
	store := NewStore()
	r := testRefresher(f, store)

	if err := r.Bootstrap(context.Background(), time.Second); err != nil {
		t.Fatalf("Bootstrap() error: %v", err)
	}
	snap := store.Current()
	if snap == nil {
		t.Fatal("Bootstrap() published no snapshot")
	}
	if snap.LeagueName != "Coq au Ian" || snap.CurrentGW != 4 {
		t.Errorf("snapshot = %q gw %d", snap.LeagueName, snap.CurrentGW)
	}
	if snap.Stale {
		t.Error("fresh snapshot marked Stale")
	}
	for _, k := range endpointKeys {
		if _, ok := r.fetchedAt[k]; !ok {
			t.Errorf("fetch clock not seeded for %q", k)
		}
	}
}

func TestRefresher_BootstrapRetriesThenSucceeds(t *testing.T) {
	f := newFakeFetcher()
	f.failN[keyGame] = 2 // fetchAll aborts on game twice before succeeding
	store := NewStore()
	r := testRefresher(f, store)

	if err := r.Bootstrap(context.Background(), time.Second); err != nil {
		t.Fatalf("Bootstrap() error: %v", err)
	}
	if store.Current() == nil {
		t.Fatal("no snapshot after retried Bootstrap")
	}
	if f.calls[keyGame] != 3 {
		t.Errorf("game fetch attempts = %d, want 3", f.calls[keyGame])
	}
}

func TestRefresher_BootstrapGivesUpAtBudget(t *testing.T) {
	f := newFakeFetcher()
	f.failN[keyGame] = 1_000
	store := NewStore()
	r := testRefresher(f, store)
	// sleep advances a fake clock so the deadline is actually reached.
	ctx, cancel := context.WithCancel(context.Background())
	r.sleep = func(context.Context, time.Duration) { cancel() }

	err := r.Bootstrap(ctx, time.Second)
	if err == nil {
		t.Fatal("Bootstrap() returned nil on a permanently failing upstream")
	}
	if store.Current() != nil {
		t.Fatal("Bootstrap() published a snapshot despite never succeeding")
	}
}

func TestReconcile_RefetchesOnlyDueEndpoints(t *testing.T) {
	f := newFakeFetcher()
	// No match in play, so live/entries sit on the 10-min idle TTL.
	f.live.Fixtures = []LiveFixture{{ID: 31, Event: 4, Started: true, FinishedProvisional: true}}
	store := NewStore()
	r := testRefresher(f, store)

	base := time.Date(2026, 9, 12, 14, 0, 0, 0, time.UTC)
	r.now = func() time.Time { return base }
	if err := r.Bootstrap(context.Background(), time.Second); err != nil {
		t.Fatal(err)
	}
	// +3 min: past the 2-min game TTL, short of every other TTL.
	r.now = func() time.Time { return base.Add(3 * time.Minute) }
	r.reconcile(context.Background())

	if f.calls[keyGame] != 2 {
		t.Errorf("game calls = %d, want 2 (refetched)", f.calls[keyGame])
	}
	for _, k := range []string{keyBootstrap, keyDetails, keyElementStatus, keyTransactions, keyLive, keyEntries} {
		if f.calls[k] != 1 {
			t.Errorf("%s calls = %d, want 1 (not yet due)", k, f.calls[k])
		}
	}
}

func TestReconcile_GameStateChangeForcesSeasonRefetch(t *testing.T) {
	f := newFakeFetcher()
	store := NewStore()
	r := testRefresher(f, store)

	base := time.Date(2026, 9, 12, 14, 0, 0, 0, time.UTC)
	r.now = func() time.Time { return base }
	if err := r.Bootstrap(context.Background(), time.Second); err != nil {
		t.Fatal(err)
	}

	// current_event rolls 4 -> 5 (also a GW change).
	f.game.CurrentEvent = 5
	r.now = func() time.Time { return base.Add(3 * time.Minute) } // only game TTL due on its own
	r.reconcile(context.Background())

	for _, k := range []string{keyBootstrap, keyDetails, keyElementStatus, keyTransactions} {
		if f.calls[k] != 2 {
			t.Errorf("%s calls = %d, want 2 (forced by game-state change)", k, f.calls[k])
		}
	}
	// A GW change also pulls fresh live + entries.
	if f.calls[keyLive] != 2 || f.calls[keyEntries] != 2 {
		t.Errorf("live/entries calls = %d/%d, want 2/2 after GW change", f.calls[keyLive], f.calls[keyEntries])
	}
	if store.Current().CurrentGW != 5 {
		t.Errorf("CurrentGW = %d, want 5", store.Current().CurrentGW)
	}
}

func TestReconcile_KeepsPreviousAndMarksStaleOnFailure(t *testing.T) {
	f := newFakeFetcher()
	store := NewStore()
	r := testRefresher(f, store)

	base := time.Date(2026, 9, 12, 14, 0, 0, 0, time.UTC)
	r.now = func() time.Time { return base }
	if err := r.Bootstrap(context.Background(), time.Second); err != nil {
		t.Fatal(err)
	}

	// details now fails every attempt; its "fresh" name would have been different.
	f.failN[keyDetails] = 1_000
	f.details.League.Name = "SHOULD NOT APPEAR"
	r.now = func() time.Time { return base.Add(15 * time.Minute) } // details TTL (10m) due
	r.reconcile(context.Background())

	snap := store.Current()
	if !snap.Stale {
		t.Error("snapshot not marked Stale after a failed fetch")
	}
	if snap.LeagueName != "Coq au Ian" {
		t.Errorf("LeagueName = %q, want the kept previous value", snap.LeagueName)
	}
	// 1 (bootstrap) + 1 initial + 3 retries.
	if f.calls[keyDetails] != 5 {
		t.Errorf("details attempts = %d, want 5 (1 + 1 + 3 retries)", f.calls[keyDetails])
	}
	// The details fetch clock must NOT advance on failure.
	if r.fetchedAt[keyDetails] != base {
		t.Errorf("details fetch clock advanced despite failure")
	}
}

func TestReconcile_IdleCycleDoesNotRepublish(t *testing.T) {
	f := newFakeFetcher()
	f.live.Fixtures = []LiveFixture{{ID: 31, Event: 4, Started: true, FinishedProvisional: true}} // no live match
	store := NewStore()
	r := testRefresher(f, store)

	base := time.Date(2026, 9, 12, 14, 0, 0, 0, time.UTC)
	r.now = func() time.Time { return base }
	if err := r.Bootstrap(context.Background(), time.Second); err != nil {
		t.Fatal(err)
	}
	first := store.Current()

	// +90s: nothing is due (game TTL 2m, live idle TTL 10m).
	r.now = func() time.Time { return base.Add(90 * time.Second) }
	r.reconcile(context.Background())

	if store.Current() != first {
		t.Error("reconcile republished a snapshot when nothing was due")
	}
}

func TestReconcile_StaleCycleKeepsPreviousBuiltAt(t *testing.T) {
	f := newFakeFetcher()
	store := NewStore()
	r := testRefresher(f, store)

	base := time.Date(2026, 9, 12, 14, 0, 0, 0, time.UTC)
	r.now = func() time.Time { return base }
	if err := r.Bootstrap(context.Background(), time.Second); err != nil {
		t.Fatal(err)
	}
	builtAt := store.Current().BuiltAt

	f.failN[keyGame] = 1_000
	r.now = func() time.Time { return base.Add(3 * time.Minute) }
	r.reconcile(context.Background())

	snap := store.Current()
	if !snap.Stale {
		t.Error("snapshot not marked Stale")
	}
	if !snap.BuiltAt.Equal(builtAt) {
		t.Errorf("BuiltAt = %v on a stale cycle, want the previous %v", snap.BuiltAt, builtAt)
	}
}

func TestReconcile_RecoversFromStaleOnNextGoodCycle(t *testing.T) {
	f := newFakeFetcher()
	store := NewStore()
	r := testRefresher(f, store)

	base := time.Date(2026, 9, 12, 14, 0, 0, 0, time.UTC)
	r.now = func() time.Time { return base }
	if err := r.Bootstrap(context.Background(), time.Second); err != nil {
		t.Fatal(err)
	}

	f.failN[keyGame] = 1_000
	r.now = func() time.Time { return base.Add(3 * time.Minute) }
	r.reconcile(context.Background())
	if !store.Current().Stale {
		t.Fatal("expected Stale after failure")
	}

	f.failN[keyGame] = 0
	r.now = func() time.Time { return base.Add(6 * time.Minute) }
	r.reconcile(context.Background())
	if store.Current().Stale {
		t.Error("snapshot still Stale after a fully successful cycle")
	}
}

func TestReconcile_LiveCadenceTracksMatchLive(t *testing.T) {
	// Match in play -> 60s live TTL.
	t.Run("live match: 90s is due", func(t *testing.T) {
		f := newFakeFetcher() // fixture 31 started, not finished_provisional
		store := NewStore()
		r := testRefresher(f, store)
		base := time.Date(2026, 9, 12, 14, 0, 0, 0, time.UTC)
		r.now = func() time.Time { return base }
		if err := r.Bootstrap(context.Background(), time.Second); err != nil {
			t.Fatal(err)
		}
		if !store.Current().MatchLive() {
			t.Fatal("precondition: MatchLive should be true")
		}
		r.now = func() time.Time { return base.Add(90 * time.Second) } // > 60s, < 2m game TTL
		r.reconcile(context.Background())
		if f.calls[keyLive] != 2 || f.calls[keyEntries] != 2 {
			t.Errorf("live/entries calls = %d/%d, want 2/2 (fast cadence)", f.calls[keyLive], f.calls[keyEntries])
		}
		if f.calls[keyGame] != 1 {
			t.Errorf("game calls = %d, want 1 (2-min TTL not reached)", f.calls[keyGame])
		}
	})

	// No match in play -> 10-min live TTL.
	t.Run("no live match: 90s is not due", func(t *testing.T) {
		f := newFakeFetcher()
		f.live.Fixtures = []LiveFixture{{ID: 31, Event: 4, Started: true, FinishedProvisional: true}}
		store := NewStore()
		r := testRefresher(f, store)
		base := time.Date(2026, 9, 12, 14, 0, 0, 0, time.UTC)
		r.now = func() time.Time { return base }
		if err := r.Bootstrap(context.Background(), time.Second); err != nil {
			t.Fatal(err)
		}
		if store.Current().MatchLive() {
			t.Fatal("precondition: MatchLive should be false")
		}
		r.now = func() time.Time { return base.Add(90 * time.Second) }
		r.reconcile(context.Background())
		if f.calls[keyLive] != 1 || f.calls[keyEntries] != 1 {
			t.Errorf("live/entries calls = %d/%d, want 1/1 (idle cadence)", f.calls[keyLive], f.calls[keyEntries])
		}
	})
}
