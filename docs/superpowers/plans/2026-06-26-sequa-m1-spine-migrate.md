# Sequa M1 — Spine + Migrate — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship a usable, embeddable Postgres migration tool — `sequa migrate create/up/down/status/version` plus a `pkg/sequa` library for app self-migration — on the shared config/db spine.

**Architecture:** Cobra CLI (`cmd/sequa` → `internal/cli`) over a shared spine (`internal/config` for DSN + migrations-dir autodetect, `internal/db` for connection-by-scheme). The `internal/migrate` package wraps `golang-migrate/v4` (via `iofs` source + `postgres.WithInstance`) and adds a goose-style UX: two-file `.up.sql`/`.down.sql` migrations, timestamp/sequential naming, and a companion `sequa_schema_history` table that supplies the "Applied At" column golang-migrate's own table lacks. `pkg/sequa` is a thin public wrapper so apps never import `internal/`.

**Tech Stack:** Go 1.23, `github.com/spf13/cobra`, `github.com/golang-migrate/migrate/v4` (`source/iofs`, `database/postgres`), `github.com/lib/pq`, `log/slog`.

## Global Constraints

- **Module:** `github.com/inovacc/sequa`; **Go:** `1.23`.
- **License:** BSD-3-Clause (`Copyright (c) 2026, inovacc` — adjust holder if needed).
- **Migration model:** SQL-only, **two files per migration** (`<version>_<name>.up.sql` + `.down.sql`); **no** goose `-- +goose` markers.
- **Versioning:** default = UTC timestamp `YYYYMMDDHHMMSS`; `-s` = sequential `%05d` (first `00001`, next = max existing prefix + 1).
- **golang-migrate usage:** build via `iofs.New(fsys, subdir)` + `postgres.WithInstance(db, &postgres.Config{})` + `migrate.NewWithInstance(...)`. **Never** use `file://` URLs (Windows path hazard). `Version()` returns `(uint, bool, error)` and yields `migrate.ErrNilVersion` on a fresh DB. `Close()` returns **two** errors. Tolerate `migrate.ErrNoChange` and `migrate.ErrShortLimit` (treat as "nothing to do").
- **"Applied At":** companion table `sequa_schema_history(id, version, name, direction, applied_at)`, written after each successful single step. Known limitation: not in the migration's own tx (a crash between the step commit and the history insert drops one history row) — `status` tolerates missing rows.
- **Engine scope (M1):** PostgreSQL only. The db layer and runner are structured so MySQL/SQLite slot in behind the same functions later (fast-follow).
- **Logging:** `log/slog` JSON to **stderr** (stdout reserved for data); gated by `--verbose`.
- **Errors:** wrap with `%w`; compare with `errors.Is`/`errors.As`.
- **Tests:** table-driven; any test needing a real database calls `t.Skip()` under `testing.Short()` **and** when `SEQUA_TEST_DATABASE_URL` is unset. Default `task test` runs `-short`; `task test:full` runs all.
- **Commits:** conventional commits, no AI attribution.

---

### Task 1: Project scaffold + CLI skeleton

**Files:**
- Create: `go.mod`, `cmd/sequa/main.go`, `internal/cli/root.go`, `internal/cli/migrate.go`, `internal/cli/generate.go`, `internal/cli/query.go`, `internal/cli/init.go`, `internal/cli/root_test.go`, `Taskfile.yml`, `.gitignore`, `LICENSE`
- Test: `internal/cli/root_test.go`

**Interfaces:**
- Produces: `cli.Execute()`; `newRootCmd() *cobra.Command`; package-level flag vars `flagDSN, flagDir string`, `flagVerbose bool`; command constructors `newMigrateCmd/newGenerateCmd/newQueryCmd/newInitCmd() *cobra.Command`; `setupLogger(verbose bool)`.

- [ ] **Step 1: Write the failing test** — `internal/cli/root_test.go`

```go
package cli

import "testing"

func TestRootHasCoreSubcommands(t *testing.T) {
	root := newRootCmd()
	want := map[string]bool{"migrate": false, "generate": false, "query": false, "init": false}
	for _, c := range root.Commands() {
		if _, ok := want[c.Name()]; ok {
			want[c.Name()] = true
		}
	}
	for name, found := range want {
		if !found {
			t.Errorf("root missing subcommand %q", name)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/cli/ -run TestRootHasCoreSubcommands`
Expected: FAIL — build error, `newRootCmd` undefined.

- [ ] **Step 3: Write `go.mod`**

```
module github.com/inovacc/sequa

go 1.23

require github.com/spf13/cobra v1.8.1
```

- [ ] **Step 4: Write `cmd/sequa/main.go`**

```go
package main

import "github.com/inovacc/sequa/internal/cli"

func main() {
	cli.Execute()
}
```

- [ ] **Step 5: Write `internal/cli/root.go`**

