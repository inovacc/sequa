package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "github.com/lib/pq" // registers the "postgres" sql driver
)

// DriverName maps a DSN scheme to a database/sql driver name.
// M1 supports PostgreSQL only; other schemes return an error.
func DriverName(dsn string) (string, error) {
	switch {
	case strings.HasPrefix(dsn, "postgres://"), strings.HasPrefix(dsn, "postgresql://"):
		return "postgres", nil
	default:
		return "", fmt.Errorf("unsupported or missing DSN scheme: %q", dsn)
	}
}

// Open opens and pings a database, selecting the driver by DSN scheme.
// The caller owns the returned *sql.DB and must Close it.
func Open(ctx context.Context, dsn string) (*sql.DB, error) {
	if dsn == "" {
		return nil, fmt.Errorf("empty DSN")
	}
	driver, err := DriverName(dsn)
	if err != nil {
		return nil, err
	}
	conn, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := conn.PingContext(pingCtx); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("ping db: %w", err)
	}
	return conn, nil
}
