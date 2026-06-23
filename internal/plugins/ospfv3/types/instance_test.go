// VALIDATES: spec-ospfv3-1-types AC-2 -- InstanceID covers the full uint8 range and
// WriteTo emits one octet.
// PREVENTS: an Instance ID that is dropped silently on the wire.
package types

import "testing"

func TestOSPFv3InstanceIDBoundaries(t *testing.T) {
	id := InstanceID(255)
	buf := make([]byte, 1)
	if n := id.WriteTo(buf, 0); n != 1 || buf[0] != 255 {
		t.Errorf("WriteTo n=%d buf=%v", n, buf)
	}
}
