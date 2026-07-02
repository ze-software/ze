// VALIDATES: spec-ospf-ext-3 RI TLV codec (RFC 7770 sec 2.3/2.4) -- the RI LSA body is a
// 4-byte-aligned Type/Length/Value stream over the ext-1 generic builder; Length excludes
// padding; the Informational Capabilities word round-trips with the sec 2.4 MSB=bit0
// numbering; and a truncated body is reported, never panicked (AC-7/AC-14, R-4).
// PREVENTS: a TLV Length that counts padding (desyncing a receiver), a mis-numbered bit
// mask, or an iterator that panics on malformed input.
package packet

import (
	"bytes"
	"testing"
)

func TestRITLVRoundTrip(t *testing.T) {
	in := []RITLV{
		{Type: RITLVInformationalCapabilities, Value: RICapabilitiesValue(RIInfoBitMask(RIInfoBitStubRouter) | RIInfoBitMask(RIInfoBitTrafficEngineering))},
		{Type: RITLVFunctionalCapabilities, Value: RICapabilitiesValue(0)},
		{Type: 8, Value: []byte{1, 2, 3}},
	}
	body := EncodeRITLVs(in)
	decoded, err := DecodeRITLVStream(body)
	if err != nil {
		t.Fatalf("decode error on well-formed body: %v", err)
	}
	got := make([]RITLV, 0, len(decoded))
	for _, tlv := range decoded {
		got = append(got, RITLV{Type: tlv.Type, Value: append([]byte(nil), tlv.Value...)})
	}
	if len(got) != len(in) {
		t.Fatalf("decoded %d TLVs, want %d", len(got), len(in))
	}
	for i := range in {
		if got[i].Type != in[i].Type || !bytes.Equal(got[i].Value, in[i].Value) {
			t.Fatalf("TLV %d = %+v, want %+v", i, got[i], in[i])
		}
	}
}

func TestRITLVAlignment(t *testing.T) {
	// RFC 7770 sec 2.3: Length is the value octets only; total on-wire size is 4 + roundup4(len).
	for n := range 8 {
		val := make([]byte, n)
		for i := range val {
			val[i] = byte(i + 1)
		}
		body := EncodeRITLVs([]RITLV{{Type: 8, Value: val}})
		wantTotal := 4 + ((n + 3) &^ 3)
		if len(body) != wantTotal {
			t.Fatalf("len %d: body %d bytes, want %d", n, len(body), wantTotal)
		}
		// The on-wire Length field (bytes 2..3) must equal n (padding excluded).
		gotLen := int(body[2])<<8 | int(body[3])
		if gotLen != n {
			t.Fatalf("len %d: encoded Length field = %d, want %d (padding excluded)", n, gotLen, n)
		}
		decoded, err := DecodeRITLVStream(body)
		if err != nil || len(decoded) != 1 {
			t.Fatalf("len %d: decode: %d TLVs err=%v", n, len(decoded), err)
		}
		if len(decoded[0].Value) != n {
			t.Fatalf("len %d: decoded value len %d", n, len(decoded[0].Value))
		}
	}
}

func TestRICapabilitiesRoundTrip(t *testing.T) {
	field := RIInfoBitMask(RIInfoBitGracefulRestartHelper) | RIInfoBitMask(RIInfoBitStubRouter)
	v := RICapabilitiesValue(field)
	if len(v) != RICapabilitiesMinLen {
		t.Fatalf("capabilities value len = %d, want %d", len(v), RICapabilitiesMinLen)
	}
	if got := RIReadCapabilities(v); got != field {
		t.Fatalf("read back %#08x, want %#08x", got, field)
	}
}

func TestRIInfoBitMask(t *testing.T) {
	// RFC 7770 sec 2.4: bits numbered left to right, MSB = bit 0.
	cases := []struct {
		bit  uint
		mask uint32
	}{
		{RIInfoBitGracefulRestart, 0x80000000},
		{RIInfoBitGracefulRestartHelper, 0x40000000},
		{RIInfoBitStubRouter, 0x20000000},
		{RIInfoBitTrafficEngineering, 0x10000000},
	}
	for _, c := range cases {
		if got := RIInfoBitMask(c.bit); got != c.mask {
			t.Fatalf("bit %d mask = %#08x, want %#08x", c.bit, got, c.mask)
		}
	}
	if RIInfoBitMask(32) != 0 {
		t.Fatalf("out-of-range bit must yield mask 0")
	}
}

// VALIDATES: spec-ospf-ext-3 (RFC 7770 sec 3) -- RITLVsEncodedLen sums each TLV as the 4-byte
// header plus its value padded to a 4-octet boundary, so the RI originator sizes an instance
// and detects overflow. It matches the actual encoded body length.
// PREVENTS: an RI originator under-counting a TLV set and overrunning the opaque LSA buffer.
func TestRITLVsEncodedLen(t *testing.T) {
	tlvs := []RITLV{
		{Type: RITLVInformationalCapabilities, Value: RICapabilitiesValue(0)}, // 4-byte value -> 8
		{Type: 8, Value: []byte{1, 2, 3}},                                     // 3-byte value padded to 4 -> 8
		{Type: 9, Value: nil},                                                 // 0-byte value -> 4
	}
	// 8 + 8 + 4 = 20 octets.
	if got := RITLVsEncodedLen(tlvs); got != 20 {
		t.Fatalf("RITLVsEncodedLen = %d, want 20", got)
	}
	// It must equal the length the builder actually produces.
	if got, wireLen := RITLVsEncodedLen(tlvs), len(EncodeRITLVs(tlvs)); got != wireLen {
		t.Fatalf("RITLVsEncodedLen = %d, but EncodeRITLVs produced %d bytes", got, wireLen)
	}
	if RITLVsEncodedLen(nil) != 0 {
		t.Fatalf("RITLVsEncodedLen(nil) = %d, want 0", RITLVsEncodedLen(nil))
	}
}

func TestRITLVDecodeMalformed(t *testing.T) {
	// A header claiming a value longer than the body: decode reports it and never panics.
	body := []byte{0x00, 0x08, 0xFF, 0xFF, 0x01, 0x02}
	if _, err := DecodeRITLVStream(body); err == nil {
		t.Fatalf("decode did not report the over-long TLV length")
	}
}
