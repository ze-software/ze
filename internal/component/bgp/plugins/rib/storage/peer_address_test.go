// Design: docs/architecture/plugin/rib-storage-design.md -- explicit typed peer addresses.
package storage

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestAddressPeerRIBKeepsExplicitAddress preserves the typed and canonical
// string forms of explicitly supplied IPv4 and IPv6 addresses.
func TestAddressPeerRIBKeepsExplicitAddress(t *testing.T) {
	for _, text := range []string{"192.0.2.173", "2001:db8::173"} {
		t.Run(text, func(t *testing.T) {
			address := netip.MustParseAddr(text)
			routes, err := NewAddressPeerRIB(address)
			require.NoError(t, err)
			require.NotNil(t, routes)
			t.Cleanup(routes.Release)
			require.Equal(t, address, routes.PeerAddress())
			require.Equal(t, address.String(), routes.PeerAddr())
		})
	}
}

// TestAddressPeerRIBRefusesUnrecordedAddress ensures the typed constructor
// cannot return a store carrying a fabricated address or peer identity.
func TestAddressPeerRIBRefusesUnrecordedAddress(t *testing.T) {
	routes, err := NewAddressPeerRIB(netip.Addr{})
	require.Error(t, err)
	require.Nil(t, routes, "an invalid address must not create a peer store")
}

// TestGenericPeerRIBDoesNotInferAddress keeps generic protocol keys opaque,
// including a key whose text happens to look like a valid IP address.
func TestGenericPeerRIBDoesNotInferAddress(t *testing.T) {
	for _, key := range []string{"router-one:192.0.2.173", "192.0.2.173"} {
		t.Run(key, func(t *testing.T) {
			routes := NewPeerRIB(key)
			t.Cleanup(routes.Release)
			require.Equal(t, key, routes.PeerAddr())
			require.False(t, routes.PeerAddress().IsValid(), "only the typed constructor records an address")
		})
	}
}
