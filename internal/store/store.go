// Package store is the pure-Go SQLite persistence for the bet game: current
// picks and archived past seasons. It is a leaf of the import graph — it never
// imports fpl or any other internal package, and deals only in strings and
// ints. Schema changes are forward-only additive `.sql` files in migrations/,
// applied at Open by the runner in migrate.go.
package store

import (
	"database/sql"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync/atomic"

	_ "modernc.org/sqlite" // registers the "sqlite" database/sql driver (no CGO)
)

// BettorPicks is one bettor's four current-season picks, in slot order.
type BettorPicks struct {
	DiscordUserID string
	Elements      [4]int
}

// ArchivePick is one slot of a frozen past-season record.
type ArchivePick struct {
	PlayerName string
	FinalGoals int
}

// ArchivedBettor is one bettor's frozen record for an archived season.
type ArchivedBettor struct {
	Name  string
	Picks [4]ArchivePick
}

// Store owns the database handle and an in-memory generation counter that ticks
// on every mutating call, so the /api/bet ETag can fold it without reading the
// DB.
type Store struct {
	db  *sql.DB
	gen atomic.Uint64
}

// Open opens (creating if absent) the SQLite database at dbPath with WAL,
// foreign keys and a 5s busy timeout, then runs pending migrations. A failed
// migration returns an error with nothing past the failure applied — the caller
// (app boot) maps that to a non-zero exit.
func Open(dbPath string) (*Store, error) {
	if dir := filepath.Dir(dbPath); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create db directory %q: %w", dir, err)
		}
	}

	// Pragmas go in the DSN so every pooled connection gets them; foreign_keys
	// and busy_timeout are per-connection settings.
	dsn := filepath.ToSlash(dbPath) +
		"?_pragma=journal_mode(WAL)" +
		"&_pragma=foreign_keys(ON)" +
		"&_pragma=busy_timeout(5000)"

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open %q: %w", dbPath, err)
	}
	// One writer, serialised: the write volume here is a handful of admin
	// commands, and a single connection keeps SQLite's locking behaviour
	// predictable on the fly volume.
	db.SetMaxOpenConns(1)

	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping %q: %w", dbPath, err)
	}

	sub, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := runMigrations(db, sub); err != nil {
		_ = db.Close()
		return nil, err
	}

	return &Store{db: db}, nil
}

// Close closes the underlying database handle.
func (s *Store) Close() error {
	return s.db.Close()
}

// Gen returns the current generation counter. It increments on every mutating
// call (SetPicks, AddArchive), so a consumer can tell picks changed without
// re-reading the DB — the /api/bet ETag folds it.
func (s *Store) Gen() uint64 { return s.gen.Load() }

