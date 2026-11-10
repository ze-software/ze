// Design: docs/contributing/committing.md -- explicit late-approval publication in a private index.
package commit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
	lepath "github.com/ze-software/ze/internal/le/le/path"
	"github.com/ze-software/ze/internal/le/rfc"
)

// TestExplicitApprovalPublicationRetainsUnselectedRows publishes one of two
// same-unit, same-reason approvals through the registered create action. The
// approval-only carrier changes no tree entry and cannot be published twice.
func TestExplicitApprovalPublicationRetainsUnselectedRows(t *testing.T) {
	root, first, second := approvalPublicationFixture(t)
	pending := readApprovalFile(t, root)
	prepared := createApprovalPublication(t, "HEAD^")
	if got := readApprovalFile(t, root); got != pending {
		t.Fatal("preparation pruned pending approvals")
	}
	beforeTree := revParse(t, root, "HEAD^{tree}")
	runCommitScript(t, root, prepared.Script)
	if tree := revParse(t, root, "HEAD^{tree}"); tree != beforeTree {
		t.Fatalf("approval-only publication changed tree: %s -> %s", beforeTree, tree)
	}
	message := runCommitGitOutput(t, root, "log", "-1", "--format=%B")
	if !carriesTrailerLine(message, rfc.ApprovalTrailerLine(approvalUnit, "judge: reviewed this exact change")) {
		t.Fatalf("approval carrier lost original judge reason: %s", message)
	}
	if !carriesTrailerLine(message, "RFC-approved-for: "+first+" "+approvalUnit) {
		t.Fatalf("revision was not bound to the once-resolved first target: %s", message)
	}
	if strings.Contains(message, "RFC-approved-for: "+second) {
		t.Fatalf("publication carried an unselected target: %s", message)
	}
	remaining := readApprovalFile(t, root)
	if strings.Contains(remaining, first) {
		t.Fatalf("successful publication left selected target pending: %s", remaining)
	}
	if !strings.Contains(remaining, second) {
		t.Fatalf("successful publication pruned a different target with the same unit/reason: %s", remaining)
	}
	if _, code := Answer([]string{"create", "subject", "do not replay", "rfc-approval", first, approvalUnit}); code != 1 {
		t.Fatalf("replayed exact approval selection exit = %d, want 1", code)
	}
}

// TestFailedApprovalPublicationRetainsEveryPendingRow refuses the actual
// compare-and-swap, then reruns the same generated script successfully. Failed
// preparation and failed publication must not spend either targeted approval.
func TestFailedApprovalPublicationRetainsEveryPendingRow(t *testing.T) {
	root, first, second := approvalPublicationFixture(t)
	pending := readApprovalFile(t, root)
	if _, code := Answer([]string{"create", "subject", "missing selected unit", "rfc-approval", first, "pkg.TestMissing"}); code != 2 {
		t.Fatalf("missing approval selection exit = %d, want 2", code)
	}
	if got := readApprovalFile(t, root); got != pending {
		t.Fatal("failed preparation changed pending approvals")
	}
	prepared := createApprovalPublication(t, first)
	before := revParse(t, root, "HEAD")
	const refuseSwap = "#!/bin/bash\nif [ \"${1:-}\" = update-ref ]; then echo 'fixture refused publication' >&2; exit 1; fi\nexec \"$ZE_TEST_REAL_GIT\" \"$@\"\n"
	output, err := runScriptWithGit(t, root, prepared.Script, refuseSwap)
	if err == nil {
		t.Fatalf("publication ignored a failed compare-and-swap: %s", output)
	}
	if !strings.Contains(output, "fixture refused publication") {
		t.Fatalf("failure did not reach actual publication boundary: %s", output)
	}
	if got := revParse(t, root, "HEAD"); got != before {
		t.Fatalf("failed publication moved HEAD: %s -> %s", before, got)
	}
	if got := readApprovalFile(t, root); got != pending {
		t.Fatal("failed publication spent a pending approval")
	}
	if _, err := os.Stat(filepath.Join(root, prepared.Message)); err != nil {
		t.Fatalf("failed publication lost its retry message: %v", err)
	}
	runCommitScript(t, root, prepared.Script)
	if remaining := readApprovalFile(t, root); !strings.Contains(remaining, second) {
		t.Fatalf("successful retry pruned the unselected second target: %s", remaining)
	}
}

func approvalPublicationFixture(t *testing.T) (root, first, second string) {
	t.Helper()
	root = newCommitRepository(t)
	configureCommitAuthor(t, root)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "explicit-approval-publication")
	t.Setenv("ZE_REPO_ROOT", root)
	env.ResetCache()
	t.Cleanup(env.ResetCache)
	if _, err := lepath.CommitSession(root, approvalSession); err != nil {
		t.Fatal(err)
	}
	commitOwed(t, root, "original unit", map[string]string{approvalTagged: taggedUnitText("check(1)")}, nil)
	first = commitOwed(t, root, "first reviewed change", map[string]string{approvalTagged: taggedUnitText("check(2)")}, nil,
		trailerRFCApprovalOwed+approvalUnit+": awaiting judgement")
	second = commitOwed(t, root, "second reviewed change", map[string]string{approvalTagged: taggedUnitText("check(3)")}, nil,
		trailerRFCApprovalOwed+approvalUnit+": awaiting judgement")
	for _, target := range []string{first, second} {
		if payload, code := rfc.Answer([]string{"approve", "commit", target, "unit", approvalUnit, "reason", "judge: reviewed this exact change"}); code != 0 {
			t.Fatalf("target approval %s = (%#v, %d)", target, payload, code)
		}
	}
	return root, first, second
}

func createApprovalPublication(t *testing.T, revision string) *Prepared {
	t.Helper()
	payload, code := Answer([]string{"create", "subject", "publish exact judge approval", "rfc-approval", revision, approvalUnit})
	if code != 0 {
		t.Fatalf("approval publication = (%#v, %d)", payload, code)
	}
	prepared, ok := payload.(*Prepared)
	if !ok {
		t.Fatalf("approval publication result = %T, want *Prepared", payload)
	}
	return prepared
}
