package codegen

import (
	"testing"

	pgquery "github.com/pganalyze/pg_query_go/v5"
)

// joinCatalog is a two-table schema (authors 1:N books) used by the JOIN tests.
// books.id and authors.id share the bare name "id" (an ambiguity source) and
// books.price_cents is nullable (so INNER-JOIN nullability preservation shows).
func joinCatalog(t *testing.T) *Catalog {
	t.Helper()
	cat, err := BuildCatalog([]string{
		`CREATE TABLE authors (id BIGSERIAL PRIMARY KEY, name TEXT NOT NULL);`,
		`CREATE TABLE books (id BIGSERIAL PRIMARY KEY, author_id BIGINT NOT NULL, title TEXT NOT NULL, price_cents INT);`,
	})
	if err != nil {
		t.Fatal(err)
	}
	return cat
}

func parseSelectFrom(t *testing.T, sql string) []*pgquery.Node {
	t.Helper()
	res, err := pgquery.Parse(sql)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	sel := res.Stmts[0].Stmt.GetSelectStmt()
	if sel == nil {
		t.Fatalf("not a SELECT: %q", sql)
	}
	return sel.FromClause
}

func TestCollectRelations(t *testing.T) {
	cat := joinCatalog(t)

	t.Run("aliased inner join", func(t *testing.T) {
		from := parseSelectFrom(t, "SELECT 1 FROM books b INNER JOIN authors a ON b.author_id = a.id")
		rels, err := collectRelations(cat, from)
		if err != nil {
			t.Fatalf("collectRelations: %v", err)
		}
		if len(rels) != 2 {
			t.Fatalf("got %d relations, want 2", len(rels))
		}
		if rels[0].name != "b" || rels[0].table.Name != "books" {
			t.Errorf("rel[0] = %+v, want alias b -> books", rels[0])
		}
		if rels[1].name != "a" || rels[1].table.Name != "authors" {
			t.Errorf("rel[1] = %+v, want alias a -> authors", rels[1])
		}
	})

	t.Run("unaliased falls back to table name", func(t *testing.T) {
		from := parseSelectFrom(t, "SELECT 1 FROM books JOIN authors ON books.author_id = authors.id")
		rels, err := collectRelations(cat, from)
		if err != nil {
			t.Fatalf("collectRelations: %v", err)
		}
		if len(rels) != 2 || rels[0].name != "books" || rels[1].name != "authors" {
			t.Fatalf("rels = %+v, want [books authors]", rels)
		}
	})

	t.Run("unknown table rejected", func(t *testing.T) {
		from := parseSelectFrom(t, "SELECT 1 FROM books b INNER JOIN nope n ON b.author_id = n.id")
		if _, err := collectRelations(cat, from); err == nil {
			t.Fatal("expected an error for an unknown joined table")
		}
	})
}

func TestResolveRelationColumn(t *testing.T) {
	cat := joinCatalog(t)
	rels, err := collectRelations(cat, parseSelectFrom(t,
		"SELECT 1 FROM books b INNER JOIN authors a ON b.author_id = a.id"))
	if err != nil {
		t.Fatal(err)
	}

	if c, err := resolveRelationColumn(rels, "a", "name"); err != nil || c.Name != "name" || c.PgType != "text" {
		t.Errorf("a.name = %+v err=%v, want text column name", c, err)
	}
	if c, err := resolveRelationColumn(rels, "", "title"); err != nil || c.Name != "title" {
		t.Errorf("unqualified title = %+v err=%v", c, err)
	}
	if _, err := resolveRelationColumn(rels, "a", "nope"); err == nil {
		t.Error("expected error for unknown column a.nope")
	}
	if _, err := resolveRelationColumn(rels, "z", "id"); err == nil {
		t.Error("expected error for unknown alias z")
	}
	if _, err := resolveRelationColumn(rels, "", "id"); err == nil {
		t.Error("expected ambiguity error for bare id (in both tables)")
	}
	if _, err := resolveRelationColumn(rels, "", "missing"); err == nil {
		t.Error("expected error for unknown unqualified column")
	}
}

