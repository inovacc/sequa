package cli

import (
	"fmt"
	"os"

	"github.com/inovacc/sequa/internal/migrate"
	"github.com/spf13/cobra"
)

func newMigrateStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show applied and pending migrations",
		RunE: func(cmd *cobra.Command, _ []string) error {
			dsn, dir, err := resolveDSNAndDir()
			if err != nil {
				return err
			}
			r, err := migrate.NewRunner(dsn, os.DirFS(dir), ".")
			if err != nil {
				return err
			}
			rows, err := r.Status(cmd.Context())
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintln(os.Stdout, "    Applied At                  Migration")
			_, _ = fmt.Fprintln(os.Stdout, "    =======================================")
			for _, s := range rows {
				when := "Pending"
				switch {
				case s.AppliedAt != nil:
					when = s.AppliedAt.Format("Mon Jan _2 15:04:05 2006")
				case s.Applied:
					when = "applied"
				}
				_, _ = fmt.Fprintf(os.Stdout, "    %-27s %d_%s\n", when, s.Version, s.Name)
			}
			return nil
		},
	}
}
