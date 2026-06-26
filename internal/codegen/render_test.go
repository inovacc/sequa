package codegen

import (
	"strings"
	"testing"
)

func TestRenderModels(t *testing.T) {
	cat, err := BuildCatalog([]string{
		`CREATE TABLE tasks (
			id         BIGSERIAL   PRIMARY KEY,
			title      TEXT        NOT NULL,
			done       BOOLEAN     NOT NULL DEFAULT false,
			note       TEXT,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);`,
	})
	if err != nil {
		t.Fatal(err)
	}

	out, err := RenderModels(cat, "db") // RenderModels runs go/format → valid Go
	if err != nil {
		t.Fatalf("RenderModels: %v", err)
	}
	src := string(out)
	t.Logf("generated:\n%s", src)

	// Collapse gofmt's alignment whitespace for robust substring checks.
	norm := strings.Join(strings.Fields(src), " ")
	for _, want := range []string{
		"package db",
		`"database/sql"`,
		`"time"`,
		"type Task struct {",
		"ID int64",
		"Title string",
		"Done bool",
		"Note sql.NullString",
		"CreatedAt time.Time",
		"DO NOT EDIT",
	} {
		if !strings.Contains(norm, want) {
			t.Errorf("generated models missing %q", want)
		}
	}
}
