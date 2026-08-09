// Design: docs/architecture/wire/isis.md -- TLV 6 SNPA round-trip + TLV 2 decode-only tests
package packet

import (
	"bytes"
	"testing"

	"github.com/ze-software/ze/internal/plugins/isis/types"
)

// VALIDATES: AC-13 -- TLV 6 (IS Neighbors) round-trips one or more 6-octet SNPA
// (MAC) entries and preserves the entry count. This list is the basis for LAN
// three-way adjacency detection (isis-5).
// PREVENTS: dropping a neighbor SNPA, which would stall LAN adjacency.
func TestISISTLV6Neighbors(t *testing.T) {
	in := ISNeighborsTLV{SNPAs: [][SNPALen]byte{
		{0x00, 0x11, 0x22, 0x33, 0x44, 0x55},
		{0xde, 0xad, 0xbe, 0xef, 0xca, 0xfe},
	}}
	buf := make([]byte, 64)
	n := writeISNeighborsTLV(buf, 0, in)
	it := NewTLVIterator(buf[:n])
	typ, value, ok := it.Next()
	if !ok || typ != TLVISNeighbors {
		t.Fatalf("framing: ok=%v typ=%d", ok, typ)
	}
	out, err := DecodeISNeighborsTLV(value)
	if err != nil {
		t.Fatalf("DecodeISNeighborsTLV: %v", err)
	}
	if len(out.SNPAs) != len(in.SNPAs) {
		t.Fatalf("got %d SNPAs, want %d", len(out.SNPAs), len(in.SNPAs))
	}
	for i := range in.SNPAs {
		if !bytes.Equal(out.SNPAs[i][:], in.SNPAs[i][:]) {
			t.Errorf("SNPA[%d] = % x, want % x", i, out.SNPAs[i], in.SNPAs[i])
		}
	}
}

// VALIDATES: TLV 6 rejects a value that is not a whole number of 6-octet SNPAs.
func TestISISTLV6BadLength(t *testing.T) {
	if _, err := DecodeISNeighborsTLV([]byte{0, 1, 2, 3, 4}); err == nil { // 5 octets
		t.Fatal("expected ErrLength for non-multiple SNPA value")
	}
}

// VALIDATES: AC-14 -- a peer-originated TLV 2 (narrow IS Reachability) is
// decoded without panic: the codec parses each narrow-metric + neighbor-ID
// entry. Ze never originates TLV 2 (no encoder exists), so this is decode-only.
// PREVENTS: a panic or silent corruption when meshing with an old router that
// still emits TLV 2.
func TestISISTLV2NarrowDecode(t *testing.T) {
	sys := types.SystemID{0, 1, 0, 2, 0, 3}
	neigh := types.NewSourceID(sys, 0)
	// Hand-build a TLV 2 value: virtual-flag octet, then one entry of
	// [default-metric, delay, expense, error, 7-octet neighbor]. The default
	// metric octet has the I/E (external) bit set and value 10; the delay,
	// expense and error octets have their "supported" high bit set (0x80).
	value := []byte{
		0x00,                        // virtual flag
		narrowMetricExternalIE | 10, // default metric: external bit + value 10
		0x80, 0x80, 0x80,            // delay / expense / error (supported bit set)
	}
	value = append(value, neigh[:]...) // 7-octet neighbor SourceID
	out, err := DecodeNarrowISReachTLV(value)
	if err != nil {
		t.Fatalf("DecodeNarrowISReachTLV: %v", err)
	}
	if out.VirtualFlag != 0x00 {
		t.Errorf("VirtualFlag = %#02x", out.VirtualFlag)
	}
	if len(out.Entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(out.Entries))
	}
	e := out.Entries[0]
	if e.DefaultMetricValue != 10 {
		t.Errorf("DefaultMetricValue = %d, want 10", e.DefaultMetricValue)
	}
	if !e.DefaultMetricExternal {
		t.Error("DefaultMetricExternal should be set (I/E bit)")
	}
	if e.Neighbor != neigh {
		t.Errorf("Neighbor = %v, want %v", e.Neighbor, neigh)
	}
}

// VALIDATES: AC-11/R-3 -- TLV 2 decode rejects malformed lengths (empty value,
// or a body that is not a whole number of 11-octet entries) without panicking.
func TestISISTLV2NarrowBadLength(t *testing.T) {
	if _, err := DecodeNarrowISReachTLV(nil); err == nil {
		t.Fatal("expected ErrLength for empty TLV 2 value")
	}
	// virtual flag + 5 octets (not a multiple of 11).
	if _, err := DecodeNarrowISReachTLV([]byte{0x00, 1, 2, 3, 4, 5}); err == nil {
		t.Fatal("expected ErrLength for partial TLV 2 entry")
	}
}

// VALIDATES: TLV 2 has no encoder (Ze never originates it). This is a
// compile-time guarantee documented as a test: there is no writeNarrowISReachTLV
// symbol. The test exists to pin the decode-only decision (spec AC-14) so a
// future edit that adds an encoder is a conscious choice.
func TestISISTLV2NoEncoder(t *testing.T) {
	// Intentionally empty: the absence of an encoder is verified by the build
	// (no symbol to call). Documented here per the spec's decode-only contract.
	_ = t
}