```go
package cli

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"
)

var (
	flagDSN     string
	flagDir     string
	flagVerbose bool
)

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "sequa",
		Short:         "SQL migration, query, and codegen toolkit",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(_ *cobra.Command, _ []string) error {
			setupLogger(flagVerbose)
			return nil
		},
	}

	pf := root.PersistentFlags()
	pf.StringVar(&flagDSN, "dsn", "", "database DSN (falls back to $DATABASE_URL)")
	pf.StringVar(&flagDir, "dir", "", "migrations directory (autodetected if empty)")
	pf.BoolVarP(&flagVerbose, "verbose", "v", false, "verbose (debug) logging")

	root.AddCommand(newMigrateCmd(), newGenerateCmd(), newQueryCmd(), newInitCmd())
	return root
}

// Execute is the single entrypoint used by main().
func Execute() {
	if err := newRootCmd().ExecuteContext(context.Background()); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func setupLogger(verbose bool) {
	level := slog.LevelInfo
	if verbose {
		level = slog.LevelDebug
	}
	// stderr: stdout is reserved for query/data output.
	h := slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: level})
	slog.SetDefault(slog.New(h))
}
```

- [ ] **Step 6: Write `internal/cli/migrate.go`** (parent only; children added in Tasks 3 & 5)

```go
package cli

import "github.com/spf13/cobra"

func newMigrateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Manage database migrations",
	}
	// children registered in later tasks:
	//   cmd.AddCommand(newMigrateCreateCmd(), newMigrateUpCmd(), newMigrateDownCmd(),
	//       newMigrateStatusCmd(), newMigrateVersionCmd())
	return cmd
}
```

- [ ] **Step 7: Write `internal/cli/generate.go` and `internal/cli/query.go`** (honest stubs for later milestones)

`internal/cli/generate.go`:
```go
package cli

import (
	"errors"

	"github.com/spf13/cobra"
)

func newGenerateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "generate",
		Short: "Generate type-safe Go from migration-defined schema (milestone M3)",
		RunE: func(_ *cobra.Command, _ []string) error {
			return errors.New("generate is not implemented yet (milestone M3)")
		},
	}
}
```

`internal/cli/query.go`:
```go
package cli

import (
	"errors"

	"github.com/spf13/cobra"
)

func newQueryCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "query",
		Short: "Interactive SQL client and REPL (milestone M2)",
		RunE: func(_ *cobra.Command, _ []string) error {
			return errors.New("query is not implemented yet (milestone M2)")
		},
	}
}
```

- [ ] **Step 8: Write `internal/cli/init.go`** (working: creates the migrations dir)

```go
package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

func newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Scaffold a migrations directory",
		RunE: func(_ *cobra.Command, _ []string) error {
			dir := flagDir
			if dir == "" {
				dir = "migrations"
			}
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return fmt.Errorf("create %s: %w", dir, err)
			}
			abs, _ := filepath.Abs(dir)
			fmt.Fprintln(os.Stdout, "initialized migrations directory:", abs)
			return nil
		},
	}
}
```

- [ ] **Step 9: Write `Taskfile.yml`**

```yaml
version: '3'

tasks:
  build:
    desc: Build all packages
    cmds:
      - go build ./...
  test:
    desc: Run short unit tests
    cmds:
      - go test -short ./...
  test:full:
    desc: Run the full test suite (needs SEQUA_TEST_DATABASE_URL)
    cmds:
      - go test ./...
  lint:
    desc: Run golangci-lint
    cmds:
      - golangci-lint run ./...
  run:
    desc: Run the sequa CLI
    cmds:
      - go run ./cmd/sequa {{.CLI_ARGS}}
```

- [ ] **Step 10: Write `.gitignore`**

```
# build output
build/
*.exe
*.exe~
*.dll
*.so
*.dylib

# tests & coverage
*.test
*.out
coverage.*
coverage/

# local / ephemeral
reference/
.scripts/
.env
```

- [ ] **Step 11: Write `LICENSE`** — BSD-3-Clause, `Copyright (c) 2026, inovacc`. Use the standard BSD 3-Clause template verbatim with that holder/year.

- [ ] **Step 12: Resolve deps and run the test**

Run: `go mod tidy && go test ./internal/cli/ -run TestRootHasCoreSubcommands`
Expected: PASS.

- [ ] **Step 13: Smoke-build**

Run: `go build ./... && go run ./cmd/sequa --help`
Expected: build OK; help lists `migrate`, `generate`, `query`, `init`.

- [ ] **Step 14: Commit**

```bash
git add -A
git commit -m "feat: scaffold sequa CLI skeleton and shared entrypoint"
```

---

### Task 2: `internal/config` — DSN resolution + migrations-dir autodetect

**Files:**
- Create: `internal/config/config.go`, `internal/config/config_test.go`

**Interfaces:**
- Produces: `config.ResolveDSN(flagDSN string) string`; `config.AutodetectDir(root string) (string, error)`; `config.ResolveDir(flagDir, root string) (string, error)`; `config.ErrNoMigrationsDir error`; `config.Candidates []string`.

- [ ] **Step 1: Write the failing test** — `internal/config/config_test.go`

```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/config/`
Expected: FAIL — undefined `ResolveDSN`/`AutodetectDir`.

- [ ] **Step 3: Write `internal/config/config.go`**

