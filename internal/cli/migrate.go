package cli

import "github.com/spf13/cobra"

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
