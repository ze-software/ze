// Design: docs/architecture/config/transaction-protocol.md -- a plugin's verify, apply and rollback of one parsed config
// Related: config_tx_test.go -- TestRIBReselectOnRuleChange, the maximum-paths trigger

package rib

import (
	"encoding/binary"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/storage"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	"github.com/ze-software/ze/internal/core/rib/routetype"
)

// relaxAttrs is ORIGIN, NEXT_HOP nh, LOCAL_PREF 100 and a one-hop AS_PATH
// [origin]: two paths built with different origins have AS_PATHs of equal
// length that differ, which only relax-as-path lets share a multipath set.
func relaxAttrs(nh [4]byte, origin uint32) []byte {
	attrs := makeAttrBytes(nh)
	attrs = append(attrs, 0x40, byte(attribute.AttrLocalPref), 4)
	attrs = binary.BigEndian.AppendUint32(attrs, 100)
	attrs = append(attrs, 0x40, byte(attribute.AttrASPath), 6, 2, 1)
	return binary.BigEndian.AppendUint32(attrs, origin)
}

// TestRIBReselectOnRelaxASPathChange proves AC-6 for the relax-as-path
// trigger: a reload that turns relax-as-path on reselects a stored prefix
// whose two paths differ only in AS_PATH into a two-path set, with no UPDATE,
// and the rollback reselects it back to one path.
// Method: the Loc-RIB entry's ECMP set is read after configure, apply and
// rollback, with maximum-paths 2 throughout.
// VALIDATES: spec-plugin-reload-applies-or-refuses AC-6 (relax-as-path).
// PREVENTS: installConfig leaving relax-as-path out of the rule comparison,
// so the edit waits for the next UPDATE of each prefix.
func TestRIBReselectOnRelaxASPathChange(t *testing.T) {
	r := newRIBManager(nil)
	loc := locrib.NewRIB()
	r.SetLocRIB(loc)
	t.Cleanup(func() { r.SetLocRIB(nil) })
	tx := r.newConfigTx(false)
	require.NoError(t, tx.Configure(ribSections(`"multipath":{"maximum-paths":2,"relax-as-path":false}`)))

	wirePrefix := ipv4Prefix(24, 10, 31, 0)
	for i, text := range []string{"192.0.2.1", "192.0.2.2"} {
		peer := netip.MustParseAddr(text)
		r.peerMeta[peer] = &peerMetadata{PeerASN: 65001, LocalASN: 65001}
		routes := storage.NewPeerRIB(text)
		r.bgpPeers[peer] = routes
		t.Cleanup(routes.Release)
		routes.Insert(family.IPv4Unicast, relaxAttrs([4]byte{198, 51, 100, byte(i + 1)}, uint32(64500+i)), wirePrefix)
	}
	r.reselectAIGPRoutes()
	ecmp := func() int {
		path, _, found := loc.LPM(family.IPv4Unicast, netip.MustParseAddr("10.31.0.1"))
		require.True(t, found)
		return len(path.ECMP)
	}
	require.Equal(t, 0, ecmp(), "different AS_PATHs without relax-as-path select one path")

	require.NoError(t, tx.Verify(ribSections(`"multipath":{"maximum-paths":2,"relax-as-path":true}`)))
	require.NoError(t, tx.Apply())
	require.Equal(t, 1, ecmp(), "the apply reselects the stored prefix into two paths")

	require.NoError(t, tx.Rollback())
	require.Equal(t, 0, ecmp(), "the rollback reselects the stored prefix back to one path")
}

