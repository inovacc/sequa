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
		v, name, err := ParseFilename(path.Base(f))
		if err != nil {
			continue
		}
		names[uint(v)] = name
	}
	return names, nil
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
	// The iofs source signals "no more migrations" with fs.ErrNotExist when
	// Steps reads past the last migration; ErrNoChange / ErrShortLimit are the
	// other terminators depending on source/driver. Treat all three as "done".
	if errors.Is(err, migratelib.ErrNoChange) || errors.Is(err, fs.ErrNotExist) {
		return true
	}
	var short migratelib.ErrShortLimit
	return errors.As(err, &short)
}

// currentVersion returns migrate's current schema version, mapping the
// fresh-database ErrNilVersion to 0.
func currentVersion(m *migratelib.Migrate) (uint, error) {
	v, _, err := m.Version()
	if err != nil {
		if errors.Is(err, migratelib.ErrNilVersion) {
			return 0, nil
		}
		return 0, err
	}
	return v, nil
}

// missingVersions returns the known versions <= current with no history row
// recorded, in ascending order (known must already be sorted ascending). These
// are the applied migrations whose history was lost and must be backfilled.
func missingVersions(known []uint, recorded map[uint]bool, current uint) []uint {
	var out []uint
	for _, v := range known {
		if v <= current && !recorded[v] {
			out = append(out, v)
		}
	}
	return out
}

// reconcileHistory backfills history rows for migrations that migrate has
// applied but whose sequa_schema_history 'up' record was lost — e.g. a crash
// between a migrate step's commit and the follow-up history INSERT. migrate is
// linear, so every known version <= current is applied; any such version with
// no history row at all is re-recorded. applied_at defaults to the reconcile
// time because the original instant was never durably captured.
func (r *Runner) reconcileHistory(ctx context.Context, db *sql.DB, current uint) error {
	if current == 0 {
		return nil
	}
	recorded, err := recordedVersions(ctx, db)
	if err != nil {
		return err
	}
	for _, v := range missingVersions(sortedVersions(r.names), recorded, current) {
		if _, err := db.ExecContext(ctx,
			`INSERT INTO sequa_schema_history (version, name, direction) VALUES ($1, $2, 'up')`,
			int64(v), r.names[v]); err != nil {
			return fmt.Errorf("backfill history for version %d: %w", v, err)
		}
	}
	return nil
}

// recordedVersions is the set of versions that already have any history row.
// The rows are fully drained and closed before the caller issues follow-up
// writes on the same pool.
func recordedVersions(ctx context.Context, db *sql.DB) (map[uint]bool, error) {
	rows, err := db.QueryContext(ctx, `SELECT DISTINCT version FROM sequa_schema_history`)
	if err != nil {
		return nil, fmt.Errorf("read history versions: %w", err)
	}
	defer func() { _ = rows.Close() }()
	recorded := make(map[uint]bool)
	for rows.Next() {
		var v int64
		if err := rows.Scan(&v); err != nil {
			return nil, fmt.Errorf("scan history version: %w", err)
		}
		recorded[uint(v)] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate history versions: %w", err)
	}
	return recorded, nil
}

// prepare ensures the history table exists and heals any history rows lost to a
// crash, returning migrate's current version. Both Up and Down call it before
// changing state.
func (r *Runner) prepare(ctx context.Context, db *sql.DB, m *migratelib.Migrate) (uint, error) {
	if err := ensureHistory(ctx, db); err != nil {
		return 0, err
	}
	current, err := currentVersion(m)
	if err != nil {
		return 0, fmt.Errorf("read version: %w", err)
	}
	if err := r.reconcileHistory(ctx, db, current); err != nil {
		return 0, err
	}
	return current, nil
}

// recordApplied reads the version migrate just advanced to and writes its 'up'
// history row.
func (r *Runner) recordApplied(ctx context.Context, db *sql.DB, m *migratelib.Migrate) (Applied, error) {
	v, _, err := m.Version()
	if err != nil {
		return Applied{}, fmt.Errorf("read version: %w", err)
	}
	name := r.names[v]
	if _, err := db.ExecContext(ctx,
		`INSERT INTO sequa_schema_history (version, name, direction) VALUES ($1, $2, 'up')`,
		int64(v), name); err != nil {
		return Applied{}, fmt.Errorf("record history: %w", err)
	}
	return Applied{Version: v, Name: name, AppliedAt: time.Now()}, nil
}

// Up applies all pending migrations one step at a time, recording history.
func (r *Runner) Up(ctx context.Context) ([]Applied, error) {
	var applied []Applied
	err := r.withMigrate(ctx, func(db *sql.DB, m *migratelib.Migrate) error {
		if _, err := r.prepare(ctx, db, m); err != nil {
			return err
		}
		for {
			if err := m.Steps(1); err != nil {
				if stop(err) {
					return nil
				}
				return fmt.Errorf("apply step: %w", err)
			}
			a, err := r.recordApplied(ctx, db, m)
			if err != nil {
				return err
			}
			applied = append(applied, a)
		}
	})
	return applied, err
}

// Down rolls back the single most-recent migration (goose semantics).
func (r *Runner) Down(ctx context.Context) (*Applied, error) {
	var result *Applied
	err := r.withMigrate(ctx, func(db *sql.DB, m *migratelib.Migrate) error {
		before, err := r.prepare(ctx, db, m)
		if err != nil {
			return err
		}
		if before == 0 {
			return nil // nothing to roll back
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
		current, err := currentVersion(m)
		if err != nil {
			return err
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
