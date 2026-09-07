package bot

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/UberJoe/fpldiscord/internal/bet"
	"github.com/UberJoe/fpldiscord/internal/fpl"
	"github.com/UberJoe/fpldiscord/internal/store"
)

// handleBet is the /bet dispatcher. With no subcommand (or `show`) it renders
// the leaderboard: the current season live-computed, or a past season from the
// archive when a `season` arg is given. `set` and `archive` are admin-only
// writes gated by the ADMIN_IDS allowlist.
func handleBet(in *cmdInput) error {
	switch in.sub {
	case "", "show":
		return betShow(in)
	case "set":
		return betSet(in)
	case "archive":
		return betArchive(in)
	default:
		return in.resp.Respond("Unknown /bet subcommand.")
	}
}

// betShow renders the leaderboard. A `season` arg switches to the static
// archived view for that season; otherwise it is the current season, computed
// live against the snapshot's cumulative goal totals.
func betShow(in *cmdInput) error {
	if season, ok := in.opts.String("season"); ok && strings.TrimSpace(season) != "" {
		return betShowArchived(in, strings.TrimSpace(season))
	}

	if in.betStore == nil {
		return in.resp.Respond("The bet game isn't configured.")
	}
	if in.snap == nil {
		return in.resp.Respond("The bet leaderboard isn't available yet — the bot is still starting up.")
	}

	picks, err := in.betStore.CurrentPicks(in.season)
	if err != nil {
		return fmt.Errorf("read current picks: %w", err)
	}
	if len(picks) == 0 {
		return in.resp.Respond("No bets have been entered yet — an admin can add them with `/bet set`.")
	}

	board := bet.Leaderboard(picks, bet.SnapshotGoals(in.snap))
	header := fmt.Sprintf("**Bet leaderboard — %s**", in.season)
	for _, msg := range packCodeBlockMessages(header, betBoardBlocks(in, board, bet.SeasonComplete(in.snap)), maxDiscordMessage) {
		if err := in.resp.Respond(msg); err != nil {
			return err
		}
	}
	return nil
}

// betShowArchived renders a frozen past-season record. Totals are the stored
// end-of-season goal counts; the same closest-to-21 ordering is applied so it
// reads like the live board.
func betShowArchived(in *cmdInput, season string) error {
	if in.betStore == nil {
		return in.resp.Respond("The bet game isn't configured.")
	}

	recs, err := in.betStore.Archive(season)
	if err != nil {
		return fmt.Errorf("read archive %q: %w", season, err)
	}
	if len(recs) == 0 {
		seasons, err := in.betStore.ArchivedSeasons()
		if err != nil {
			return fmt.Errorf("list archived seasons: %w", err)
		}
		if len(seasons) == 0 {
			return in.resp.Respond("No seasons have been archived yet.")
		}
		return in.resp.Respond(fmt.Sprintf(
			"No archived record for %q. Archived seasons: %s", season, strings.Join(seasons, ", ")))
	}

	header := fmt.Sprintf("**Bet — %s (archived)**", season)
	for _, msg := range packCodeBlockMessages(header, archivedBlocks(recs), maxDiscordMessage) {
		if err := in.resp.Respond(msg); err != nil {
			return err
		}
	}
	return nil
}

