package update

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"
)

// PREVENTS: a malformed source-DOWN request silently becoming a blind ordinary
// withdrawal, and a departure with no session generation reaching the writer
// with a defaulted bound. Method: parse a valid source and generation, each
// malformed form, and no meta.
func TestRecoveryMetadataRejectsMalformedOwnership(t *testing.T) {
	source, generation, err := recoveryFromMeta(map[string]any{
		"recovery-source": "192.0.2.10", "recovery-generation": "7"})
	require.NoError(t, err)
	require.Equal(t, netip.MustParseAddr("192.0.2.10"), source)
	require.Equal(t, uint64(7), generation)
	for _, meta := range []map[string]any{
		{"recovery-source": true, "recovery-generation": "7"},
		{"recovery-source": "bad", "recovery-generation": "7"},
		{"recovery-source": "", "recovery-generation": "7"},
		{"recovery-source": "192.0.2.10"},
		{"recovery-source": "192.0.2.10", "recovery-generation": "0"},
		{"recovery-source": "192.0.2.10", "recovery-generation": 7},
		{"recovery-source": "192.0.2.10", "recovery-generation": "seven"},
	} {
		_, _, err := recoveryFromMeta(meta)
		require.Error(t, err, "meta %v", meta)
	}
	source, _, err = recoveryFromMeta(nil)
	require.NoError(t, err)
	require.False(t, source.IsValid(), "ordinary UPDATE commands do not enter lifecycle recovery")
}
