package update

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"
)

// PREVENTS: a malformed source-DOWN request silently becoming a blind ordinary
// withdrawal. Method: parse a valid source, each malformed form, and no meta.
func TestRecoveryMetadataRejectsMalformedOwnership(t *testing.T) {
	source, err := recoveryFromMeta(map[string]any{"recovery-source": "192.0.2.10"})
	require.NoError(t, err)
	require.Equal(t, netip.MustParseAddr("192.0.2.10"), source)
	for _, meta := range []map[string]any{
		{"recovery-source": true},
		{"recovery-source": "bad"},
		{"recovery-source": ""},
	} {
		_, err := recoveryFromMeta(meta)
		require.Error(t, err)
	}
	source, err = recoveryFromMeta(nil)
	require.NoError(t, err)
	require.False(t, source.IsValid(), "ordinary UPDATE commands do not enter lifecycle recovery")
}
