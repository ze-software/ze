// Design: docs/architecture/wire/nlri.md -- encode -n full label stack (F13) test
package labeled

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// VALIDATES: F13 -- `ze bgp encode -n` keeps the FULL label stack in the NLRI,
// not just the first label. A 2-label /24 NLRI has bit-length 72 (0x48: two
// 24-bit labels + 24-bit prefix); the single-label bug produced 0x30 (48 bits).
func TestEncodeRoutePreservesLabelStack(t *testing.T) {
	_, nlriBytes, err := EncodeRoute(
		"10.0.0.0/24 next-hop 1.2.3.4 label [100 200]",
		"ipv4/mpls-label", 65000, false, true, false)
	require.NoError(t, err)
	require.NotEmpty(t, nlriBytes)
	assert.Equal(t, byte(0x48), nlriBytes[0],
		"NLRI must encode both labels (0x48 bit-length), not just the first (0x30)")
}
