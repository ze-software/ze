// Design: docs/guide/configuration.md — claims live until their connection ends.
// Related: routerid_unique.go — per-Session ownership in the AS-wide registry.
package reactor

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestRouterIDRevalidationRetainsSiblingClaim covers two simultaneous Sessions
// that initially name the same speaker. Revalidating one to a different AS or
// identifier must retain the sibling's exclusion until that sibling ends.
func TestRouterIDRevalidationRetainsSiblingClaim(t *testing.T) {
	for _, next := range []routerIDKey{{peerAS: 65001, bgpID: 0x03030303}, {peerAS: 65002, bgpID: 0x02020202}} {
		t.Run(bgpIDString(next.bgpID), func(t *testing.T) {
			address := mustParseAddr("192.0.2.1")
			peer := NewPeer(NewPeerSettings(address, 65000, 65001, 0x01010101))
			incumbent := peer.newSession()
			t.Cleanup(incumbent.stopSessionTimers)
			candidate := peer.newSession()
			t.Cleanup(candidate.stopSessionTimers)
			other := NewPeer(NewPeerSettings(mustParseAddr("192.0.2.2"), 65000, 65001, 0x01010101))
			otherSession := other.newSession()
			t.Cleanup(otherSession.stopSessionTimers)
			var claims routerIDClaims
			const originalID = 0x02020202
			_, claimed := claims.claim(peer, incumbent, address, 65001, originalID)
			require.True(t, claimed)
			_, claimed = claims.claim(peer, candidate, address, 65001, originalID)
			require.True(t, claimed)
			_, claimed = claims.claim(peer, candidate, address, next.peerAS, next.bgpID)
			require.True(t, claimed)
			_, claimed = claims.claim(other, otherSession, other.settings.Address, 65001, originalID)
			require.False(t, claimed, "candidate revalidation cannot free the incumbent's key")
			claims.releaseSession(peer, incumbent)
			_, claimed = claims.claim(other, otherSession, other.settings.Address, 65001, originalID)
			require.True(t, claimed, "the ending connection releases its own old key")
			owner, held := claims.holder(next.peerAS, next.bgpID)
			require.True(t, held, "the revalidated candidate retains its different key")
			require.Same(t, peer, owner)
			claims.release(peer)
			_, held = claims.holder(next.peerAS, next.bgpID)
			require.False(t, held, "peer removal releases the revalidated connection")
			owner, held = claims.holder(65001, originalID)
			require.True(t, held)
			require.Same(t, other, owner, "peer removal cannot release the new peer's claim")
		})
	}
}
