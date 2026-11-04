// Design: docs/contributing/committing.md -- a selected approval target must precede the actual publication parent.
package commit

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestPreparedApprovalPublicationRefusesARewrittenTarget prepares an immutable
// target selection, rewrites only this fixture's HEAD behind that target, and
// drives the actual generated script. A stale target cannot be translated to
// the replacement history, and refusal must retain all rows and its message.
func TestPreparedApprovalPublicationRefusesARewrittenTarget(t *testing.T) {
	root, first, _ := approvalPublicationFixture(t)
	prepared := createApprovalPublication(t, first, approvalUnit)
	parent := revParse(t, root, first+"^")
	previous := revParse(t, root, "HEAD")
	runCommitGit(t, root, "update-ref", "HEAD", parent, previous)
	pending := readApprovalFile(t, root)
	command := exec.CommandContext(t.Context(), "bash", filepath.Join(root, prepared.Script)) // #nosec G204 -- Execute this fixture's actual generated native script.
	command.Dir = root
	output, err := command.CombinedOutput()
	if err == nil {
		t.Fatalf("prepared script published a target outside its actual parent history: %s", output)
	}
	if !strings.Contains(string(output), "RFC approval target does not precede publication parent") {
		t.Fatalf("script did not reach the exact target ancestry boundary: %v: %s", err, output)
	}
	if got := revParse(t, root, "HEAD"); got != parent {
		t.Fatalf("refused stale target moved HEAD: %s -> %s", parent, got)
	}
	if got := readApprovalFile(t, root); got != pending {
		t.Fatal("refused stale target changed pending approval rows")
	}
	if _, err := os.Stat(filepath.Join(root, prepared.Message)); err != nil {
		t.Fatalf("refused stale target lost its own retry message: %v", err)
	}
}
