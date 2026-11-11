// Design: docs/architecture/testing/test-health.md -- canonical approval identity before publication.
package testweakened

import (
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/le/le/human/humantest"
	lepath "github.com/ze-software/ze/internal/le/le/path"
	"github.com/ze-software/ze/internal/le/rfc"
)

const weakeningIdentityPath = "internal/canonical/identity_test.go"
const weakeningIdentityUnit = "canonical.TestIdentity"
const weakeningIdentityReason = "judge: the exact synthetic carrier change was reviewed"
const weakeningIdentityBefore = "package carrieralias\n\nimport \"testing\"\n\n" +
	"func TestIdentity(t *testing.T) {\n\tif got := 1; got != 1 {\n\t\tt.Fatal(\"unexpected value\")\n\t}\n}\n"

// TestWeakeningApprovalValidatesCarrierBeforeWrites reaches both callable and
// registered writers. A plausible spelling is not authority: each rejected
// identity or reason must preserve every file, directory and permission.
func TestWeakeningApprovalValidatesCarrierBeforeWrites(t *testing.T) {
	for _, entry := range []string{"callable", "registered", "prepared"} {
		for _, test := range []struct {
			name, unit, reason string
			ambiguous          bool
		}{
			{name: "unknown directory", unit: "missing.TestIdentity", reason: weakeningIdentityReason},
			{name: "unknown declaration", unit: "canonical.TestMissing", reason: weakeningIdentityReason},
			{name: "package clause alias", unit: "carrieralias.TestIdentity", reason: weakeningIdentityReason},
			{name: "ambiguous directory key", unit: "shared.TestIdentity", reason: weakeningIdentityReason, ambiguous: true},
			{name: "empty reason", unit: weakeningIdentityUnit, reason: "   "},
			{name: "table separator in reason", unit: weakeningIdentityUnit, reason: "judge: first | second"},
			{name: "newline in reason", unit: weakeningIdentityUnit, reason: "judge: first\nsecond"},
			{name: "carriage return in reason", unit: weakeningIdentityUnit, reason: "judge: first\rsecond"},
		} {
			t.Run(entry+"/"+test.name, func(t *testing.T) {
				paths := []string{weakeningIdentityPath}
				if test.ambiguous {
					paths = []string{"internal/one/shared/identity_test.go", "internal/two/shared/identity_test.go"}
				}
				root := weakeningIdentityFixture(t, paths...)
				weakeningIdentityEnvironment(t, root, "p31-weakening-refusal")
				before := humantest.Snapshot(t, root)
				if entry == "callable" {
					if _, err := Approve(root, fixtureSession, test.unit, test.reason); err == nil {
						t.Errorf("callable approval accepted invalid request %q, %q", test.unit, test.reason)
					}
				} else if entry == "registered" {
					if _, code := Answer([]string{"approve", "unit", test.unit, "reason", test.reason}); code != 2 {
						t.Errorf("registered approval exit = %d, want 2 for %q, %q", code, test.unit, test.reason)
					}
				} else {
					line := []string{"approve", "unit", test.unit, "reason", test.reason}
					prepare, words := Actions().Preparer(line)
					if prepare == nil {
						t.Fatal("registered approval has no read-only preparation before startup")
					}
					if _, code := prepare(words); code != 2 {
						t.Errorf("approval preparation exit = %d, want 2 for %q, %q", code, test.unit, test.reason)
					}
				}
				if !maps.Equal(before, humantest.Snapshot(t, root)) {
					t.Error("refused approval changed authority, namespace, directory or file bytes")
				}
			})
		}
	}
}

