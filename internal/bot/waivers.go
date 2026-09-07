package bot

import (
	"fmt"
	"sort"
	"strings"

	"github.com/UberJoe/fpldiscord/internal/fpl"
)

// maxDiscordMessage is the ceiling a single /waivers message is packed to. The
// hard Discord limit is 2000 characters; the headroom covers the code-fence
// markers and the bold header on the first message.
const maxDiscordMessage = 1900

// handleWaivers renders a processed waiver round. With no arguments it shows the
// latest processed gameweek's accepted claims; `gw` selects a round and
// `result` (accepted / failed / all) selects which claims. failed and all show
// each contested player's full bid list in priority order, so a reader can see
// who out-bid whom. Long rounds are split across multiple messages rather than
// truncated.
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

	result := "accepted"
	if v, ok := in.opts.String("result"); ok {
		result = strings.ToLower(strings.TrimSpace(v))
	}
	switch result {
	case "accepted", "failed", "all":
	default:
		return in.resp.Respond(`The "result" option must be accepted, failed or all.`)
	}

	rows := snap.LeagueTransactions(gw)
	if len(rows) == 0 {
		return in.resp.Respond(fmt.Sprintf("Couldn't find any waivers for GW%d.", gw))
	}

	blocks := waiverBlocks(result, rows)
	if len(blocks) == 0 {
		return in.resp.Respond(fmt.Sprintf("No %s waiver claims in GW%d.", result, gw))
	}

	header := fmt.Sprintf("**GW%d waivers — %s**", gw, result)
	for _, msg := range packWaiverMessages(header, blocks, maxDiscordMessage) {
		if err := in.resp.Respond(msg); err != nil {
			return err
		}
	}
	return nil
}

// waiverBlocks turns the resolved rows into display blocks. accepted mode is a
// flat list, one line per accepted claim in index order. failed and all mode
// group by the contested incoming player, each group listed in priority order
// (winner first) so the out-bid chain is visible; failed mode drops groups with
// no failed claim.
func waiverBlocks(result string, rows []fpl.WaiverRow) []string {
	if result == "accepted" {
		var blocks []string
		for _, r := range rows {
			if r.Status == fpl.WaiverStatusAccepted {
				blocks = append(blocks, waiverLine(r))
			}
		}
		return blocks
	}

	// Group by the contested incoming player, keyed on the element id (not its
	// name, which can collide or be blank), preserving first-seen index order.
	var order []fpl.ElementID
	groups := map[fpl.ElementID][]fpl.WaiverRow{}
	for _, r := range rows {
		if _, seen := groups[r.ElementIn]; !seen {
			order = append(order, r.ElementIn)
		}
		groups[r.ElementIn] = append(groups[r.ElementIn], r)
	}

	var blocks []string
	for _, elem := range order {
		g := groups[elem]
		sort.SliceStable(g, func(i, j int) bool { return g[i].Priority < g[j].Priority })

		failed := false
		for _, r := range g {
			if r.Status == fpl.WaiverStatusFailed {
				failed = true
			}
		}
		if result == "failed" && !failed {
			continue
		}

		var b strings.Builder
		fmt.Fprintf(&b, "%s:\n", waiverName(g[0].In))
		for _, r := range g {
			b.WriteString("  ")
			b.WriteString(waiverBidLine(r))
			b.WriteByte('\n')
		}
		blocks = append(blocks, strings.TrimRight(b.String(), "\n"))
	}
	return blocks
}

// waiverLine is one accepted claim on a single line: "Owner  Out -> In  (type)".
func waiverLine(r fpl.WaiverRow) string {
	return fmt.Sprintf("%s  %s%s", waiverName(r.OwnerName), waiverMove(r), waiverKindSuffix(r.Type))
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

// packWaiverMessages lays the blocks into as few messages as possible, each a
// monospace code block under limit, with header prepended to the first. A block
// is kept whole where it fits; a block larger than a whole message is split on
// its own line boundaries rather than truncated.
func packWaiverMessages(header string, blocks []string, limit int) []string {
	var msgs []string
	var cur strings.Builder
	started := false

	openMsg := func() {
		cur.Reset()
		if !started {
			cur.WriteString(header)
			cur.WriteByte('\n')
		}
		cur.WriteString("```\n")
	}
	closeMsg := func() {
		cur.WriteString("```")
		msgs = append(msgs, cur.String())
		started = true
	}
	// room reports whether adding n more characters keeps the current message
	// (plus its closing fence) under limit.
	room := func(n int) bool { return cur.Len()+n+len("```") <= limit }
	bodyEmpty := func() bool { return strings.HasSuffix(cur.String(), "```\n") }

	openMsg()
	for _, blk := range blocks {
		// Keep the block whole when it fits the current or a fresh message.
		if room(len(blk) + 1) {
			cur.WriteString(blk)
			cur.WriteByte('\n')
			continue
		}
		if !bodyEmpty() {
			closeMsg()
			openMsg()
			if room(len(blk) + 1) {
				cur.WriteString(blk)
				cur.WriteByte('\n')
				continue
			}
		}
		// Block larger than a whole message: split on its line boundaries.
		for _, ln := range strings.Split(blk, "\n") {
			cost := len(ln) + 1
			if !room(cost) && !bodyEmpty() {
				closeMsg()
				openMsg()
			}
			cur.WriteString(ln)
			cur.WriteByte('\n')
		}
	}
	closeMsg()
	return msgs
}
