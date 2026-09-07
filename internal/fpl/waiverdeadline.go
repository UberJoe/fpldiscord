package fpl

import "time"

// NextWaiverDeadline returns the gameweek id and scheduled waivers_time of the
// next waiver round still ahead of now, read from the event calendar by event
// id (never by array position — the Draft events array is 0-indexed over
// 1-indexed ids). ok is false when the snapshot carries no event with a future
// waivers_time.
//
// "Next relevant" falls out of the future filter: once a round's waivers_time
// has passed it is no longer the earliest future one, so a round that has
// already run is never re-announced — with no dependence on game.waivers_processed
// having caught up.
func (s *Snapshot) NextWaiverDeadline(now time.Time) (gw int, deadline time.Time, ok bool) {
	for i := range s.Bootstrap.Events {
		ev := &s.Bootstrap.Events[i]
		wt := ev.WaiversTime.Time()
		if wt.IsZero() || !wt.After(now) {
			continue
		}
		if !ok || wt.Before(deadline) {
			gw, deadline, ok = ev.ID, wt, true
		}
	}
	return gw, deadline, ok
}
