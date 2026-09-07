package fpl

import (
	"bytes"
	"encoding/json"
	"strconv"
	"strings"
	"time"
)

// num holds a numeric field that the Draft API sends as a JSON string in
// bootstrap-static ("4.0", "0.00") but as a JSON number in /event/{gw}/live
// (0.0), and frequently as null. It always unmarshals to a float64; a missing,
// null, or empty value becomes 0.
type num float64

// Float returns the value as a plain float64.
func (n num) Float() float64 { return float64(n) }

func (n *num) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), ` "`)
	if s == "" || s == "null" {
		*n = 0
		return nil
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return err
	}
	*n = num(f)
	return nil
}

// apiTime is an RFC3339 timestamp that tolerates null and "" (several Draft
// event fields are nullable). A zero apiTime means "not set".
type apiTime time.Time

// Time returns the value as a time.Time (zero when unset).
func (t apiTime) Time() time.Time { return time.Time(t) }

func (t *apiTime) UnmarshalJSON(b []byte) error {
	s := strings.Trim(strings.TrimSpace(string(b)), `"`)
	if s == "" || s == "null" {
		*t = apiTime(time.Time{})
		return nil
	}
	parsed, err := time.Parse(time.RFC3339, s)
	if err != nil {
		// bootstrap event timestamps sometimes carry sub-second precision.
		parsed, err = time.Parse(time.RFC3339Nano, s)
		if err != nil {
			return err
		}
	}
	*t = apiTime(parsed.UTC())
	return nil
}

// ---------------------------------------------------------------------------
// /api/game
// ---------------------------------------------------------------------------

// Game is the small polling endpoint that drives refresh cadence: which GW is
// current, whether it has finished, and whether this GW's waivers have run.
type Game struct {
	CurrentEvent          int    `json:"current_event"`
	CurrentEventFinished  bool   `json:"current_event_finished"`
	NextEvent             int    `json:"next_event"`
	WaiversProcessed      bool   `json:"waivers_processed"`
	ProcessingStatus      string `json:"processing_status"`
	TradesTimeForApproval bool   `json:"trades_time_for_approval"`
}

// ---------------------------------------------------------------------------
// /api/bootstrap-static
// ---------------------------------------------------------------------------

// Bootstrap is the season-static reference data: every player, the position and
// club lookups, the 38-event calendar, and the squad rules used for auto-subs.
type Bootstrap struct {
	Elements     []Element     `json:"elements"`
	ElementTypes []ElementType `json:"element_types"`
	Teams        []Team        `json:"teams"`
	Events       []Event       `json:"-"` // flattened from the events object below
	Settings     Settings      `json:"settings"`
}

// bootstrapWire matches the raw payload; events is an object, not an array.
type bootstrapWire struct {
	Elements     []Element     `json:"elements"`
	ElementTypes []ElementType `json:"element_types"`
	Teams        []Team        `json:"teams"`
	Events       struct {
		Data []Event `json:"data"`
	} `json:"events"`
	Settings Settings `json:"settings"`
}

func (b *Bootstrap) UnmarshalJSON(raw []byte) error {
	var w bootstrapWire
	if err := json.Unmarshal(raw, &w); err != nil {
		return err
	}
	b.Elements = w.Elements
	b.ElementTypes = w.ElementTypes
	b.Teams = w.Teams
	b.Events = w.Events.Data
	b.Settings = w.Settings
	return nil
}

