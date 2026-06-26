package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/inovacc/sequa/internal/codegen"
)

func newGenerateCmd() *cobra.Command {
	var cfgPath string
	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate type-safe Go from the migration-defined schema",
		RunE: func(_ *cobra.Command, _ []string) error {
			cfg, err := codegen.LoadConfig(cfgPath)
			if err != nil {
				return err
			}
			files, err := codegen.Generate(cfg, filepath.Dir(cfgPath))
			if err != nil {
				return err
			}
			for _, f := range files {
				if err := os.MkdirAll(filepath.Dir(f.Path), 0o755); err != nil {
					return err
				}
				if err := os.WriteFile(f.Path, f.Content, 0o644); err != nil {
					return fmt.Errorf("write %s: %w", f.Path, err)
				}
				_, _ = fmt.Fprintln(os.Stdout, "generated", f.Path)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&cfgPath, "config", "sequa.yaml", "path to the sequa.yaml config")
	return cmd
}
