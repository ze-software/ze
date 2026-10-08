package rsvpte

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	mplsfibevents "github.com/ze-software/ze/internal/core/mplsfib"
)

// floorPath builds a PATH from the ingress whose ADSPEC carries mtu. Toward
// the egress it carries no ERO, so the explicit route cannot refuse it first.
func floorPath(t *testing.T, mtu uint32, toEgress bool) []byte {
	t.Helper()
	psb := samplePSB()
	if toEgress {
		psb.ERO = nil
	}
	var adspec [adspecSize]byte
	require.Equal(t, adspecSize, encodeAdspec(adspec[:], mtu, serviceControlledLoad))
	psb.Adspec = adspec[:]
	return buildPath(psb, netip.MustParseAddr("10.0.0.1"), defaultIPTTL)
}

// requireMTUPathErr asserts one PathErr toward the ingress with RFC 2205 Error
// Code 21, sub-code 05 (Bad Adspec value).
func requireMTUPathErr(t *testing.T, ft *fakeTransport) {
	t.Helper()
	perr, dst, ok := ft.lastByType(MsgTypePathErr)
	require.True(t, ok, "a PATH under the MTU floor is answered with a PathErr")
	assert.Equal(t, netip.MustParseAddr("10.0.0.1"), dst, "the PathErr goes back toward the sender")
	require.True(t, perr.HasErrorSpec)
	assert.Equal(t, ErrCodeTrafficControlError, perr.ErrorSpec.ErrorCode, "Error Code 21 (Traffic Control Error)")
	assert.Equal(t, ErrValueBadAdspec, perr.ErrorSpec.ErrorValue, "ss=00, sub-code 05 (Bad Adspec value)")
}

// TestTransitRefusesPathMTUBelowFloor drives a PATH through a transit with the
// received ADSPEC and the outgoing link each at and below the floor, and reads
// what the transit forwards and what it answers.
// PREVENTS: a peer's tiny ADSPEC MTU, or a tiny local link, being composed
// onward and later installed as a push metric that livelocks a stock kernel.
func TestTransitRefusesPathMTUBelowFloor(t *testing.T) {
	floor := mplsfibevents.PathMTUMinimum
	target := netip.MustParsePrefix("10.0.0.9/32")
	for _, tc := range []struct {
		name           string
		arrived, route uint32
		refused        bool
	}{
		{name: "the received ADSPEC is one under the floor", arrived: floor - 1, route: 1500, refused: true},
		{name: "the outgoing link is one under the floor", arrived: 1500, route: floor - 1, refused: true},
		{name: "the composed MTU is exactly the floor", arrived: floor, route: 1500, refused: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e, ft, _ := testEngine(t, "10.0.0.5", nil)
			ft.routes = map[netip.Prefix]RouteInfo{target: {NextHop: target.Addr(), Lookup: target.Addr(), MTU: tc.route}}

			e.handlePacket(Packet{Src: netip.MustParseAddr("10.0.0.1"), Payload: floorPath(t, tc.arrived, false)})

			path, _, forwarded := ft.lastByType(MsgTypePath)
			if tc.refused {
				assert.False(t, forwarded, "a PATH under the floor is not forwarded")
				requireMTUPathErr(t, ft)
				_, found := e.table.Get(lspKey{
					TunnelEndpoint: target.Addr(), TunnelID: samplePSB().Session.TunnelID,
					ExtTunnelID: samplePSB().Session.ExtTunnelID, SenderAddr: netip.MustParseAddr("10.0.0.1"),
					LSPID: samplePSB().SenderTemplate.LSPID,
				})
				assert.False(t, found, "no path state is kept for a refused PATH")
				return
			}
			require.True(t, forwarded, "a PATH at the floor is forwarded")
			assert.Equal(t, floor, path.PathMTU, "the forwarded ADSPEC carries the floor")
			assert.Zero(t, ft.countByType(MsgTypePathErr), "a PATH at the floor draws no PathErr")
		})
	}
}

// TestEgressRefusesPathMTUBelowFloor delivers a PATH to the egress with an
// ADSPEC one under the floor and at it, and reads the pop and the RESV.
// PREVENTS: the egress echoing a tiny MTU as its RESV M, which carries it
// upstream to every push and swap on the LSP.
func TestEgressRefusesPathMTUBelowFloor(t *testing.T) {
	floor := mplsfibevents.PathMTUMinimum
	for _, tc := range []struct {
		name    string
		arrived uint32
		refused bool
	}{
		{name: "one under the floor", arrived: floor - 1, refused: true},
		{name: "exactly the floor", arrived: floor, refused: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e, ft, fib := testEngine(t, "10.0.0.9", nil)

			e.handlePacket(Packet{Src: netip.MustParseAddr("10.0.0.1"), Payload: floorPath(t, tc.arrived, true)})

			resv, _, replied := ft.lastByType(MsgTypeResv)
			if tc.refused {
				assert.False(t, replied, "the egress sends no RESV for a PATH under the floor")
				assert.Empty(t, fib.popped, "the egress installs no pop for a PATH under the floor")
				requireMTUPathErr(t, ft)
				return
			}
			require.True(t, replied, "the egress answers a PATH at the floor")
			require.Len(t, resv.FlowDescriptors, 1)
			assert.Equal(t, floor, resv.FlowDescriptors[0].FlowSpec.MaxPacketSize, "the RESV M is the floor")
			require.Len(t, fib.pathMTUs, 1)
			assert.Equal(t, floor, fib.pathMTUs[0], "the pop carries the floor")
		})
	}
}

