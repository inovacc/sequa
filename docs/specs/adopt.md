# `sequa adopt` — deterministic detection & conversion of existing DB tooling

Status: proposed · Date: 2026-07-03

## Question

Given an arbitrary Go/SQL project that already uses *some* database approach
(sqlc, golang-migrate, goose, dbmate, atlas, Flyway, raw embedded SQL, …), can
we build a **deterministic** process that (a) detects the approach, (b) decides
whether the project is *suitable* to run on sequa, and (c) converts it — or
tells us precisely what a human must do?

**Answer: yes — as a graded pipeline.** Detection and planning are 100%
deterministic and read-only. *Applying* the conversion is deterministic for the
mechanical tier and explicitly hands off the rest. The trick that makes
suitability *exact* rather than heuristic is that **sequa is its own oracle**
(see §4).

## 1. What "convert into sequa" actually means

sequa's contract is narrow, which is what makes conversion tractable. A project
"runs on sequa" iff three artifacts match sequa's shape, behind one hard gate:

| # | Artifact | sequa's required form |
|---|----------|-----------------------|
| G | **Engine** | PostgreSQL only (hard gate for `generate`; `migrate` is golang-migrate-backed) |
| 1 | **Migrations** | two files per version: `<version>_<name>.up.sql` + `.down.sql`, SQL-only |
| 2 | **Queries** (optional) | `-- name: Name :one\|:many\|:exec` blocks, each **one** sequa-parseable statement |
| 3 | **Config** | a `sequa.yaml` (`engine`, `schema`, `queries?`, `gen.go.{package,out}`) |

Conversion = **normalize (1),(2),(3) to these forms**, refusing anything that
fails gate G. Everything below is "how deterministic is each normalization,
per source system."

## 2. Source-system → sequa conversion matrix

Determinism levels: **Full** = byte-mechanical, no semantics invented ·
**Partial** = mechanical but something is missing (usually `.down`) ·
**Oracle** = decided exactly by running sequa's parser (§4) · **None** = requires
a human or a live database.

| Source system | Fingerprint (marker) | migrate conversion | query conversion | config | Determinism | Default tier |
|---|---|---|---|---|---|---|
| **golang-migrate** (PG) | `*.up.sql`+`*.down.sql`; `golang-migrate` in `go.mod` | **identity** — already the target form | usually none | synth from dir + DSN | **Full** | Turnkey |
| **sqlc** (PG, pgx/pq) | `sqlc.yaml/.json` | schema dir copied; single `schema.sql` → `0001_init.up.sql` (+ empty down stub) | `:one/:many/:exec` 1:1; `:execrows/:copyfrom/:batch*` flagged; each stmt **oracle-checked** | `sqlc.yaml` → `sequa.yaml` (near 1:1) | **Full for shared subset + Oracle** | Turnkey / Assisted |
| **goose** (SQL) | `-- +goose Up/Down` in `.sql`; `pressly/goose` in `go.mod` | **split** single-file on the markers → up/down pair; renumber | n/a | synth | **Full** | Turnkey |
| **goose** (Go migrations) | `*.go` registering migrations | — | — | — | **None** | Manual |
| **dbmate** | `db/migrations`, `-- migrate:up/down` | **split** on markers → pair | n/a | synth | **Full** | Turnkey |
| **atlas** (versioned) | `atlas.sum`, versioned dir | up files copied; **`.down` absent** → generate stub / require `atlas … --dir` down | n/a | map | **Partial** | Assisted |
| **atlas** (declarative HCL) | `atlas.hcl`, `schema.hcl` | no versioned files → must `atlas migrate diff` first | — | — | **None w/o atlas** | Manual |
| **Flyway** | `V<n>__name.sql` (+ `U<n>__`) | rename `V__`→`<n>_<name>.up.sql`; `U__`→`.down` when present | n/a | synth | **Partial** | Assisted |
| **Prisma / Drizzle / TypeORM** | `schema.prisma`, `drizzle.config.*` | Prisma emits SQL under `migrations/` (up-only) — extractable but JS-ecosystem | — | — | **Cross-ecosystem** | Manual / OOS |
| **Alembic / Django** | `alembic.ini`, `migrations/*.py` | Python migration ops, not SQL | — | — | **None** | Out of scope |
| **raw embedded SQL** | `//go:embed *.sql`, `schema.sql`, ad-hoc `db.Exec` | versioned pairs → identity; lone `schema.sql` → `0001_init` (+ down stub) | hand-written queries oracle-checked | synth | **Partial** | Assisted |
| **no DB tooling** | — | — | — | — | — | N/A |

