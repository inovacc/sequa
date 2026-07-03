# Backlog
<!-- rev:006 -->

Prioritized future work and tech debt for **sequa**. Items that map to a planned
milestone are marked `→ M5` (tracked in [docs/ROADMAP.md](./ROADMAP.md)). Known
bugs and by-design limits live in [docs/ISSUES.md](./ISSUES.md).

Priority: **P1** = blocks real-world use or a core quality gate · **P2** =
release readiness / next milestone · **P3** = later.

## Recently shipped

- ✅ **CI hardening** (maturity Phase 1) — gitleaks secret scan, `-race` on unit + integration, coverage-floor gate, pinned govulncheck, aligned action versions.
- ✅ **Graceful shutdown** — SIGINT/SIGTERM cancels the command context.
- ✅ **CLI test coverage** — database-free command tests; coverage floor ratcheted to 67%.
- ✅ **Maturity assessment** — `docs/analysis/MATURITY.md` (Stage 4, 90.7) + leverage-ranked route.
- ✅ **Inner JOINs in `generate`** — multi-table INNER JOIN queries with an explicit column list; qualified/unqualified resolution, cross-table param binding, aggregates over joined columns. Outer joins deferred (ISS-2).
- ✅ **Coverage reporting** — `task test:cover` + a CI coverage summary and artifact.
- ✅ **`verify` + `--ephemeral`** — live drift check; ephemeral spins up a throwaway DB, applies migrations, verifies, drops it.
- ✅ **`generate` aggregates** — `count`/`min`/`max`/`sum`/`avg` result columns, typed per Postgres promotion.
- ✅ **M5 phase 1** — `Engine` interface + `postgresEngine`; `generate` routes through the seam (Postgres output unchanged).
- ✅ **Release automation** — `.goreleaser.yaml` + release workflow (linux/windows via zig), `sequa --version`.
- ✅ **CI as a required status check on `main`**; **Dependabot** (gomod + actions).
- ✅ **Contributor scaffolding** — `CONTRIBUTING.md`, issue/PR templates, `CHANGELOG.md`.
- ✅ **Complexity ratchet** — codegen functions ≤ 15; gocognit/gocyclo gate lowered to 15.
- ✅ **Golden-file codegen tests**, **schema-history self-heal** (ISS-1), **Go 1.26.4** stdlib CVE patch, **verify bookkeeping-table** fix.

## P1 — blocks real-world use / core quality gates

- **Broaden `generate`: outer joins + computed expressions** — inner JOINs shipped; `LEFT`/`RIGHT`/`FULL` (outer-join nullability), `SELECT *` across joins, `ON`-clause params, and arbitrary result expressions remain (ISS-2; [specs/generate-joins.md](./specs/generate-joins.md)).

## P2 — release readiness / next milestone

- **macOS release builds** — the release workflow ships linux + windows; darwin needs a native runner or the macOS SDK for cgo (`.goreleaser.yaml` note).
- **M5 phases 2–3: MySQL & SQLite codegen** `→ M5` — implement engines behind the phase-1 seam; design in [specs/M5-engines.md](./specs/M5-engines.md).

## P3 — later

- **`:copyfrom` / `:batch` query verbs** — bulk-insert / batched access; impedance mismatch with `lib/pq`/`database/sql` (batch needs pgx). Design in [specs/generate-batch-verbs.md](./specs/generate-batch-verbs.md).
- **Docs site** — publish `docs/` as a browsable site rather than raw Markdown.
