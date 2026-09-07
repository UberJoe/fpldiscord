package fpl

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// defaultBaseURL is the root for every Draft FPL endpoint. All are public — no
// login, cookie, or key. Overridable in tests.
const defaultBaseURL = "https://draft.premierleague.com/api"

// userAgent is a real identifying UA. A blank or Go-default UA occasionally
// trips the WAF in front of the PL estate.
const userAgent = "fpldiscord/2.0 (+https://github.com/UberJoe/fpldiscord)"

// maxBodyBytes caps a single response read; bootstrap-static is ~1 MB.
const maxBodyBytes = 16 << 20

// Client fetches and parses payloads from the Draft FPL API. One shared
// keep-alive http.Client, sequential requests.
type Client struct {
	http     *http.Client
	baseURL  string
	leagueID string
}

// NewClient builds a Client for one league id.
func NewClient(leagueID string) *Client {
	return &Client{
		http:     &http.Client{Timeout: 15 * time.Second},
		baseURL:  defaultBaseURL,
		leagueID: leagueID,
	}
}

// Build performs one full pass over every endpoint the MVP reads and returns an
// immutable Snapshot. The supplied context bounds the whole pass; the boot
// sequence gives it the snapshot-#1 budget.
func (c *Client) Build(ctx context.Context) (*Snapshot, error) {
	p, err := c.fetchAll(ctx)
	if err != nil {
		return nil, err
	}
	return assemble(p, time.Now().UTC(), false), nil
}

// fetchAll pulls every endpoint in dependency order (game and details first,
// because they decide the current GW and the league membership to iterate) and
// returns the raw parsed pieces.
func (c *Client) fetchAll(ctx context.Context) (pieces, error) {
	var p pieces
	var err error

	if p.game, err = c.fetchGame(ctx); err != nil {
		return pieces{}, err
	}
	if p.bootstrap, err = c.fetchBootstrap(ctx); err != nil {
		return pieces{}, err
	}
	if p.details, err = c.fetchDetails(ctx); err != nil {
		return pieces{}, err
	}
	if p.status, err = c.fetchElementStatus(ctx); err != nil {
		return pieces{}, err
	}
	if p.txns, err = c.fetchTransactions(ctx); err != nil {
		return pieces{}, err
	}

	p.currentGW = resolveCurrentGW(p.game)
	if p.currentGW != 0 {
		if p.live, err = c.fetchLive(ctx, p.currentGW); err != nil {
			return pieces{}, err
		}
	}

	p.entries, err = c.fetchEntries(ctx, p.details.LeagueEntries, p.currentGW)
	if err != nil {
		return pieces{}, err
	}

	return p, nil
}

func (c *Client) fetchGame(ctx context.Context) (Game, error) {
	var g Game
	err := c.getJSON(ctx, "/game", &g)
	return g, wrap("game", err)
}

func (c *Client) fetchBootstrap(ctx context.Context) (Bootstrap, error) {
	var b Bootstrap
	err := c.getJSON(ctx, "/bootstrap-static", &b)
	return b, wrap("bootstrap-static", err)
}

func (c *Client) fetchDetails(ctx context.Context) (LeagueDetails, error) {
	var d LeagueDetails
	err := c.getJSON(ctx, "/league/"+c.leagueID+"/details", &d)
	return d, wrap("league details", err)
}

func (c *Client) fetchElementStatus(ctx context.Context) ([]ElementStatus, error) {
	var body struct {
		ElementStatus []ElementStatus `json:"element_status"`
	}
	err := c.getJSON(ctx, "/league/"+c.leagueID+"/element-status", &body)
	return body.ElementStatus, wrap("element-status", err)
}

func (c *Client) fetchTransactions(ctx context.Context) ([]Transaction, error) {
	var body struct {
		Transactions []Transaction `json:"transactions"`
	}
	// Note the "draft/" segment — the un-prefixed path is a 404.
	err := c.getJSON(ctx, "/draft/league/"+c.leagueID+"/transactions", &body)
	return body.Transactions, wrap("transactions", err)
}

func (c *Client) fetchLive(ctx context.Context, gw int) (LiveGW, error) {
	var l LiveGW
	err := c.getJSON(ctx, "/event/"+strconv.Itoa(gw)+"/live", &l)
	return l, wrap("event/"+strconv.Itoa(gw)+"/live", err)
}

func (c *Client) fetchEntryEvent(ctx context.Context, entry EntryID, gw int) (EntryEvent, error) {
	var e EntryEvent
	path := "/entry/" + strconv.Itoa(int(entry)) + "/event/" + strconv.Itoa(gw)
	err := c.getJSON(ctx, path, &e)
	return e, wrap(path, err)
}

// fetchEntries pulls every league member's team for the current GW, one request
// at a time (polite; a private league has under a dozen members).
func (c *Client) fetchEntries(ctx context.Context, entries []LeagueEntry, gw int) (map[EntryID]EntryEvent, error) {
	out := make(map[EntryID]EntryEvent, len(entries))
	if gw == 0 {
		return out, nil
	}
	for _, le := range entries {
		ev, err := c.fetchEntryEvent(ctx, le.EntryID, gw)
		if err != nil {
			return nil, err
		}
		out[le.EntryID] = ev
	}
	return out, nil
}

// getJSON issues a GET against baseURL+path, refuses anything that is not a 200
// with a JSON content-type before it reaches json.Unmarshal, and decodes into v.
// Accept-Encoding is left unset so net/http's transport transparently negotiates
// and decompresses gzip.
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

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
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

func wrap(endpoint string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", endpoint, err)
}
