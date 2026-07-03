# AGENTS.md
<!-- rev:001 -->

Canonical instructions for AI agents and contributors working on **sequa**.
Human-facing contribution workflow lives in [CONTRIBUTING.md](CONTRIBUTING.md);
detailed docs live in [docs/](docs/README.md).

## What sequa is

One Go tool for PostgreSQL: SQL migrations (`migrate`), an interactive SQL
client/REPL (`query`), type-safe Go codegen driven by the migrations
(`generate`), and schema-drift verification (`verify`) — plus an embeddable
migration library (`pkg/sequa`). Migrations are the single source of truth.

## Layout

- `cmd/sequa/` — CLI entrypoint.
- `internal/cli/` — Cobra commands.
- `internal/config`, `internal/db` — shared spine (dir/DSN resolution, connection layer).
- `internal/migrate` — golang-migrate engine + goose-style UX.
- `internal/query` — embedded usql client/REPL.
- `internal/codegen` — schema catalog + Go codegen (pg_query_go); the `Engine` seam lives here.
- `pkg/sequa` — public embeddable API.

## Build, test, lint

Prefer [Task](https://taskfile.dev):

```
task build        # go build ./...
task test         # go test -short ./...   (no database needed)
task lint         # golangci-lint run ./...
task test:cover   # short tests + total coverage
task test:full    # full suite; needs SEQUA_TEST_DATABASE_URL
task test:docker  # full suite incl. integration against a real Postgres in Docker
```

Tests that need a database `t.Skip()` under `-short` and when
`SEQUA_TEST_DATABASE_URL` is unset, so `task test` is safe offline. When codegen
output changes, regenerate goldens: `go test ./internal/codegen -run TestGolden -update`.

## Conventions

- Go **1.26.4+**; a C toolchain is required (cgo — `generate` uses `libpg_query`).
- Idiomatic Go: wrap errors with `%w` and match with `errors.Is`/`errors.As`;
  lowercase, unpunctuated error strings; `context.Context` first, never on a struct;
  doc comment on every exported identifier; cognitive complexity **≤ 15** per function.
- Files are LF (`.gitattributes`) and `gofmt`/`goimports`-clean (local prefix
  `github.com/inovacc/sequa`). The `golangci-lint` config is the source of truth.
- Migrations are **SQL-only**, two files per migration (`<version>_<name>.up.sql`
  + `.down.sql`). Query annotations are sqlc-style (`-- name: X :one|:many|:exec`).
- Security: parameterized queries only; never commit secrets; keep DSNs out of
  logs and errors.

## Git & PRs

- Branch from `main` with a conventional prefix (`feat/`, `fix/`, `refactor/`,
  `test/`, `docs/`, `chore/`, `build/`, `ci/`).
- [Conventional Commits](https://www.conventionalcommits.org); **no AI attribution**.
- `main` is protected: land work via a PR with green CI (build · lint ·
  govulncheck · Postgres integration are required checks).

## License

BSD 3-Clause — see [LICENSE](LICENSE).
