package cli

import (
	"fmt"
	"os"

	"github.com/inovacc/sequa/internal/migrate"
	"github.com/spf13/cobra"
)

func newMigrateDownCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "down",
		Short: "Roll back the most recent migration",
		RunE: func(cmd *cobra.Command, _ []string) error {
			dsn, dir, err := resolveDSNAndDir()
			if err != nil {
				return err
			}
			r, err := migrate.NewRunner(dsn, os.DirFS(dir), ".")
			if err != nil {
				return err
			}
			down, err := r.Down(cmd.Context())
			if err != nil {
				return err
			}
			if down == nil {
				_, _ = fmt.Fprintln(os.Stdout, "nothing to roll back")
				return nil
			}
			_, _ = fmt.Fprintf(os.Stdout, "DOWN %d_%s\n", down.Version, down.Name)
			return nil
		},
	}
}
