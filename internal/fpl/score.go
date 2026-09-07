package fpl

import (
	"fmt"
	"sort"
)

// ManagerScore is a manager's gameweek score: the scoring XI with auto-subs
// resolved, and the plain sum of its live points.
type ManagerScore struct {
	EntryID EntryID
	GW      int
	Total   int
	Players []ScoredPlayer
}

// SquadPlayer is one of a manager's 15 squad players for a gameweek once
// auto-subs have been resolved: the player and club, their original squad slot
// and position, their live points/minutes (stats.total_points as-is, with bonus
// already folded in), and where the pick landed after auto-subs.
type SquadPlayer struct {
	Element       ElementID
	WebName       string // accent-stripped display name
	TeamShort     string // club short name, e.g. "ARS"
	Pos           Pos
	Slot          int // original squad slot 1..15 (Pick.Position)
	Points        int
	Minutes       int
	InScoringXI   bool // counts towards the gameweek total
	AutoSubbedIn  bool // a bench pick an auto-sub brought on
	AutoSubbedOut bool // a slot 1-11 starter an auto-sub replaced
}

// ManagerSquad is a manager's full 15-player gameweek squad with auto-subs
// resolved and the gameweek total (the plain sum of the scoring XI's live
// points). It is the derived view behind the web /manager/{entryId} drill-down;
// ManagerScore is the same computation reduced to just the scoring XI.
type ManagerSquad struct {
	EntryID     EntryID
	GW          int
	Total       int
	Provisional bool // true while the GW is live/provisional; false once finalised
	Players     []SquadPlayer
}

// ManagerSquad computes a manager's full gameweek squad for gw: all 15 picks in
// squad-slot order, each tagged with where it landed after ApplyAutoSubs
// (InScoringXI / AutoSubbedIn / AutoSubbedOut), plus the gameweek Total (the
// plain sum of the scoring XI's live stats.total_points, bonus trusted as-is).
//
// gw may be 0 (meaning the current GW). The MVP snapshot only carries the
// current GW's picks and live feed, so any other gw, an unknown entry, a
// missing live feed, or empty picks all return an error.
func (s *Snapshot) ManagerSquad(id EntryID, gw int) (ManagerSquad, error) {
	if gw == 0 {
		gw = s.CurrentGW
	}
	if gw != s.CurrentGW {
		return ManagerSquad{}, fmt.Errorf("manager squad: snapshot carries GW %d, not %d", s.CurrentGW, gw)
	}

	ev, ok := s.Entries[id]
	if !ok {
		return ManagerSquad{}, fmt.Errorf("manager squad: no picks for entry %d in GW %d", id, gw)
	}
	if len(ev.Picks) == 0 {
		return ManagerSquad{}, fmt.Errorf("manager squad: empty picks for entry %d in GW %d", id, gw)
	}
	live, ok := s.Live[gw]
	if !ok {
		return ManagerSquad{}, fmt.Errorf("manager squad: no live feed for GW %d", gw)
	}

	// One pass over bootstrap-static for every per-element lookup the squad view
	// needs: playing position, accent-stripped name, club short name. Built from
	// the exported Elements/Teams slices rather than the unexported indexes so
	// ManagerSquad stays a pure function of the exported Snapshot fields —
	// sibling-package seam tests (internal/web) build a Snapshot literal and
	// call it directly.
	shortByTeam := make(map[int]string, len(s.Bootstrap.Teams))
	for i := range s.Bootstrap.Teams {
		t := &s.Bootstrap.Teams[i]
		shortByTeam[t.ID] = t.ShortName
	}
	type elemInfo struct {
		pos       Pos
		webName   string
		teamShort string
	}
	infoByElement := make(map[ElementID]elemInfo, len(s.Bootstrap.Elements))
	for i := range s.Bootstrap.Elements {
		e := &s.Bootstrap.Elements[i]
		infoByElement[e.ID] = elemInfo{
			pos:       Pos(e.ElementType),
			webName:   stripAccents(e.WebName),
			teamShort: shortByTeam[e.Team],
		}
	}

	picks := make([]Pick, len(ev.Picks))
	copy(picks, ev.Picks)
	for i := range picks {
		picks[i].Pos = infoByElement[picks[i].Element].pos
	}

	xi := ApplyAutoSubs(picks, ev.Subs, live, s.Bootstrap.Settings.Squad)
	inXI := make(map[ElementID]bool, len(xi))
	subbedIn := make(map[ElementID]bool, len(xi))
	total := 0
	for _, p := range xi {
		inXI[p.Element] = true
		if p.AutoSubbedIn {
			subbedIn[p.Element] = true
		}
		total += p.Points
	}

	players := make([]SquadPlayer, 0, len(picks))
	for _, pk := range picks {
		st := live.Elements[pk.Element].Stats
		info := infoByElement[pk.Element]
		starter := pk.Position >= 1 && pk.Position <= 11
		players = append(players, SquadPlayer{
			Element:       pk.Element,
			WebName:       info.webName,
			TeamShort:     info.teamShort,
			Pos:           pk.Pos,
			Slot:          pk.Position,
			Points:        st.TotalPoints,
			Minutes:       st.Minutes,
			InScoringXI:   inXI[pk.Element],
			AutoSubbedIn:  subbedIn[pk.Element],
			AutoSubbedOut: starter && !inXI[pk.Element],
		})
	}
	sort.Slice(players, func(i, j int) bool { return players[i].Slot < players[j].Slot })

	return ManagerSquad{
		EntryID:     id,
		GW:          gw,
		Total:       total,
		Provisional: !s.GWFinished,
		Players:     players,
	}, nil
}

// ManagerScore computes a manager's score for gw. It is ManagerSquad reduced to
// the scoring XI: the players ApplyAutoSubs put in the scoring eleven — in
// squad-slot order — and the plain sum of their live stats.total_points as-is
// (stats.bonus is already part of total_points and is likewise trusted as-is,
// with no provisional bonus recompute and no scoring engine).
//
// gw may be 0 (meaning the current GW); the same errors as ManagerSquad apply.
func (s *Snapshot) ManagerScore(id EntryID, gw int) (ManagerScore, error) {
	sq, err := s.ManagerSquad(id, gw)
	if err != nil {
		return ManagerScore{}, err
	}

	xi := make([]ScoredPlayer, 0, 11)
	for _, p := range sq.Players {
		if !p.InScoringXI {
			continue
		}
		xi = append(xi, ScoredPlayer{
			Element:      p.Element,
			Pos:          p.Pos,
			Slot:         p.Slot,
			Points:       p.Points,
			Minutes:      p.Minutes,
			AutoSubbedIn: p.AutoSubbedIn,
		})
	}
	return ManagerScore{EntryID: sq.EntryID, GW: sq.GW, Total: sq.Total, Players: xi}, nil
}