// TestRIBReselectOnBlackholeRuleChange proves AC-6 for the blackhole honoring
// trigger: a reload that agrees to honor BLACKHOLE from a peer turns its
// stored, tagged, authorized route into a discard in the Loc-RIB with no
// UPDATE, and the rollback turns it back into a forwarding route.
// Method: the Loc-RIB best path's route type is read after configure, apply
// and rollback. The winner never changes, so only the route-type comparison
// in checkBestPathChange publishes the change.
// VALIDATES: spec-plugin-reload-applies-or-refuses AC-6 (blackhole rules).
// PREVENTS: installConfig leaving the blackhole rules out of the rule
// comparison, so a new agreement waits for the peer's next UPDATE.
func TestRIBReselectOnBlackholeRuleChange(t *testing.T) {
	peer := netip.MustParseAddr("192.0.2.1")
	r := newTestRIBManagerWithBus(newTestEventBus())
	loc := locrib.NewRIB()
	r.SetLocRIB(loc)
	t.Cleanup(func() { r.SetLocRIB(nil) })
	r.peerMeta[peer] = &peerMetadata{PeerASN: 65001, LocalASN: 65000}
	routes := storage.NewPeerRIB(peer.String())
	r.bgpPeers[peer] = routes
	t.Cleanup(routes.Release)
	tx := r.newConfigTx(false)
	peerBody := func(blackhole string) string {
		return `"peer":{"edge":{"connection":{"remote":{"ip":"192.0.2.1"}}` + blackhole + `}}`
	}
	require.NoError(t, tx.Configure(ribSections(peerBody(""))))

	routes.Insert(family.IPv4Unicast, blackholeAttrBytes([4]byte{192, 168, 1, 1}, blackholeValue), ipv4Prefix(32, 10, 0, 0, 1))
	r.reselectAIGPRoutes()
	host := netip.MustParsePrefix("10.0.0.1/32")
	require.NotEqual(t, routetype.Blackhole, blackholeLocRIBType(t, loc, host), "no agreement: an ordinary route")

	require.NoError(t, tx.Verify(ribSections(peerBody(`,"blackhole":{"communities":["blackhole"],"prefixes":["10.0.0.0/24"]}`))))
	require.NoError(t, tx.Apply())
	require.Equal(t, routetype.Blackhole, blackholeLocRIBType(t, loc, host), "the apply reselects the stored route into a discard")

	require.NoError(t, tx.Rollback())
	require.NotEqual(t, routetype.Blackhole, blackholeLocRIBType(t, loc, host), "the rollback reselects it back to an ordinary route")
}

// TestReselectKeyOneElectionPerRoute proves the reselect walk's dedupe key:
// the same prefix from two add-path ids, or from a peer without add-path, is
// one election, and two prefixes are two. A VPN route, which has no prefix and
// is keyed by its route key, is one election across add-path ids and labels,
// and two Route Distinguishers are two.
// VALIDATES: reselectAIGPRoutes runs one election per route, not per stored path.
// PREVENTS: a reload's reselect running each prefix's election once per peer
// that holds it, inside the apply deadline.
func TestReselectKeyOneElectionPerRoute(t *testing.T) {
	var scratch [64]byte
	plain := ipv4Prefix(24, 10, 32, 0)
	pathOne := append([]byte{0, 0, 0, 1}, plain...)
	pathTwo := append([]byte{0, 0, 0, 2}, plain...)
	other := ipv4Prefix(24, 10, 33, 0)

	keyOf := func(nlri []byte, addPath bool) reselectKey {
		key, ok := newReselectKey(family.IPv4Unicast, nlri, addPath, scratch[:])
		require.True(t, ok)
		return key
	}
	require.Equal(t, keyOf(plain, false), keyOf(pathOne, true), "add-path id 1 and no add-path name one route")
	require.Equal(t, keyOf(pathOne, true), keyOf(pathTwo, true), "two add-path ids of one prefix are one route")
	require.NotEqual(t, keyOf(plain, false), keyOf(other, false), "two prefixes are two routes")

	vpnKeyOf := func(nlri []byte, addPath bool) reselectKey {
		key, ok := newReselectKey(vpnv4Family, nlri, addPath, scratch[:])
		require.True(t, ok)
		return key
	}
	vpnPlain := vpnv4NLRI(100, vpnRouteKeyRD, 0x0a)
	vpnPathOne := framedKey(1, vpnv4NLRI(100, vpnRouteKeyRD, 0x0a))
	vpnPathTwo := framedKey(2, vpnv4NLRI(101, vpnRouteKeyRD, 0x0a))
	vpnOtherRD := vpnv4NLRI(100, [8]byte{0, 0, 0, 0, 0, 0, 0, 2}, 0x0a)
	require.Equal(t, vpnKeyOf(vpnPlain, false), vpnKeyOf(vpnPathOne, true), "a VPN route with and without add-path is one route")
	require.Equal(t, vpnKeyOf(vpnPathOne, true), vpnKeyOf(vpnPathTwo, true), "two add-path ids and labels of one VPN route are one route")
	require.NotEqual(t, vpnKeyOf(vpnPlain, false), vpnKeyOf(vpnOtherRD, false), "two Route Distinguishers are two routes")
}
