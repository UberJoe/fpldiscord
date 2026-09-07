package web

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/UberJoe/fpldiscord/internal/fpl"
)

// Poll-interval hints handed to the client in meta.pollAfterMs: fast while a
// match is in play, slow otherwise. The client owns cadence from this single
// server-side source.
const (
	pollLiveMs = 20000
	pollIdleMs = 60000
)

// envelope is the shape of every /api/* 200: identical meta across endpoints,
// built once per request from the current snapshot, plus an endpoint-specific
// data payload.
type envelope struct {
	Meta meta `json:"meta"`
	Data any  `json:"data"`
}

// meta is the shared header block. All keys are camelCase; ids, gameweeks and
// intervals are numbers; builtAt is the only timestamp anywhere in the payload.
type meta struct {
	MatchLive    bool   `json:"matchLive"`
	PollAfterMs  int    `json:"pollAfterMs"`
	Stale        bool   `json:"stale"`
	BuiltAt      string `json:"builtAt"`
	LeagueName   string `json:"leagueName"`
	LeagueMode   string `json:"leagueMode"`
	CurrentGw    int    `json:"currentGw"`
	GwFinished   bool   `json:"gwFinished"`
	ProcessedGws []int  `json:"processedGws"`
}

// buildMeta projects the current snapshot onto the shared envelope header.
func buildMeta(snap *fpl.Snapshot) meta {
	live := snap.MatchLive()
	poll := pollIdleMs
	if live {
		poll = pollLiveMs
	}
	gws := snap.ProcessedGWs()
	if gws == nil {
		gws = []int{}
	}
	return meta{
		MatchLive:    live,
		PollAfterMs:  poll,
		Stale:        snap.Stale,
		BuiltAt:      snap.BuiltAt.UTC().Format(time.RFC3339),
		LeagueName:   snap.LeagueName,
		LeagueMode:   string(snap.LeagueMode),
		CurrentGw:    snap.CurrentGW,
		GwFinished:   snap.GWFinished,
		ProcessedGws: gws,
	}
}

// respondAPI writes a standard /api/* response: the shared envelope wrapped
// around data, with Cache-Control and an ETag, honouring If-None-Match with a
// bodiless 304. Handlers call this once they have built their data and ETag.
func respondAPI(w http.ResponseWriter, r *http.Request, snap *fpl.Snapshot, etag string, data any) {
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("ETag", etag)
	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	writeJSON(w, http.StatusOK, envelope{Meta: buildMeta(snap), Data: data})
}

// requireSnapshot returns the current snapshot, or writes the pre-first-snapshot
// 503 and returns nil. That 503 is the only non-200 an /api/* endpoint gives:
// it can only happen in the <=30s boot window before snapshot #1. Once a
// snapshot exists every response is 200 and upstream trouble shows only as
// meta.stale.
func requireSnapshot(w http.ResponseWriter, snap SnapshotProvider) *fpl.Snapshot {
	if cur := snap.Current(); cur != nil {
		return cur
	}
	w.Header().Set("Retry-After", "3")
	writeJSON(w, http.StatusServiceUnavailable, map[string]any{
		"error":        "starting up",
		"retryAfterMs": 3000,
	})
	return nil
}

// standingsData is GET /api/standings -> data: the server-sorted classic league
// table with live-within-gameweek ranks. In h2h mode Rows is empty (never nil,
// so it marshals as []).
type standingsData struct {
	Rows []standingsRow `json:"rows"`
}

// standingsRow is the camelCase JSON projection of fpl.LiveStandingRow — already
// ordered by live points desc by fpl. The client renders array order and does
// no league-wide maths.
type standingsRow struct {
	EntryID      int    `json:"entryId"`
	OwnerName    string `json:"ownerName"`
	EntryName    string `json:"entryName"`
	OfficialRank int    `json:"officialRank"`
	LiveRank     int    `json:"liveRank"`
	Arrow        int    `json:"arrow"` // officialRank - liveRank; positive = moved up
	TotalPoints  int    `json:"totalPoints"`
	LiveGwPoints int    `json:"liveGwPoints"`
	LivePoints   int    `json:"livePoints"`
}

