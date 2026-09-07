package fpl

import "sync/atomic"

// Store publishes the current Snapshot. Current() is lock-free and always
// returns the last good snapshot; it returns nil only until the first
// successful build.
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
