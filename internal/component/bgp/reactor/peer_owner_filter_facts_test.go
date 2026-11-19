// Design: docs/architecture/core-design.md -- original runtime filter identity.
package reactor

import (
	"github.com/stretchr/testify/require"
	"net/netip"
	"testing"
)

// NewPeer, rather than an address lookup, supplies immutable identity to each
// captured forward view even when two configured peers share that address.
// MUTATION: omit PeerOwner from buildForwardFacts.
func TestRuntimeFilterFactsKeepSameAddressOriginalOwners(t *testing.T) {
	address := netip.MustParseAddr("192.0.2.82")
	a := NewPeer(&PeerSettings{Address: address, Name: "a", LocalAS: 65000, PeerAS: 65001, MigrationAS: 65001})
	b := NewPeer(&PeerSettings{Address: address, Name: "b", LocalAS: 65000, PeerAS: 65001})
	require.NotZero(t, a.sourceOwner)
	require.NotEqual(t, a.sourceOwner, b.sourceOwner)
	a.refreshForwardFacts()
	b.refreshForwardFacts()
	require.Equal(t, a.sourceOwner, a.forwardFacts().filterInfo.PeerOwner)
	require.Equal(t, b.sourceOwner, b.forwardFacts().filterInfo.PeerOwner)
	require.Equal(t, address, a.forwardFacts().filterInfo.Address)
	require.Equal(t, address, b.forwardFacts().filterInfo.Address)
	require.True(t, a.forwardFacts().filterInfo.IsIBGP, "actual migration settings produce internal-session fact")
	require.False(t, b.forwardFacts().filterInfo.IsIBGP, "same address and AS facts without migration stay external")
	require.Zero(t, a.settings.SessionFacts(65001).PeerOwner, "configuration facts have no runtime owner")
}
