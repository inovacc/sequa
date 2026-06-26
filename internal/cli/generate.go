package cli

import (
	"errors"

	"github.com/spf13/cobra"
)

func newGenerateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "generate",
		Short: "Generate type-safe Go from migration-defined schema (milestone M3)",
		RunE: func(_ *cobra.Command, _ []string) error {
			return errors.New("generate is not implemented yet (milestone M3)")
		},
	}
}
