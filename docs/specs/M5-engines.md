# M5 — MySQL & SQLite codegen (design spec)

Status: proposed · Author: hardening pass 2026-07-02 · Milestone: M5

Point-in-time design record. Not a living doc — do not rev-tag.

## Goal

Extend `sequa generate` (today Postgres-only) to emit type-safe Go for MySQL and
SQLite, behind one `Engine` abstraction, without changing the Postgres output.
`migrate` and `query` already run on whatever golang-migrate/usql drivers are
wired; this milestone is specifically about **codegen** parity.

## Why it is not a small change

The whole codegen pipeline is bound to `pganalyze/pg_query_go` — PostgreSQL's
own C parser. It cannot parse MySQL or SQLite DDL/queries. `internal/codegen`
references `pgquery.*` throughout `catalog.go` and `query.go` (statement walking,
`RangeVar`, `ColumnRef`, `ResTarget`, `FuncCall`, type-name extraction). Adding
engines therefore requires (a) new parser dependencies and (b) extracting an
engine boundary so the Postgres path becomes one implementation of it.

## Proposed `Engine` interface

```go
// internal/codegen/engine/engine.go
type Engine interface {
    Name() string                                   // "postgresql" | "mysql" | "sqlite"
    BuildCatalog(migrations []string) (*Catalog, error)  // DDL -> schema
    AnalyzeQueries(cat *Catalog, sql string) ([]Query, error) // annotated SQL -> typed
    GoType(col Column) GoType                        // engine-specific type map
}
```

`Catalog`, `Column`, `Query`, `GoType`, and the renderers (`render.go`,
`render_query.go`) stay engine-agnostic and are reused as-is. The Postgres code
in `catalog.go`/`query.go`/`gotype.go` moves behind a `postgres` engine that
satisfies the interface. `Generate` selects the engine from `sql[].engine` in
`sequa.yaml` (already parsed; today it errors for non-`postgresql`).

## Parser choices

| Engine | DDL/query parser | Notes |
|--------|------------------|-------|
| MySQL  | `vitess.io/vitess/go/vt/sqlparser` (or the lighter `github.com/blastrain/vitess-sqlparser`) | Mature, pure-Go, parses CREATE TABLE + DML. Verify license (Apache-2.0) before vendoring. |
| SQLite | `modernc.org/sqlite` ships a parser; alternatively a small hand-rolled CREATE TABLE parser (SQLite DDL is simple) | SQLite typing is affinity-based, not strict — the type map is coarser. |

Prefer a pure-Go parser to avoid a second cgo dependency alongside pg_query_go.

## Type maps (new, per engine)

- **MySQL:** `bigint`→`int64`, `int`→`int32`, `tinyint(1)`→`bool`, `varchar`/`text`→`string`,
  `datetime`/`timestamp`→`time.Time`, `decimal`→`string`, `blob`→`[]byte`, `json`→`[]byte`.
  Nullable → `sql.Null*` (same pattern as the Postgres map).
- **SQLite:** affinity-based — `INTEGER`→`int64`, `REAL`→`float64`, `TEXT`→`string`,
  `BLOB`→`[]byte`, `NUMERIC`→`string`. Nullability from `NOT NULL`; arrays N/A.

## Phasing

1. **Extract the engine boundary** (no behavior change): move the Postgres impl
   behind `Engine`, add the interface, route `Generate` through it. Golden tests
   must stay byte-identical — this is the safety net.
2. **MySQL codegen:** add the MySQL engine (parser + type map) + golden fixtures
   + a gated integration test (MySQL service container in CI).
3. **SQLite codegen:** add the SQLite engine + golden fixtures + integration
   test (embedded SQLite, no service needed).

Each phase is its own PR. Phase 1 is the risky refactor; 2 and 3 are additive.

## Testing

- Reuse the golden-file harness (`golden_test.go`) with per-engine fixtures under
  `testdata/golden/<engine>/`.
- Extend the CI `integration` job (or add jobs) with a MySQL 8 service; SQLite
  runs in-process. Verify (`sequa verify`) introspection is Postgres-specific
  today (`pg_catalog`) — MySQL/SQLite introspection is a follow-on, not required
  for codegen parity.

## Non-goals for M5

- Cross-engine query portability (a query is written for one engine).
- MySQL/SQLite drift `verify` (separate, engine-specific introspection).
- Advanced type coverage (spatial, enum, generated columns) — start with the
  common column types above.

## Effort

Large. Phase 1 (extraction) ~0.5 day incl. keeping goldens green; Phase 2
(MySQL) ~1 day; Phase 3 (SQLite) ~0.5 day. Gated behind real MySQL in CI.
