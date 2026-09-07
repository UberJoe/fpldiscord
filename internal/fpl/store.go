package fpl

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"
)

// Store publishes the current Snapshot. Current() is lock-free and always
// returns the last good snapshot; it returns nil only until the first successful
// build.
type Store struct {
	cur atomic.Pointer[Snapshot]
}

// NewStore returns an empty Store. Current() returns nil until Set is called.
func NewStore() *Store {
	return &Store{}
}

// Current returns the last-published snapshot, or nil before snapshot #1.
func (s *Store) Current() *Snapshot {
	return s.cur.Load()
}

// Set publishes snap as the current snapshot with a single atomic pointer swap.
func (s *Store) Set(snap *Snapshot) {
	s.cur.Store(snap)
}

// BuiltAt reports when the current snapshot was built. ok is false before
// snapshot #1. This satisfies the consumer-side SnapshotProvider interface in
// internal/web without web importing the whole fpl surface.
func (s *Store) BuiltAt() (t time.Time, ok bool) {
	if snap := s.cur.Load(); snap != nil {
		return snap.BuiltAt, true
	}
	return time.Time{}, false
}

// BuildFirst attempts to build snapshot #1, retrying on failure until it
// succeeds or the deadline is reached, and publishes it to the store. It returns
// an error only if no build ever succeeds within the budget — the boot sequence
// maps that to a non-zero exit so fly restarts the machine.
func BuildFirst(ctx context.Context, c *Client, store *Store, budget time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, budget)
	defer cancel()

	var lastErr error
	backoff := time.Second
	timer := time.NewTimer(backoff)
	defer timer.Stop()
	for {
		snap, err := c.Build(ctx)
		if err == nil {
			store.Set(snap)
			return nil
		}
		lastErr = err

		select {
		case <-ctx.Done():
			return fmt.Errorf("snapshot #1 never succeeded within %s: %w", budget, lastErr)
		case <-timer.C:
		}
		if backoff < 4*time.Second {
			backoff *= 2
		}
		timer.Reset(backoff)
	}
}
