package cli

import "github.com/spf13/cobra"

func newMigrateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Manage database migrations",
	}
	// children registered in later tasks:
	//   cmd.AddCommand(newMigrateCreateCmd(), newMigrateUpCmd(), newMigrateDownCmd(),
	//       newMigrateStatusCmd(), newMigrateVersionCmd())
	return cmd
}
