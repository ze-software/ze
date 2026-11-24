// Design: docs/architecture/api/architecture.md -- defaults request no transit.
package bgpconfig

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
)

// TestDefaultAIGPDoesNotRequestTransit proves an API-only internal peer loads
// with RFC 7311's enabled default and allows narrowing away from the selecting
// RIB without activating any AIGP bus requirement.
//
// VALIDATES: an iBGP peer with no written `aigp` keeps AIGPEnabled() true and
// does not refuse narrowing its bus away from bgp-rib.
// PREVENTS: default iBGP AIGP forcing transit or blocking API-only bus isolation.
func TestDefaultAIGPDoesNotRequestTransit(t *testing.T) {
	tree, err := config.ParseTreeForValidation(`
bus {
 endpoint bgp-rib {
  internal bgp-rib;
  send { allow none; }
 }
}
bgp {
 router-id 192.0.2.1;
 session { asn { local 65000; } }
 peer api-only {
  connection { remote { ip 192.0.2.2; } local { ip 192.0.2.1; } }
  session { asn { local 65000; remote 65000; } }
 }
}`)
	require.NoError(t, err)
	peers, err := PeersFromConfigTree(tree)
	require.NoError(t, err)
	require.Len(t, peers, 1)
	require.True(t, peers[0].AIGPEnabled())
	require.False(t, peers[0].DisseminationOwner.IsSet())
}
