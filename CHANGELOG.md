# Changelog

All notable changes to sequa are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the project aims
to follow [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Graceful shutdown: SIGINT/SIGTERM cancels the running command's context, so an
  interrupted `migrate up` unwinds cleanly instead of being killed mid-step.
- CI hardening: gitleaks secret scan, `-race` on the unit and real-Postgres
  integration jobs, and a coverage-floor gate.
- Maturity assessment at `docs/analysis/MATURITY.md` (Stage 4, weighted 90.7).

### Changed
- Pinned `govulncheck` (v1.5.0) and aligned GitHub Action versions across
  workflows for reproducibility.
- Raised CLI test coverage with database-free command tests; coverage floor
  ratcheted to 67%.

## [0.1.0] - 2026-07-03

First tagged release: a single Go tool for PostgreSQL migrations, an interactive
SQL client/REPL, and type-safe Go codegen driven by your migrations, plus schema
drift verification.

### Added
- `sequa verify` (M4): introspect the live database via `pg_catalog` and diff it
  against the migration-defined schema; reports missing/extra tables and columns,
  type mismatches, and nullability mismatches, and exits non-zero on drift.
- `generate`: `count`/`min`/`max`/`sum`/`avg` aggregate result columns, typed
  per Postgres promotion rules (`count` → non-null `int64`; `min`/`max` → the
  column's type, nullable; `sum`/`avg` → widened numeric, nullable).
- `generate`: multi-table **INNER JOIN** support — explicit column lists with
  qualified/unqualified resolution, cross-table param binding, and aggregates
  over joined columns (outer joins and `*`-across-joins deferred).
- Coverage reporting — `task test:cover` and a CI coverage summary/artifact.
- `sequa verify --ephemeral`: create a throwaway database, apply the migrations,
  and verify against it — a zero-setup drift check / migration smoke test.
- Release automation: `.goreleaser.yaml` + release workflow (linux/windows via
  zig cgo cross-compile) and `sequa --version`.
- Golden-file tests pinning the full generated `models.go`/`queries.go`.
- CI workflow: build + vet + `test -short`, golangci-lint v2, govulncheck, and a
  real-Postgres integration job; required status checks on `main`.
- Contributor scaffolding: `CONTRIBUTING.md`, issue/PR templates, Dependabot.
- Project docs: `README`, `docs/ROADMAP.md`, `docs/BACKLOG.md`,
  `docs/ISSUES.md`, and the M5 engine design spec.

### Changed
- Go toolchain bumped to 1.26.4, patching called standard-library
  vulnerabilities (GO-2026-4870, GO-2026-4866, GO-2026-4865).
- Extracted an `Engine` interface for codegen (M5 phase 1); `generate` routes
  through it. Postgres output is unchanged.
- Reduced codegen cognitive complexity and tightened the gocognit/gocyclo gate
  to 15.
- CI jobs are now required status checks on `main`.
- Consolidated duplicated codegen/CLI/migrate helpers (`migrate.ParseFilename`,
  `cli.newRunnerFromFlags`, `codegen.camelWords`/`buildGoStruct`/`renderFormatted`,
  `slices.DeleteFunc`).

### Fixed
- Codegen no longer treats a malformed migration filename as version 0 (which
  could silently reorder the schema catalog); it now propagates the parse error.
- Schema-history rows lost to an ill-timed crash are reconciled on the next
  `migrate up`/`down` (ISS-1 mitigated).
- `verify` no longer reports migrate's `schema_migrations` and sequa's
  `sequa_schema_history` bookkeeping tables as drift.

[Unreleased]: https://github.com/inovacc/sequa/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/inovacc/sequa/releases/tag/v0.1.0
