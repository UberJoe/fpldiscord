// Package fpl holds an immutable in-memory snapshot of the Draft FPL API. Every
// downstream reader (Discord commands, the /api/* surface) is a pure function
// over the current snapshot.
//
// Ticket 01 establishes only the spine: the id types, a Snapshot value, a Store
// that hands out the last-good pointer lock-free, and Build() — one pass over the
// public Draft endpoints that gates the HTTP listener at boot. The refresher
// goroutine, per-endpoint TTLs, full struct parsing, the join indexes, and the
// six derived views arrive in ticket 02.
package fpl

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"
	"time"
)

// Base URL for every Draft FPL endpoint. All are public — no login, cookie, or
// key. Overridable in tests.
const defaultBaseURL = "https://draft.premierleague.com/api"

// userAgent is a real identifying UA, per the Draft API research: error bodies
// and edge-cache behaviour are friendlier when the client identifies itself.
const userAgent = "fpldiscord/2.0 (+https://github.com/UberJoe/fpldiscord)"

// Distinct id spaces. league_entries.id (LeagueEntryID) is NOT entry_id
// (EntryID); matches/standings key on LeagueEntryID, while element-status.owner,
// transactions.entry and /entry/{id}/... URLs key on EntryID. Modelling them as
// separate types makes a conflation a compile error.
type (
	// ElementID is a Draft player id (elements[].id).
	ElementID int
	// EntryID identifies a manager's entry (entry_id).
	EntryID int
	// LeagueEntryID identifies a manager's membership row in this league
	// (league_entries[].id).
	LeagueEntryID int
)

// Snapshot is one immutable point-in-time view of the Draft FPL API. It is only
// ever published through Store; readers must treat it as read-only.
type Snapshot struct {
	BuiltAt time.Time
	// Stale is true when the most recent refresh failed and this is the
	// previously-good data served again. Always false for a freshly built
	// snapshot.
	Stale bool

	// Minimal derived fields for the ticket-01 startup summary. Ticket 02
	// replaces these with the fully parsed Game / Bootstrap / LeagueDetails /
	// ElementStatus / Transactions / Live structures plus the join indexes.
	LeagueName   string
	LeagueMode   LeagueMode
	CurrentGW    int
	GWFinished   bool
	ElementCount int
}

// LeagueMode is derived from league.scoring ("c" / "h"), never configured.
type LeagueMode string

const (
	ModeClassic LeagueMode = "classic"
	ModeH2H     LeagueMode = "h2h"
)

// Client fetches and assembles snapshots from the Draft FPL API.
type Client struct {
	http    *http.Client
	baseURL string
	// leagueID is carried verbatim into the /league/{id}/... paths.
	leagueID string
}

// NewClient builds a Client with a shared keep-alive HTTP client.
func NewClient(leagueID string) *Client {
	return &Client{
		http:     &http.Client{Timeout: 15 * time.Second},
		baseURL:  defaultBaseURL,
		leagueID: leagueID,
	}
}

// Build performs one pass over the public Draft endpoints and returns an
// immutable Snapshot. The supplied context bounds the whole pass; callers give
// it the boot snapshot-#1 budget (<=30s).
func (c *Client) Build(ctx context.Context) (*Snapshot, error) {
	var bootstrap struct {
		Elements []json.RawMessage `json:"elements"`
		Events   struct {
			Current int `json:"current"`
		} `json:"events"`
	}
	if err := c.getJSON(ctx, "/bootstrap-static", &bootstrap); err != nil {
		return nil, fmt.Errorf("bootstrap-static: %w", err)
	}

	var game struct {
		CurrentEvent         int  `json:"current_event"`
		CurrentEventFinished bool `json:"current_event_finished"`
		NextEvent            int  `json:"next_event"`
	}
	if err := c.getJSON(ctx, "/game", &game); err != nil {
		return nil, fmt.Errorf("game: %w", err)
	}

	var details struct {
		League struct {
			Name    string `json:"name"`
			Scoring string `json:"scoring"`
		} `json:"league"`
	}
	if err := c.getJSON(ctx, "/league/"+c.leagueID+"/details", &details); err != nil {
		return nil, fmt.Errorf("league details: %w", err)
	}

	currentGW := game.CurrentEvent
	if currentGW == 0 {
		// Null current_event before the season starts — fall back to next.
		currentGW = game.NextEvent
	}

	mode := ModeClassic
	if details.League.Scoring == "h" {
		mode = ModeH2H
	}

	return &Snapshot{
		BuiltAt:      time.Now().UTC(),
		LeagueName:   details.League.Name,
		LeagueMode:   mode,
		CurrentGW:    currentGW,
		GWFinished:   game.CurrentEventFinished,
		ElementCount: len(bootstrap.Elements),
	}, nil
}

// getJSON issues a GET against baseURL+path, refuses anything that is not a
// 200 with a JSON content-type before it reaches json.Unmarshal, and decodes
// into v.
func (c *Client) getJSON(ctx context.Context, path string, v any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !isJSON(ct) {
		return fmt.Errorf("unexpected content-type %q (error bodies may be HTML)", ct)
	}
	if err := json.Unmarshal(body, v); err != nil {
		return fmt.Errorf("decode: %w", err)
	}
	return nil
}

func isJSON(contentType string) bool {
	if contentType == "" {
		return false
	}
	mt, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return false
	}
	return mt == "application/json" || strings.HasSuffix(mt, "+json")
}
