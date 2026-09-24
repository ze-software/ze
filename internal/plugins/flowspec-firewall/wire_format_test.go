package flowspecfirewall

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/plugins/nlri/flowspec"
	"github.com/ze-software/ze/internal/component/firewall"
	"github.com/ze-software/ze/internal/core/family"
)

// No narrowing component may disappear between the native NLRI codec and the firewall.
func TestNativeWirePreservesEveryMatch(t *testing.T) {
	fam := flowFamily()
	data := flowWire(t, fam,
		flowspec.NewFlowDestPrefixComponent(netip.MustParsePrefix("10.1.0.0/24")),
		flowspec.NewFlowSourcePrefixComponent(netip.MustParsePrefix("192.0.2.0/24")),
		flowspec.NewFlowIPProtocolComponent(6), flowspec.NewFlowDestPortComponent(80))
	fs, err := flowspec.ParseFlowSpec(fam, data)
	require.NoError(t, err)
	terms, err := translateFlowSpec(fs, flowAction{discard: true}, "wire-key")
	require.NoError(t, err)
	require.Len(t, terms, 1)
	assert.ElementsMatch(t, []firewall.Match{
		firewall.MatchDestinationAddress{Prefix: netip.MustParsePrefix("10.1.0.0/24")},
		firewall.MatchSourceAddress{Prefix: netip.MustParsePrefix("192.0.2.0/24")},
		firewall.MatchProtocol{Protocol: "tcp"},
		firewall.MatchDestinationPort{Ranges: []firewall.PortRange{{Lo: 80, Hi: 80}}},
	}, terms[0].Matches)
}

func parseAndTranslate(fam family.Family, data []byte) error {
	fs, err := flowspec.ParseFlowSpec(fam, data)
	if err != nil {
		return err
	}
	_, err = translateFlowSpec(fs, flowAction{discard: true}, "wire-key")
	return err
}

func TestNativeWireProtocolsTranslateOrRefuseWithoutWidening(t *testing.T) {
	for _, num := range []uint8{1, 2, 6, 17, 47, 50, 51, 58, 89, 112, 132, 253} {
		data := flowWire(t, flowFamily(), flowspec.NewFlowIPProtocolComponent(num))
		err := parseAndTranslate(flowFamily(), data)
		_, canonical := firewall.ProtocolName(num)
		if canonical {
			require.NoError(t, err, "protocol %d", num)
		} else {
			require.ErrorIs(t, err, errUnknownProtocol, "protocol %d", num)
		}
	}
	// A protocol inequality is not silently reduced to equality.
	require.Error(t, parseAndTranslate(flowFamily(), []byte{3, 3, 0x82, 6}))
}

func TestNativeWireTCPFlagsRemainACondition(t *testing.T) {
	fs, err := flowspec.ParseFlowSpec(flowFamily(), flowWire(t, flowFamily(), flowspec.NewFlowTCPFlagsComponent(2)))
	require.NoError(t, err)
	terms, err := translateFlowSpec(fs, flowAction{discard: true}, "syn")
	require.NoError(t, err)
	require.Len(t, terms, 1)
	assert.ElementsMatch(t, []firewall.Match{
		firewall.MatchProtocol{Protocol: "tcp"}, firewall.MatchTCPFlags{Flags: 2, Mask: 2},
	}, terms[0].Matches)
}

func TestNativeIPv6SourceOffsetCannotLoseItsCondition(t *testing.T) {
	fam := family.Family{AFI: family.AFIIPv6, SAFI: family.SAFIFlowSpec}
	// Destination 2001:db8::/32; source ::1:0:0:0:0/32-64, four pattern octets.
	fs, err := flowspec.ParseFlowSpec(fam, []byte{14, 1, 32, 0, 0x20, 1, 0x0d, 0xb8, 2, 64, 32, 0, 1, 0, 0})
	require.NoError(t, err)
	terms, err := translateFlowSpec(fs, flowAction{discard: true}, "offset")
	require.Error(t, err)
	assert.Empty(t, terms)
}
