# Backlog
<!-- rev:004 -->

Prioritized future work and tech debt for **sequa**. Items that map to a planned
milestone are marked `→ M5` (tracked in [docs/ROADMAP.md](./ROADMAP.md)). Known
bugs and by-design limits live in [docs/ISSUES.md](./ISSUES.md).

Priority: **P1** = blocks real-world use or a core quality gate · **P2** =
release readiness / next milestone · **P3** = later.

## Recently shipped

- ✅ **`verify` + `--ephemeral`** — live drift check; ephemeral spins up a throwaway DB, applies migrations, verifies, drops it.
- ✅ **`generate` aggregates** — `count`/`min`/`max`/`sum`/`avg` result columns, typed per Postgres promotion.
- ✅ **M5 phase 1** — `Engine` interface + `postgresEngine`; `generate` routes through the seam (Postgres output unchanged).
- ✅ **Release automation** — `.goreleaser.yaml` + release workflow (linux/windows via zig), `sequa --version`.
- ✅ **CI as a required status check on `main`**; **Dependabot** (gomod + actions).
- ✅ **Contributor scaffolding** — `CONTRIBUTING.md`, issue/PR templates, `CHANGELOG.md`.
- ✅ **Complexity ratchet** — codegen functions ≤ 15; gocognit/gocyclo gate lowered to 15.
- ✅ **Golden-file codegen tests**, **schema-history self-heal** (ISS-1), **Go 1.26.4** stdlib CVE patch, **verify bookkeeping-table** fix.

## P1 — blocks real-world use / core quality gates

- **Broaden `generate`: multi-table JOINs** — the last major single-table limit (ISS-2). Correctness-sensitive (outer-join nullability); design in [specs/generate-joins.md](./specs/generate-joins.md). Arbitrary computed expressions also remain.

## P2 — release readiness / next milestone

- **macOS release builds** — the release workflow ships linux + windows; darwin needs a native runner or the macOS SDK for cgo (`.goreleaser.yaml` note).
- **M5 phases 2–3: MySQL & SQLite codegen** `→ M5` — implement engines behind the phase-1 seam; design in [specs/M5-engines.md](./specs/M5-engines.md).

## P3 — later

- **`:copyfrom` / `:batch` query verbs** — bulk-insert / batched access; impedance mismatch with `lib/pq`/`database/sql` (batch needs pgx). Design in [specs/generate-batch-verbs.md](./specs/generate-batch-verbs.md).
- **Docs site** — publish `docs/` as a browsable site rather than raw Markdown.
