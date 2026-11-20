// VALIDATES: `le spec design pending` lists every ready spec whose design as
// it stands has no approval, ordered by Priority, then category, then
// Updated, each with what it lacks, and ends with the away state
// (plan/release/validation/spec-design-needs-owner-approval.md, AC-30, C-2).
// PREVENTS: a ready spec waiting for Thomas that nobody shows him, and an
// approved one shown as waiting.
package specdesign_test

import (
	"strings"
	"testing"

	specdesign "github.com/ze-software/ze/internal/le/spec/design"
	"github.com/ze-software/ze/internal/le/spec/design/designtest"
)

// gitInit makes root a git repository with no commit, so the approval lookup
// reads an empty history rather than failing on a missing one.
func gitInit(t *testing.T, root string) {
	t.Helper()
	if err := designtest.InitRepository(root); err != nil {
		t.Fatal(err)
	}
}

// readySpec is a ready spec of priority and updated date with no design
// sections.
func readySpec(priority, updated string) string {
	return "# Spec: x\n\n| Field | Value |\n|-------|-------|\n| Status | ready |\n| Priority | " + priority +
		" |\n| Updated | " + updated + " |\n\n## Task\n\nDo x.\n"
}

// TestPendingListsUnapprovedReadySpecsByPriority is AC-30. Method: a checkout
// holds ready specs across two categories and two priorities, one of them
// admitted only by a committed away trailer, an approved ready spec, and a
// design spec; pending lists the five unapproved ready specs P1 before P2,
// fixes before validation, older Updated first, each lacking its sections,
// the judge and the approval (an away admission is not an approval), leaves
// out the approved and the design spec, and ends saying Thomas is not away.
func TestPendingListsUnapprovedReadySpecsByPriority(t *testing.T) {
	root := widgetRoot(t)
	gitInit(t, root)
	approvedText := strings.Replace(completeSpecText, "| Status | design |", "| Status | ready |", 1)
	writeTree(t, root, map[string]string{
		"plan/release/fixes/spec-late.md":       readySpec("P1", "2026-11-10"),
		"plan/release/fixes/spec-early.md":      readySpec("P1", "2026-10-02"),
		"plan/release/validation/spec-check.md": readySpec("P1", "2026-09-01"),
		"plan/release/fixes/spec-second.md":     readySpec("P2", "2026-01-01"),
		"plan/release/fixes/spec-approved.md":   approvedText,
		"tmp/spec-design/approved/spec-approved.md": "Spec: spec-approved.md\nDigest: " + specdesign.DesignDigest(approvedText) +
			"\nDate: 2026-11-19\nMachine: m\nOwner words: approved\nJudge: 0 2026-11-19 holds\n",
	})

	awayText := readySpec("P2", "2026-02-01")
	writeTree(t, root, map[string]string{"plan/release/fixes/spec-away.md": awayText})
	if err := designtest.CommitAwayAdmission(root, "plan/release/fixes/spec-away.md", awayText, "2026-11-19", "2026-11-25"); err != nil {
		t.Fatal(err)
	}

	answer, code := specdesign.Answer([]string{"pending"})
	if code != 0 {
		t.Fatalf("pending answered %d, want 0", code)
	}
	rendered, ok := answer.(interface{ Text() string })
	if !ok {
		t.Fatalf("pending answered %T, which renders no text", answer)
	}
	text := rendered.Text()
	order := []string{"spec-early.md", "spec-late.md", "spec-check.md", "spec-second.md", "spec-away.md"}
	last := -1
	for _, name := range order {
		at := strings.Index(text, name)
		if at < 0 {
			t.Fatalf("pending does not list %s:\n%s", name, text)
		}
		if at < last {
			t.Errorf("pending lists %s out of order:\n%s", name, text)
		}
		last = at
	}
	requireContains(t, "pending", text,
		"P1 release/fixes  spec-early.md  ready since 2026-10-02  lacks: Design documents, Design presentation, judge, approval",
		"P1 release/validation  spec-check.md",
		"P2 release/fixes  spec-away.md  ready since 2026-02-01  lacks: Design documents, Design presentation, judge, approval",
		"5 ready specs wait for an approval. Thomas is not away.")
	for _, absent := range []string{"spec-approved.md", "spec-widget.md"} {
		if strings.Contains(text, absent) {
			t.Errorf("pending lists %s, which waits for nothing:\n%s", absent, text)
		}
	}
}
