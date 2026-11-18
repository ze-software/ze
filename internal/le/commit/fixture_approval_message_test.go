// Design: docs/contributing/committing.md -- fixture approvals in commit messages
// Related: input.go -- Message; weekly_source.go -- approval record readback

package commit

import (
	"slices"
	"strings"
	"testing"
)

// TestMessageKeepsFixtureApprovalReadable proves a fixture verdict survives
// message formatting as the same physical record the weekly reader counts.
// Method: pass the actual long PPP fixture approval through Message, then
// inspect its whole line and the existing approval reader's count and carrier.
func TestMessageKeepsFixtureApprovalReadable(t *testing.T) {
	const fixture = "internal/component/l2tp/ppp/kernel/native_ppp_global_support_integration_linux_test.go"
	const verdict = "APPROVE only with the complete Source13 chain plus13d/13da: exact2047 and explicit bit9 presence match the newly admitted full-runtime compiled declaration while preserving all ABI, malformed, bound-file, capability, duplicate, unbound and no-effect assertions."
	approval := testJudgeLine + fixture + ": " + verdict
	if len(approval) <= messageWidth {
		t.Fatal("the fixture approval does not exceed the ordinary body width")
	}
	prose := "Ordinary body " + strings.TrimSpace(strings.Repeat("word ", 30))
	message, err := Message("fix: preserve the fixture approval record", []string{prose, "", approval}, nil)
	if err != nil {
		t.Fatalf("a fixture approval body was refused: %v", err)
	}
	if !strings.Contains(message, "\n"+approval+"\n") {
		t.Errorf("the fixture approval is not one complete physical line:\n%s", message)
	}
	revision := strings.Repeat("a", 40)
	count, carriers := weeklyCountLines([]string{revision}, map[string]string{revision: message}, testJudgeLine)
	if count != 1 {
		t.Errorf("the formatted fixture approval was counted %d times, want 1", count)
	}
	if !slices.Equal(carriers, []string{revision}) {
		t.Errorf("approval carriers %v, want only %s", carriers, revision)
	}

	// The verdict record is exceptional; surrounding ordinary prose keeps its
	// existing width and content rather than escaping the formatter with it.
	var proseLines []string
	for _, line := range bodyLines(t, message) {
		if line == "" {
			break
		}
		if len(line) > messageWidth {
			t.Errorf("ordinary body line exceeds %d characters: %q", messageWidth, line)
		}
		proseLines = append(proseLines, line)
	}
	if len(proseLines) < 2 {
		t.Error("ordinary prose did not wrap across physical lines")
	}
	if strings.Join(proseLines, " ") != prose {
		t.Errorf("ordinary prose changed: %q", proseLines)
	}
}
