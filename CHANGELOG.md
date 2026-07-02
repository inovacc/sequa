# Changelog

All notable changes to sequa are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the project aims
to follow [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- `sequa verify` (M4): introspect the live database via `pg_catalog` and diff it
  against the migration-defined schema; reports missing/extra tables and columns,
  type mismatches, and nullability mismatches, and exits non-zero on drift.
- `generate`: `count`/`min`/`max` aggregate result columns (`count` → non-null
  `int64`; `min`/`max` → the argument column's type, nullable).
- Golden-file tests pinning the full generated `models.go`/`queries.go`.
- CI workflow: build + vet + `test -short`, golangci-lint v2, govulncheck, and a
  real-Postgres integration job; required status checks on `main`.
- Contributor scaffolding: `CONTRIBUTING.md`, issue/PR templates, Dependabot.
- Project docs: `README`, `docs/ROADMAP.md`, `docs/BACKLOG.md`,
  `docs/ISSUES.md`, and the M5 engine design spec.

### Changed
- Go toolchain bumped to 1.26.4, patching called standard-library
  vulnerabilities (GO-2026-4870, GO-2026-4866, GO-2026-4865).
- Consolidated duplicated codegen/CLI/migrate helpers (`migrate.ParseFilename`,
  `cli.newRunnerFromFlags`, `codegen.camelWords`/`buildGoStruct`/`renderFormatted`,
  `slices.DeleteFunc`).

### Fixed
- Codegen no longer treats a malformed migration filename as version 0 (which
  could silently reorder the schema catalog); it now propagates the parse error.
- Schema-history rows lost to an ill-timed crash are reconciled on the next
  `migrate up`/`down` (ISS-1 mitigated).

[Unreleased]: https://github.com/inovacc/sequa/commits/main