```go
package config

import (
	"errors"
	"os"
	"path/filepath"
)

// ErrNoMigrationsDir is returned when no candidate migrations directory is found.
var ErrNoMigrationsDir = errors.New("no migrations directory found")

// Candidates are the conventional locations scanned during autodetect.
var Candidates = []string{"migrations", "db/migrations", "sql/migrations", "database/migrations"}

// ResolveDSN returns flagDSN when non-empty, else $DATABASE_URL.
func ResolveDSN(flagDSN string) string {
	if flagDSN != "" {
		return flagDSN
	}
	return os.Getenv("DATABASE_URL")
}

// AutodetectDir returns the first candidate dir under root that exists and
// contains at least one *.up.sql file. Returns ErrNoMigrationsDir otherwise.
func AutodetectDir(root string) (string, error) {
	for _, c := range Candidates {
		p := filepath.Join(root, c)
		info, err := os.Stat(p)
		if err != nil || !info.IsDir() {
			continue
		}
		matches, _ := filepath.Glob(filepath.Join(p, "*.up.sql"))
		if len(matches) > 0 {
			return p, nil
		}
	}
	return "", ErrNoMigrationsDir
}

// ResolveDir returns flagDir when non-empty, else autodetects under root.
func ResolveDir(flagDir, root string) (string, error) {
	if flagDir != "" {
		return flagDir, nil
	}
	return AutodetectDir(root)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/config/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/config/
git commit -m "feat: add config DSN resolution and migrations-dir autodetect"
```

---

### Task 3: `migrate create` — filenames, templates, and the `create` command

**Files:**
- Create: `internal/migrate/create.go`, `internal/migrate/create_test.go`, `internal/cli/migrate_create.go`
- Modify: `internal/cli/migrate.go` (register `newMigrateCreateCmd`)

**Interfaces:**
- Consumes: `config.ResolveDir`.
- Produces: `migrate.Slugify(name string) string`; `migrate.TimestampVersion(t time.Time) string`; `migrate.NextSequential(dir string) (string, error)`; `migrate.Create(dir, name string, sequential bool, now time.Time) ([]string, error)`; `newMigrateCreateCmd() *cobra.Command`.

- [ ] **Step 1: Write the failing test** — `internal/migrate/create_test.go`

```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/migrate/ -run TestCreate`
Expected: FAIL — undefined `Create`.

- [ ] **Step 3: Write `internal/migrate/create.go`**

```go
package migrate

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	upTemplate   = "-- Migration: %s (up)\n-- Write the forward SQL here.\n"
	downTemplate = "-- Migration: %s (down)\n-- Write the rollback SQL here.\n"
)

var nonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

// Slugify lower-cases name and collapses non-alphanumerics to single underscores.
func Slugify(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = nonAlnum.ReplaceAllString(s, "_")
	return strings.Trim(s, "_")
}

// TimestampVersion formats t (UTC) as YYYYMMDDHHMMSS.
func TimestampVersion(t time.Time) string {
	return t.UTC().Format("20060102150405")
}

// NextSequential returns the next zero-padded 5-digit version for dir.
func NextSequential(dir string) (string, error) {
	max, err := maxVersion(dir)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%05d", max+1), nil
}

func maxVersion(dir string) (uint64, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("read dir: %w", err)
	}
	var max uint64
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".up.sql") {
			continue
		}
		trimmed := strings.TrimSuffix(e.Name(), ".up.sql")
		idx := strings.IndexByte(trimmed, '_')
		if idx <= 0 {
			continue
		}
		v, err := strconv.ParseUint(trimmed[:idx], 10, 64)
		if err != nil {
			continue
		}
		if v > max {
			max = v
		}
	}
	return max, nil
}

// Create writes <version>_<slug>.up.sql and .down.sql in dir, returning [up, down].
func Create(dir, name string, sequential bool, now time.Time) ([]string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create dir: %w", err)
	}
	slug := Slugify(name)
	if slug == "" {
		return nil, fmt.Errorf("invalid migration name %q", name)
	}
	var version string
	if sequential {
		v, err := NextSequential(dir)
		if err != nil {
			return nil, err
		}
		version = v
	} else {
		version = TimestampVersion(now)
	}
	stem := version + "_" + slug
	upPath := filepath.Join(dir, stem+".up.sql")
	downPath := filepath.Join(dir, stem+".down.sql")
	if err := os.WriteFile(upPath, []byte(fmt.Sprintf(upTemplate, slug)), 0o644); err != nil {
		return nil, fmt.Errorf("write up: %w", err)
	}
	if err := os.WriteFile(downPath, []byte(fmt.Sprintf(downTemplate, slug)), 0o644); err != nil {
		return nil, fmt.Errorf("write down: %w", err)
	}
	return []string{upPath, downPath}, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/migrate/ -run TestCreate -run TestSlugify -run TestTimestampVersion`
Expected: PASS. (Or simply `go test ./internal/migrate/`.)

- [ ] **Step 5: Write `internal/cli/migrate_create.go`**

```go
package cli

import (
	"fmt"
	"os"
	"time"

	"github.com/inovacc/sequa/internal/config"
	"github.com/inovacc/sequa/internal/migrate"
	"github.com/spf13/cobra"
)

func newMigrateCreateCmd() *cobra.Command {
	var seq bool
	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a new up/down migration pair",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			dir, err := config.ResolveDir(flagDir, ".")
			if err != nil {
				dir = "migrations" // default target when none detected yet
			}
			paths, err := migrate.Create(dir, args[0], seq, time.Now())
			if err != nil {
				return err
			}
			for _, p := range paths {
				fmt.Fprintln(os.Stdout, "Created", p)
			}
			return nil
		},
	}
	cmd.Flags().BoolVarP(&seq, "sequential", "s", false, "sequential numbering instead of timestamp")
	return cmd
}
```

