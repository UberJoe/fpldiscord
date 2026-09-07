package fpl

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// Refresh cadence and per-endpoint TTLs. The refresher loops on RefreshInterval
// and refetches each endpoint only once its own TTL has elapsed.
const (
	// RefreshInterval is how often the refresher wakes to reconcile.
	RefreshInterval = 30 * time.Second

	ttlBootstrap     = time.Hour
	ttlDetails       = 10 * time.Minute
	ttlElementStatus = 10 * time.Minute
	ttlTransactions  = 10 * time.Minute
	ttlGame          = 2 * time.Minute
	ttlLiveWhileLive = 60 * time.Second
	ttlLiveIdle      = 10 * time.Minute
)

// Endpoint keys used for the per-endpoint fetch clock.
const (
	keyBootstrap     = "bootstrap"
	keyDetails       = "details"
	keyElementStatus = "element-status"
	keyTransactions  = "transactions"
	keyGame          = "game"
	keyLive          = "live"
	keyEntries       = "entries"
)

// endpointKeys is every key on the fetch clock, seeded together after snapshot #1.
var endpointKeys = []string{
	keyBootstrap, keyDetails, keyElementStatus, keyTransactions,
	keyGame, keyLive, keyEntries,
}

// fetcher is the subset of *Client the refresher depends on. Tests supply a fake.
type fetcher interface {
	fetchAll(ctx context.Context) (pieces, error)
	fetchGame(ctx context.Context) (Game, error)
	fetchBootstrap(ctx context.Context) (Bootstrap, error)
	fetchDetails(ctx context.Context) (LeagueDetails, error)
	fetchElementStatus(ctx context.Context) ([]ElementStatus, error)
	fetchTransactions(ctx context.Context) ([]Transaction, error)
	fetchLive(ctx context.Context, gw int) (LiveGW, error)
	fetchEntries(ctx context.Context, entries []LeagueEntry, gw int) (map[EntryID]EntryEvent, error)
}

// Refresher owns the single goroutine that rebuilds the snapshot. It keeps the
// last raw pieces so each endpoint can be refetched independently on its TTL and
// the Snapshot reassembled without re-hitting everything. All state is owned by
// the one goroutine that calls Bootstrap then Run; tests drive reconcile
// directly on a single goroutine.
type Refresher struct {
	fetch fetcher
	store *Store
	log   *slog.Logger

	now         func() time.Time
	sleep       func(context.Context, time.Duration)
	retryDelays []time.Duration
	interval    time.Duration

	// parts is the last raw payload set; reconcile refetches into it piecemeal.
	parts     pieces
	fetchedAt map[string]time.Time
}

// NewRefresher wires a Refresher around a real Client.
func NewRefresher(c *Client, store *Store, log *slog.Logger) *Refresher {
	return newRefresher(c, store, log)
}

func newRefresher(f fetcher, store *Store, log *slog.Logger) *Refresher {
	return &Refresher{
		fetch:       f,
		store:       store,
		log:         log,
		now:         func() time.Time { return time.Now().UTC() },
		sleep:       sleepCtx,
		retryDelays: []time.Duration{1 * time.Second, 2 * time.Second, 4 * time.Second},
		interval:    RefreshInterval,
		fetchedAt:   map[string]time.Time{},
	}
}

// Bootstrap builds snapshot #1, retrying the full pass on failure with 1/2/4s
// backoff until it succeeds or the budget is spent. On success it seeds the
// per-endpoint clock and publishes the snapshot. It returns an error only if no
// build ever succeeds within the budget — the boot sequence maps that to a
// non-zero exit so fly restarts the machine.
func (r *Refresher) Bootstrap(ctx context.Context, budget time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, budget)
	defer cancel()

	backoff := 1 * time.Second
	var lastErr error
	for {
		p, err := r.fetch.fetchAll(ctx)
		if err == nil {
			now := r.now()
			r.parts = p
			for _, k := range endpointKeys {
				r.fetchedAt[k] = now
			}
			snap := assemble(p, now, false)
			r.store.Set(snap)
			r.logStartup(snap)
			return nil
		}
		lastErr = err
		if ctx.Err() != nil {
			return fmt.Errorf("snapshot #1 never succeeded within %s: %w", budget, lastErr)
		}
		r.sleep(ctx, backoff)
		if ctx.Err() != nil {
			return fmt.Errorf("snapshot #1 never succeeded within %s: %w", budget, lastErr)
		}
		if backoff < 4*time.Second {
			backoff *= 2
		}
	}
}

// Run loops reconcile on the refresh interval until ctx is cancelled.
func (r *Refresher) Run(ctx context.Context) {
	t := time.NewTicker(r.interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			r.reconcile(ctx)
		}
	}
}

