package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/inovacc/sequa/internal/config"
	"github.com/inovacc/sequa/internal/query"
)

func newQueryCmd() *cobra.Command {
	var command string
	cmd := &cobra.Command{
		Use:   "query [dsn]",
		Short: "Interactive SQL client and REPL (powered by usql)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dsn := ""
			if len(args) == 1 {
				dsn = args[0]
			} else {
				dsn = config.ResolveDSN(flagDSN)
			}
			if dsn == "" {
				return fmt.Errorf("no DSN: pass it as an argument, --dsn, or set DATABASE_URL")
			}
			return query.Run(cmd.Context(), dsn, command)
		},
	}
	cmd.Flags().StringVarP(&command, "command", "c", "", "run a single SQL command and exit")
	return cmd
}