// Element is one Draft player. Numeric-text fields (form, points_per_game, the
// expected_* family) are parsed to numbers on ingest via the num type.
type Element struct {
	ID            ElementID `json:"id"`
	WebName       string    `json:"web_name"`
	FirstName     string    `json:"first_name"`
	SecondName    string    `json:"second_name"`
	ElementType   int       `json:"element_type"` // -> ElementType.ID (1..4)
	Team          int       `json:"team"`         // -> Team.ID
	Code          int       `json:"code"`
	Status        string    `json:"status"` // a/d/i/s/u
	DraftRank     int       `json:"draft_rank"`
	TotalPoints   int       `json:"total_points"`
	EventPoints   int       `json:"event_points"`
	Minutes       int       `json:"minutes"`
	GoalsScored   int       `json:"goals_scored"`
	Assists       int       `json:"assists"`
	CleanSheets   int       `json:"clean_sheets"`
	GoalsConceded int       `json:"goals_conceded"`
	Bonus         int       `json:"bonus"`
	BPS           int       `json:"bps"`
	Saves         int       `json:"saves"`
	YellowCards   int       `json:"yellow_cards"`
	RedCards      int       `json:"red_cards"`
	OwnGoals      int       `json:"own_goals"`
	Starts        int       `json:"starts"`

	Form                     num `json:"form"`
	PointsPerGame            num `json:"points_per_game"`
	ExpectedGoals            num `json:"expected_goals"`
	ExpectedAssists          num `json:"expected_assists"`
	ExpectedGoalInvolvements num `json:"expected_goal_involvements"`
	ExpectedGoalsConceded    num `json:"expected_goals_conceded"`
}

// ElementType is a position lookup row (1=GK, 2=DEF, 3=MID, 4=FWD).
type ElementType struct {
	ID         int    `json:"id"`
	Plural     string `json:"plural"`      // "Goalkeepers"
	Singular   string `json:"singular"`    // "Goalkeeper"
	PluralName string `json:"plural_name"` // "goalkeepers"
}

// Team is a Premier League club lookup row.
type Team struct {
	ID        int    `json:"id"`
	Code      int    `json:"code"`
	Name      string `json:"name"`
	ShortName string `json:"short_name"`
}

// Event is one gameweek in the 38-event calendar. The Draft API sends these in a
// 0-indexed array whose items carry a 1-indexed id; always look up by id.
type Event struct {
	ID           int     `json:"id"`
	Name         string  `json:"name"`
	DeadlineTime apiTime `json:"deadline_time"`
	Finished     bool    `json:"finished"`
	TradesTime   apiTime `json:"trades_time"`
	WaiversTime  apiTime `json:"waivers_time"`
}

// Settings carries only the squad rules — the one part of settings the MVP
// parses, and only for auto-sub validity (ticket 03).
type Settings struct {
	Squad SquadSettings `json:"squad"`
}

// SquadSettings is settings.squad: composition counts, the play limit, the
// per-position minimums, and the position 12 backup-GK lock.
type SquadSettings struct {
	Size              int               `json:"size"`
	SelectGKP         int               `json:"select_GKP"`
	SelectDEF         int               `json:"select_DEF"`
	SelectMID         int               `json:"select_MID"`
	SelectFWD         int               `json:"select_FWD"`
	Play              int               `json:"play"`
	MinPlayGKP        int               `json:"min_play_GKP"`
	MaxPlayGKP        int               `json:"max_play_GKP"`
	MinPlayDEF        int               `json:"min_play_DEF"`
	MinPlayMID        int               `json:"min_play_MID"`
	MinPlayFWD        int               `json:"min_play_FWD"`
	PositionTypeLocks map[string]string `json:"position_type_locks"`
	CaptainsDisabled  bool              `json:"captains_disabled"`
}

// ---------------------------------------------------------------------------
// /api/league/{id}/details
// ---------------------------------------------------------------------------

// LeagueDetails is the league directory: the league object (mode lives here),
// the membership rows that bridge the two id spaces, the h2h match list (empty
// in classic mode), and the standings table.
type LeagueDetails struct {
	League        League        `json:"league"`
	LeagueEntries []LeagueEntry `json:"league_entries"`
	Matches       []Match       `json:"matches"`
	Standings     []Standing    `json:"standings"`
}

// League is the league configuration. Scoring ("c"/"h") is the classic-vs-h2h
// discriminator.
type League struct {
	ID              int    `json:"id"`
	Name            string `json:"name"`
	Scoring         string `json:"scoring"`
	AdminEntry      int    `json:"admin_entry"`
	StartEvent      int    `json:"start_event"`
	StopEvent       int    `json:"stop_event"`
	TransactionMode string `json:"transaction_mode"`
}

