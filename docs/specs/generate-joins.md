# generate: multi-table JOIN support (design spec)

Status: proposed (2026-07-02) · Tracks: ISS-2, BACKLOG P1

Point-in-time design record. Not a living doc — do not rev-tag.

## Goal

Let `sequa generate` type queries that span multiple tables via JOINs, so
result columns and parameters resolve across the joined relations. Today the
analyzer is single-table (`fromTable` returns the first `RangeVar`;
`resolveResultColumn`/`bindWhere` resolve against one `*Table`).

## Why it is correctness-sensitive (do not rush)

The subtle part is **nullability under outer joins**, which directly changes the
generated Go type (`string` vs `sql.NullString`):

- `INNER JOIN` — every column keeps its own nullability.
- `LEFT JOIN b` — all of **b**'s columns become nullable (a row may have no match).
- `RIGHT JOIN b` — all of **a**'s (the left side's) columns become nullable.
- `FULL JOIN` — both sides become nullable.

Getting this wrong silently generates a non-nullable field for a column that can
be NULL, so a scan into it fails at runtime. That is why this is specced, not
rushed.

## Design

1. **Collect relations.** Replace `fromTable` with a walk of the `FromClause`
   that descends `JoinExpr` nodes, producing an ordered list of
   `{table, alias, nullable bool}` — `nullable` set per the outer-join rules
   above (propagated down the join tree).
2. **Resolution map.** Build `map[string]relation` keyed by alias (falling back
   to table name) for qualified refs (`t.col`), plus an unqualified index that
   errors on ambiguity (a bare `id` present in two relations).
3. **Result typing.** `resolveResultColumn` resolves qualified/unqualified refs
   against the map; a column from a nullable-side relation is forced nullable
   (set `NotNull=false` on the produced `Column` regardless of its own DDL
   nullability).
4. **Params.** `bindWhere`/`bindInsert`/`bindUpdate` take the relation map so
   `WHERE a.id = $1` binds to the right table's column.
5. Aggregates over joined columns reuse the existing aggregate typing on the
   resolved (possibly nullability-adjusted) column.

## Phasing

1. **INNER JOIN only** — nullability-preserving, so the risky outer-join logic is
   deferred. Delivers the common case (join to a lookup/parent table) correctly.
2. **Outer joins** — add the nullable-side propagation with dedicated golden
   fixtures per join type.

## Test plan

- Unit: relation collection + ambiguity detection + per-join-type nullability.
- Golden: fixtures for INNER, LEFT, RIGHT, FULL joins asserting the exact
  generated struct field types (this is where outer-join nullability is pinned).
- Reject ambiguous unqualified columns with a clear error.

## Non-goals (initial)

Subqueries/CTEs in FROM, lateral joins, `USING`/natural joins (require column
merging), and cross joins.
