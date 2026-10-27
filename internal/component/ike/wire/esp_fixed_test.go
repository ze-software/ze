// Design: docs/architecture/ike/ipsec-6-ikev2-crypto.md -- fixed-key ESP wire boundary
package wire

import (
	"encoding/binary"
	"errors"
	"testing"
)

// TestESPFixedTransformAttributes drives the byte parser with no attribute, a
// forbidden zero-valued attribute, and the nominal and malformed key sizes.
// Layout (RFC 7296 Section 3.3.2): bytes 0..3 header, 4 type, 5 reserved,
// 6..7 transform ID; bytes 8..11 hold a TV Key Length attribute when present.
func TestESPFixedTransformAttributes(t *testing.T) {
	for _, id := range []uint16{3, 28} {
		data := []byte{0, 0, 0, 8, TransformTypeENCR, 0, 0, 0}
		binary.BigEndian.PutUint16(data[6:], id)
		var transform Transform
		if err := transform.ReadFrom(data); err != nil {
			t.Fatalf("fixed transform %d: %v", id, err)
		}
		data = append(data, 0x80, 14, 0, 0)
		data[3] = 12
		for _, length := range []uint16{0, 1, 128, 192, 256, 65535} {
			binary.BigEndian.PutUint16(data[10:], length)
			if err := transform.ReadFrom(data); !errors.Is(err, ErrKeyLengthNotAllowed) {
				t.Fatalf("transform %d key attribute %d: %v", id, length, err)
			}
		}
	}
}
