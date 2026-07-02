package codegen

import (
	"context"
	"database/sql"
	"os"
	"testing"

	dbpkg "github.com/inovacc/sequa/internal/db"
)

func introspectTestDB(t *testing.T) *sql.DB {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping introspection integration test in -short mode")
	}
	dsn := os.Getenv("SEQUA_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set SEQUA_TEST_DATABASE_URL to run introspection integration tests")
	}
	db, err := dbpkg.Open(context.Background(), dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// TestIntrospectMatchesStatic is the core M4 guarantee: introspecting a schema
// produces the same catalog BuildCatalog derives from the DDL that created it,
// so a faithfully-migrated database shows zero drift. It also confirms a real
// change is detected.
func TestIntrospectMatchesStatic(t *testing.T) {
	db := introspectTestDB(t)
	ctx := context.Background()

	reset := func() {
		if _, err := db.ExecContext(ctx, `DROP TABLE IF EXISTS books, authors CASCADE`); err != nil {
			t.Fatalf("reset: %v", err)
		}
	}
	reset()
	t.Cleanup(reset)

	schema := `CREATE TABLE authors (
		id         BIGSERIAL   PRIMARY KEY,
		name       TEXT        NOT NULL,
		bio        TEXT,
		created_at TIMESTAMPTZ NOT NULL DEFAULT now()
	);
	CREATE TABLE books (
		id          BIGSERIAL   PRIMARY KEY,
		author_id   BIGINT      NOT NULL,
		title       TEXT        NOT NULL,
		published   BOOLEAN     NOT NULL DEFAULT FALSE,
		tags        TEXT[],
		price_cents INT
	);`
	if _, err := db.ExecContext(ctx, schema); err != nil {
		t.Fatalf("create schema: %v", err)
	}

	static, err := BuildCatalog([]string{schema})
	if err != nil {
		t.Fatal(err)
	}
	live, err := Introspect(ctx, db)
	if err != nil {
		t.Fatalf("introspect: %v", err)
	}
	if diffs := DiffCatalogs(static, live); len(diffs) != 0 {
		t.Fatalf("expected no drift between migrations and live schema, got: %v", diffs)
	}

	// Drop a column live and confirm the drift is reported.
	if _, err := db.ExecContext(ctx, `ALTER TABLE books DROP COLUMN tags`); err != nil {
		t.Fatalf("alter: %v", err)
	}
	live, err = Introspect(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	diffs := DiffCatalogs(static, live)
	found := false
	for _, d := range diffs {
		if d.Kind == DiffColumnMissing && d.Table == "books" && d.Column == "tags" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected books.tags column_missing after DROP COLUMN, got: %v", diffs)
	}
}
