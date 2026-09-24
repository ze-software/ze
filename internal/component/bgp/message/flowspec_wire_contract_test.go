package message_test

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/plugins/nlri/flowspec"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
)

// RFC requirement: RFC8955-4-3 positive -- the real UPDATE writer emits a zero-length FlowSpec next hop (§4).
// RFC requirement: RFC8955-4-3 negative -- configured IPv4 and IPv6 next hops cannot override the family-specific empty field (§4).
// RFC requirement: RFC5575-4-8 positive -- an advertised FlowSpec UPDATE carries a zero-length MP_REACH next hop (§4).
// RFC requirement: RFC5575-4-8 negative -- a supplied IPv4 or IPv6 forwarding address cannot appear in that field (§4).
func TestFlowSpecUpdateIgnoresConfiguredNextHop(t *testing.T) {
	for _, address := range []string{"192.0.2.9", "2001:db8::9"} {
		t.Run(address, func(t *testing.T) {
			builder := message.NewUpdateBuilder(65001, false, true, false)
			nlri := []byte{5, 1, 24, 10, 0, 0}
			update := builder.BuildFlowSpec(message.FlowSpecParams{NLRI: nlri, NextHop: netip.MustParseAddr(address)})
			wire := make([]byte, update.Len(nil))
			update.WriteTo(wire, 0, nil)
			reach, err := wireu.NewWireUpdate(wire[19:], 0).MPReach()
			require.NoError(t, err)
			require.Equal(t, byte(0), reach[3])
			require.Equal(t, nlri, reach.NLRIBytes(), "next-hop octets must not displace the flow NLRI")
			require.False(t, reach.NextHop().IsValid())
		})
	}
}

// RFC requirement: RFC5575-8-1 positive -- the VPN FlowSpec UPDATE length counts its RD and components (§8).
// RFC requirement: RFC5575-8-1 negative -- a VPN NLRI whose declared length omits the eight RD octets is rejected (§8).
func TestFlowSpecVPNUpdateCountsRouteDistinguisher(t *testing.T) {
	builder := message.NewUpdateBuilder(65001, false, true, false)
	rd := [8]byte{0, 0, 0xfd, 0xe9, 0, 0, 0, 1}
	update := builder.BuildFlowSpec(message.FlowSpecParams{RD: rd, NLRI: []byte{1, 24, 10, 0, 0}})
	wire := make([]byte, update.Len(nil))
	update.WriteTo(wire, 0, nil)
	reach, err := wireu.NewWireUpdate(wire[19:], 0).MPReach()
	require.NoError(t, err)
	require.Equal(t, []byte{13, 0, 0, 0xfd, 0xe9, 0, 0, 0, 1, 1, 24, 10, 0, 0}, reach.NLRIBytes())
	require.Equal(t, uint8(134), reach.SAFI())
	parsed, err := flowspec.ParseFlowSpecVPN(flowspec.IPv4FlowSpecVPN, reach.NLRIBytes())
	require.NoError(t, err)
	require.Equal(t, []byte{5, 1, 24, 10, 0, 0}, parsed.FlowSpec().Bytes())
	bad := append([]byte(nil), reach.NLRIBytes()...)
	bad[0] -= 8
	_, err = flowspec.ParseFlowSpecVPN(flowspec.IPv4FlowSpecVPN, bad)
	require.Error(t, err)
}