// betSet replaces a bettor's four current-season picks. Admin-only; player args
// are resolved against the snapshot by (accent-stripped) name.
func betSet(in *cmdInput) error {
	if !in.callerIsAdmin() {
		return in.resp.Respond("Only a league admin can use `/bet set`.")
	}
	if in.betStore == nil {
		return in.resp.Respond("The bet game isn't configured.")
	}
	if in.snap == nil {
		return in.resp.Respond("Can't set picks yet — the bot is still starting up.")
	}

	bettor, _ := in.opts.String("bettor")
	if strings.TrimSpace(bettor) == "" {
		return in.resp.Respond("Name the bettor whose picks these are.")
	}

	var ids [4]int
	labels := make([]string, 0, 4)
	var problems []string
	for i, key := range [4]string{"p1", "p2", "p3", "p4"} {
		raw, _ := in.opts.String(key)
		el, ok := betResolvePlayer(in.snap, raw)
		if !ok {
			problems = append(problems, fmt.Sprintf("%q", strings.TrimSpace(raw)))
			continue
		}
		ids[i] = int(el.ID)
		labels = append(labels, el.WebName)
	}
	if len(problems) > 0 {
		return in.resp.Respond("Couldn't match these players: " + strings.Join(problems, ", "))
	}
	if dup := firstDuplicate(ids); dup != 0 {
		return in.resp.Respond("A bettor's four picks must be different players.")
	}

	if err := in.betStore.SetPicks(in.season, bettor, ids); err != nil {
		return fmt.Errorf("set picks for %s: %w", bettor, err)
	}
	return in.resp.Respond(fmt.Sprintf("Set <@%s>'s picks: %s.", bettor, strings.Join(labels, ", ")))
}

// betArchive records a completed past season. Admin-only. The bettor name is
// free text (they may have left the server); entries is four comma-separated
// `Name:goals` pairs.
func betArchive(in *cmdInput) error {
	if !in.callerIsAdmin() {
		return in.resp.Respond("Only a league admin can use `/bet archive`.")
	}
	if in.betStore == nil {
		return in.resp.Respond("The bet game isn't configured.")
	}

	season, _ := in.opts.String("season")
	name, _ := in.opts.String("bettor_name")
	entries, _ := in.opts.String("entries")
	season, name = strings.TrimSpace(season), strings.TrimSpace(name)
	if season == "" || name == "" {
		return in.resp.Respond("Give both a season and a bettor name.")
	}

	picks, err := parseArchiveEntries(entries)
	if err != nil {
		return in.resp.Respond(err.Error())
	}

	if err := in.betStore.AddArchive(season, name, picks); err != nil {
		return fmt.Errorf("archive %s / %s: %w", season, name, err)
	}

	total := 0
	for _, p := range picks {
		total += p.FinalGoals
	}
	return in.resp.Respond(fmt.Sprintf("Archived %s for %s — total %d.", name, season, total))
}

// callerIsAdmin reports whether the invoking user is on the admin allowlist. A
// nil gate (only in a misconfigured test) denies.
func (in *cmdInput) callerIsAdmin() bool {
	return in.isAdmin != nil && in.isAdmin(in.caller)
}

// betResolvePlayer matches a raw player arg to an element via the snapshot's
// accent-stripped autocomplete slice: an exact (folded) name wins; otherwise a
// single substring match is accepted; zero or an ambiguous match fails.
func betResolvePlayer(snap *fpl.Snapshot, raw string) (fpl.PlayerName, bool) {
	q := fold(raw)
	if q == "" {
		return fpl.PlayerName{}, false
	}
	var partial []fpl.PlayerName
	for _, pn := range snap.PlayerNames {
		s := strings.ToLower(pn.Stripped)
		if s == q {
			return pn, true
		}
		if strings.Contains(s, q) {
			partial = append(partial, pn)
		}
	}
	if len(partial) == 1 {
		return partial[0], true
	}
	return fpl.PlayerName{}, false
}

// firstDuplicate returns a repeated element id in ids, or 0 if all four are
// distinct (a real element id is never 0).
func firstDuplicate(ids [4]int) int {
	seen := map[int]bool{}
	for _, id := range ids {
		if seen[id] {
			return id
		}
		seen[id] = true
	}
	return 0
}

// parseArchiveEntries parses `Name:goals, Name:goals, Name:goals, Name:goals`
// into four archive picks in the order given. Exactly four entries are
// required; goals must be a non-negative integer.
func parseArchiveEntries(raw string) ([4]store.ArchivePick, error) {
	var out [4]store.ArchivePick
	parts := strings.Split(raw, ",")
	trimmed := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			trimmed = append(trimmed, p)
		}
	}
	if len(trimmed) != 4 {
		return out, fmt.Errorf("give exactly four Name:goals entries (got %d).", len(trimmed))
	}
	for i, entry := range trimmed {
		name, goalsStr, ok := strings.Cut(entry, ":")
		name = strings.TrimSpace(name)
		if !ok || name == "" {
			return out, fmt.Errorf("entry %q isn't in Name:goals form.", entry)
		}
		goals, err := strconv.Atoi(strings.TrimSpace(goalsStr))
		if err != nil || goals < 0 {
			return out, fmt.Errorf("entry %q has a bad goal count.", entry)
		}
		out[i] = store.ArchivePick{PlayerName: name, FinalGoals: goals}
	}
	return out, nil
}

