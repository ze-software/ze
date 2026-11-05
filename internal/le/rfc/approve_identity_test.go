// Design: docs/contributing/rfc-implementation-guide.md -- approval identity before publication.
package rfc

import (
	"maps"
	"os"
	"path/filepath"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/le/le/human/humantest"
)

// TestApprovalRefusesUnknownAliasAndAmbiguousCarriers checks both the callable
// writer and registered entry against real HEAD/current carrier populations.
// Every refusal must leave even the session namespace unchanged.
func TestApprovalRefusesUnknownAliasAndAmbiguousCarriers(t *testing.T) {
	const before = "package alias\n\nimport \"testing\"\n\nfunc TestIdentity(t *testing.T) {\n\tif got() != 1 { t.Fatal(\"wrong value\") }\n}\n"
	const after = "package alias\n\nimport \"testing\"\n\nfunc TestIdentity(t *testing.T) {\n\tif got() != 2 { t.Fatal(\"wrong value\") }\n}\n"
	for _, entry := range []string{"callable", "registered"} {
		for _, identity := range []string{"unknown", "package alias", "ambiguous directory"} {
			t.Run(entry+"/"+identity, func(t *testing.T) {
				files := map[string]string{"internal/canonical/identity_test.go": before}
				unit := "missing.TestIdentity"
				if identity == "package alias" {
					unit = "alias.TestIdentity"
				}
				if identity == "ambiguous directory" {
					files = map[string]string{"internal/one/shared/identity_test.go": before, "internal/two/shared/identity_test.go": before}
					unit = "shared.TestIdentity"
				}
				root, _ := fixtureCommit(t, files)
				for path := range files {
					writeApprovalIdentityFixture(t, root, path, after)
				}
				t.Setenv("ZE_REPO_ROOT", root)
				t.Setenv("CLAUDE_CODE_SESSION_ID", "approval-identity-refusal")
				env.ResetCache()
				t.Cleanup(env.ResetCache)
				snapshot := humantest.Snapshot(t, root)
				if entry == "callable" {
					if _, err := Approve(root, approvalSession, unit, "judge: the reviewed assertion is correct"); err == nil {
						t.Errorf("Approve accepted noncanonical identity %q", unit)
					}
				} else {
					if _, code := Answer([]string{"approve", "unit", unit, "reason", "judge: the reviewed assertion is correct"}); code != 2 {
						t.Errorf("registered approval exit = %d, want 2 for %q", code, unit)
					}
				}
				if !maps.Equal(snapshot, humantest.Snapshot(t, root)) {
					t.Error("invalid carrier identity wrote an approval or namespace before refusal")
				}
			})
		}
	}
}

// TestApprovalCanonicalCarrierIncludesSemanticAndDeletedUnits supplies the
// canonical directory key despite a different package clause, a semantic-only
// assertion edit, a non-Go carrier and a unit deleted from the current tree.
func TestApprovalCanonicalCarrierIncludesSemanticAndDeletedUnits(t *testing.T) {
	for _, shape := range []struct {
		name, path, unit, before, after string
	}{
		{"semantic", "internal/canonical/identity_test.go", "canonical.TestIdentity", "package alias\nfunc TestIdentity(t *testing.T) {\n\tif got() != 1 { t.Fatal(\"wrong value\") }\n}\n", "package alias\nfunc TestIdentity(t *testing.T) {\n\tif got() != 2 { t.Fatal(\"wrong value\") }\n}\n"},
		{"non-Go", "test/runner/identity.ci", "runner.identity", "# RFC requirement: RFC2119-1-1 positive\nexec=ze help\nexpect=stdout:pattern=before\n", "# RFC requirement: RFC2119-1-1 positive\nexec=ze help\nexpect=stdout:pattern=after\n"},
		{"deleted", "internal/canonical/identity_test.go", "canonical.TestIdentity", "package alias\n// RFC requirement: RFC2119-1-1 positive\nfunc TestIdentity(t *testing.T) {\n\tt.Fatal(\"required assertion\")\n}\n", "package alias\n"},
	} {
		t.Run(shape.name, func(t *testing.T) {
			root, _ := fixtureCommit(t, map[string]string{shape.path: shape.before})
			writeApprovalIdentityFixture(t, root, shape.path, shape.after)
			report, err := Approve(root, approvalSession, shape.unit, "judge: reviewed the exact carrier change")
			if err != nil {
				t.Fatal(err)
			}
			if report.Unit != shape.unit {
				t.Fatalf("approval unit = %q, want %q", report.Unit, shape.unit)
			}
			if report.Replaced {
				t.Fatal("first approval unexpectedly replaced another row")
			}
		})
	}
}

func writeApprovalIdentityFixture(t *testing.T, root, path, body string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}
