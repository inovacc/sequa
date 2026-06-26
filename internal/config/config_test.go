package config

import (
	"errors"
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
	if _, err := AutodetectDir(t.TempDir()); !errors.Is(err, ErrNoMigrationsDir) {
		t.Errorf("expected ErrNoMigrationsDir, got %v", err)
	}
}

func TestResolveDir(t *testing.T) {
	// flag wins
	got, err := ResolveDir("custom/dir", t.TempDir())
	if err != nil || got != "custom/dir" {
		t.Fatalf("flag should win: got=%q err=%v", got, err)
	}
	// autodetect fallback
	root := t.TempDir()
	mig := filepath.Join(root, "migrations")
	if err := os.MkdirAll(mig, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mig, "00001_x.up.sql"), []byte("-- up"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err = ResolveDir("", root)
	if err != nil || got != mig {
		t.Fatalf("autodetect fallback: got=%q err=%v", got, err)
	}
}

func TestAutodetectDirOrder(t *testing.T) {
	root := t.TempDir()
	for _, c := range []string{"migrations", "db/migrations"} {
		d := filepath.Join(root, filepath.FromSlash(c))
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(d, "00001_x.up.sql"), []byte("-- up"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got, err := AutodetectDir(root)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, "migrations")
	if got != want {
		t.Errorf("ordering: got %q want %q (first candidate must win)", got, want)
	}
}
