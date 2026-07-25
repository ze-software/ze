// Design: docs/architecture/plugin/rib-storage-design.md — BMP injection tests

package rib

import (
	"encoding/json"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/storage"
	"github.com/ze-software/ze/internal/core/family"
)

// minimalAttrs is a valid attribute set with ORIGIN(IGP) + AS_PATH(empty) + NEXT_HOP(10.0.0.1).
// Satisfies RFC 7606 mandatory attribute requirements for UPDATE with NLRI.
var minimalAttrs = []byte{
	0x40, 0x01, 0x01, 0x00, // ORIGIN: flags=0x40, type=1, len=1, value=IGP(0)
	0x40, 0x02, 0x00, // AS_PATH: flags=0x40, type=2, len=0 (empty)
	0x40, 0x03, 0x04, 10, 0, 0, 1, // NEXT_HOP: flags=0x40, type=3, len=4, value=10.0.0.1
}

// ipv4UpdateBody builds a minimal BGP UPDATE body (no header) announcing
// the given IPv4 prefix with mandatory attributes.
// Wire format: withdrawn_len(2) + attr_len(2) + attrs + NLRI(prefix_len_byte + prefix_bytes).
func ipv4UpdateBody(prefixLen byte, prefixBytes ...byte) []byte {
	attrLen := len(minimalAttrs)
	buf := []byte{
		0x00, 0x00, // withdrawn routes length = 0
		byte(attrLen >> 8), byte(attrLen), // total path attribute length
	}
	buf = append(buf, minimalAttrs...)
	buf = append(buf, prefixLen)
	return append(buf, prefixBytes...)
}

// ipv4WithdrawBody builds a BGP UPDATE body that withdraws the given prefix.
// Wire format: withdrawn_len(2) + withdrawn(prefix_len + bytes) + attr_len(2).
func ipv4WithdrawBody(prefixLen byte, prefixBytes ...byte) []byte {
	wdLen := 1 + len(prefixBytes)
	buf := []byte{
		byte(wdLen >> 8), byte(wdLen), // withdrawn routes length
		prefixLen, // withdrawn prefix length
	}
	buf = append(buf, prefixBytes...)
	buf = append(buf, 0x00, 0x00) // total path attribute length = 0
	return buf
}

// TestInjectWireRouteStoresBMPProtocol verifies handleInjectWireRoute stores
// routes under ribInPool[bmpProtocolID], not under bgpPeers.
//
// VALIDATES: AC-1 — Route stored in ribInPool[bmpProtocolID] under composite peer key.
// PREVENTS: BMP routes leaking into bgpPeers.
func TestInjectWireRouteStoresBMPProtocol(t *testing.T) {
	r := newTestRIBManager(t)
	ipv4Uni := family.Family{AFI: 1, SAFI: 1}

	update := ipv4UpdateBody(24, 10, 0, 0)
	err := r.handleInjectWireRoute("bmp", "router1:10.0.0.1", update)
	require.NoError(t, err)

	// Route must be in bmpProtocolID slot.
	bmpPeers := r.ribInPool[bmpProtocolID]
	require.NotNil(t, bmpPeers["router1:10.0.0.1"])
	assert.Equal(t, 1, bmpPeers["router1:10.0.0.1"].Len())

	// Route must NOT be in bgpPeers (composite keys cannot even be
	// netip.Addr keys; assert the map stayed untouched).
	assert.Empty(t, r.bgpPeers, "BMP injection must not create bgpPeers entries")

	// Verify the actual prefix is stored by looking it up.
	nlri := []byte{24, 10, 0, 0}
	_, ok := bmpPeers["router1:10.0.0.1"].Lookup(ipv4Uni, nlri)
	assert.True(t, ok, "prefix 10.0.0.0/24 should be stored")
}

