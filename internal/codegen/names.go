package codegen

import "strings"

// initialisms are rendered fully upper-cased in Go identifiers (per Go style).
var initialisms = map[string]string{
	"id": "ID", "api": "API", "url": "URL", "uri": "URI", "uuid": "UUID",
	"http": "HTTP", "https": "HTTPS", "json": "JSON", "sql": "SQL",
	"ip": "IP", "db": "DB", "html": "HTML", "ttl": "TTL", "cpu": "CPU",
}

// camelWords joins the underscore-separated words of snake into one camel
// identifier. When exported, every word is title-cased and known initialisms are
// upper-cased (task_id -> TaskID). When unexported, the first word is fully
// lower-cased and initialisms are left untouched (user_id -> userId), which
// keeps generated query argument names stable.
func camelWords(snake string, exported bool) string {
	var b strings.Builder
	for _, p := range strings.Split(snake, "_") {
		if p == "" {
			continue
		}
		if !exported && b.Len() == 0 {
			b.WriteString(strings.ToLower(p))
			continue
		}
		if exported {
			if up, ok := initialisms[strings.ToLower(p)]; ok {
				b.WriteString(up)
				continue
			}
		}
		b.WriteString(strings.ToUpper(p[:1]))
		b.WriteString(strings.ToLower(p[1:]))
	}
	return b.String()
}

// goName converts a snake_case identifier to an exported Go CamelCase name,
// upper-casing known initialisms (e.g. "task_id" -> "TaskID").
func goName(snake string) string { return camelWords(snake, true) }

// lowerCamel converts a snake_case identifier to an unexported camelCase name,
// leaving initialisms untouched (e.g. "user_id" -> "userId").
func lowerCamel(snake string) string { return camelWords(snake, false) }

// lowerFirst lower-cases the first byte of an already-CamelCase identifier. It
// differs from lowerCamel, which takes snake_case input.
func lowerFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToLower(s[:1]) + s[1:]
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
