package codegen

import "testing"

func mustCatalog(t *testing.T, sql string) *Catalog {
	t.Helper()
	cat, err := BuildCatalog([]string{sql})
	if err != nil {
		t.Fatalf("build catalog: %v", err)
	}
	return cat
}

func TestDiffCatalogs(t *testing.T) {
	tests := []struct {
		name      string
		static    string
		live      string
		wantKinds []DiffKind
	}{
		{
			name:   "identical schemas have no drift",
			static: `CREATE TABLE t (id BIGINT PRIMARY KEY, name TEXT NOT NULL, tags TEXT[]);`,
			live:   `CREATE TABLE t (id BIGINT PRIMARY KEY, name TEXT NOT NULL, tags TEXT[]);`,
		},
		{
			name:   "bigserial normalizes to int8",
			static: `CREATE TABLE t (id BIGSERIAL PRIMARY KEY);`,
			live:   `CREATE TABLE t (id BIGINT NOT NULL);`,
		},
		{
			name:      "missing table",
			static:    `CREATE TABLE a (id BIGINT); CREATE TABLE b (id BIGINT);`,
			live:      `CREATE TABLE a (id BIGINT);`,
			wantKinds: []DiffKind{DiffTableMissing},
		},
		{
			name:      "extra table",
			static:    `CREATE TABLE a (id BIGINT);`,
			live:      `CREATE TABLE a (id BIGINT); CREATE TABLE b (id BIGINT);`,
			wantKinds: []DiffKind{DiffTableExtra},
		},
		{
			name:      "missing column",
			static:    `CREATE TABLE t (id BIGINT, name TEXT);`,
			live:      `CREATE TABLE t (id BIGINT);`,
			wantKinds: []DiffKind{DiffColumnMissing},
		},
		{
			name:      "extra column",
			static:    `CREATE TABLE t (id BIGINT);`,
			live:      `CREATE TABLE t (id BIGINT, extra TEXT);`,
			wantKinds: []DiffKind{DiffColumnExtra},
		},
		{
			name:      "type mismatch",
			static:    `CREATE TABLE t (id BIGINT);`,
			live:      `CREATE TABLE t (id INT);`,
			wantKinds: []DiffKind{DiffType},
		},
		{
			name:      "nullability mismatch",
			static:    `CREATE TABLE t (name TEXT NOT NULL);`,
			live:      `CREATE TABLE t (name TEXT);`,
			wantKinds: []DiffKind{DiffNullability},
		},
		{
			name:      "array vs scalar is a type mismatch",
			static:    `CREATE TABLE t (tags TEXT[]);`,
			live:      `CREATE TABLE t (tags TEXT);`,
			wantKinds: []DiffKind{DiffType},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diffs := DiffCatalogs(mustCatalog(t, tt.static), mustCatalog(t, tt.live))
			if len(diffs) != len(tt.wantKinds) {
				t.Fatalf("got %d diffs %v, want %d %v", len(diffs), diffs, len(tt.wantKinds), tt.wantKinds)
			}
			for i, k := range tt.wantKinds {
				if diffs[i].Kind != k {
					t.Errorf("diff[%d].Kind = %s, want %s (all=%v)", i, diffs[i].Kind, k, diffs)
				}
			}
		})
	}
}
