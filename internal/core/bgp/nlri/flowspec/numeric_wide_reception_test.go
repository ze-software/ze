// Design: docs/architecture/wire/nlri-flowspec.md -- preferred numeric emission widths.
package flowspec

import (
	"bytes"
	"testing"
)

// TestFlowSpecPreferredNumericWideReception prevents an emission SHOULD from
// becoming a receive-side width MUST. Legal wide operands retain their value;
// reconstruction emits the preferred width when that value fits the component.
// RFC 8955 Section 4.2.1.1: "This encodes 1 (len=00), 2 (len=01), 4
// (len=10), and 8 (len=11) octets."
// These are compatibility controls for value 255, not negative inputs to the
// emission SHOULDs and not proof of arbitrary high-bit operand preservation.
func TestFlowSpecPreferredNumericWideReception(t *testing.T) {
	for _, kind := range []FlowComponentType{FlowDestPort, FlowSourcePort, FlowICMPCode, FlowPacketLength} {
		for _, operand := range [][]byte{
			{0x91, 0, 0xff},
			{0xa1, 0, 0, 0, 0xff},
			{0xb1, 0, 0, 0, 0, 0, 0, 0, 0xff},
		} {
			wire := append([]byte{byte(1 + len(operand)), byte(kind)}, operand...)
			// RFC 8955 Sections 4.2.1.1 and 4.2.2.5/6/8/10.
			fs, err := ParseFlowSpec(ipv4FlowSpec, wire)
			if err != nil {
				t.Fatalf("valid wider NLRI %x rejected: %v", wire, err)
			}
			if len(fs.Components()) != 1 {
				t.Fatalf("%x decoded %d components", wire, len(fs.Components()))
			}
			numeric, ok := fs.Components()[0].(*numericComponent)
			if !ok {
				t.Fatalf("%x decoded as %T", wire, fs.Components()[0])
			}
			matches := numeric.Matches()
			if len(matches) != 1 {
				t.Fatalf("%x decoded %d matches", wire, len(matches))
			}
			if matches[0].Value != 255 || matches[0].Op != FlowOpEqual || matches[0].And {
				t.Fatalf("%x decoded as %+v", wire, matches[0])
			}
			if got, want := fs.Bytes(), []byte{3, byte(kind), 0x81, 0xff}; !bytes.Equal(got, want) {
				t.Errorf("reconstructed %x as %x, want %x", wire, got, want)
			}
		}
	}
}
