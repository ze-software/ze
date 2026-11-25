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

// TestGroupPeerBusInheritancePreservesOwnAndGroupBlocks verifies that
// PeersFromConfigTree separates a grouped peer's own `bus` block from its
// parent group's `bus` block so the live delivery graph can attribute inherited
// group items to the group rather than stamping them as written on the peer.
//
// VALIDATES: PeersFromConfigTree populates OwnBus, GroupBus, and GroupBusSource
// alongside resolved Bus for grouped peers (S8).
// PREVENTS: ResolveBGPTree flattening group and peer bus blocks into one written peer block.
func TestGroupPeerBusInheritancePreservesOwnAndGroupBlocks(t *testing.T) {
	tree, err := config.ParseTreeForValidation(`
bgp {
 router-id 192.0.2.1;
 session { asn { local 65000; } }
 group transit {
  bus {
   send { to [ /bus/endpoint/bgp-rib ]; }
   receive { from [ /bus/endpoint/bgp-rib ]; }
  }
  peer t1 {
   connection { remote { ip 192.0.2.2; } local { ip 192.0.2.1; } }
   session { asn { local 65000; remote 65001; } }
   bus {
    send { to [ /bus/endpoint/collector ]; }
   }
  }
 }
}`)
	require.NoError(t, err)
	peers, err := PeersFromConfigTree(tree)
	require.NoError(t, err)
	require.Len(t, peers, 1)

	ps := peers[0]
	require.Equal(t, "/bgp/group/transit > bus", ps.GroupBusSource)
	require.Equal(t, []string{"/bus/endpoint/bgp-rib"}, ps.GroupBus.Send.Targets)
	require.Equal(t, []string{"/bus/endpoint/bgp-rib"}, ps.GroupBus.Receive.Targets)
	require.Equal(t, []string{"/bus/endpoint/collector"}, ps.OwnBus.Send.Targets)
	require.False(t, ps.OwnBus.Receive.TargetsPresent)
	require.Equal(t, []string{"/bus/endpoint/collector"}, ps.Bus.Send.Targets)
	require.Equal(t, []string{"/bus/endpoint/bgp-rib"}, ps.Bus.Receive.Targets)
}
