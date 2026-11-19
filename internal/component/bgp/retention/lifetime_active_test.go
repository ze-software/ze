// Design: docs/guide/graceful-restart.md -- active original-source admission.
package retention

import (
	"github.com/stretchr/testify/require"
	"testing"
)

// Active is current ownership, not historical permission. Invalid identities,
// delayed old DOWN and duplicate terminal events never create a lifetime.
// MUTATION: retain an ended lifetime or treat connection zero as a wildcard.
func TestRetentionActiveHasNoCompletedOwnerHistory(t *testing.T) {
	var ledger Ledger
	require.False(t, ledger.Active(1701, 41))
	require.Empty(t, ledger.verdicts)
	require.NoError(t, ledger.Begin(1701, 41))
	require.True(t, ledger.Active(1701, 41))
	require.NoError(t, ledger.Begin(1702, 42))
	require.NoError(t, ledger.Begin(1701, 43))
	_, err := ledger.Take(1701, 41)
	require.NoError(t, err)
	require.True(t, ledger.Active(1701, 43))
	require.False(t, ledger.Active(1701, 41))
	require.Len(t, ledger.verdicts, 2)
	require.NoError(t, ledger.End(1701))
	require.NoError(t, ledger.End(1701))
	require.False(t, ledger.Active(1701, 43))
	require.True(t, ledger.Active(1702, 42))
	require.Len(t, ledger.verdicts, 1)
	_, err = ledger.Take(1702, 42)
	require.NoError(t, err)
	require.Empty(t, ledger.verdicts)
	require.False(t, ledger.Active(0, 0))
	require.False(t, ledger.Active(1701, 0))
	require.False(t, ledger.Active(0, 41))
	require.Empty(t, ledger.verdicts)
}
