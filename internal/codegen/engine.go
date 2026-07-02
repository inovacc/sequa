package codegen

import "fmt"

// Engine is a database-specific codegen backend: it parses migration DDL into a
// schema Catalog and types annotated queries against it. Rendering (models and
// query methods) is engine-agnostic and shared. Only PostgreSQL is implemented
// today; MySQL and SQLite are planned — see docs/specs/M5-engines.md.
type Engine interface {
	// Name is the sequa.yaml engine identifier.
	Name() string
	// BuildCatalog assembles the schema from the ordered up-migrations.
	BuildCatalog(migrations []string) (*Catalog, error)
	// AnalyzeQueries types each annotated query in content against the catalog.
	AnalyzeQueries(cat *Catalog, content string) ([]Query, error)
}

// engineFor returns the codegen engine for a sequa.yaml engine name. An empty
// name defaults to PostgreSQL.
func engineFor(name string) (Engine, error) {
	switch name {
	case "postgresql", "postgres", "":
		return postgresEngine{}, nil
	default:
		return nil, fmt.Errorf("unsupported engine %q (only postgresql is implemented; MySQL/SQLite are planned in M5)", name)
	}
}

// postgresEngine is the PostgreSQL codegen backend, built on pg_query_go.
type postgresEngine struct{}

func (postgresEngine) Name() string { return "postgresql" }

func (postgresEngine) BuildCatalog(migrations []string) (*Catalog, error) {
	return BuildCatalog(migrations)
}

func (postgresEngine) AnalyzeQueries(cat *Catalog, content string) ([]Query, error) {
	return AnalyzeQueries(cat, content)
}

var _ Engine = postgresEngine{}