// CurrentPicks returns every bettor's current picks for season, ordered by
// Discord user id. Bettors without exactly four rows still appear; missing
// slots read back as element id 0.
func (s *Store) CurrentPicks(season string) ([]BettorPicks, error) {
	rows, err := s.db.Query(`
		SELECT discord_user_id, slot, element_id
		FROM bet_pick_current
		WHERE season = ?
		ORDER BY discord_user_id, slot`, season)
	if err != nil {
		return nil, fmt.Errorf("query current picks: %w", err)
	}
	defer rows.Close()

	byUser := make(map[string]*BettorPicks)
	var order []string
	for rows.Next() {
		var uid string
		var slot, elementID int
		if err := rows.Scan(&uid, &slot, &elementID); err != nil {
			return nil, fmt.Errorf("scan current pick: %w", err)
		}
		bp, ok := byUser[uid]
		if !ok {
			bp = &BettorPicks{DiscordUserID: uid}
			byUser[uid] = bp
			order = append(order, uid)
		}
		if slot >= 1 && slot <= 4 {
			bp.Elements[slot-1] = elementID
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	out := make([]BettorPicks, 0, len(order))
	for _, uid := range order {
		out = append(out, *byUser[uid])
	}
	return out, nil
}

// SetPicks replaces a bettor's four current-season picks in one transaction
// (delete then insert exactly four rows), so a previous pick set of the same
// size never leaves an orphan slot. It bumps the generation counter.
func (s *Store) SetPicks(season, discordUserID string, elements [4]int) error {
	err := s.replaceSlots(
		`DELETE FROM bet_pick_current WHERE season = ? AND discord_user_id = ?`,
		[]any{season, discordUserID},
		`INSERT INTO bet_pick_current (season, discord_user_id, slot, element_id) VALUES (?, ?, ?, ?)`,
		func(slot int) []any { return []any{season, discordUserID, slot, elements[slot-1]} },
	)
	if err != nil {
		return fmt.Errorf("set picks for %s: %w", discordUserID, err)
	}
	return nil
}

// ArchivedSeasons returns the distinct seasons present in the archive, sorted
// ascending. It is the one read that is deliberately not season-filtered — its
// job is to enumerate which seasons an archived record exists for.
func (s *Store) ArchivedSeasons() ([]string, error) {
	rows, err := s.db.Query(`SELECT DISTINCT season FROM bet_archive ORDER BY season`)
	if err != nil {
		return nil, fmt.Errorf("query archived seasons: %w", err)
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var season string
		if err := rows.Scan(&season); err != nil {
			return nil, fmt.Errorf("scan archived season: %w", err)
		}
		out = append(out, season)
	}
	return out, rows.Err()
}

// Archive returns the frozen records for season, ordered by bettor name.
// Bettors without exactly four rows still appear; missing slots read back
// zero-valued.
func (s *Store) Archive(season string) ([]ArchivedBettor, error) {
	rows, err := s.db.Query(`
		SELECT bettor_name, slot, player_name, final_goals
		FROM bet_archive
		WHERE season = ?
		ORDER BY bettor_name, slot`, season)
	if err != nil {
		return nil, fmt.Errorf("query archive: %w", err)
	}
	defer rows.Close()

	byName := make(map[string]*ArchivedBettor)
	var order []string
	for rows.Next() {
		var name, playerName string
		var slot, finalGoals int
		if err := rows.Scan(&name, &slot, &playerName, &finalGoals); err != nil {
			return nil, fmt.Errorf("scan archive row: %w", err)
		}
		ab, ok := byName[name]
		if !ok {
			ab = &ArchivedBettor{Name: name}
			byName[name] = ab
			order = append(order, name)
		}
		if slot >= 1 && slot <= 4 {
			ab.Picks[slot-1] = ArchivePick{PlayerName: playerName, FinalGoals: finalGoals}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	out := make([]ArchivedBettor, 0, len(order))
	for _, name := range order {
		out = append(out, *byName[name])
	}
	return out, nil
}

// AddArchive records (or replaces) one bettor's frozen four-pick record for a
// past season in one transaction. It bumps the generation counter.
func (s *Store) AddArchive(season, bettorName string, picks [4]ArchivePick) error {
	err := s.replaceSlots(
		`DELETE FROM bet_archive WHERE season = ? AND bettor_name = ?`,
		[]any{season, bettorName},
		`INSERT INTO bet_archive (season, bettor_name, slot, player_name, final_goals) VALUES (?, ?, ?, ?, ?)`,
		func(slot int) []any {
			p := picks[slot-1]
			return []any{season, bettorName, slot, p.PlayerName, p.FinalGoals}
		},
	)
	if err != nil {
		return fmt.Errorf("add archive for %s / %s: %w", season, bettorName, err)
	}
	return nil
}

// replaceSlots is the shared write path for both pick tables: in one
// transaction, delete the caller's existing rows for a key (deleteSQL +
// keyArgs) then insert exactly four fresh slot rows (insertSQL + slotArgs(slot)
// for slot 1..4). On success it commits and bumps the generation counter; any
// error rolls the whole thing back and leaves the counter untouched.
func (s *Store) replaceSlots(deleteSQL string, keyArgs []any, insertSQL string, slotArgs func(slot int) []any) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	if err := func() error {
		if _, err := tx.Exec(deleteSQL, keyArgs...); err != nil {
			return err
		}
		for slot := 1; slot <= 4; slot++ {
			if _, err := tx.Exec(insertSQL, slotArgs(slot)...); err != nil {
				return err
			}
		}
		return nil
	}(); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	s.gen.Add(1)
	return nil
}
