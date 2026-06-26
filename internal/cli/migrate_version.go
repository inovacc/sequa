package cli

import (
	"fmt"
	"os"

	"github.com/inovacc/sequa/internal/migrate"
	"github.com/spf13/cobra"
)

func newMigrateVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the current schema version",
		RunE: func(cmd *cobra.Command, _ []string) error {
			dsn, dir, err := resolveDSNAndDir()
			if err != nil {
				return err
			}
			r, err := migrate.NewRunner(dsn, os.DirFS(dir), ".")
			if err != nil {
				return err
			}
			v, dirty, err := r.Version(cmd.Context())
			if err != nil {
				return err
			}
			if v == 0 {
				_, _ = fmt.Fprintln(os.Stdout, "no migrations applied")
				return nil
			}
			_, _ = fmt.Fprintf(os.Stdout, "version: %d (dirty=%v)\n", v, dirty)
			return nil
		},
	}
}
