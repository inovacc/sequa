package migrate

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	dbpkg "github.com/inovacc/sequa/internal/db"
)

// writeMigration creates an up/down pair in dir.
func writeMigration(t *testing.T, dir, stem, up, down string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, stem+".up.sql"), []byte(up), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, stem+".down.sql"), []byte(down), 0o644); err != nil {
		t.Fatal(err)
	}
}

func testDSN(t *testing.T) string {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping migrate integration test in -short mode")
	}
	dsn := os.Getenv("SEQUA_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set SEQUA_TEST_DATABASE_URL to run migrate integration tests")
	}
	return dsn
}

// resetSchema drops the migration-tracking tables and this test's tables so the
// test starts from a clean database. Integration tests share one Postgres, so
// each must isolate itself rather than rely on leftover state.
func resetSchema(t *testing.T, dsn string) {
	t.Helper()
	db, err := dbpkg.Open(context.Background(), dsn)
	if err != nil {
		t.Fatalf("reset open: %v", err)
	}
	defer func() { _ = db.Close() }()
	if _, err := db.ExecContext(context.Background(),
		"DROP TABLE IF EXISTS schema_migrations, sequa_schema_history, widgets CASCADE"); err != nil {
		t.Fatalf("reset drop: %v", err)
	}
}

func TestMissingVersions(t *testing.T) {
	known := []uint{1, 2, 3, 5}
	tests := []struct {
		name     string
		recorded map[uint]bool
		current  uint
		want     []uint
	}{
		{name: "all recorded", recorded: map[uint]bool{1: true, 2: true, 3: true}, current: 3, want: nil},
		{name: "gap in middle", recorded: map[uint]bool{1: true, 3: true}, current: 3, want: []uint{2}},
		{name: "none recorded", recorded: map[uint]bool{}, current: 3, want: []uint{1, 2, 3}},
		{name: "current caps the set", recorded: map[uint]bool{}, current: 2, want: []uint{1, 2}},
		{name: "current zero backfills nothing", recorded: map[uint]bool{}, current: 0, want: nil},
		{name: "highest applied but unrecorded", recorded: map[uint]bool{1: true, 2: true, 3: true}, current: 5, want: []uint{5}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := missingVersions(known, tt.recorded, tt.current)
			if len(got) != len(tt.want) {
				t.Fatalf("missingVersions = %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("missingVersions = %v, want %v", got, tt.want)
				}
			}
		})
	}
}

// TestRunnerReconcilesLostHistory proves the self-heal for ISS-1: a history row
// lost to a crash (migrate step committed, follow-up INSERT never landed) is
// backfilled by the next Up.
func TestRunnerReconcilesLostHistory(t *testing.T) {
	dsn := testDSN(t)
	resetSchema(t, dsn)

	dir := t.TempDir()
	writeMigration(t, dir, "00001_create_widgets",
		"CREATE TABLE widgets (id INT PRIMARY KEY);", "DROP TABLE widgets;")
	writeMigration(t, dir, "00002_add_name",
		"ALTER TABLE widgets ADD COLUMN name TEXT;", "ALTER TABLE widgets DROP COLUMN name;")

	ctx := context.Background()
	r, err := NewRunner(dsn, os.DirFS(dir), ".")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.Up(ctx); err != nil {
		t.Fatalf("up: %v", err)
	}

	// Simulate the crash: drop version 2's history row directly.
	db, err := dbpkg.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	if _, err := db.ExecContext(ctx, `DELETE FROM sequa_schema_history WHERE version = 2`); err != nil {
		t.Fatalf("delete history: %v", err)
	}

	st, err := r.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(st) != 2 || st[1].Version != 2 || !st[1].Applied || st[1].AppliedAt != nil {
		t.Fatalf("pre-reconcile status[1]=%+v, want v2 applied with nil AppliedAt", st[1])
	}

	// A second Up has no pending migrations but must backfill the lost row.
	if _, err := r.Up(ctx); err != nil {
		t.Fatalf("second up: %v", err)
	}
	st, err = r.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if st[1].AppliedAt == nil {
		t.Fatalf("post-reconcile status[1]=%+v, want backfilled AppliedAt", st[1])
	}
}

func TestRunnerUpStatusDownVersion(t *testing.T) {
	dsn := testDSN(t)
	resetSchema(t, dsn)

	dir := t.TempDir()
	writeMigration(t, dir, "00001_create_widgets",
		"CREATE TABLE widgets (id INT PRIMARY KEY);",
		"DROP TABLE widgets;")
	writeMigration(t, dir, "00002_add_name",
		"ALTER TABLE widgets ADD COLUMN name TEXT;",
		"ALTER TABLE widgets DROP COLUMN name;")

	ctx := context.Background()
	r, err := NewRunner(dsn, os.DirFS(dir), ".")
	if err != nil {
		t.Fatal(err)
	}

	applied, err := r.Up(ctx)
	if err != nil {
		t.Fatalf("up: %v", err)
	}
	if len(applied) != 2 {
		t.Fatalf("applied=%d want 2", len(applied))
	}

	v, dirty, err := r.Version(ctx)
	if err != nil || dirty || v != 2 {
		t.Fatalf("version=(%d,%v,%v)", v, dirty, err)
	}

	st, err := r.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(st) != 2 || !st[0].Applied || st[0].AppliedAt == nil {
		t.Fatalf("status=%+v", st)
	}

	down, err := r.Down(ctx)
	if err != nil || down == nil || down.Version != 2 {
		t.Fatalf("down=%+v err=%v", down, err)
	}
	v, _, _ = r.Version(ctx)
	if v != 1 {
		t.Fatalf("after down version=%d want 1", v)
	}
}
