// VALIDATES: spec-ospf-ext-1 AC-13/R-5/R-8 -- the generic opaque-LSA TLV builder pads
// every value to a 4-byte boundary and the zero-copy iterator reads type+value back
// exactly, reports the total padded length, and never panics on malformed input.
// PREVENTS: a mis-padded TLV body that self-round-trips but is rejected by FRR, and an
// out-of-range slice panic on a truncated or over-long TLV from an untrusted peer.
package packet

import (
	"bytes"
	"testing"
)

// TestOpaqueTLVAlignment pins AC-13 / R-5: the TLV builder 4-byte-aligns every value
// (lengths 0..7), the iterator reads the exact type+value back, and the encoded length
// accounts for the pad bytes.
func TestOpaqueTLVAlignment(t *testing.T) {
	for valLen := range 8 {
		value := make([]byte, valLen)
		for i := range value {
			value[i] = byte(0xA0 + i)
		}
		tlv := opaqueTLV{Type: uint16(0x1000 + valLen), Value: value}

		wantPadded := (valLen + 3) &^ 3
		wantLen := OpaqueTLVHeaderLen + wantPadded
		if got := tlv.EncodedLen(); got != wantLen {
			t.Fatalf("valLen %d EncodedLen = %d, want %d", valLen, got, wantLen)
		}

		buf := make([]byte, tlv.EncodedLen())
		end := tlv.WriteTo(buf, 0)
		if end != wantLen {
			t.Fatalf("valLen %d WriteTo end = %d, want %d", valLen, end, wantLen)
		}
		// The whole encoded length is a 4-byte multiple.
		if end%4 != 0 {
			t.Fatalf("valLen %d encoded length %d not 4-byte aligned", valLen, end)
		}
		// Pad bytes are zero.
		for i := OpaqueTLVHeaderLen + valLen; i < end; i++ {
			if buf[i] != 0 {
				t.Fatalf("valLen %d pad byte %d = %#x, want 0", valLen, i, buf[i])
			}
		}

		it := newOpaqueTLVIterator(buf)
		if !it.Next() {
			t.Fatalf("valLen %d iterator produced no TLV (err=%v)", valLen, it.Err())
		}
		if it.Type() != tlv.Type {
			t.Fatalf("valLen %d type = %#x, want %#x", valLen, it.Type(), tlv.Type)
		}
		if !bytes.Equal(it.Value(), value) {
			t.Fatalf("valLen %d value = % x, want % x", valLen, it.Value(), value)
		}
		if it.Next() {
			t.Fatalf("valLen %d iterator produced a spurious second TLV", valLen)
		}
		if it.Err() != nil {
			t.Fatalf("valLen %d iterator error = %v", valLen, it.Err())
		}
	}
}

// TestOpaqueTLVMultipleRoundTrip writes several TLVs and reads them all back in order.
func TestOpaqueTLVMultipleRoundTrip(t *testing.T) {
	tlvs := []opaqueTLV{
		{Type: 1, Value: []byte{0x01}},
		{Type: 2, Value: []byte{0x02, 0x03, 0x04, 0x05, 0x06}},
		{Type: 3, Value: nil},
		{Type: 4, Value: []byte{0x07, 0x08}},
	}
	buf := make([]byte, opaqueTLVsLen(tlvs))
	if end := writeOpaqueTLVs(buf, tlvs); end != len(buf) {
		t.Fatalf("writeOpaqueTLVs end = %d, want %d", end, len(buf))
	}
	it := newOpaqueTLVIterator(buf)
	got := 0
	for it.Next() {
		if it.Type() != tlvs[got].Type {
			t.Fatalf("tlv %d type = %#x, want %#x", got, it.Type(), tlvs[got].Type)
		}
		if !bytes.Equal(it.Value(), tlvs[got].Value) {
			t.Fatalf("tlv %d value = % x, want % x", got, it.Value(), tlvs[got].Value)
		}
		got++
	}
	if it.Err() != nil {
		t.Fatalf("iterator error = %v", it.Err())
	}
	if got != len(tlvs) {
		t.Fatalf("read %d TLVs, want %d", got, len(tlvs))
	}
}

