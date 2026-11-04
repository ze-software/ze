// Design: docs/contributing/rfc-implementation-guide.md -- historical approval requires the exact named unit to change.
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

// TestTargetApprovalExactOneLineBoundary drives the registered writer with
// immutable parent/target blobs. A following variable is outside the unchanged
// function; a separate real function edit remains approvable.
// Source-only regression hypothesis until the main verifier observes RED.
func TestTargetApprovalExactOneLineBoundary(t *testing.T) {
	const path = "pkg/identity_test.go"
	const unit = "pkg.TestIdentity"
	const prefix = "package pkg\nimport \"testing\"\n// RFC requirement: RFC2119-1-1 positive\n"
	const function = "func TestIdentity(t *testing.T) { check(1) }\n"
	const before = prefix + function + "var unrelated = 1\n"
	for _, change := range []struct {
		name, after string
		accept      bool
	}{
		{"following_declaration_only", prefix + function + "var unrelated = 2\n", false},
		{"actual_function_change", prefix + "func TestIdentity(t *testing.T) { check(2) }\nvar unrelated = 1\n", true},
	} {
		t.Run(change.name, func(t *testing.T) {
			root, parent := fixtureCommit(t, map[string]string{path: before})
			fixtureGit(t, root, "", "read-tree", "HEAD")
			blob := fixtureGit(t, root, change.after, "hash-object", "-w", "--stdin")
			fixtureGit(t, root, "", "update-index", "--add", "--cacheinfo", "100644", blob, path)
			tree := fixtureGit(t, root, "", "write-tree")
			target := fixtureGit(t, root, "exact unit boundary\n", "-c", "user.name=Fixture", "-c", "user.email=fixture@example.test",
				"-c", "commit.gpgsign=false", "commit-tree", tree, "-p", parent)
			fixtureGit(t, root, "", "update-ref", "HEAD", target, parent)
			if got := fixtureGit(t, root, "", "show", parent+":"+path); got != strings.TrimSpace(before) {
				t.Fatalf("parent blob = %q, want %q", got, before)
			}
			if got := fixtureGit(t, root, "", "show", target+":"+path); got != strings.TrimSpace(change.after) {
				t.Fatalf("target blob = %q, want %q", got, change.after)
			}
			t.Setenv("ZE_REPO_ROOT", root)
			t.Setenv("CLAUDE_CODE_SESSION_ID", "approval-exact-boundary-writer")
			t.Setenv("ZE_AGENT_ID", "")
			env.ResetCache()
			t.Cleanup(env.ResetCache)
			const reason = "judge: reviewed this exact historical function change"
			beforeWrite := humantest.Snapshot(t, root)
			payload, code := Answer([]string{"approve", "commit", target, "unit", unit, "reason", reason})
			if !change.accept {
				if code != 2 {
					t.Errorf("unchanged one-line function approval = (%#v, %d), want refusal exit 2", payload, code)
				}
				if !maps.Equal(beforeWrite, humantest.Snapshot(t, root)) {
					t.Error("unchanged historical unit wrote a namespace, approval, or other fixture bytes")
				}
				return
			}
			if code != 0 {
				t.Fatalf("actual function-change approval = (%#v, %d), want success", payload, code)
			}
			report, ok := payload.(ApprovalReport)
			if !ok {
				t.Fatalf("approval result = %T, want ApprovalReport", payload)
			}
			if report.Unit != unit {
				t.Fatalf("approved unit = %q, want %q", report.Unit, unit)
			}
			if report.Target != target {
				t.Fatalf("approved target = %q, want %q", report.Target, target)
			}
			content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(report.Path)))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(content), "| "+unit+" | "+reason+" | "+target+" |") {
				t.Fatalf("real function edit did not persist its exact judge row: %s", content)
			}
		})
	}
}
