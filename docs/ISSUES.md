# Known Issues & Limitations
<!-- rev:007 -->

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
- **Status / workaround:** Mitigated (self-healing). The insert is still
  separate from migrate's transaction — golang-migrate does not expose a hook to
  join it — so the crash window still exists momentarily. But `up` and `down`
  now reconcile on start: because migrate is linear, every known version at or
  below its current version is applied, so any such version missing its history
  row is backfilled automatically (`reconcileHistory`). A dropped row is
  therefore restored on the next migration operation rather than lost
  permanently (`applied_at` reflects the reconcile time, since the original
  instant was never captured). `migrate status` continues to tolerate a
  transient gap.

## ISS-2 — `generate` JOIN support is limited to inner joins

- **Type:** limitation (by design, current scope)
- **Description:** `sequa generate` handles single-statement queries. Single-table
  result lists may be plain columns, `*`, or the `count`, `min`, `max`, `sum`,
  and `avg` aggregates (typed per Postgres's promotion rules — e.g. `count` →
  non-null `int64`, `sum` of an integer → nullable `int64`, `avg` → nullable
  numeric). Multi-table **INNER JOINs are now supported** with an explicit
  result-column list: qualified (`a.x`) or unqualified columns resolve across the
  joined relations (ambiguous bare names and duplicate result names error out),
  `WHERE` params bind across the tables, and aggregates may take a joined column.
  Still unsupported: outer joins (`LEFT`/`RIGHT`/`FULL`), `SELECT *` across a
  JOIN, parameters inside a JOIN `ON` clause (put filter params in `WHERE`), and
  arbitrary computed expressions in the result list.
- **Impact:** Inner-join queries with explicit columns now generate typed
  methods. Outer-join nullability, `*`-across-joins, and derived expressions
  cannot yet be turned into typed methods by codegen.
- **Status / workaround:** Partially addressed — `count`/`min`/`max`/`sum`/`avg`
  and INNER JOINs (explicit columns) landed; outer joins, `*`-across-joins, and
  arbitrary expressions remain planned (see
  [docs/specs/generate-joins.md](./specs/generate-joins.md)). For now, list JOIN
  result columns explicitly, use INNER JOIN, and write outer-join / derived-value
  access by hand.

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
