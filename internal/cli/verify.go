package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/inovacc/sequa/internal/codegen"
	"github.com/inovacc/sequa/internal/config"
	dbpkg "github.com/inovacc/sequa/internal/db"
)

func newVerifyCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "verify",
		Short: "Check the live database schema against the migration-defined schema",
		Long: "verify parses the up-migrations into a schema catalog, introspects the " +
			"live database, and reports any drift between them. It exits non-zero when " +
			"the live schema differs from what the migrations define.",
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
			db, err := dbpkg.Open(cmd.Context(), dsn)
			if err != nil {
				return err
			}
			defer func() { _ = db.Close() }()
			live, err := codegen.Introspect(cmd.Context(), db)
			if err != nil {
				return err
			}
			diffs := codegen.DiffCatalogs(static, live)
			if len(diffs) == 0 {
				_, _ = fmt.Fprintln(os.Stdout, "OK: live schema matches the migrations")
				return nil
			}
			for _, d := range diffs {
				_, _ = fmt.Fprintln(os.Stdout, "DRIFT", d.String())
			}
			return fmt.Errorf("%d schema difference(s) found", len(diffs))
		},
	}
}
