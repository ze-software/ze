// VALIDATES: spec-ospf-ext-4 -- the RFC 7684 sec 2 4-octet TLV alignment for sub-TLV value
// lengths 0..7 (R-1) and the sec 5 malformed-LSA rules: an overrun TLV/sub-TLV or trailing
// data smaller than a TLV header returns an error and NEVER panics (AC-7/R-2).
// PREVENTS: a mis-padded TLV FRR would reject, or a crafted LSA crashing the decoder.
package packet

import (
	"bytes"
	"testing"
)

func TestExtTLVAlignment(t *testing.T) {
	// A sub-TLV value of every length 0..7 must round-trip through the Extended Prefix TLV
	// with the ext-1 pad-correct builder; the whole body stays 4-octet aligned.
	for n := range 8 {
		val := bytes.Repeat([]byte{0xa5}, n)
		in := ExtPrefixLSA{Prefixes: []ExtPrefixTLV{{
			RouteType:     ExtRouteTypeIntraArea,
			PrefixLength:  32,
			AddressPrefix: [4]byte{10, 0, 0, 1},
			SubTLVs:       []ExtSubTLV{{Type: 5, Value: val}},
		}}}
		body := EncodeExtPrefixLSA(in)
		if len(body)%4 != 0 {
			t.Fatalf("n=%d: body length %d not 4-octet aligned", n, len(body))
		}
		out, err := DecodeExtPrefixLSA(body)
		if err != nil {
			t.Fatalf("n=%d decode: %v", n, err)
		}
		if len(out.Prefixes[0].SubTLVs) != 1 || !bytes.Equal(out.Prefixes[0].SubTLVs[0].Value, val) {
			t.Fatalf("n=%d: sub-tlv value not preserved: %+v", n, out.Prefixes[0].SubTLVs)
		}
	}
}

func TestExtMalformedNotStored(t *testing.T) {
	cases := []struct {
		name string
		body []byte
	}{
		// Top-level TLV Length runs past the body (overrun).
		{"prefix-top-overrun", []byte{0x00, 0x01, 0x00, 0xff, 0x01, 0x20, 0x00, 0x00}},
		// Trailing data smaller than a TLV header (1 octet after a valid TLV).
		{"prefix-short-trailing", append(EncodeExtPrefixLSA(ExtPrefixLSA{Prefixes: []ExtPrefixTLV{{RouteType: 1, PrefixLength: 32, AddressPrefix: [4]byte{10, 0, 0, 1}}}}), 0x00)},
		// Extended Prefix TLV value shorter than its 8-octet fixed header.
		{"prefix-fixed-short", []byte{0x00, 0x01, 0x00, 0x04, 0x01, 0x20, 0x00, 0x00}},
		// A sub-TLV inside the Extended Prefix TLV overruns the TLV value.
		{"prefix-subtlv-overrun", extPrefixWithBadSubTLV()},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// The only contract is: an error, never a panic, never partial data.
			if _, err := DecodeExtPrefixLSA(c.body); err == nil {
				t.Fatalf("%s: expected malformed error, got nil", c.name)
			}
		})
	}
	// Extended Link malformed cases.
	linkCases := [][]byte{
		{0x00, 0x01, 0x00, 0xff, 0x01, 0x00, 0x00, 0x00}, // top-level overrun
		{0x00, 0x01, 0x00, 0x04, 0x01, 0x00, 0x00, 0x00}, // value shorter than 12-octet fixed
		append(EncodeExtLinkLSA(ExtLinkTLV{LinkType: 1, LinkID: [4]byte{2, 2, 2, 2}, LinkData: [4]byte{10, 0, 0, 1}}), 0x00, 0x00), // 2-octet trailing < header
	}
	for i, body := range linkCases {
		if _, err := DecodeExtLinkLSA(body); err == nil {
			t.Fatalf("link case %d: expected malformed error, got nil", i)
		}
	}
}

// extPrefixWithBadSubTLV builds an Extended Prefix Opaque LSA whose Extended Prefix TLV
// carries a sub-TLV whose Length overruns the enclosing TLV value.
func extPrefixWithBadSubTLV() []byte {
	// Top-level TLV: type 1, length 12 (8 fixed + a 4-octet sub-TLV header claiming a huge
	// value). Sub-TLV header: type 5, length 0xffff -> overruns.
	return []byte{
		0x00, 0x01, 0x00, 0x0c, // top TLV type 1, len 12
		0x01, 0x20, 0x00, 0x00, // route type/plen/af/flags
		0x0a, 0x00, 0x00, 0x01, // address prefix
		0x00, 0x05, 0xff, 0xff, // sub-TLV type 5, len 65535 (overruns)
	}
}
