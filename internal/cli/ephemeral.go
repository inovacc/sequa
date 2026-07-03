package cli

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/url"
	"os"

	"github.com/inovacc/sequa/internal/codegen"
	dbpkg "github.com/inovacc/sequa/internal/db"
	"github.com/inovacc/sequa/internal/migrate"
)

// ephemeralIntrospect creates a throwaway database on the same server as dsn,
// applies the up-migrations in dir to it, introspects the result, and drops the
// database. It gives `verify --ephemeral` a clean target without a pre-migrated
// database. Requires the CREATEDB privilege.
func ephemeralIntrospect(ctx context.Context, dsn, dir string) (*codegen.Catalog, error) {
	name, err := ephemeralDBName()
	if err != nil {
		return nil, err
	}
	maintDSN, err := withDatabase(dsn, "postgres")
	if err != nil {
		return nil, err
	}
	maint, err := dbpkg.Open(ctx, maintDSN)
	if err != nil {
		return nil, fmt.Errorf("connect maintenance database: %w", err)
	}
	defer func() { _ = maint.Close() }()

	if _, err := maint.ExecContext(ctx, `CREATE DATABASE "`+name+`"`); err != nil {
		return nil, fmt.Errorf("create ephemeral database: %w", err)
	}
	defer dropDatabase(ctx, maint, name)

	ephDSN, err := withDatabase(dsn, name)
	if err != nil {
		return nil, err
	}
	runner, err := migrate.NewRunner(ephDSN, os.DirFS(dir), ".")
	if err != nil {
		return nil, err
	}
	if _, err := runner.Up(ctx); err != nil {
		return nil, fmt.Errorf("apply migrations to ephemeral database: %w", err)
	}

	eph, err := dbpkg.Open(ctx, ephDSN)
	if err != nil {
		return nil, err
	}
	live, err := codegen.Introspect(ctx, eph)
	_ = eph.Close() // close before the deferred DROP; Postgres refuses to drop a db in use
	if err != nil {
		return nil, err
	}
	return live, nil
}

// dropDatabase terminates lingering connections to name and drops it. It
// detaches cancellation from ctx so cleanup still runs if the parent context is
// already done. Errors are logged rather than returned — the caller already has
// its result.
func dropDatabase(ctx context.Context, maint *sql.DB, name string) {
	ctx = context.WithoutCancel(ctx)
	_, _ = maint.ExecContext(ctx,
		`SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = $1 AND pid <> pg_backend_pid()`, name)
	if _, err := maint.ExecContext(ctx, `DROP DATABASE IF EXISTS "`+name+`"`); err != nil {
		slog.Warn("drop ephemeral database", "name", name, "err", err)
	}
}

// ephemeralDBName returns a collision-resistant temporary database name.
func ephemeralDBName() (string, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("random database name: %w", err)
	}
	return "sequa_verify_" + hex.EncodeToString(b[:]), nil
}

// withDatabase returns dsn with its database (the URL path) replaced by dbname.
func withDatabase(dsn, dbname string) (string, error) {
	u, err := url.Parse(dsn)
	if err != nil {
		return "", fmt.Errorf("parse DSN: %w", err)
	}
	u.Path = "/" + dbname
	return u.String(), nil
}
