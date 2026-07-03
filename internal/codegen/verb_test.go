package codegen

import (
	"strings"
	"testing"
)

func verbCatalog(t *testing.T) *Catalog {
	t.Helper()
	cat, err := BuildCatalog([]string{
		`CREATE TABLE items (id BIGSERIAL PRIMARY KEY, name TEXT NOT NULL, deleted_at TIMESTAMPTZ);`,
	})
	if err != nil {
		t.Fatal(err)
	}
	return cat
}

// TestVerbHeaderDoesNotMerge is the direct regression for the silent-merge bug:
// an unrecognized/new verb on a later query used to fail the header regex, so its
// SQL was appended to the PREVIOUS query — surfacing a misleading "expected
// exactly one SQL statement" against the wrong query. The header must now be
// recognized so the two queries stay separate.
func TestVerbHeaderDoesNotMerge(t *testing.T) {
	cat := verbCatalog(t)
	src := `-- name: ListItems :many
SELECT id, name FROM items;

-- name: SoftDelete :execrows
UPDATE items SET deleted_at = now() WHERE id = $1;
`
	qs, err := AnalyzeQueries(cat, src)
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	if len(qs) != 2 {
		t.Fatalf("got %d queries, want 2 (the :execrows header must not merge into the previous query)", len(qs))
	}
	if qs[1].Name != "SoftDelete" || qs[1].Cmd != CmdExecRows {
		t.Fatalf("q[1] = %q %q, want SoftDelete :execrows", qs[1].Name, qs[1].Cmd)
	}
}

func TestExecRowsRender(t *testing.T) {
	cat := verbCatalog(t)
	qs, err := AnalyzeQueries(cat, "-- name: SoftDelete :execrows\nUPDATE items SET deleted_at = now() WHERE id = $1;\n")
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	if len(qs[0].Columns) != 0 || qs[0].Star {
		t.Errorf(":execrows should scan no result columns, got columns=%v star=%v", qs[0].Columns, qs[0].Star)
	}
	out, err := RenderQueries(cat, qs, "db")
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	src := string(out)
	if !strings.Contains(src, "func (q *Queries) SoftDelete(ctx context.Context, id int64) (int64, error)") {
		t.Errorf(":execrows signature missing:\n%s", src)
	}
	if !strings.Contains(src, "return result.RowsAffected()") {
		t.Errorf(":execrows body should return RowsAffected():\n%s", src)
	}
}

func TestExecResultRender(t *testing.T) {
	cat := verbCatalog(t)
	qs, err := AnalyzeQueries(cat, "-- name: Rename :execresult\nUPDATE items SET name = $1 WHERE id = $2;\n")
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	if qs[0].Cmd != CmdExecResult {
		t.Fatalf("cmd = %q, want :execresult", qs[0].Cmd)
	}
	out, err := RenderQueries(cat, qs, "db")
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(string(out), "(sql.Result, error)") {
		t.Errorf(":execresult signature missing:\n%s", out)
	}
}

func TestRejectedAndUnknownVerbs(t *testing.T) {
	cat := verbCatalog(t)
	cases := []struct {
		name, src, want string
	}{
		{"unknown", "-- name: X :bogus\nSELECT id FROM items;\n", "unknown command"},
		{"copyfrom", "-- name: X :copyfrom\nINSERT INTO items (name) VALUES ($1);\n", "requires the pgx driver"},
		{"batchexec", "-- name: X :batchexec\nINSERT INTO items (name) VALUES ($1);\n", "requires the pgx driver"},
		{"execlastid", "-- name: X :execlastid\nINSERT INTO items (name) VALUES ($1);\n", "LastInsertId"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := AnalyzeQueries(cat, tc.src)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want error containing %q, got %v", tc.want, err)
			}
		})
	}
}

// TestExecRowsParamCollision guards the identifier-collision defect the review
// found: the :execrows body uses a local named `result`, so a parameter bound to
// a column named `result` must be renamed (result2) or the generated code passes
// gofmt but fails to compile.
func TestExecRowsParamCollision(t *testing.T) {
	cat, err := BuildCatalog([]string{
		`CREATE TABLE jobs (id BIGSERIAL PRIMARY KEY, result TEXT NOT NULL, done BOOL NOT NULL);`,
	})
	if err != nil {
		t.Fatal(err)
	}
	qs, err := AnalyzeQueries(cat, "-- name: ClearResult :execrows\nUPDATE jobs SET done = $1 WHERE result = $2;\n")
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	out, err := RenderQueries(cat, qs, "db")
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	src := string(out)
	// The param bound to column "result" must not be emitted as the bare local.
	if strings.Contains(src, ", result string") {
		t.Errorf("param named 'result' collides with the :execrows local; must be renamed:\n%s", src)
	}
	if !strings.Contains(src, "result2 string") {
		t.Errorf("expected the colliding param renamed to result2:\n%s", src)
	}
}

// TestProseNameCommentStaysSQL guards the malformed-header over-broadness the
// review found: a comment that merely starts with "-- name:" but is not a header
// must be treated as SQL, not hard-error the whole file.
func TestProseNameCommentStaysSQL(t *testing.T) {
	cat := verbCatalog(t)
	src := "-- name: GetItem :one\n-- name: lookup is by primary key\nSELECT id, name FROM items WHERE id = $1;\n"
	if _, err := AnalyzeQueries(cat, src); err != nil {
		t.Fatalf("a prose -- name: comment inside a query should not error: %v", err)
	}
}

// TestSpacedColonHeaderIsMalformed guards the malformed-header under-narrowness:
// a header typo with a displaced colon must fail loudly, not silently merge.
func TestSpacedColonHeaderIsMalformed(t *testing.T) {
	cat := verbCatalog(t)
	_, err := AnalyzeQueries(cat, "-- name : GetItem :one\nSELECT id FROM items WHERE id = $1;\n")
	if err == nil || !strings.Contains(err.Error(), "malformed query header") {
		t.Fatalf("spaced-colon header typo should be a malformed-header error, got %v", err)
	}
}

// TestReturningOneStillWorks locks in the already-supported RETURNING path so the
// verb changes don't regress it (the census flagged RETURNING as a "blocker" but
// it needs no new code — only this guard).
func TestReturningOneStillWorks(t *testing.T) {
	cat := verbCatalog(t)
	qs, err := AnalyzeQueries(cat, "-- name: CreateItem :one\nINSERT INTO items (name) VALUES ($1) RETURNING *;\n")
	if err != nil {
		t.Fatalf("RETURNING * :one should be supported: %v", err)
	}
	if !qs[0].Star || qs[0].Table != "items" {
		t.Errorf("RETURNING * should set Star + Table=items, got star=%v table=%q", qs[0].Star, qs[0].Table)
	}
	out, err := RenderQueries(cat, qs, "db")
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(string(out), "func (q *Queries) CreateItem(ctx context.Context, name string) (") {
		t.Errorf("RETURNING * :one should return the table model:\n%s", out)
	}
}
