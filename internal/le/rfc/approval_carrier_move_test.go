// Design: docs/contributing/rfc-implementation-guide.md -- resolve canonical unit identity independently on each side.
package rfc

import (
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/le/le/human/humantest"
)

// TestApprovalResolvesUniqueCarrierMovesAndRefusesSimultaneousDuplicates drives
// current/HEAD and exact parent/target populations through the native writer.
// Moving the only carrier does not make two simultaneous units; actually keeping
// both same-directory functions must still refuse without namespace writes.
func TestApprovalResolvesUniqueCarrierMovesAndRefusesSimultaneousDuplicates(t *testing.T) {
	const oldPath = "pkg/old_test.go"
	const newPath = "pkg/new_test.go"
	const unit = "pkg.TestIdentity"
	const before = "package pkg\nimport \"testing\"\nfunc TestIdentity(t *testing.T) { check(1) }\n"
	const after = "package pkg\nimport \"testing\"\nfunc TestIdentity(t *testing.T) { check(2) }\n"
	for _, side := range []string{"current", "target"} {
		for _, duplicate := range []bool{false, true} {
			name := side + "/move"
			if duplicate {
				name = side + "/simultaneous duplicates"
			}
			t.Run(name, func(t *testing.T) {
				root, parent := fixtureCommit(t, map[string]string{oldPath: before})
				fixtureGit(t, root, "", "read-tree", "HEAD")
				target := ""
				if side == "current" {
					writeApprovalIdentityFixture(t, root, newPath, after)
					if duplicate {
						writeApprovalIdentityFixture(t, root, oldPath, before)
					}
				} else {
					blob := fixtureGit(t, root, after, "hash-object", "-w", "--stdin")
					fixtureGit(t, root, "", "update-index", "--add", "--cacheinfo", "100644", blob, newPath)
					if !duplicate {
						fixtureGit(t, root, "", "update-index", "--force-remove", "--", oldPath)
					}
					tree := fixtureGit(t, root, "", "write-tree")
					target = fixtureGit(t, root, "move carrier\n", "-c", "user.name=Fixture", "-c", "user.email=fixture@example.test",
						"-c", "commit.gpgsign=false", "commit-tree", tree, "-p", parent)
					fixtureGit(t, root, "", "update-ref", "HEAD", target, parent)
				}
				t.Setenv("ZE_REPO_ROOT", root)
				t.Setenv("CLAUDE_CODE_SESSION_ID", "approval-carrier-move-fixture")
				env.ResetCache()
				t.Cleanup(env.ResetCache)
				args := []string{"approve", "unit", unit, "reason", "judge: reviewed this exact carrier move"}
				if target != "" {
					args = append(args, "commit", target)
				}
				snapshot := humantest.Snapshot(t, root)
				payload, code := Answer(args)
				if duplicate {
					if code != 2 {
						t.Errorf("simultaneous carriers exit = %d, want 2", code)
					}
					if !maps.Equal(snapshot, humantest.Snapshot(t, root)) {
						t.Error("ambiguous unit wrote a namespace or approval")
					}
					return
				}
				if code != 0 {
					t.Fatalf("unique moved carrier approval = (%#v, %d), want success", payload, code)
				}
				report, ok := payload.(ApprovalReport)
				if !ok {
					t.Fatalf("approval result = %T, want ApprovalReport", payload)
				}
				if report.Unit != unit || report.Target != target {
					t.Fatalf("approval identity = %+v, want target %q and unit %q", report, target, unit)
				}
				content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(report.Path)))
				if err != nil {
					t.Fatal(err)
				}
				if !strings.Contains(string(content), unit) || !strings.Contains(string(content), report.Reason) {
					t.Fatalf("approval did not persist the original unit/verdict: %s", content)
				}
			})
		}
	}
}
