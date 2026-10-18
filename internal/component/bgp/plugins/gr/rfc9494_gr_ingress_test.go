// Design: docs/architecture/route-selection.md -- Step 0, stale depreference
// RFC: rfc/short/rfc9494.md -- Section 4.3, processing a received LLGR_STALE route
// Related: gr_ingress.go -- LLGRIngressFilter, llgrStaleFromSource

package gr

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
)

// ingressTestPeer is the neighbor every test below receives the route from.
var ingressTestPeer = netip.MustParseAddr("192.0.2.1")

// ingressTestUpdate announces 10.0.0.0/8 with a COMMUNITIES attribute holding
// the one community given.
func ingressTestUpdate(community [4]byte) []byte {
	return []byte{
		0x00, 0x00, // Withdrawn Routes length 0
		0x00, 0x15, // Total Path Attribute Length 21
		0x40, 0x01, 0x01, 0x00, // ORIGIN = IGP
		0x40, 0x02, 0x00, // AS_PATH = empty
		0x40, 0x03, 0x04, 0x0a, 0x00, 0x00, 0x01, // NEXT_HOP = 10.0.0.1
		0xc0, 0x08, 0x04, community[0], community[1], community[2], community[3], // COMMUNITIES
		0x08, 0x0a, // NLRI 10.0.0.0/8
	}
}

// ingressTestPlainUpdate announces 10.0.0.0/8 with no COMMUNITIES attribute.
func ingressTestPlainUpdate() []byte {
	return []byte{
		0x00, 0x00, // Withdrawn Routes length 0
		0x00, 0x0e, // Total Path Attribute Length 14
		0x40, 0x01, 0x01, 0x00, // ORIGIN = IGP
		0x40, 0x02, 0x00, // AS_PATH = empty
		0x40, 0x03, 0x04, 0x0a, 0x00, 0x00, 0x01, // NEXT_HOP = 10.0.0.1
		0x08, 0x0a, // NLRI 10.0.0.0/8
	}
}

var (
	ingressLLGRStale = [4]byte{0xff, 0xff, 0x00, 0x06}
	ingressNoExport  = [4]byte{0xff, 0xff, 0xff, 0x01}
)

// runIngress calls the filter the way the reactor does, for a session whose own
// OPEN carried the LLGR Capability when advertised is true, and returns the
// metadata.
func runIngress(t *testing.T, advertised bool, payload []byte) map[string]any {
	t.Helper()
	meta := make(map[string]any, 2)
	src := filterapi.PeerFilterInfo{Address: ingressTestPeer, LLGRAdvertised: advertised}
	accept, modified := LLGRIngressFilter(src, payload, meta)
	require.True(t, accept, "the step never rejects")
	require.Nil(t, modified, "the step never rewrites the payload")
	return meta
}

// TestLLGRIngressFilter_MarksReceivedStale verifies a route received carrying
// LLGR_STALE from a neighbor Ze sent the LLGR Capability to is marked stale level 2.
//
// VALIDATES: AC-13: the ingress step writes meta["stale"] = 2, which the RIB
// stores as least preferred and LLGREgressFilter acts on.
// PREVENTS: a received LLGR_STALE route entering the RIB and the forward rails
// as a fresh one.
//
// RFC requirement: RFC9494-4.3-1 positive -- a route received from 192.0.2.1, to which
// Ze sent the LLGR Capability, carrying LLGR_STALE in COMMUNITIES gets stale level 2:
// "A BGP speaker that has advertised the Long-Lived Graceful Restart Capability to a
// neighbor MUST perform the following upon receiving a route from that neighbor with
// the LLGR_STALE community".
// MUTATION: make llgrStaleFromSource return false after the community check, and this goes red.
func TestLLGRIngressFilter_MarksReceivedStale(t *testing.T) {
	meta := runIngress(t, true, ingressTestUpdate(ingressLLGRStale))
	assert.Equal(t, map[string]any{"stale": uint8(2)}, meta)
}

