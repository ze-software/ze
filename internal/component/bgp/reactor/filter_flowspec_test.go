// Design: docs/architecture/api/commands.md -- show policy test, its text-before and text-after
// Related: filter_flowspec.go -- appendFlowSpecFilterBlock, the producer under test
// Related: policy_dryrun_display.go -- displayUpdate, which renders the NLRI blocks

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

// flowSpecDisplayNLRI answers the NLRI blocks show policy test prints for
// update, as displayUpdate renders them.
func flowSpecDisplayNLRI(t *testing.T, update *wireu.WireUpdate) string {
	t.Helper()
	display, err := displayUpdate(update)
	require.NoError(t, err)
	return string(display.nlri)
}

// TestFlowSpecUpdateExposesDestinationToPrefixPolicy proves the dry-run
// display shows a FlowSpec rule by its destination prefix, and a rule whose
// destination cannot be read as invalid. Method: three FlowSpec UPDATEs built
// by the UPDATE builder, displayed through displayUpdate.
func TestFlowSpecUpdateExposesDestinationToPrefixPolicy(t *testing.T) {
	family.RegisterTestFamilies()
	builder := message.NewUpdateBuilder(65001, false, true, false)
	for _, tc := range []struct {
		name string
		nlri []byte
		want string
	}{
		{"destination", []byte{8, 1, 24, 10, 1, 0, 5, 0x81, 80}, "mp ipv4/flow add 10.1.0.0/24"},
		{"no-destination", []byte{3, 5, 0x81, 80}, "mp ipv4/flow add invalid"},
		{"truncated", []byte{5, 1, 24, 10}, "mp ipv4/flow add invalid"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			update := builder.BuildFlowSpec(message.FlowSpecParams{NLRI: tc.nlri})
			wire := make([]byte, update.Len(nil))
			update.WriteTo(wire, 0, nil)
			require.Equal(t, tc.want, flowSpecDisplayNLRI(t, wireu.NewWireUpdate(wire[19:], 0)))
		})
	}
}

// TestFlowSpecPrefixPolicyUsesNativeAddPathFraming proves the display reads a
// FlowSpec rule on a session that negotiated ADD-PATH for its family with the
// path identifier framing in front of the rule. Method: MP_REACH_NLRI
// carrying a path identifier then the rule, on an ADD-PATH context.
func TestFlowSpecPrefixPolicyUsesNativeAddPathFraming(t *testing.T) {
	family.RegisterTestFamilies()
	for _, tc := range []struct {
		name   string
		family family.Family
		nlri   []byte
		want   string
	}{
		{"ipv4-extended", family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpec}, []byte{0xf0, 5, 1, 24, 10, 1, 0}, "mp ipv4/flow add 10.1.0.0/24"},
		{"vpn4", family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpecVPN}, []byte{13, 0, 0, 0xfd, 0xe9, 0, 0, 0, 1, 1, 24, 10, 1, 0}, "mp ipv4/flow-vpn add 10.1.0.0/24"},
		{"ipv6", family.Family{AFI: family.AFIIPv6, SAFI: family.SAFIFlowSpec}, []byte{7, 1, 32, 0, 0x20, 1, 0x0d, 0xb8}, "mp ipv6/flow add 2001:db8::/32"},
		{"ipv6-offset", family.Family{AFI: family.AFIIPv6, SAFI: family.SAFIFlowSpec}, []byte{5, 1, 32, 16, 0x0d, 0xb8}, "mp ipv6/flow add invalid"},
		{"truncated-destination", family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpec}, []byte{4, 1, 24, 10, 1}, "mp ipv4/flow add invalid"},
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
			require.Equal(t, tc.want, flowSpecDisplayNLRI(t, wireu.NewWireUpdate(body, ctxID)))
		})
	}
}
