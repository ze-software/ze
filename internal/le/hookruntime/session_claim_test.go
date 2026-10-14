// Design: docs/contributing/running-commands.md -- session identity, and what each agent owns
package hookruntime

import (
	"os"
	"path/filepath"
	"testing"
)

// VALIDATES: a hook reads an agent's own spec claim when it made one, falls
// back to the parent session's claim when it did not, and a main thread reads
// the session's claim.
// PREVENTS: a subagent's hooks judging its edits against a sibling's claim,
// which is what one shared marker per session did.
func TestSpecClaimIsPerAgentWithParentFallback(t *testing.T) {
	root := t.TempDir()
	write := func(name, spec string) {
		t.Helper()
		dir := filepath.Join(root, "tmp", "session")
		if err := os.MkdirAll(dir, 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), []byte(spec+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write(".session-parent-17", "spec-parent.md")
	write(".session-parent-17+one", "spec-one.md")
	claim := func(agent string) string {
		return specClaim(context{root: root, payload: Payload{AgentID: agent}}, "parent-17")
	}
	if got := claim("one"); got != "spec-one.md" {
		t.Errorf("agent one's claim = %q, want spec-one.md", got)
	}
	if got := claim("two"); got != "spec-parent.md" {
		t.Errorf("agent two's claim = %q, want the parent's spec-parent.md", got)
	}
	if got := claim(""); got != "spec-parent.md" {
		t.Errorf("main thread's claim = %q, want spec-parent.md", got)
	}
}