// betStatusGlyph maps a computed status to the marker the leaderboard shows.
func betStatusGlyph(s bet.Status) string {
	switch s {
	case bet.StatusProvisionallyOut:
		return "🕓"
	case bet.StatusBust:
		return "💥"
	default:
		return "in"
	}
}

// betBoardBlocks renders one display block per bettor for the live leaderboard:
// a header line (name, total, status marker, leader marker) and a second line
// with the four players and each one's season goals. packCodeBlockMessages
// wraps the blocks in monospace fences and splits across messages if the round
// is long, so /bet can never produce a message Discord rejects for length. The
// 🏆 is stamped on the leader only once the season is complete.
func betBoardBlocks(in *cmdInput, board []bet.Bettor, complete bool) []string {
	blocks := make([]string, 0, len(board))
	for _, row := range board {
		leader := ""
		if row.Leader {
			if complete {
				leader = "  🏆"
			} else {
				leader = "  (leading)"
			}
		}
		head := fmt.Sprintf("%-20s  %3d  %s%s", betDisplayName(in, row.DiscordUserID), row.Total, betStatusGlyph(row.Status), leader)
		picks := "   " + strings.Join(pickCells(row.Picks[:]), "  ")
		blocks = append(blocks, head+"\n"+picks)
	}
	return blocks
}

// archivedBlocks renders a frozen past season in the same block shape as the
// live board, ordered closest-to-21 with over-21 totals last.
func archivedBlocks(recs []store.ArchivedBettor) []string {
	type line struct {
		name  string
		total int
		picks [4]store.ArchivePick
	}
	lines := make([]line, 0, len(recs))
	for _, r := range recs {
		total := 0
		for _, p := range r.Picks {
			total += p.FinalGoals
		}
		lines = append(lines, line{name: r.Name, total: total, picks: r.Picks})
	}
	sort.SliceStable(lines, func(i, j int) bool {
		bi, bj := lines[i].total > bet.Target, lines[j].total > bet.Target
		if bi != bj {
			return !bi
		}
		if lines[i].total != lines[j].total {
			return lines[i].total > lines[j].total
		}
		return lines[i].name < lines[j].name
	})

	blocks := make([]string, 0, len(lines))
	for _, l := range lines {
		bustMark := ""
		if l.total > bet.Target {
			bustMark = "  💥"
		}
		cells := make([]string, 0, 4)
		for _, p := range l.picks {
			cells = append(cells, fmt.Sprintf("%s (%d)", p.PlayerName, p.FinalGoals))
		}
		head := fmt.Sprintf("%-20s  %3d%s", l.name, l.total, bustMark)
		blocks = append(blocks, head+"\n   "+strings.Join(cells, "  "))
	}
	return blocks
}

// pickCells renders each pick as "WebName (goals)", falling back to the raw
// element id when the snapshot doesn't know the player.
func pickCells(picks []bet.Pick) []string {
	out := make([]string, 0, len(picks))
	for _, p := range picks {
		name := p.WebName
		if name == "" {
			name = fmt.Sprintf("#%d", int(p.ElementID))
		}
		out = append(out, fmt.Sprintf("%s (%d)", name, p.Goals))
	}
	return out
}

// betDisplayName resolves a bettor's Discord id to a name via the namer, with
// the raw id as the fallback (matching /api/bet).
func betDisplayName(in *cmdInput, discordUserID string) string {
	if in.namer != nil {
		if name, ok := in.namer.MemberName(discordUserID); ok && name != "" {
			return name
		}
	}
	return discordUserID
}
