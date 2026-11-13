// Design: docs/architecture/core-design.md -- le's composition, one import per tool
// Related: register.go -- the expensive mark of the unit selftest
// Related: ../../hookruntime/bash.go -- expensiveWords, the hook's reader of the mark through leroot.HeavyLine

package aihooks

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/ze-software/ze/internal/le/hookruntime"
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

// TestHookSelftestIsAnExpensiveCommand is hook defect (a) of the parity
// golden's first judgment by the hook (AC-36). Its goal: the hook selftest,
// which prints each failing row before its summary, is an expensive command
// under each name the program takes, so piping it through a filter that
// keeps the last lines is refused, while the hook verbs beside it are not.
// Its method: the registration this package's init makes, then the real
// pretool-bash action over a main-thread payload, which links that
// registration as the shared le does.
// PREVENTS: `ze le ai hooks unit | tail -25` passing, which it did because
// no area rule of the hook reads an action outside `test`, `verify` and
// `go lint` as heavy.
func TestHookSelftestIsAnExpensiveCommand(t *testing.T) {
	if !leroot.Expensive(area + " " + unitVerb) {
		t.Fatalf("%q is not registered expensive", area+" "+unitVerb)
	}
	if leroot.Expensive(area + " pretool-bash") {
		t.Errorf("%q is registered expensive: only the selftest is", area+" pretool-bash")
	}
	t.Setenv("CLAUDE_PROJECT_DIR", t.TempDir())
	t.Setenv("CLAUDE_CODE_FORK_SUBAGENT", "")
	for _, row := range []struct {
		command string
		code    int
	}{
		{"ze le ai hooks unit | tail -25", 2},
		{"./le ai hooks unit | tail -25", 2},
		{"bin/ze le ai hooks unit 2>&1 | grep FAIL", 2},
		{"for p in a; do ./le ai hooks unit | tail -25; done", 2},
		{"./le ai hooks unit", 0},
		{"./le ai hooks pretool-bash | tail -5", 0},
	} {
		payload, err := json.Marshal(map[string]any{
			"session_id": "selftest-expensive", "tool_name": "Bash",
			"tool_input": map[string]any{"command": row.command},
		})
		if err != nil {
			t.Fatal(err)
		}
		var out, errOut bytes.Buffer
		if code := hookruntime.Run("pretool-bash", bytes.NewReader(payload), &out, &errOut); code != row.code {
			t.Errorf("%q: code=%d, want %d: %s", row.command, code, row.code, errOut.String())
		}
	}
}
