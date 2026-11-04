// Design: docs/architecture/testing/test-health.md -- semantic RFC approvals use selected existing units.
// Related: semantic_approval_test.go -- weakening approval selection and publication fixtures.
package commit

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/rfc"
	testweakened "github.com/ze-software/ze/internal/le/test/weakened"
)

const semanticRFCBaseline = "package pkg\n\n// RFC requirement: RFC2119-1-1 positive\nfunc TestOne(t *testing.T) {\n\trequire.Equal(t, 1, got)\n}\n"

// TestCreateSemanticRFCApproval proves comment-only approvals survive native
// preparation, publication and pruning, while overlapping selectors emit one row.
func TestCreateSemanticRFCApproval(t *testing.T) {
	for _, mode := range []string{"comment only", "structural overlap", "weakening precedence"} {
		t.Run(mode, func(t *testing.T) {
			root := newLedgerRepository(t)
			const path = "pkg/one_test.go"
			commitFixture(t, root, "tagged baseline", map[string]string{path: semanticRFCBaseline}, nil)
			changed := strings.Replace(semanticRFCBaseline, "1, got)", "1, got) // Explain the assertion.", 1)
			if mode == "structural overlap" {
				changed = strings.Replace(semanticRFCBaseline, "require.Equal", "t.Skip(\"judged\")\n\trequire.Equal", 1)
			}
			writeCommitFixture(t, root, path, changed)
			const unit = "pkg.TestOne"
			const reason = "independent judge approved this exact test change"
			if _, err := rfc.Approve(root, sessionOne, unit, reason); err != nil {
				t.Fatal(err)
			}
			writeCommitFixture(t, root, "pkg/unused_test.go", "package pkg\n\nimport \"testing\"\n\nfunc TestUnused(t *testing.T) {}\n")
			if _, err := rfc.Approve(root, sessionOne, "pkg.TestUnused", "reserved verdict"); err != nil {
				t.Fatal(err)
			}
			rfcPath := rfc.ApprovalPath(sessionOne)
			before := readLedgerFile(t, root, rfcPath)
			trailer := rfc.ApprovalTrailerLine(unit, reason)
			if mode == "weakening precedence" {
				if _, err := testweakened.Approve(root, sessionOne, unit, reason); err != nil {
					t.Fatal(err)
				}
				trailer = testweakened.ApprovalTrailerLine(unit, reason)
			}
			prepared, err := ledgerCreate(t, root, path)
			if err != nil {
				t.Fatal(err)
			}
			if mode == "structural overlap" {
				if len(prepared.Weakened) != 1 {
					t.Fatalf("structural fixture findings = %v", prepared.Weakened)
				}
				if len(prepared.RFCChanges) != 1 {
					t.Fatalf("RFC fixture changes = %v", prepared.RFCChanges)
				}
			} else {
				if len(prepared.Weakened) != 0 {
					t.Fatalf("comment edit has structural findings: %v", prepared.Weakened)
				}
				if len(prepared.RFCChanges) != 0 {
					t.Fatalf("comment edit changed RFC behavior: %v", prepared.RFCChanges)
				}
			}
			message := readLedgerFile(t, root, prepared.Message)
			if strings.Count(message, "\n"+trailer+"\n") != 1 {
				t.Fatalf("exact semantic verdict missing or duplicated:\n%s", message)
			}
			if strings.Count(message, "-approved:") != 1 {
				t.Fatalf("unexpected extra verdict:\n%s", message)
			}
			if got := readLedgerFile(t, root, rfcPath); got != before {
				t.Fatalf("preparation consumed an RFC approval: %q", got)
			}
			runCommitScript(t, root, prepared.Script)
			if body := runCommitGitOutput(t, root, "log", "-1", "--format=%B"); strings.Count(body, "\n"+trailer+"\n") != 1 {
				t.Fatalf("published message lost the exact verdict:\n%s", body)
			}
			if got := runCommitGitOutput(t, root, "show", "HEAD:"+path); got != changed {
				t.Fatalf("published content = %q, want %q", got, changed)
			}
			after := readLedgerFile(t, root, rfcPath)
			if mode == "weakening precedence" {
				if after != before {
					t.Fatalf("weakening precedence consumed unused RFC row: %q", after)
				}
				if got := readLedgerFile(t, root, testweakened.ApprovalPath(sessionOne)); strings.Contains(got, "| "+unit+" |") {
					t.Fatalf("used weakening row survived: %q", got)
				}
				return
			}
			if strings.Contains(after, "| "+unit+" |") {
				t.Fatalf("used RFC row survived: %q", after)
			}
			if !strings.Contains(after, "| pkg.TestUnused | reserved verdict |") {
				t.Fatalf("unrelated RFC row lost: %q", after)
			}
			if mode == "comment only" {
				// A failed prune must not let this already-published verdict recur.
				if _, err := rfc.Approve(root, sessionOne, unit, reason); err != nil {
					t.Fatal(err)
				}
				writeCommitFixture(t, root, path, strings.Replace(changed, "Explain", "Clarify", 1))
				second, err := ledgerCreate(t, root, path)
				if err != nil {
					t.Fatal(err)
				}
				if got := readLedgerFile(t, root, second.Message); strings.Contains(got, "-approved:") {
					t.Fatalf("already-carried verdict reused: %q", got)
				}
			}
		})
	}
}

