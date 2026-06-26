package codegen

import "strings"

// initialisms are rendered fully upper-cased in Go identifiers (per Go style).
var initialisms = map[string]string{
	"id": "ID", "api": "API", "url": "URL", "uri": "URI", "uuid": "UUID",
	"http": "HTTP", "https": "HTTPS", "json": "JSON", "sql": "SQL",
	"ip": "IP", "db": "DB", "html": "HTML", "ttl": "TTL", "cpu": "CPU",
}

// goName converts a snake_case identifier to an exported Go CamelCase name,
// upper-casing known initialisms (e.g. "task_id" -> "TaskID").
func goName(snake string) string {
	var b strings.Builder
	for _, p := range strings.Split(snake, "_") {
		if p == "" {
			continue
		}
		if up, ok := initialisms[strings.ToLower(p)]; ok {
			b.WriteString(up)
			continue
		}
		b.WriteString(strings.ToUpper(p[:1]))
		b.WriteString(strings.ToLower(p[1:]))
	}
	return b.String()
}

// singular applies a small, predictable English singularization to a table
// name so "tasks" -> "task". It is intentionally simple; exotic plurals may
// need an explicit override (a future config knob).
func singular(s string) string {
	switch {
	case strings.HasSuffix(s, "ies") && len(s) > 3:
		return s[:len(s)-3] + "y"
	case strings.HasSuffix(s, "ses"), strings.HasSuffix(s, "xes"),
		strings.HasSuffix(s, "zes"), strings.HasSuffix(s, "ches"),
		strings.HasSuffix(s, "shes"):
		return s[:len(s)-2]
	case strings.HasSuffix(s, "s") && !strings.HasSuffix(s, "ss"):
		return s[:len(s)-1]
	}
	return s
}

// structName is the Go struct name for a table (singularized + CamelCased).
func structName(table string) string { return goName(singular(table)) }
