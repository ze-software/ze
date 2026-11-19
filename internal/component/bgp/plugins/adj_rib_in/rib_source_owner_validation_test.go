// Design: docs/architecture/plugin/rib-storage-design.md -- exact received validation identities.
package adj_rib_in

import (
	"net/netip"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
)

// TestValidationInstalledPathUsesOriginalOwner asks the actual receive gate
// about the same stored path under its owner, an address alias and no owner.
func TestValidationInstalledPathUsesOriginalOwner(t *testing.T) {
	r := newTestManager(t)
	peer := netip.MustParseAddr("192.0.2.188")
	prefix := netip.MustParsePrefix("10.77.0.0/24")
	key := compactRouteKey{Fam: family.IPv4Unicast, Prefix: prefix}
	r.ribIn[receivedPeer{address: peer, owner: 1701}] = newSeqMap()
	r.ribIn[receivedPeer{address: peer, owner: 1701}].Put(key, 1, &RawRoute{ConnectionID: 41, Family: family.IPv4Unicast, SourceOwner: 1701, MsgID: 71})
	for _, owner := range []uint64{1701, 1702, 0} {
		t.Run(strconv.FormatUint(owner, 10), func(t *testing.T) {
			query := ribevents.ValidationRoute{ConnectionID: 41, Peer: peer, PeerOwner: owner, Family: family.IPv4Unicast, Prefix: prefix}
			require.Equal(t, owner == 1701, r.routeEligible(query, 71))
			require.Equal(t, owner == 1701, r.routePresent(query))
		})
	}
}

// TestValidationPendingPathUsesOriginalOwner preserves only the pending path's
// own identifier; its existence never makes that unvalidated path eligible.
func TestValidationPendingPathUsesOriginalOwner(t *testing.T) {
	r := newTestManager(t)
	peer := netip.MustParseAddr("192.0.2.188")
	prefix := netip.MustParsePrefix("10.77.0.0/24")
	key := compactRouteKey{Fam: family.IPv4Unicast, Prefix: prefix}
	r.pending[pendingKey(receivedPeer{address: peer, owner: 1701}, key)] = &pendingRoute{
		route: &RawRoute{ConnectionID: 41, Family: family.IPv4Unicast, SourceOwner: 1701, MsgID: 71},
	}
	for _, owner := range []uint64{1701, 1702, 0} {
		t.Run(strconv.FormatUint(owner, 10), func(t *testing.T) {
			query := ribevents.ValidationRoute{ConnectionID: 41, Peer: peer, PeerOwner: owner, Family: family.IPv4Unicast, Prefix: prefix}
			require.False(t, r.routeEligible(query, 71))
			require.Equal(t, owner == 1701, r.routePresent(query))
		})
	}
}
