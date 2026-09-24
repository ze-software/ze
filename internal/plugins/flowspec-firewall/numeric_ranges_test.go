package flowspecfirewall

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/plugins/nlri/flowspec"
	"github.com/ze-software/ze/internal/component/firewall"
)

func TestReceivedPortOperatorsKeepPacketPredicate(t *testing.T) {
	// Destination 10.0.0.0/24, (port > 80 AND port < 90) OR port == 443.
	// Leading AND and the reserved numeric bit are intentionally set.
	fs, err := flowspec.ParseFlowSpec(flowspec.IPv4FlowSpec, []byte{13, 1, 24, 10, 0, 0, 5, 0x4a, 80, 0x44, 90, 0x91, 1, 187})
	require.NoError(t, err)
	terms, err := translateFlowSpec(fs, flowAction{discard: true}, "operators")
	require.NoError(t, err)
	require.Len(t, terms, 2, "port-only NLRI applies to TCP and UDP, never another transport")
	protocols := make([]string, 0, len(terms))
	for _, term := range terms {
		var ranges []firewall.PortRange
		for _, match := range term.Matches {
			switch m := match.(type) {
			case firewall.MatchDestinationPort:
				ranges = m.Ranges
			case firewall.MatchProtocol:
				protocols = append(protocols, m.Protocol)
			}
		}
		require.Equal(t, []firewall.PortRange{{Lo: 81, Hi: 89}, {Lo: 443, Hi: 443}}, ranges)
	}
	require.ElementsMatch(t, []string{"tcp", "udp"}, protocols)
}

func TestImpossiblePortAndProtocolCannotWidenMatch(t *testing.T) {
	for _, wire := range [][]byte{
		// SCTP cannot satisfy a TCP-or-UDP port component.
		{11, 1, 24, 10, 0, 0, 3, 0x81, 132, 5, 0x81, 80},
		// No one-octet protocol equals 256; truncation would match zero.
		{9, 1, 24, 10, 0, 0, 3, 0x91, 1, 0},
		// No port is greater than 65535; truncating the operand would widen it.
		{11, 1, 24, 10, 0, 0, 5, 0xa2, 0, 1, 0, 0},
	} {
		fs, err := flowspec.ParseFlowSpec(flowspec.IPv4FlowSpec, wire)
		require.NoError(t, err)
		terms, err := translateFlowSpec(fs, flowAction{discard: true}, "impossible")
		require.Error(t, err)
		require.Nil(t, terms)
	}
}
