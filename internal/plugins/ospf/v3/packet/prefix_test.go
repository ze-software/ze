// VALIDATES: spec-ospfv3-2-wire AC-14, AC-18 -- an IPv6 prefix at the boundary
// lengths 0,1,31,32,33,64,127,128 encodes ((len+31)/32)*4 address bytes, the
// padding past the prefix length is zero, a non-zero padding bit is rejected on
// decode, a /129 is rejected, and a short buffer never panics. Both carriage
// forms (repeating entry and inlined) are covered.
// PREVENTS: a hardcoded /128 length, mis-padding, or an over-read on a short
// prefix buffer.

package packet

import (
	"errors"
	"testing"

	"github.com/ze-software/ze/internal/plugins/ospf/v3/types"
)

func TestOSPFv3WirePrefixBoundaries(t *testing.T) {
	cases := []struct {
		bits      uint8
		wantBytes int
	}{
		{0, 0}, {1, 4}, {31, 4}, {32, 4}, {33, 8}, {64, 8}, {127, 16}, {128, 16},
	}
	for _, tc := range cases {
		t.Run("repeating-entry", func(t *testing.T) {
			p := makePrefix(t, tc.bits, types.OptPrefixLA, 0x1234)
			if p.Length.ByteLen() != tc.wantBytes {
				t.Fatalf("/%d ByteLen = %d, want %d", tc.bits, p.Length.ByteLen(), tc.wantBytes)
			}
			buf := make([]byte, p.encodedLen())
			n := p.writeTo(buf, 0)
			if n != prefixHeaderLen+tc.wantBytes {
				t.Fatalf("/%d writeTo wrote %d, want %d", tc.bits, n, prefixHeaderLen+tc.wantBytes)
			}
			got, consumed, err := decodePrefix(buf, 0)
			if err != nil {
				t.Fatalf("/%d decodePrefix: %v", tc.bits, err)
			}
			if consumed != prefixHeaderLen+tc.wantBytes {
				t.Fatalf("/%d consumed %d, want %d", tc.bits, consumed, prefixHeaderLen+tc.wantBytes)
			}
			assertPrefixEqual(t, got, p)
			if got.Field16 != 0x1234 {
				t.Fatalf("/%d 16-bit field = %#x, want 0x1234", tc.bits, got.Field16)
			}
		})
	}
}

// TestIPv4OverV3PrefixRoundTrip pins RFC 5838 §2.7 / RFC 5340 §A.4.1: an IPv4 prefix (0..32
// bits) rides the OSPFv3 prefix codec as a single 32-bit word and decodes back byte-exact.
func TestIPv4OverV3PrefixRoundTrip(t *testing.T) {
	cases := []struct {
		bits      uint8
		addr      []byte // ByteLen-padded IPv4 address bytes
		wantBytes int
	}{
		{0, nil, 0},
		{8, []byte{10, 0, 0, 0}, 4},
		{24, []byte{10, 20, 30, 0}, 4},
		{32, []byte{192, 0, 2, 1}, 4},
	}
	for _, tc := range cases {
		plen := mustPrefixLen(t, tc.bits)
		if plen.ByteLen() != tc.wantBytes {
			t.Fatalf("/%d ByteLen = %d, want %d (one word for an IPv4 prefix)", tc.bits, plen.ByteLen(), tc.wantBytes)
		}
		addr := make([]byte, plen.ByteLen())
		copy(addr, tc.addr)
		p := Prefix{Length: plen, Field16: 20, Address: addr}
		buf := make([]byte, p.encodedLen())
		if n := p.writeTo(buf, 0); n != prefixHeaderLen+tc.wantBytes {
			t.Fatalf("/%d writeTo = %d, want %d", tc.bits, n, prefixHeaderLen+tc.wantBytes)
		}
		got, consumed, err := decodePrefix(buf, 0)
		if err != nil {
			t.Fatalf("/%d decodePrefix: %v", tc.bits, err)
		}
		if consumed != prefixHeaderLen+tc.wantBytes {
			t.Fatalf("/%d consumed %d, want %d", tc.bits, consumed, prefixHeaderLen+tc.wantBytes)
		}
		assertPrefixEqual(t, got, p)
	}
}

func TestOSPFv3WirePrefixRejectsNonZeroPadding(t *testing.T) {
	// A /33 carries 8 address bytes; bits 33..63 must be zero. Flip a bit in the
	// padding region and confirm decode rejects it.
	plen := mustPrefixLen(t, 33)
	buf := make([]byte, prefixHeaderLen+plen.ByteLen())
	buf[0] = 33
	buf[prefixHeaderLen+7] = 0x01 // a bit well past bit 33
	if _, _, err := decodePrefix(buf, 0); err == nil {
		t.Fatalf("decodePrefix accepted non-zero padding")
	}
}

func TestOSPFv3WirePrefixRejectsOverlongLength(t *testing.T) {
	// PrefixLength 129 is invalid; the types layer rejects it.
	if _, err := types.NewPrefixLength(129); !errors.Is(err, types.ErrOutOfRange) {
		t.Fatalf("NewPrefixLength(129) err = %v, want out-of-range", err)
	}
	buf := []byte{129, 0, 0, 0, 0, 0, 0, 0}
	if _, _, err := decodePrefix(buf, 0); err == nil {
		t.Fatalf("decodePrefix accepted PrefixLength 129")
	}
}

func TestOSPFv3WirePrefixRejectsShortBuffer(t *testing.T) {
	// A /64 needs 4 (header) + 8 (address) bytes; a 6-byte buffer must be rejected
	// without panic.
	buf := []byte{64, 0, 0, 0, 0, 0}
	if _, _, err := decodePrefix(buf, 0); err == nil {
		t.Fatalf("decodePrefix accepted a short buffer")
	}
	// A buffer too short even for the 4-octet header.
	if _, _, err := decodePrefix([]byte{64, 0}, 0); err == nil {
		t.Fatalf("decodePrefix accepted a sub-header buffer")
	}
}

func TestOSPFv3WirePrefixInlinedForm(t *testing.T) {
	// The inlined carriage form (Inter-Area-Prefix / AS-External) lays the length,
	// options, and 16-bit field at separate offsets. Exercise it via the
	// Inter-Area-Prefix-LSA, which is the canonical inlined consumer.
	for _, bits := range []uint8{0, 1, 64, 128} {
		want := makePrefix(t, bits, types.OptPrefixNU, 0)
		lsa := InterAreaPrefixLSA{Metric: 0x010203, Prefix: want}
		buf := make([]byte, lsa.EncodedLen())
		n := lsa.WriteTo(buf, 0)
		if n != lsa.EncodedLen() {
			t.Fatalf("/%d inlined WriteTo wrote %d, want %d", bits, n, lsa.EncodedLen())
		}
		got, err := decodeInterAreaPrefixLSA(buf)
		if err != nil {
			t.Fatalf("/%d decodeInterAreaPrefixLSA: %v", bits, err)
		}
		assertPrefixEqual(t, got.Prefix, want)
	}
}
