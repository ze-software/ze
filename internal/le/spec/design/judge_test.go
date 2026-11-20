// VALIDATES: `le spec design judge` refuses a judge that has not read the
// section behind every row of the spec's Design documents table, and writes
// no record (spec-design-needs-owner-approval,
// AC-16).
// PREVENTS: a verdict recorded by a judge that never opened the page it
// vouches for.
package specdesign_test

import (
	"testing"

	specdesign "github.com/ze-software/ze/internal/le/spec/design"
)

// judgedRecord is where the judge action writes the widget spec's verdict.
const judgedRecord = "tmp/spec-design/judged/spec-widget.md"

// TestJudgeRefusesUncoveredRow is the wiring row "`./le spec design judge`
// from an agent". Method: an agent identity with no section read record runs
// the judge over the widget spec; it exits 2 naming the page and the Read
// that clears it, and the judged record is not written.
func TestJudgeRefusesUncoveredRow(t *testing.T) {
	root := widgetRoot(t)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "judge-session")
	t.Setenv("ZE_AGENT_ID", "judge-agent")

	var code int
	stderr := captureStderr(t, func() {
		_, code = specdesign.Answer([]string{"judge", "spec", widgetSpec, "verdict", "approve", "reason", "every verdict holds"})
	})
	if code != 2 {
		t.Fatalf("a judge with no read of %s answered %d, want 2:\n%s", widgetPage, code, stderr)
	}
	requireContains(t, "the judge refusal", stderr, widgetPage, "Design decisions", "Read it with offset")
	requireAbsent(t, root, judgedRecord)
}
