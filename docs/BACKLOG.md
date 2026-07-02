# Backlog
<!-- rev:002 -->

Prioritized future work and tech debt for **sequa**. Items here are not owned by
a milestone number; where an item maps to a planned milestone it is marked
`→ M4` / `→ M5` (tracked in [docs/ROADMAP.md](./ROADMAP.md)) and listed here for
visibility. Known bugs and by-design limits live in
[docs/ISSUES.md](./ISSUES.md).

Priority: **P1** = blocks real-world use or a core quality gate · **P2** =
release readiness / next milestone · **P3** = later.

## P1 — blocks real-world use / core quality gates

- **Broaden `generate`: JOINs, aggregates & computed expressions, table aliases, multi-statement queries** — codegen today handles only single-statement, single-table, plain-column/`*` result lists, so most real application queries cannot be turned into typed methods (see ISS-2).
- **Golden-file codegen tests** — pin generated Go per input so codegen output cannot regress silently; a prerequisite for safely broadening `generate`.
- **Fix schema-history transaction gap** — fold the history-row insert into the migration step's own transaction so both commit atomically; an ill-timed crash can currently leave a step applied but unrecorded (ISS-1).
- **Wire CI as a required status check on `main`** — no `.github/workflows` exists yet; the Docker suite (`task test:docker`) is the authoritative gate but nothing enforces it on PRs.

## P2 — release readiness / next milestone

- **`.goreleaser` config + release workflow** — no release automation exists; needed to build and publish versioned, multi-platform `sequa` binaries.
- **`--verify` (ephemeral-DB replay + introspect + drift diff)** `→ M4` — spin an ephemeral database, replay the up chain, introspect the real catalog, and fail on drift vs the static parse; doubles as a migration smoke test. Not yet built.

## P3 — later

- **MySQL & SQLite codegen behind the `Engine` interface** `→ M5` — extend `generate` beyond Postgres. Today all three verbs (`migrate`/`query`/`generate`) are Postgres-only; multi-engine support is the M5 design intent (see ISS-3). Not yet built.
- **More query annotations (`:copyfrom`, `:batch`)** — extend the supported result verbs beyond `:one` / `:many` / `:exec` for bulk-insert and batched access.
- **Docs site** — publish `docs/` as a browsable site rather than raw Markdown in the repo.
