package cli

import (
	"testing"

	"github.com/ze-software/ze/internal/component/cli/sshclient"
	"github.com/ze-software/ze/internal/component/command"
)

// TestEditorExecutorSendsTheDeclaredOperations asks what the editor, a
// renderer in another process than the daemon, sends for a typed line whose
// command declares a pipe operation: the command part and the declared
// operation as a pipe operation, asking for the dispatcher's JSON, and none
// of the residual chain, which the editor's model renders once itself
// (package 3c of spec-pipe-chain-honored-on-every-command-surface).
//
// Method: a command path declaring `scope`, and the SSH send replaced by one
// that records the line. `<path> | scope | json` must be sent as
// `<path> | scope | raw`.
//
// MUTATION: send the typed line; the daemon would render `| json` and the
// model would render it again. Send the command part alone; `scope` never
// reaches the daemon.
func TestEditorExecutorSendsTheDeclaredOperations(t *testing.T) {
	const path = "show test editor-declared"
	command.RegisterPipeFilters([]string{path},
		command.PipeFilter{Name: "scope", Description: "Pick a scope", Argument: command.ArgNone})

	var sent []string
	original := execEditorCommand
	execEditorCommand = func(_ sshclient.Credentials, line string) (string, error) {
		sent = append(sent, line)
		return `{"ok":"yes"}`, nil
	}
	t.Cleanup(func() { execEditorCommand = original })

	output, err := sshCommandExecutor(sshclient.Credentials{})(path + " | scope | json")
	if err != nil {
		t.Fatalf("editor executor: %v", err)
	}
	if output.Text != `{"ok":"yes"}` {
		t.Errorf("editor executor answered %q, want the dispatcher's JSON", output.Text)
	}
	if want := path + " | scope | raw"; len(sent) != 1 || sent[0] != want {
		t.Errorf("editor executor sent %q, want %q", sent, want)
	}
}
