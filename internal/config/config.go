package config

import (
	"errors"
	"os"
	"path/filepath"
)

// ErrNoMigrationsDir is returned when no candidate migrations directory is found.
var ErrNoMigrationsDir = errors.New("no migrations directory found")

// Candidates are the conventional locations scanned during autodetect.
var Candidates = []string{"migrations", "db/migrations", "sql/migrations", "database/migrations"}

// ResolveDSN returns flagDSN when non-empty, else $DATABASE_URL.
func ResolveDSN(flagDSN string) string {
	if flagDSN != "" {
		return flagDSN
	}
	return os.Getenv("DATABASE_URL")
}

// AutodetectDir returns the first candidate dir under root that exists and
// contains at least one *.up.sql file. Returns ErrNoMigrationsDir otherwise.
func AutodetectDir(root string) (string, error) {
	for _, c := range Candidates {
		p := filepath.Join(root, c)
		info, err := os.Stat(p)
		if err != nil || !info.IsDir() {
			continue
		}
		matches, _ := filepath.Glob(filepath.Join(p, "*.up.sql"))
		if len(matches) > 0 {
			return p, nil
		}
	}
	return "", ErrNoMigrationsDir
}

// ResolveDir returns flagDir when non-empty, else autodetects under root.
func ResolveDir(flagDir, root string) (string, error) {
	if flagDir != "" {
		return flagDir, nil
	}
	return AutodetectDir(root)
}
