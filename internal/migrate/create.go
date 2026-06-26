package migrate

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	upTemplate   = "-- Migration: %s (up)\n-- Write the forward SQL here.\n"
	downTemplate = "-- Migration: %s (down)\n-- Write the rollback SQL here.\n"
)

var nonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

// Slugify lower-cases name and collapses non-alphanumerics to single underscores.
func Slugify(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = nonAlnum.ReplaceAllString(s, "_")
	return strings.Trim(s, "_")
}

// TimestampVersion formats t (UTC) as YYYYMMDDHHMMSS.
func TimestampVersion(t time.Time) string {
	return t.UTC().Format("20060102150405")
}

// NextSequential returns the next zero-padded 5-digit version for dir.
func NextSequential(dir string) (string, error) {
	max, err := maxVersion(dir)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%05d", max+1), nil
}

func maxVersion(dir string) (uint64, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("read dir: %w", err)
	}
	var max uint64
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".up.sql") {
			continue
		}
		trimmed := strings.TrimSuffix(e.Name(), ".up.sql")
		idx := strings.IndexByte(trimmed, '_')
		if idx <= 0 {
			continue
		}
		v, err := strconv.ParseUint(trimmed[:idx], 10, 64)
		if err != nil {
			continue
		}
		if v > max {
			max = v
		}
	}
	return max, nil
}

// Create writes <version>_<slug>.up.sql and .down.sql in dir, returning [up, down].
func Create(dir, name string, sequential bool, now time.Time) ([]string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create dir: %w", err)
	}
	slug := Slugify(name)
	if slug == "" {
		return nil, fmt.Errorf("invalid migration name %q", name)
	}
	var version string
	if sequential {
		v, err := NextSequential(dir)
		if err != nil {
			return nil, err
		}
		version = v
	} else {
		version = TimestampVersion(now)
	}
	stem := version + "_" + slug
	upPath := filepath.Join(dir, stem+".up.sql")
	downPath := filepath.Join(dir, stem+".down.sql")
	if err := os.WriteFile(upPath, []byte(fmt.Sprintf(upTemplate, slug)), 0o644); err != nil {
		return nil, fmt.Errorf("write up: %w", err)
	}
	if err := os.WriteFile(downPath, []byte(fmt.Sprintf(downTemplate, slug)), 0o644); err != nil {
		return nil, fmt.Errorf("write down: %w", err)
	}
	return []string{upPath, downPath}, nil
}
