# Known Issues & Limitations
<!-- rev:002 -->

This document tracks known bugs and by-design limitations in sequa. Each
entry has a short id, a description, its impact, and a status or workaround.

Entries are classified as either:

- **bug** — behavior that is unintended and may be fixed.
- **limitation (by design)** — a deliberate constraint or non-goal, not a defect.

## ISS-1 — schema-history insert is outside the migration transaction

- **Type:** bug
- **Description:** When a migration step is applied, the schema-history row is
  inserted separately from the step's own transaction rather than as part of
  it. If the process crashes in the window between the step commit and the
  history insert, the step's changes are durable but its history row is never
  written, leaving one migration applied but unrecorded.
- **Impact:** A single history row can be dropped on an ill-timed crash. The
  schema change itself is not lost or corrupted — only its bookkeeping row.
- **Status / workaround:** Open. `migrate status` tolerates missing history
  rows, so a dropped row does not block normal operation or reporting. A
  future fix would fold the history insert into the step transaction so both
  commit atomically.

## ISS-2 — `generate` supports only simple single-table queries

- **Type:** limitation (by design, current scope)
- **Description:** `sequa generate` supports only single-statement,
  single-table queries whose result list is plain columns or `*`. JOINs,
  aggregates, and computed expressions in the result list are not yet
  supported.
- **Impact:** Queries that span multiple tables or project derived values
  cannot be turned into typed methods by codegen today.
- **Status / workaround:** Planned expansion (later milestone). For now, keep
  annotated queries to a single table with plain-column or `*` result lists;
  write more complex access by hand.

## ISS-3 — the current build is Postgres-only (all verbs)

- **Type:** limitation (by design, current scope)
- **Description:** All three verbs are Postgres-only today. `migrate` wires only
  golang-migrate's Postgres driver, `query` registers only usql's Postgres
  driver, and `generate` uses PostgreSQL's own parser (`pg_query_go`). The
  foundations (golang-migrate, usql, and a planned `Engine` abstraction) are
  designed so MySQL/SQLite can be added later — that is the M5 design intent,
  not shipped behavior.
- **Impact:** `migrate`, `query`, and `generate` accept only `postgres://` /
  `postgresql://` DSNs; other engines are not usable yet.
- **Status / workaround:** Open. Multi-engine support is planned for M5 (see
  [docs/ROADMAP.md](./ROADMAP.md)). For now, run every verb against Postgres.

## ISS-4 — SQL-only migrations (no Go migrations)

- **Type:** limitation (by design — deliberate non-goal)
- **Description:** Migrations are authored as SQL only. There is no support
  for `.go` migrations, and adding them is an explicit non-goal.
- **Impact:** Migration logic that would require imperative Go code cannot be
  expressed as a migration.
- **Status / workaround:** Not a bug — intentional design. Express migrations
  in SQL. This entry is listed as a limitation for transparency, not as
  planned work.

## ISS-5 — host Application Control may block freshly-built binaries

- **Type:** limitation (host/environment)
- **Description:** On machines with Application Control policies that block
  running freshly-built binaries, executing the just-compiled `sequa` binary
  may be prevented by the host.
- **Impact:** Local run-based verification of a fresh build can fail on such
  hosts even when the build itself succeeds.
- **Status / workaround:** Verify with `go build` plus `go test -short`, and
  run the full suite via Docker (`task test:docker`).
