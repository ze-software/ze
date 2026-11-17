// Design: docs/architecture/plugin/rib-storage-design.md -- original owner in validation identities.
package ribevents

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/family"
)

// TestValidationRouteRoundTripRetainsPeerOwner checks the real public codec,
// including the largest stable identity and a binary opaque native key.
func TestValidationRouteRoundTripRetainsPeerOwner(t *testing.T) {
	family.RegisterTestFamilies()
	for _, owner := range []string{"1", "18446744073709551615"} {
		t.Run(owner, func(t *testing.T) {
			input := []byte(`{"peer-address":"192.0.2.188","peer-owner":` + owner +
				`,"family":"ipv4/unicast","prefix":"10.77.0.0/24","path-id":0,"nlri":"00ff80"}`)
			var route ValidationRoute
			require.NoError(t, json.Unmarshal(input, &route))
			encoded, err := json.Marshal(route)
			require.NoError(t, err)
			var fields map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(encoded, &fields))
			require.Equal(t, owner, string(fields["peer-owner"]), "the original uint64 is encoded without rounding")
			require.Equal(t, string([]byte{0, 255, 128}), route.NLRI)
			require.GreaterOrEqual(t, route.JSONSizeBound(), len(encoded), "the frame fit proof includes the full owner")
		})
	}
}

// TestValidationRouteSeparatesOwnersAtOneAddress checks the comparable key
// used by validation debt and FlowSpec snapshots. Address, prefix and path ID
// equality cannot erase the original Peer identity.
func TestValidationRouteSeparatesOwnersAtOneAddress(t *testing.T) {
	family.RegisterTestFamilies()
	var first, second ValidationRoute
	common := `,"family":"ipv4/unicast","prefix":"10.77.0.0/24","path-id":0}`
	require.NoError(t, json.Unmarshal([]byte(`{"peer-address":"192.0.2.188","peer-owner":1701`+common), &first))
	require.NoError(t, json.Unmarshal([]byte(`{"peer-address":"192.0.2.188","peer-owner":1702`+common), &second))
	require.NotEqual(t, first, second)
	require.Len(t, map[ValidationRoute]bool{first: true, second: true}, 2)
}
