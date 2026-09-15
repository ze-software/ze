// Design: docs/contributing/rfc-implementation-guide.md -- owner approval for RFC-tagged test changes
// Related: approve.go -- the session file the hook and the commit gate read.
package rfc

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestApproveWritesTheSessionRow proves AC-1: one command writes one row into
// the session's approval file, a second call for the same unit replaces the
// reason rather than adding a row, and a malformed call writes nothing.
func TestApproveWritesTheSessionRow(t *testing.T) {
	root := t.TempDir()
	const session = "abcd1234"

	report, err := Approve(root, session, "pkg.TestX", "Thomas approved the new count")
	if err != nil {
		t.Fatalf("approve: %v", err)
	}
	if report.Path != ApprovalPath(session) || report.Replaced {
		t.Fatalf("first approval = %#v", report)
	}
	content := readApprovals(t, root, session)
	if strings.Count(content, "| pkg.TestX |") != 1 ||
		!strings.Contains(content, "| pkg.TestX | Thomas approved the new count |") {
		t.Fatalf("first approval wrote:\n%s", content)
	}

	report, err = Approve(root, session, "pkg.TestX", "Thomas changed his mind: the old count")
	if err != nil {
		t.Fatalf("second approval: %v", err)
	}
	if !report.Replaced {
		t.Fatalf("second approval = %#v, want Replaced", report)
	}
	content = readApprovals(t, root, session)
	if strings.Count(content, "| pkg.TestX |") != 1 ||
		strings.Contains(content, "the new count") ||
		!strings.Contains(content, "| pkg.TestX | Thomas changed his mind: the old count |") {
		t.Fatalf("second approval wrote:\n%s", content)
	}

	if _, err := Approve(root, session, "other.TestY", "a second unit"); err != nil {
		t.Fatalf("second unit: %v", err)
	}
	if content = readApprovals(t, root, session); strings.Count(content, "\n| ") != 3 {
		t.Fatalf("two units did not give two rows under the header:\n%s", content)
	}

	refused := []struct {
		name, unit, reason string
	}{
		{"missing reason", "pkg.TestZ", ""},
		{"blank reason", "pkg.TestZ", "   "},
		{"newline in reason", "pkg.TestZ", "yes\nno"},
		{"pipe in reason", "pkg.TestZ", "yes | no"},
		{"unit without package", "TestZ", "yes"},
		{"unit with a path", "internal/pkg.TestZ", "yes"},
		{"unit with a space", "pkg.Test Z", "yes"},
		{"empty unit", "", "yes"},
	}
	before := readApprovals(t, root, session)
	for _, tc := range refused {
		if _, err := Approve(root, session, tc.unit, tc.reason); err == nil {
			t.Errorf("%s: approved unit %q reason %q", tc.name, tc.unit, tc.reason)
		}
	}
	if after := readApprovals(t, root, session); after != before {
		t.Fatalf("a refused approval changed the file:\n%s", after)
	}

	// The entry point: the action refuses a call without its reason.
	if _, code := Answer([]string{"approve", "unit", "pkg.TestX"}); code == 0 {
		t.Fatal("rfc approve without a reason exited 0")
	}
}

func readApprovals(t *testing.T, root, session string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(ApprovalPath(session))))
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}