// TestBMPRoutesExcludedFromBestPath verifies gatherCandidatesLocked does
// not consider routes stored under bmpProtocolID.
//
// VALIDATES: AC-3 — Best-path selects only real peer's route. BMP route not considered.
// PREVENTS: BMP-monitored routes entering FIB/loc-RIB.
func TestBMPRoutesExcludedFromBestPath(t *testing.T) {
	r := newTestRIBManager(t)
	ipv4Uni := family.Family{AFI: 1, SAFI: 1}
	nlri := []byte{24, 10, 0, 0}
	attrBytes := []byte{0x40, 0x01, 0x01, 0x00} // ORIGIN IGP

	// Insert same prefix via both BGP and BMP.
	r.bgpPeers[netip.MustParseAddr("10.0.0.1")] = storage.NewPeerRIB("10.0.0.1")
	r.bgpPeers[netip.MustParseAddr("10.0.0.1")].Insert(ipv4Uni, attrBytes, nlri, true)

	bmpPeers := r.ribInPool[bmpProtocolID]
	bmpPeers["router1:10.0.0.2"] = storage.NewPeerRIB("router1:10.0.0.2")
	bmpPeers["router1:10.0.0.2"].Insert(ipv4Uni, attrBytes, nlri, true)

	r.peerMu.RLock()
	candidates := r.gatherCandidatesLocked(ipv4Uni, nlri)
	r.peerMu.RUnlock()

	require.Len(t, candidates, 1, "only BGP peer should be a candidate")
	assert.Equal(t, "10.0.0.1", candidates[0].PeerAddr)
}

// TestInjectWireRouteWithdraw verifies withdrawal removes the route from
// the bmpProtocolID peer.
//
// VALIDATES: withdraw within InjectWireRoute removes the correct prefix.
// PREVENTS: stale routes persisting after withdrawal.
func TestInjectWireRouteWithdraw(t *testing.T) {
	r := newTestRIBManager(t)
	ipv4Uni := family.Family{AFI: 1, SAFI: 1}
	nlri := []byte{24, 10, 0, 0}

	// Inject a route.
	update := ipv4UpdateBody(24, 10, 0, 0)
	err := r.handleInjectWireRoute("bmp", "router1:peer1", update)
	require.NoError(t, err)

	bmpPeers := r.ribInPool[bmpProtocolID]
	require.Equal(t, 1, bmpPeers["router1:peer1"].Len())

	// Withdraw it.
	withdraw := ipv4WithdrawBody(24, 10, 0, 0)
	err = r.handleInjectWireRoute("bmp", "router1:peer1", withdraw)
	require.NoError(t, err)

	_, ok := bmpPeers["router1:peer1"].Lookup(ipv4Uni, nlri)
	assert.False(t, ok, "prefix should be withdrawn")
	assert.Equal(t, 0, bmpPeers["router1:peer1"].Len())
}

// TestWithdrawAllForPeer verifies withdrawAllForPeer removes all routes
// for a specific peer under a protocol.
//
// VALIDATES: AC-5 — BMP Peer Down withdraws all routes for that peer.
// PREVENTS: orphaned routes after peer down.
func TestWithdrawAllForPeer(t *testing.T) {
	r := newTestRIBManager(t)
	ipv4Uni := family.Family{AFI: 1, SAFI: 1}
	attrBytes := []byte{0x40, 0x01, 0x01, 0x00}

	bmpPeers := r.ribInPool[bmpProtocolID]
	bmpPeers["router1:peer1"] = storage.NewPeerRIB("router1:peer1")
	bmpPeers["router1:peer1"].Insert(ipv4Uni, attrBytes, []byte{24, 10, 0, 0}, true)
	bmpPeers["router1:peer1"].Insert(ipv4Uni, attrBytes, []byte{24, 10, 0, 1}, true)
	bmpPeers["router1:peer2"] = storage.NewPeerRIB("router1:peer2")
	bmpPeers["router1:peer2"].Insert(ipv4Uni, attrBytes, []byte{24, 10, 0, 2}, true)

	r.withdrawAllForPeer("bmp", "router1:peer1")

	assert.Nil(t, r.ribInPool[bmpProtocolID]["router1:peer1"], "peer1 should be removed")
	assert.NotNil(t, r.ribInPool[bmpProtocolID]["router1:peer2"], "peer2 should remain")
	assert.Equal(t, 1, r.ribInPool[bmpProtocolID]["router1:peer2"].Len())
}

