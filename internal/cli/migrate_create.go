package cli

import (
	"fmt"
	"os"
	"time"

	"github.com/inovacc/sequa/internal/config"
	"github.com/inovacc/sequa/internal/migrate"
	"github.com/spf13/cobra"
)

func newMigrateCreateCmd() *cobra.Command {
	var seq bool
	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a new up/down migration pair",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			dir, err := config.ResolveDir(flagDir, ".")
			if err != nil {
				dir = "migrations" // default target when none detected yet
			}
			paths, err := migrate.Create(dir, args[0], seq, time.Now())
			if err != nil {
				return err
			}
			for _, p := range paths {
				_, _ = fmt.Fprintln(os.Stdout, "Created", p)
			}
			return nil
		},
	}
	cmd.Flags().BoolVarP(&seq, "sequential", "s", false, "sequential numbering instead of timestamp")
	return cmd
}
