package codegen

import "testing"

func TestBuildCatalog(t *testing.T) {
	migrations := []string{
		`CREATE TABLE tasks (
			id         BIGSERIAL   PRIMARY KEY,
			title      TEXT        NOT NULL,
			done       BOOLEAN     NOT NULL DEFAULT FALSE,
			note       TEXT,
			tags       TEXT[],
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);`,
		`ALTER TABLE tasks ADD COLUMN priority INT;`,
		`ALTER TABLE tasks DROP COLUMN note;`,
	}

	cat, err := BuildCatalog(migrations)
	if err != nil {
		t.Fatalf("BuildCatalog: %v", err)
	}
	tbl := cat.Table("tasks")
	if tbl == nil {
		t.Fatal("tasks table missing from catalog")
	}

	got := map[string]Column{}
	for _, c := range tbl.Columns {
		t.Logf("col %-10s pg=%-12q notnull=%-5v array=%v", c.Name, c.PgType, c.NotNull, c.Array)
		got[c.Name] = c
	}

	if _, dropped := got["note"]; dropped {
		t.Error("column 'note' should have been dropped")
	}
	if _, added := got["priority"]; !added {
		t.Error("column 'priority' should have been added")
	}
	if !got["id"].NotNull {
		t.Errorf("id should be NotNull (primary key): %+v", got["id"])
	}
	if c := got["title"]; c.PgType != "text" || !c.NotNull {
		t.Errorf("title: got %+v, want pg=text notnull=true", c)
	}
	if c := got["done"]; c.PgType != "bool" {
		t.Errorf("done pgType: got %q, want bool", c.PgType)
	}
	if c := got["created_at"]; c.PgType != "timestamptz" {
		t.Errorf("created_at pgType: got %q, want timestamptz", c.PgType)
	}
	if c := got["priority"]; c.NotNull {
		t.Errorf("priority should be nullable: %+v", c)
	}
	if c := got["tags"]; !c.Array {
		t.Errorf("tags should be an array: %+v", c)
	}
}

func TestBuildCatalogDropTable(t *testing.T) {
	cat, err := BuildCatalog([]string{
		`CREATE TABLE keep (id BIGINT PRIMARY KEY);`,
		`CREATE TABLE gone (id BIGINT PRIMARY KEY);`,
		`DROP TABLE gone;`,
	})
	if err != nil {
		t.Fatalf("BuildCatalog: %v", err)
	}
	if cat.Table("gone") != nil {
		t.Error("dropped table 'gone' still resolvable by name")
	}
	if cat.Table("keep") == nil {
		t.Error("table 'keep' missing after dropping a sibling")
	}
	if len(cat.Tables) != 1 || cat.Tables[0].Name != "keep" {
		t.Errorf("Tables = %v, want exactly [keep]", tableNames(cat.Tables))
	}
}

func tableNames(tables []*Table) []string {
	out := make([]string, len(tables))
	for i, t := range tables {
		out[i] = t.Name
	}
	return out
}
