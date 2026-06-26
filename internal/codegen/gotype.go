package codegen

// GoType is a Go type reference plus the import path it requires (empty for
// builtins).
type GoType struct {
	Name   string
	Import string
}

// goTypeFor maps a column to its Go type, honoring array-ness and nullability.
// Nullable scalars use database/sql's sql.Null* wrappers (like sqlc's default);
// arrays map to slices (a nil slice represents SQL NULL).
func goTypeFor(col Column) GoType {
	base, imp := baseGoType(col.PgType)
	if col.Array {
		return GoType{Name: "[]" + base, Import: imp}
	}
	if col.NotNull {
		return GoType{Name: base, Import: imp}
	}
	if nt, nimp, ok := nullGoType(col.PgType); ok {
		return GoType{Name: nt, Import: nimp}
	}
	return GoType{Name: "*" + base, Import: imp}
}

func baseGoType(pg string) (name, imp string) {
	switch pg {
	case "int8", "bigserial", "serial8":
		return "int64", ""
	case "int4", "serial", "serial4":
		return "int32", ""
	case "int2", "smallserial", "serial2":
		return "int16", ""
	case "bool":
		return "bool", ""
	case "text", "varchar", "bpchar", "char", "name", "citext":
		return "string", ""
	case "uuid", "numeric", "money":
		return "string", ""
	case "float8":
		return "float64", ""
	case "float4":
		return "float32", ""
	case "bytea":
		return "[]byte", ""
	case "json", "jsonb":
		return "[]byte", ""
	case "timestamptz", "timestamp", "date", "time", "timetz":
		return "time.Time", "time"
	default:
		return "interface{}", ""
	}
}

func nullGoType(pg string) (name, imp string, ok bool) {
	switch pg {
	case "int8", "bigserial", "serial8":
		return "sql.NullInt64", "database/sql", true
	case "int4", "serial", "serial4":
		return "sql.NullInt32", "database/sql", true
	case "int2", "smallserial", "serial2":
		return "sql.NullInt16", "database/sql", true
	case "bool":
		return "sql.NullBool", "database/sql", true
	case "text", "varchar", "bpchar", "char", "name", "citext", "uuid", "numeric", "money":
		return "sql.NullString", "database/sql", true
	case "float8", "float4":
		return "sql.NullFloat64", "database/sql", true
	case "timestamptz", "timestamp", "date", "time", "timetz":
		return "sql.NullTime", "database/sql", true
	}
	return "", "", false
}
