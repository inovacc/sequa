package codegen

import (
	"fmt"
	"sort"
)

// DiffKind classifies a discrepancy between the migration-defined (static)
// catalog and the live database catalog.
type DiffKind string

const (
	DiffTableMissing  DiffKind = "table_missing"  // defined by migrations, absent live
	DiffTableExtra    DiffKind = "table_extra"    // present live, not defined by migrations
	DiffColumnMissing DiffKind = "column_missing" // defined by migrations, absent live
	DiffColumnExtra   DiffKind = "column_extra"   // present live, not defined by migrations
	DiffType          DiffKind = "type_mismatch"
	DiffNullability   DiffKind = "nullability_mismatch"
)

// SchemaDiff is one discrepancy reported by DiffCatalogs.
type SchemaDiff struct {
	Kind   DiffKind
	Table  string
	Column string // empty for table-level diffs
	Detail string
}

// String renders a diff for CLI output.
func (d SchemaDiff) String() string {
	target := fmt.Sprintf("table %q", d.Table)
	if d.Column != "" {
		target = fmt.Sprintf("%q.%q", d.Table, d.Column)
	}
	if d.Detail == "" {
		return fmt.Sprintf("%s: %s", d.Kind, target)
	}
	return fmt.Sprintf("%s: %s (%s)", d.Kind, target, d.Detail)
}

// DiffCatalogs compares the static catalog (parsed from migrations) against the
// live catalog (from Introspect) and returns every discrepancy in a stable
// order. An empty result means the live schema matches the migrations.
func DiffCatalogs(static, live *Catalog) []SchemaDiff {
	var diffs []SchemaDiff
	for _, st := range static.Tables {
		lt := live.Table(st.Name)
		if lt == nil {
			diffs = append(diffs, SchemaDiff{Kind: DiffTableMissing, Table: st.Name})
			continue
		}
		diffs = append(diffs, diffColumns(st, lt)...)
	}
	for _, lt := range live.Tables {
		if static.Table(lt.Name) == nil {
			diffs = append(diffs, SchemaDiff{Kind: DiffTableExtra, Table: lt.Name})
		}
	}
	return diffs
}

func diffColumns(static, live *Table) []SchemaDiff {
	var diffs []SchemaDiff
	liveCols := make(map[string]Column, len(live.Columns))
	for _, c := range live.Columns {
		liveCols[c.Name] = c
	}
	staticCols := make(map[string]bool, len(static.Columns))
	for _, sc := range static.Columns {
		staticCols[sc.Name] = true
		lc, ok := liveCols[sc.Name]
		if !ok {
			diffs = append(diffs, SchemaDiff{Kind: DiffColumnMissing, Table: static.Name, Column: sc.Name})
			continue
		}
		if normalizeType(sc.PgType) != normalizeType(lc.PgType) || sc.Array != lc.Array {
			diffs = append(diffs, SchemaDiff{
				Kind: DiffType, Table: static.Name, Column: sc.Name,
				Detail: fmt.Sprintf("migrations=%s live=%s", typeLabel(sc), typeLabel(lc)),
			})
		}
		if sc.NotNull != lc.NotNull {
			diffs = append(diffs, SchemaDiff{
				Kind: DiffNullability, Table: static.Name, Column: sc.Name,
				Detail: fmt.Sprintf("migrations notnull=%v live notnull=%v", sc.NotNull, lc.NotNull),
			})
		}
	}
	extra := make([]string, 0)
	for name := range liveCols {
		if !staticCols[name] {
			extra = append(extra, name)
		}
	}
	sort.Strings(extra)
	for _, name := range extra {
		diffs = append(diffs, SchemaDiff{Kind: DiffColumnExtra, Table: static.Name, Column: name})
	}
	return diffs
}

func typeLabel(c Column) string {
	if c.Array {
		return c.PgType + "[]"
	}
	return c.PgType
}

// normalizeType canonicalizes pg type names so migration-parsed names and
// introspected names compare equal. Serial pseudo-types resolve to their
// underlying integer type (a bigserial column is stored as int8 in the live DB).
func normalizeType(pg string) string {
	switch pg {
	case "bigserial", "serial8":
		return "int8"
	case "serial", "serial4":
		return "int4"
	case "smallserial", "serial2":
		return "int2"
	}
	return pg
}