// reconcile runs one refresh cycle: refetch whatever endpoint TTLs are due (plus
// anything a current_event / waivers_processed change forces), keep the previous
// piece for anything that fails after its retries, then republish.
//
//   - nothing due, nothing failed  -> no republish; the current snapshot still
//     describes reality, pointer and BuiltAt untouched.
//   - something refetched OK        -> publish a fresh snapshot, BuiltAt = now.
//   - a wanted fetch failed         -> republish the last-good data with
//     Stale=true and the previous BuiltAt, so a brief upstream outage does not
//     churn the /api ETags.
func (r *Refresher) reconcile(ctx context.Context) {
	now := r.now()

	var attempted, failed bool
	run := func(key string, want bool, apply func(context.Context) error) {
		if !want {
			return
		}
		attempted = true
		if !r.tryFetch(ctx, key, now, apply) {
			failed = true
		}
	}

	// game drives everything else and is cheap — refetch it first when due, so
	// the forced-refetch check below sees this cycle's value.
	prevGame := r.parts.game
	run(keyGame, r.due(keyGame, ttlGame, now), func(c context.Context) error {
		g, err := r.fetch.fetchGame(c)
		if err == nil {
			r.parts.game = g
		}
		return err
	})

	// A change to the current gameweek or the waivers-processed flag forces a
	// refetch of the season-reference endpoints this cycle regardless of TTL.
	forced := r.parts.game.CurrentEvent != prevGame.CurrentEvent ||
		r.parts.game.WaiversProcessed != prevGame.WaiversProcessed
	if forced {
		r.log.Info("forced refetch: game state changed",
			"currentEvent", r.parts.game.CurrentEvent,
			"waiversProcessed", r.parts.game.WaiversProcessed)
	}

	prevGW := r.parts.currentGW
	newGW := resolveCurrentGW(r.parts.game)
	gwChanged := newGW != 0 && newGW != prevGW

	run(keyBootstrap, forced || r.due(keyBootstrap, ttlBootstrap, now), func(c context.Context) error {
		b, err := r.fetch.fetchBootstrap(c)
		if err == nil {
			r.parts.bootstrap = b
		}
		return err
	})
	run(keyDetails, forced || r.due(keyDetails, ttlDetails, now), func(c context.Context) error {
		d, err := r.fetch.fetchDetails(c)
		if err == nil {
			r.parts.details = d
		}
		return err
	})
	run(keyElementStatus, forced || r.due(keyElementStatus, ttlElementStatus, now), func(c context.Context) error {
		es, err := r.fetch.fetchElementStatus(c)
		if err == nil {
			r.parts.status = es
		}
		return err
	})
	run(keyTransactions, forced || r.due(keyTransactions, ttlTransactions, now), func(c context.Context) error {
		tx, err := r.fetch.fetchTransactions(c)
		if err == nil {
			r.parts.txns = tx
		}
		return err
	})

	// live TTL is 60s while a match is in play, else 10 min. entries (per-member
	// picks/subs) track the same cadence, plus a forced refetch when the GW rolls.
	liveTTL := ttlLiveIdle
	if cur := r.store.Current(); cur != nil && cur.MatchLive() {
		liveTTL = ttlLiveWhileLive
	}
	run(keyLive, newGW != 0 && (gwChanged || r.due(keyLive, liveTTL, now)), func(c context.Context) error {
		l, err := r.fetch.fetchLive(c, newGW)
		if err == nil {
			r.parts.live = l
		}
		return err
	})
	run(keyEntries, newGW != 0 && (gwChanged || r.due(keyEntries, liveTTL, now)), func(c context.Context) error {
		e, err := r.fetch.fetchEntries(c, r.parts.details.LeagueEntries, newGW)
		if err == nil {
			r.parts.entries = e
		}
		return err
	})

	r.parts.currentGW = newGW

	if !attempted {
		return // nothing was due — keep the published snapshot as-is
	}
	builtAt := now
	if failed {
		if prev := r.store.Current(); prev != nil {
			builtAt = prev.BuiltAt // keep the previous snapshot's identity
		}
	}
	r.store.Set(assemble(r.parts, builtAt, failed))
}

// due reports whether an endpoint's TTL has elapsed since it was last fetched.
func (r *Refresher) due(key string, ttl time.Duration, now time.Time) bool {
	last, ok := r.fetchedAt[key]
	if !ok {
		return true
	}
	return now.Sub(last) >= ttl
}

// tryFetch runs fn, retrying on error with the configured 1/2/4s delays (capped
// at 3 retries within the cycle). It records the fetch time and returns true on
// success; on final failure it logs and returns false so the caller keeps the
// previous value and marks the cycle stale.
func (r *Refresher) tryFetch(ctx context.Context, name string, now time.Time, fn func(context.Context) error) bool {
	err := fn(ctx)
	for i := 0; err != nil && i < len(r.retryDelays); i++ {
		r.sleep(ctx, r.retryDelays[i])
		if ctx.Err() != nil {
			break
		}
		err = fn(ctx)
	}
	if err != nil {
		r.log.Warn("refresh fetch failed, keeping previous", "endpoint", name, "err", err)
		return false
	}
	r.fetchedAt[name] = now
	return true
}

// logStartup writes the one-line summary after snapshot #1.
func (r *Refresher) logStartup(snap *Snapshot) {
	r.log.Info("fpl snapshot ready",
		"league", snap.LeagueName,
		"currentGw", snap.CurrentGW,
		"matchLive", snap.MatchLive(),
		"elements", snap.ElementCount,
	)
}

// sleepCtx sleeps for d or until ctx is done, whichever comes first.
func sleepCtx(ctx context.Context, d time.Duration) {
	if d <= 0 {
		return
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}
