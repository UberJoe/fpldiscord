package fpl

import "sort"

// ProcessedGWs returns the ascending, de-duplicated gameweek ids that have had a
// waiver round processed. It drives the web Waiver-History gameweek selector and
// the meta.processedGws envelope field, and is drawn from all three sources the
// spec names:
//
//   - bootstrap-static events whose scheduled waivers_time has already passed
//     (as of this snapshot's build time) — catches a round with zero claims,
//     which leaves no transaction;
//   - waiver rows in the transaction feed;
//   - the current gameweek when game.waivers_processed is set (the events and
//     transaction feeds can both lag that flag by a refresh cycle).
//
// Free-agent and trade transactions are ignored: those do not happen in a
// scheduled round, so they do not make a gameweek "processed".
func (s *Snapshot) ProcessedGWs() []int {
	seen := map[int]bool{}

	for _, ev := range s.Bootstrap.Events {
		if wt := ev.WaiversTime.Time(); !wt.IsZero() && wt.Before(s.BuiltAt) {
			seen[ev.ID] = true
		}
	}
	for _, t := range s.Transactions {
		if t.Kind == "w" && t.Event > 0 {
			seen[t.Event] = true
		}
	}
	if s.Game.WaiversProcessed && s.CurrentGW > 0 {
		seen[s.CurrentGW] = true
	}

	out := make([]int, 0, len(seen))
	for gw := range seen {
		out = append(out, gw)
	}
	sort.Ints(out)
	return out
}
