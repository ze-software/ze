// Design: docs/architecture/config/syntax.md -- scoped TCP endpoints and dynamic groups
// Related: reactor_dynamic.go -- range matching does not erase the created peer's transport zone

package reactor

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDynamicScopedTransportRangeKeepsPeerIdentity separates prefix membership
// from scoped TCP identity. An admitted address chooses the longest matching
// range, then the resulting peer keeps the original interface zone in its key.
func TestDynamicScopedTransportRangeKeepsPeerIdentity(t *testing.T) {
	r := New(&Config{LocalAS: 65000})
	broad := newTestDynamicGroup("broad", []string{"fe80::/10"}, 10)
	narrow := newTestDynamicGroup("narrow", []string{"fe80::/64"}, 10)
	narrow.Settings.LocalAddress = netip.MustParseAddr("fe80::1%if0")
	r.dynamicGroups = []*DynamicGroupConfig{broad, narrow}
	address := netip.MustParseAddr("fe80::2%if0")

	require.True(t, broad.containsAddr(address), "a transport zone does not change prefix membership")
	require.True(t, narrow.containsAddr(address))
	r.mu.RLock()
	chosen := r.findDynamicGroup(address)
	r.mu.RUnlock()
	require.Same(t, narrow, chosen, "the more specific range must win")

	peer := r.tryCreateDynamicPeer(address)
	require.NotNil(t, peer, "the scoped incoming address must produce a useful peer")
	assert.Equal(t, address, peer.Settings().Address)
	assert.Equal(t, "narrow", peer.Settings().GroupName)
	key := netip.MustParseAddrPort("[fe80::2%if0]:179")
	assert.Equal(t, key, peer.Settings().PeerKey())
	assert.Same(t, peer, r.peers[key])
	assert.Equal(t, int32(1), narrow.ActivePeers.Load())
	assert.Zero(t, broad.ActivePeers.Load())

	outside := netip.MustParseAddr("2001:db8::2%if0")
	assert.False(t, broad.containsAddr(outside))
	assert.False(t, narrow.containsAddr(outside))
	r.mu.RLock()
	outsideGroup := r.findDynamicGroup(outside)
	r.mu.RUnlock()
	assert.Nil(t, outsideGroup)
	assert.Nil(t, r.tryCreateDynamicPeer(outside))
	assert.Len(t, r.peers, 1, "an unmatched address must not create a second peer")
}
