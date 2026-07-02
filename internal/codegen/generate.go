package codegen

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/inovacc/sequa/internal/migrate"
)

// GeneratedFile is a file the generator produced; the caller writes it to disk.
type GeneratedFile struct {
	Path    string
	Content []byte
}

// Generate runs codegen for every sql block in cfg. Relative paths in cfg are
// resolved against root (the directory containing sequa.yaml). M3 emits the
// models file; query methods are added by a later increment.
func Generate(cfg *Config, root string) ([]GeneratedFile, error) {
	var files []GeneratedFile
	for i := range cfg.SQL {
		blockFiles, err := generateBlock(cfg.SQL[i], root, i)
		if err != nil {
			return nil, err
		}
		files = append(files, blockFiles...)
	}
	return files, nil
}

// generateBlock renders the models file (always) and the queries file (when the
// block sets a queries path) for one sql block.
func generateBlock(blk SQLBlock, root string, i int) ([]GeneratedFile, error) {
	eng, err := engineFor(blk.Engine)
	if err != nil {
		return nil, fmt.Errorf("sql[%d]: %w", i, err)
	}

	migrations, err := readUpMigrations(filepath.Join(root, blk.Schema))
	if err != nil {
		return nil, err
	}
	cat, err := eng.BuildCatalog(migrations)
	if err != nil {
		return nil, err
	}
	models, err := RenderModels(cat, blk.Gen.Go.Package)
	if err != nil {
		return nil, err
	}
	files := []GeneratedFile{{
		Path:    filepath.Join(root, blk.Gen.Go.Out, "models.go"),
		Content: models,
	}}

	if strings.TrimSpace(blk.Queries) == "" {
		return files, nil
	}
	content, err := readQueries(filepath.Join(root, blk.Queries))
	if err != nil {
		return nil, err
	}
	queries, err := eng.AnalyzeQueries(cat, content)
	if err != nil {
		return nil, fmt.Errorf("sql[%d] queries: %w", i, err)
	}
	qsrc, err := RenderQueries(cat, queries, blk.Gen.Go.Package)
	if err != nil {
		return nil, err
	}
	return append(files, GeneratedFile{
		Path:    filepath.Join(root, blk.Gen.Go.Out, "queries.go"),
		Content: qsrc,
	}), nil
}

// readQueries reads a single .sql file, or concatenates every *.sql file in a
// directory (sorted by name).
func readQueries(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("read queries %s: %w", path, err)
	}
	if !info.IsDir() {
		data, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("read queries %s: %w", path, err)
		}
		return string(data), nil
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return "", fmt.Errorf("read queries dir %s: %w", path, err)
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	var b strings.Builder
	for _, n := range names {
		data, err := os.ReadFile(filepath.Join(path, n))
		if err != nil {
			return "", err
		}
		b.Write(data)
		b.WriteString("\n")
	}
	return b.String(), nil
}

// readUpMigrations reads every *.up.sql file from dir in ascending version order.
func readUpMigrations(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read schema dir %s: %w", dir, err)
	}
	type mig struct {
		version uint64
		path    string
	}
	var migs []mig
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".up.sql") {
			continue
		}
		v, _, err := migrate.ParseFilename(e.Name())
		if err != nil {
			return nil, fmt.Errorf("schema dir %s: %w", dir, err)
		}
		migs = append(migs, mig{version: v, path: filepath.Join(dir, e.Name())})
	}
	sort.Slice(migs, func(i, j int) bool { return migs[i].version < migs[j].version })

	out := make([]string, 0, len(migs))
	for _, m := range migs {
		data, err := os.ReadFile(m.path)
		if err != nil {
			return nil, fmt.Errorf("read migration %s: %w", m.path, err)
		}
		out = append(out, string(data))
	}
	return out, nil
}

// CatalogFromMigrations builds the schema catalog by parsing every up-migration
// in dir (ascending version order) — the static half of `sequa verify`.
func CatalogFromMigrations(dir string) (*Catalog, error) {
	migrations, err := readUpMigrations(dir)
	if err != nil {
		return nil, err
	}
	return BuildCatalog(migrations)
}
