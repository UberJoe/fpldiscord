// Package fpl holds one immutable in-memory snapshot of the Draft FPL API,
// rebuilt on a schedule by a single refresher goroutine and served last-good
// without locks. Every downstream reader (Discord commands, the /api/* surface)
// is a pure function over the current snapshot, so all Draft API drift is
// absorbed in this one package.
//
// Ticket 02 builds the spine: typed structs for every endpoint the MVP reads,
// the join indexes between the two id spaces, event look-up by id, numeric-text
// fields parsed on ingest, accent-stripped name slices for autocomplete, the
// MatchLive / LeagueMode derivations, and the refresher with per-endpoint TTLs.
// The six derived views and ApplyAutoSubs arrive in tickets 03+.
package fpl

import (
	"time"
)

// Distinct id spaces. league_entries.id (LeagueEntryID) is NOT entry_id
// (EntryID); matches/standings key on LeagueEntryID, while element-status.owner,
// transactions.entry and /entry/{id}/... URLs key on EntryID. Modelling them as
// separate types makes a conflation a compile error.
type (
	// ElementID is a Draft player id (elements[].id).
	ElementID int
	// EntryID identifies a manager's entry (entry_id) — the global team id.
	EntryID int
	// LeagueEntryID identifies a manager's membership row in this league
	// (league_entries[].id).
	LeagueEntryID int
)

// Pos is a playing position, numbered as element_type is in bootstrap-static
// (1=GK, 2=DEF, 3=MID, 4=FWD). Auto-sub selection reasons about squad shape in
// these terms.
type Pos int

const (
	PosGK  Pos = 1
	PosDEF Pos = 2
	PosMID Pos = 3
	PosFWD Pos = 4
)

// LeagueMode is derived from league.scoring ("c" / "h"), never configured.
type LeagueMode string

const (
	ModeClassic LeagueMode = "classic"
	ModeH2H     LeagueMode = "h2h"
)

// modeFromScoring maps the league.scoring discriminator onto a LeagueMode.
// Anything other than "h" is treated as classic.
func modeFromScoring(scoring string) LeagueMode {
	if scoring == "h" {
		return ModeH2H
	}
	return ModeClassic
}

// PlayerName is one autocomplete candidate for a player-name argument: the
// display name plus its accent-stripped form for matching.
type PlayerName struct {
	ID       ElementID
	WebName  string // display form, e.g. "Højlund"
	Stripped string // match form, e.g. "Hojlund"
}

// OwnerName is one autocomplete candidate for an owner-name argument.
type OwnerName struct {
	EntryID  EntryID
	Name     string // display form (player_first_name)
	Stripped string
}

// Snapshot is one immutable point-in-time view of the Draft FPL API. It is only
// ever published through Store; readers must treat every field as read-only.
type Snapshot struct {
	BuiltAt time.Time
	// Stale is true when the most recent refresh cycle had at least one fetch
	// failure and this snapshot carries some previous-cycle data. Always false
	// for a fully successful build.
	Stale bool

	// Parsed endpoint payloads.
	Game          Game
	Bootstrap     Bootstrap
	LeagueDetails LeagueDetails
	ElementStatus []ElementStatus
	Transactions  []Transaction
	// Live is keyed by GW id. Ticket 02 populates only the current GW; past-GW
	// entries are added lazily by later tickets.
	Live map[int]LiveGW
	// Entries is each league member's team for the current GW, keyed by the
	// global EntryID.
	Entries map[EntryID]EntryEvent

	// Derived scalars, cheap to precompute once per build.
	LeagueName   string
	LeagueMode   LeagueMode
	CurrentGW    int
	GWFinished   bool
	ElementCount int

	// Autocomplete candidate slices, accent-stripped on build.
	PlayerNames []PlayerName
	OwnerNames  []OwnerName

	// Join indexes. Unexported: readers go through the accessor methods so the
	// two id spaces are never conflated by hand.
	elementByID          map[ElementID]*Element
	elementTypeByID      map[int]*ElementType
	teamByID             map[int]*Team
	eventByID            map[int]*Event
	entryByEntryID       map[EntryID]*LeagueEntry
	entryByLeagueEntryID map[LeagueEntryID]*LeagueEntry
	ownerByElement       map[ElementID]*EntryID
}

// pieces is the set of raw parsed payloads the refresher keeps between cycles so
// it can refetch endpoints independently on their own TTLs and reassemble a
// Snapshot without re-hitting everything.
type pieces struct {
	game      Game
	bootstrap Bootstrap
	details   LeagueDetails
	status    []ElementStatus
	txns      []Transaction
	live      LiveGW
	entries   map[EntryID]EntryEvent
	currentGW int
}

// resolveCurrentGW returns the GW to treat as current: game.current_event, or
// next_event when current_event is null/0 before the season's first deadline.
func resolveCurrentGW(g Game) int {
	if g.CurrentEvent != 0 {
		return g.CurrentEvent
	}
	return g.NextEvent
}