// TestWithdrawAllForRouter verifies withdrawAllForRouter removes all peers
// matching the router prefix.
//
// VALIDATES: AC-6 — BMP session disconnect withdraws all peers of that router.
// PREVENTS: orphaned routes from disconnected routers.
func TestWithdrawAllForRouter(t *testing.T) {
	r := newTestRIBManager(t)
	ipv4Uni := family.Family{AFI: 1, SAFI: 1}
	attrBytes := []byte{0x40, 0x01, 0x01, 0x00}

	bmpPeers := r.ribInPool[bmpProtocolID]
	bmpPeers["192.168.1.1:5678:peer1"] = storage.NewPeerRIB("192.168.1.1:5678:peer1")
	bmpPeers["192.168.1.1:5678:peer1"].Insert(ipv4Uni, attrBytes, []byte{24, 10, 0, 0}, true)
	bmpPeers["192.168.1.1:5678:peer2"] = storage.NewPeerRIB("192.168.1.1:5678:peer2")
	bmpPeers["192.168.1.1:5678:peer2"].Insert(ipv4Uni, attrBytes, []byte{24, 10, 0, 1}, true)
	bmpPeers["192.168.2.1:9999:peer3"] = storage.NewPeerRIB("192.168.2.1:9999:peer3")
	bmpPeers["192.168.2.1:9999:peer3"].Insert(ipv4Uni, attrBytes, []byte{24, 10, 0, 2}, true)

	r.withdrawAllForRouter("bmp", "192.168.1.1:5678")

	assert.Nil(t, r.ribInPool[bmpProtocolID]["192.168.1.1:5678:peer1"], "router1 peer1 should be removed")
	assert.Nil(t, r.ribInPool[bmpProtocolID]["192.168.1.1:5678:peer2"], "router1 peer2 should be removed")
	assert.NotNil(t, r.ribInPool[bmpProtocolID]["192.168.2.1:9999:peer3"], "router2 peer should remain")
}

// TestShowProtocolPipelineBMP verifies showProtocolPipeline returns routes
// only from the requested protocol.
//
// VALIDATES: AC-10 — bmp rib show displays BMP routes only.
// PREVENTS: BGP routes appearing in BMP show output.
func TestShowProtocolPipelineBMP(t *testing.T) {
	r := newTestRIBManager(t)
	ipv4Uni := family.Family{AFI: 1, SAFI: 1}
	attrBytes := []byte{0x40, 0x01, 0x01, 0x00}

	// Insert BGP route.
	r.bgpPeers[netip.MustParseAddr("10.0.0.1")] = storage.NewPeerRIB("10.0.0.1")
	r.bgpPeers[netip.MustParseAddr("10.0.0.1")].Insert(ipv4Uni, attrBytes, []byte{24, 10, 0, 0}, true)

	// Insert BMP route.
	bmpPeers := r.ribInPool[bmpProtocolID]
	bmpPeers["router1:peer1"] = storage.NewPeerRIB("router1:peer1")
	bmpPeers["router1:peer1"].Insert(ipv4Uni, attrBytes, []byte{24, 10, 0, 1}, true)

	result := r.showProtocolPipeline("bmp", "", nil)

	var parsed map[string]any
	require.NoError(t, json.Unmarshal(mustMarshal(t, result), &parsed))

	ribIn, ok := parsed["adj-rib-in"].(map[string]any)
	require.True(t, ok, "result should have adj-rib-in")

	assert.Contains(t, ribIn, "router1:peer1", "BMP peer should be present")
	assert.NotContains(t, ribIn, "10.0.0.1", "BGP peer should not be in BMP show")
}

