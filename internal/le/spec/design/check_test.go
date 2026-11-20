// VALIDATES: `le spec design check spec <path>` refuses each way a spec's
// Design documents table and Design presentation fall short, and passes a
// spec that owes nothing
// (plan/release/validation/spec-design-needs-owner-approval.md, AC-14).
// PREVENTS: a design presented to Thomas with a linked page or decision left
// out, a verdict quoting words its page does not hold, or no judge.
package specdesign_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	specdesign "github.com/ze-software/ze/internal/le/spec/design"
)

// completeSpecText is the widget spec with every row its files owe, the five
// presentation subsections, and nothing else missing: the check's clean case.
const completeSpecText = "# Spec: widget\n\n| Field | Value |\n|-------|-------|\n| Status | design |\n" +
	"| Priority | P2 |\n| Updated | 2026-11-19 |\n\n## Task\n\nRender the widget in the surface.\n\n" +
	"## Acceptance Criteria\n\n| AC ID | Input | Expected |\n|---|---|---|\n| AC-1 | a widget | the surface renders it |\n\n" +
	"## Files to Modify\n- `" + widgetCode + "` - render in the surface\n\n" +
	"## Design documents\n\n| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |\n" +
	"|---|---|---|---|---|---|\n" +
	"| `" + widgetPage + "` | Design decisions | Design header of `" + widgetCode + "` | \"" + widgetDecision + "\" | agrees | The surface renders |\n" +
	"| `" + widgetDoor + "` | System door | every change | - | silent | The door names no rule on rendering |\n\n" +
	"## Design presentation\n\n### What changes\n\nThe surface renders the widget.\n\n" +
	"### Before and after\n\nBefore: the widget renders text. After: the surface renders it.\n\n" +
	"### Alternatives set aside\n\nRendering in the widget, which the decision forbids.\n\n" +
	"### Documents departed from\n\nnone\n\n### Choices for Thomas\n\nNone.\n\n## Implementation Steps\n\n1. Render.\n"

// writeJudgeRecord writes the judge record of the widget spec for text, as
// the judge action spells it.
func writeJudgeRecord(t *testing.T, root, text, verdict string) {
	t.Helper()
	writeTree(t, root, map[string]string{judgedRecord: "Spec: spec-widget.md\nDigest: " + specdesign.DesignDigest(text) +
		"\nVerdict: " + verdict + "\nSentence: every verdict holds\nJudge: judge-session+judge-agent\nDate: 2026-11-20\n"})
}

// runCheck runs the check over the widget spec written as text.
func runCheck(t *testing.T, root, text string) (string, int) {
	t.Helper()
	writeTree(t, root, map[string]string{widgetSpec: text})
	var answer any
	var code int
	stderr := captureStderr(t, func() {
		answer, code = specdesign.Answer([]string{"check", "spec", widgetSpec})
	})
	return answerText(t, answer) + stderr, code
}

// TestDesignCheckJudgement is AC-14. Method: the complete widget spec with a
// current judge approval passes with "ready to present"; then one edit per
// problem the criterion names, each exiting 1 with a line naming the problem
// and the closing line naming the documents command.
func TestDesignCheckJudgement(t *testing.T) {
	root := widgetRoot(t)
	writeJudgeRecord(t, root, completeSpecText, "approve")
	text, code := runCheck(t, root, completeSpecText)
	if code != 0 {
		t.Fatalf("the complete spec answered %d, want 0:\n%s", code, text)
	}
	requireContains(t, "the clean check", text, "ready to present")

	modifiesPage := strings.Replace(completeSpecText, "render in the surface\n",
		"render in the surface\n- `"+widgetPage+"` - the page this spec changes\n", 1)
	cases := []struct {
		name   string
		text   string
		judged string
		want   string
	}{
		{"no Design documents section", cutBetween(completeSpecText, "## Design documents", "## Design presentation"), "", "has no ## Design documents section"},
		{"a derived row missing", strings.Replace(completeSpecText, "| `"+widgetDoor+"` |", "| `ai/doors/other.md` |", 1), "", widgetDoor},
		{"a verdict outside the three", strings.Replace(completeSpecText, "| agrees |", "| agree |", 1), "", "none of agrees, departs, silent"},
		{"a section the page does not hold", strings.Replace(completeSpecText, "| Design decisions |", "| Design choices |", 1), "", "holds no section"},
		{"a quote that is not verbatim", strings.Replace(completeSpecText, "\""+widgetDecision+"\" | agrees", "\"The widget renders text\" | agrees", 1), "", "not a verbatim span"},
		{"a silent row with no reason", strings.Replace(completeSpecText, "| The door names no rule on rendering |", "| |", 1), "", "gives no reason"},
		{"a page the spec modifies marked silent", strings.Replace(modifiesPage, "| `"+widgetDoor+"` |",
			"| `"+widgetPage+"` | Wire format | modified | - | silent | nothing on rendering |\n| `"+widgetDoor+"` |", 1), "", "cannot be silent on it"},
		{"a decision row marked silent", strings.Replace(completeSpecText, "| agrees | The surface renders |", "| silent | The surface renders |", 1), "", "a decision row cannot be silent"},
		{"no What changes", strings.Replace(completeSpecText, "### What changes\n\nThe surface renders the widget.\n\n", "", 1), "", "has no ### What changes"},
		{"an empty Before and after", strings.Replace(completeSpecText, "Before: the widget renders text. After: the surface renders it.\n\n", "", 1), "", "### Before and after is empty"},
		{"no Alternatives set aside", strings.Replace(completeSpecText, "### Alternatives set aside", "### Alternatives", 1), "", "has no ### Alternatives set aside"},
		{"no Documents departed from", strings.Replace(completeSpecText, "### Documents departed from", "### Departures", 1), "", "has no ### Documents departed from"},
		{"no Choices for Thomas", strings.Replace(completeSpecText, "### Choices for Thomas", "### Choices", 1), "", "has no ### Choices for Thomas"},
		{"a departs page left unnamed", strings.Replace(completeSpecText, "| agrees | The surface renders |", "| departs | The surface renders |", 1), "", "does not name " + widgetPage},
		{"no judge record", completeSpecText, "none", "no judge has judged this design"},
		{"a judge record of another text", completeSpecText, "stale", "the judge judged another text"},
		{"a refusing judge", completeSpecText, "refuse", "the judge refused this design"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			switch tc.judged {
			case "":
				writeJudgeRecord(t, root, tc.text, "approve")
			case "none":
				requireRemoved(t, root, judgedRecord)
			case "stale":
				writeJudgeRecord(t, root, strings.Replace(tc.text, "Render the widget", "Render the gadget", 1), "approve")
			case "refuse":
				writeJudgeRecord(t, root, tc.text, "refuse")
			}
			text, code := runCheck(t, root, tc.text)
			if code != 1 {
				t.Fatalf("answered %d, want 1:\n%s", code, text)
			}
			requireContains(t, "the check", text, tc.want, "run ./le spec design documents spec "+widgetSpec+" to list the owed rows")
		})
	}
}

// cutBetween removes text from the line starting from up to the line starting
// to, keeping to.
func cutBetween(text, from, to string) string {
	start := strings.Index(text, from)
	end := strings.Index(text, to)
	if start < 0 || end < start {
		return text
	}
	return text[:start] + text[end:]
}

// requireRemoved removes path under root, which may not exist.
func requireRemoved(t *testing.T, root, path string) {
	t.Helper()
	err := os.Remove(filepath.Join(root, filepath.FromSlash(path)))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
}
