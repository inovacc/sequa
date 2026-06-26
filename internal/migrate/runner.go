package migrate

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	migratelib "github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"

	dbpkg "github.com/inovacc/sequa/internal/db"
)

const historyDDL = `CREATE TABLE IF NOT EXISTS sequa_schema_history (
  id         BIGSERIAL   PRIMARY KEY,
  version    BIGINT      NOT NULL,
  name       TEXT        NOT NULL,
  direction  TEXT        NOT NULL DEFAULT 'up',
  applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
);`

// Applied describes one migration that was applied (or rolled back).
type Applied struct {
	Version   uint
	Name      string
	AppliedAt time.Time
}

// Status describes a migration's applied state for `migrate status`.
type Status struct {
	Version   uint
	Name      string
	Applied   bool
	AppliedAt *time.Time
}

// Runner applies migrations from srcFS/subdir against the database at dsn.
// It opens a fresh *sql.DB per operation (the migrate driver closes it),
// so it never closes a database it does not own.
type Runner struct {
	dsn    string
	srcFS  fs.FS
	subdir string
	names  map[uint]string // version -> migration name
}

// NewRunner indexes the migration filenames in srcFS/subdir.
func NewRunner(dsn string, srcFS fs.FS, subdir string) (*Runner, error) {
	names, err := loadNames(srcFS, subdir)
	if err != nil {
		return nil, err
	}
	return &Runner{dsn: dsn, srcFS: srcFS, subdir: subdir, names: names}, nil
}

func loadNames(srcFS fs.FS, subdir string) (map[uint]string, error) {
	matches, err := fs.Glob(srcFS, path.Join(subdir, "*.up.sql"))
	if err != nil {
		return nil, fmt.Errorf("glob migrations: %w", err)
	}
	names := make(map[uint]string, len(matches))
	for _, f := range matches {
		v, name, err := parseFilename(path.Base(f))
		if err != nil {
			continue
		}
		names[v] = name
	}
	return names, nil
}

func parseFilename(base string) (uint, string, error) {
	trimmed := strings.TrimSuffix(base, ".up.sql")
	idx := strings.IndexByte(trimmed, '_')
	if idx <= 0 {
		return 0, "", fmt.Errorf("bad migration filename %q", base)
	}
	v, err := strconv.ParseUint(trimmed[:idx], 10, 64)
	if err != nil {
		return 0, "", fmt.Errorf("bad version in %q: %w", base, err)
	}
	return uint(v), trimmed[idx+1:], nil
}

func sortedVersions(names map[uint]string) []uint {
	out := make([]uint, 0, len(names))
	for v := range names {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// withMigrate opens a db + migrate instance, runs fn, then closes both via
// m.Close() (which closes the source and the per-op db). The db is created
// here, so closing it is safe.
func (r *Runner) withMigrate(ctx context.Context, fn func(db *sql.DB, m *migratelib.Migrate) error) error {
	conn, err := dbpkg.Open(ctx, r.dsn)
	if err != nil {
		return err
	}
	src, err := iofs.New(r.srcFS, r.subdir)
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("iofs source: %w", err)
	}
	drv, err := postgres.WithInstance(conn, &postgres.Config{})
	if err != nil {
		_ = src.Close()
		_ = conn.Close()
		return fmt.Errorf("postgres driver: %w", err)
	}
	m, err := migratelib.NewWithInstance("iofs", src, "postgres", drv)
	if err != nil {
		_ = src.Close()
		_ = conn.Close()
		return fmt.Errorf("migrate init: %w", err)
	}
	defer func() {
		// Closes the iofs source and the per-op db driver.
		if srcErr, dbErr := m.Close(); srcErr != nil || dbErr != nil {
			slog.Warn("migrate close", "src", srcErr, "db", dbErr)
		}
	}()
	return fn(conn, m)
}

func ensureHistory(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, historyDDL); err != nil {
		return fmt.Errorf("ensure history table: %w", err)
	}
	return nil
}

func stop(err error) bool {
	if errors.Is(err, migratelib.ErrNoChange) {
		return true
	}
	var short migratelib.ErrShortLimit
	return errors.As(err, &short)
}

