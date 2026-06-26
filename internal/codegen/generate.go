package codegen

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
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
		blk := cfg.SQL[i]
		switch blk.Engine {
		case "postgresql", "postgres", "":
		default:
			return nil, fmt.Errorf("sql[%d]: unsupported engine %q (only postgresql in M3)", i, blk.Engine)
		}

		migrations, err := readUpMigrations(filepath.Join(root, blk.Schema))
		if err != nil {
			return nil, err
		}
		cat, err := BuildCatalog(migrations)
		if err != nil {
			return nil, err
		}
		models, err := RenderModels(cat, blk.Gen.Go.Package)
		if err != nil {
			return nil, err
		}
		files = append(files, GeneratedFile{
			Path:    filepath.Join(root, blk.Gen.Go.Out, "models.go"),
			Content: models,
		})
	}
	return files, nil
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
		migs = append(migs, mig{version: leadingVersion(e.Name()), path: filepath.Join(dir, e.Name())})
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

func leadingVersion(name string) uint64 {
	if i := strings.IndexByte(name, '_'); i > 0 {
		v, _ := strconv.ParseUint(name[:i], 10, 64)
		return v
	}
	return 0
}
