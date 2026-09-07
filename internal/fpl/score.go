package fpl

import "fmt"

// ManagerScore is a manager's gameweek score: the scoring XI with auto-subs
// resolved, and the plain sum of its live points.
type ManagerScore struct {
	EntryID EntryID
	GW      int
	Total   int
	Players []ScoredPlayer
}

// ManagerScore computes a manager's score for gw. It resolves the scoring XI
// with ApplyAutoSubs, then sums that XI's live stats.total_points as-is —
// stats.bonus is already part of total_points and is likewise trusted as-is,
// with no provisional bonus recompute and no scoring engine.
//
// gw may be 0 (meaning the current GW). The MVP snapshot only carries the
// current GW's picks and live feed, so any other gw, an unknown entry, a missing
// live feed, or empty picks all return an error.
func (s *Snapshot) ManagerScore(id EntryID, gw int) (ManagerScore, error) {
	if gw == 0 {
		gw = s.CurrentGW
	}
	if gw != s.CurrentGW {
		return ManagerScore{}, fmt.Errorf("manager score: snapshot carries GW %d, not %d", s.CurrentGW, gw)
	}

	ev, ok := s.Entries[id]
	if !ok {
		return ManagerScore{}, fmt.Errorf("manager score: no picks for entry %d in GW %d", id, gw)
	}
	if len(ev.Picks) == 0 {
		return ManagerScore{}, fmt.Errorf("manager score: empty picks for entry %d in GW %d", id, gw)
	}
	live, ok := s.Live[gw]
	if !ok {
		return ManagerScore{}, fmt.Errorf("manager score: no live feed for GW %d", gw)
	}

	picks := make([]Pick, len(ev.Picks))
	copy(picks, ev.Picks)
	for i := range picks {
		if el, ok := s.elementByID[picks[i].Element]; ok {
			picks[i].Pos = Pos(el.ElementType)
		}
	}

	xi := ApplyAutoSubs(picks, ev.Subs, live, s.Bootstrap.Settings.Squad)
	total := 0
	for _, p := range xi {
		total += p.Points
	}
	return ManagerScore{EntryID: id, GW: gw, Total: total, Players: xi}, nil
}
