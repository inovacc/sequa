package codegen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeUp writes base (a *.up.sql name) with body into dir.
func writeUp(t *testing.T, dir, base, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, base), []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", base, err)
	}
}

func TestReadUpMigrationsOrdersByVersion(t *testing.T) {
	dir := t.TempDir()
	// Written out of order; must come back sorted by numeric version.
	writeUp(t, dir, "0002_second.up.sql", "SELECT 2;")
	writeUp(t, dir, "0001_first.up.sql", "SELECT 1;")
	writeUp(t, dir, "0010_tenth.up.sql", "SELECT 10;")

	got, err := readUpMigrations(dir)
	if err != nil {
		t.Fatalf("readUpMigrations: %v", err)
	}
	want := []string{"SELECT 1;", "SELECT 2;", "SELECT 10;"}
	if len(got) != len(want) {
		t.Fatalf("got %d migrations, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if strings.TrimSpace(got[i]) != want[i] {
			t.Errorf("migration[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// A malformed .up.sql (no numeric version) must be rejected, not silently
// treated as version 0 and sorted first — that would build the schema catalog
// from the wrong statement order.
func TestReadUpMigrationsRejectsMalformed(t *testing.T) {
	dir := t.TempDir()
	writeUp(t, dir, "0001_ok.up.sql", "SELECT 1;")
	writeUp(t, dir, "oops.up.sql", "SELECT 99;")

	if _, err := readUpMigrations(dir); err == nil {
		t.Fatal("readUpMigrations accepted a malformed filename, want error")
	}
}
