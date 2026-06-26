package sequa_test

import (
	"context"
	"embed"
	"os"
	"testing"

	"github.com/inovacc/sequa/pkg/sequa"
)

//go:embed testdata/migrations/*.sql
var migrationsFS embed.FS

func TestLibraryUpVersion(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping library integration test in -short mode")
	}
	dsn := os.Getenv("SEQUA_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set SEQUA_TEST_DATABASE_URL to run library integration tests")
	}
	ctx := context.Background()
	m, err := sequa.New(dsn, migrationsFS, "testdata/migrations")
	if err != nil {
		t.Fatal(err)
	}
	// reset
	for {
		if err := m.Down(ctx); err != nil {
			t.Fatalf("down: %v", err)
		}
		v, _, _ := m.Version(ctx)
		if v == 0 {
			break
		}
	}
	if err := m.Up(ctx); err != nil {
		t.Fatalf("up: %v", err)
	}
	v, _, err := m.Version(ctx)
	if err != nil || v == 0 {
		t.Fatalf("version=(%d,%v)", v, err)
	}
}
