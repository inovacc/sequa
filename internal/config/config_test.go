package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveDSN(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://env")
	if got := ResolveDSN("postgres://flag"); got != "postgres://flag" {
		t.Errorf("flag should win, got %q", got)
	}
	if got := ResolveDSN(""); got != "postgres://env" {
		t.Errorf("env fallback failed, got %q", got)
	}
}

func TestAutodetectDir(t *testing.T) {
	root := t.TempDir()
	mig := filepath.Join(root, "db", "migrations")
	if err := os.MkdirAll(mig, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mig, "00001_init.up.sql"), []byte("-- up"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := AutodetectDir(root)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got != mig {
		t.Errorf("got %q want %q", got, mig)
	}
}

func TestAutodetectDirNone(t *testing.T) {
	if _, err := AutodetectDir(t.TempDir()); err == nil {
		t.Error("expected ErrNoMigrationsDir")
	}
}
