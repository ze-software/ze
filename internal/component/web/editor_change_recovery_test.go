// Design: docs/architecture/web-interface.md -- discoverable corrupt-change recovery.
package web

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/cli"
)

// TestCorruptCandidateRefusalOffersRecovery gives the operator a reachable next
// step without reflecting unreadable secret data from the parser's error.
func TestCorruptCandidateRefusalOffersRecovery(t *testing.T) {
	mgr := newTestEditorManager(t)
	require.NoError(t, mgr.store.WriteFile(cli.ChangePath(mgr.configPath, "alice"),
		[]byte("set unknown-path secret-corruption-sentinel\n"), 0o600))
	_, err := mgr.GetOrCreate("alice")
	require.Error(t, err)
	require.True(t, cli.IsCorruptChangeFile(err))
	refusal := editorSessionRefusal(err)
	require.Contains(t, refusal.Error(), "/config/recover")
	require.NotContains(t, refusal.Error(), "secret-corruption-sentinel")
}
