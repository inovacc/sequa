package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/inovacc/sequa/internal/codegen"
)

func TestWithDatabase(t *testing.T) {
	cases := []struct{ dsn, db, want string }{
		{"postgres://u:p@host:5432/mydb?sslmode=disable", "other", "postgres://u:p@host:5432/other?sslmode=disable"},
		{"postgres://host/one", "two", "postgres://host/two"},
	}
	for _, c := range cases {
		got, err := withDatabase(c.dsn, c.db)
		if err != nil {
			t.Fatalf("withDatabase(%q, %q): %v", c.dsn, c.db, err)
		}
		if got != c.want {
			t.Errorf("withDatabase(%q, %q) = %q, want %q", c.dsn, c.db, got, c.want)
		}
	}
}

func TestEphemeralDBName(t *testing.T) {
	a, err := ephemeralDBName()
	if err != nil {
		t.Fatal(err)
	}
	b, _ := ephemeralDBName()
	if a == b {
		t.Fatal("ephemeral database names should differ")
	}
	if !strings.HasPrefix(a, "sequa_verify_") {
		t.Errorf("name %q lacks the sequa_verify_ prefix", a)
	}
}

func ephTestDSN(t *testing.T) string {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping ephemeral integration test in -short mode")
	}
	dsn := os.Getenv("SEQUA_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set SEQUA_TEST_DATABASE_URL to run ephemeral integration tests")
	}
	return dsn
}

// TestEphemeralIntrospect exercises the full create -> migrate -> introspect ->
// drop cycle against a real server and asserts the throwaway database matches
// the migrations that built it.
func TestEphemeralIntrospect(t *testing.T) {
	dsn := ephTestDSN(t)
	dir := t.TempDir()
	writeUpDown(t, dir, "0001_init",
		"CREATE TABLE things (id BIGSERIAL PRIMARY KEY, label TEXT NOT NULL, tags TEXT[]);",
		"DROP TABLE things;")

	ctx := context.Background()
	static, err := codegen.CatalogFromMigrations(dir)
	if err != nil {
		t.Fatal(err)
	}
	live, err := ephemeralIntrospect(ctx, dsn, dir)
	if err != nil {
		t.Fatalf("ephemeralIntrospect: %v", err)
	}
	if diffs := codegen.DiffCatalogs(static, live); len(diffs) != 0 {
		t.Fatalf("expected zero drift for the ephemeral database, got: %v", diffs)
	}
}

func writeUpDown(t *testing.T, dir, stem, up, down string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, stem+".up.sql"), []byte(up), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, stem+".down.sql"), []byte(down), 0o644); err != nil {
		t.Fatal(err)
	}
}
