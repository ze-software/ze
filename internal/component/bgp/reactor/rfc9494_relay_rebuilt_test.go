// Design: docs/architecture/meta/README.md -- the rebuilt key on a relayed route
// RFC: rfc/short/rfc9494.md -- Section 4.3, a received LLGR_STALE route on the relay rail
// Related: reactor_api_relay.go -- buildRelayUpdate and resolveRelaySource, the producers under test

package reactor

import (
	"encoding/json"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// relayObserveEgress relays one stored route from 10.0.0.1 to 10.0.0.2, whose
// source session's own OPEN carried the LLGR Capability when advertised is
// true, and returns the route fields the destination's export filter was
// sent: an external member of a library chain (probeExportChain) that
// declares it reads rebuilt, source-llgr-advertised and source-peer-address,
// the inputs LLGREgressFilter recovers the LLGR_STALE fact from.
func relayObserveEgress(t *testing.T, advertised bool) map[string]any {
	t.Helper()
	api, _, _, _, _ := relayFixture(t)
	source, found := api.r.findPeerByAddr(netip.MustParseAddr("10.0.0.1"))
	require.True(t, found)
	session := NewSession(source.settings)
	session.localLLGR.Store(advertised)
	source.mu.Lock()
	source.session = session
	source.mu.Unlock()

	destination, found := api.r.findPeerByAddr(netip.MustParseAddr("10.0.0.2"))
	require.True(t, found)
	transport := &externalTransport{
		reads: []string{filterapi.MetaKeyRebuilt, "source-llgr-advertised", "source-peer-address"},
		reply: rpc.FilterUpdateOutput{Action: rpc.FilterAccept},
	}
	api.r.filterTransportSeam = transport
	api.r.orderedEgressSteps = buildOrderedEgressSteps()
	destination.settings.ExportPolicy = probeExportChain(t)
	destination.fwdFacts.Store(destination.buildForwardFacts())

	require.NoError(t, api.RelayStoredRoute(netip.MustParseAddr("10.0.0.2"),
		[]rpc.StoredRoute{storedIPv4Route("10.0.0.1")}, plugin.OperatorSender()))
	require.Len(t, transport.sent, 1, "the export filter was asked once, for the one destination")
	var fields map[string]any
	require.NoError(t, json.Unmarshal(transport.sent[0].Fields, &fields), "fields=%s", transport.sent[0].Fields)
	return fields
}

// TestBuildRelayUpdateMarksRebuilt verifies the relay rail hands the export
// filters the two facts LLGREgressFilter needs to recover a received
// LLGR_STALE route rebuilt from storage (AC-16 rail 3). Method: relay a stored
// route through RelayStoredRoute and read the route fields the destination's
// export filter, an external library-chain member, is sent, for a source
// session whose OPEN carried LLGR and one whose OPEN did not.
//
// VALIDATES: every rebuilt route carries rebuilt (filterapi.MetaKeyRebuilt),
// and the source's source-llgr-advertised follows that source session's own
// OPEN.
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
	assert.Equal(t, true, llgr[filterapi.MetaKeyRebuilt], "a relayed route says it was rebuilt from storage")
	assert.Equal(t, true, llgr["source-llgr-advertised"], "the source session's OPEN carried LLGR")
	assert.Equal(t, "10.0.0.1", llgr["source-peer-address"])

	plain := relayObserveEgress(t, false)
	assert.Equal(t, true, plain[filterapi.MetaKeyRebuilt], "the key does not depend on the source's capabilities")
	assert.Equal(t, false, plain["source-llgr-advertised"], "the source session's OPEN carried no LLGR")
}
