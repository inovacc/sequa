package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

func newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Scaffold a migrations directory",
		RunE: func(_ *cobra.Command, _ []string) error {
			dir := flagDir
			if dir == "" {
				dir = "migrations"
			}
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return fmt.Errorf("create %s: %w", dir, err)
			}
			abs, _ := filepath.Abs(dir)
			fmt.Fprintln(os.Stdout, "initialized migrations directory:", abs)
			return nil
		},
	}
}
