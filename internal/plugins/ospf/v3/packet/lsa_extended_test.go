// VALIDATES: spec-ospf-ext-5 AC-18/AC-19 -- the RFC 8362 Extended-LSA body codec
// round-trips top-level TLVs and nested sub-TLVs byte-for-byte, extracts sub-TLVs at
// a type's fixed-field offset, and never panics on malformed input.
// PREVENTS: an Extended-LSA that does not re-encode identically; a decoder that
// overruns a truncated TLV.
package packet

import (
	"bytes"
	"testing"
)

func TestExtendedLSABodyRoundTrip(t *testing.T) {
	// E-Router-LSA-like: one Router-Link TLV (type 1) whose value is 16 fixed bytes
	// plus one Adj-SID sub-TLV (type 5).
	adjSub := ExtendedTLV{Type: 5, Value: []byte{0x60, 1, 0, 0, 0, 0x40, 0x00}} // 7-byte value
	linkFixed := []byte{1, 0, 0, 10, 0, 0, 0, 5, 0, 0, 0, 6, 10, 0, 0, 2}       // 16 bytes
	linkValue := append(append([]byte{}, linkFixed...), encodeSubTLVs([]ExtendedTLV{adjSub})...)
	body := EncodeExtendedLSABody(ExtendedLSA{TLVs: []ExtendedTLV{{Type: 1, Value: linkValue}}})

	got, err := DecodeExtendedLSABody(body)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got.TLVs) != 1 || got.TLVs[0].Type != 1 {
		t.Fatalf("top-level TLV = %+v", got.TLVs)
	}
	// Re-encode is byte-for-byte identical (verbatim carriage).
	if !bytes.Equal(body, EncodeExtendedLSABody(got)) {
		t.Fatalf("re-encode not byte-identical")
	}
	// Extract sub-TLVs at the 16-byte fixed offset.
	subs, err := SubTLVsAt(got.TLVs[0].Value, 16)
	if err != nil {
		t.Fatalf("sub-TLVs: %v", err)
	}
	if len(subs) != 1 || subs[0].Type != 5 || !bytes.Equal(subs[0].Value, adjSub.Value) {
		t.Fatalf("Adj-SID sub-TLV not recovered: %+v", subs)
	}
}

// VALIDATES: spec-ospf-ext-5 AC-18 -- AppendSubTLVs builds an Extended-LSA TLV value that is
// the fixed-field prefix followed by the 4-octet-aligned framed sub-TLVs, and SubTLVsAt at the
// fixed length recovers exactly those sub-TLVs. With no sub-TLVs the value is just the fixed part.
// PREVENTS: a lost or misaligned sub-TLV region when an Extended TLV carries both fixed fields
// and nested sub-TLVs.
func TestAppendSubTLVs(t *testing.T) {
	fixed := []byte{1, 2, 3, 4, 5, 6, 7, 8} // 8-byte fixed prefix
	subs := []ExtendedTLV{
		{Type: 5, Value: []byte{0xaa, 0xbb, 0xcc}},
		{Type: 6, Value: []byte{0x11, 0x22, 0x33, 0x44}},
	}
	value := AppendSubTLVs(fixed, subs)

	// The fixed prefix is preserved verbatim at the front.
	if !bytes.Equal(value[:len(fixed)], fixed) {
		t.Fatalf("fixed prefix not preserved: got % x", value[:len(fixed)])
	}
	// The remainder must be the exact framed sub-TLV bytes.
	if !bytes.Equal(value[len(fixed):], encodeSubTLVs(subs)) {
		t.Fatalf("framed sub-TLV region mismatch")
	}
	// Parsing the sub-TLVs back at the fixed offset recovers them in order.
	got, err := SubTLVsAt(value, len(fixed))
	if err != nil {
		t.Fatalf("SubTLVsAt: %v", err)
	}
	if len(got) != len(subs) {
		t.Fatalf("recovered %d sub-TLVs, want %d", len(got), len(subs))
	}
	for i := range subs {
		if got[i].Type != subs[i].Type || !bytes.Equal(got[i].Value, subs[i].Value) {
			t.Errorf("sub-TLV %d = %+v, want %+v", i, got[i], subs[i])
		}
	}

	// With no sub-TLVs the value is exactly the fixed prefix.
	if only := AppendSubTLVs(fixed, nil); !bytes.Equal(only, fixed) {
		t.Errorf("AppendSubTLVs(fixed, nil) = % x, want the fixed prefix only", only)
	}
}

func TestExtendedLSABodyMultipleTLVs(t *testing.T) {
	body := EncodeExtendedLSABody(ExtendedLSA{TLVs: []ExtendedTLV{
		{Type: 1, Value: []byte{1, 2, 3}},
		{Type: 2, Value: []byte{4, 5, 6, 7, 8}},
	}})
	got, err := DecodeExtendedLSABody(body)
	if err != nil || len(got.TLVs) != 2 {
		t.Fatalf("decode = %+v,%v", got, err)
	}
	if got.TLVs[1].Type != 2 || len(got.TLVs[1].Value) != 5 {
		t.Fatalf("second TLV wrong: %+v", got.TLVs[1])
	}
}

func TestExtendedLSABodyMalformed(t *testing.T) {
	inputs := [][]byte{
		{0, 1},             // truncated header
		{0, 1, 0, 8, 1, 2}, // length 8 but only 2 value bytes
	}
	for i, in := range inputs {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("input %d panicked: %v", i, r)
				}
			}()
			if _, err := DecodeExtendedLSABody(in); err == nil {
				t.Fatalf("input %d should be malformed", i)
			}
		}()
	}
	// SubTLVsAt with a fixed length beyond the value is safe.
	if _, err := SubTLVsAt([]byte{1, 2, 3}, 16); err != nil {
		t.Fatalf("SubTLVsAt past end should return empty, not error: %v", err)
	}
}

func TestExtendedLSABodyUnpaddedFinalTolerated(t *testing.T) {
	// A final TLV whose value fits but whose 4-octet pad is missing is TOLERATED on receive
	// (be liberal; consistent with the OSPFv2 SR sub-TLV iterator). {0,1,0,3,1,2,3} is Type 1,
	// Length 3, value {1,2,3}, no pad: it decodes to exactly one TLV with no error.
	ext, err := DecodeExtendedLSABody([]byte{0, 1, 0, 3, 1, 2, 3})
	if err != nil {
		t.Fatalf("unpadded final TLV must be tolerated, got err %v", err)
	}
	if len(ext.TLVs) != 1 || ext.TLVs[0].Type != 1 || len(ext.TLVs[0].Value) != 3 {
		t.Fatalf("unpadded final TLV = %+v, want one TLV type 1 value 3 bytes", ext.TLVs)
	}
}