- [ ] **Step 6: Register the child in `internal/cli/migrate.go`**

```go
func newMigrateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Manage database migrations",
	}
	cmd.AddCommand(newMigrateCreateCmd())
	return cmd
}
```

- [ ] **Step 7: Verify build + manual smoke**

Run: `go build ./... && go run ./cmd/sequa migrate create add_users --dir ./_smoke && ls ./_smoke && rm -rf ./_smoke`
Expected: two files `*_add_users.up.sql` / `.down.sql` printed and listed.

- [ ] **Step 8: Commit**

```bash
git add internal/migrate/create.go internal/migrate/create_test.go internal/cli/migrate_create.go internal/cli/migrate.go
git commit -m "feat: add 'migrate create' with timestamp/sequential two-file migrations"
```

---

### Task 4: `internal/db` — connection by DSN scheme

**Files:**
- Create: `internal/db/db.go`, `internal/db/db_test.go`

**Interfaces:**
- Produces: `db.DriverName(dsn string) (string, error)`; `db.Open(ctx context.Context, dsn string) (*sql.DB, error)`.

- [ ] **Step 1: Write the failing test** — `internal/db/db_test.go`

```go
package db

import (
	"context"
	"os"
	"testing"
)

func TestDriverName(t *testing.T) {
	cases := []struct {
		dsn     string
		want    string
		wantErr bool
	}{
		{"postgres://u:p@localhost/db", "postgres", false},
		{"postgresql://u:p@localhost/db", "postgres", false},
		{"mysql://u:p@localhost/db", "", true},
		{"", "", true},
	}
	for _, c := range cases {
		got, err := DriverName(c.dsn)
		if (err != nil) != c.wantErr || got != c.want {
			t.Errorf("DriverName(%q)=(%q,%v) want (%q,err=%v)", c.dsn, got, err, c.want, c.wantErr)
		}
	}
}

func TestOpenIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB integration test in -short mode")
	}
	dsn := os.Getenv("SEQUA_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set SEQUA_TEST_DATABASE_URL to run DB integration tests")
	}
	conn, err := Open(context.Background(), dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = conn.Close() }()
	if err := conn.Ping(); err != nil {
		t.Fatalf("ping: %v", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/db/ -run TestDriverName`
Expected: FAIL — undefined `DriverName`.

- [ ] **Step 3: Write `internal/db/db.go`**

```go
package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "github.com/lib/pq" // registers the "postgres" sql driver
)

// DriverName maps a DSN scheme to a database/sql driver name.
// M1 supports PostgreSQL only; other schemes return an error.
func DriverName(dsn string) (string, error) {
	switch {
	case strings.HasPrefix(dsn, "postgres://"), strings.HasPrefix(dsn, "postgresql://"):
		return "postgres", nil
	default:
		return "", fmt.Errorf("unsupported or missing DSN scheme: %q", dsn)
	}
}

// Open opens and pings a database, selecting the driver by DSN scheme.
// The caller owns the returned *sql.DB and must Close it.
func Open(ctx context.Context, dsn string) (*sql.DB, error) {
	if dsn == "" {
		return nil, fmt.Errorf("empty DSN")
	}
	driver, err := DriverName(dsn)
	if err != nil {
		return nil, err
	}
	conn, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := conn.PingContext(pingCtx); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("ping db: %w", err)
	}
	return conn, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go mod tidy && go test -short ./internal/db/`
Expected: PASS (integration test skipped under `-short`).

- [ ] **Step 5: Commit**

```bash
git add internal/db/
git commit -m "feat: add db connection layer with DSN-scheme driver selection"
```

---

### Task 5: `internal/migrate` runner + `up`/`down`/`status`/`version` commands

**Files:**
- Create: `internal/migrate/runner.go`, `internal/migrate/runner_test.go`, `internal/cli/migrate_up.go`, `internal/cli/migrate_down.go`, `internal/cli/migrate_status.go`, `internal/cli/migrate_version.go`
- Modify: `internal/cli/migrate.go` (register the four children)

**Interfaces:**
- Consumes: `db.Open`; `config.ResolveDSN`, `config.ResolveDir`; `iofs.New`, `postgres.WithInstance`, `migrate.NewWithInstance`.
- Produces:
  - `migrate.NewRunner(dsn string, srcFS fs.FS, subdir string) (*Runner, error)`
  - `(*Runner).Up(ctx context.Context) ([]Applied, error)`
  - `(*Runner).Down(ctx context.Context) (*Applied, error)`
  - `(*Runner).Status(ctx context.Context) ([]Status, error)`
  - `(*Runner).Version(ctx context.Context) (version uint, dirty bool, err error)`
  - `type Applied struct { Version uint; Name string; AppliedAt time.Time }`
  - `type Status struct { Version uint; Name string; Applied bool; AppliedAt *time.Time }`
  - `newMigrateUpCmd/newMigrateDownCmd/newMigrateStatusCmd/newMigrateVersionCmd() *cobra.Command`

> **Naming note:** the package is imported as `migrate` for our code and the upstream library is also `migrate`. Import the upstream as `migratelib "github.com/golang-migrate/migrate/v4"` inside `runner.go` to avoid the self-collision (runner.go is in `package migrate`).

- [ ] **Step 1: Write the failing integration test** — `internal/migrate/runner_test.go`

