// VALIDATES: spec-ospf-ext-1 AC-7/A-1/AC-12 -- the opaque Link State ID splits into an
// 8-bit Opaque Type + 24-bit Opaque ID (both directions), and opaque LSAs (types 9/10/11)
// decode and re-encode verbatim with a valid Fletcher checksum.
// PREVENTS: a byte-order slip in the LS-ID split (consumers colliding in the key
// namespace) and a codec regression that corrupts the verbatim opaque passthrough.
package packet

import (
	"bytes"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/plugins/ospf/types"
)

// TestOpaqueLinkStateIDSplit pins AC-7 / R-4: the Opaque Type is the high byte and the
// Opaque ID is the low 24 bits of the Link State ID, in both directions (RFC 5250 App A.2).
func TestOpaqueLinkStateIDSplit(t *testing.T) {
	id := types.LinkStateID{0xAA, 0xBB, 0xCC, 0xDD}
	// RFC requirement: RFC5250-3-2 positive -- the Link State ID splits into a 1-byte Opaque Type (high octet) and a 3-byte Opaque ID (low 24 bits)
	if got := OpaqueTypeOf(id); got != 0xAA {
		t.Fatalf("OpaqueType = %#x, want 0xAA", got)
	}
	if got := OpaqueIDOf(id); got != 0x00BBCCDD {
		t.Fatalf("OpaqueID = %#x, want 0x00BBCCDD", got)
	}
	// Round-trip: composing (type, id) reproduces the Link State ID exactly.
	if got := OpaqueLinkStateID(0xAA, 0x00BBCCDD); got != id {
		t.Fatalf("OpaqueLinkStateID round-trip = % x, want % x", got[:], id[:])
	}
	// The Opaque ID is masked to 24 bits: a high byte in the id argument does not leak
	// into the Opaque Type slot.
	if got := OpaqueLinkStateID(0x01, 0xFFBBCCDD); got != (types.LinkStateID{0x01, 0xBB, 0xCC, 0xDD}) {
		t.Fatalf("OpaqueLinkStateID mask = % x", got[:])
	}
	// Boundary: max Opaque Type (255) and max Opaque ID (0xFFFFFF).
	maxID := OpaqueLinkStateID(0xFF, 0x00FFFFFF)
	if OpaqueTypeOf(maxID) != 0xFF || OpaqueIDOf(maxID) != 0x00FFFFFF {
		t.Fatalf("boundary split failed: type=%#x id=%#x", OpaqueTypeOf(maxID), OpaqueIDOf(maxID))
	}
	// The accessors on a decoded LSA agree with the standalone functions.
	lsa := LSA{Header: LSAHeader{LinkStateID: id}}
	if lsa.OpaqueType() != 0xAA || lsa.OpaqueID() != 0x00BBCCDD {
		t.Fatalf("LSA accessors = type %#x id %#x", lsa.OpaqueType(), lsa.OpaqueID())
	}
}

// TestOpaqueLSARoundTrip pins A-1 / AC-12: an opaque LSA (types 9/10/11) decodes and
// re-encodes byte-for-byte and its Fletcher checksum verifies.
func TestOpaqueLSARoundTrip(t *testing.T) {
	for _, lt := range []types.LSType{types.LSTypeOpaqueLink, types.LSTypeOpaqueArea, types.LSTypeOpaqueAS} {
		body := []byte{0x00, 0x01, 0x00, 0x04, 0xde, 0xad, 0xbe, 0xef}
		lsa := LSA{
			Header: LSAHeader{
				Options:           types.OptionO,
				Type:              lt,
				LinkStateID:       OpaqueLinkStateID(1, 0x000010),
				AdvertisingRouter: types.RouterID{1, 1, 1, 1},
				Sequence:          types.InitialSequenceNumber,
			},
			Opaque: &OpaqueLSA{Type: lt, Data: body},
		}
		buf := make([]byte, lsa.EncodedLen())
		lsa.WriteTo(buf, 0)
		decoded, err := DecodeLSA(buf)
		if err != nil {
			t.Fatalf("type %v DecodeLSA: %v", lt, err)
		}
		if !decoded.VerifyChecksum() {
			t.Fatalf("type %v checksum invalid", lt)
		}
		if decoded.Header.Type != lt {
			t.Fatalf("type %v decoded as %v", lt, decoded.Header.Type)
		}
		if !bytes.Equal(decoded.Body, body) {
			t.Fatalf("type %v body mismatch: % x", lt, decoded.Body)
		}
		// Re-encode the decoded (raw) form verbatim.
		reenc := make([]byte, decoded.EncodedLen())
		decoded.WriteTo(reenc, 0)
		if !bytes.Equal(reenc, buf) {
			t.Fatalf("type %v re-encode not byte-for-byte:\n got % x\nwant % x", lt, reenc, buf)
		}
	}
}
