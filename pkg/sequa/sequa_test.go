package sequa_test

import (
	"context"
	"embed"
	"os"
	"testing"

	dbpkg "github.com/inovacc/sequa/internal/db"
	"github.com/inovacc/sequa/pkg/sequa"
)

//go:embed testdata/migrations/*.sql
var migrationsFS embed.FS

// resetSchema drops the migration-tracking tables and this test's table so the
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
		"DROP TABLE IF EXISTS schema_migrations, sequa_schema_history, lib_widgets CASCADE"); err != nil {
		t.Fatalf("reset drop: %v", err)
	}
}

func TestLibraryUpVersion(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping library integration test in -short mode")
	}
	dsn := os.Getenv("SEQUA_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set SEQUA_TEST_DATABASE_URL to run library integration tests")
	}
	resetSchema(t, dsn)

	ctx := context.Background()
	m, err := sequa.New(dsn, migrationsFS, "testdata/migrations")
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Up(ctx); err != nil {
		t.Fatalf("up: %v", err)
	}
	v, _, err := m.Version(ctx)
	if err != nil || v == 0 {
		t.Fatalf("version=(%d,%v)", v, err)
	}
}
