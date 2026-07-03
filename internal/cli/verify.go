package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/inovacc/sequa/internal/codegen"
	"github.com/inovacc/sequa/internal/config"
	dbpkg "github.com/inovacc/sequa/internal/db"
)

func newVerifyCmd() *cobra.Command {
	var ephemeral bool
	cmd := &cobra.Command{
		Use:   "verify",
		Short: "Check the live database schema against the migration-defined schema",
		Long: "verify parses the up-migrations into a schema catalog, introspects the " +
			"database, and reports any drift between them. It exits non-zero when the " +
			"schema differs from what the migrations define. With --ephemeral it creates " +
			"a throwaway database, applies the migrations, and verifies against that.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			dsn := config.ResolveDSN(flagDSN)
			if dsn == "" {
				return fmt.Errorf("no DSN: pass --dsn or set DATABASE_URL")
			}
			dir, err := config.ResolveDir(flagDir, ".")
			if err != nil {
				return err
			}
			static, err := codegen.CatalogFromMigrations(dir)
			if err != nil {
				return err
			}
			live, err := verifyLive(cmd.Context(), dsn, dir, ephemeral)
			if err != nil {
				return err
			}
			return reportDrift(static, live)
		},
	}
	cmd.Flags().BoolVar(&ephemeral, "ephemeral", false,
		"create a throwaway database, apply the migrations, and verify against it (requires CREATEDB)")
	return cmd
}

// verifyLive returns the live schema catalog to compare against: the ephemeral
// path builds a throwaway database from the migrations; otherwise it introspects
// the database at dsn directly.
func verifyLive(ctx context.Context, dsn, dir string, ephemeral bool) (*codegen.Catalog, error) {
	if ephemeral {
		return ephemeralIntrospect(ctx, dsn, dir)
	}
	db, err := dbpkg.Open(ctx, dsn)
	if err != nil {
		return nil, err
	}
	defer func() { _ = db.Close() }()
	return codegen.Introspect(ctx, db)
}

// reportDrift prints the diff between the migration-defined and live catalogs
// and returns a non-nil error when they differ.
func reportDrift(static, live *codegen.Catalog) error {
	diffs := codegen.DiffCatalogs(static, live)
	if len(diffs) == 0 {
		_, _ = fmt.Fprintln(os.Stdout, "OK: live schema matches the migrations")
		return nil
	}
	for _, d := range diffs {
		_, _ = fmt.Fprintln(os.Stdout, "DRIFT", d.String())
	}
	return fmt.Errorf("%d schema difference(s) found", len(diffs))
}