// VALIDATES: spec-ospf-ext-1 AC-3 -- DecodeOpaqueTLVs walks a well-formed opaque body and
// returns each (type, declared length, zero-copy value) view in order.
// PREVENTS: the generic ext-14 debug fallback dropping, reordering, or mis-lengthing a TLV.
func TestDecodeOpaqueTLVsRoundTrip(t *testing.T) {
	tlvs := []opaqueTLV{
		{Type: 0x0001, Value: []byte{0xaa}},
		{Type: 0x0002, Value: []byte{0xbb, 0xcc, 0xdd, 0xee, 0xff}},
		{Type: 0x0003, Value: nil},
	}
	body := make([]byte, opaqueTLVsLen(tlvs))
	writeOpaqueTLVs(body, tlvs)

	views, err := DecodeOpaqueTLVs(body)
	if err != nil {
		t.Fatalf("DecodeOpaqueTLVs returned error: %v", err)
	}
	if len(views) != len(tlvs) {
		t.Fatalf("decoded %d TLVs, want %d", len(views), len(tlvs))
	}
	for i, v := range views {
		if v.Type != tlvs[i].Type {
			t.Errorf("tlv %d type = %#x, want %#x", i, v.Type, tlvs[i].Type)
		}
		if v.Length != len(tlvs[i].Value) {
			t.Errorf("tlv %d length = %d, want %d", i, v.Length, len(tlvs[i].Value))
		}
		if !bytes.Equal(v.Value, tlvs[i].Value) {
			t.Errorf("tlv %d value = % x, want % x", i, v.Value, tlvs[i].Value)
		}
	}
}

// VALIDATES: spec-ospf-ext-1 AC-3 -- on a malformed body DecodeOpaqueTLVs returns the good
// prefix decoded before the fault PLUS a non-nil error, and never panics.
// PREVENTS: a truncated peer body aborting the whole render or reading out of bounds.
func TestDecodeOpaqueTLVsMalformedReturnsPrefix(t *testing.T) {
	good := []opaqueTLV{{Type: 0x0001, Value: []byte{0x01, 0x02, 0x03, 0x04}}}
	body := make([]byte, opaqueTLVsLen(good))
	writeOpaqueTLVs(body, good)
	// Append a truncated second TLV header (says a 16-byte value, none present).
	body = append(body, 0x00, 0x02, 0x00, 0x10)

	views, err := DecodeOpaqueTLVs(body)
	if err == nil {
		t.Fatalf("DecodeOpaqueTLVs on truncated body returned nil error")
	}
	if len(views) != 1 || views[0].Type != 0x0001 {
		t.Fatalf("expected the one good TLV before the fault, got %+v", views)
	}
}

// TestOpaqueTLVIteratorMalformed pins R-8: truncated header, over-length value, and a
// missing pad each stop iteration with an error and never panic.
func TestOpaqueTLVIteratorMalformed(t *testing.T) {
	cases := map[string][]byte{
		"truncated-header": {0x00, 0x01, 0x00},                   // 3 bytes < 4-byte header
		"length-past-end":  {0x00, 0x01, 0x00, 0x10, 0xaa, 0xbb}, // says 16-byte value, only 2 present
		"missing-pad":      {0x00, 0x01, 0x00, 0x02, 0xaa, 0xbb}, // value fits but pad byte absent
	}
	for name, body := range cases {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("%s: iterator panicked: %v", name, r)
				}
			}()
			it := newOpaqueTLVIterator(body)
			for it.Next() {
			}
			if it.Err() == nil {
				t.Fatalf("%s: expected an iterator error, got nil", name)
			}
		}()
	}
}
