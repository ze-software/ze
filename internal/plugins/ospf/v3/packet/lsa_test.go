// VALIDATES: spec-ospfv3-2-wire AC-7, AC-17, AC-18 -- the 20-octet LSA header
// round-trips and builds a types.LSAKey, the iterator bound-checks the Length,
// retained RawBytes re-encode byte-for-byte, and an unknown LS Type passes
// through as an opaque span.
// PREVENTS: an Options byte creeping back into the LSA header, re-marshal drift on
// flood, and a crafted Length over-reading the buffer.

package packet

import (
	"bytes"
	"testing"

	"github.com/ze-software/ze/internal/plugins/ospf/v3/types"
)

func TestOSPFv3LSAHeaderRoundTrip(t *testing.T) {
	h := sampleLSAHeader(t, types.LSTypeRouter, "0.0.0.1")
	h.Checksum = 0x1234
	h.Length = LSAHeaderLen
	buf := make([]byte, LSAHeaderLen)
	if n := h.WriteTo(buf, 0); n != LSAHeaderLen {
		t.Fatalf("LSAHeader.WriteTo wrote %d, want %d", n, LSAHeaderLen)
	}
	got, err := DecodeLSAHeader(buf)
	if err != nil {
		t.Fatalf("DecodeLSAHeader: %v", err)
	}
	if got != h {
		t.Fatalf("DecodeLSAHeader = %+v, want %+v", got, h)
	}
	if LSAHeaderLen != 20 {
		t.Fatalf("LSAHeaderLen = %d, want 20", LSAHeaderLen)
	}
}

func TestOSPFv3LSALengthClampsAt16Bits(t *testing.T) {
	// LS Length is a 16-bit field (RFC 5340 sec A.4.1); an over-size LSA body (impossible in
	// practice, but a guard against a hypothetical bug) must clamp the length field to 0xFFFF
	// rather than silently wrap to a small value that a receiver would misparse into its LSDB.
	lsa := LSA{Header: sampleLSAHeader(t, types.LSTypeLink, "0.0.0.3"), Body: make([]byte, 0x10005)}
	buf := make([]byte, LSAHeaderLen+len(lsa.Body))
	lsa.WriteTo(buf, 0)
	if got := readUint16(buf, lsaLengthOff); got != 0xFFFF {
		t.Fatalf("clamped LS Length = %#x, want 0xFFFF (no silent wrap)", got)
	}
	if lsa.Header.Length != 0xFFFF {
		t.Fatalf("header Length = %#x, want 0xFFFF", lsa.Header.Length)
	}
}

func TestOSPFv3WireUsesTypesLSAKey(t *testing.T) {
	// LSTypeRouter (0x2001) is the canonical area-scoped type (RFC 5340 S2/S1=01).
	areaScope := types.LSTypeRouter.Scope()
	h := sampleLSAHeader(t, types.LSTypeIntraAreaPrefix, "0.0.0.9")
	h.AdvertisingRouter = mustRouterID(t, "203.0.113.5")
	h.Length = LSAHeaderLen
	buf := make([]byte, LSAHeaderLen)
	h.WriteTo(buf, 0)
	got, err := DecodeLSAHeader(buf)
	if err != nil {
		t.Fatalf("DecodeLSAHeader: %v", err)
	}
	want := types.LSAKey{Type: h.Type, LinkStateID: h.LinkStateID, AdvertisingRouter: h.AdvertisingRouter}
	if got.Key() != want {
		t.Fatalf("Key() = %+v, want %+v", got.Key(), want)
	}
	// The key carries scope via the LS Type; a separate scope field would be a bug.
	if got.Key().Type.Scope() != areaScope {
		t.Fatalf("Intra-Area-Prefix scope = %v, want area", got.Key().Type.Scope())
	}
}

func TestOSPFv3LSAIteratorBounds(t *testing.T) {
	good := encodeLSA(t, sampleRouterLSA(t))

	t.Run("truncated length past buffer", func(t *testing.T) {
		bad := append([]byte(nil), good...)
		writeUint16(bad, lsaLengthOff, uint16(len(bad)+1))
		it := NewLSAIterator(bad)
		if it.Next() {
			t.Fatalf("iterator accepted a Length past the buffer")
		}
		if it.Err() == nil {
			t.Fatalf("iterator Err = nil, want truncation")
		}
	})

	t.Run("length below header", func(t *testing.T) {
		bad := append([]byte(nil), good...)
		writeUint16(bad, lsaLengthOff, LSAHeaderLen-1)
		it := NewLSAIterator(bad)
		if it.Next() {
			t.Fatalf("iterator accepted a Length below 20")
		}
		if it.Err() == nil {
			t.Fatalf("iterator Err = nil, want length error")
		}
	})

	t.Run("trailing partial header", func(t *testing.T) {
		bad := append(append([]byte(nil), good...), 0x00, 0x01, 0x02)
		it := NewLSAIterator(bad)
		if !it.Next() {
			t.Fatalf("iterator rejected the first valid LSA: %v", it.Err())
		}
		if it.Next() {
			t.Fatalf("iterator accepted a trailing partial header")
		}
		if it.Err() == nil {
			t.Fatalf("iterator Err = nil, want truncation on the trailing bytes")
		}
	})
}

func TestOSPFv3LSARawBytesRoundTrip(t *testing.T) {
	wire := encodeLSA(t, sampleRouterLSA(t))
	decoded, err := DecodeLSA(wire)
	if err != nil {
		t.Fatalf("DecodeLSA: %v", err)
	}
	// A decoded LSA with no typed body re-encodes verbatim from RawBytes.
	out := make([]byte, decoded.EncodedLen())
	n := decoded.WriteTo(out, 0)
	if n != len(wire) {
		t.Fatalf("re-encoded length = %d, want %d", n, len(wire))
	}
	if !bytes.Equal(out, wire) {
		t.Fatalf("re-encode drifted from RawBytes:\n got % x\nwant % x", out, wire)
	}
}

func TestOSPFv3UnknownLSAPassthrough(t *testing.T) {
	// An unknown LS Type (function code outside the base set) with the U-bit set:
	// the codec retains it as an opaque span and re-encodes it byte-for-byte.
	unknownType := types.LSType(0x8000 | 0x2000 | 0x0abc) // U-bit + area scope + unknown function
	h := sampleLSAHeader(t, unknownType, "1.2.3.4")
	h.Length = LSAHeaderLen + 4
	h.Checksum = 0
	raw := make([]byte, h.Length)
	h.WriteTo(raw, 0)
	copy(raw[LSAHeaderLen:], []byte{0xde, 0xad, 0xbe, 0xef})
	FinalizeLSAChecksum(raw)

	decoded, err := DecodeLSA(raw)
	if err != nil {
		t.Fatalf("DecodeLSA unknown: %v", err)
	}
	if decoded.Header.Type.Known() {
		t.Fatalf("test type %#x should be unknown", uint16(unknownType))
	}
	out := make([]byte, decoded.EncodedLen())
	decoded.WriteTo(out, 0)
	if !bytes.Equal(out, raw) {
		t.Fatalf("opaque passthrough changed bytes:\n got % x\nwant % x", out, raw)
	}
}