// TestWeakeningApprovalUsesConsumerIdentity keeps the package clause different
// from the directory. The real edit and commit consumers must reject the same
// weakening before issuance, then consume exactly the canonical writer's row.
func TestWeakeningApprovalUsesConsumerIdentity(t *testing.T) {
	for _, entry := range []string{"callable", "registered"} {
		t.Run(entry, func(t *testing.T) {
			root := weakeningIdentityFixture(t, weakeningIdentityPath)
			weakeningIdentityEnvironment(t, root, "p31-weakening-consumer")
			before := weakeningIdentityBefore
			after := strings.Replace(before, "\tif got :=", "\tt.Skip(\"synthetic weakening\")\n\tif got :=", 1)
			writeProposedFile(t, root, weakeningIdentityPath, after)
			request := ProposedRequest{Path: weakeningIdentityPath, Tool: "Edit", Old: &before, New: &after, ApprovalSession: fixtureSession}
			unapproved, err := proposedFixture(root, request)
			if err != nil {
				t.Fatal(err)
			}
			if unapproved.ExitCode() != 2 {
				t.Fatalf("edit consumer accepted an unapproved weakening: %+v", unapproved)
			}
			if len(unapproved.Weakened) != 1 {
				t.Fatalf("edit consumer found %d weakened units, want one: %+v", len(unapproved.Weakened), unapproved)
			}
			if unapproved.Weakened[0].Unit() != weakeningIdentityUnit {
				t.Fatalf("edit consumer identity = %q, want directory key %q", unapproved.Weakened[0].Unit(), weakeningIdentityUnit)
			}
			unapprovedCommit := Check(Request{Root: root, ApprovalSession: fixtureSession, Paths: []string{weakeningIdentityPath}})
			if unapprovedCommit.ExitCode() != 1 {
				t.Fatalf("commit consumer did not refuse the unapproved weakening: %+v", unapprovedCommit)
			}

			var issued rfc.ApprovalReport
			if entry == "callable" {
				issued, err = Approve(root, fixtureSession, weakeningIdentityUnit, weakeningIdentityReason)
				if err != nil {
					t.Fatal(err)
				}
			} else {
				payload, code := Answer([]string{"approve", "unit", weakeningIdentityUnit, "reason", weakeningIdentityReason})
				if code != 0 {
					t.Fatalf("registered canonical approval exit = %d: %+v", code, payload)
				}
				var ok bool
				issued, ok = payload.(rfc.ApprovalReport)
				if !ok {
					t.Fatalf("registered approval returned %T, want ApprovalReport", payload)
				}
			}
			if issued.Unit != weakeningIdentityUnit {
				t.Fatalf("writer issued identity %q, want %q", issued.Unit, weakeningIdentityUnit)
			}
			session := weakeningIdentitySession(t, issued)
			request.ApprovalSession = session
			admitted, err := proposedFixture(root, request)
			if err != nil {
				t.Fatal(err)
			}
			if admitted.ExitCode() != 0 {
				t.Fatalf("canonical writer row did not reach the edit consumer: %+v", admitted)
			}
			if len(admitted.Weakened) != 1 {
				t.Fatalf("approved edit stopped detecting its actual weakening: %+v", admitted)
			}
			commit := Check(Request{Root: root, ApprovalSession: session, Paths: []string{weakeningIdentityPath}})
			if commit.ExitCode() != 0 {
				t.Fatalf("canonical writer row did not reach the commit consumer: %+v", commit)
			}
			if len(commit.Findings) != 1 {
				t.Fatalf("approved commit stopped detecting its actual weakening: %+v", commit)
			}
			if commit.Findings[0].Unit() != weakeningIdentityUnit {
				t.Fatalf("commit consumer identity = %q, want %q", commit.Findings[0].Unit(), weakeningIdentityUnit)
			}
			if len(commit.Approvals.Trailers) != 1 {
				t.Fatalf("commit consumed %d verdicts, want one: %+v", len(commit.Approvals.Trailers), commit)
			}
			if commit.Approvals.Trailers[0] != ApprovalTrailerLine(weakeningIdentityUnit, weakeningIdentityReason) {
				t.Fatalf("commit carried the wrong approval: %+v", commit.Approvals)
			}
		})
	}
}

// TestWeakeningApprovalPreparesWithoutWrites exercises the same early
// preparation boundary that root dispatch uses before startup may create files.
// Valid preparation is read-only; only its admitted answer issues authority.
func TestWeakeningApprovalPreparesWithoutWrites(t *testing.T) {
	root := weakeningIdentityFixture(t, weakeningIdentityPath)
	weakeningIdentityEnvironment(t, root, "p31-weakening-preparation")
	line := []string{"approve", "unit", weakeningIdentityUnit, "reason", weakeningIdentityReason}
	before := humantest.Snapshot(t, root)
	prepare, words := Actions().Preparer(line)
	if prepare == nil {
		t.Fatal("registered approval has no read-only preparation before startup")
	}
	admitted, code := prepare(words)
	if code != 0 {
		t.Fatalf("canonical request preparation exit = %d, want 0", code)
	}
	if !maps.Equal(before, humantest.Snapshot(t, root)) {
		t.Fatal("preparation wrote an approval or namespace before admitted execution")
	}
	payload, code := admitted.Answer()
	if code != 0 {
		t.Fatalf("admitted approval exit = %d: %+v", code, payload)
	}
	issued, ok := payload.(rfc.ApprovalReport)
	if !ok {
		t.Fatalf("admitted approval returned %T, want ApprovalReport", payload)
	}
	if issued.Unit != weakeningIdentityUnit {
		t.Fatalf("admitted writer identity = %q, want %q", issued.Unit, weakeningIdentityUnit)
	}
	content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(issued.Path)))
	if err != nil {
		t.Fatal(err)
	}
	rows, problems := ParseLedger(string(content), issued.Path)
	if len(problems) != 0 {
		t.Fatalf("admitted approval wrote an unreadable ledger: %v", problems)
	}
	if len(rows) != 1 {
		t.Fatalf("admitted approval wrote %d rows, want one", len(rows))
	}
	if rows[0].Name != weakeningIdentityUnit {
		t.Fatalf("admitted approval row = %+v, want %q", rows[0], weakeningIdentityUnit)
	}
}

func weakeningIdentityFixture(t *testing.T, paths ...string) string {
	t.Helper()
	root := t.TempDir()
	for _, path := range paths {
		writeProposedFile(t, root, path, weakeningIdentityBefore)
	}
	commitProposedFixture(t, root)
	return root
}

func weakeningIdentityEnvironment(t *testing.T, root, session string) {
	t.Helper()
	t.Setenv("ZE_REPO_ROOT", root)
	t.Setenv("CLAUDE_CODE_SESSION_ID", session)
	t.Setenv("ZE_AGENT_ID", "")
	env.ResetCache()
	t.Cleanup(env.ResetCache)
}

func weakeningIdentitySession(t *testing.T, issued rfc.ApprovalReport) string {
	t.Helper()
	session := strings.TrimSuffix(strings.TrimPrefix(issued.Path, "tmp/commit-weakening-approved-"), ".md")
	if !lepath.ValidCommitSession(session) {
		t.Fatalf("writer returned a noncanonical weakening approval path %q", issued.Path)
	}
	if issued.Path != ApprovalPath(session) {
		t.Fatalf("writer path = %q, want %q", issued.Path, ApprovalPath(session))
	}
	return session
}
