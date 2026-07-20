// RFC: rfc/short/rfc9494.md -- Long-Lived Graceful Restart requirement bindings.
//
// These tests bind RFC 9494 MUST-level requirements to the producing functions in
// this package: the LLGR_STALE / NO_LLGR community commands (rib_commands_community.go)
// and the stale-level depreference step of best-path (bestpath.go).

package rib

import (
	"encoding/json"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/rib/pool"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/rib/storage"
	"codeberg.org/thomas-mangin/ze/internal/core/family"
)

// llgrStaleCommunity is LLGR_STALE (0xFFFF0006) in wire form.
var llgrStaleCommunity = []byte{0xFF, 0xFF, 0x00, 0x06}

// noExportCommunity is NO_EXPORT (0xFFFFFF01) in wire form, the community the LLGR
// egress filter adds when a stale route goes to a non-LLGR iBGP neighbor.
var noExportCommunity = []byte{0xFF, 0xFF, 0xFF, 0x01}

// TestRFC9494_FreshRoutesDoNotGetLLGRStale verifies LLGR_STALE only lands on stale routes.
//
// VALIDATES: attach-community skips routes that are not stale.
// PREVENTS: live routes being marked as long-lived stale and depreferenced.
//
// RFC requirement: RFC9494-4.2-4 negative -- the marking is scoped to retained stale routes:
// attachCommunityCommand returns early for every entry whose StaleLevel is StaleLevelFresh
// (internal/component/bgp/plugins/rib/rib_commands_community.go:97-99), so with no preceding
// mark-stale nothing is attached and no route gains the community.
func TestRFC9494_FreshRoutesDoNotGetLLGRStale(t *testing.T) {
	t.Parallel()
	r := setupGRTestRIB(t)

	// No mark-stale: every route is fresh.
	status, data, err := r.handleCommand("request bgp rib attach-community", "*",
		[]string{"192.0.2.1", "ipv4/unicast", "ffff0006"})
	require.NoError(t, err)
	assert.Equal(t, statusDone, status)

	var result map[string]any
	require.NoError(t, json.Unmarshal(mustMarshal(t, data), &result))
	assert.Equal(t, float64(0), result["attached"], "no fresh route may be marked LLGR_STALE")

	entry, found := r.bgpPeers[netip.MustParseAddr("192.0.2.1")].Lookup(family.IPv4Unicast, []byte{24, 10, 0, 0})
	require.True(t, found, "the fresh route is still present")
	assert.Equal(t, storage.StaleLevelFresh, entry.StaleLevel, "fresh route keeps its level")
	assert.False(t, entry.GetBundle().HasCommunities(), "no community attached to a fresh route")
}

// TestRFC9494_StaleRouteWithoutNoLLGRRetained verifies the NO_LLGR purge is selective.
//
// VALIDATES: delete-with-community only removes routes carrying the named community.
// PREVENTS: the NO_LLGR purge wiping out routes that should enter the LLGR period.
//
// RFC requirement: RFC9494-4.2-5 negative -- routes NOT carrying NO_LLGR are retained:
// deleteWithCommunityCommand collects a route only when containsCommunity finds the exact
// 4-byte value in its COMMUNITIES blob
// (internal/component/bgp/plugins/rib/rib_commands_community.go:166-175), so a stale route
// carrying only LLGR_STALE survives the NO_LLGR sweep.
func TestRFC9494_StaleRouteWithoutNoLLGRRetained(t *testing.T) {
	t.Parallel()
	r := setupGRTestRIB(t)
	peerRIB := r.bgpPeers[netip.MustParseAddr("192.0.2.1")]

	_, _, err := r.handleCommand("request bgp rib mark-stale", "*", []string{"192.0.2.1", "120"})
	require.NoError(t, err)
	_, _, err = r.handleCommand("request bgp rib attach-community", "*",
		[]string{"192.0.2.1", "ipv4/unicast", "ffff0006"})
	require.NoError(t, err)

	before := peerRIB.Len()

	// NO_LLGR sweep: this route carries LLGR_STALE, not NO_LLGR.
	status, data, err := r.handleCommand("request bgp rib delete-with-community", "*",
		[]string{"192.0.2.1", "ipv4/unicast", "ffff0007"})
	require.NoError(t, err)
	assert.Equal(t, statusDone, status)

	var result map[string]any
	require.NoError(t, json.Unmarshal(mustMarshal(t, data), &result))
	assert.Equal(t, float64(0), result["deleted"], "no route deleted without NO_LLGR")
	assert.Equal(t, before, peerRIB.Len(), "the retained stale route is untouched")

	_, found := peerRIB.Lookup(family.IPv4Unicast, []byte{24, 10, 0, 0})
	assert.True(t, found, "stale route without NO_LLGR is retained for the LLGR period")
}

