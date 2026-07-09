// Design: (none -- shell completion glue assertions)

package completion

import (
	"strings"
	"testing"
)

// TestShellScriptEmitsConfigCompletionGlue verifies each generated shell script
// wires the config-section completion through the existing `ze config completion`
// engine (AC-9) and the flag/value inventory through `ze completion flags` /
// `ze completion families` (AC-8), rather than hardcoding lists.
//
// VALIDATES: AC-8, AC-9 -- generated scripts call the shared engines.
// PREVENTS: silent regression where a generator drops the dynamic glue and
// falls back to hardcoded or file-only completion.
func TestShellScriptEmitsConfigCompletionGlue(t *testing.T) {
	cases := []struct {
		shell  string
		script string
	}{
		{"bash", bashScript()},
		{"zsh", zshScript()},
		{"fish", fishScript()},
	}
	// Every generator must reference these engines to satisfy the ACs.
	want := []string{
		"ze config completion", // AC-9 config-section completion for `config show`
		"ze completion flags",  // AC-8 flag-name inventory
		"ze completion families",
	}
	for _, c := range cases {
		for _, w := range want {
			if !strings.Contains(c.script, w) {
				t.Errorf("%s script missing glue %q", c.shell, w)
			}
		}
		// The `show` subcommand must appear in the config subcommand list.
		if !strings.Contains(c.script, "show") {
			t.Errorf("%s script does not mention the config `show` subcommand", c.shell)
		}
	}
}
