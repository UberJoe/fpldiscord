package bot

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/UberJoe/fpldiscord/internal/fpl"
	"github.com/bwmarrin/discordgo"
)

// waiverResult is the resolved `result` option: which claim set /waivers shows.
// A named string type mirrors overview.go's overviewMode so the value is
// validated once and compared against constants, not scattered string literals.
type waiverResult string

const (
	waiverAccepted waiverResult = "accepted" // the default: the flat accepted-claims table
	waiverFailed   waiverResult = "failed"   // contested groups with at least one failed claim
	waiverAll      waiverResult = "all"      // every contested group
)

// waiverOwnerCap bounds the owner column of the accepted-claims table, mirroring
// standingsNameCap: an owner first name then stays inside the ~30–34 characters a
// Discord mobile client shows before it wraps or side-scrolls.
const waiverOwnerCap = 14

// waiverTableBudget is the byte ceiling for the accepted-claims table body. It
// sits well under Discord's 4096-character description limit (bytes >= runes, and
// the code fence adds a handful more) with margin to spare. A pathological
// free-agent week whose table would exceed it keeps as many whole rows as fit
// and appends a summary line rather than overflowing.
const waiverTableBudget = 4000

// handleWaivers renders a processed waiver round as one or more framed embeds.
// With no arguments it shows the latest processed gameweek's accepted claims;
// `gw` selects a round and `result` (accepted / failed / all) selects which
// claims.
//
// The reply takes one of two shapes keyed on `result`:
//
//   - accepted (the default) is a flat homogeneous list, so it stays a fenced
//     code-block table in one embed's description — owner, the roster move and
//     the claim-kind tag in aligned columns, rows in Index order, no pagination.
//   - failed / all are contested-player groups, so each contested incoming
//     player becomes one non-inline embed field (the bid chain in priority
//     order, winner first) packed by the shared embedFieldChunker, which spills
//     a long round across further messages rather than truncating it.
//
// A processed round is settled and historical, so both shapes use colorNeutral —
// colorFinal green is reserved for "the gameweek has finished". Every short
// reply (startup, no processed rounds, the result validation error, an unknown
// gameweek, an empty result set) stays plain text.
func handleWaivers(in *cmdInput) error {
	if in.snap == nil {
		return in.resp.Respond("Waivers aren't available yet — the bot is still starting up.")
	}
	snap := in.snap

	processed := snap.ProcessedGWs()
	gw, given := in.opts.Int("gw")
	if !given || gw == 0 {
		if len(processed) == 0 {
			return in.resp.Respond("No waiver rounds have been processed yet.")
		}
		gw = processed[len(processed)-1]
	}

	result := waiverAccepted
	if v, ok := in.opts.String("result"); ok {
		switch waiverResult(strings.ToLower(strings.TrimSpace(v))) {
		case waiverAccepted:
			result = waiverAccepted
		case waiverFailed:
			result = waiverFailed
		case waiverAll:
			result = waiverAll
		default:
			return in.resp.Respond(`The "result" option must be accepted, failed or all.`)
		}
	}

	rows := snap.LeagueTransactions(gw)
	if len(rows) == 0 {
		return in.resp.Respond(fmt.Sprintf("Couldn't find any waivers for GW%d.", gw))
	}

	noClaims := func() error {
		return in.resp.Respond(fmt.Sprintf("No %s waiver claims in GW%d.", result, gw))
	}

	if result == waiverAccepted {
		accepted := acceptedWaiverRows(rows)
		if len(accepted) == 0 {
			return noClaims()
		}
		e := renderWaiversAccepted(snap.LeagueName, snap.BuiltAt, gw, result, accepted)
		return in.resp.RespondEmbeds([]*discordgo.MessageEmbed{e})
	}

	groups := contestedWaiverGroups(result, rows)
	if len(groups) == 0 {
		return noClaims()
	}
	for _, embeds := range renderWaiversContested(snap.LeagueName, snap.BuiltAt, gw, result, groups) {
		if err := in.resp.RespondEmbeds(embeds); err != nil {
			return err
		}
	}
	return nil
}

// acceptedWaiverRows keeps only the claims that went through, in the Index order
// LeagueTransactions already sorted them into.
func acceptedWaiverRows(rows []fpl.WaiverRow) []fpl.WaiverRow {
	var out []fpl.WaiverRow
	for _, r := range rows {
		if r.Status == fpl.WaiverStatusAccepted {
			out = append(out, r)
		}
	}
	return out
}

// contestedWaiverGroups buckets the rows by the contested incoming player, keyed
// on the element id (not its name, which can collide or be blank), preserving
// first-seen ElementIn order. Each group is sorted into Priority order so the
// winner leads the out-bid chain. In failed mode a group with no failed claim is
// dropped; all mode keeps every group.
func contestedWaiverGroups(result waiverResult, rows []fpl.WaiverRow) [][]fpl.WaiverRow {
	var order []fpl.ElementID
	groups := map[fpl.ElementID][]fpl.WaiverRow{}
	for _, r := range rows {
		if _, seen := groups[r.ElementIn]; !seen {
			order = append(order, r.ElementIn)
		}
		groups[r.ElementIn] = append(groups[r.ElementIn], r)
	}

	var out [][]fpl.WaiverRow
	for _, elem := range order {
		g := groups[elem]
		sort.SliceStable(g, func(i, j int) bool { return g[i].Priority < g[j].Priority })

		if result == waiverFailed {
			failed := false
			for _, r := range g {
				if r.Status == fpl.WaiverStatusFailed {
					failed = true
				}
			}
			if !failed {
				continue
			}
		}
		out = append(out, g)
	}
	return out
}

