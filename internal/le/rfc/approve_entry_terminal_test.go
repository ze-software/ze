// Design: docs/contributing/rfc-implementation-guide.md -- refusal before session namespace writes.
package rfc

import (
	"maps"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/le/le/human/humantest"
)

// TestOwnerRulingCommandRefusesBeforeNamespaceWrites reaches the registered RFC
// entry, not just Approve, so creating the commit namespace before admission fails.
func TestOwnerRulingCommandRefusesBeforeNamespaceWrites(t *testing.T) {
	for _, mode := range []string{"piped-stdin", "piped-stdout", "closed-stdin", "closed-stdout"} {
		t.Run(mode, func(t *testing.T) {
			humantest.Streams(t, mode)
			root := t.TempDir()
			t.Setenv("ZE_REPO_ROOT", root)
			t.Setenv("CLAUDE_CODE_SESSION_ID", "rfc-terminal-entry-fixture")
			env.ResetCache()
			t.Cleanup(env.ResetCache)
			before := humantest.Snapshot(t, root)
			_, code := Answer([]string{"approve", "unit", "pkg.TestX", "reason", "owner ruling 2026-11-03 accepts the move"})
			if code != 2 {
				t.Errorf("owner-ruling command exit = %d, want 2", code)
			}
			if !maps.Equal(before, humantest.Snapshot(t, root)) {
				t.Error("refused RFC command created a namespace, directory or approval")
			}
		})
	}
}
