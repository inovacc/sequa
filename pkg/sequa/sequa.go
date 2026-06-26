// Package sequa provides an embeddable migration runner so a Go application
// can apply its own migrations on startup.
package sequa

import (
	"context"
	"io/fs"

	imigrate "github.com/inovacc/sequa/internal/migrate"
)

// Migrator runs embedded migrations against the database identified by a DSN.
type Migrator struct {
	r *imigrate.Runner
}

// New builds a Migrator from a DSN and a filesystem (e.g. embed.FS) whose
// subdir holds <version>_<name>.up.sql / .down.sql pairs.
func New(dsn string, fsys fs.FS, subdir string) (*Migrator, error) {
	r, err := imigrate.NewRunner(dsn, fsys, subdir)
	if err != nil {
		return nil, err
	}
	return &Migrator{r: r}, nil
}

// Up applies all pending migrations.
func (m *Migrator) Up(ctx context.Context) error {
	_, err := m.r.Up(ctx)
	return err
}

// Down rolls back the most recent migration.
func (m *Migrator) Down(ctx context.Context) error {
	_, err := m.r.Down(ctx)
	return err
}

// Version returns the current schema version and dirty flag.
func (m *Migrator) Version(ctx context.Context) (uint, bool, error) {
	return m.r.Version(ctx)
}
