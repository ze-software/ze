// VALIDATES: spec-ospfv3-1-types -- the buffer-first WriteTo helpers write exact
// big-endian fixed-width fields into a caller-owned buffer at the given offset, so the
// wire codec can lay an OSPFv3 header out field by field.
// PREVENTS: a WriteTo that mis-sizes a field or writes little-endian.
package types

import (
	"bytes"
	"testing"
)

func TestOSPFv3TypesWriteTo(t *testing.T) {
	// Lay out a fragment resembling the OSPFv3 common header tail: Router ID (4),
	// Area ID (4), Instance ID (1), then an LSType (2).
	buf := make([]byte, 11)
	off := 0
	off += RouterID{1, 2, 3, 4}.WriteTo(buf, off)
	off += AreaID{0, 0, 0, 5}.WriteTo(buf, off)
	off += InstanceID(7).WriteTo(buf, off)
	off += LSTypeRouter.WriteTo(buf, off)
	if off != 11 {
		t.Fatalf("total written = %d, want 11", off)
	}
	want := []byte{1, 2, 3, 4, 0, 0, 0, 5, 7, 0x20, 0x01}
	if !bytes.Equal(buf, want) {
		t.Errorf("layout = % x, want % x", buf, want)
	}

	// Offset honored: writing at a non-zero offset leaves earlier bytes untouched.
	buf2 := make([]byte, 6)
	InterfaceID(0x01020304).WriteTo(buf2, 2)
	if !bytes.Equal(buf2, []byte{0, 0, 1, 2, 3, 4}) {
		t.Errorf("offset write = % x", buf2)
	}
}
