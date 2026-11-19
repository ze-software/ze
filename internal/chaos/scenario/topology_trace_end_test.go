// Design: docs/architecture/chaos-web-dashboard.md — how a trace of the simulated network ends
// Related: topology.go — topologyModel.trace and traceEnd

package scenario

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTraceEndsOriginInsideOnAnEmptyASPath walks one router whose chosen route
// is given by hand, once for each pair of next hop and AS path a route can
// hold.
//
// VALIDATES: an empty AS path ends the trace origin-inside whatever the next
// hop is, as the looking glass ends it; with an AS path, the next hop decides
// between the router's own address and an address outside the AS.
// PREVENTS: a route with an empty AS path and a next hop outside the AS ending
// exit-external-address in the model and origin-inside in the looking glass,
// so the comparison of the two reports a difference neither side has.
//
// Method: NewTopology builds no route with an empty AS path and an external
// next hop, so the test hands topologyModel.trace the selection directly.
func TestTraceEndsOriginInsideOnAnEmptyASPath(t *testing.T) {
	loopback := netip.MustParseAddr("198.51.100.1")
	outside := netip.MustParseAddr("203.0.113.9")
	prefix := netip.MustParsePrefix("198.18.0.0/24")
	model, err := newTopologyModel(TopologySpec{LocalAS: 65000, Routers: []TopologyRouter{{Loopback: loopback}}})
	require.NoError(t, err)

	tests := []struct {
		name    string
		nextHop netip.Addr
		asPath  []uint32
		want    TraceEnd
	}{
		{name: "empty AS path, next hop outside the AS", nextHop: outside, want: TraceEndOriginInside},
		{name: "empty AS path, the router's own address", nextHop: loopback, want: TraceEndOriginInside},
		{name: "AS path, next hop outside the AS", nextHop: outside, asPath: []uint32{64496}, want: TraceEndExitExternalAddress},
		{name: "AS path, the router's own address", nextHop: loopback, asPath: []uint32{64496}, want: TraceEndExitOwnAddress},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chosen := []selection{{
				route:  modelRoute{asPath: tt.asPath, nextHop: tt.nextHop, source: sourceOriginated, from: loopback},
				reason: ChoiceReasonOnlyRoute,
				found:  true,
			}}
			trace, err := model.trace(0, prefix, chosen)
			require.NoError(t, err)
			assert.Equal(t, tt.want, trace.End)
			assert.Equal(t, []netip.Addr{loopback}, trace.Routers)
			assert.Equal(t, tt.nextHop, trace.NextHop, "the trace states the next hop the route carries")
		})
	}
}
