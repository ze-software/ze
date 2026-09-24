package reactor

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/family"
)

func TestFlowSpecUpdateExposesDestinationToPrefixPolicy(t *testing.T) {
	family.RegisterTestFamilies()
	builder := message.NewUpdateBuilder(65001, false, true, false)
	for _, tc := range []struct {
		name string
		nlri []byte
		want string
	}{
		{"destination", []byte{8, 1, 24, 10, 1, 0, 5, 0x81, 80}, "nlri ipv4/flow add 10.1.0.0/24"},
		{"no-destination", []byte{3, 5, 0x81, 80}, "nlri ipv4/flow add invalid"},
		{"truncated", []byte{5, 1, 24, 10}, "nlri ipv4/flow add invalid"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			update := builder.BuildFlowSpec(message.FlowSpecParams{NLRI: tc.nlri})
			wire := make([]byte, update.Len(nil))
			update.WriteTo(wire, 0, nil)
			subject := AppendUpdateForFilter(nil, nil, wireu.NewWireUpdate(wire[19:], 0), nil)
			require.Equal(t, tc.want, string(subject))
		})
	}
}

func TestFlowSpecPrefixPolicyUsesNativeAddPathFraming(t *testing.T) {
	family.RegisterTestFamilies()
	for _, tc := range []struct {
		name   string
		family family.Family
		nlri   []byte
		want   string
	}{
		{"ipv4-extended", family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpec}, []byte{0xf0, 5, 1, 24, 10, 1, 0}, "nlri ipv4/flow add 10.1.0.0/24"},
		{"vpn4", family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpecVPN}, []byte{13, 0, 0, 0xfd, 0xe9, 0, 0, 0, 1, 1, 24, 10, 1, 0}, "nlri ipv4/flow-vpn add 10.1.0.0/24"},
		{"ipv6", family.Family{AFI: family.AFIIPv6, SAFI: family.SAFIFlowSpec}, []byte{7, 1, 32, 0, 0x20, 1, 0x0d, 0xb8}, "nlri ipv6/flow add 2001:db8::/32"},
		{"ipv6-offset", family.Family{AFI: family.AFIIPv6, SAFI: family.SAFIFlowSpec}, []byte{5, 1, 32, 16, 0x0d, 0xb8}, "nlri ipv6/flow add invalid"},
		{"truncated-destination", family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpec}, []byte{4, 1, 24, 10, 1}, "nlri ipv4/flow add invalid"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := bgpctx.EncodingContextWithAddPath(true, map[family.Family]bool{tc.family: true})
			ctxID, err := bgpctx.Registry.Register(ctx)
			require.NoError(t, err)
			framed := binary.BigEndian.AppendUint32(nil, 0xf0ffffff)
			framed = append(framed, tc.nlri...)
			var attrs [128]byte
			n := writeMPReach(attrs[:], 0, tc.family, nil, framed)
			body := buildUpdatePayload(attrs[:n], nil)
			subject := AppendUpdateForFilter(nil, nil, wireu.NewWireUpdate(body, ctxID), nil)
			require.Equal(t, tc.want, string(subject))
		})
	}
}
