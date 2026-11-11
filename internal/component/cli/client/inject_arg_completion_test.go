package client

import (
	"errors"
	"testing"

	cmd "github.com/ze-software/ze/internal/component/command"
)

// TestDaemonArgCompletionAsksTheDaemon proves the `ze cli` tree completes the
// argument of a command the daemon marks completable by asking the daemon.
//
// Goal: Tab after `request plugin restart` in `ze cli` offers the plugin names
// the daemon answers, and a command not marked completable gets no hint.
// Method: a tree holding both commands, a command list marking only the
// restart command, and a fake daemon that records the line it was asked and
// answers two candidates; then the same fake failing.
func TestDaemonArgCompletionAsksTheDaemon(t *testing.T) {
	tree := &cmd.Node{Children: map[string]*cmd.Node{
		"request": {Name: "request", Children: map[string]*cmd.Node{
			"plugin": {Name: "plugin", Children: map[string]*cmd.Node{
				"restart": {Name: "restart"},
			}},
			"shutdown": {Name: "shutdown"},
		}},
	}}
	commands := []commandEntry{
		{Value: "request plugin restart", Completable: true},
		{Value: "request shutdown"},
	}
	var asked []string
	fail := false
	query := func(line string) (string, error) {
		asked = append(asked, line)
		if fail {
			return "", errors.New("daemon unreachable")
		}
		return `{"completions":[{"value":"alpha","short-help":"configured plugin, not running"},{"value":"beta"}]}`, nil
	}

	wireDaemonArgCompletion(tree, commands, query)

	restart := cmd.FindNode(tree, []string{"request", "plugin", "restart"})
	if restart.ValueHints == nil {
		t.Fatal("the restart node got no value hint")
	}
	if shutdown := cmd.FindNode(tree, []string{"request", "shutdown"}); shutdown.ValueHints != nil {
		t.Error("a command the daemon does not mark completable got a value hint")
	}

	hints := restart.ValueHints()
	if len(hints) != 2 || hints[0].Text != "alpha" || hints[1].Text != "beta" {
		t.Fatalf("hints = %+v, want alpha and beta", hints)
	}
	if hints[0].Description != "configured plugin, not running" {
		t.Errorf("hint description = %q, want the daemon's short help", hints[0].Description)
	}
	if hints[0].Type != cmd.SuggestionValue {
		t.Errorf("hint type = %q, want %q", hints[0].Type, cmd.SuggestionValue)
	}
	if len(asked) != 1 || asked[0] != `system command complete args "request plugin restart"` {
		t.Errorf("asked %q, want one system command complete args line for the restart command", asked)
	}

	fail = true
	if hints := restart.ValueHints(); len(hints) != 0 {
		t.Errorf("an unreachable daemon must offer nothing, got %+v", hints)
	}

	decoded, err := decodeCommandList([]byte(`{"commands":[{"value":"request plugin restart","short-help":"x","completable":true}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(decoded) != 1 || !decoded[0].Completable {
		t.Errorf("decodeCommandList lost the completable flag: %+v", decoded)
	}
}