// renderWaiversAccepted frames the accepted claims as one embed on the shared
// scaffold: league name on the author line, colorNeutral bar, the resolved
// result mode as the footer, the snapshot build time as the Timestamp, and the
// aligned table as a fenced code block in the description.
func renderWaiversAccepted(leagueName string, builtAt time.Time, gw int, result waiverResult, rows []fpl.WaiverRow) *discordgo.MessageEmbed {
	e := dataEmbed(
		leagueName,
		builtAt,
		fmt.Sprintf("GW%d waivers", gw),
		colorNeutral,
		string(result),
	)
	e.Description = codeBlock(waiverAcceptedTable(rows))
	return e
}

// waiverAcceptedTable lays the accepted claims out as space-padded fixed-width
// columns for a monospace code block: the owner (left, capped at waiverOwnerCap),
// the roster move ("Out -> In", or just "In" when nothing was dropped) and the
// claim-kind tag ("(free agent)" for a free-agent pickup, blank for a plain
// waiver), each in its own column. Rows stay in the Index order the caller
// passed. It returns the bare body with no code fence — renderWaiversAccepted
// wraps it via codeBlock, the same split as standingsTable.
//
// The body is guarded against the description limit: once it would pass
// waiverTableBudget it keeps the whole rows so far and appends a
// "…and N more claims" line rather than overflowing. Not expected in practice —
// it takes a pathological free-agent week — but a reply never silently drops
// claims.
func waiverAcceptedTable(rows []fpl.WaiverRow) string {
	ownerCol := make([]string, len(rows))
	moveCol := make([]string, len(rows))
	kindCol := make([]string, len(rows))
	for i, r := range rows {
		ownerCol[i] = capRunes(waiverName(r.OwnerName), waiverOwnerCap)
		moveCol[i] = waiverMove(r)
		kindCol[i] = strings.TrimSpace(waiverKindSuffix(r.Type))
	}

	ownerW := colWidth("Owner", ownerCol)
	moveW := colWidth("Move", moveCol)
	kindW := colWidth("Kind", kindCol)

	line := func(owner, move, kind string) string {
		return strings.TrimRight(
			fmt.Sprintf("%-*s  %-*s  %-*s", ownerW, owner, moveW, move, kindW, kind),
			" ",
		)
	}

	var b strings.Builder
	b.WriteString(line("Owner", "Move", "Kind"))
	kept := 0
	for i := range rows {
		row := "\n" + line(ownerCol[i], moveCol[i], kindCol[i])
		if b.Len()+len(row) > waiverTableBudget {
			break
		}
		b.WriteString(row)
		kept++
	}
	if kept < len(rows) {
		fmt.Fprintf(&b, "\n…and %d more claims", len(rows)-kept)
	}
	return b.String()
}

// renderWaiversContested packs each contested group into one non-inline embed
// field with the shared embedFieldChunker: the field name is the incoming
// player's name, the value is the bid chain — one waiverBidLine per claim in
// Priority order, winner first. The chunker carries the same scaffold as the
// accepted view (league name on the author line, colorNeutral, the result mode
// in the footer, snapshot-time Timestamp), titles only the first embed, and
// spills a long round across further messages exactly as /overview does. The
// return is one []*discordgo.MessageEmbed per Discord message.
func renderWaiversContested(leagueName string, builtAt time.Time, gw int, result waiverResult, groups [][]fpl.WaiverRow) [][]*discordgo.MessageEmbed {
	c := newEmbedFieldChunker(embedFieldScaffold{
		leagueName:     leagueName,
		builtAt:        builtAt,
		title:          fmt.Sprintf("GW%d waivers", gw),
		color:          colorNeutral,
		footer:         string(result),
		titleFirstOnly: true,
	})
	for _, g := range groups {
		var b strings.Builder
		for i, r := range g {
			if i > 0 {
				b.WriteByte('\n')
			}
			b.WriteString(waiverBidLine(r))
		}
		c.add(waiverName(g[0].In), capRunes(b.String(), maxEmbedFieldValue))
	}
	c.flushMessage()
	return c.messages
}

// waiverBidLine is one row within a contested group: the bid slot, owner, the
// move, an accepted/out-bid marker.
func waiverBidLine(r fpl.WaiverRow) string {
	marker := "out-bid"
	if r.Status == fpl.WaiverStatusAccepted {
		marker = "won"
	}
	return fmt.Sprintf("%s %s  %s  [%s%s]", waiverBid(r), waiverName(r.OwnerName), waiverMove(r), marker, waiverKindSuffix(r.Type))
}

// waiverBid is the bid-order slot shown against a claim: "FA" for a free agent
// (no waiver order), "#N" for a real priority, "—" when none is recorded.
func waiverBid(r fpl.WaiverRow) string {
	switch {
	case r.Type == fpl.WaiverTypeFreeAgent:
		return "FA"
	case r.Priority > 0:
		return fmt.Sprintf("#%d", r.Priority)
	default:
		return "—"
	}
}

// waiverMove renders the roster change: "Out -> In", or just "In" when nothing
// was dropped (a free-agent pickup into an open slot).
func waiverMove(r fpl.WaiverRow) string {
	if r.Out == "" {
		return waiverName(r.In)
	}
	return fmt.Sprintf("%s -> %s", waiverName(r.Out), waiverName(r.In))
}

// waiverKindSuffix tags a free-agent pickup; a plain waiver gets no suffix.
func waiverKindSuffix(t fpl.WaiverType) string {
	if t == fpl.WaiverTypeFreeAgent {
		return " (free agent)"
	}
	return ""
}

// waiverName falls back to a placeholder for an element or owner id that did
// not resolve to a name, so a row is never rendered with a blank field.
func waiverName(s string) string {
	if s == "" {
		return "?"
	}
	return s
}

