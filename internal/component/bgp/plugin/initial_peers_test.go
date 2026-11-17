// Design: docs/architecture/api/process-protocol.md -- initial runtime grants.
package plugin

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestInitialPeerConfigRetriesRefusal proves an initial refusal does not cache
// an obsolete candidate. Method: fail, retry successfully, then simulate a later
// auto-load and ensure the old running map is not rebuilt with new declarations.
func TestInitialPeerConfigRetriesRefusal(t *testing.T) {
	var initial initialPeerConfig
	refused := errors.New("candidate grants are incomplete")
	calls := 0
	require.ErrorIs(t, initial.resolve(func() error { calls++; return refused }), refused)
	require.NoError(t, initial.resolve(func() error { calls++; return nil }))
	require.Equal(t, 2, calls)
	require.NoError(t, initial.resolve(func() error {
		t.Fatal("a reload must reconcile its candidate, not rebuild the old map during auto-load")
		return refused
	}))
	var replacement initialPeerConfig
	require.NoError(t, replacement.resolve(func() error { calls++; return nil }))
	require.Equal(t, 3, calls, "a replacement reactor owes its own initial gate")
}
