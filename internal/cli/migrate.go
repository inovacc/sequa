package cli

import (
	"fmt"
	"os"

	"github.com/inovacc/sequa/internal/config"
	"github.com/inovacc/sequa/internal/migrate"
	"github.com/spf13/cobra"
)

// newRunnerFromFlags resolves the DSN and migrations directory from the shared
// --dsn/--dir flags and returns a Runner rooted there. The four migrate
// subcommands (up, down, status, version) all open the database this way.
func newRunnerFromFlags() (*migrate.Runner, error) {
	dsn := config.ResolveDSN(flagDSN)
	if dsn == "" {
		return nil, fmt.Errorf("no DSN: pass --dsn or set DATABASE_URL")
	}
	dir, err := config.ResolveDir(flagDir, ".")
	if err != nil {
		return nil, err
	}
	return migrate.NewRunner(dsn, os.DirFS(dir), ".")
}

func newMigrateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Manage database migrations",
	}
	cmd.AddCommand(
		newMigrateCreateCmd(),
		newMigrateUpCmd(),
		newMigrateDownCmd(),
		newMigrateStatusCmd(),
		newMigrateVersionCmd(),
	)
	return cmd
}
