package fpl

import (
	"sort"
	"strconv"
)

// ScoredPlayer is one member of a manager's scoring XI once auto-subs have been
// resolved: the player, their position and original squad slot, their live
// points (stats.total_points as-is, with bonus already folded in), minutes, and
// whether an auto-sub brought them on.
type ScoredPlayer struct {
	Element      ElementID
	Pos          Pos
	Slot         int // original squad slot 1..15 (Pick.Position)
	Points       int
	Minutes      int
	AutoSubbedIn bool
}

// ApplyAutoSubs selects a manager's actual scoring XI for a gameweek.
//
// Two modes:
//
//   - subs populated — the Draft API fills subs[] once the GW is finalised, so
//     it is authoritative: apply each ElementOut→ElementIn swap verbatim,
//     regardless of minutes played.
//   - subs empty — the GW is live or provisional. For each starter (slots 1-11)
//     who played zero minutes AND whose fixture(s) are all finished-provisional,
//     bring on the first bench player (slots 12→15) whose introduction keeps the
//     squad a valid formation (per-position min/max from squad). A bench slot
//     that settings.squad locks to a position (slot 12 → "GKP") can only come on
//     for a starter of that position, so the backup GK only ever replaces the
//     starting GK. A starter whose match is still in play is never subbed out.
//
// Each Pick must carry a resolved Pos (ManagerScore fills it from
// bootstrap-static). The result is the 11-player scoring XI.
func ApplyAutoSubs(picks []Pick, subs []Sub, live LiveGW, squad SquadSettings) []ScoredPlayer {
	xi, bench := splitSquad(picks)
	broughtOn := map[ElementID]bool{}

	if len(subs) > 0 {
		for _, sub := range subs {
			for i := range xi {
				if xi[i].Element != sub.ElementOut {
					continue
				}
				if b, ok := findPick(bench, sub.ElementIn); ok {
					xi[i] = b
				} else {
					xi[i] = Pick{Element: sub.ElementIn}
				}
				broughtOn[sub.ElementIn] = true
				break
			}
		}
		return scoredXI(xi, broughtOn, live)
	}

	used := map[ElementID]bool{}
	for i := range xi {
		if !starterNeedsSub(xi[i].Element, live) {
			continue
		}
		for _, b := range bench {
			if used[b.Element] {
				continue
			}
			// A locked bench slot (settings.squad.position_type_locks, e.g.
			// "12": "GKP") may only come on for a starter of the locked position.
			if lp, locked := lockedPos(squad, b.Position); locked && xi[i].Pos != lp {
				continue
			}
			trial := append([]Pick(nil), xi...)
			trial[i] = b
			if !formationOK(trial, squad) {
				continue
			}
			xi[i] = b
			used[b.Element] = true
			broughtOn[b.Element] = true
			break
		}
	}
	return scoredXI(xi, broughtOn, live)
}

// lockedPos reports the position a bench slot is locked to by
// settings.squad.position_type_locks ("GKP"/"DEF"/"MID"/"FWD"), if any.
func lockedPos(squad SquadSettings, slot int) (Pos, bool) {
	switch squad.PositionTypeLocks[strconv.Itoa(slot)] {
	case "GKP":
		return PosGK, true
	case "DEF":
		return PosDEF, true
	case "MID":
		return PosMID, true
	case "FWD":
		return PosFWD, true
	default:
		return 0, false
	}
}

// splitSquad separates the submitted XI (slots 1-11) from the bench (12-15),
// each returned in ascending slot order.
func splitSquad(picks []Pick) (xi, bench []Pick) {
	for _, p := range picks {
		switch {
		case p.Position >= 1 && p.Position <= 11:
			xi = append(xi, p)
		case p.Position >= 12:
			bench = append(bench, p)
		}
	}
	sort.Slice(xi, func(i, j int) bool { return xi[i].Position < xi[j].Position })
	sort.Slice(bench, func(i, j int) bool { return bench[i].Position < bench[j].Position })
	return xi, bench
}

func findPick(picks []Pick, el ElementID) (Pick, bool) {
	for _, p := range picks {
		if p.Element == el {
			return p, true
		}
	}
	return Pick{}, false
}

// starterNeedsSub reports whether a starter blanked (zero minutes) and their
// gameweek is over — every fixture that fed their live line is
// finished-provisional. A starter with no linked fixture yet (kickoff still to
// come) is left alone.
func starterNeedsSub(el ElementID, live LiveGW) bool {
	le, ok := live.Elements[el]
	if !ok || le.Stats.Minutes != 0 {
		return false
	}
	if len(le.Explain) == 0 {
		return false
	}
	for _, ex := range le.Explain {
		f, ok := fixtureByID(live.Fixtures, ex.Fixture)
		if !ok || !f.FinishedProvisional {
			return false
		}
	}
	return true
}

func fixtureByID(fixtures []LiveFixture, id int) (LiveFixture, bool) {
	for _, f := range fixtures {
		if f.ID == id {
			return f, true
		}
	}
	return LiveFixture{}, false
}

// formationOK reports whether an XI matches the squad's play count, the GK
// minimum and maximum, and the outfield per-position minimums. (settings.squad
// carries no DEF/MID/FWD maximum — that is the whole ruleset.)
func formationOK(xi []Pick, squad SquadSettings) bool {
	if len(xi) != squad.Play {
		return false
	}
	var gk, def, mid, fwd int
	for _, p := range xi {
		switch p.Pos {
		case PosGK:
			gk++
		case PosDEF:
			def++
		case PosMID:
			mid++
		case PosFWD:
			fwd++
		}
	}
	switch {
	case gk < squad.MinPlayGKP || gk > squad.MaxPlayGKP:
		return false
	case def < squad.MinPlayDEF:
		return false
	case mid < squad.MinPlayMID:
		return false
	case fwd < squad.MinPlayFWD:
		return false
	}
	return true
}

func scoredXI(xi []Pick, broughtOn map[ElementID]bool, live LiveGW) []ScoredPlayer {
	out := make([]ScoredPlayer, 0, len(xi))
	for _, p := range xi {
		st := live.Elements[p.Element].Stats
		out = append(out, ScoredPlayer{
			Element:      p.Element,
			Pos:          p.Pos,
			Slot:         p.Position,
			Points:       st.TotalPoints,
			Minutes:      st.Minutes,
			AutoSubbedIn: broughtOn[p.Element],
		})
	}
	return out
}