// Up applies all pending migrations one step at a time, recording history.
func (r *Runner) Up(ctx context.Context) ([]Applied, error) {
	var applied []Applied
	err := r.withMigrate(ctx, func(db *sql.DB, m *migratelib.Migrate) error {
		if err := ensureHistory(ctx, db); err != nil {
			return err
		}
		for {
			if err := m.Steps(1); err != nil {
				if stop(err) {
					return nil
				}
				return fmt.Errorf("apply step: %w", err)
			}
			v, _, err := m.Version()
			if err != nil {
				return fmt.Errorf("read version: %w", err)
			}
			name := r.names[v]
			if _, err := db.ExecContext(ctx,
				`INSERT INTO sequa_schema_history (version, name, direction) VALUES ($1, $2, 'up')`,
				int64(v), name); err != nil {
				return fmt.Errorf("record history: %w", err)
			}
			applied = append(applied, Applied{Version: v, Name: name, AppliedAt: time.Now()})
		}
	})
	return applied, err
}

// Down rolls back the single most-recent migration (goose semantics).
func (r *Runner) Down(ctx context.Context) (*Applied, error) {
	var result *Applied
	err := r.withMigrate(ctx, func(db *sql.DB, m *migratelib.Migrate) error {
		if err := ensureHistory(ctx, db); err != nil {
			return err
		}
		before, _, verr := m.Version()
		if verr != nil {
			if errors.Is(verr, migratelib.ErrNilVersion) {
				return nil // nothing to roll back
			}
			return fmt.Errorf("read version: %w", verr)
		}
		if err := m.Steps(-1); err != nil {
			if stop(err) {
				return nil
			}
			return fmt.Errorf("rollback step: %w", err)
		}
		name := r.names[before]
		if _, err := db.ExecContext(ctx,
			`INSERT INTO sequa_schema_history (version, name, direction) VALUES ($1, $2, 'down')`,
			int64(before), name); err != nil {
			return fmt.Errorf("record history: %w", err)
		}
		result = &Applied{Version: before, Name: name, AppliedAt: time.Now()}
		return nil
	})
	return result, err
}

// Version returns the current schema version and dirty flag (0,false on fresh DB).
func (r *Runner) Version(ctx context.Context) (uint, bool, error) {
	var version uint
	var dirty bool
	err := r.withMigrate(ctx, func(_ *sql.DB, m *migratelib.Migrate) error {
		v, d, verr := m.Version()
		if verr != nil {
			if errors.Is(verr, migratelib.ErrNilVersion) {
				version, dirty = 0, false
				return nil
			}
			return verr
		}
		version, dirty = v, d
		return nil
	})
	return version, dirty, err
}

// Status reports every known migration with its applied state + applied_at.
func (r *Runner) Status(ctx context.Context) ([]Status, error) {
	var out []Status
	err := r.withMigrate(ctx, func(db *sql.DB, m *migratelib.Migrate) error {
		var current uint
		if v, _, verr := m.Version(); verr != nil {
			if !errors.Is(verr, migratelib.ErrNilVersion) {
				return verr
			}
			current = 0
		} else {
			current = v
		}
		hist, err := historyMap(ctx, db)
		if err != nil {
			return err
		}
		for _, v := range sortedVersions(r.names) {
			s := Status{Version: v, Name: r.names[v], Applied: current != 0 && v <= current}
			if at, ok := hist[v]; ok {
				a := at
				s.AppliedAt = &a
			}
			out = append(out, s)
		}
		return nil
	})
	return out, err
}

// historyMap returns the latest up-applied_at per version. A missing history
// table yields an empty map (history is best-effort).
func historyMap(ctx context.Context, db *sql.DB) (map[uint]time.Time, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT version, max(applied_at) FROM sequa_schema_history WHERE direction = 'up' GROUP BY version`)
	if err != nil {
		return map[uint]time.Time{}, nil // table not created yet
	}
	defer func() { _ = rows.Close() }()
	m := make(map[uint]time.Time)
	for rows.Next() {
		var v int64
		var at time.Time
		if err := rows.Scan(&v, &at); err != nil {
			return nil, fmt.Errorf("scan history: %w", err)
		}
		m[uint(v)] = at
	}
	return m, rows.Err()
}