// TestLLGRIngressFilter_NoLLGRSentLeavesFresh verifies the same route from a
// neighbor Ze did NOT send the LLGR Capability to is left fresh.
//
// VALIDATES: AC-14: the obligation binds only a speaker that has advertised
// the capability to that neighbor.
// PREVENTS: depreferencing routes from a neighbor that never agreed to the LLGR
// procedures with Ze.
//
// RFC requirement: RFC9494-4.3-1 negative -- the same LLGR_STALE route from 192.0.2.1,
// to which Ze sent no LLGR Capability, gets no stale level.
// MUTATION: drop the LLGRAdvertised test in llgrStaleFromSource, and this goes red.
func TestLLGRIngressFilter_NoLLGRSentLeavesFresh(t *testing.T) {
	meta := runIngress(t, false, ingressTestUpdate(ingressLLGRStale))
	assert.Empty(t, meta)
}

// TestLLGRIngressFilter_NoCommunityLeavesFresh verifies a route without
// LLGR_STALE from an LLGR neighbor is left fresh, with another community and
// with no COMMUNITIES attribute at all.
//
// VALIDATES: the step is confined to LLGR_STALE, read from the attribute section.
// PREVENTS: a check that marks every route of an LLGR neighbor, or every route
// carrying COMMUNITIES.
func TestLLGRIngressFilter_NoCommunityLeavesFresh(t *testing.T) {
	assert.Empty(t, runIngress(t, true, ingressTestUpdate(ingressNoExport)), "NO_EXPORT is not LLGR_STALE")
	assert.Empty(t, runIngress(t, true, ingressTestPlainUpdate()), "no COMMUNITIES attribute")
}

// TestLLGRIngressFilter_JudgedBeforePluginState verifies a session's first
// LLGR_STALE route is marked when the GR plugin holds no state at all, as it does
// before the asynchronous sent-OPEN event reaches it.
//
// VALIDATES: the advertised-LLGR fact comes from the session (src.LLGRAdvertised),
// so the step's answer does not depend on event delivery order.
// PREVENTS: the race in which the first UPDATEs of a session are read before the
// plugin recorded the OPEN Ze sent, and compete as fresh routes.
func TestLLGRIngressFilter_JudgedBeforePluginState(t *testing.T) {
	setEgressState(nil)

	meta := runIngress(t, true, ingressTestUpdate(ingressLLGRStale))
	assert.Equal(t, map[string]any{"stale": uint8(2)}, meta)
}

// TestPayloadCarriesLLGRStale_MalformedPayload verifies a payload whose sections
// do not parse answers false rather than reading past its end.
//
// VALIDATES: the bounded read of a truncated payload.
// PREVENTS: a panic on a short UPDATE.
func TestPayloadCarriesLLGRStale_MalformedPayload(t *testing.T) {
	assert.False(t, payloadCarriesLLGRStale(nil))
	assert.False(t, payloadCarriesLLGRStale([]byte{0x00}))
	assert.False(t, payloadCarriesLLGRStale([]byte{0x00, 0x00, 0x00, 0x30, 0xc0}))
}

// BenchmarkLLGRIngressFilter measures the step on the receive path for the three
// shapes a route takes: a session that did not advertise LLGR (the flag test
// alone), an LLGR session's route without LLGR_STALE (one attribute walk), and an
// LLGR_STALE route (the walk and one metadata write).
func BenchmarkLLGRIngressFilter(b *testing.B) {
	cases := []struct {
		name       string
		advertised bool
		payload    []byte
	}{
		{"no-llgr-session", false, ingressTestUpdate(ingressLLGRStale)},
		{"llgr-session-fresh", true, ingressTestUpdate(ingressNoExport)},
		{"llgr-session-stale", true, ingressTestUpdate(ingressLLGRStale)},
	}
	for _, c := range cases {
		b.Run(c.name, func(b *testing.B) {
			src := filterapi.PeerFilterInfo{Address: ingressTestPeer, LLGRAdvertised: c.advertised}
			meta := make(map[string]any, 2)
			b.ReportAllocs()
			for b.Loop() {
				LLGRIngressFilter(src, c.payload, meta)
			}
		})
	}
}