// TestRFC9494_LLGRStalePreservedOnFurtherAdvertisement verifies LLGR_STALE survives the
// attribute changes applied when a stale route is advertised onward.
//
// VALIDATES: a subsequent community modification keeps LLGR_STALE in the COMMUNITIES attribute.
// PREVENTS: LLGR_STALE being dropped when NO_EXPORT is added for a non-LLGR iBGP neighbor.
//
// RFC requirement: RFC9494-4.3-2 positive -- attachCommunity copies the existing COMMUNITIES
// blob and appends to it (internal/component/bgp/plugins/rib/rib_commands_community.go:222-236),
// so adding NO_EXPORT on the readvertise path leaves LLGR_STALE in place; it is never rewritten
// or truncated.
func TestRFC9494_LLGRStalePreservedOnFurtherAdvertisement(t *testing.T) {
	t.Parallel()
	r := setupGRTestRIB(t)
	peerRIB := r.bgpPeers[netip.MustParseAddr("192.0.2.1")]

	_, _, err := r.handleCommand("request bgp rib mark-stale", "*", []string{"192.0.2.1", "120"})
	require.NoError(t, err)
	_, _, err = r.handleCommand("request bgp rib attach-community", "*",
		[]string{"192.0.2.1", "ipv4/unicast", "ffff0006"})
	require.NoError(t, err)

	// Readvertisement toward a non-LLGR iBGP neighbor adds NO_EXPORT.
	_, _, err = r.handleCommand("request bgp rib attach-community", "*",
		[]string{"192.0.2.1", "ipv4/unicast", "ffffff01"})
	require.NoError(t, err)

	entry, found := peerRIB.Lookup(family.IPv4Unicast, []byte{24, 10, 0, 0})
	require.True(t, found)
	commData, err := pool.Communities.Get(entry.GetBundle().Communities)
	require.NoError(t, err)
	assert.True(t, containsCommunity(commData, llgrStaleCommunity), "LLGR_STALE is still present")
	assert.True(t, containsCommunity(commData, noExportCommunity), "NO_EXPORT was added alongside it")
}

// TestRFC9494_LLGRStaleRouteIsLeastPreferred verifies marking and depreference are one step.
//
// VALIDATES: attaching LLGR_STALE raises the route to the depreference threshold, and a
// candidate at that level loses to a fresh candidate with worse attributes.
// PREVENTS: an LLGR_STALE route continuing to win best-path.
//
// RFC requirement: RFC9494-4.2-6 positive -- the helper's LLGR_STALE processing is realized by
// attachCommunityCommand raising StaleLevel to storage.DepreferenceThreshold
// (internal/component/bgp/plugins/rib/rib_commands_community.go:101), which is exactly the
// level comparePair's Step 0 treats as least preferred
// (internal/component/bgp/plugins/rib/bestpath.go:309-316).
func TestRFC9494_LLGRStaleRouteIsLeastPreferred(t *testing.T) {
	t.Parallel()
	r := setupGRTestRIB(t)

	_, _, err := r.handleCommand("request bgp rib mark-stale", "*", []string{"192.0.2.1", "120"})
	require.NoError(t, err)
	_, _, err = r.handleCommand("request bgp rib attach-community", "*",
		[]string{"192.0.2.1", "ipv4/unicast", "ffff0006"})
	require.NoError(t, err)

	entry, found := r.bgpPeers[netip.MustParseAddr("192.0.2.1")].Lookup(family.IPv4Unicast, []byte{24, 10, 0, 0})
	require.True(t, found)
	require.GreaterOrEqual(t, entry.StaleLevel, storage.DepreferenceThreshold,
		"LLGR_STALE route sits at the depreference threshold")

	llgrStale := &Candidate{
		PeerAddr: "192.0.2.1", PeerIP: netip.MustParseAddr("192.0.2.1"),
		LocalPref: 500, StaleLevel: entry.StaleLevel,
	}
	fresh := &Candidate{
		PeerAddr: "192.0.2.2", PeerIP: netip.MustParseAddr("192.0.2.2"),
		LocalPref: 100,
	}
	assert.Equal(t, 1, ComparePair(llgrStale, fresh), "the LLGR_STALE route loses despite better LOCAL_PREF")
	assert.Equal(t, "192.0.2.2", SelectBest([]*Candidate{llgrStale, fresh}).PeerAddr)
}

// TestRFC9494_RouteWithoutLLGRStaleNotLeastPreferred verifies the depreference is scoped to
// routes that actually went through LLGR_STALE processing.
//
// VALIDATES: a route that never received LLGR_STALE keeps StaleLevel 0 and competes normally.
// PREVENTS: every route being treated as least preferred while a peer is in LLGR.
//
// RFC requirement: RFC9494-4.2-6 negative -- a route that was not marked keeps StaleLevelFresh
// (attachCommunityCommand skips it, internal/component/bgp/plugins/rib/rib_commands_community.go:97),
// and comparePair's Step 0 does not fire below DepreferenceThreshold
// (internal/component/bgp/plugins/rib/bestpath.go:309-316), so it still wins on LOCAL_PREF.
func TestRFC9494_RouteWithoutLLGRStaleNotLeastPreferred(t *testing.T) {
	t.Parallel()
	r := setupGRTestRIB(t)

	// Peer 2 is never marked stale, so the LLGR sweep on peer 1 leaves it alone.
	_, _, err := r.handleCommand("request bgp rib mark-stale", "*", []string{"192.0.2.1", "120"})
	require.NoError(t, err)
	_, _, err = r.handleCommand("request bgp rib attach-community", "*",
		[]string{"192.0.2.1", "ipv4/unicast", "ffff0006"})
	require.NoError(t, err)

	entry, found := r.bgpPeers[netip.MustParseAddr("192.0.2.2")].Lookup(family.IPv4Unicast, []byte{24, 172, 16, 0})
	require.True(t, found)
	require.Equal(t, storage.StaleLevelFresh, entry.StaleLevel, "unmarked route stays fresh")

	unmarked := &Candidate{
		PeerAddr: "192.0.2.2", PeerIP: netip.MustParseAddr("192.0.2.2"),
		LocalPref: 500, StaleLevel: entry.StaleLevel,
	}
	other := &Candidate{
		PeerAddr: "192.0.2.3", PeerIP: netip.MustParseAddr("192.0.2.3"),
		LocalPref: 100,
	}
	assert.Equal(t, -1, ComparePair(unmarked, other), "an unmarked route is not least preferred")
}
