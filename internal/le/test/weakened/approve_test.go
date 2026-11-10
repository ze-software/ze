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
	// The writer resolves each unit to its carrier before writing, so the
	// fixture tree holds the two tests it approves.
	writeApproveCarrier(t, root, "pkg/x_test.go",
		"package pkg\n\nfunc TestX(t *testing.T) {}\n\nfunc TestY(t *testing.T) {}\n")
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

func writeApproveCarrier(t *testing.T, root, path, content string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

// TestWeakenedApproveRefusesPackageClauseAlias proves `./le test weakened
// approve` refuses a unit spelled with the Go package clause when the
// directory that holds the test is named otherwise. Every consumer keys a
// unit by its directory's base name, so `repoinventory.TestX` for a test in
// internal/le/repo/inventory would be written and then never matched. The
// refusal names the directory key, nothing is written, and that key is
// accepted.
//
// MUTATION: guard the carrier resolution in rfc.ApproveInto with
// `file.Target != ""` again (the old shape), and `repoinventory.TestX` is
// written; drop noApprovalCarrier's package-clause search and the refusal no
// longer names `inventory.TestX`.
func TestWeakenedApproveRefusesPackageClauseAlias(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeApproveCarrier(t, root, "internal/le/repo/inventory/inventory_test.go",
		"package repoinventory\n\nfunc TestX(t *testing.T) {}\n")
	approvals := filepath.Join(root, filepath.FromSlash(ApprovalPath(approveSession)))

	_, err := Approve(root, approveSession, "repoinventory.TestX", "judge: the alias")
	if err == nil || !strings.Contains(err.Error(), "record inventory.TestX instead") {
		t.Fatalf("Approve(repoinventory.TestX) = %v; want a refusal naming inventory.TestX", err)
	}
	if _, statErr := os.Stat(approvals); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("a refused alias wrote the approval file: %v", statErr)
	}
	_, err = Approve(root, approveSession, "nothing.TestX", "judge: unknown")
	if err == nil || !strings.Contains(err.Error(), "has no current/parent carrier") ||
		strings.Contains(err.Error(), "instead") {
		t.Fatalf("Approve(nothing.TestX) = %v; want a plain unknown-unit refusal", err)
	}
	if _, err := Approve(root, approveSession, "inventory.TestX", "judge: the directory key"); err != nil {
		t.Fatalf("Approve(inventory.TestX) = %v; want the directory key accepted", err)
	}
}

// TestOrdinaryApprovalRefusesChangedUnitContent proves an ordinary approval
// row judges the unit content it was recorded against, not the unit's name.
// The row recorded after a change admits that change; once the unit changes
// again the loader marks the row stale, so no gate admits the later change on
// the earlier verdict (the 06975b27c5 redistribute-import-modify case). The
// admission rule is then checked on its own for a verdict recorded before
// the edit, which admits any change from the base it saw and none from a
// moved base.
//
// MUTATION: make staleApproval answer "" (a row keyed by name alone) and the
// changed-again case stays live; make ApprovalContentAdmits ignore the base
// and the moved-base case admits.
func TestOrdinaryApprovalRefusesChangedUnitContent(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	const judged = "package pkg\n\nfunc TestA(t *testing.T) {\n\tt.Skip(\"judged\")\n}\n"
	writeApproveCarrier(t, root, "pkg/a_test.go", judged)
	if _, err := Approve(root, approveSession, "pkg.TestA", "judge: the skip is the change I read"); err != nil {
		t.Fatalf("Approve: %v", err)
	}
	source := WeakeningApprovals(approveSession)
	rows, problems := LoadApprovals(root, source)
	if len(problems) != 0 || len(rows) != 1 || rows[0].Stale != "" {
		t.Fatalf("judged content: rows %+v, problems %v; want one live row", rows, problems)
	}

	writeApproveCarrier(t, root, "pkg/a_test.go",
		"package pkg\n\nfunc TestA(t *testing.T) {\n\tt.Skip(\"another change\")\n}\n")
	rows, problems = LoadApprovals(root, source)
	if len(problems) != 0 || len(rows) != 1 || rows[0].Stale == "" {
		t.Fatalf("changed again: rows %+v, problems %v; want the row stale", rows, problems)
	}

	base, other := strings.Repeat("a", 64), strings.Repeat("b", 64)
	before := rfc.ApprovalUnitContent{Base: base, Current: other}
	if !rfc.ApprovalContentAdmits(base, base, before) {
		t.Fatal("a verdict recorded before the edit must admit a change from its base")
	}
	if rfc.ApprovalContentAdmits(other, other, before) {
		t.Fatal("a verdict recorded before the edit must not admit a change from a moved base")
	}
	if rfc.ApprovalContentAdmits(base, strings.Repeat("c", 64), before) {
		t.Fatal("a verdict recorded after the edit must not admit other content")
	}
}
