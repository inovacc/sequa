package cli

import (
	"errors"

	"github.com/spf13/cobra"
)

func newQueryCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "query",
		Short: "Interactive SQL client and REPL (milestone M2)",
		RunE: func(_ *cobra.Command, _ []string) error {
			return errors.New("query is not implemented yet (milestone M2)")
		},
	}
}
