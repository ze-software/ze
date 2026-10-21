// Design: docs/architecture/wire/nlri-flowspec.md -- preferred numeric emission widths.
package flowspec

import (
	"bytes"
	"testing"
)

// TestFlowLabelEmitsFourOctets pins the Flow Label operand width. Every value,
// including one that would fit in one or two octets, is written as a
// four-octet quantity, from the typed constructor and when a parsed component
// narrower on the wire is reconstructed, and Len agrees with the bytes written.
// RFC 8956 Section 3.7: "Type 13 component values SHOULD be encoded as 4-octet
// quantities (numeric_op len=10)." ExaBGP 6.0 sends the same width.
//
// VALIDATES: RFC8956-3.7-1, the Flow Label operand is four octets on emission.
// PREVENTS: a two-octet Flow Label that ExaBGP 6.0 frames and the RFC disagree with.
func TestFlowLabelEmitsFourOctets(t *testing.T) {
	for _, tc := range []struct {
		label uint32
		want  []byte
	}{
		{0, []byte{0x0d, 0xa1, 0, 0, 0, 0}},
		{255, []byte{0x0d, 0xa1, 0, 0, 0, 0xff}},
		{2013, []byte{0x0d, 0xa1, 0, 0, 0x07, 0xdd}},
		{0xfffff, []byte{0x0d, 0xa1, 0, 0x0f, 0xff, 0xff}},
	} {
		component := NewFlowFlowLabelComponent(tc.label)
		if got := component.Len(); got != len(tc.want) {
			t.Errorf("label %d: Len %d, want %d", tc.label, got, len(tc.want))
		}
		buf := make([]byte, 16)
		written := component.WriteTo(buf, 0)
		if got := buf[:written]; !bytes.Equal(got, tc.want) {
			t.Errorf("label %d: wrote %x, want %x", tc.label, got, tc.want)
		}
	}

	// A peer may send a narrower operand (RFC 8955 Section 4.2.1.1 allows
	// every len value), and reconstruction still emits the preferred width.
	narrow := []byte{4, byte(FlowFlowLabel), 0x91, 0x07, 0xdd}
	fs, err := ParseFlowSpec(ipv6FlowSpec, narrow)
	if err != nil {
		t.Fatalf("two-octet flow label %x rejected: %v", narrow, err)
	}
	if got, want := fs.Bytes(), []byte{6, 0x0d, 0xa1, 0, 0, 0x07, 0xdd}; !bytes.Equal(got, want) {
		t.Errorf("reconstructed %x as %x, want %x", narrow, got, want)
	}
}
