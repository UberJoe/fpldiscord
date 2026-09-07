package fpl

import "sort"

// WaiverType is a resolved transaction kind. The Draft API sends single-letter
// codes ("w", "f", plus trade codes); LeagueTransactions resolves the two the
// MVP renders and drops the rest.
type WaiverType string

const (
	// WaiverTypeWaiver is a scheduled waiver claim (raw kind "w").
	WaiverTypeWaiver WaiverType = "waiver"
	// WaiverTypeFreeAgent is an instant free-agent pickup (raw kind "f").
	WaiverTypeFreeAgent WaiverType = "freeAgent"
)

// WaiverStatus is a resolved transaction outcome. The Draft API sends "a" for
// accepted and a family of denied/pending codes ("do" = denied, a
// higher-priority claim won; others for denied/pending) that all collapse to
// failed here.
type WaiverStatus string

const (
	// WaiverStatusAccepted is a claim that went through (raw result "a").
	WaiverStatusAccepted WaiverStatus = "accepted"
	// WaiverStatusFailed is any non-accepted claim.
	WaiverStatusFailed WaiverStatus = "failed"
)

// WaiverRow is one processed waiver or free-agent claim, with the raw kind and
// result codes resolved and the element / owner ids joined to names. It backs
// the /waivers command and the web Waiver-History view. Priority is the
// manager's waiver order for the claim (lower = better) and Index is the
// ordering within the processed batch — together they place who out-bid whom.
type WaiverRow struct {
	GW        int
	EntryID   EntryID
	OwnerName string    // player_first_name; "" if the entry is not in this league
	ElementIn ElementID // element_in id — the stable key for grouping a contested claim
	In        string    // element_in web_name, accents intact; "" if unknown
	Out       string    // element_out web_name, accents intact; "" when nothing was dropped
	Type      WaiverType
	Status    WaiverStatus
	Priority  int
	Index     int
}

// LeagueTransactions returns the waiver and free-agent claims for gw, sorted by
// Index (then GW, when gw is 0 and rows span gameweeks). The raw kind/result
// codes are resolved to WaiverType / WaiverStatus. Trades — every kind other
// than "w" and "f" — are excluded: the MVP has no Trades view, and a trade has
// two sides that do not fit the one-row-per-claim shape.
//
// A gw of 0 returns every gameweek's rows. Names are joined from the exported
// Snapshot fields (bootstrap-static, league_entries) rather than the unexported
// indexes, so the view is a pure function of an exported Snapshot and the
// sibling-package seam tests can build one by hand.
func (s *Snapshot) LeagueTransactions(gw int) []WaiverRow {
	nameByElement := make(map[ElementID]string, len(s.Bootstrap.Elements))
	for i := range s.Bootstrap.Elements {
		e := &s.Bootstrap.Elements[i]
		nameByElement[e.ID] = e.WebName
	}
	firstByEntry := make(map[EntryID]string, len(s.LeagueDetails.LeagueEntries))
	for i := range s.LeagueDetails.LeagueEntries {
		le := &s.LeagueDetails.LeagueEntries[i]
		firstByEntry[le.EntryID] = le.PlayerFirstName
	}

	rows := make([]WaiverRow, 0, len(s.Transactions))
	for i := range s.Transactions {
		t := &s.Transactions[i]
		typ, ok := resolveWaiverType(t.Kind)
		if !ok {
			continue // a trade — excluded
		}
		if gw != 0 && t.Event != gw {
			continue
		}
		row := WaiverRow{
			GW:        t.Event,
			EntryID:   t.Entry,
			OwnerName: firstByEntry[t.Entry],
			ElementIn: t.ElementIn,
			In:        nameByElement[t.ElementIn],
			Type:      typ,
			Status:    resolveWaiverStatus(t.Result),
			Priority:  t.Priority,
			Index:     t.Index,
		}
		if t.ElementOut != 0 {
			row.Out = nameByElement[t.ElementOut]
		}
		rows = append(rows, row)
	}

	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].GW != rows[j].GW {
			return rows[i].GW < rows[j].GW
		}
		return rows[i].Index < rows[j].Index
	})
	return rows
}

// resolveWaiverType maps a raw transactions[].kind code onto a WaiverType. ok is
// false for a trade kind, which the caller drops.
func resolveWaiverType(kind string) (WaiverType, bool) {
	switch kind {
	case "w":
		return WaiverTypeWaiver, true
	case "f":
		return WaiverTypeFreeAgent, true
	default:
		return "", false
	}
}

// resolveWaiverStatus maps a raw transactions[].result code onto a
// WaiverStatus: "a" is accepted, everything else (denied / denied-other /
// pending) is failed.
func resolveWaiverStatus(result string) WaiverStatus {
	if result == "a" {
		return WaiverStatusAccepted
	}
	return WaiverStatusFailed
}
