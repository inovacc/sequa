package codegen

import "testing"

func TestAnalyzeQueries(t *testing.T) {
	cat, err := BuildCatalog([]string{
		`CREATE TABLE users (id BIGSERIAL PRIMARY KEY, email TEXT NOT NULL, name TEXT);`,
	})
	if err != nil {
		t.Fatal(err)
	}

	qs, err := AnalyzeQueries(cat, `
-- name: GetUser :one
SELECT * FROM users WHERE id = $1;

-- name: ListByEmail :many
SELECT id, email FROM users WHERE email = $1;

-- name: CreateUser :one
INSERT INTO users (email, name) VALUES ($1, $2) RETURNING *;

-- name: DeleteUser :exec
DELETE FROM users WHERE id = $1;
`)
	if err != nil {
		t.Fatalf("AnalyzeQueries: %v", err)
	}
	if len(qs) != 4 {
		t.Fatalf("got %d queries, want 4", len(qs))
	}
	by := map[string]Query{}
	for _, q := range qs {
		by[q.Name] = q
		t.Logf("%s cmd=%s table=%s star=%v cols=%d params=%v", q.Name, q.Cmd, q.Table, q.Star, len(q.Columns), q.Params)
	}

	if g := by["GetUser"]; g.Cmd != CmdOne || !g.Star || g.Table != "users" ||
		len(g.Params) != 1 || g.Params[0].GoType.Name != "int64" {
		t.Errorf("GetUser wrong: %+v", g)
	}
	if l := by["ListByEmail"]; l.Star || len(l.Columns) != 2 ||
		len(l.Params) != 1 || l.Params[0].GoType.Name != "string" {
		t.Errorf("ListByEmail wrong: %+v", l)
	}
	if c := by["CreateUser"]; !c.Star || len(c.Params) != 2 ||
		c.Params[0].GoType.Name != "string" || c.Params[1].GoType.Name != "sql.NullString" {
		t.Errorf("CreateUser wrong: params=%+v star=%v", c.Params, c.Star)
	}
	if d := by["DeleteUser"]; d.Cmd != CmdExec || len(d.Columns) != 0 || len(d.Params) != 1 {
		t.Errorf("DeleteUser wrong: %+v", d)
	}
}
