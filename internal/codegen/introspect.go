package codegen

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// introspectQuery reads the live "public" schema from pg_catalog. typname uses
// Postgres's internal short names (int8, text, timestamptz, bool, _text for
// arrays), which line up with the names BuildCatalog derives from migrations.
const introspectQuery = `
SELECT c.relname, a.attname, t.typname, a.attnotnull, (t.typcategory = 'A')
FROM pg_catalog.pg_attribute a
JOIN pg_catalog.pg_class c ON c.oid = a.attrelid
JOIN pg_catalog.pg_type t ON t.oid = a.atttypid
JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
WHERE c.relkind = 'r'
  AND n.nspname = 'public'
  AND a.attnum > 0
  AND NOT a.attisdropped
  -- Exclude migration bookkeeping tables so verify does not flag them as drift.
  AND c.relname NOT IN ('schema_migrations', 'sequa_schema_history')
ORDER BY c.relname, a.attnum`

// Introspect builds a schema Catalog from the live database by reading
// pg_catalog, in the same shape BuildCatalog derives statically from
// migrations, so the two can be compared with DiffCatalogs.
func Introspect(ctx context.Context, db *sql.DB) (*Catalog, error) {
	rows, err := db.QueryContext(ctx, introspectQuery)
	if err != nil {
		return nil, fmt.Errorf("introspect schema: %w", err)
	}
	defer func() { _ = rows.Close() }()

	cat := newCatalog()
	for rows.Next() {
		var table, column, typ string
		var notNull, isArray bool
		if err := rows.Scan(&table, &column, &typ, &notNull, &isArray); err != nil {
			return nil, fmt.Errorf("scan column: %w", err)
		}
		t, ok := cat.byName[table]
		if !ok {
			t = &Table{Name: table}
			cat.Tables = append(cat.Tables, t)
			cat.byName[table] = t
		}
		if isArray {
			typ = strings.TrimPrefix(typ, "_") // "_text" -> "text"
		}
		t.Columns = append(t.Columns, Column{Name: column, PgType: typ, NotNull: notNull, Array: isArray})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate columns: %w", err)
	}
	return cat, nil
}
