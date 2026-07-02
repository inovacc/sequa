package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func newMigrateUpCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "up",
		Short: "Apply all pending migrations",
		RunE: func(cmd *cobra.Command, _ []string) error {
			r, err := newRunnerFromFlags()
			if err != nil {
				return err
			}
			applied, err := r.Up(cmd.Context())
			if err != nil {
				return err
			}
			if len(applied) == 0 {
				_, _ = fmt.Fprintln(os.Stdout, "no migrations to run")
			}
			for _, a := range applied {
				_, _ = fmt.Fprintf(os.Stdout, "OK   %d_%s\n", a.Version, a.Name)
			}
			return nil
		},
	}
}
