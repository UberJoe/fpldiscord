package store

import (
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strconv"
	"strings"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// migration is one parsed `.sql` file: its integer version (the NNNN filename
// prefix) and its body.
type migration struct {
	version int
	name    string
	body    string
}

// loadMigrations reads every `*.sql` file at the root of fsys, parses the
// leading NNNN version off each filename, and returns them sorted ascending. It
// errors on a non-numeric prefix or a duplicated version.
func loadMigrations(fsys fs.FS) ([]migration, error) {
	names, err := fs.Glob(fsys, "*.sql")
	if err != nil {
		return nil, err
	}
	out := make([]migration, 0, len(names))
	seen := make(map[int]string, len(names))
	for _, name := range names {
		base := path.Base(name)
		prefix, _, ok := strings.Cut(base, "_")
		if !ok {
			return nil, fmt.Errorf("migration %q is not NNNN_name.sql", base)
		}
		version, err := strconv.Atoi(prefix)
		if err != nil {
			return nil, fmt.Errorf("migration %q has a non-numeric version prefix: %w", base, err)
		}
		if prev, dup := seen[version]; dup {
			return nil, fmt.Errorf("migration version %d is used by both %s and %s", version, prev, base)
		}
		seen[version] = base
		body, err := fs.ReadFile(fsys, name)
		if err != nil {
			return nil, err
		}
		out = append(out, migration{version: version, name: base, body: string(body)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].version < out[j].version })
	return out, nil
}

// runMigrations applies every migration in fsys whose version is above the
// highest already recorded in schema_migrations, in version order, each in its
// own transaction. Running it again with no new files is a no-op. A failing
// migration rolls back its own transaction and returns the error with nothing
// past that point applied.
func runMigrations(db *sql.DB, fsys fs.FS) error {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version    INTEGER PRIMARY KEY,
		applied_at TEXT NOT NULL DEFAULT (datetime('now'))
	)`); err != nil {
		return fmt.Errorf("ensure schema_migrations: %w", err)
	}

	var current int
	if err := db.QueryRow(`SELECT COALESCE(MAX(version), 0) FROM schema_migrations`).Scan(&current); err != nil {
		return fmt.Errorf("read current schema version: %w", err)
	}

	migs, err := loadMigrations(fsys)
	if err != nil {
		return err
	}

	for _, m := range migs {
		if m.version <= current {
			continue
		}
		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("begin migration %s: %w", m.name, err)
		}
		if _, err := tx.Exec(m.body); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("apply migration %s: %w", m.name, err)
		}
		if _, err := tx.Exec(`INSERT INTO schema_migrations (version) VALUES (?)`, m.version); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("record migration %s: %w", m.name, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %s: %w", m.name, err)
		}
	}
	return nil
}
