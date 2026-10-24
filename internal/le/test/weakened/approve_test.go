// Design: docs/contributing/testing.md -- when a test must be weakened
//
// VALIDATES: `./le test weakened approve` writes one row per unit into the
// session's tmp/commit-weakening-approved-<ns>.md, replaces the reason of a
// unit it already names, and refuses a unit or a reason the trailer cannot
// carry before anything is written.
// PREVENTS: an approval that names a tree or a bare test, and a reason that
// would split the trailer line or the table cell.
package testweakened

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/rfc"
)

const approveSession = "0a1b2c3d"

func TestWeakeningApproveWritesTheSessionFile(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	report, err := Approve(root, approveSession, "pkg.TestX", "  words  ")
	if err != nil {
		t.Fatalf("Approve: %v", err)
	}
	if report.Path != "tmp/commit-weakening-approved-"+approveSession+".md" || report.Replaced {
		t.Fatalf("first approval = %+v", report)
	}
	if _, err := Approve(root, approveSession, "pkg.TestY", "other words"); err != nil {
		t.Fatalf("Approve second unit: %v", err)
	}
	report, err = Approve(root, approveSession, "pkg.TestX", "new words")
	if err != nil || !report.Replaced {
		t.Fatalf("re-approval = %+v, %v; want the row replaced", report, err)
	}
	content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(ApprovalPath(approveSession))))
	if err != nil {
		t.Fatalf("read approval file: %v", err)
	}
	rows, problems := ParseLedger(string(content), ApprovalPath(approveSession))
	if len(problems) != 0 {
		t.Fatalf("approval file does not parse: %v\n%s", problems, content)
	}
	if len(rows) != 2 || rows[0].Name != "pkg.TestX" || rows[0].Reason != "new words" ||
		rows[1].Name != "pkg.TestY" || rows[1].Reason != "other words" {
		t.Fatalf("rows = %+v; want pkg.TestX=new words then pkg.TestY=other words", rows)
	}
	if strings.Contains(string(content), rfc.ApprovalTrailer) {
		t.Fatal("the weakening approval file must not be the RFC one")
	}
	if got, want := ApprovalTrailerLine("pkg.TestX", "new words"), "Weakening-approved: pkg.TestX: new words"; got != want {
		t.Fatalf("trailer = %q, want %q", got, want)
	}
}

func TestWeakeningApproveRefusesBadUnitOrReason(t *testing.T) {
	t.Parallel()
	cases := []struct{ name, unit, reason string }{
		{"bare name", "TestX", "words"},
		{"path", "internal/le/pkg.TestX", "words"},
		{"tree scope", "scripts/**", "words"},
		{"empty reason", "pkg.TestX", "   "},
		{"pipe in reason", "pkg.TestX", "a | b"},
		{"newline in reason", "pkg.TestX", "a\nb"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			if _, err := Approve(root, approveSession, tc.unit, tc.reason); err == nil {
				t.Fatalf("Approve(%q, %q) succeeded; want a refusal", tc.unit, tc.reason)
			}
			_, err := os.Stat(filepath.Join(root, filepath.FromSlash(ApprovalPath(approveSession))))
			if !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("a refused approval wrote the file: %v", err)
			}
		})
	}
}
