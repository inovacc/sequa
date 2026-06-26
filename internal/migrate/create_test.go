package migrate

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSlugify(t *testing.T) {
	cases := map[string]string{
		"Add Some Column": "add_some_column",
		"  trim--me  ":     "trim_me",
		"users":            "users",
	}
	for in, want := range cases {
		if got := Slugify(in); got != want {
			t.Errorf("Slugify(%q)=%q want %q", in, got, want)
		}
	}
}

func TestTimestampVersion(t *testing.T) {
	ts := time.Date(2017, 5, 6, 8, 24, 20, 0, time.UTC)
	if got := TimestampVersion(ts); got != "20170506082420" {
		t.Errorf("got %q", got)
	}
}

func TestCreateTimestamp(t *testing.T) {
	dir := t.TempDir()
	ts := time.Date(2017, 5, 6, 8, 24, 20, 0, time.UTC)
	paths, err := Create(dir, "add some column", false, ts)
	if err != nil {
		t.Fatal(err)
	}
	wantUp := filepath.Join(dir, "20170506082420_add_some_column.up.sql")
	wantDown := filepath.Join(dir, "20170506082420_add_some_column.down.sql")
	if paths[0] != wantUp || paths[1] != wantDown {
		t.Fatalf("paths=%v", paths)
	}
	for _, p := range paths {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("missing %s: %v", p, err)
		}
	}
}

func TestCreateSequential(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "00001_first.up.sql"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	paths, err := Create(dir, "second", true, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(paths[0]) != "00002_second.up.sql" {
		t.Errorf("got %s", filepath.Base(paths[0]))
	}
}
