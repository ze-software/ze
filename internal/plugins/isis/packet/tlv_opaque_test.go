// Design: plan/spec-isis-2-wire.md -- unknown-TLV passthrough tests
package packet

import (
	"bytes"
	"testing"
)

// VALIDATES: AC-5, R-6 -- a PDU TLV region containing a TLV type the codec does
// not recognize is retained as an opaque span and re-encoded byte-for-byte
// identical to the input. This is the invariant the LSDB relies on to re-flood
// LSPs carrying TLVs Ze does not understand (ISO/IEC 10589 clause 7.3.14).
// PREVENTS: dropping or mangling unknown TLVs on re-flood, which corrupts the
// link-state database of every downstream router.
func TestISISUnknownTLVPassthrough(t *testing.T) {
	// A region mixing a known TLV (1, area) with two unknown types (199, 250).
	region := []byte{
		1, 3, 0x49, 0x00, 0x01, // TLV 1, len 3
		199, 4, 0xde, 0xad, 0xbe, 0xef, // unknown TLV 199, len 4
		250, 0, // unknown TLV 250, len 0 (empty value)
		137, 2, 'h', 'i', // TLV 137 hostname "hi"
	}
	tlvs, err := DecodeTLVs(region)
	if err != nil {
		t.Fatalf("DecodeTLVs: %v", err)
	}
	if len(tlvs) != 4 {
		t.Fatalf("got %d TLVs, want 4", len(tlvs))
	}
	wantTypes := []uint8{1, 199, 250, 137}
	for i, want := range wantTypes {
		if tlvs[i].Type != want {
			t.Errorf("tlv[%d].Type = %d, want %d", i, tlvs[i].Type, want)
		}
	}
	// The empty-value TLV must round-trip as a zero-length value, not nil-drop.
	if len(tlvs[2].Value) != 0 {
		t.Errorf("tlv[2] (type 250) value len = %d, want 0", len(tlvs[2].Value))
	}

	// Re-encode and compare byte-for-byte.
	out := make([]byte, len(region))
	n := writeTLVs(out, 0, tlvs)
	if n != len(region) {
		t.Fatalf("re-encoded %d bytes, want %d", n, len(region))
	}
	if !bytes.Equal(out, region) {
		t.Fatalf("re-encode mismatch:\n got % x\nwant % x", out, region)
	}
}

// VALIDATES: the opaque carrier copies its value on demand so retained TLVs do
// not dangle when the source buffer is recycled (decode lifetime contract).
// PREVENTS: a use-after-recycle bug in the LSDB.
func TestISISTLVCopyValue(t *testing.T) {
	src := []byte{0xaa, 0xbb, 0xcc}
	tlv := TLV{Type: 199, Value: src}
	cp := tlv.CopyValue()
	src[0] = 0x00 // mutate the original backing array
	if cp.Value[0] != 0xaa {
		t.Errorf("CopyValue did not detach from source: got %#02x", cp.Value[0])
	}
	if cp.Type != 199 {
		t.Errorf("CopyValue lost type: %d", cp.Type)
	}
}

// VALIDATES: AC-11, R-3 -- the iterator stops cleanly (no panic, ErrTruncated)
// when a TLV's declared length runs past the end of the region, and yields the
// well-formed TLVs that precede the truncation.
// PREVENTS: a slice-out-of-range panic on crafted or corrupted wire input.
func TestISISTLVIteratorTruncated(t *testing.T) {
	cases := []struct {
		name      string
		region    []byte
		wantCount int
		wantErr   bool
	}{
		{
			name:      "clean",
			region:    []byte{1, 2, 0xaa, 0xbb, 8, 0},
			wantCount: 2,
			wantErr:   false,
		},
		{
			name:      "length-overruns",
			region:    []byte{1, 2, 0xaa, 0xbb, 9, 10, 0x01}, // TLV 9 claims 10 bytes, only 1 present
			wantCount: 1,
			wantErr:   true,
		},
		{
			name:      "trailing-type-only",
			region:    []byte{1, 1, 0xaa, 22}, // a lone type octet with no length
			wantCount: 1,
			wantErr:   true,
		},
		{
			name:      "empty",
			region:    nil,
			wantCount: 0,
			wantErr:   false,
		},
		{
			name:      "single-empty-tlv",
			region:    []byte{250, 0},
			wantCount: 1,
			wantErr:   false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			it := NewTLVIterator(tc.region)
			count := 0
			for {
				_, _, ok := it.Next()
				if !ok {
					break
				}
				count++
			}
			if count != tc.wantCount {
				t.Errorf("yielded %d TLVs, want %d", count, tc.wantCount)
			}
			if (it.Err() != nil) != tc.wantErr {
				t.Errorf("Err() = %v, wantErr = %v", it.Err(), tc.wantErr)
			}
		})
	}
}

// VALIDATES: AuthTLVIndex reports the position of the first TLV 10 so isis-10
// can enforce RFC 5304 first-TLV ordering; -1 when absent.
// PREVENTS: silently losing the auth-TLV position the enforcement layer needs.
func TestISISAuthTLVIndex(t *testing.T) {
	none := []TLV{{Type: 1}, {Type: 137}}
	if got := AuthTLVIndex(none); got != -1 {
		t.Errorf("AuthTLVIndex(no auth) = %d, want -1", got)
	}
	first := []TLV{{Type: 10}, {Type: 1}}
	if got := AuthTLVIndex(first); got != 0 {
		t.Errorf("AuthTLVIndex(auth first) = %d, want 0", got)
	}
	middle := []TLV{{Type: 1}, {Type: 10}, {Type: 137}}
	if got := AuthTLVIndex(middle); got != 1 {
		t.Errorf("AuthTLVIndex(auth middle) = %d, want 1", got)
	}
}
