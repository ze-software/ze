// Design: docs/contributing/rfc-implementation-guide.md -- approval of an exact historical change.
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

// TestTargetApprovalUsesHistoricalCanonicalCarriers reaches the registered
// writer with a full commit identity. Dirty current bytes cannot substitute for
// either target side, and deleting the entire carrier remains approvable.
func TestTargetApprovalUsesHistoricalCanonicalCarriers(t *testing.T) {
	for _, carrier := range []struct {
		name, path, unit, before, after string
	}{
		{"Go", "internal/canonical/identity_test.go", "canonical.TestIdentity", "package alias\n// RFC requirement: RFC2119-1-1 positive\nfunc TestIdentity(t *testing.T) {\n\tcheck(1)\n}\n", "package alias\n// RFC requirement: RFC2119-1-1 positive\nfunc TestIdentity(t *testing.T) {\n\tcheck(2)\n}\n"},
		{"non-Go", "test/runner/identity.ci", "runner.identity", "# RFC requirement: RFC2119-1-1 positive\nexec=ze help\nexpect=stdout:pattern=before\n", "# RFC requirement: RFC2119-1-1 positive\nexec=ze help\nexpect=stdout:pattern=after\n"},
		{"semantic", "internal/canonical/identity_test.go", "canonical.TestIdentity", "package alias\nfunc TestIdentity(t *testing.T) {\n\tif got() != 1 { t.Fatal(\"wrong\") }\n}\n", "package alias\nfunc TestIdentity(t *testing.T) {\n\tif got() != 2 { t.Fatal(\"wrong\") }\n}\n"},
		{"deleted", "internal/canonical/identity_test.go", "canonical.TestIdentity", "package alias\n// RFC requirement: RFC2119-1-1 positive\nfunc TestIdentity(t *testing.T) {\n\tcheck(1)\n}\n", ""},
	} {
		t.Run(carrier.name, func(t *testing.T) {
			root, _ := fixtureCommit(t, map[string]string{carrier.path: carrier.before})
			writeApprovalIdentityFixture(t, root, carrier.path, carrier.before)
			fixtureGit(t, root, "", "read-tree", "HEAD")
			if carrier.after == "" {
				if err := os.Remove(filepath.Join(root, carrier.path)); err != nil {
					t.Fatal(err)
				}
				fixtureGit(t, root, "", "update-index", "--force-remove", "--", carrier.path)
			} else {
				writeApprovalIdentityFixture(t, root, carrier.path, carrier.after)
				fixtureGit(t, root, "", "add", "--", carrier.path)
			}
			fixtureGit(t, root, "", "-c", "user.name=Fixture", "-c", "user.email=fixture@example.test", "-c", "commit.gpgsign=false", "commit", "-q", "-m", "reviewed change")
			target := fixtureGit(t, root, "", "rev-parse", "HEAD")
			writeApprovalIdentityFixture(t, root, carrier.path, "unselected current bytes must not become the historical carrier\n")
			t.Setenv("ZE_REPO_ROOT", root)
			t.Setenv("CLAUDE_CODE_SESSION_ID", "target-approval-fixture")
			env.ResetCache()
			t.Cleanup(env.ResetCache)
			payload, code := Answer([]string{"approve", "commit", target, "unit", carrier.unit, "reason", "judge: reviewed this exact historical change"})
			if code != 0 {
				t.Fatalf("target approval = (%#v, %d), want success", payload, code)
			}
			paths, err := filepath.Glob(filepath.Join(root, "tmp", "commit-rfc-approved-*.md"))
			if err != nil {
				t.Fatal(err)
			}
			if len(paths) != 1 {
				t.Fatalf("approval files = %v, want existing single session ledger", paths)
			}
			body, err := os.ReadFile(paths[0])
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(body), target) {
				t.Fatalf("approval lost exact target identity: %s", body)
			}
			if !strings.Contains(string(body), carrier.unit) {
				t.Fatalf("approval lost canonical unit: %s", body)
			}
		})
	}
}

// TestTargetApprovalRefusesAliasesUnchangedAndNoncanonicalRevisions keeps the
// registered writer's complete filesystem unchanged on invalid target identity.
func TestTargetApprovalRefusesAliasesUnchangedAndNoncanonicalRevisions(t *testing.T) {
	for _, shape := range []string{"abbreviated", "symbolic", "unknown SHA", "unknown unit", "package alias", "unchanged unit", "ambiguous carrier"} {
		t.Run(shape, func(t *testing.T) {
			const carrier = "package alias\n// RFC requirement: RFC2119-1-1 positive\nfunc TestIdentity(t *testing.T) {\n\tcheck(1)\n}\n"
			files := map[string]string{"internal/canonical/identity_test.go": carrier}
			if shape == "ambiguous carrier" {
				files = map[string]string{"internal/one/canonical/identity_test.go": carrier, "internal/two/canonical/identity_test.go": carrier}
			}
			root, parent := fixtureCommit(t, files)
			var change strings.Builder
			change.WriteString("commit refs/heads/main\ncommitter Fixture <fixture@example.test> 1700000001 +0000\ndata 6\ntarget\nfrom " + parent + "\n")
			for path, body := range files {
				if shape != "unchanged unit" {
					body = strings.Replace(body, "check(1)", "check(2)", 1)
				}
				object := fixtureGit(t, root, body, "hash-object", "-w", "--stdin")
				change.WriteString("M 100644 " + object + " " + path + "\n")
			}
			change.WriteString("\ndone\n")
			fixtureGit(t, root, change.String(), "fast-import", "--quiet")
			target := fixtureGit(t, root, "", "rev-parse", "HEAD")
			unit := "canonical.TestIdentity"
			if shape == "abbreviated" {
				target = target[:12]
			}
			if shape == "symbolic" {
				target = "HEAD"
			}
			if shape == "unknown SHA" {
				target = strings.Repeat("f", 40)
			}
			if shape == "unknown unit" {
				unit = "canonical.TestMissing"
			}
			if shape == "package alias" {
				unit = "alias.TestIdentity"
			}
			t.Setenv("ZE_REPO_ROOT", root)
			t.Setenv("CLAUDE_CODE_SESSION_ID", "target-approval-refusal")
			env.ResetCache()
			t.Cleanup(env.ResetCache)
			before := humantest.Snapshot(t, root)
			if _, code := Answer([]string{"approve", "commit", target, "unit", unit, "reason", "judge: reviewed"}); code != 2 {
				t.Errorf("invalid %s target approval exit = %d, want 2", shape, code)
			}
			if !maps.Equal(before, humantest.Snapshot(t, root)) {
				t.Error("invalid target approval wrote a namespace or approval before refusal")
			}
		})
	}
}
