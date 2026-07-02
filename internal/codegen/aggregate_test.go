package codegen

import "testing"

func TestAnalyzeAggregates(t *testing.T) {
	cat, err := BuildCatalog([]string{
		`CREATE TABLE books (id BIGINT PRIMARY KEY, price_cents INT, author_id BIGINT NOT NULL);`,
	})
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name     string
		sql      string
		wantType map[string]string // result column name -> Go type
	}{
		{
			name:     "count is a non-null bigint",
			sql:      "-- name: C :one\nSELECT count(*) AS n FROM books;",
			wantType: map[string]string{"n": "int64"},
		},
		{
			name:     "max keeps the column type but is nullable",
			sql:      "-- name: M :one\nSELECT max(id) AS hi FROM books;",
			wantType: map[string]string{"hi": "sql.NullInt64"},
		},
		{
			name:     "min of an int column is nullable int32",
			sql:      "-- name: L :one\nSELECT min(price_cents) AS lo FROM books;",
			wantType: map[string]string{"lo": "sql.NullInt32"},
		},
		{
			name:     "mixed plain column and aggregate",
			sql:      "-- name: G :many\nSELECT author_id, count(*) AS n FROM books GROUP BY author_id;",
			wantType: map[string]string{"author_id": "int64", "n": "int64"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			qs, err := AnalyzeQueries(cat, tt.sql)
			if err != nil {
				t.Fatalf("analyze: %v", err)
			}
			if len(qs) != 1 {
				t.Fatalf("got %d queries, want 1", len(qs))
			}
			got := map[string]string{}
			for _, c := range qs[0].Columns {
				got[c.Name] = goTypeFor(c).Name
			}
			for name, want := range tt.wantType {
				if got[name] != want {
					t.Errorf("column %q = %q, want %q (all=%v)", name, got[name], want, got)
				}
			}
		})
	}
}

func TestAnalyzeUnsupportedAggregate(t *testing.T) {
	cat, err := BuildCatalog([]string{`CREATE TABLE t (x INT);`})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := AnalyzeQueries(cat, "-- name: S :one\nSELECT sum(x) AS s FROM t;"); err == nil {
		t.Fatal("expected error for unsupported aggregate sum(...)")
	}
}