func (s *Server) handleStandings(w http.ResponseWriter, r *http.Request) {
	snap := requireSnapshot(w, s.snap)
	if snap == nil {
		return
	}
	etag := fmt.Sprintf("\"standings-%d\"", snap.BuiltAt.Unix())
	respondAPI(w, r, snap, etag, buildStandings(snap))
}

// managerData is GET /api/manager/{entryId} -> data: one manager's full
// gameweek squad with auto-subs resolved. Standalone — no fixtures, goalscorer
// lists or bonus breakdown. Only EntryID crosses the wire; LeagueEntryID is
// resolved server-side.
type managerData struct {
	EntryID     int             `json:"entryId"`
	OwnerName   string          `json:"ownerName"`
	GW          int             `json:"gw"`
	Provisional bool            `json:"provisional"`
	Total       int             `json:"total"`
	Players     []managerPlayer `json:"players"`
}

// managerPlayer is the camelCase JSON projection of fpl.SquadPlayer. The client
// groups XI-then-bench and orders within each group by pos then squadSlot.
type managerPlayer struct {
	ElementID     int    `json:"elementId"`
	WebName       string `json:"webName"`
	TeamShort     string `json:"teamShort"`
	Pos           int    `json:"pos"` // 1=GK 2=DEF 3=MID 4=FWD
	SquadSlot     int    `json:"squadSlot"`
	Points        int    `json:"points"`
	Minutes       int    `json:"minutes"`
	InScoringXI   bool   `json:"inScoringXI"`
	AutoSubbedIn  bool   `json:"autoSubbedIn"`
	AutoSubbedOut bool   `json:"autoSubbedOut"`
}

func (s *Server) handleManager(w http.ResponseWriter, r *http.Request) {
	snap := requireSnapshot(w, s.snap)
	if snap == nil {
		return
	}

	id, err := strconv.Atoi(r.PathValue("entryId"))
	if err != nil {
		writeError(w, http.StatusNotFound, "unknown manager")
		return
	}
	entryID := fpl.EntryID(id)

	// Resolve the membership row from the exported league_entries slice rather
	// than the snapshot's unexported index, so this handler can be driven by a
	// hand-built *fpl.Snapshot in tests (matching fpl's own seam-safe views).
	var le *fpl.LeagueEntry
	for i := range snap.LeagueDetails.LeagueEntries {
		if snap.LeagueDetails.LeagueEntries[i].EntryID == entryID {
			le = &snap.LeagueDetails.LeagueEntries[i]
			break
		}
	}
	if le == nil {
		writeError(w, http.StatusNotFound, "unknown manager")
		return
	}

	sq, err := snap.ManagerSquad(entryID, snap.CurrentGW)
	if err != nil {
		// A known manager the snapshot cannot score yet (no picks / no live
		// feed). Rare mid-gameweek; a 404 keeps the endpoint standalone. The
		// internal reason stays server-side — clients get a fixed string like
		// the other 404s.
		s.log.Warn("manager squad unavailable", "entryId", id, "err", err)
		writeError(w, http.StatusNotFound, "manager unavailable")
		return
	}

	etag := fmt.Sprintf("\"manager-%d-%d\"", id, snap.BuiltAt.Unix())
	respondAPI(w, r, snap, etag, buildManager(le, sq))
}

// buildManager re-tags fpl.ManagerSquad as camelCase JSON. ownerName is the
// manager's first name (as on the Standings rows); LeagueEntryID never leaves
// the server.
func buildManager(le *fpl.LeagueEntry, sq fpl.ManagerSquad) managerData {
	players := make([]managerPlayer, 0, len(sq.Players))
	for _, p := range sq.Players {
		players = append(players, managerPlayer{
			ElementID:     int(p.Element),
			WebName:       p.WebName,
			TeamShort:     p.TeamShort,
			Pos:           int(p.Pos),
			SquadSlot:     p.Slot,
			Points:        p.Points,
			Minutes:       p.Minutes,
			InScoringXI:   p.InScoringXI,
			AutoSubbedIn:  p.AutoSubbedIn,
			AutoSubbedOut: p.AutoSubbedOut,
		})
	}
	return managerData{
		EntryID:     int(sq.EntryID),
		OwnerName:   le.PlayerFirstName,
		GW:          sq.GW,
		Provisional: sq.Provisional,
		Total:       sq.Total,
		Players:     players,
	}
}

