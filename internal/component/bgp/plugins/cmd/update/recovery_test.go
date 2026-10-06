package update

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"
)

// PREVENTS: a malformed source-DOWN request silently becoming a blind ordinary
// withdrawal, or a JSON numeric cut losing the received generation's precision.
func TestRecoveryMetadataPreservesCutAndRejectsMalformedOwnership(t *testing.T) {
	source, cut, err := recoveryFromMeta(map[string]any{"recovery-source": "192.0.2.10", "recovery-cut": "18446744073709551615"})
	require.NoError(t, err)
	require.Equal(t, netip.MustParseAddr("192.0.2.10"), source)
	require.Equal(t, ^uint64(0), cut)
	for _, meta := range []map[string]any{
		{"recovery-source": true, "recovery-cut": "1"},
		{"recovery-source": "bad", "recovery-cut": "1"},
		{"recovery-source": "192.0.2.10"},
		{"recovery-source": "192.0.2.10", "recovery-cut": float64(1)},
		{"recovery-source": "192.0.2.10", "recovery-cut": "-1"},
	} {
		_, _, err := recoveryFromMeta(meta)
		require.Error(t, err)
	}
	source, cut, err = recoveryFromMeta(nil)
	require.NoError(t, err)
	require.False(t, source.IsValid(), "ordinary UPDATE commands do not enter lifecycle recovery")
	require.Zero(t, cut)
}
