// Design: docs/contributing/rfc-implementation-guide.md -- terminal-only owner rulings.
package rfc

import (
	"errors"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/le/human"
	"github.com/ze-software/ze/internal/le/le/human/humantest"
)

// TestOwnerRulingApprovalTerminalBoundary drives the real RFC producer with
// terminal, piped and unanswered streams, for new and replacement approvals.
func TestOwnerRulingApprovalTerminalBoundary(t *testing.T) {
	for _, mode := range []string{"terminal", "piped-stdin", "piped-stdout", "piped-both", "closed-stdin", "closed-stdout"} {
		t.Run(mode, func(t *testing.T) {
			humantest.Streams(t, mode)
			for _, existing := range []bool{false, true} {
				name := "new"
				if existing {
					name = "replacement"
				}
				t.Run(name, func(t *testing.T) {
					root, _ := fixtureCommit(t, map[string]string{
						"pkg/x_test.go": "package pkg\n\nimport \"testing\"\n\nfunc TestX(t *testing.T) {}\n",
					})
					if existing {
						if _, err := Approve(root, approvalSession, "pkg.TestX", "independent judge verdict"); err != nil {
							t.Fatal(err)
						}
					}
					before := humantest.Snapshot(t, root)
					report, err := Approve(root, approvalSession, "pkg.TestX", "OWNER RULING (2026-11-03) accepts this boundary")
					if mode == "terminal" {
						if err != nil {
							t.Fatal(err)
						}
						if report.Replaced != existing || !strings.Contains(readApprovals(t, root), report.Reason) {
							t.Fatalf("approval = %+v", report)
						}
						return
					}
					if _, ok := errors.AsType[*human.Refusal](err); !ok {
						t.Errorf("owner ruling answered %v, want terminal refusal", err)
					} else if strings.Contains(err.Error(), "full verification") || !strings.Contains(err.Error(), "./le rfc approve") {
						t.Errorf("authorization refusal = %v", err)
					}
					if !maps.Equal(before, humantest.Snapshot(t, root)) {
						t.Error("refused owner ruling changed files or directories")
					}
				})
			}
		})
	}
}

// TestOrdinaryApprovalRemainsNoninteractive preserves the judge route, the
// shared weakening writer, and noninteractive reading of a recorded ruling.
func TestOrdinaryApprovalRemainsNoninteractive(t *testing.T) {
	humantest.Streams(t, "piped-both")
	root, _ := fixtureCommit(t, map[string]string{
		"pkg/x_test.go": "package pkg\n\nimport \"testing\"\n\nfunc TestX(t *testing.T) {}\n",
	})
	if _, err := Approve(root, approvalSession, "pkg.TestX", "judge: requirement remains proven"); err != nil {
		t.Fatal(err)
	}
	file := ApprovalFile{Path: "tmp/weakening.md", Header: approvalHeader, Command: "test weakened approve"}
	if _, err := ApproveInto(root, file, "pkg.TestX", "owner ruling 2026-11-03 does not gate the shared writer"); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, filepath.FromSlash(ApprovalPath(approvalSession)))
	if err := os.WriteFile(path, []byte(approvalHeader+"| pkg.TestX | owner ruling 2026-11-03 |  | none..none |\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if ruling := ownerRuling(readApprovals(t, root)); ruling != "2026-11-03" {
		t.Fatalf("recorded ruling = %q", ruling)
	}
}
