// Design: docs/contributing/spec-workflow.md -- unknown producer lookup is not absence
// VALIDATES: permission denial of one selected producer cannot erase its obligation.
// PREVENTS: an unrelated readable producer replacing an unknown subject population.
package hookruntime

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDesignEvidencePermissionUnknownKeepsReadableControl(t *testing.T) {
	for _, heading := range []string{"Current Behavior", "Files to Modify", "Files to Create"} {
		t.Run(heading, func(t *testing.T) {
			root := t.TempDir()
			const unknown = "internal/locked/producer.go"
			const readable = "internal/control/producer.go"
			writeHookFixture(t, root, unknown, "package locked\n")
			writeHookFixture(t, root, readable, "package control\n")
			recordObservedRead(t, root, filepath.Join(root, readable), "package control\n", 1, 2, 2)
			dir := filepath.Join(root, "internal", "locked")
			if err := os.Chmod(dir, 0); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := os.Chmod(dir, 0o750); err != nil {
					t.Error(err)
				}
			})
			if _, err := os.Stat(filepath.Join(root, unknown)); !os.IsPermission(err) {
				t.Skip("the current user can traverse a mode-000 directory; permission-denied fixture is unavailable")
			}
			// The read control must remain usable under the actual permission failure.
			observedDesignGate(t, root, "## Files to Modify\n`"+readable+"`\n", 0)
			observedDesignGate(t, root, "## "+heading+"\n`"+unknown+"`\n", 2)
		})
	}
}
