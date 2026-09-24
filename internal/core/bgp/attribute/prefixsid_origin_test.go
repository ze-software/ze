// Design: docs/architecture/api/update-syntax.md -- SRv6 Prefix-SID origination
// RFC 9252 Section 3.2.1 -- see rfc/short/rfc9252.md.
package attribute

import (
	"bytes"
	"testing"
)

// TestEncodePrefixSIDSRv6StructureBounds exercises the operator-text boundary.
// A transposed field ending at bit 128 is valid under erratum 7817, while
// overflowing structures and nonzero transposed bits must not reach the wire.
func TestEncodePrefixSIDSRv6StructureBounds(t *testing.T) {
	encoded, err := EncodePrefixSIDSRv6("l3-service 2001:db8:: 0x13 [64,0,64,0,16,112]")
	if err != nil {
		t.Fatalf("exact 128-bit boundary: %v", err)
	}
	want := []byte{
		5, 0, 34, 0, 1, 0, 30, 0,
		0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0x13, 0, 1, 0, 6, 64, 0, 64, 0, 16, 112,
	}
	if !bytes.Equal(encoded, want) {
		t.Fatalf("encoded SID = %x, want %x", encoded, want)
	}

	for _, tc := range []struct {
		name  string
		value string
	}{
		{"sum beyond SID", "l3-service 2001:db8:: 0x13 [64,0,65,0,0,0]"},
		{"range beyond structure", "l3-service 2001:db8:: 0x13 [64,0,16,0,17,64]"},
		{"range beyond SID", "l3-service 2001:db8:: 0x13 [64,0,64,0,2,127]"},
		{"offset without transposition", "l3-service 2001:db8:: 0x13 [64,0,16,0,0,64]"},
		{"unsupported arguments", "l3-service 2001:db8:: 0x13 [64,0,16,8,0,0]"},
		{"transposed bit set", "l3-service 2001:db8::1 0x13 [64,0,64,0,1,127]"},
		{"trailing token", "l3-service 2001:db8:: 0x13 [64,0,64,0,16,112] extra"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			encoded, err := EncodePrefixSIDSRv6(tc.value)
			if err == nil {
				t.Fatalf("invalid SID produced wire bytes %x", encoded)
			}
			if encoded != nil {
				t.Fatalf("rejected SID returned partial attribute %x", encoded)
			}
		})
	}
}
