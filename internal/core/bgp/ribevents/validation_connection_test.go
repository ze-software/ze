// Design: docs/architecture/plugin/rib-storage-design.md -- original validation connection.
package ribevents

import (
	"encoding/json"
	"github.com/stretchr/testify/require"
	"github.com/ze-software/ze/internal/core/family"
	"math"
	"net/netip"
	"testing"
)

// The actual process codec must retain full-width physical identity and native
// bytes. A malformed incoming identity leaves the previous value unchanged.
func TestValidationConnectionCodecKeepsOriginalWidth(t *testing.T) {
	family.RegisterTestFamilies()
	original := ValidationRoute{Peer: netip.MustParseAddr("192.0.2.188"), PeerOwner: math.MaxUint64,
		ConnectionID: math.MaxUint64 - 1, Family: family.IPv4Unicast,
		Prefix: netip.MustParsePrefix("10.77.0.0/24"), NLRI: string([]byte{0, 255, 128})}
	encoded, err := json.Marshal(original)
	require.NoError(t, err)
	require.Contains(t, string(encoded), `"connection-id":"18446744073709551614"`)
	require.GreaterOrEqual(t, original.JSONSizeBound(), len(encoded))
	var decoded ValidationRoute
	require.NoError(t, json.Unmarshal(encoded, &decoded))
	require.Equal(t, original, decoded)
	sibling := original
	sibling.ConnectionID--
	require.NotEqual(t, original, sibling)
	require.Len(t, map[ValidationRoute]bool{original: true, sibling: true}, 2)
	previous := decoded
	require.Error(t, json.Unmarshal([]byte(`{"connection-id":"18446744073709551616"}`), &decoded))
	require.Equal(t, previous, decoded)
}
