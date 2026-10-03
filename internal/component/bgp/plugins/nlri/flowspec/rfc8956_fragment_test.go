package flowspec

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// RFC requirement: RFC8956-3.6-1 positive -- the enclosing IPv6 FlowSpec and VPN producers emit exactly one octet for each fragment operand, even when the supplied value contains high reserved bits.
// RFC requirement: RFC8956-3.6-1 negative -- a received two-octet fragment operand is malformed rather than accepted as a wider numeric component.
func TestRFC8956FragmentSingleOctet(t *testing.T) {
	for _, fam := range []Family{IPv6FlowSpec, IPv6FlowSpecVPN} {
		t.Run(fam.String(), func(t *testing.T) {
			component := newFlowFragmentMatchComponent([]FlowMatch{{Op: FlowOpMatch, Value: ^uint64(0)}})
			want := []byte{3, 12, 0x81, 0x0e}
			malformed := []byte{4, 12, 0x91, 0, 0x0e}
			if fam.SAFI == SAFIFlowSpecVPN {
				flow := NewFlowSpecVPN(fam, RouteDistinguisher{})
				require.NoError(t, flow.AddComponent(component))
				want = append(append([]byte{11}, make([]byte, 8)...), want[1:]...)
				require.Equal(t, len(want), flow.Len())
				buf := make([]byte, flow.Len()+4)
				n := flow.WriteTo(buf, 2)
				assert.Equal(t, want, buf[2:2+n])
				assert.Equal(t, want, flow.Bytes())
				malformed = append(append([]byte{12}, make([]byte, 8)...), malformed[1:]...)
				_, err := ParseFlowSpecVPN(fam, malformed)
				require.Error(t, err)
			} else {
				flow := NewFlowSpec(fam)
				require.NoError(t, flow.AddComponent(component))
				require.Equal(t, len(want), flow.Len())
				buf := make([]byte, flow.Len()+4)
				n := flow.WriteTo(buf, 2)
				assert.Equal(t, want, buf[2:2+n])
				assert.Equal(t, want, flow.Bytes())
				_, err := ParseFlowSpec(fam, malformed)
				require.Error(t, err)
			}
		})
	}
}

// RFC requirement: RFC8956-3.6-2 positive -- semantic IsF, FF and LF flags emit the Figure 1 values 2, 4 and 8 through both IPv6 NLRI producers.
// RFC requirement: RFC8956-3.6-2 negative -- IPv4's Don't Fragment bit is reserved in IPv6 and cannot leak into IPv6 output; sharing the component does not remove it from IPv4 output.
func TestRFC8956FragmentEncodingRespectsEnclosingFamily(t *testing.T) {
	component := NewFlowFragmentComponent(FlowFragDontFragment|FlowFragIsFragment, FlowFragFirstFragment, FlowFragLastFragment)
	for _, fam := range []Family{IPv6FlowSpec, IPv4FlowSpec, IPv6FlowSpecVPN, IPv4FlowSpecVPN} {
		t.Run(fam.String(), func(t *testing.T) {
			want := []byte{7, 12, 0, 2, 0, 4, 0x80, 8}
			if fam.AFI == AFIIPv4 {
				want[3] = 3
			}
			if fam.SAFI == SAFIFlowSpecVPN {
				flow := NewFlowSpecVPN(fam, RouteDistinguisher{})
				require.NoError(t, flow.AddComponent(component))
				want = append(append([]byte{15}, make([]byte, 8)...), want[1:]...)
				assert.Equal(t, want, flow.Bytes())
			} else {
				flow := NewFlowSpec(fam)
				require.NoError(t, flow.AddComponent(component))
				assert.Equal(t, want, flow.Bytes())
			}
		})
	}
}

// RFC requirement: RFC8956-3.6-3 positive -- reserved operand bits are accepted and ignored while IPv6 IsF, FF and LF decode to their semantic flags.
// RFC requirement: RFC8956-3.6-3 negative -- reserved bits, including IPv4's DF bit, cannot appear in the decoded IPv6 match or its re-encoded NLRI.
func TestRFC8956FragmentDecodeIgnoresReservedBits(t *testing.T) {
	for _, fam := range []Family{IPv6FlowSpec, IPv6FlowSpecVPN} {
		t.Run(fam.String(), func(t *testing.T) {
			// Three OR terms each carry one meaningful flag and every reserved
			// bit. The last term is reserved bits only and must decode to zero.
			raw := []byte{9, 12, 1, 0xf3, 1, 0xf5, 1, 0xf9, 0x81, 0xf1}
			want := []byte{9, 12, 1, 2, 1, 4, 1, 8, 0x81, 0}
			var components []FlowComponent
			var encoded []byte
			if fam.SAFI == SAFIFlowSpecVPN {
				raw = append(append([]byte{17}, make([]byte, 8)...), raw[1:]...)
				want = append(append([]byte{17}, make([]byte, 8)...), want[1:]...)
				flow, err := ParseFlowSpecVPN(fam, raw)
				require.NoError(t, err)
				components, encoded = flow.Components(), flow.Bytes()
			} else {
				flow, err := ParseFlowSpec(fam, raw)
				require.NoError(t, err)
				components, encoded = flow.Components(), flow.Bytes()
			}
			require.Len(t, components, 1)
			matches, ok := components[0].(interface{ Matches() []FlowMatch })
			require.True(t, ok)
			assert.Equal(t, []FlowMatch{
				{Op: FlowOpMatch, Value: uint64(FlowFragIsFragment)},
				{Op: FlowOpMatch, Value: uint64(FlowFragFirstFragment)},
				{Op: FlowOpMatch, Value: uint64(FlowFragLastFragment)},
				{Op: FlowOpMatch, Value: 0},
			}, matches.Matches())
			assert.Equal(t, want, encoded)
		})
	}
}
