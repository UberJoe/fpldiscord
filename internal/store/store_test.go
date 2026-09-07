package store

import (
	"database/sql"
	"path/filepath"
	"reflect"
	"testing"
	"testing/fstest"
)

// openTemp opens a Store against a throwaway DB file in a per-test temp dir.
func openTemp(t *testing.T) *Store {
	t.Helper()
	st, err := Open(filepath.Join(t.TempDir(), "bet.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

func TestSetPicksCurrentPicksRoundTrip(t *testing.T) {
	st := openTemp(t)

	if err := st.SetPicks("2026/27", "user-a", [4]int{10, 20, 30, 40}); err != nil {
		t.Fatalf("SetPicks: %v", err)
	}
	if err := st.SetPicks("2026/27", "user-b", [4]int{1, 2, 3, 4}); err != nil {
		t.Fatalf("SetPicks: %v", err)
	}

	got, err := st.CurrentPicks("2026/27")
	if err != nil {
		t.Fatalf("CurrentPicks: %v", err)
	}
	want := []BettorPicks{
		{DiscordUserID: "user-a", Elements: [4]int{10, 20, 30, 40}},
		{DiscordUserID: "user-b", Elements: [4]int{1, 2, 3, 4}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("CurrentPicks = %+v, want %+v", got, want)
	}
}

func TestSetPicksFullyReplacesPreviousRows(t *testing.T) {
	st := openTemp(t)

	if err := st.SetPicks("2026/27", "user-a", [4]int{10, 20, 30, 40}); err != nil {
		t.Fatalf("first SetPicks: %v", err)
	}
	if err := st.SetPicks("2026/27", "user-a", [4]int{99, 98, 97, 96}); err != nil {
		t.Fatalf("second SetPicks: %v", err)
	}

	got, err := st.CurrentPicks("2026/27")
	if err != nil {
		t.Fatalf("CurrentPicks: %v", err)
	}
	want := []BettorPicks{{DiscordUserID: "user-a", Elements: [4]int{99, 98, 97, 96}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("CurrentPicks = %+v, want %+v (delete-then-insert must leave no orphan slot)", got, want)
	}
}

func TestCurrentPicksIsSeasonIsolated(t *testing.T) {
	st := openTemp(t)

	if err := st.SetPicks("2025/26", "user-a", [4]int{5, 6, 7, 8}); err != nil {
		t.Fatalf("SetPicks 2025/26: %v", err)
	}
	if err := st.SetPicks("2026/27", "user-a", [4]int{10, 20, 30, 40}); err != nil {
		t.Fatalf("SetPicks 2026/27: %v", err)
	}

	got, err := st.CurrentPicks("2026/27")
	if err != nil {
		t.Fatalf("CurrentPicks: %v", err)
	}
	want := []BettorPicks{{DiscordUserID: "user-a", Elements: [4]int{10, 20, 30, 40}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("CurrentPicks(2026/27) = %+v, want %+v", got, want)
	}
}

func TestArchiveRoundTrip(t *testing.T) {
	st := openTemp(t)

	picks := [4]ArchivePick{
		{PlayerName: "Haaland", FinalGoals: 9},
		{PlayerName: "Salah", FinalGoals: 5},
		{PlayerName: "Watkins", FinalGoals: 4},
		{PlayerName: "Isak", FinalGoals: 3},
	}
	if err := st.AddArchive("2024/25", "Joe", picks); err != nil {
		t.Fatalf("AddArchive: %v", err)
	}
	if err := st.AddArchive("2024/25", "Steve", [4]ArchivePick{
		{PlayerName: "Palmer", FinalGoals: 7},
		{PlayerName: "Saka", FinalGoals: 6},
		{PlayerName: "Foden", FinalGoals: 5},
		{PlayerName: "Gordon", FinalGoals: 4},
	}); err != nil {
		t.Fatalf("AddArchive: %v", err)
	}

	seasons, err := st.ArchivedSeasons()
	if err != nil {
		t.Fatalf("ArchivedSeasons: %v", err)
	}
	if !reflect.DeepEqual(seasons, []string{"2024/25"}) {
		t.Fatalf("ArchivedSeasons = %v, want [2024/25]", seasons)
	}

	got, err := st.Archive("2024/25")
	if err != nil {
		t.Fatalf("Archive: %v", err)
	}
	want := []ArchivedBettor{
		{Name: "Joe", Picks: picks},
		{Name: "Steve", Picks: [4]ArchivePick{
			{PlayerName: "Palmer", FinalGoals: 7},
			{PlayerName: "Saka", FinalGoals: 6},
			{PlayerName: "Foden", FinalGoals: 5},
			{PlayerName: "Gordon", FinalGoals: 4},
		}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Archive(2024/25) = %+v, want %+v", got, want)
	}
}

func TestAddArchiveFullyReplacesPreviousRows(t *testing.T) {
	st := openTemp(t)

	if err := st.AddArchive("2024/25", "Joe", [4]ArchivePick{
		{PlayerName: "A", FinalGoals: 1}, {PlayerName: "B", FinalGoals: 2},
		{PlayerName: "C", FinalGoals: 3}, {PlayerName: "D", FinalGoals: 4},
	}); err != nil {
		t.Fatalf("first AddArchive: %v", err)
	}
	replacement := [4]ArchivePick{
		{PlayerName: "W", FinalGoals: 9}, {PlayerName: "X", FinalGoals: 8},
		{PlayerName: "Y", FinalGoals: 7}, {PlayerName: "Z", FinalGoals: 6},
	}
	if err := st.AddArchive("2024/25", "Joe", replacement); err != nil {
		t.Fatalf("second AddArchive: %v", err)
	}

	got, err := st.Archive("2024/25")
	if err != nil {
		t.Fatalf("Archive: %v", err)
	}
	want := []ArchivedBettor{{Name: "Joe", Picks: replacement}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Archive = %+v, want %+v", got, want)
	}
}

func TestGenIncrementsOnEveryMutatingCall(t *testing.T) {
	st := openTemp(t)

	start := st.Gen()
	if err := st.SetPicks("2026/27", "user-a", [4]int{1, 2, 3, 4}); err != nil {
		t.Fatalf("SetPicks: %v", err)
	}
	if got := st.Gen(); got != start+1 {
		t.Fatalf("Gen after SetPicks = %d, want %d", got, start+1)
	}
	if err := st.AddArchive("2024/25", "Joe", [4]ArchivePick{
		{PlayerName: "A", FinalGoals: 1}, {PlayerName: "B", FinalGoals: 2},
		{PlayerName: "C", FinalGoals: 3}, {PlayerName: "D", FinalGoals: 4},
	}); err != nil {
		t.Fatalf("AddArchive: %v", err)
	}
	if got := st.Gen(); got != start+2 {
		t.Fatalf("Gen after AddArchive = %d, want %d", got, start+2)
	}
}

func TestCurrentPicksEmptySeason(t *testing.T) {
	st := openTemp(t)
	got, err := st.CurrentPicks("2026/27")
	if err != nil {
		t.Fatalf("CurrentPicks: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("CurrentPicks on empty season = %+v, want none", got)
	}
}

// --- migration runner --------------------------------------------------------

// twoMigrations is a synthetic migration set: 0002 depends on 0001 having run,
// so it only applies cleanly if the runner applies files in version order.
var twoMigrations = fstest.MapFS{
	"0001_a.sql": {Data: []byte(`CREATE TABLE widget (id INTEGER PRIMARY KEY);`)},
	"0002_b.sql": {Data: []byte(`ALTER TABLE widget ADD COLUMN label TEXT;`)},
}

func appliedVersions(t *testing.T, db *sql.DB) []int {
	t.Helper()
	rows, err := db.Query(`SELECT version FROM schema_migrations ORDER BY version`)
	if err != nil {
		t.Fatalf("query schema_migrations: %v", err)
	}
	defer rows.Close()
	var out []int
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			t.Fatalf("scan: %v", err)
		}
		out = append(out, v)
	}
	return out
}

func TestRunMigrationsAppliesFilesInOrder(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "m.db"))
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	defer db.Close()

	if err := runMigrations(db, twoMigrations); err != nil {
		t.Fatalf("runMigrations: %v", err)
	}
	if got := appliedVersions(t, db); !reflect.DeepEqual(got, []int{1, 2}) {
		t.Fatalf("applied versions = %v, want [1 2]", got)
	}
	// 0002's ALTER only succeeds after 0001's CREATE — this insert proves the
	// post-migration schema has both the table and the added column.
	if _, err := db.Exec(`INSERT INTO widget (id, label) VALUES (1, 'ok')`); err != nil {
		t.Fatalf("post-migration schema wrong: %v", err)
	}
}

func TestRunMigrationsIsIdempotent(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "m.db"))
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	defer db.Close()

	if err := runMigrations(db, twoMigrations); err != nil {
		t.Fatalf("first runMigrations: %v", err)
	}
	if err := runMigrations(db, twoMigrations); err != nil {
		t.Fatalf("second runMigrations (should be a no-op): %v", err)
	}
	if got := appliedVersions(t, db); !reflect.DeepEqual(got, []int{1, 2}) {
		t.Fatalf("applied versions after second run = %v, want [1 2]", got)
	}
}

func TestOpenReopenKeepsData(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bet.db")

	st1, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := st1.SetPicks("2026/27", "user-a", [4]int{11, 22, 33, 44}); err != nil {
		t.Fatalf("SetPicks: %v", err)
	}
	st1.Close()

	st2, err := Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer st2.Close()

	got, err := st2.CurrentPicks("2026/27")
	if err != nil {
		t.Fatalf("CurrentPicks: %v", err)
	}
	want := []BettorPicks{{DiscordUserID: "user-a", Elements: [4]int{11, 22, 33, 44}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("CurrentPicks after reopen = %+v, want %+v", got, want)
	}
}