// assemble builds an immutable Snapshot from a set of raw pieces: it wires the
// join indexes, precomputes the derived scalars, derives MatchLive/LeagueMode,
// and builds the accent-stripped autocomplete slices. It is pure — no I/O — so
// it is exercised directly from fixtures.
func assemble(p pieces, builtAt time.Time, stale bool) *Snapshot {
	s := &Snapshot{
		BuiltAt:       builtAt,
		Stale:         stale,
		Game:          p.game,
		Bootstrap:     p.bootstrap,
		LeagueDetails: p.details,
		ElementStatus: p.status,
		Transactions:  p.txns,
		Live:          map[int]LiveGW{},
		Entries:       p.entries,

		LeagueName:   p.details.League.Name,
		LeagueMode:   modeFromScoring(p.details.League.Scoring),
		CurrentGW:    p.currentGW,
		GWFinished:   p.game.CurrentEventFinished,
		ElementCount: len(p.bootstrap.Elements),
	}
	if p.currentGW != 0 {
		s.Live[p.currentGW] = p.live
	}
	if s.Entries == nil {
		s.Entries = map[EntryID]EntryEvent{}
	}

	// Join indexes over the parsed payloads. Events come back 0-indexed with a
	// 1-indexed id field, so keying on ev.ID (not slice position) is what makes
	// Event(gw) correct.
	s.elementByID = indexBy(s.Bootstrap.Elements, func(e *Element) ElementID { return e.ID })
	s.elementTypeByID = indexBy(s.Bootstrap.ElementTypes, func(t *ElementType) int { return t.ID })
	s.teamByID = indexBy(s.Bootstrap.Teams, func(t *Team) int { return t.ID })
	s.eventByID = indexBy(s.Bootstrap.Events, func(e *Event) int { return e.ID })
	s.entryByEntryID = indexBy(s.LeagueDetails.LeagueEntries, func(le *LeagueEntry) EntryID { return le.EntryID })
	s.entryByLeagueEntryID = indexBy(s.LeagueDetails.LeagueEntries, func(le *LeagueEntry) LeagueEntryID { return le.ID })

	s.ownerByElement = make(map[ElementID]*EntryID, len(s.ElementStatus))
	for i := range s.ElementStatus {
		es := &s.ElementStatus[i]
		s.ownerByElement[es.Element] = es.Owner
	}

	s.PlayerNames = buildPlayerNames(s.Bootstrap.Elements)
	s.OwnerNames = buildOwnerNames(s.LeagueDetails.LeagueEntries)

	return s
}

// indexBy returns a map from key(&s[i]) to &s[i] for every element of s. The
// pointers alias the caller's slice, which is safe here because an assembled
// Snapshot is immutable once published.
func indexBy[T any, K comparable](s []T, key func(*T) K) map[K]*T {
	m := make(map[K]*T, len(s))
	for i := range s {
		row := &s[i]
		m[key(row)] = row
	}
	return m
}

func buildPlayerNames(els []Element) []PlayerName {
	out := make([]PlayerName, 0, len(els))
	for _, e := range els {
		out = append(out, PlayerName{
			ID:       e.ID,
			WebName:  e.WebName,
			Stripped: stripAccents(e.WebName),
		})
	}
	return out
}

func buildOwnerNames(entries []LeagueEntry) []OwnerName {
	out := make([]OwnerName, 0, len(entries))
	for _, le := range entries {
		out = append(out, OwnerName{
			EntryID:  le.EntryID,
			Name:     le.PlayerFirstName,
			Stripped: stripAccents(le.PlayerFirstName),
		})
	}
	return out
}

// Element returns the player row for id.
func (s *Snapshot) Element(id ElementID) (*Element, bool) {
	e, ok := s.elementByID[id]
	return e, ok
}

// ElementType returns the position lookup row for a 1..4 element_type id.
func (s *Snapshot) ElementType(id int) (*ElementType, bool) {
	t, ok := s.elementTypeByID[id]
	return t, ok
}

// Team returns the club lookup row for a team id.
func (s *Snapshot) Team(id int) (*Team, bool) {
	t, ok := s.teamByID[id]
	return t, ok
}

// Event returns the calendar row for a 1-indexed gameweek id — never by array
// position.
func (s *Snapshot) Event(id int) (*Event, bool) {
	ev, ok := s.eventByID[id]
	return ev, ok
}

// EntryByLeagueEntry resolves a LeagueEntryID (matches/standings space) to the
// membership row.
func (s *Snapshot) EntryByLeagueEntry(id LeagueEntryID) (*LeagueEntry, bool) {
	le, ok := s.entryByLeagueEntryID[id]
	return le, ok
}

// EntryByEntryID resolves a global EntryID (element-status/transactions/URL
// space) to the membership row.
func (s *Snapshot) EntryByEntryID(id EntryID) (*LeagueEntry, bool) {
	le, ok := s.entryByEntryID[id]
	return le, ok
}

// OwnerOf returns the league member who owns element id, joining
// element_status.owner (an EntryID) to league_entries. ok is false for a free
// agent or an unknown element.
func (s *Snapshot) OwnerOf(id ElementID) (*LeagueEntry, bool) {
	owner, ok := s.ownerByElement[id]
	if !ok || owner == nil {
		return nil, false
	}
	le, ok := s.entryByEntryID[*owner]
	return le, ok
}

// LiveGW returns the live feed for a gameweek id, if the snapshot carries it.
func (s *Snapshot) LiveGW(gw int) (LiveGW, bool) {
	l, ok := s.Live[gw]
	return l, ok
}

// MatchLive reports whether any fixture in the current GW has started and is not
// yet finished-provisional — the single signal for refresher cadence and the
// client poll-interval hint.
func (s *Snapshot) MatchLive() bool {
	live, ok := s.Live[s.CurrentGW]
	if !ok {
		return false
	}
	for _, f := range live.Fixtures {
		if f.Started && !f.FinishedProvisional {
			return true
		}
	}
	return false
}