// LeagueEntry is one manager's membership row. It is the only place that maps
// the league-membership id space (LeagueEntryID) to the global entry id space
// (EntryID) and to the manager's name.
type LeagueEntry struct {
	ID              LeagueEntryID `json:"id"`       // matches/standings key on this
	EntryID         EntryID       `json:"entry_id"` // element-status/transactions/URLs key on this
	EntryName       string        `json:"entry_name"`
	ShortName       string        `json:"short_name"`
	PlayerFirstName string        `json:"player_first_name"`
	PlayerLastName  string        `json:"player_last_name"`
	WaiverPick      int           `json:"waiver_pick"`
}

// Match is one h2h fixture between two league entries. Present only in h2h
// leagues; classic league payloads omit the matches key entirely.
type Match struct {
	Event              int            `json:"event"`
	Finished           bool           `json:"finished"`
	Started            bool           `json:"started"`
	LeagueEntry1       LeagueEntryID  `json:"league_entry_1"`
	LeagueEntry1Points int            `json:"league_entry_1_points"`
	LeagueEntry2       LeagueEntryID  `json:"league_entry_2"`
	LeagueEntry2Points int            `json:"league_entry_2_points"`
	WinningLeagueEntry *LeagueEntryID `json:"winning_league_entry"`
	WinningMethod      *string        `json:"winning_method"`
}

// Standing is one row of the league table. Classic rows carry Total (cumulative
// score) and EventTotal (this GW); h2h rows carry the matches_*/points_* family
// instead.
type Standing struct {
	Rank        int           `json:"rank"`
	LastRank    int           `json:"last_rank"`
	RankSort    int           `json:"rank_sort"`
	LeagueEntry LeagueEntryID `json:"league_entry"`
	Total       int           `json:"total"`
	EventTotal  int           `json:"event_total"`

	MatchesPlayed int `json:"matches_played"`
	MatchesWon    int `json:"matches_won"`
	MatchesDrawn  int `json:"matches_drawn"`
	MatchesLost   int `json:"matches_lost"`
	PointsFor     int `json:"points_for"`
	PointsAgainst int `json:"points_against"`
}

// ---------------------------------------------------------------------------
// /api/league/{id}/element-status
// ---------------------------------------------------------------------------

// ElementStatus is the element -> owner map. Owner is nil for a free agent.
type ElementStatus struct {
	Element         ElementID `json:"element"`
	Owner           *EntryID  `json:"owner"`
	Status          string    `json:"status"` // "o" owned / "a" available
	InAcceptedTrade bool      `json:"in_accepted_trade"`
}

// ---------------------------------------------------------------------------
// /api/draft/league/{id}/transactions
// ---------------------------------------------------------------------------

// Transaction is one waiver / free-agent / trade movement. Entry keys on the
// global EntryID space.
type Transaction struct {
	ID         int       `json:"id"`
	Entry      EntryID   `json:"entry"`
	Event      int       `json:"event"`
	ElementIn  ElementID `json:"element_in"`
	ElementOut ElementID `json:"element_out"`
	Kind       string    `json:"kind"`   // "w" waiver / "f" free agent / trade kinds
	Result     string    `json:"result"` // "a" accepted / "do" denied-other / ...
	Priority   int       `json:"priority"`
	Index      int       `json:"index"`
	Added      apiTime   `json:"added"`
}

// ---------------------------------------------------------------------------
// /api/event/{gw}/live
// ---------------------------------------------------------------------------

// LiveGW is the live scoring feed for one gameweek: per-player accumulated
// stats and the fixture list with per-fixture stat breakdowns.
type LiveGW struct {
	Elements map[ElementID]LiveElement `json:"elements"`
	Fixtures []LiveFixture             `json:"fixtures"`
}

// liveGWWire matches the raw payload; elements is an object keyed by element id
// as a string.
type liveGWWire struct {
	Elements map[string]LiveElement `json:"elements"`
	Fixtures []LiveFixture          `json:"fixtures"`
}

func (l *LiveGW) UnmarshalJSON(raw []byte) error {
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		l.Elements = map[ElementID]LiveElement{}
		l.Fixtures = nil
		return nil
	}
	var w liveGWWire
	if err := json.Unmarshal(raw, &w); err != nil {
		return err
	}
	l.Elements = make(map[ElementID]LiveElement, len(w.Elements))
	for k, v := range w.Elements {
		id, err := strconv.Atoi(k)
		if err != nil {
			return err
		}
		l.Elements[ElementID(id)] = v
	}
	l.Fixtures = w.Fixtures
	return nil
}

