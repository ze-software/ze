// VALIDATES: spec-ospf-ext-4 -- the RFC 7684 Extended Prefix Opaque LSA body codec: the
// Extended Prefix TLV fixed fields + 32-bit Address Prefix + sub-TLV region round-trip
// byte-for-byte (AC-2/AC-12/AC-17), the AF=0 Address Prefix is a fixed 32 bits for any Prefix
// Length (AC-17/R-10), the Extended Prefix Range TLV is carried as an opaque container (A-5),
// and an empty-sub-TLV container round-trips (AC-12/A-4).
// PREVENTS: a variable-length prefix parse, a mis-padded TLV, or a lost sub-TLV region.
package packet

import (
	"bytes"
	"testing"
)

func TestExtPrefixTLVRoundTrip(t *testing.T) {
	in := ExtPrefixLSA{Prefixes: []ExtPrefixTLV{{
		RouteType:     ExtRouteTypeIntraArea,
		PrefixLength:  24,
		AF:            ExtPrefixAFIPv4Unicast,
		Flags:         ExtPrefixFlagN,
		AddressPrefix: [4]byte{10, 1, 2, 0},
		SubTLVs:       []ExtSubTLV{{Type: 9, Value: []byte{1, 2, 3}}},
	}}}
	body := EncodeExtPrefixLSA(in)
	if len(body)%4 != 0 {
		t.Fatalf("body length %d not 4-octet aligned", len(body))
	}
	out, err := DecodeExtPrefixLSA(body)
	// RFC requirement: RFC7684-5-1 positive -- a well-formed Extended Prefix body decodes without
	// error through the bound-checked decoder (DecodeExtPrefixLSA, packet/ext_prefix.go:153),
	// proving the malformed-detection bounds do not reject valid input.
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(out.Prefixes) != 1 {
		t.Fatalf("prefixes = %d, want 1", len(out.Prefixes))
	}
	p := out.Prefixes[0]
	if p.RouteType != ExtRouteTypeIntraArea || p.PrefixLength != 24 || p.AF != 0 || p.Flags != ExtPrefixFlagN {
		t.Fatalf("fixed fields = %+v", p)
	}
	if p.AddressPrefix != [4]byte{10, 1, 2, 0} {
		t.Fatalf("address prefix = %v", p.AddressPrefix)
	}
	if len(p.SubTLVs) != 1 || p.SubTLVs[0].Type != 9 || !bytes.Equal(p.SubTLVs[0].Value, []byte{1, 2, 3}) {
		t.Fatalf("sub-tlvs = %+v", p.SubTLVs)
	}
	// Re-encode must be byte-identical (idempotent, no lost padding).
	if again := EncodeExtPrefixLSA(out); !bytes.Equal(again, body) {
		t.Fatalf("re-encode mismatch\n got %x\nwant %x", again, body)
	}
}

func TestExtPrefixAddressPrefixFixed32(t *testing.T) {
	// For every Prefix Length 0..32 the AF=0 Address Prefix is a fixed 32-bit field and the
	// following sub-TLV parses at the same offset.
	for plen := range 33 {
		in := ExtPrefixLSA{Prefixes: []ExtPrefixTLV{{
			RouteType:     ExtRouteTypeIntraArea,
			PrefixLength:  uint8(plen),
			AF:            ExtPrefixAFIPv4Unicast,
			AddressPrefix: [4]byte{192, 0, 2, 0},
			SubTLVs:       []ExtSubTLV{{Type: 1, Value: []byte{0xaa, 0xbb, 0xcc, 0xdd}}},
		}}}
		out, err := DecodeExtPrefixLSA(EncodeExtPrefixLSA(in))
		if err != nil {
			t.Fatalf("plen %d decode: %v", plen, err)
		}
		p := out.Prefixes[0]
		if p.PrefixLength != uint8(plen) || p.AddressPrefix != [4]byte{192, 0, 2, 0} {
			t.Fatalf("plen %d: fields %+v", plen, p)
		}
		if len(p.SubTLVs) != 1 || !bytes.Equal(p.SubTLVs[0].Value, []byte{0xaa, 0xbb, 0xcc, 0xdd}) {
			t.Fatalf("plen %d: sub-tlv offset wrong: %+v", plen, p.SubTLVs)
		}
	}
}

func TestExtPrefixRangeTLVContainerRoundTrip(t *testing.T) {
	// RFC 8665 sec 4 Extended Prefix Range TLV carried as an opaque container: the value is
	// preserved byte-for-byte with no range field interpretation.
	rawValue := []byte{0x20, 0x00, 0x00, 0x05, 0xde, 0xad, 0xbe, 0xef}
	in := ExtPrefixLSA{Ranges: []ExtPrefixRangeTLV{{Value: rawValue}}}
	out, err := DecodeExtPrefixLSA(EncodeExtPrefixLSA(in))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(out.Ranges) != 1 || !bytes.Equal(out.Ranges[0].Value, rawValue) {
		t.Fatalf("range container not preserved: %+v", out.Ranges)
	}
	if len(out.Prefixes) != 0 {
		t.Fatalf("range TLV must not decode as a prefix TLV")
	}
}

func TestExtPrefixEmptyContainerRoundTrip(t *testing.T) {
	// AC-12/A-4: an Extended Prefix TLV with zero sub-TLVs round-trips byte-for-byte.
	in := ExtPrefixLSA{Prefixes: []ExtPrefixTLV{{
		RouteType:     ExtRouteTypeIntraArea,
		PrefixLength:  32,
		AF:            ExtPrefixAFIPv4Unicast,
		AddressPrefix: [4]byte{10, 0, 0, 1},
	}}}
	body := EncodeExtPrefixLSA(in)
	out, err := DecodeExtPrefixLSA(body)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(out.Prefixes) != 1 || len(out.Prefixes[0].SubTLVs) != 0 {
		t.Fatalf("empty container decode = %+v", out.Prefixes)
	}
	if again := EncodeExtPrefixLSA(out); !bytes.Equal(again, body) {
		t.Fatalf("empty container re-encode mismatch")
	}
	// The body is exactly the 4-byte TLV header + 8-byte fixed value = 12 octets.
	if len(body) != 12 {
		t.Fatalf("empty container body = %d octets, want 12", len(body))
	}
}
