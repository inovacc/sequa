# Roadmap
<!-- rev:002 -->

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
| **Now** (shipped) | M1 Spine + migrate, M2 query, M3 generate (Postgres) | ✅ done |
| **Next** | M4 `--verify` (ephemeral-DB replay + introspect + drift diff) | ⬜ planned |
| **Later** | M5 engines 2 & 3 (MySQL + SQLite codegen) | ⬜ planned |

## Milestones

| Milestone | Goal | Status |
|-----------|------|--------|
| M1 — Spine + migrate | Shared config/db spine plus a usable migration tool | ✅ shipped |
| M2 — query | Ad-hoc SQL client + REPL | ✅ shipped |
| M3 — generate (Postgres) | Type-safe Go codegen from Postgres migrations | ✅ shipped |
| M4 — `--verify` | Live-schema verification against static parse | ⬜ planned (next) |
| M5 — engines 2 & 3 | MySQL + SQLite codegen behind `Engine` | ⬜ planned |

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

### M4 — `--verify` ⬜

**Goal:** guarantee the statically parsed catalog matches the schema the
migrations actually produce; doubles as a migration smoke test.

**Planned to deliver:**
- Ephemeral database spun up on demand.
- Replay of the migration up chain.
- Introspection of the real catalog.
- Drift diff between the introspected catalog and the static parse; fail on
  drift.

**Status:** planned — **not yet built.**

### M5 — engines 2 & 3 ⬜

**Goal:** extend codegen beyond Postgres.

**Planned to deliver:**
- MySQL codegen behind the `Engine` interface.
- SQLite codegen behind the `Engine` interface.

**Status:** planned — **not yet built.**

## Related docs

- [docs/BACKLOG.md](./BACKLOG.md) — future work and tech debt.
- [docs/ISSUES.md](./ISSUES.md) — known bugs.