```go
package migrate

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// writeMigration creates an up/down pair in dir.
func writeMigration(t *testing.T, dir, stem, up, down string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, stem+".up.sql"), []byte(up), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, stem+".down.sql"), []byte(down), 0o644); err != nil {
		t.Fatal(err)
	}
}

func testDSN(t *testing.T) string {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping migrate integration test in -short mode")
	}
	dsn := os.Getenv("SEQUA_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set SEQUA_TEST_DATABASE_URL to run migrate integration tests")
	}
	return dsn
}

func TestRunnerUpStatusDownVersion(t *testing.T) {
	dsn := testDSN(t)
	dir := t.TempDir()
	writeMigration(t, dir, "00001_create_widgets",
		"CREATE TABLE widgets (id INT PRIMARY KEY);",
		"DROP TABLE widgets;")
	writeMigration(t, dir, "00002_add_name",
		"ALTER TABLE widgets ADD COLUMN name TEXT;",
		"ALTER TABLE widgets DROP COLUMN name;")

	ctx := context.Background()
	r, err := NewRunner(dsn, os.DirFS(dir), ".")
	if err != nil {
		t.Fatal(err)
	}

	// Clean slate: roll everything down first (ignore result), then up.
	for {
		if _, err := r.Down(ctx); err != nil {
			t.Fatalf("predown: %v", err)
		}
		v, _, _ := r.Version(ctx)
		if v == 0 {
			break
		}
	}

	applied, err := r.Up(ctx)
	if err != nil {
		t.Fatalf("up: %v", err)
	}
	if len(applied) != 2 {
		t.Fatalf("applied=%d want 2", len(applied))
	}

	v, dirty, err := r.Version(ctx)
	if err != nil || dirty || v != 2 {
		t.Fatalf("version=(%d,%v,%v)", v, dirty, err)
	}

	st, err := r.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(st) != 2 || !st[0].Applied || st[0].AppliedAt == nil {
		t.Fatalf("status=%+v", st)
	}

	down, err := r.Down(ctx)
	if err != nil || down == nil || down.Version != 2 {
		t.Fatalf("down=%+v err=%v", down, err)
	}
	v, _, _ = r.Version(ctx)
	if v != 1 {
		t.Fatalf("after down version=%d want 1", v)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/migrate/ -run TestRunnerUpStatusDownVersion`
Expected: FAIL — undefined `NewRunner` (compile error). (Under `-short` it would skip; run without `-short`, but it still fails to compile until Step 3.)

- [ ] **Step 3: Write `internal/migrate/runner.go`**

