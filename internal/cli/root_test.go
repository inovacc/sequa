package cli

import "testing"

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
