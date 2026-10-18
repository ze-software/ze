// Design: docs/architecture/meta/README.md -- the rebuilt key on a relayed route
// RFC: rfc/short/rfc9494.md -- Section 4.3, a received LLGR_STALE route on the relay rail
// Related: reactor_api_relay.go -- buildRelayUpdate and resolveRelaySource, the producers under test

package reactor

import (
	"net/netip"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// relaySeen is what one egress step observed for one relayed route.
type relaySeen struct {
	src  filterapi.PeerFilterInfo
	meta map[string]any
}

// relayObserveEgress relays one stored route from 10.0.0.1 to 10.0.0.2, whose
// source session's own OPEN carried the LLGR Capability when advertised is
// true, and returns what an in-process egress step saw.
func relayObserveEgress(t *testing.T, advertised bool) relaySeen {
	t.Helper()
	api, _, _, _, _ := relayFixture(t)
	source, found := api.r.findPeerByAddr(netip.MustParseAddr("10.0.0.1"))
	require.True(t, found)
	session := NewSession(source.settings)
	session.localLLGR.Store(advertised)
	source.mu.Lock()
	source.session = session
	source.mu.Unlock()

	var mu sync.Mutex
	var seen []relaySeen
	observe := func(src, _ filterapi.PeerFilterInfo, _ []byte, meta map[string]any, _ *filterapi.ModAccumulator) bool {
		mu.Lock()
		seen = append(seen, relaySeen{src: src, meta: meta})
		mu.Unlock()
		return true
	}
	// egressFilters is what makes resolveRelaySource fill the source's filter
	// facts, as it is in a daemon with any in-process egress filter loaded.
	api.r.egressFilters = []filterapi.EgressFilterFunc{observe}
	api.r.orderedEgressSteps = []orderedEgressStep{{name: "observe", inproc: observe}}

	require.NoError(t, api.RelayStoredRoute(netip.MustParseAddr("10.0.0.2"),
		[]rpc.StoredRoute{storedIPv4Route("10.0.0.1")}, plugin.OperatorSender()))
	mu.Lock()
	defer mu.Unlock()
	require.Len(t, seen, 1, "the egress step ran once, for the one destination")
	return seen[0]
}

// TestBuildRelayUpdateMarksRebuilt verifies the relay rail hands the egress
// filters the two facts LLGREgressFilter needs to recover a received
// LLGR_STALE route rebuilt from storage (AC-16 rail 3). Method: relay a stored
// route through RelayStoredRoute and read what an egress step receives, for a
// source session whose OPEN carried LLGR and one whose OPEN did not.
//
// VALIDATES: every rebuilt route carries filterapi.MetaKeyRebuilt, and the
// source's LLGRAdvertised follows that source session's own OPEN.
// PREVENTS: a bgp-adj-rib-in replay reaching the GR egress filter as a fresh
// live route, which it then sends to a neighbor without LLGR.
//
// RFC requirement: RFC9494-4.3-3 positive -- a stored route relayed through RelayStoredRoute
// reaches the egress filters with meta "rebuilt" true and, when the source session's own OPEN carried
// the LLGR Capability, src.LLGRAdvertised true, the two inputs LLGREgressFilter recovers the
// LLGR_STALE fact from.
// RFC requirement: RFC9494-4.3-3 negative -- the same relay from a source session whose OPEN carried
// no LLGR Capability reaches the egress filters with src.LLGRAdvertised false, so the recovery
// leaves the route fresh.
func TestBuildRelayUpdateMarksRebuilt(t *testing.T) {
	llgr := relayObserveEgress(t, true)
	rebuilt, _ := llgr.meta[filterapi.MetaKeyRebuilt].(bool)
	assert.True(t, rebuilt, "a relayed route says it was rebuilt from storage")
	assert.True(t, llgr.src.LLGRAdvertised, "the source session's OPEN carried LLGR")
	assert.Equal(t, netip.MustParseAddr("10.0.0.1"), llgr.src.Address)

	plain := relayObserveEgress(t, false)
	rebuilt, _ = plain.meta[filterapi.MetaKeyRebuilt].(bool)
	assert.True(t, rebuilt, "the key does not depend on the source's capabilities")
	assert.False(t, plain.src.LLGRAdvertised, "the source session's OPEN carried no LLGR")
}
