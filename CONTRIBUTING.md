# Contributing to sequa
<!-- rev:001 -->

Thanks for your interest in sequa. This guide covers the workflow, standards,
and gates for contributions.

## Development setup

Requires Go **1.26.4+** and a C toolchain (cgo — `generate` depends on
`pg_query_go`/`libpg_query`). [Task](https://taskfile.dev) drives the common
commands:

```
task build        # go build ./...
task test         # go test -short ./...   (no database needed)
task lint         # golangci-lint run ./...
task test:full    # full suite; needs SEQUA_TEST_DATABASE_URL
task test:docker  # full suite incl. integration, against a real Postgres in Docker
```

Tests that need a database `t.Skip()` under `-short` and when
`SEQUA_TEST_DATABASE_URL` is unset, so `task test` is safe offline.

## Making a change

1. Branch from `main` using a conventional prefix: `feat/…`, `fix/…`,
   `refactor/…`, `test/…`, `docs/…`, `chore/…`, `build/…`, `ci/…`.
2. Keep commits **atomic** and use
   [Conventional Commits](https://www.conventionalcommits.org)
   (`feat: …`, `fix: …`). Do not add AI attribution or `Co-Authored-By` trailers.
3. Add or update tests. If you change codegen output, regenerate the golden
   files: `go test ./internal/codegen -run TestGolden -update`.
4. Run `task build && task lint && task test` before pushing.
5. Open a PR against `main`. CI (build · lint · govulncheck · Postgres
   integration) must be green — these are **required checks**.

## Code standards

- Idiomatic Go per the Uber/Google/Effective-Go guidance the linter enforces.
- Cognitive complexity budget: **≤ 15 per function** (`gocognit`).
- Errors: wrap with `%w`, match with `errors.Is`/`errors.As`, lowercase and
  unpunctuated messages.
- `context.Context` is the first parameter; never stored on a struct.
- Every exported identifier has a doc comment beginning with its name.
- Line endings are LF (enforced by `.gitattributes`); files are `gofmt`- and
  `goimports`-clean (local prefix `github.com/inovacc/sequa`).

## Reporting bugs & requesting features

Use the issue templates. For security-sensitive reports, please avoid filing a
public issue with exploit details — open a minimal report and request a private
channel.

## License

By contributing you agree your contributions are licensed under the project's
**BSD 3-Clause** license (see [LICENSE](LICENSE)).
