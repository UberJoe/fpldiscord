// Package bet owns the season-long bet game's rule logic: each bettor's running
// goal total, their status (in / provisionally out / bust), the leader, and the
// leaderboard's display order. It is a pure function over stored picks (from
// package store) and the snapshot's cumulative season goal totals (from package
// fpl) — no I/O, no storage of its own.
package bet

import (
	"sort"

	"github.com/UberJoe/fpldiscord/internal/fpl"
	"github.com/UberJoe/fpldiscord/internal/store"
)

// Target is the goal total a bettor aims to get closest to without exceeding.
const Target = 21

// finalGameweek is the last gameweek of a Premier League season; the bet is
// settled once it has finished.
const finalGameweek = 38

// Status is a bettor's standing in the current season.
type Status string

const (
	// StatusIn: all four picks have scored at least once and the total is
	// within Target.
	StatusIn Status = "in"
	// StatusProvisionallyOut: the total is within Target but at least one pick
	// is still on zero goals. Flips back to StatusIn automatically once that
	// player scores.
	StatusProvisionallyOut Status = "provisionallyOut"
	// StatusBust: the total has gone over Target. Terminal for the season.
	StatusBust Status = "bust"
)

// GoalSource resolves an element id to its player row, from which the display
// name and cumulative season goals are read. *fpl.Snapshot satisfies it, as
// does the value returned by SnapshotGoals.
type GoalSource interface {
	Element(fpl.ElementID) (*fpl.Element, bool)
}

// SnapshotGoals adapts a snapshot to GoalSource over its exported
// Bootstrap.Elements slice. The snapshot's own Element method reads an
// unexported index that only fpl's internal assembler populates, so callers
// that drive Leaderboard with a hand-built snapshot (the seam tests in web and
// bot) must go through this to resolve players the way production does.
func SnapshotGoals(snap *fpl.Snapshot) GoalSource {
	byID := make(map[fpl.ElementID]*fpl.Element, len(snap.Bootstrap.Elements))
	for i := range snap.Bootstrap.Elements {
		e := &snap.Bootstrap.Elements[i]
		byID[e.ID] = e
	}
	return snapshotGoals{byID: byID}
}

type snapshotGoals struct {
	byID map[fpl.ElementID]*fpl.Element
}

func (g snapshotGoals) Element(id fpl.ElementID) (*fpl.Element, bool) {
	e, ok := g.byID[id]
	return e, ok
}

// Pick is one of a bettor's four players resolved against the snapshot.
type Pick struct {
	ElementID fpl.ElementID
	WebName   string // "" if the id is unknown to the snapshot
	Goals     int    // cumulative Premier League goals this season; 0 if unknown
}

// Bettor is one computed leaderboard row.
type Bettor struct {
	DiscordUserID string
	Picks         [4]Pick
	Total         int
	Status        Status
	Leader        bool
}

// Leaderboard resolves each bettor's four picks against goals, computes the
// running total and status, marks the leader, and returns the rows in display
// order: non-bust first, closest to Target (highest total) first, busts last,
// ties broken by Discord user id.
func Leaderboard(picks []store.BettorPicks, goals GoalSource) []Bettor {
	bettors := make([]Bettor, 0, len(picks))
	for _, bp := range picks {
		bettors = append(bettors, computeBettor(bp, goals))
	}
	markLeader(bettors)
	sortBoard(bettors)
	return bettors
}

// computeBettor resolves one bettor's picks and derives their total and status.
func computeBettor(bp store.BettorPicks, goals GoalSource) Bettor {
	b := Bettor{DiscordUserID: bp.DiscordUserID}
	anyOnZero := false
	for i, rawID := range bp.Elements {
		id := fpl.ElementID(rawID)
		p := Pick{ElementID: id}
		if el, ok := goals.Element(id); ok {
			p.WebName = el.WebName
			p.Goals = el.GoalsScored
		}
		if p.Goals == 0 {
			anyOnZero = true
		}
		b.Total += p.Goals
		b.Picks[i] = p
	}
	switch {
	case b.Total > Target:
		b.Status = StatusBust
	case anyOnZero:
		b.Status = StatusProvisionallyOut
	default:
		b.Status = StatusIn
	}
	return b
}

// SeasonComplete reports whether the bet is settled — the final gameweek has
// finished — which is the gate for stamping the trophy on the leader. It lives
// here so the bet game owns all of its rules, including when the season ends.
func SeasonComplete(snap *fpl.Snapshot) bool {
	return snap != nil &&
		snap.Game.CurrentEvent == finalGameweek &&
		snap.Game.CurrentEventFinished
}

// markLeader sets Leader on the non-bust bettors with the highest total —
// closest to Target without going over. A pick still on zero (provisionallyOut)
// does not remove a bettor from contention, since it flips back automatically
// once that player scores; only a bust does. A total of exactly Target is the
// highest a non-bust bettor can reach, so it wins outright with no special case;
// equal-highest totals are marked joint. An all-bust board has no leader.
func markLeader(bs []Bettor) {
	best := -1
	for _, b := range bs {
		if b.Status != StatusBust && b.Total > best {
			best = b.Total
		}
	}
	if best < 0 {
		return
	}
	for i := range bs {
		if bs[i].Status != StatusBust && bs[i].Total == best {
			bs[i].Leader = true
		}
	}
}

// sortBoard orders the board: non-bust before bust, then highest total first
// (closest to Target from below), then Discord user id for a stable tie-break.
func sortBoard(bs []Bettor) {
	sort.Slice(bs, func(i, j int) bool {
		a, b := bs[i], bs[j]
		if aBust, bBust := a.Status == StatusBust, b.Status == StatusBust; aBust != bBust {
			return !aBust
		}
		if a.Total != b.Total {
			return a.Total > b.Total
		}
		return a.DiscordUserID < b.DiscordUserID
	})
}
