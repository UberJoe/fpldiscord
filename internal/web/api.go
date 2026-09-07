package web

import (
	"fmt"
	"net/http"
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
