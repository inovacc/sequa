package migrate

import (
	"context"
	"os"
	"path/filepath"
	"testing"
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

func TestRunnerUpStatusDownVersion(t *testing.T) {
	dsn := testDSN(t)
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

	// Clean slate: roll everything down first (ignore result), then up.
	for {
		if _, err := r.Down(ctx); err != nil {
			t.Fatalf("predown: %v", err)
		}
		v, _, _ := r.Version(ctx)
		if v == 0 {
			break
		}
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
