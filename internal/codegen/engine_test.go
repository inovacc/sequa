package codegen

import "testing"

func TestEngineFor(t *testing.T) {
	for _, name := range []string{"postgresql", "postgres", ""} {
		eng, err := engineFor(name)
		if err != nil {
			t.Errorf("engineFor(%q) error: %v", name, err)
			continue
		}
		if eng.Name() != "postgresql" {
			t.Errorf("engineFor(%q).Name() = %q, want postgresql", name, eng.Name())
		}
	}
	if _, err := engineFor("mysql"); err == nil {
		t.Fatal("engineFor(mysql) should error until M5")
	}
}

func TestPostgresEngineRoundTrip(t *testing.T) {
	eng, err := engineFor("postgresql")
	if err != nil {
		t.Fatal(err)
	}
	cat, err := eng.BuildCatalog([]string{`CREATE TABLE t (id BIGINT PRIMARY KEY, name TEXT NOT NULL);`})
	if err != nil {
		t.Fatal(err)
	}
	if cat.Table("t") == nil {
		t.Fatal("table t missing from catalog")
	}
	qs, err := eng.AnalyzeQueries(cat, "-- name: Get :one\nSELECT id, name FROM t WHERE id = $1;")
	if err != nil {
		t.Fatal(err)
	}
	if len(qs) != 1 || len(qs[0].Columns) != 2 {
		t.Fatalf("unexpected analysis: %+v", qs)
	}
}
