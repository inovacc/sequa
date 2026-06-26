package codegen

import (
	"strings"
	"testing"
)

func TestRenderQueries(t *testing.T) {
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
		t.Fatal(err)
	}

	out, err := RenderQueries(cat, qs, "db") // go/format → valid Go syntax
	if err != nil {
		t.Fatalf("RenderQueries: %v", err)
	}
	src := string(out)
	t.Logf("generated:\n%s", src)
	norm := strings.Join(strings.Fields(src), " ")

	for _, want := range []string{
		"type DBTX interface",
		"func New(db DBTX) *Queries",
		"func (q *Queries) GetUser(ctx context.Context, id int64) (User, error)",
		"func (q *Queries) ListByEmail(ctx context.Context, email string) ([]ListByEmailRow, error)",
		"func (q *Queries) CreateUser(ctx context.Context, email string, name sql.NullString) (User, error)",
		"func (q *Queries) DeleteUser(ctx context.Context, id int64) error",
		"type ListByEmailRow struct {",
		"row.Scan(&i.ID, &i.Email, &i.Name)",
	} {
		if !strings.Contains(norm, want) {
			t.Errorf("generated queries missing %q", want)
		}
	}
}