// LiveElement is one player's live line for the GW. Stats are already summed
// across all of the player's fixtures in the GW (relevant for double
// gameweeks). Explain links the line back to the fixture(s) that produced it,
// which is how auto-subs tell whether a blanking starter's match is over.
type LiveElement struct {
	Stats   LiveStats     `json:"stats"`
	Explain []LiveExplain `json:"explain"`
}

// LiveExplain is one fixture's contribution to a player's live line. Only the
// fixture id is needed here — to join to LiveFixture.FinishedProvisional.
type LiveExplain struct {
	Fixture int `json:"fixture"`
}

// LiveStats is the accumulated live scoring line. total_points and bonus are
// trusted as-is; there is no bonus recompute.
type LiveStats struct {
	Minutes               int `json:"minutes"`
	GoalsScored           int `json:"goals_scored"`
	Assists               int `json:"assists"`
	CleanSheets           int `json:"clean_sheets"`
	GoalsConceded         int `json:"goals_conceded"`
	OwnGoals              int `json:"own_goals"`
	PenaltiesSaved        int `json:"penalties_saved"`
	PenaltiesMissed       int `json:"penalties_missed"`
	YellowCards           int `json:"yellow_cards"`
	RedCards              int `json:"red_cards"`
	Saves                 int `json:"saves"`
	Bonus                 int `json:"bonus"`
	BPS                   int `json:"bps"`
	Starts                int `json:"starts"`
	DefensiveContribution int `json:"defensive_contribution"`
	TotalPoints           int `json:"total_points"`
}

// LiveFixture is one PL match in the GW with its per-stat home/away breakdown.
type LiveFixture struct {
	ID                  int           `json:"id"`
	Event               int           `json:"event"`
	TeamH               int           `json:"team_h"`
	TeamA               int           `json:"team_a"`
	TeamHScore          *int          `json:"team_h_score"`
	TeamAScore          *int          `json:"team_a_score"`
	Started             bool          `json:"started"`
	Finished            bool          `json:"finished"`
	FinishedProvisional bool          `json:"finished_provisional"`
	Minutes             int           `json:"minutes"`
	KickoffTime         apiTime       `json:"kickoff_time"`
	Stats               []FixtureStat `json:"stats"`
}

// FixtureStat is one named stat for a fixture, split into home (H) and away (A)
// per-player values. Parsed as part of the live payload shape; the goalscorer
// stats feed /overview (ticket 09). Scoring never touches this — total_points
// and bonus are trusted from LiveStats as-is, with no bonus recompute.
type FixtureStat struct {
	S string      `json:"s"`
	H []StatValue `json:"h"`
	A []StatValue `json:"a"`
}

// StatValue is one player's value for a fixture stat.
type StatValue struct {
	Element ElementID `json:"element"`
	Value   int       `json:"value"`
}

// ---------------------------------------------------------------------------
// /api/entry/{entryId}/event/{gw}
// ---------------------------------------------------------------------------

// EntryEvent is one manager's team for one GW: the 15 submitted picks and the
// applied auto-subs (populated once the GW is finalised).
type EntryEvent struct {
	Picks []Pick `json:"picks"`
	Subs  []Sub  `json:"subs"`
}

// Pick is one squad slot. Position 1-11 is the submitted XI, 12-15 the bench
// (12 is the locked backup GK).
//
// Pos is not in the wire payload: ManagerScore resolves it from
// bootstrap-static before handing the picks to ApplyAutoSubs, which needs each
// player's position to keep the auto-subbed XI a valid formation.
type Pick struct {
	Element       ElementID `json:"element"`
	Position      int       `json:"position"`
	IsCaptain     bool      `json:"is_captain"`
	IsViceCaptain bool      `json:"is_vice_captain"`
	Multiplier    int       `json:"multiplier"`

	Pos Pos `json:"-"`
}

// Sub is one applied auto-substitution.
type Sub struct {
	ElementIn  ElementID `json:"element_in"`
	ElementOut ElementID `json:"element_out"`
	Event      int       `json:"event"`
}
