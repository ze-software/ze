// VALIDATES: spec-ospf-ext-9 A-5, R-8 -- the OSPFv3 Grace-LSA tlv codec round-trips a
// 1-byte value with 4-octet padding (the Reason tlv pitfall) and never panics on malformed
// input (truncated header, over-length value).
// PREVENTS: a decoder advancing by the raw Length rather than the padded length, and an
// out-of-bounds read on attacker-controlled tlv lengths.

package packet

import "testing"

func TestGraceLSAv6TLVRoundTrip(t *testing.T) {
	tlvs := []tlv{
		{Type: 1, Value: []byte{0x00, 0x00, 0x00, 0x78}}, // 4-byte value (Grace Period 120)
		{Type: 2, Value: []byte{0x03}},                   // 1-byte value (Reason), pads to 4 octets
	}
	// Each tlv is 4 (header) + 4 (padded value) = 8 octets.
	want := tlvsEncodedLen(tlvs)
	if want != 16 {
		t.Fatalf("tlvsEncodedLen = %d, want 16", want)
	}
	buf := make([]byte, want)
	if n := writeTLVs(buf, tlvs); n != want {
		t.Fatalf("writeTLVs wrote %d, want %d", n, want)
	}
	// The Reason tlv declares Length 1 (not 4): the pad is not counted in the Length field.
	if got := readUint16(buf, 8+2); got != 1 {
		t.Fatalf("Reason tlv on-wire Length = %d, want 1 (pad not counted)", got)
	}
	it := newTLVIterator(buf)
	var got []tlv
	for it.Next() {
		got = append(got, tlv{Type: it.Type(), Value: append([]byte(nil), it.Value()...)})
	}
	if it.Err() != nil {
		t.Fatalf("iterator error: %v", it.Err())
	}
	if len(got) != 2 {
		t.Fatalf("iterated %d TLVs, want 2", len(got))
	}
	if got[0].Type != 1 || len(got[0].Value) != 4 || got[0].Value[3] != 0x78 {
		t.Fatalf("tlv[0] = %+v, want type 1 value 4 bytes ending 0x78", got[0])
	}
	if got[1].Type != 2 || len(got[1].Value) != 1 || got[1].Value[0] != 0x03 {
		t.Fatalf("tlv[1] = %+v, want type 2 value {0x03}", got[1])
	}
}

func TestGraceLSAv6TLVIteratorMalformed(t *testing.T) {
	cases := map[string][]byte{
		"truncated-header":   {0x00, 0x01, 0x00}, // only 3 bytes, header needs 4
		"length-past-region": {0x00, 0x01, 0x00, 0x08, 0x01, 0x02},
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			it := newTLVIterator(body)
			for it.Next() { // must not panic
			}
			if it.Err() == nil {
				t.Fatalf("%s: iterator accepted malformed body, want error", name)
			}
		})
	}
}

func TestGraceLSAv6TLVIteratorUnpaddedFinalTolerated(t *testing.T) {
	// A final tlv whose value fits but whose 4-octet pad is missing is TOLERATED on receive
	// (be liberal), consistent with the OSPFv2 SR sub-TLV iterator. Type 2, Length 1, value
	// {0x03}, no pad bytes: it decodes to exactly one tlv with no error.
	it := newTLVIterator([]byte{0x00, 0x02, 0x00, 0x01, 0x03})
	var got []tlv
	for it.Next() {
		got = append(got, tlv{Type: it.Type(), Value: append([]byte(nil), it.Value()...)})
	}
	if it.Err() != nil {
		t.Fatalf("unpadded final tlv must be tolerated, got err %v", it.Err())
	}
	if len(got) != 1 || got[0].Type != 2 || len(got[0].Value) != 1 || got[0].Value[0] != 0x03 {
		t.Fatalf("unpadded final tlv = %+v, want one tlv type 2 value {0x03}", got)
	}
}
