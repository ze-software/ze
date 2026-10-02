// Design: docs/guide/graceful-restart.md -- LLGR entry sweeps NO_LLGR routes
// Related: rib_commands_community.go -- deleteWithCommunityCommand, the sweep the LLGR entry dispatches

package rib

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/storage"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
)

// noLLGRSweepRIB holds one stale route for 10.0.0.0/24 from 192.0.2.1, already
// selected as the best path (its Add best-change emitted), with or without the
// NO_LLGR community ffff0007. It returns the manager and the bus after the
// setup events, so a test reads only what the sweep emits.
func noLLGRSweepRIB(t *testing.T, withNoLLGR bool) (*RIBManager, *testEventBus) {
	t.Helper()
	bus := newTestEventBus()
	r := newTestRIBManagerWithBus(bus)
	peerAddr := netip.MustParseAddr("192.0.2.1")
	r.peerMeta[peerAddr] = &peerMetadata{PeerASN: 65001, LocalASN: 65000}

	attrs := concatBytes(testWireOriginIGP, testWireASPath65001, testWireNextHop)
	// The route without NO_LLGR still carries a community, so the sweep's
	// community match is reached and must reject it, not skip it.
	community := []byte{0xC0, 0x08, 0x04, 0xFD, 0xE9, 0x00, 0x64} // COMMUNITIES 65001:100
	if withNoLLGR {
		community = testWireCommunityB
	}
	attrs = concatBytes(attrs, community)
	prefix := ipv4Prefix(24, 10, 0, 0)
	r.bgpPeers[peerAddr] = storage.NewPeerRIB(peerAddr.String())
	r.bgpPeers[peerAddr].Insert(family.IPv4Unicast, attrs, prefix)
	change, ok := r.checkBestPathChange(family.IPv4Unicast, prefix, false, nil)
	require.True(t, ok, "setup: the route becomes the best path")
	publishBestChanges([]bestChangeEntry{change}, family.IPv4Unicast)

	_, _, err := r.handleCommand("request bgp rib mark-stale", "*", []string{"192.0.2.1", "120"})
	require.NoError(t, err, "setup: the session's routes are marked stale")
	return r, bus
}

// sweepBestChange returns the one (bgp-rib, best-change) batch emitted after
// the first `before` events. publishBestChanges also emits the redistribution
// event, which is not counted here.
func sweepBestChange(t *testing.T, bus *testEventBus, before int) *bestChangeBatch {
	t.Helper()
	bus.mu.Lock()
	emitted := append([]testEvent{}, bus.events[before:]...)
	bus.mu.Unlock()
	var found []*bestChangeBatch
	for _, event := range emitted {
		if event.Namespace != "bgp-rib" || event.EventType != ribevents.EventBestChange {
			continue
		}
		batch, ok := event.Payload.(*bestChangeBatch)
		require.True(t, ok, "best-change payload is %T", event.Payload)
		found = append(found, batch)
	}
	require.Len(t, found, 1, "the sweep publishes one best-change batch")
	return found[0]
}

// VALIDATES: RFC 9494 Section 4.2, "they MUST NOT be retained and MUST be
// removed as per the normal operation of [RFC4271]". The sweep the LLGR entry
// dispatches removes the stale NO_LLGR route AND runs the normal removal: the
// best path for 10.0.0.0/24 is recomputed and, with no other path, a
// best-change Withdraw for that prefix is published, which is what the
// forwarding plane and the export side act on.
// PREVENTS: a sweep that drops the route from the peer RIB while the Loc-RIB,
// the FIB and the peers Ze advertised it to keep the old best path.
//
// RFC requirement: RFC9494-4.2-5 positive -- after delete-with-community ffff0007 removes the stale NO_LLGR route, exactly one best-change event follows, a Withdraw for 10.0.0.0/24 in ipv4/unicast.
func TestRFC9494NoLLGRSweepWithdrawsTheBestPath(t *testing.T) {
	r, bus := noLLGRSweepRIB(t, true)
	before := bus.eventCount()

	_, _, err := r.handleCommand("request bgp rib delete-with-community", "*", []string{"192.0.2.1", "ipv4/unicast", "ffff0007"})
	require.NoError(t, err)

	batch := sweepBestChange(t, bus, before)
	assert.Equal(t, family.IPv4Unicast, batch.Family)
	require.Len(t, batch.Changes, 1)
	assert.Equal(t, ribevents.BestChangeWithdraw, batch.Changes[0].Action, "the removed best path is withdrawn")
	assert.Equal(t, netip.MustParsePrefix("10.0.0.0/24"), batch.Changes[0].Prefix)
}

// VALIDATES: the same sweep leaves a stale route WITHOUT NO_LLGR in place and
// publishes no best-change, so the withdrawal above is caused by the community
// and not by the sweep running at all.
// PREVENTS: a sweep that withdraws every stale route on LLGR entry.
//
// RFC requirement: RFC9494-4.2-5 negative -- a stale route without NO_LLGR survives delete-with-community ffff0007 with no best-change published.
func TestRFC9494SweepKeepsTheBestPathWithoutNoLLGR(t *testing.T) {
	r, bus := noLLGRSweepRIB(t, false)
	before := bus.eventCount()

	_, _, err := r.handleCommand("request bgp rib delete-with-community", "*", []string{"192.0.2.1", "ipv4/unicast", "ffff0007"})
	require.NoError(t, err)

	assert.Equal(t, before, bus.eventCount(), "no best-change for a route that keeps its place")
	assert.Equal(t, 1, r.bgpPeers[netip.MustParseAddr("192.0.2.1")].Len(), "the route is retained")
}
