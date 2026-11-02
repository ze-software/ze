// Design: docs/architecture/api/commands.md — a flag that selects the answer is a leaf

package cli

import (
	"strings"
	"testing"
)

// testConfigUnformatted is testConfigPlain's first block with its indentation
// lost, so formatting changes it.
const testConfigUnformatted = "bgp {\nrouter-id 1.1.1.1;\n}\n"

// TestShowConfigFmtCheckAndDiffLeaves proves the choices `show config fmt`
// made only through --check and --diff are declared leaves.
//
// Goal: AC-8 part 2. On the daemon route, where a flag never reaches the
// handler, `check` and `diff` answer exactly what the operator's own `ze`
// answers for --check and --diff, and differ from the plain formatted text.
// Method: each call runs through the model validator on both routes over a
// confined, unformatted file.
func TestShowConfigFmtCheckAndDiffLeaves(t *testing.T) {
	file := confinedConfigDir(t, testConfigUnformatted)
	plain := encodedAnswer(t, answerOn(t, "show config fmt", []string{file}, false))

	for leaf, flag := range map[string]string{"check": "--check", "diff": "--diff"} {
		want := encodedAnswer(t, answerOn(t, "show config fmt", []string{flag, file}, false))
		if want == plain {
			t.Fatalf("%s answered the plain formatted text: the local option selects nothing to compare", flag)
		}
		for _, remote := range []bool{false, true} {
			got := encodedAnswer(t, answerOn(t, "show config fmt", []string{leaf, file}, remote))
			if got != want {
				t.Errorf("remote=%v: %s answered\n%s\nwant what %s answers\n%s", remote, leaf, got, flag, want)
			}
		}
	}

	refusal := answerOn(t, "show config fmt", []string{"check", "--check", file}, false)
	if refusal.Code == 0 || !strings.Contains(refusal.Diagnostic, "make the same choice") {
		t.Errorf("check with --check: code %d %q, want the twice-made choice refused", refusal.Code, refusal.Diagnostic)
	}
}
