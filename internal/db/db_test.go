package db

import (
	"context"
	"os"
	"strings"
	"testing"
)

func TestDriverName(t *testing.T) {
	cases := []struct {
		dsn     string
		want    string
		wantErr bool
	}{
		{"postgres://u:p@localhost/db", "postgres", false},
		{"postgresql://u:p@localhost/db", "postgres", false},
		{"mysql://u:p@localhost/db", "", true},
		{"", "", true},
	}
	for _, c := range cases {
		got, err := DriverName(c.dsn)
		if (err != nil) != c.wantErr || got != c.want {
			t.Errorf("DriverName(%q)=(%q,%v) want (%q,err=%v)", c.dsn, got, err, c.want, c.wantErr)
		}
	}
}

func TestDriverNameRedactsCredentials(t *testing.T) {
	_, err := DriverName("mysql://user:supersecret@localhost/db")
	if err == nil {
		t.Fatal("expected error for unsupported scheme")
	}
	if strings.Contains(err.Error(), "supersecret") {
		t.Errorf("error leaked DSN credentials: %v", err)
	}
}

func TestOpenIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB integration test in -short mode")
	}
	dsn := os.Getenv("SEQUA_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set SEQUA_TEST_DATABASE_URL to run DB integration tests")
	}
	conn, err := Open(context.Background(), dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = conn.Close() }()
	if err := conn.PingContext(context.Background()); err != nil {
		t.Fatalf("ping: %v", err)
	}
}
