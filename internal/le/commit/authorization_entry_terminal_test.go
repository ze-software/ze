// Design: docs/contributing/committing.md -- entry refusals precede namespace writes.
package commit

import (
	"maps"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/le/le/human/humantest"
)

// TestAttestationCommandRefusesBeforeNamespaceWrites reaches the registered
// create entry with valid isolated debt, before any session or message is written.
func TestAttestationCommandRefusesBeforeNamespaceWrites(t *testing.T) {
	for _, mode := range []string{"piped-stdin", "piped-stdout", "closed-stdin", "closed-stdout"} {
		t.Run(mode, func(t *testing.T) {
			humantest.Streams(t, mode)
			root := newOwedRepository(t)
			t.Setenv("ZE_REPO_ROOT", root)
			t.Setenv("CLAUDE_CODE_SESSION_ID", "attest-terminal-entry-fixture")
			env.ResetCache()
			t.Cleanup(env.ResetCache)
			owed := commitOwed(t, root, "the change", map[string]string{"a.txt": "a\n"}, nil, "Review-owed: demo: no reviewer")
			before := humantest.Snapshot(t, root)
			_, code := Answer([]string{"create", "subject", "owner attests", "attest", owed, "Thomas reviewed the change"})
			if code != 2 {
				t.Errorf("attestation command exit = %d, want 2", code)
			}
			if !maps.Equal(before, humantest.Snapshot(t, root)) {
				t.Error("refused attestation command created a namespace, directory or artifact")
			}
		})
	}
}
