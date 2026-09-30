// VALIDATES: the AIGP TLV is Type 1, Length 11 with an 8-octet value, the
// attribute value is a whole set of TLVs, and other TLVs survive the codec
// (RFC 7311 Section 3).
// PREVENTS: accepting an AIGP TLV of the wrong length or stray trailing octets,
// and a codec that drops TLVs after the first.

package attribute

import (
	"bytes"
	"testing"
)

// TestRFC7311AIGPTLVLengthIsEleven proves the AIGP TLV shape both ways: Ze
// writes Type 1, Length 11 and an 8-octet value, and refuses a Type-1 TLV
// whose Length is anything else, above 11 as well as below.
// Method: encode through NewAIGPMetric and WriteTo, then decode single-TLV
// buffers of Length 10, 11 and 12 through ParseAIGP, the registered parser.
//
// RFC requirement: RFC7311-3-1 positive -- Ze writes Type 1, Length 11 and the 8-octet metric, and parses that TLV to the same metric.
// RFC requirement: RFC7311-3-1 negative -- a Type-1 TLV of Length 12 (9 value octets) or Length 10 (7 value octets) is refused.
func TestRFC7311AIGPTLVLengthIsEleven(t *testing.T) {
	var buf [11]byte
	if n := NewAIGPMetric(0x0102030405060708).WriteTo(buf[:], 0); n != 11 {
		t.Fatalf("WriteTo wrote %d octets, want 11", n)
	}
	want := []byte{1, 0, 11, 1, 2, 3, 4, 5, 6, 7, 8}
	if !bytes.Equal(buf[:], want) {
		t.Fatalf("encoded TLV %x, want %x", buf[:], want)
	}
	parsed, err := ParseAIGP(want)
	if err != nil {
		t.Fatalf("Length 11 refused: %v", err)
	}
	if metric, ok := parsed.Metric(); !ok || metric != 0x0102030405060708 {
		t.Fatalf("metric %x (present %v), want 0102030405060708", metric, ok)
	}
	for _, tc := range []struct {
		name string
		data []byte
	}{
		{"length-12", []byte{1, 0, 12, 0, 0, 0, 0, 0, 0, 0, 100, 0}},
		{"length-10", []byte{1, 0, 10, 0, 0, 0, 0, 0, 0, 100}},
	} {
		if _, err := ParseAIGP(tc.data); err == nil {
			t.Errorf("%s: a Type-1 TLV whose Length is not 11 was accepted", tc.name)
		}
	}
}

// TestRFC7311AIGPValueIsWholeTLVSet proves the attribute value is a set of
// TLVs and nothing else: two TLVs that consume the value exactly parse, and a
// value with octets after the last TLV that do not form a TLV is refused.
// Method: ParseAIGP over a two-TLV value, then the same value followed by one,
// two and three stray octets (the last a header whose Length is below 3).
//
// RFC requirement: RFC7311-3-2 positive -- a value made of two whole TLVs parses into exactly those two TLVs.
// RFC requirement: RFC7311-3-2 negative -- one, two or three trailing octets that do not form a TLV make the value refused.
func TestRFC7311AIGPValueIsWholeTLVSet(t *testing.T) {
	value := []byte{1, 0, 11, 0, 0, 0, 0, 0, 0, 0, 100, 9, 0, 4, 0xee}
	parsed, err := ParseAIGP(value)
	if err != nil {
		t.Fatalf("a whole TLV set was refused: %v", err)
	}
	if len(parsed.TLVs) != 2 {
		t.Fatalf("parsed %d TLVs, want 2", len(parsed.TLVs))
	}
	if parsed.TLVs[1].Type != 9 || !bytes.Equal(parsed.TLVs[1].Data, []byte{0xee}) {
		t.Fatalf("second TLV %d %x, want 9 ee", parsed.TLVs[1].Type, parsed.TLVs[1].Data)
	}
	for _, trailing := range [][]byte{{0}, {9, 0}, {9, 0, 2}} {
		data := append(bytes.Clone(value), trailing...)
		if _, err := ParseAIGP(data); err == nil {
			t.Errorf("trailing %x after the last TLV was accepted", trailing)
		}
	}
}

// TestRFC7311OtherAIGPTLVsSurviveTheCodec proves the codec half of passing the
// other AIGP TLVs along: a second Type-1 TLV and an unknown TLV come back out
// of ParseAIGP and WriteTo octet for octet, and only the first TLV is read as
// the metric. The forwarding rail half is
// internal/component/bgp/reactor/rfc7311_aigp_other_tlvs_test.go.
//
// RFC requirement: RFC7311-3-3 positive -- the second Type-1 TLV and an unknown TLV survive parse and write unchanged.
func TestRFC7311OtherAIGPTLVsSurviveTheCodec(t *testing.T) {
	value := []byte{
		1, 0, 11, 0, 0, 0, 0, 0, 0, 0, 100,
		1, 0, 11, 0, 0, 0, 0, 0, 0, 0, 7,
		9, 0, 5, 0xab, 0xcd,
	}
	parsed, err := ParseAIGP(value)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if metric, ok := parsed.Metric(); !ok || metric != 100 {
		t.Fatalf("metric %d (present %v), want the first TLV's 100", metric, ok)
	}
	out := make([]byte, parsed.Len())
	if n := parsed.WriteTo(out, 0); n != len(value) {
		t.Fatalf("WriteTo wrote %d octets, want %d", n, len(value))
	}
	if !bytes.Equal(out, value) {
		t.Fatalf("written %x, want %x", out, value)
	}
}
