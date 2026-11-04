// Design: docs/contributing/committing.md -- a published exact target/unit cannot be renewed by message framing.
package commit

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/rfc"
)

// TestNativeRFCBindingWithRecordSeparatorCannotBePublishedAgain keeps the
// judge's opaque reason in the actual first publication, then asks the native
// producer to spend that exact target/unit again. If preparation admits it,
// the actual generated script distinguishes a preparation defect from a
// duplicate publication; a later push refusal is not publication protection.
func TestNativeRFCBindingWithRecordSeparatorCannotBePublishedAgain(t *testing.T) {
	for _, tc := range []struct {
		name   string
		reason string
	}{
		{"record_separator", "judge: reviewed this exact change\x1eopaque reason suffix"},
		{"record_and_unit_separator", "judge: checked\x1eforged carrier\x1fopaque suffix"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assertOpaqueApprovalBindingCannotBeRepublished(t, tc.reason)
		})
	}
}

func assertOpaqueApprovalBindingCannotBeRepublished(t *testing.T, reason string) {
	t.Helper()
	root, target, _ := approvalPublicationFixture(t)
	if payload, code := rfc.Answer([]string{"approve", "commit", target, "unit", approvalUnit, "reason", reason}); code != 0 {
		t.Fatalf("record first opaque verdict = (%#v, %d)", payload, code)
	}
	first := createApprovalPublication(t, target, approvalUnit)
	runCommitScript(t, root, first.Script)
	head := revParse(t, root, "HEAD")
	message := "\n" + runCommitGitOutput(t, root, "show", "-s", "--format=%B", head) + "\n"
	for _, trailer := range []string{rfc.ApprovalTrailerLine(approvalUnit, reason), rfc.ApprovalForTrailerLine(target, approvalUnit)} {
		if !strings.Contains(message, "\n"+trailer+"\n") {
			t.Fatalf("actual first carrier lost its exact judge reason or binding %q: %q", trailer, message)
		}
	}
	if payload, code := rfc.Answer([]string{"approve", "commit", target, "unit", approvalUnit,
		"reason", "judge: a fresh normal wording for the same exact change"}); code != 0 {
		t.Fatalf("record replacement verdict = (%#v, %d)", payload, code)
	}
	pending := readApprovalFile(t, root)
	payload, code := Answer([]string{"create", "subject", "do not republish an opaque bound verdict", "rfc-approval", target, approvalUnit})
	if code == 0 {
		second, ok := payload.(*Prepared)
		if !ok {
			t.Fatalf("admitted replay result = %T, want *Prepared", payload)
		}
		command := exec.CommandContext(t.Context(), "bash", filepath.Join(root, second.Script)) // #nosec G204 -- Execute this fixture's actual generated native script.
		command.Dir = root
		output, err := command.CombinedOutput()
		if err == nil {
			t.Errorf("actual second script published the already-published target: %s", output)
		} else {
			if _, statErr := os.Stat(filepath.Join(root, second.Message)); statErr != nil {
				t.Errorf("refused replay lost its own retry message: %v", statErr)
			}
		}
		t.Errorf("native create admitted an already-published opaque binding; actual second script result: %v: %s", err, output)
	}
	if got := revParse(t, root, "HEAD"); got != head {
		t.Errorf("replay moved HEAD: %s -> %s", head, got)
	}
	if got := readApprovalFile(t, root); got != pending {
		t.Error("replay changed the pending approval rows")
	}
}