```go
package migrate

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	migratelib "github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"

	dbpkg "github.com/inovacc/sequa/internal/db"
)

const historyDDL = `CREATE TABLE IF NOT EXISTS sequa_schema_history (
  id         BIGSERIAL   PRIMARY KEY,
  version    BIGINT      NOT NULL,
  name       TEXT        NOT NULL,
  direction  TEXT        NOT NULL DEFAULT 'up',
  applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
);`

// Applied describes one migration that was applied (or rolled back).
type Applied struct {
	Version   uint
	Name      string
	AppliedAt time.Time
}

// Status describes a migration's applied state for `migrate status`.
type Status struct {
	Version   uint
	Name      string
	Applied   bool
	AppliedAt *time.Time
}

// Runner applies migrations from srcFS/subdir against the database at dsn.
// It opens a fresh *sql.DB per operation (the migrate driver closes it),
// so it never closes a database it does not own.
type Runner struct {
	dsn    string
	srcFS  fs.FS
	subdir string
	names  map[uint]string // version -> migration name
}

// NewRunner indexes the migration filenames in srcFS/subdir.
func NewRunner(dsn string, srcFS fs.FS, subdir string) (*Runner, error) {
	names, err := loadNames(srcFS, subdir)
	if err != nil {
		return nil, err
	}
	return &Runner{dsn: dsn, srcFS: srcFS, subdir: subdir, names: names}, nil
}

func loadNames(srcFS fs.FS, subdir string) (map[uint]string, error) {
	matches, err := fs.Glob(srcFS, path.Join(subdir, "*.up.sql"))
	if err != nil {
		return nil, fmt.Errorf("glob migrations: %w", err)
	}
	names := make(map[uint]string, len(matches))
	for _, f := range matches {
		v, name, err := parseFilename(path.Base(f))
		if err != nil {
			continue
		}
		names[v] = name
	}
	return names, nil
}

func parseFilename(base string) (uint, string, error) {
	trimmed := strings.TrimSuffix(base, ".up.sql")
	idx := strings.IndexByte(trimmed, '_')
	if idx <= 0 {
		return 0, "", fmt.Errorf("bad migration filename %q", base)
	}
	v, err := strconv.ParseUint(trimmed[:idx], 10, 64)
	if err != nil {
		return 0, "", fmt.Errorf("bad version in %q: %w", base, err)
	}
	return uint(v), trimmed[idx+1:], nil
}

func sortedVersions(names map[uint]string) []uint {
	out := make([]uint, 0, len(names))
	for v := range names {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// withMigrate opens a db + migrate instance, runs fn, then closes both via
// m.Close() (which closes the source and the per-op db). The db is created
// here, so closing it is safe.
func (r *Runner) withMigrate(ctx context.Context, fn func(db *sql.DB, m *migratelib.Migrate) error) error {
	conn, err := dbpkg.Open(ctx, r.dsn)
	if err != nil {
		return err
	}
	src, err := iofs.New(r.srcFS, r.subdir)
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("iofs source: %w", err)
	}
	drv, err := postgres.WithInstance(conn, &postgres.Config{})
	if err != nil {
		_ = src.Close()
		_ = conn.Close()
		return fmt.Errorf("postgres driver: %w", err)
	}
	m, err := migratelib.NewWithInstance("iofs", src, "postgres", drv)
	if err != nil {
		_ = src.Close()
		_ = conn.Close()
		return fmt.Errorf("migrate init: %w", err)
	}
	defer func() {
		// Closes the iofs source and the per-op db driver.
		if srcErr, dbErr := m.Close(); srcErr != nil || dbErr != nil {
			slog.Warn("migrate close", "src", srcErr, "db", dbErr)
		}
	}()
	return fn(conn, m)
}

func ensureHistory(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, historyDDL); err != nil {
		return fmt.Errorf("ensure history table: %w", err)
	}
	return nil
}

func stop(err error) bool {
	if errors.Is(err, migratelib.ErrNoChange) {
		return true
	}
	var short migratelib.ErrShortLimit
	return errors.As(err, &short)
}

// Up applies all pending migrations one step at a time, recording history.
func (r *Runner) Up(ctx context.Context) ([]Applied, error) {
	var applied []Applied
	err := r.withMigrate(ctx, func(db *sql.DB, m *migratelib.Migrate) error {
		if err := ensureHistory(ctx, db); err != nil {
			return err
		}
		for {
			if err := m.Steps(1); err != nil {
				if stop(err) {
					return nil
				}
				return fmt.Errorf("apply step: %w", err)
			}
			v, _, err := m.Version()
			if err != nil {
				return fmt.Errorf("read version: %w", err)
			}
			name := r.names[v]
			if _, err := db.ExecContext(ctx,
				`INSERT INTO sequa_schema_history (version, name, direction) VALUES ($1, $2, 'up')`,
				int64(v), name); err != nil {
				return fmt.Errorf("record history: %w", err)
			}
			applied = append(applied, Applied{Version: v, Name: name, AppliedAt: time.Now()})
		}
	})
	return applied, err
}

// Down rolls back the single most-recent migration (goose semantics).
func (r *Runner) Down(ctx context.Context) (*Applied, error) {
	var result *Applied
	err := r.withMigrate(ctx, func(db *sql.DB, m *migratelib.Migrate) error {
		if err := ensureHistory(ctx, db); err != nil {
			return err
		}
		before, _, verr := m.Version()
		if verr != nil {
			if errors.Is(verr, migratelib.ErrNilVersion) {
				return nil // nothing to roll back
			}
			return fmt.Errorf("read version: %w", verr)
		}
		if err := m.Steps(-1); err != nil {
			if stop(err) {
				return nil
			}
			return fmt.Errorf("rollback step: %w", err)
		}
		name := r.names[before]
		if _, err := db.ExecContext(ctx,
			`INSERT INTO sequa_schema_history (version, name, direction) VALUES ($1, $2, 'down')`,
			int64(before), name); err != nil {
			return fmt.Errorf("record history: %w", err)
		}
		result = &Applied{Version: before, Name: name, AppliedAt: time.Now()}
		return nil
	})
	return result, err
}

// Version returns the current schema version and dirty flag (0,false on fresh DB).
func (r *Runner) Version(ctx context.Context) (uint, bool, error) {
	var version uint
	var dirty bool
	err := r.withMigrate(ctx, func(_ *sql.DB, m *migratelib.Migrate) error {
		v, d, verr := m.Version()
		if verr != nil {
			if errors.Is(verr, migratelib.ErrNilVersion) {
				version, dirty = 0, false
				return nil
			}
			return verr
		}
		version, dirty = v, d
		return nil
	})
	return version, dirty, err
}

// Status reports every known migration with its applied state + applied_at.
func (r *Runner) Status(ctx context.Context) ([]Status, error) {
	var out []Status
	err := r.withMigrate(ctx, func(db *sql.DB, m *migratelib.Migrate) error {
		var current uint
		if v, _, verr := m.Version(); verr != nil {
			if !errors.Is(verr, migratelib.ErrNilVersion) {
				return verr
			}
			current = 0
		} else {
			current = v
		}
		hist, err := historyMap(ctx, db)
		if err != nil {
			return err
		}
		for _, v := range sortedVersions(r.names) {
			s := Status{Version: v, Name: r.names[v], Applied: current != 0 && v <= current}
			if at, ok := hist[v]; ok {
				a := at
				s.AppliedAt = &a
			}
			out = append(out, s)
		}
		return nil
	})
	return out, err
}

// historyMap returns the latest up-applied_at per version. A missing history
// table yields an empty map (history is best-effort).
func historyMap(ctx context.Context, db *sql.DB) (map[uint]time.Time, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT version, max(applied_at) FROM sequa_schema_history WHERE direction = 'up' GROUP BY version`)
	if err != nil {
		return map[uint]time.Time{}, nil // table not created yet
	}
	defer func() { _ = rows.Close() }()
	m := make(map[uint]time.Time)
	for rows.Next() {
		var v int64
		var at time.Time
		if err := rows.Scan(&v, &at); err != nil {
			return nil, fmt.Errorf("scan history: %w", err)
		}
		m[uint(v)] = at
	}
	return m, rows.Err()
}
```

- [ ] **Step 4: Run the integration test to verify it passes**

Prereq: a throwaway Postgres, e.g. `docker run --rm -d -p 5432:5432 -e POSTGRES_PASSWORD=pass --name sequa-pg postgres:16` then
`export SEQUA_TEST_DATABASE_URL='postgres://postgres:pass@localhost:5432/postgres?sslmode=disable'`

Run: `go test ./internal/migrate/ -run TestRunnerUpStatusDownVersion -v`
Expected: PASS. (Also confirm `go test -short ./internal/migrate/` PASSES by skipping.)

- [ ] **Step 5: Write the four command files**

`internal/cli/migrate_up.go`:
```go
package cli

