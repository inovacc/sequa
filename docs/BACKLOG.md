# Backlog
<!-- rev:003 -->

Prioritized future work and tech debt for **sequa**. Items here are not owned by
a milestone number; where an item maps to a planned milestone it is marked
`→ M5` (tracked in [docs/ROADMAP.md](./ROADMAP.md)) and listed here for
visibility. Known bugs and by-design limits live in
[docs/ISSUES.md](./ISSUES.md).

Priority: **P1** = blocks real-world use or a core quality gate · **P2** =
release readiness / next milestone · **P3** = later.

## Recently shipped (2026-07-02)

- ✅ **Golden-file codegen tests** — generated Go pinned per input fixture so codegen cannot regress silently.
- ✅ **Schema-history self-heal** — a history row lost to an ill-timed crash is reconciled on the next `up`/`down` (ISS-1 mitigated).
- ✅ **CI workflow** — build + vet + `test -short`, golangci-lint v2, govulncheck, and a real-Postgres integration job on push/PR.
- ✅ **`count`/`min`/`max` aggregates in `generate`** — first slice of "broaden generate".
- ✅ **`sequa verify`** (M4 core) — introspect the live schema via `pg_catalog` and diff it against the migrations.
- ✅ **Go 1.26.4 bump** — patches the called stdlib CVEs govulncheck flagged.

## P1 — blocks real-world use / core quality gates

- **Broaden `generate` further: JOINs, `sum`/`avg`, computed expressions, table aliases, multi-statement queries** — `count`/`min`/`max` landed; the rest of ISS-2 remains, so multi-table and most derived-value queries still cannot be turned into typed methods.

## P2 — release readiness / next milestone

- **Wire CI as a required status check on `main`** — the workflow exists and is green; make it a required check under branch protection so a red PR cannot merge.
- **`.goreleaser` config + release workflow** — no release automation exists; needed to build and publish versioned, multi-platform `sequa` binaries.
- **Ephemeral-DB auto-replay for `verify`** `→ M4 follow-up` — spin up a throwaway database, replay the up chain, and verify, so `verify` needs no pre-migrated target.

## P3 — later

- **MySQL & SQLite codegen behind an `Engine` interface** `→ M5` — extend `generate` beyond Postgres; design in [specs/M5-engines.md](./specs/M5-engines.md). Today all three verbs are Postgres-only (see ISS-3). Not yet built.
- **More query annotations (`:copyfrom`, `:batch`)** — extend the supported result verbs beyond `:one` / `:many` / `:exec` for bulk-insert and batched access.
- **Docs site** — publish `docs/` as a browsable site rather than raw Markdown in the repo.
