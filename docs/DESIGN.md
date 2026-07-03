# Sequa — Design Spec

> One Go tool to **migrate, query, and generate** type-safe Go from a single SQL schema, where your **migrations are the source of truth**.

- **Status:** the original design reference. Implemented through v0.1.0 (M1–M4);
  see [ROADMAP.md](ROADMAP.md) for current milestone state and any deviations.
- **Binary:** `sequa`
- **Module path:** `github.com/inovacc/sequa`

## 1. Thesis & Scope

Sequa unifies three tools developers juggle daily on Go + SQL projects into one binary + one embeddable library:

| Verb | Lineage | Role |
|---|---|---|
| `migrate` | `golang-migrate` engine + goose-style UX | versioned up/down SQL migrations; embeddable so the app self-migrates |
| `generate` | first-party codegen **adapted from sqlc's** Postgres path | introspect the *migration-defined* schema → idiomatic, type-safe Go |
| `query` | **embed `usql`** core | ad-hoc SQL client + REPL across many engines |

**Defining property (the "de facto" claim):** the schema is defined exactly **once** — as the migration set. `generate` derives its catalog from those same migrations, so generated Go can never drift from the applied schema.

## 2. Goals / Non-Goals (YAGNI)

**Goals (v1):**
- Postgres-first `generate`; `migrate` + `query` work on all engines golang-migrate/usql support from day one.
- Migrations dictate codegen (static parse by default; `--verify` live path optional).
- Autodetect the migrations dir; only prompt for DSN.
- Embeddable library for app self-migration.

**Non-goals (v1):** MySQL/SQLite *codegen* (added later behind the same `Engine` interface); `.go` migrations (SQL-only); a schema-diff/declarative mode (migrations stay imperative).

## 3. Resolved Decisions

1. **Integration:** deep — migrations are the single source of truth.
2. **Schema derivation for codegen:** static parse of `*.up.sql` by default **+** optional `--verify` (ephemeral DB → replay up chain → introspect real catalog → diff → fail on drift; doubles as a migration smoke test).
3. **Generate engines:** PostgreSQL first; MySQL + SQLite later (same interface).
4. **Migration engine:** `golang-migrate` + a goose-style UX layer (`create / up / down / status`, `-s` sequential vs timestamp).
5. **Migration format:** SQL-only (no `.go` migrations).
6. **Codegen build:** copy/fork sqlc's Postgres parse→catalog→codegen path into our tree and adapt (we own it). `pg_query_go` (sqlc's parser dep) is imported, not copied.
7. **Query engine:** embed usql's core (`handler` + `drivers`) — full REPL, meta-commands, formats, drivers (Q1).
8. **Name:** Sequa.

## 4. Architecture

```
cmd/sequa/                 Cobra entrypoint
internal/
  config/                  autodetect migrations dir + DSN ($DATABASE_URL / flag / config)  ← shared spine
  db/                      connection layer: dburl parse, *sql.DB pool, driver registry      ← shared spine
  migrate/                 golang-migrate engine + goose-style UX (create/up/down/status, seq|ts)
  query/                   usql handler embed (REPL + one-shot exec)
  codegen/
    engine/                Engine interface + postgres impl (parser, type map)  [copied/adapted from sqlc]
    catalog/               shared schema model (tables/columns/types/nullability)
    resolve/               query result+param typing vs catalog
    render/                shared Go templates (embed.FS)
    schema/                static up-migration parse  +  live --verify introspector
pkg/sequa/                 public library API: New(db, migrationsFS).Up/Down/Version  ← app self-migration
third_party/               upstream LICENSE files for any copied code (sqlc, …)
templates/                 codegen Go templates (embed.FS)
```

`internal/config` + `internal/db` are the shared spine used by every verb — what makes Sequa *one* tool, not three bundled binaries.

## 5. Headline Data Flow — migrations dictate codegen

1. `config` autodetects the migrations dir (`migrations/`, `db/migrations/`, `sql/migrations/`…) and resolves the DSN.
2. `codegen/schema` reads `*.up.sql` in version order → `engine`(postgres) parses DDL → builds `catalog`.
3. `resolve` type-checks each annotated query against the catalog.
4. `render` emits idiomatic Go (models + query methods).
5. `--verify` (optional): `db` spins an ephemeral Postgres → `migrate` replays the up chain → introspect the *real* catalog → diff vs the static one → fail on drift.

**Query annotations:** adopt sqlc's `-- name: GetUser :one | :many | :exec` convention verbatim, so existing sqlc query files run unchanged (zero-friction path for sqlc users).

## 6. CLI Surface

```
sequa init                          scaffold config + migrations/
sequa migrate create <name> [-s]    new timestamped (or -s sequential) .up/.down.sql pair
sequa migrate up | down | status | version
sequa generate [--verify]
sequa query [DSN] [-c "SQL"]        REPL, or one-shot with -c
```

**Library:** `sequa.New(db, migrationsFS).Up(ctx)` — app embeds its `migrations/` via `embed.FS` and self-migrates on startup.

## 7. Build Strategy — copy vs import (+ licensing)

| Upstream | License | Action | Why |
|---|---|---|---|
| **sqlc** | permissive (confirm at clone) | **copy/fork** PG codegen path → `internal/codegen/` | not importable (lives in `internal/`); copying is the only way to reuse it |
| **golang-migrate** | permissive (confirm) | **import** `migrate/v4` | clean library; forking it = losing every upstream fix |
| **usql** | permissive (confirm) | **import** core | importable REPL; copy only later if trimming drivers |
| **pg_query_go** | — | **import** | external module sqlc itself imports |

**Attribution (non-negotiable):** every copied file keeps its upstream copyright header; each source's `LICENSE` is placed under `third_party/<name>/`; provenance recorded in `NOTICE`. Project license = BSD-3 (compatible with the permissive upstreams). Exact license terms verified at clone time **before** copying a byte.

**Sequencing:** (1) clone all three into a gitignored `reference/`; (2) confirm licenses; (3) import migrate + usql; (4) copy sqlc's PG codegen path into `internal/codegen/`, rewrite package paths, wire `codegen/schema` to read the golang-migrate migrations dir.

## 8. Error Handling, Logging, Testing (per Go standards)

- Errors: wrap with `%w`; compare via `errors.Is`/`errors.As`.
- Logging: `log/slog`, structured.
- Tests: table-driven; **golden-file** tests for codegen output (per engine); ephemeral-DB integration tests gated behind `testing.Short()` / build tags (default `test` task runs `-short`; `test:full` runs everything).
- Hexagonal layout, Cobra CLI, `go run` for execution.

## 9. Milestones (Postgres-first, de-risked)

- **M1 — Spine + migrate:** `config` autodetect + `db` + `migrate` (golang-migrate + goose UX, SQL-only) + library API. Usable migration tool.
- **M2 — query:** embed usql (REPL + one-shot).
- **M3 — generate (Postgres):** copy/adapt sqlc PG path; static schema source; golden tests.
- **M4 — `--verify`:** ephemeral-DB replay + introspect + drift diff.
- **M5 — engines 2 & 3:** MySQL + SQLite codegen behind `Engine`.

## 10. Open Inputs

- **Module owner** for `github.com/<owner>/sequa` (the only input blocking scaffold).