func TestJoinAnalyzeErrors(t *testing.T) {
	cat := joinCatalog(t)
	cases := map[string]string{
		"outer join rejected":       "-- name: Q :many\nSELECT b.id AS bid, a.name FROM books b LEFT JOIN authors a ON b.author_id = a.id;",
		"right join rejected":       "-- name: Q :many\nSELECT b.id AS bid, a.name FROM books b RIGHT JOIN authors a ON b.author_id = a.id;",
		"star across join rejected": "-- name: Q :many\nSELECT * FROM books b INNER JOIN authors a ON b.author_id = a.id;",
		"qualified star rejected":   "-- name: Q :many\nSELECT b.* FROM books b INNER JOIN authors a ON b.author_id = a.id;",
		"ambiguous unqualified":     "-- name: Q :many\nSELECT id FROM books b INNER JOIN authors a ON b.author_id = a.id;",
		"duplicate result name":     "-- name: Q :many\nSELECT b.id, a.id FROM books b INNER JOIN authors a ON b.author_id = a.id;",
		"unknown qualifier":         "-- name: Q :many\nSELECT x.id AS xid FROM books b INNER JOIN authors a ON b.author_id = a.id;",
	}
	for name, sql := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := AnalyzeQueries(cat, sql); err == nil {
				t.Fatalf("expected an error, got none")
			}
		})
	}
}

func TestJoinAnalyzeSuccess(t *testing.T) {
	cat := joinCatalog(t)
	qs, err := AnalyzeQueries(cat, `
-- name: ListBooksWithAuthor :many
SELECT b.id AS book_id, b.title, b.price_cents, a.name AS author_name
FROM books b
INNER JOIN authors a ON b.author_id = a.id
WHERE a.id = $1;
`)
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	if len(qs) != 1 {
		t.Fatalf("got %d queries, want 1", len(qs))
	}
	q := qs[0]
	if q.Star {
		t.Error("a JOIN query must not be Star (needs a per-query row struct)")
	}
	if len(q.Columns) != 4 {
		t.Fatalf("got %d result columns, want 4", len(q.Columns))
	}
	wantType := map[string]string{
		"book_id":     "int64",         // b.id (PK) resolved via alias, aliased
		"title":       "string",        // b.title NOT NULL
		"price_cents": "sql.NullInt32", // b.price_cents nullable — preserved under INNER JOIN
		"author_name": "string",        // a.name NOT NULL, aliased
	}
	for _, c := range q.Columns {
		if got := goTypeFor(c).Name; got != wantType[c.Name] {
			t.Errorf("column %q Go type = %q, want %q", c.Name, got, wantType[c.Name])
		}
	}
	if len(q.Params) != 1 || q.Params[0].GoType.Name != "int64" {
		t.Errorf("params = %+v, want one int64 bound to a.id across the join", q.Params)
	}
}

func TestJoinAggregateOverJoinedColumn(t *testing.T) {
	cat := joinCatalog(t)
	qs, err := AnalyzeQueries(cat, `
-- name: PricePerAuthor :many
SELECT a.name AS author_name, count(*) AS n, sum(b.price_cents) AS total
FROM books b
INNER JOIN authors a ON b.author_id = a.id
GROUP BY a.name;
`)
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	got := map[string]string{}
	for _, c := range qs[0].Columns {
		got[c.Name] = goTypeFor(c).Name
	}
	want := map[string]string{
		"author_name": "string",
		"n":           "int64",         // count(*) -> non-null bigint
		"total":       "sql.NullInt64", // sum(b.price_cents int4) -> nullable bigint
	}
	for name, w := range want {
		if got[name] != w {
			t.Errorf("column %q = %q, want %q (all=%v)", name, got[name], w, got)
		}
	}
}
