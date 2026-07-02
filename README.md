# sequa
<!-- rev:002 -->

[![CI](https://github.com/inovacc/sequa/actions/workflows/ci.yml/badge.svg)](https://github.com/inovacc/sequa/actions/workflows/ci.yml)

**SQL migration, query, and codegen toolkit — your migrations are the single source of truth.**

`sequa` is a single Go binary (`sequa`) plus an embeddable library (`pkg/sequa`).
It applies SQL migrations, gives you an interactive SQL client, and generates
type-safe Go from the schema your migrations define. The central idea:
**migrations dictate codegen**. The schema is assembled by parsing your
up-migrations — not by introspecting a live database — so generated models and
query methods never drift from the schema you actually ship.

## Status & engine support

`sequa` is Postgres-first. Today every verb targets PostgreSQL; the migration
and query layers are built on engine-agnostic foundations so more engines can be
added without reworking the design.

| Verb | Built on | Engine support today | Direction |
|------|----------|----------------------|-----------|
| `migrate` | [golang-migrate](https://github.com/golang-migrate/migrate) | PostgreSQL (`postgres://`, `postgresql://`) | Engine-agnostic by design; only the Postgres driver is wired up today |
| `query` | [xo/usql](https://github.com/xo/usql) | PostgreSQL | usql supports many engines; only its Postgres driver is registered today |
| `generate` | [pg_query_go](https://github.com/pganalyze/pg_query_go) (`libpg_query`) | PostgreSQL only | MySQL + SQLite planned (M5) |
| `verify` | `pg_catalog` introspection | PostgreSQL only | Drift check: live schema vs migrations |

Codegen is Postgres-only by nature — it uses PostgreSQL's own parser. `migrate`
and `query` are Postgres-only in the current build even though their
foundations are engine-neutral.

## Install

```
go install github.com/inovacc/sequa/cmd/sequa@latest
```

> **cgo note:** the `generate` verb imports `pg_query_go`, which is cgo — building
> the codegen path requires a C compiler (gcc). The `migrate` and `query` paths
> are pure Go.

## Quickstart

### Scaffold a migrations directory

```
sequa init                 # creates ./migrations
sequa init --dir db/migrations
```

### migrate

```
sequa migrate create add_users           # timestamped up/down pair
sequa migrate create add_users -s        # sequential numbering instead

sequa migrate up --dsn "$DATABASE_URL"   # apply all pending migrations
sequa migrate down --dsn "$DATABASE_URL" # roll back the most recent migration
sequa migrate status                     # applied + pending, with timestamps
sequa migrate version                    # current schema version (+ dirty flag)
```

`create` writes a `<version>_<name>.up.sql` / `.down.sql` pair. The other
subcommands resolve the DSN from `--dsn` or `$DATABASE_URL`, and the migrations
directory from `--dir` or autodetection (see [Configuration](#configuration)).

### generate

Codegen is static and offline — no database connection needed. Place a
`sequa.yaml` at your project root:

```yaml
version: "1"
sql:
  - engine: postgresql                  # only postgresql in this milestone
    schema: internal/store/migrations   # the migrations dir (schema source)
    queries: internal/store/queries.sql # a .sql file OR a dir of *.sql (optional)
    gen:
      go:
        package: db                     # generated package name
        out: internal/store/db          # output directory
```

Annotate each query with sqlc-style headers in the `queries` file:

```sql
-- name: GetUser :one
SELECT * FROM users WHERE id = $1;

-- name: ListByEmail :many
SELECT id, email FROM users WHERE email = $1;

-- name: CreateUser :one
INSERT INTO users (email, name) VALUES ($1, $2) RETURNING *;

-- name: DeleteUser :exec
DELETE FROM users WHERE id = $1;
```

| Verb | Returns |
|------|---------|
| `:one`  | `(Row, error)` via `QueryRowContext` |
| `:many` | `([]Row, error)` via `QueryContext` |
| `:exec` | `error` via `ExecContext` |

Result lists may be plain columns, `*`, or the `count`/`min`/`max` aggregates
(`count` → non-null `int64`; `min`/`max` → the column's type, nullable). JOINs
and `sum`/`avg` are not supported yet — see ISS-2 in [docs/ISSUES.md](docs/ISSUES.md).

Then run:

```
sequa generate                          # uses ./sequa.yaml
sequa generate --config path/to/sequa.yaml
```

It writes `<out>/models.go` (one struct per table) always, and `<out>/queries.go`
when `queries` is set. See [docs/GENERATE_AND_QUERY.md](docs/GENERATE_AND_QUERY.md)
for the full type-mapping table and supported query shapes.

### query

An interactive SQL client and REPL powered by usql (psql-style result tables and
backslash meta-commands like `\dt`, `\d`):

```
sequa query "postgres://user:pass@localhost/db?sslmode=disable"   # REPL
sequa query --dsn "$DATABASE_URL"                                 # REPL (env/flag DSN)
sequa query --dsn "$DATABASE_URL" -c "SELECT * FROM tasks;"       # one-shot table
```

The DSN may be a positional argument, `--dsn`, or `$DATABASE_URL`. With `-c`
(`--command`) it runs one statement and exits; otherwise it opens the REPL.

### verify

Check that the live database schema matches what your migrations define — a
drift check and migration smoke test:

```
sequa verify --dsn "$DATABASE_URL"    # prints OK, or lists drift and exits non-zero
```

It parses the up-migrations into a catalog, introspects the live schema via
`pg_catalog`, and reports missing/extra tables and columns plus type and
nullability mismatches.

## Embeddable library — `pkg/sequa`

Apply your own embedded migrations on application startup:

```go
import (
	"context"
	"embed"

	"github.com/inovacc/sequa/pkg/sequa"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

func main() {
	m, err := sequa.New(dsn, migrationsFS, "migrations")
	if err != nil {
		// handle err
	}
	ctx := context.Background()
	if err := m.Up(ctx); err != nil {
		// handle err
	}
}
```

`New(dsn string, fsys fs.FS, subdir string)` returns a `*Migrator` with:

- `Up(ctx) error` — apply all pending migrations
- `Down(ctx) error` — roll back the most recent migration
- `Version(ctx) (uint, bool, error)` — current schema version and dirty flag

The filesystem's `subdir` holds `<version>_<name>.up.sql` / `.down.sql` pairs.

## Configuration

Persistent flags (available on every command):

| Flag | Purpose |
|------|---------|
| `--dsn` | Database DSN. Falls back to `$DATABASE_URL` when empty. |
| `--dir` | Migrations directory. Autodetected when empty. |
| `-v`, `--verbose` | Verbose (debug) logging. |

- **DSN** resolves as `--dsn` → `$DATABASE_URL`. For `query`, a positional DSN
  argument takes precedence.
- **Directory autodetection** scans, in order: `migrations/`, `db/migrations/`,
  `sql/migrations/`, `database/migrations/` — picking the first that exists and
  contains at least one `*.up.sql` file.
- Logs are written to **stderr** as JSON; **stdout** is reserved for query and
  data output.

## Development & testing

The project uses a [Taskfile](https://taskfile.dev):

```
task build        # go build ./...
task test         # go test -short ./...       (unit tests only)
task test:full    # go test ./...              (needs SEQUA_TEST_DATABASE_URL)
task lint         # golangci-lint run ./...
task test:docker  # full suite incl. integration, in Docker vs real Postgres
```

**`-short` gating.** Tests that need a real database call `t.Skip()` under
`testing.Short()` **and** when `SEQUA_TEST_DATABASE_URL` is unset. So `task test`
(which passes `-short`) runs only unit tests; the integration tests run via
`task test:full` (with `SEQUA_TEST_DATABASE_URL` pointing at a Postgres) or via
`task test:docker`.

**Docker integration path.** `task test:docker` brings up `postgres:16-alpine`,
waits for health, and runs the whole suite from a cgo-capable `golang:1.26` image
(gcc is required for `pg_query_go`). Package test binaries run with `-p 1` so the
integration tests, which share one Postgres, don't race on schema creation.

## Milestones

| Milestone | Scope | State |
|-----------|-------|-------|
| **M1** | Spine + `migrate` — config autodetect, DB connection, golang-migrate runner (up/down/status/version), library API | Done |
| **M2** | `query` — embedded usql REPL and one-shot command | Done |
| **M3** | `generate` (Postgres) — models + typed query methods from the migration-defined schema | Done |
| **M4** | `verify` — introspection + drift diff (live schema vs migrations) | Done |
| **M5** | Engines 2 & 3 — MySQL + SQLite codegen behind an `Engine` abstraction | Planned |

## License

BSD 3-Clause. See [LICENSE](LICENSE).
