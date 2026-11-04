// Design: docs/contributing/committing.md -- publication consumes an exact approval pair once, not merely once per preparation.
package commit

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestPreparedApprovalPublicationCannotSpendAnAlreadyPublishedTarget prepares
// two independent native scripts while the same judge row is live. Once the
// first publishes, the second must refuse at publication and retain its retry
// message and every remaining row rather than publishing a duplicate binding.
func TestPreparedApprovalPublicationCannotSpendAnAlreadyPublishedTarget(t *testing.T) {
	root, first, _ := approvalPublicationFixture(t)
	one := createApprovalPublication(t, first, approvalUnit)
	two := createApprovalPublication(t, first, approvalUnit)
	if one.Script == two.Script || one.Message == two.Message {
		t.Fatal("fixture did not reserve independent script and message identities")
	}
	runCommitScript(t, root, one.Script)
	head := revParse(t, root, "HEAD")
	pending := readApprovalFile(t, root)
	command := exec.CommandContext(t.Context(), "bash", filepath.Join(root, two.Script)) // #nosec G204 -- Execute this fixture's actual generated native script.
	command.Dir = root
	output, err := command.CombinedOutput()
	if err == nil {
		t.Fatalf("second prepared script spent the already-published target: %s", output)
	}
	if !strings.Contains(string(output), "RFC approval binding already published") {
		t.Fatalf("second script did not reach the exact binding replay boundary: %v: %s", err, output)
	}
	if got := revParse(t, root, "HEAD"); got != head {
		t.Fatalf("refused replay moved HEAD: %s -> %s", head, got)
	}
	if got := readApprovalFile(t, root); got != pending {
		t.Fatal("refused replay changed remaining approval rows")
	}
	if _, err := os.Stat(filepath.Join(root, two.Message)); err != nil {
		t.Fatalf("refused replay lost its own retry message: %v", err)
	}
}
