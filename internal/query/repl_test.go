package query

import (
	"strings"
	"testing"
)

func TestRedact(t *testing.T) {
	tests := []struct {
		name string
		dsn  string
		want string
	}{
		{name: "postgres with credentials", dsn: "postgres://user:secretpass@localhost:5432/mydb", want: "postgres://…"},
		{name: "postgresql with query params", dsn: "postgresql://admin:hunter2@db.internal/app?sslmode=require", want: "postgresql://…"},
		{name: "empty dsn", dsn: "", want: "database"},
		{name: "no scheme separator", dsn: "just-a-name", want: "database"},
		{name: "empty scheme is rejected", dsn: "://nohost", want: "database"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := redact(tt.dsn); got != tt.want {
				t.Errorf("redact(%q) = %q, want %q", tt.dsn, got, tt.want)
			}
		})
	}
}

// redact must never surface credentials, host, port, or database name — only
// the scheme. This is the property that keeps DSNs out of error messages.
func TestRedactNeverLeaksSecrets(t *testing.T) {
	const dsn = "postgres://alice:topsecret@10.0.0.5:5432/prod?password=alsosecret"
	got := redact(dsn)
	for _, leak := range []string{"alice", "topsecret", "10.0.0.5", "5432", "prod", "alsosecret"} {
		if strings.Contains(got, leak) {
			t.Errorf("redact(%q) = %q leaks %q", dsn, got, leak)
		}
	}
}
