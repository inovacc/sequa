# generate: :copyfrom and :batch query verbs (design spec)

Status: proposed (2026-07-02) · Tracks: BACKLOG P3

Point-in-time design record. Not a living doc — do not rev-tag.

## Goal

Add bulk/batched query verbs to the annotation set (today `:one`/`:many`/
`:exec`): `:copyfrom` for high-throughput inserts, and `:batch` for issuing many
parameterized statements efficiently.

## The impedance mismatch (why this is specced, not rushed)

sequa's generated `Queries` runs against a small `DBTX` interface
(`ExecContext`/`QueryContext`/`QueryRowContext`) and connects with `lib/pq` over
`database/sql`. Both new verbs push past that contract:

- **`:copyfrom`** needs PostgreSQL's COPY protocol. With `lib/pq` that is
  `pq.CopyIn(table, cols...)` prepared **inside a transaction** — it requires
  `Begin`/`Prepare`, which `DBTX` does not expose. So `:copyfrom` forces either a
  richer generated interface (add `BeginTx`) or a runtime type-assertion of
  `DBTX` to a copier.
- **`:batch`** wants pipelining. `database/sql`/`lib/pq` has **no batch/pipeline
  API** — that is a `pgx` feature (`pgx.Batch`, `SendBatch`). Emulating it with a
  per-statement loop in one transaction gives the API shape but none of the
  round-trip savings.

## Options

1. **`:copyfrom` via an extended contract (recommended first).** Add a
   `BeginTx(context.Context, *sql.TxOptions) (*sql.Tx, error)` capability
   (either widen `DBTX`, or generate the copy method only when `New` is given a
   `*sql.DB`). Generate:
   `func (q *Queries) InsertXs(ctx, arg []XParams) (int64, error)` that opens a
   tx, `pq.CopyIn(table, cols...)`, execs per row, flushes, commits.
2. **`:batch` — defer or gate on a driver.** A correct, savings-bearing `:batch`
   needs `pgx`. Options: (a) defer until sequa optionally supports a `pgx`
   backend; (b) generate a transaction-loop "batch" and document that it is
   sequential, not pipelined. Recommendation: **defer** rather than ship a verb
   whose name implies performance it does not deliver.

## Phasing

1. `:copyfrom` (Postgres, `lib/pq` COPY) with the extended contract + golden
   tests asserting the generated bulk method and the `[]XParams` type.
2. `:batch` only alongside a `pgx` backend (ties into the M5 engine work).

## Test plan

- Extend the query-header regex and `QueryCmd` set; unit-test parsing.
- Golden fixtures for a `:copyfrom` insert (method signature + params struct).
- Gated integration test: bulk-insert N rows via the generated method against
  real Postgres and count them back.

## Non-goals (initial)

`:copyfrom` for non-INSERT shapes; `ON CONFLICT` upsert batching; MySQL/SQLite
bulk paths (fold into the M5 engine backends).