Hard gate G collapses any **non-PostgreSQL** project to **Unsupported (today)**
for `generate`, regardless of migration hygiene — SQLite/MySQL codegen is the M5
line. `migrate`/`verify` remain PG-only in the current build.

## 3. Command surface — `sequa adopt`

Three verbs, increasing in commitment; the determinism contract is per-verb.

```
sequa adopt detect [path]     # read-only. fingerprint + tier + one-line why.
sequa adopt plan   [path]     # read-only. full plan: per-file {auto|assisted|manual},
                              #   a generated sequa.yaml, and a per-query portability
                              #   report from the oracle. writes a report; mutates nothing.
sequa adopt apply  [path] --out DIR
                              # executes ONLY the auto-tier transforms into DIR
                              #   (or --in-place behind a clean-git guard). assisted/manual
                              #   items become clearly-marked stubs + TODOs. idempotent.
```

**Contract:** `detect`/`plan` are 100% deterministic and never write to the
source. `apply` performs only byte-mechanical transforms and **never fabricates
semantics** — a missing `.down` becomes a commented stub named
`<v>_<name>.down.sql` containing `-- TODO: no down migration in source (<system>)`,
and an unsupported query is copied verbatim into `queries/_unsupported.sql` with
the exact reason from the oracle. Nothing is silently dropped.

## 4. The oracle — why suitability is *exact*, not guessed

sequa already ships the two components a converter would otherwise have to
re-implement heuristically:

- `codegen.readUpMigrations` + catalog builder — proves whether a migration set
  parses into a schema (via `pg_query_go` AST), and *which statement* fails.
- `codegen.AnalyzeQueries` — proves, per query, whether sequa can type it against
  that catalog, and *why not* (unknown verb, JOIN it can't resolve, CTE, window
  function, multi-statement block, …).

So `adopt plan` doesn't estimate portability — it **runs sequa's own analyzer in
dry-run** over the candidate and reports the verdict per artifact. This is the
deterministic backbone: the question "can sequa handle this project?" is answered
by *the same code path that would run at generate time*. Two real findings from a
live trial against a 106-migration Postgres project (`lensr`) validate this:

- Models codegen succeeded on all 106 migrations → 114 structs of compiling Go.
- Query codegen halted deterministically on the first `:execrows` query — and the
  oracle can pinpoint that verb, and every JOIN/CTE the current analyzer rejects,
  *before* any conversion is attempted.

## 5. Suitability scoring (deterministic rubric)

**Gates** (any true → not Turnkey/Assisted):
- engine ≠ postgres → **Unsupported (today)**
- migrations are Go/Python code, or there are no versioned migrations and no
  `schema.sql` → **Manual**

**Score** (0–100), all inputs measured, none estimated:
- `paired_down_ratio` — fraction of versions with a real `.down` (missing ⇒ stubs).
- `query_portability` — `portable / total` from the oracle (0 queries ⇒ N/A, not 0).
- `catalog_parse` — migrations parse into a catalog cleanly (bool).
- `config_automappable` — a source config (`sqlc.yaml`, …) maps 1:1 (bool).

**Buckets:** Turnkey ≥ 85 · Assisted 50–84 · Manual < 50 · Unsupported (gated).
The report always prints the numbers, so the bucket is auditable.

## 6. Implementation phases

1. **`detect`** — marker-file fingerprinting + engine detection + counts. Pure,
   fast, no parsing. (This is also exactly the scan used to categorize a whole
   tree of projects.)
2. **`plan`** — add the oracle pass (reuse `codegen` in dry-run) + `sequa.yaml`
   synthesis + per-file action tagging. Emits a Markdown/JSON report.
3. **`apply`** — the mechanical transforms per §2, auto-tier only, idempotent,
   git-guarded. Assisted/manual → stubs + TODOs.
4. **Verb/feature coverage** grows the Turnkey set over time: recognizing (and
   supporting) `:execrows`, then JOIN/CTE breadth in the analyzer, each of which
   promotes real queries from `unsupported` to `portable` in the oracle — a
   measurable graduation.

## 7. Honest limits

- **Postgres-only** is the dominant gate; most "not suitable" verdicts are engine,
  not hygiene.
- **Down migrations** cannot be invented from up-only sources (atlas/Flyway) —
  the tool stubs and flags; it does not guess reversibility.
- **Go/Python code migrations** are opaque to a SQL converter — Manual by design.
- **Cross-ecosystem** (Prisma/Drizzle/Alembic/Django) is out of sequa's Go scope
  even when the emitted SQL is extractable.
- The oracle is only as broad as the analyzer: today it rejects outer joins,
  `SELECT *` across joins, CTEs, and non-`:one/:many/:exec` verbs. That's a
  *feature*, not a bug, of the report — it tells the truth about coverage.