// TestIngressRefusesResvMTUBelowFloor returns a RESV to an ingress whose M is
// one under the floor and at it, and reads the push and the ResvErr.
// PREVENTS: a downstream peer's tiny FLOWSPEC M reaching the push route's
// RTAX_MTU, the input that livelocks a stock kernel's IPv4 fragmentation.
func TestIngressRefusesResvMTUBelowFloor(t *testing.T) {
	floor := mplsfibevents.PathMTUMinimum
	endpoint := netip.MustParseAddr("10.0.0.9")
	for _, tc := range []struct {
		name     string
		received uint32
		refused  bool
	}{
		{name: "one under the floor", received: floor - 1, refused: true},
		{name: "exactly the floor", received: floor, refused: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e, ft, fib := testEngine(t, "10.0.0.1", nil)
			prefix := netip.PrefixFrom(endpoint, 32)
			ft.routes = map[netip.Prefix]RouteInfo{prefix: {NextHop: endpoint, Lookup: endpoint, MTU: 1500}}
			key := lspKey{
				TunnelEndpoint: endpoint, TunnelID: 1,
				ExtTunnelID: 0x0a000001, SenderAddr: netip.MustParseAddr("10.0.0.1"), LSPID: 1,
			}
			setupTunnel(e.log, e.table, tunnelConfig{Destination: endpoint, TunnelID: key.TunnelID}, e.cfg(), e)
			sent, _, ok := ft.lastByType(MsgTypePath)
			require.True(t, ok, "ingress signaled the PATH")
			require.Equal(t, uint32(1500), sent.PathMTU, "ingress advertised its link MTU")

			rsb := &resvStateBlock{
				Session:  sessionIPv4{TunnelEndpoint: endpoint, TunnelID: key.TunnelID, ExtTunnelID: key.ExtTunnelID},
				Label:    labelObject{Label: 16050},
				Style:    StyleSharedExplicit,
				FlowSpec: FlowSpec{Service: serviceControlledLoad, MaxPacketSize: tc.received},
			}
			filter := senderTemplateIPv4{SenderAddr: key.SenderAddr, LSPID: key.LSPID}
			e.handlePacket(Packet{Src: endpoint, Payload: buildResv(rsb, filter, DefaultRefreshPeriod, endpoint)})

			if tc.refused {
				assert.Empty(t, fib.pushed, "no push is installed for a RESV under the floor")
				resvErr, dst, ok := ft.lastByType(MsgTypeResvErr)
				require.True(t, ok, "a RESV under the floor is answered with a ResvErr")
				assert.Equal(t, endpoint, dst, "the ResvErr goes back toward the receiver")
				require.True(t, resvErr.HasErrorSpec)
				assert.Equal(t, ErrCodeTrafficControlError, resvErr.ErrorSpec.ErrorCode, "Error Code 21 (Traffic Control Error)")
				assert.Equal(t, ErrValueBadFlowspec, resvErr.ErrorSpec.ErrorValue, "ss=00, sub-code 03 (Bad Flowspec value)")
				got, _ := e.table.Get(key)
				assert.NotEqual(t, LSPStateUp, got.State, "the LSP does not come up")
				return
			}
			require.Len(t, fib.pushed, 1, "a RESV at the floor installs the push")
			require.Len(t, fib.pathMTUs, 1)
			assert.Equal(t, floor, fib.pathMTUs[0], "the push carries the floor")
			assert.Zero(t, ft.countByType(MsgTypeResvErr), "a RESV at the floor draws no ResvErr")
		})
	}
}

// TestIngressRefusesToOriginateBelowFloor sets up a tunnel whose outgoing link
// MTU is one under the floor and at it, and reads what the ingress signals.
// PREVENTS: an ingress advertising a path MTU in its ADSPEC that every
// downstream node refuses, on every refresh, instead of refusing it locally.
func TestIngressRefusesToOriginateBelowFloor(t *testing.T) {
	floor := mplsfibevents.PathMTUMinimum
	endpoint := netip.MustParseAddr("10.0.0.9")
	for _, tc := range []struct {
		name    string
		link    uint32
		refused bool
	}{
		{name: "the link is one under the floor", link: floor - 1, refused: true},
		{name: "the link is exactly the floor", link: floor, refused: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e, ft, _ := testEngine(t, "10.0.0.1", nil)
			prefix := netip.PrefixFrom(endpoint, 32)
			ft.routes = map[netip.Prefix]RouteInfo{prefix: {NextHop: endpoint, Lookup: endpoint, MTU: tc.link}}
			key := lspKey{
				TunnelEndpoint: endpoint, TunnelID: 1,
				ExtTunnelID: 0x0a000001, SenderAddr: netip.MustParseAddr("10.0.0.1"), LSPID: 1,
			}
			setupTunnel(e.log, e.table, tunnelConfig{Destination: endpoint, TunnelID: key.TunnelID}, e.cfg(), e)

			sent, _, signaled := ft.lastByType(MsgTypePath)
			lsp, found := e.table.Get(key)
			require.True(t, found, "the tunnel's LSP exists")
			err := e.sendPath(lsp)
			if tc.refused {
				assert.False(t, signaled, "no PATH leaves an ingress whose link is under the floor")
				require.ErrorIs(t, err, errPathMTUBelowFloor, "a refresh is refused too")
				return
			}
			require.True(t, signaled, "an ingress whose link is at the floor signals the PATH")
			assert.Equal(t, floor, sent.PathMTU, "the ADSPEC carries the link MTU")
			require.NoError(t, err)
		})
	}
}
