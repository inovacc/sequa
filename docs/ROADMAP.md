# Roadmap
<!-- rev:005 -->

Milestone roadmap for **sequa** — one Go tool to migrate, query, and generate
type-safe Go from a single SQL schema.

## Design principle

- **Migrations are the source of truth.** The schema is defined exactly once, as
  the migration set. `generate` derives its catalog from those same migrations,
  so generated Go cannot drift from the applied schema.
- **Postgres-first, de-risked.** Codegen ships for PostgreSQL first; other
  engines are added later behind a common `Engine` interface. `migrate` and
  `query` build on golang-migrate and usql — whose designs span many engines —
  but only the Postgres driver is wired up today.

## Now / Next / Later

| Horizon | Milestone | State |
|---------|-----------|-------|
| **Now** (shipped) | M1–M3, M4 `verify` (introspect + drift diff, incl. `--ephemeral`), M5 phase 1 (Engine boundary) | ✅ done |
| **Next** | M5 phases 2–3: MySQL + SQLite codegen behind the Engine seam — see [specs/M5-engines.md](./specs/M5-engines.md) | ⬜ planned |
| **Later** | Broaden `generate` (inner JOINs shipped; outer joins, `:copyfrom`/`:batch` remain) — see [specs/generate-joins.md](./specs/generate-joins.md) & [specs/generate-batch-verbs.md](./specs/generate-batch-verbs.md) | ⬜ backlog |

## Milestones

| Milestone | Goal | Status |
|-----------|------|--------|
| M1 — Spine + migrate | Shared config/db spine plus a usable migration tool | ✅ shipped |
| M2 — query | Ad-hoc SQL client + REPL | ✅ shipped |
| M3 — generate (Postgres) | Type-safe Go codegen from Postgres migrations | ✅ shipped |
| M4 — `verify` | Live-schema verification against static parse | ✅ shipped (core) |
| M5 — engines 2 & 3 | MySQL + SQLite codegen behind `Engine` | ⬜ planned ([spec](./specs/M5-engines.md)) |

### M1 — Spine + migrate ✅

**Goal:** stand up the shared spine every verb depends on, and deliver a usable
migration tool on top of it.

**Delivers:**
- `internal/config` migrations-dir autodetect + DSN resolution.
- `internal/db` connection layer.
- `migrate` on the golang-migrate engine with a goose-style UX
  (`create` / `up` / `down` / `status`, sequential vs timestamp).
- SQL-only migrations.
- Embeddable library API for app self-migration.

**Status:** shipped.

### M2 — query ✅

**Goal:** ad-hoc SQL access without leaving the tool.

**Delivers:**
- `query` verb embedding the usql core.
- One-shot execution (renders result tables) and an interactive REPL.

**Status:** shipped.

### M3 — generate (Postgres) ✅

**Goal:** derive idiomatic, type-safe Go from the migration-defined schema.

**Delivers:**
- Models codegen from migrations, and typed query methods from annotated SQL.
- Postgres schema parsing via `pg_query_go` AST.
- `sequa.yaml`-driven configuration.

**Status:** shipped.

### M4 — `verify` ✅ (core)

**Goal:** guarantee the statically parsed catalog matches the schema the
migrations actually produce; doubles as a migration smoke test.

**Delivers:**
- `sequa verify`: parse the up-migrations into a catalog, introspect the live
  database via `pg_catalog`, and diff them.
- Drift reporting for missing/extra tables and columns, type mismatches, and
  nullability mismatches; non-zero exit on any drift.

**Status:** shipped. `verify` runs against the DSN you point it at. **Follow-up:**
optional ephemeral-DB auto-replay (spin up a throwaway database, apply the up
chain, verify) — tracked in the backlog.

### M5 — engines 2 & 3 ⬜ (phase 1 shipped)

**Goal:** extend codegen beyond Postgres.

**Delivers:**
- ✅ Phase 1: an `Engine` interface + `postgresEngine`; `generate` routes through
  `engineFor(engine)`. This is the seam other engines plug into; Postgres output
  is unchanged.
- ⬜ Phase 2: MySQL codegen behind the interface.
- ⬜ Phase 3: SQLite codegen behind the interface.

**Status:** phase 1 (engine boundary) shipped; MySQL/SQLite planned — design in
[specs/M5-engines.md](./specs/M5-engines.md).

## Related docs

- [docs/BACKLOG.md](./BACKLOG.md) — future work and tech debt.
- [docs/ISSUES.md](./ISSUES.md) — known bugs.
