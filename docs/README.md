# sequa documentation
<!-- rev:001 -->

Documentation index. For install and usage, start with the project
[README](../README.md).

## Reference

- [DESIGN.md](DESIGN.md) — architecture and design rationale.
- [GENERATE_AND_QUERY.md](GENERATE_AND_QUERY.md) — guide to `sequa generate` and `sequa query`.

## Planning & status

- [ROADMAP.md](ROADMAP.md) — milestones (M1–M5) and progress.
- [BACKLOG.md](BACKLOG.md) — prioritized future work.
- [ISSUES.md](ISSUES.md) — known bugs and by-design limitations.

## Design specs (planned features)

- [specs/adopt.md](specs/adopt.md) — `sequa adopt`: deterministic detection & conversion of existing DB tooling (sqlc / golang-migrate / goose / dbmate / atlas …) into sequa.
- [specs/generate-joins.md](specs/generate-joins.md) — outer-join support (inner joins already shipped).
- [specs/generate-batch-verbs.md](specs/generate-batch-verbs.md) — `:copyfrom` / `:batch` query verbs.
- [specs/M5-engines.md](specs/M5-engines.md) — MySQL / SQLite codegen behind the `Engine` seam.

## Project files (repo root)

- [README](../README.md) — features, install, usage.
- [CONTRIBUTING](../CONTRIBUTING.md) — how to contribute.
- [CHANGELOG](../CHANGELOG.md) — release history.
- [AGENTS.md](../AGENTS.md) — agent/contributor quick reference.
