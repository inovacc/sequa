package cli

import (
	"fmt"
	"os"

	"github.com/inovacc/sequa/internal/config"
	"github.com/inovacc/sequa/internal/migrate"
	"github.com/spf13/cobra"
)

func resolveDSNAndDir() (string, string, error) {
	dsn := config.ResolveDSN(flagDSN)
	if dsn == "" {
		return "", "", fmt.Errorf("no DSN: pass --dsn or set DATABASE_URL")
	}
	dir, err := config.ResolveDir(flagDir, ".")
	if err != nil {
		return "", "", err
	}
	return dsn, dir, nil
}

func newMigrateUpCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "up",
		Short: "Apply all pending migrations",
		RunE: func(cmd *cobra.Command, _ []string) error {
			dsn, dir, err := resolveDSNAndDir()
			if err != nil {
				return err
			}
			r, err := migrate.NewRunner(dsn, os.DirFS(dir), ".")
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
