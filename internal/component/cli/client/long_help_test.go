// Design: docs/architecture/cli/command-completion.md -- what the attached console's tree carries

package client

import (
	"testing"

	unicli "github.com/ze-software/ze/internal/component/cli"
	cmd "github.com/ze-software/ze/internal/component/command"
)

// commandListDispatch answers the given JSON to `system command list`, which is
// the one command the runtime tree builder sends, and an empty answer to
// everything else.
func commandListDispatch(list string) CommandFunc {
	return func(command string) (unicli.CommandOutput, error) {
		if command == "system command list" {
			return unicli.CommandOutput{Text: list}, nil
		}
		return unicli.CommandOutput{}, nil
	}
}

// TestRuntimeTreeCarriesLongHelp: the attached console of `ze start --cli`
// answers the `?` key from its own tree. The daemon's command list is the only
// place that tree reads an explanation from. A YANG-backed command and a
// plugin-only command reach the tree by different routes, so this test reads
// both. A command that declares no explanation still answers none.
//
// VALIDATES: `?` on a candidate in the attached console prints the explanation
// the command declares.
// PREVENTS: `<command>: no explanation is declared` for every command in the
// attached console.
func TestRuntimeTreeCarriesLongHelp(t *testing.T) {
	const list = `{"commands":[
		{"value":"show env get","help":"Read one environment value","long-help":"The explanation a YANG-backed command declares."},
		{"value":"zz-plugin-explained","help":"A plugin command","long-help":"The explanation a plugin declares."},
		{"value":"zz-plugin-bare","help":"A plugin command that declares no explanation"}
	]}`

	tree := buildRuntimeTreeFromDispatch(commandListDispatch(list))
	if tree == nil {
		t.Fatal("buildRuntimeTreeFromDispatch returned nil")
	}
	if node := tree.Children["show"]; node == nil || node.Children["env"] == nil || node.Children["env"].Children["get"] == nil {
		t.Fatal("show env get is absent from the runtime tree: the answer was not parsed")
	}

	completer := cmd.NewTreeCompleter(tree)

	cases := []struct {
		command  string
		want     string
		declared bool
	}{
		{"show env get", "The explanation a YANG-backed command declares.", true},
		{"zz-plugin-explained", "The explanation a plugin declares.", true},
		{"zz-plugin-bare", "", false},
	}
	for _, c := range cases {
		got, ok := completer.Explain(c.command)
		if ok != c.declared {
			t.Errorf("Explain(%q) declared = %v, want %v", c.command, ok, c.declared)
		}
		if got != c.want {
			t.Errorf("Explain(%q) = %q, want %q", c.command, got, c.want)
		}
	}
}
