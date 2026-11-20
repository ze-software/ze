// VALIDATES: `le spec design approve` refuses without an interactive terminal
// through the owner-terminal gate, and leaves tmp/spec-design/ as it was
// (plan/release/validation/spec-design-needs-owner-approval.md, AC-17).
// PREVENTS: an agent recording Thomas's approval of a design.
package specdesign_test

import (
	"testing"

	specdesign "github.com/ze-software/ze/internal/le/spec/design"
)

// TestApproveRefusesWithoutTerminal is the wiring row "`./le spec design
// approve` without a terminal". Method: a test process has no terminal on
// stdin and stdout; the approve action exits 2 with human.RequireAuthorization's
// refusal and writes nothing under tmp/spec-design/.
func TestApproveRefusesWithoutTerminal(t *testing.T) {
	root := widgetRoot(t)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "approve-session")

	var code int
	stderr := captureStderr(t, func() {
		_, code = specdesign.Answer([]string{"approve", "spec", widgetSpec, "words", "approved"})
	})
	if code != 2 {
		t.Fatalf("approve without a terminal answered %d, want 2:\n%s", code, stderr)
	}
	requireContains(t, "the approve refusal", stderr,
		"owner authorization requires interactive stdin and stdout", "spec design approve")
	requireAbsent(t, root, "tmp/spec-design")
}
