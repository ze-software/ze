// Design: docs/contributing/running-commands.md -- le's exit codes and their exemptions
// Related: actions.go -- the hook verbs, which answer the hook protocol

package aihooks

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestHookActionsKeepTheHookProtocolCodes validates AC-32 and R-13: `le ai
// hooks <hook>` answers the hook protocol, 0 to allow and 2 to block, and not
// le's usage code 2 or found-a-problem code 1. Claude Code reads 2 from a hook
// as "block", so a hook that answered 1 for a block would let every refused
// command run. Method: feed the pretool-bash verb, through the area's own
// Answer, a payload it allows and one it blocks, on stdin as the harness does.
func TestHookActionsKeepTheHookProtocolCodes(t *testing.T) {
	root := t.TempDir()
	t.Setenv("CLAUDE_PROJECT_DIR", root)
	for _, row := range []struct {
		command string
		code    int
	}{
		{command: "echo ok", code: 0},
		{command: "git reset --hard", code: 2},
	} {
		body, err := json.Marshal(map[string]any{"tool_name": "Bash", "tool_input": map[string]any{"command": row.command}})
		if err != nil {
			t.Fatal(err)
		}
		payload := filepath.Join(root, "payload.json")
		if err := os.WriteFile(payload, body, 0o600); err != nil {
			t.Fatal(err)
		}
		in, err := os.Open(payload)
		if err != nil {
			t.Fatal(err)
		}
		saved := os.Stdin
		os.Stdin = in
		_, code := Answer([]string{"pretool-bash"})
		os.Stdin = saved
		in.Close() //nolint:errcheck // read-only fixture file
		if code != row.code {
			t.Errorf("pretool-bash over %q answered %d, want the hook protocol's %d", row.command, code, row.code)
		}
	}
}
