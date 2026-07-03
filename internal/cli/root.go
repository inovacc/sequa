package cli

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
)

var (
	flagDSN     string
	flagDir     string
	flagVerbose bool
)

// Version is the build version, shown by `sequa --version`. main() sets it from
// its own ldflags-injected value.
var Version = "dev"

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "sequa",
		Short:         "SQL migration, query, and codegen toolkit",
		Version:       Version,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(_ *cobra.Command, _ []string) error {
			setupLogger(flagVerbose)
			return nil
		},
	}

	pf := root.PersistentFlags()
	pf.StringVar(&flagDSN, "dsn", "", "database DSN (falls back to $DATABASE_URL)")
	pf.StringVar(&flagDir, "dir", "", "migrations directory (autodetected if empty)")
	pf.BoolVarP(&flagVerbose, "verbose", "v", false, "verbose (debug) logging")

	root.AddCommand(newMigrateCmd(), newGenerateCmd(), newQueryCmd(), newInitCmd(), newVerifyCmd())
	return root
}

// Execute is the single entrypoint used by main(). It runs under a context that
// is cancelled on SIGINT/SIGTERM so an interrupted command (e.g. `migrate up`)
// unwinds cleanly instead of being killed mid-step.
func Execute() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := newRootCmd().ExecuteContext(ctx); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func setupLogger(verbose bool) {
	level := slog.LevelInfo
	if verbose {
		level = slog.LevelDebug
	}
	// stderr: stdout is reserved for query/data output.
	h := slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: level})
	slog.SetDefault(slog.New(h))
}
