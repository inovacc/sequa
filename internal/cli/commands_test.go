package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// resetFlags saves and restores the package-level flag globals around a test.
func resetFlags(t *testing.T) {
	t.Helper()
	dsn, dir := flagDSN, flagDir
	t.Cleanup(func() { flagDSN, flagDir = dsn, dir })
}

func TestInitCreatesDir(t *testing.T) {
	resetFlags(t)
	dir := filepath.Join(t.TempDir(), "migrations")
	flagDir = dir
	cmd := newInitCmd()
	cmd.SetContext(context.Background())
	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("init: %v", err)
	}
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
		t.Fatalf("init did not create %s", dir)
	}
}

func TestMigrateCreateWritesPair(t *testing.T) {
	resetFlags(t)
	dir := t.TempDir()
	flagDir = dir
	cmd := newMigrateCreateCmd()
	cmd.SetContext(context.Background())
	if err := cmd.RunE(cmd, []string{"add_users"}); err != nil {
		t.Fatalf("migrate create: %v", err)
	}
	ups, _ := filepath.Glob(filepath.Join(dir, "*_add_users.up.sql"))
	downs, _ := filepath.Glob(filepath.Join(dir, "*_add_users.down.sql"))
	if len(ups) != 1 || len(downs) != 1 {
		t.Fatalf("expected one up/down pair, got up=%v down=%v", ups, downs)
	}
}

// The database-touching commands must fail fast with a clear error when no DSN
// is configured — before any connection is attempted.
func TestCommandsRequireDSN(t *testing.T) {
	resetFlags(t)
	t.Setenv("DATABASE_URL", "")
	flagDSN = ""
	flagDir = t.TempDir()
	cmds := map[string]*cobra.Command{
		"migrate up":      newMigrateUpCmd(),
		"migrate down":    newMigrateDownCmd(),
		"migrate status":  newMigrateStatusCmd(),
		"migrate version": newMigrateVersionCmd(),
		"verify":          newVerifyCmd(),
	}
	for name, cmd := range cmds {
		t.Run(name, func(t *testing.T) {
			cmd.SetContext(context.Background())
			err := cmd.RunE(cmd, nil)
			if err == nil || !strings.Contains(err.Error(), "DSN") {
				t.Fatalf("%s: expected a no-DSN error, got %v", name, err)
			}
		})
	}
}

func TestMigrateCmdHasSubcommands(t *testing.T) {
	want := map[string]bool{"create": false, "up": false, "down": false, "status": false, "version": false}
	for _, c := range newMigrateCmd().Commands() {
		if _, ok := want[c.Name()]; ok {
			want[c.Name()] = true
		}
	}
	for name, found := range want {
		if !found {
			t.Errorf("migrate missing subcommand %q", name)
		}
	}
}