import (
	"fmt"
	"os"

	"github.com/inovacc/sequa/internal/config"
	"github.com/inovacc/sequa/internal/migrate"
	"github.com/spf13/cobra"
)

func resolveDSNAndDir() (string, string, error) {
	dsn := config.ResolveDSN(flagDSN)
	if dsn == "" {
		return "", "", fmt.Errorf("no DSN: pass --dsn or set DATABASE_URL")
	}
	dir, err := config.ResolveDir(flagDir, ".")
	if err != nil {
		return "", "", err
	}
	return dsn, dir, nil
}

func newMigrateUpCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "up",
		Short: "Apply all pending migrations",
		RunE: func(cmd *cobra.Command, _ []string) error {
			dsn, dir, err := resolveDSNAndDir()
			if err != nil {
				return err
			}
			r, err := migrate.NewRunner(dsn, os.DirFS(dir), ".")
			if err != nil {
				return err
			}
			applied, err := r.Up(cmd.Context())
			if err != nil {
				return err
			}
			if len(applied) == 0 {
				fmt.Fprintln(os.Stdout, "no migrations to run")
			}
			for _, a := range applied {
				fmt.Fprintf(os.Stdout, "OK   %d_%s\n", a.Version, a.Name)
			}
			return nil
		},
	}
}
```

`internal/cli/migrate_down.go`:
```go
package cli

import (
	"fmt"
	"os"

	"github.com/inovacc/sequa/internal/migrate"
	"github.com/spf13/cobra"
)

func newMigrateDownCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "down",
		Short: "Roll back the most recent migration",
		RunE: func(cmd *cobra.Command, _ []string) error {
			dsn, dir, err := resolveDSNAndDir()
			if err != nil {
				return err
			}
			r, err := migrate.NewRunner(dsn, os.DirFS(dir), ".")
			if err != nil {
				return err
			}
			down, err := r.Down(cmd.Context())
			if err != nil {
				return err
			}
			if down == nil {
				fmt.Fprintln(os.Stdout, "nothing to roll back")
				return nil
			}
			fmt.Fprintf(os.Stdout, "DOWN %d_%s\n", down.Version, down.Name)
			return nil
		},
	}
}
```

`internal/cli/migrate_status.go`:
```go
package cli

import (
	"fmt"
	"os"

	"github.com/inovacc/sequa/internal/migrate"
	"github.com/spf13/cobra"
)

func newMigrateStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show applied and pending migrations",
		RunE: func(cmd *cobra.Command, _ []string) error {
			dsn, dir, err := resolveDSNAndDir()
			if err != nil {
				return err
			}
			r, err := migrate.NewRunner(dsn, os.DirFS(dir), ".")
			if err != nil {
				return err
			}
			rows, err := r.Status(cmd.Context())
			if err != nil {
				return err
			}
			fmt.Fprintln(os.Stdout, "    Applied At                  Migration")
			fmt.Fprintln(os.Stdout, "    =======================================")
			for _, s := range rows {
				when := "Pending"
				switch {
				case s.AppliedAt != nil:
					when = s.AppliedAt.Format("Mon Jan _2 15:04:05 2006")
				case s.Applied:
					when = "applied"
				}
				fmt.Fprintf(os.Stdout, "    %-27s %d_%s\n", when, s.Version, s.Name)
			}
			return nil
		},
	}
}
```

`internal/cli/migrate_version.go`:
```go
package cli

import (
	"fmt"
	"os"

	"github.com/inovacc/sequa/internal/migrate"
	"github.com/spf13/cobra"
)

func newMigrateVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the current schema version",
		RunE: func(cmd *cobra.Command, _ []string) error {
			dsn, dir, err := resolveDSNAndDir()
			if err != nil {
				return err
			}
			r, err := migrate.NewRunner(dsn, os.DirFS(dir), ".")
			if err != nil {
				return err
			}
			v, dirty, err := r.Version(cmd.Context())
			if err != nil {
				return err
			}
			if v == 0 {
				fmt.Fprintln(os.Stdout, "no migrations applied")
				return nil
			}
			fmt.Fprintf(os.Stdout, "version: %d (dirty=%v)\n", v, dirty)
			return nil
		},
	}
}
```

- [ ] **Step 6: Register the four children in `internal/cli/migrate.go`**

```go
func newMigrateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Manage database migrations",
	}
	cmd.AddCommand(
		newMigrateCreateCmd(),
		newMigrateUpCmd(),
		newMigrateDownCmd(),
		newMigrateStatusCmd(),
		newMigrateVersionCmd(),
	)
	return cmd
}
```

- [ ] **Step 7: Build + full vet/lint, then commit**

Run: `go build ./... && go vet ./... && go test -short ./...`
Expected: all PASS (integration tests skipped under `-short`).

```bash
git add internal/migrate/runner.go internal/migrate/runner_test.go internal/cli/migrate_up.go internal/cli/migrate_down.go internal/cli/migrate_status.go internal/cli/migrate_version.go internal/cli/migrate.go
git commit -m "feat: add migrate runner (up/down/status/version) with applied-at history"
```

---

### Task 6: `pkg/sequa` — embeddable self-migration library

**Files:**
- Create: `pkg/sequa/sequa.go`, `pkg/sequa/sequa_test.go`

**Interfaces:**
- Consumes: `internal/migrate.NewRunner`, `(*Runner).Up/Down/Version`.
- Produces:
  - `sequa.New(dsn string, fsys fs.FS, subdir string) (*Migrator, error)`
  - `(*Migrator).Up(ctx context.Context) error`
  - `(*Migrator).Down(ctx context.Context) error`
  - `(*Migrator).Version(ctx context.Context) (uint, bool, error)`

> **Note (deviation from spec):** the library takes a **DSN string**, not a borrowed `*sql.DB`. Rationale: golang-migrate's `postgres.WithInstance` driver `Close()` would close a caller-owned pool. Taking a DSN lets the library own a short-lived connection per boot-time migration without ever closing the app's pool. This is the safe, idiomatic shape for "self-migrate on startup."

- [ ] **Step 1: Write the failing test** — `pkg/sequa/sequa_test.go`

```go
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
```

Also create the embedded fixtures:
- `pkg/sequa/testdata/migrations/00001_lib_widgets.up.sql`:
```sql
CREATE TABLE lib_widgets (id INT PRIMARY KEY);
```
- `pkg/sequa/testdata/migrations/00001_lib_widgets.down.sql`:
```sql
DROP TABLE lib_widgets;
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./pkg/sequa/ -run TestLibraryUpVersion`
Expected: FAIL — undefined `sequa.New` (compile error).

- [ ] **Step 3: Write `pkg/sequa/sequa.go`**

```go
// Package sequa provides an embeddable migration runner so a Go application
// can apply its own migrations on startup.
package sequa

import (
	"context"
	"io/fs"

	imigrate "github.com/inovacc/sequa/internal/migrate"
)

// Migrator runs embedded migrations against the database identified by a DSN.
type Migrator struct {
	r *imigrate.Runner
}

// New builds a Migrator from a DSN and a filesystem (e.g. embed.FS) whose
// subdir holds <version>_<name>.up.sql / .down.sql pairs.
func New(dsn string, fsys fs.FS, subdir string) (*Migrator, error) {
	r, err := imigrate.NewRunner(dsn, fsys, subdir)
	if err != nil {
		return nil, err
	}
	return &Migrator{r: r}, nil
}

// Up applies all pending migrations.
func (m *Migrator) Up(ctx context.Context) error {
	_, err := m.r.Up(ctx)
	return err
}

// Down rolls back the most recent migration.
func (m *Migrator) Down(ctx context.Context) error {
	_, err := m.r.Down(ctx)
	return err
}

// Version returns the current schema version and dirty flag.
func (m *Migrator) Version(ctx context.Context) (uint, bool, error) {
	return m.r.Version(ctx)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./pkg/sequa/ -run TestLibraryUpVersion -v` (with `SEQUA_TEST_DATABASE_URL` set)
Expected: PASS. Also confirm `go test -short ./pkg/sequa/` PASSES (skips).

- [ ] **Step 5: Commit**

```bash
git add pkg/sequa/
git commit -m "feat: add embeddable pkg/sequa self-migration library"
```

---

## Final verification (whole milestone)

- [ ] `go build ./...` — PASS
- [ ] `go vet ./...` — PASS
- [ ] `golangci-lint run ./...` — clean (install if absent: `go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest`)
- [ ] `go test -short ./...` — PASS (unit tests; integration skipped)
- [ ] With a throwaway Postgres + `SEQUA_TEST_DATABASE_URL`: `go test ./...` — PASS (integration included)
- [ ] Manual end-to-end:
  ```
  go run ./cmd/sequa init
  go run ./cmd/sequa migrate create create_users -s
  # edit the generated .up.sql / .down.sql
  go run ./cmd/sequa --dsn "$SEQUA_TEST_DATABASE_URL" migrate up
  go run ./cmd/sequa --dsn "$SEQUA_TEST_DATABASE_URL" migrate status
  go run ./cmd/sequa --dsn "$SEQUA_TEST_DATABASE_URL" migrate version
  go run ./cmd/sequa --dsn "$SEQUA_TEST_DATABASE_URL" migrate down
  ```

## Out of scope (later milestones)
- M2 `query` (embed usql), M3 `generate` (copy/adapt sqlc Postgres path), M4 `--verify` (ephemeral DB replay), M5 MySQL + SQLite engines. Each gets its own plan.
- Multi-engine `migrate` (MySQL/SQLite drivers in `internal/db` + `withMigrate`) — small fast-follow after M1.
