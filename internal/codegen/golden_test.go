package codegen

import (
	"flag"
	"os"
	"path/filepath"
	"testing"
)

var updateGolden = flag.Bool("update", false, "regenerate codegen golden files")

// TestGolden pins the full generated output (models.go + queries.go) for a
// representative schema and query set. Run `go test ./internal/codegen -run
// TestGolden -update` to regenerate the golden files after an intended change.
func TestGolden(t *testing.T) {
	const dir = "testdata/golden"

	migrations, err := readUpMigrations(filepath.Join(dir, "migrations"))
	if err != nil {
		t.Fatalf("read migrations: %v", err)
	}
	cat, err := BuildCatalog(migrations)
	if err != nil {
		t.Fatalf("build catalog: %v", err)
	}

	models, err := RenderModels(cat, "db")
	if err != nil {
		t.Fatalf("render models: %v", err)
	}
	assertGolden(t, filepath.Join(dir, "models.go.golden"), models)

	querySrc, err := os.ReadFile(filepath.Join(dir, "queries.sql"))
	if err != nil {
		t.Fatalf("read queries.sql: %v", err)
	}
	analyzed, err := AnalyzeQueries(cat, string(querySrc))
	if err != nil {
		t.Fatalf("analyze queries: %v", err)
	}
	queries, err := RenderQueries(cat, analyzed, "db")
	if err != nil {
		t.Fatalf("render queries: %v", err)
	}
	assertGolden(t, filepath.Join(dir, "queries.go.golden"), queries)
}

// assertGolden compares got against the golden file at path, or rewrites it
// under -update.
func assertGolden(t *testing.T, path string, got []byte) {
	t.Helper()
	if *updateGolden {
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatalf("update golden %s: %v", path, err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s (run with -update to create): %v", path, err)
	}
	if string(got) != string(want) {
		t.Errorf("%s mismatch — run 'go test ./internal/codegen -run TestGolden -update' to regenerate.\n--- got ---\n%s", path, got)
	}
}
