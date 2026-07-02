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

func TestParseFilename(t *testing.T) {
	tests := []struct {
		name        string
		base        string
		wantVersion uint64
		wantName    string
		wantErr     bool
	}{
		{name: "zero-padded sequential", base: "00001_add_users.up.sql", wantVersion: 1, wantName: "add_users"},
		{name: "timestamp version", base: "20170506082420_add_col.up.sql", wantVersion: 20170506082420, wantName: "add_col"},
		{name: "name keeps later underscores", base: "5_a_b_c.up.sql", wantVersion: 5, wantName: "a_b_c"},
		{name: "no underscore", base: "nounderscore.up.sql", wantErr: true},
		{name: "leading underscore is empty version", base: "_leading.up.sql", wantErr: true},
		{name: "non-numeric version", base: "abc_name.up.sql", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, name, err := ParseFilename(tt.base)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseFilename(%q) = (%d, %q, nil), want error", tt.base, v, name)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseFilename(%q) unexpected error: %v", tt.base, err)
			}
			if v != tt.wantVersion || name != tt.wantName {
				t.Errorf("ParseFilename(%q) = (%d, %q), want (%d, %q)", tt.base, v, name, tt.wantVersion, tt.wantName)
			}
		})
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