// TestCreateSemanticRFCApprovalSelection proves that absent, unrelated,
// unchanged, excluded and foreign approvals never enter a fixture publication.
func TestCreateSemanticRFCApprovalSelection(t *testing.T) {
	for _, mode := range []string{"unchanged unit", "excluded hunk", "foreign session", "unrelated row", "no approval"} {
		t.Run(mode, func(t *testing.T) {
			root := newLedgerRepository(t)
			const path = "pkg/one_test.go"
			head := semanticRFCBaseline + "\nfunc TestOther(t *testing.T) {\n\trequire.Equal(t, 1, other)\n}\n"
			commitFixture(t, root, "tagged baseline", map[string]string{path: head}, nil)
			changed := strings.Replace(head, "1, got)", "1, got) // Explain the assertion.", 1)
			options := &Options{Session: sessionOne, Subject: "selected comment", Files: []string{path}}
			session, unit := sessionOne, "pkg.TestOne"
			switch mode {
			case "unchanged unit":
				writeCommitFixture(t, root, path, strings.Replace(head, "1, other", "2, other", 1))
			case "excluded hunk":
				patch := writeHunkPatch(t, root, path, head+"\n// Selected file comment.\n", changed+"\n// Selected file comment.\n")
				options.Files = nil
				options.Hunks = []HunkSelection{{Path: path, Patch: patch}}
			case "foreign session":
				session = sessionTwo
				writeCommitFixture(t, root, path, changed)
			case "unrelated row":
				unit = "pkg.TestOther"
				writeCommitFixture(t, root, path, changed)
			case "no approval":
				writeCommitFixture(t, root, path, changed)
			default:
				t.Fatalf("unknown fixture %q", mode)
			}
			before := ""
			if mode != "no approval" {
				if _, err := rfc.Approve(root, session, unit, "reserved verdict"); err != nil {
					t.Fatal(err)
				}
				before = readLedgerFile(t, root, rfc.ApprovalPath(session))
			}
			prepared, err := Create(root, options)
			if err != nil {
				t.Fatal(err)
			}
			if message := readLedgerFile(t, root, prepared.Message); strings.Contains(message, "-approved:") {
				t.Fatalf("unused approval entered message: %q", message)
			}
			runCommitScript(t, root, prepared.Script)
			if body := runCommitGitOutput(t, root, "log", "-1", "--format=%B"); strings.Contains(body, "-approved:") {
				t.Fatalf("unused approval published: %q", body)
			}
			if mode != "no approval" {
				if after := readLedgerFile(t, root, rfc.ApprovalPath(session)); after != before {
					t.Fatalf("unused approval changed: before %q, after %q", before, after)
				}
			}
		})
	}
}

// TestCreateSemanticRFCApprovalCollision proves RFC fallback contributes its
// selected units to the same directory-ambiguity guard as structural findings.
func TestCreateSemanticRFCApprovalCollision(t *testing.T) {
	for _, mixed := range []bool{false, true} {
		t.Run(map[bool]string{false: "both semantic", true: "semantic and structural"}[mixed], func(t *testing.T) {
			root := newCommitRepository(t)
			configureCommitAuthor(t, root)
			paths := []string{"x/pkg/tagged_test.go", "y/pkg/tagged_test.go"}
			commitFixture(t, root, "first collision baseline", map[string]string{paths[0]: semanticRFCBaseline}, nil)
			if _, err := rfc.Approve(root, sessionOne, "pkg.TestOne", "ambiguous verdict"); err != nil {
				t.Fatal(err)
			}
			commitFixture(t, root, "second collision baseline", map[string]string{paths[1]: semanticRFCBaseline}, nil)
			for index, path := range paths {
				changed := strings.Replace(semanticRFCBaseline, "1, got)", "1, got) // Explain the assertion.", 1)
				if mixed {
					if index == 1 {
						changed = strings.Replace(semanticRFCBaseline, "require.Equal", "t.Skip(\"judged\")\n\trequire.Equal", 1)
					}
				}
				writeCommitFixture(t, root, path, changed)
			}
			before := readLedgerFile(t, root, rfc.ApprovalPath(sessionOne))
			_, err := ledgerCreate(t, root, paths...)
			if err == nil {
				t.Fatal("one RFC approval admitted units in two directories")
			}
			for _, want := range []string{"pkg.TestOne", "x/pkg", "y/pkg", "split the commit"} {
				if !strings.Contains(err.Error(), want) {
					t.Fatalf("refusal lacks %q: %v", want, err)
				}
			}
			if after := readLedgerFile(t, root, rfc.ApprovalPath(sessionOne)); after != before {
				t.Fatalf("refused preparation consumed approval: %q", after)
			}
		})
	}
}
