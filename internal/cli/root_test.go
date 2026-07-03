package cli

import (
	"context"
	"fmt"
	"testing"

	"github.com/spf13/cobra"
)

func TestRootHasCoreSubcommands(t *testing.T) {
	root := newRootCmd()
	want := map[string]bool{"migrate": false, "generate": false, "query": false, "init": false}
	for _, c := range root.Commands() {
		if _, ok := want[c.Name()]; ok {
			want[c.Name()] = true
		}
	}
	for name, found := range want {
		if !found {
			t.Errorf("root missing subcommand %q", name)
		}
	}
}

func TestRunExitCode(t *testing.T) {
	ctx := context.Background()
	quiet := func(c *cobra.Command) *cobra.Command {
		c.SilenceUsage, c.SilenceErrors = true, true
		return c
	}
	ok := quiet(&cobra.Command{Use: "ok", RunE: func(*cobra.Command, []string) error { return nil }})
	if code := run(ctx, ok); code != 0 {
		t.Errorf("run(success) = %d, want 0", code)
	}
	bad := quiet(&cobra.Command{Use: "bad", RunE: func(*cobra.Command, []string) error { return fmt.Errorf("boom") }})
	if code := run(ctx, bad); code != 1 {
		t.Errorf("run(failure) = %d, want 1", code)
	}
}