// waiversData is GET /api/waivers?gw=N -> data: one processed waiver round.
// GW is the resolved gameweek (an out-of-range request is clamped, never a
// 400), echoed back so the client can sync its selector. Rows is never nil.
type waiversData struct {
	GW   int          `json:"gw"`
	Rows []waiversRow `json:"rows"`
}

// waiversRow is the camelCase JSON projection of fpl.WaiverRow, already sorted
// by index by fpl. type is "waiver" / "freeAgent"; status is
// "accepted" / "failed" (fpl resolves the raw Draft codes). priority is on this
// unauthenticated feed so the client can lay out bid order without a login.
type waiversRow struct {
	OwnerName string `json:"ownerName"`
	EntryID   int    `json:"entryId"`
	In        string `json:"in"`
	Out       string `json:"out"`
	Type      string `json:"type"`
	Status    string `json:"status"`
	Priority  int    `json:"priority"`
	Index     int    `json:"index"`
}

func (s *Server) handleWaivers(w http.ResponseWriter, r *http.Request) {
	snap := requireSnapshot(w, s.snap)
	if snap == nil {
		return
	}
	gw := resolveWaiverGW(snap, r.URL.Query().Get("gw"))
	etag := fmt.Sprintf("\"waivers-%d-%d\"", gw, snap.BuiltAt.Unix())
	respondAPI(w, r, snap, etag, buildWaivers(snap, gw))
}

// resolveWaiverGW turns the raw ?gw= query value into the gameweek to serve. An
// absent, empty or non-numeric value resolves to the latest processed
// gameweek; an in-range number is used as-is; an out-of-range number is clamped
// to the processed span (never a 400). With no processed rounds at all the
// request value (or 0) passes through and the rows come back empty.
func resolveWaiverGW(snap *fpl.Snapshot, raw string) int {
	processed := snap.ProcessedGWs()
	latest := 0
	if len(processed) > 0 {
		latest = processed[len(processed)-1]
	}

	n, err := strconv.Atoi(raw)
	if raw == "" || err != nil {
		return latest
	}
	if len(processed) == 0 {
		return n
	}
	if lo := processed[0]; n < lo {
		return lo
	}
	if n > latest {
		return latest
	}
	return n
}

// buildWaivers re-tags fpl.LeagueTransactions(gw) as camelCase JSON, folding the
// accent-stripped webName rule the rest of /api/* follows onto the in/out
// names. Rows stay in fpl's index order.
func buildWaivers(snap *fpl.Snapshot, gw int) waiversData {
	src := snap.LeagueTransactions(gw)
	rows := make([]waiversRow, 0, len(src))
	for _, t := range src {
		rows = append(rows, waiversRow{
			OwnerName: t.OwnerName,
			EntryID:   int(t.EntryID),
			In:        fpl.StripAccents(t.In),
			Out:       fpl.StripAccents(t.Out),
			Type:      string(t.Type),
			Status:    string(t.Status),
			Priority:  t.Priority,
			Index:     t.Index,
		})
	}
	return waiversData{GW: gw, Rows: rows}
}

// buildStandings re-tags fpl.LiveStandings() as camelCase JSON. All ordering,
// the LeagueEntryID->EntryID join and the live-rank maths live in fpl;
// LiveStandings returns nil in h2h mode, which becomes an empty (non-nil) rows
// array here.
func buildStandings(snap *fpl.Snapshot) standingsData {
	src := snap.LiveStandings()
	rows := make([]standingsRow, 0, len(src))
	for _, r := range src {
		rows = append(rows, standingsRow{
			EntryID:      int(r.EntryID),
			OwnerName:    r.OwnerName,
			EntryName:    r.EntryName,
			OfficialRank: r.OfficialRank,
			LiveRank:     r.LiveRank,
			Arrow:        r.Arrow,
			TotalPoints:  r.TotalPoints,
			LiveGwPoints: r.LiveGwPoints,
			LivePoints:   r.LivePoints,
		})
	}
	return standingsData{Rows: rows}
}
