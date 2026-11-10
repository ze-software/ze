// Design: docs/contributing/committing.md -- explicit historical selection must change the exact named function.
package commit

import (
	"maps"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/le/le/human/humantest"
	lepath "github.com/ze-software/ze/internal/le/le/path"
	"github.com/ze-software/ze/internal/le/rfc"
	testweakened "github.com/ze-software/ze/internal/le/test/weakened"
)

// TestExplicitApprovalExactOneLineBoundary independently drives native Create
// with a real pending target row, so repairing issuance cannot hide a consumer
// defect. Its separate accepting control obtains its row from the native writer.
// Source-only regression hypothesis until the main verifier observes RED.
func TestExplicitApprovalExactOneLineBoundary(t *testing.T) {
	const path = "pkg/identity_test.go"
	const unit = "pkg.TestIdentity"
	const prefix = "package pkg\nimport \"testing\"\n// RFC requirement: RFC2119-1-1 positive\n"
	const function = "func TestIdentity(t *testing.T) { check(1) }\n"
	const before = prefix + function + "var unrelated = 1\n"
	const reason = "judge: reviewed this exact historical function change"
	for _, change := range []struct {
		name, after string
		accept      bool
	}{
		{"following_declaration_only", prefix + function + "var unrelated = 2\n", false},
		{"actual_function_change", prefix + "func TestIdentity(t *testing.T) { check(2) }\nvar unrelated = 1\n", true},
	} {
		t.Run(change.name, func(t *testing.T) {
			root := newCommitRepository(t)
			configureCommitAuthor(t, root)
			parent := commitOwed(t, root, "original exact-boundary carrier", map[string]string{path: before}, nil)
			target := commitOwed(t, root, "historical exact-boundary change", map[string]string{path: change.after}, nil)
			if got := runCommitGitOutput(t, root, "show", parent+":"+path); got != before {
				t.Fatalf("parent blob = %q, want %q", got, before)
			}
			if got := runCommitGitOutput(t, root, "show", target+":"+path); got != change.after {
				t.Fatalf("target blob = %q, want %q", got, change.after)
			}
			t.Setenv("ZE_REPO_ROOT", root)
			t.Setenv("CLAUDE_CODE_SESSION_ID", "approval-exact-boundary-selection")
			t.Setenv("ZE_AGENT_ID", "")
			env.ResetCache()
			t.Cleanup(env.ResetCache)
			if _, err := lepath.CommitSession(root, approvalSession); err != nil {
				t.Fatal(err)
			}
			if change.accept {
				payload, code := rfc.Answer([]string{"approve", "commit", target, "unit", unit, "reason", reason})
				if code != 0 {
					t.Fatalf("real changed-function issuance = (%#v, %d), want success", payload, code)
				}
			} else {
				// Seed a persisted row solely to exercise consumption independently
				// of issuance. Both immutable historical blobs stay untouched; no
				// resolver or gate is replaced and this is not an issuance verdict.
				writeCommitFixture(t, root, rfc.ApprovalPath(approvalSession),
					"# Pending fixture approval\n\n| Test | Reason | Target | Content |\n| --- | --- | --- | --- |\n| "+unit+" | "+reason+" | "+target+" | none..none |\n")
			}
			const foreignReason = "judge: a foreign origin's pending verdict"
			writeCommitFixture(t, root, rfc.ApprovalPath("feed5678"),
				"# Foreign fixture approval\n\n| Test | Reason | Target | Content |\n| --- | --- | --- | --- |\n| "+unit+" | "+foreignReason+" | "+target+" | none..none |\n")
			rows, problems := testweakened.LoadApprovals(root, testweakened.RFCApprovals(approvalSession))
			if len(problems) != 0 {
				t.Fatalf("pending fixture row is unreadable: %v", problems)
			}
			if len(rows) != 1 {
				t.Fatalf("pending fixture rows = %v, want exactly one", rows)
			}
			if rows[0].Target != target {
				t.Fatalf("pending target = %q, want %q", rows[0].Target, target)
			}
			if rows[0].Name != unit {
				t.Fatalf("pending unit = %q, want %q", rows[0].Name, unit)
			}
			if rows[0].Reason != reason {
				t.Fatalf("pending reason = %q, want %q", rows[0].Reason, reason)
			}
			if rows[0].Carrier != "" {
				t.Fatalf("fixture row is already spent by %s", rows[0].Carrier)
			}
			pending := readApprovalFile(t, root)
			beforeCreate := humantest.Snapshot(t, root)
			payload, code := Answer([]string{"create", "subject", "publish exact boundary judgement", "rfc-approval", target, unit})
			if !change.accept {
				if code != 2 {
					t.Errorf("unchanged exact-unit selection = (%#v, %d), want refusal exit 2", payload, code)
				}
				if !maps.Equal(beforeCreate, humantest.Snapshot(t, root)) {
					t.Error("unchanged historical unit selection wrote namespace, ledger, message, script, or Git bytes")
				}
				return
			}
			if code != 0 {
				t.Fatalf("actual changed-function selection = (%#v, %d), want success", payload, code)
			}
			prepared, ok := payload.(*Prepared)
			if !ok {
				t.Fatalf("selection result = %T, want *Prepared", payload)
			}
			if prepared.Session != approvalSession {
				t.Fatalf("prepared namespace = %q, want active origin %q", prepared.Session, approvalSession)
			}
			for _, trailer := range []string{rfc.ApprovalTrailerLine(unit, reason), rfc.ApprovalForTrailerLine(target, unit)} {
				if !slices.Contains(prepared.RFCApprovals.Trailers, trailer) {
					t.Fatalf("prepared selection lacks %q: %v", trailer, prepared.RFCApprovals.Trailers)
				}
			}
			message, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(prepared.Message)))
			if err != nil {
				t.Fatal(err)
			}
			if !carriesTrailerLine(string(message), rfc.ApprovalForTrailerLine(target, unit)) {
				t.Fatalf("real prepared carrier lost exact historical binding: %s", message)
			}
			if !carriesTrailerLine(string(message), rfc.ApprovalTrailerLine(unit, reason)) {
				t.Fatalf("real prepared carrier lost original verdict: %s", message)
			}
			if carriesTrailerLine(string(message), rfc.ApprovalTrailerLine(unit, foreignReason)) {
				t.Fatalf("foreign namespace supplied a verdict to the active origin: %s", message)
			}
			if got := readApprovalFile(t, root); got != pending {
				t.Fatal("preparation changed the active pending row")
			}
			if got := revParse(t, root, "HEAD"); got != target {
				t.Fatalf("preparation moved HEAD: %s -> %s", target, got)
			}
		})
	}
}