// TestShowProtocolPipelineSelector verifies showProtocolPipeline filters
// by peer selector when provided.
//
// VALIDATES: LG routes endpoint filters by peer name.
func TestShowProtocolPipelineSelector(t *testing.T) {
	r := newTestRIBManager(t)
	ipv4Uni := family.Family{AFI: 1, SAFI: 1}
	attrBytes := []byte{0x40, 0x01, 0x01, 0x00}

	bmpPeers := r.ribInPool[bmpProtocolID]
	bmpPeers["router1:peer1"] = storage.NewPeerRIB("router1:peer1")
	bmpPeers["router1:peer1"].Insert(ipv4Uni, attrBytes, []byte{24, 10, 0, 0}, true)
	bmpPeers["router1:peer2"] = storage.NewPeerRIB("router1:peer2")
	bmpPeers["router1:peer2"].Insert(ipv4Uni, attrBytes, []byte{24, 10, 0, 1}, true)

	result := r.showProtocolPipeline("bmp", "router1:peer1", nil)

	var parsed map[string]any
	require.NoError(t, json.Unmarshal(mustMarshal(t, result), &parsed))

	ribIn, ok := parsed["adj-rib-in"].(map[string]any)
	require.True(t, ok)
	assert.Contains(t, ribIn, "router1:peer1")
	assert.NotContains(t, ribIn, "router1:peer2")
}

// TestInjectWireRouteShortBody verifies that a too-short UPDATE body
// is rejected without panic.
//
// VALIDATES: boundary test — UPDATE body below minimum length.
func TestInjectWireRouteShortBody(t *testing.T) {
	r := newTestRIBManager(t)

	err := r.handleInjectWireRoute("bmp", "router1:peer1", []byte{0x00, 0x00})
	require.NoError(t, err, "short body should be silently dropped, not error")

	bmpPeers := r.ribInPool[bmpProtocolID]
	assert.Nil(t, bmpPeers["router1:peer1"], "no PeerRIB should be created for short body")
}

// TestInjectWireRouteUnknownProtocol verifies that an unregistered protocol
// is silently ignored.
func TestInjectWireRouteUnknownProtocol(t *testing.T) {
	r := newTestRIBManager(t)

	update := ipv4UpdateBody(24, 10, 0, 0)
	err := r.handleInjectWireRoute("nonexistent", "peer1", update)
	require.NoError(t, err)

	// No crash, no new entries in ribInPool.
	for _, protoPeers := range r.ribInPool {
		assert.Empty(t, protoPeers)
	}
}

// TestBGPShowExcludesBMP verifies the inbound show pipeline (show bgp rib)
// does not include BMP protocol routes.
//
// VALIDATES: AC-10 — show bgp rib excludes BMP routes.
func TestBGPShowExcludesBMP(t *testing.T) {
	r := newTestRIBManager(t)
	ipv4Uni := family.Family{AFI: 1, SAFI: 1}
	attrBytes := []byte{0x40, 0x01, 0x01, 0x00}

	r.bgpPeers[netip.MustParseAddr("10.0.0.1")] = storage.NewPeerRIB("10.0.0.1")
	r.bgpPeers[netip.MustParseAddr("10.0.0.1")].Insert(ipv4Uni, attrBytes, []byte{24, 10, 0, 0}, true)

	bmpPeers := r.ribInPool[bmpProtocolID]
	bmpPeers["router1:peer1"] = storage.NewPeerRIB("router1:peer1")
	bmpPeers["router1:peer1"].Insert(ipv4Uni, attrBytes, []byte{24, 10, 0, 1}, true)

	result := r.showPipeline("*", nil)

	var parsed map[string]any
	require.NoError(t, json.Unmarshal(mustMarshal(t, result), &parsed))

	ribIn, ok := parsed["adj-rib-in"].(map[string]any)
	require.True(t, ok)
	assert.Contains(t, ribIn, "10.0.0.1", "BGP peer should be present")
	assert.NotContains(t, ribIn, "router1:peer1", "BMP peer must not appear in show bgp rib")
}
