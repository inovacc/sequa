package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// runRoot executes a fresh root command with args, capturing stdout+stderr.
func runRoot(t *testing.T, args ...string) (string, error) {
	t.Helper()
	root := newRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(args)
	err := root.Execute()
	return out.String(), err
}

func TestToolingCommandsRegistered(t *testing.T) {
	want := map[string]bool{"cmdtree": false, "aicontext": false}
	for _, c := range newRootCmd().Commands() {
		if _, ok := want[c.Name()]; ok {
			want[c.Name()] = true
		}
	}
	for name, found := range want {
		if !found {
			t.Errorf("root missing tooling command %q", name)
		}
	}
}

func TestCmdtreeDefault(t *testing.T) {
	out, err := runRoot(t, "cmdtree")
	if err != nil {
		t.Fatalf("cmdtree: %v", err)
	}
	for _, want := range []string{"Command Tree", "migrate", "generate", "aicontext"} {
		if !strings.Contains(out, want) {
			t.Errorf("cmdtree output missing %q:\n%s", want, out)
		}
	}
}

func TestCmdtreeFull(t *testing.T) {
	out, err := runRoot(t, "cmdtree", "--full")
	if err != nil {
		t.Fatalf("cmdtree --full: %v", err)
	}
	// Verbose mode prints per-command usage lines and the root's global flags.
	if !strings.Contains(out, "Usage:") || !strings.Contains(out, "Global Flags") {
		t.Errorf("cmdtree --full missing details:\n%s", out)
	}
}

func TestCmdtreeJSON(t *testing.T) {
	out, err := runRoot(t, "cmdtree", "--json")
	if err != nil {
		t.Fatalf("cmdtree --json: %v", err)
	}
	var detail commandDetail
	if err := json.Unmarshal([]byte(out), &detail); err != nil {
		t.Fatalf("cmdtree --json is not valid JSON: %v\n%s", err, out)
	}
	if detail.Name != "sequa" || len(detail.Subcommands) == 0 {
		t.Errorf("unexpected JSON tree: name=%q subcommands=%d", detail.Name, len(detail.Subcommands))
	}
}

func TestCmdtreeSingleCommand(t *testing.T) {
	out, err := runRoot(t, "cmdtree", "--command", "migrate")
	if err != nil {
		t.Fatalf("cmdtree --command migrate: %v", err)
	}
	if !strings.Contains(out, "# migrate") || !strings.Contains(out, "Subcommands:") {
		t.Errorf("single-command output unexpected:\n%s", out)
	}
}

func TestCmdtreeUnknownCommand(t *testing.T) {
	if _, err := runRoot(t, "cmdtree", "--command", "does-not-exist"); err == nil {
		t.Fatal("expected an error for an unknown command")
	}
}

func TestAicontextMarkdown(t *testing.T) {
	out, err := runRoot(t, "aicontext")
	if err != nil {
		t.Fatalf("aicontext: %v", err)
	}
	for _, want := range []string{"# sequa — AI Context", "## Commands", "### migrate", "## Project Structure"} {
		if !strings.Contains(out, want) {
			t.Errorf("aicontext output missing %q:\n%s", want, out)
		}
	}
}

func TestAicontextJSON(t *testing.T) {
	out, err := runRoot(t, "aicontext", "--json")
	if err != nil {
		t.Fatalf("aicontext --json: %v", err)
	}
	var doc aiContextDoc
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("aicontext --json is not valid JSON: %v\n%s", err, out)
	}
	if doc.Tool != "sequa" || len(doc.Commands) == 0 {
		t.Errorf("unexpected JSON doc: tool=%q commands=%d", doc.Tool, len(doc.Commands))
	}
}

func TestAicontextCompact(t *testing.T) {
	out, err := runRoot(t, "aicontext", "--compact")
	if err != nil {
		t.Fatalf("aicontext --compact: %v", err)
	}
	if !strings.Contains(out, "`sequa migrate`") {
		t.Errorf("compact output missing command line:\n%s", out)
	}
}
