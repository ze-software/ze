package mup

import (
	"bytes"
	"testing"
)

// t1stWire is a 3gpp-5G Type 1 ST NLRI: RD 0:0, prefix 10.0.0.1/32, TEID 1, QFI 9,
// endpoint 192.0.2.1/32, then tail (the Source Address Length and what follows).
func t1stWire(tail ...byte) []byte {
	body := make([]byte, 8)
	body = append(body, 32, 10, 0, 0, 1, 0, 0, 0, 1, 9, 32, 192, 0, 2, 1)
	body = append(body, tail...)
	return append([]byte{byte(MUPArch3GPP5G), 0, byte(MUPT1ST), byte(len(body))}, body...)
}

// TestCanonicalT1STGrow covers which routes the canonical form rewrites.
//
// VALIDATES: only a 3gpp-5G Type 1 ST route ending at its Endpoint Address grows,
// by one octet, with or without an ADD-PATH path identifier; a current-form route,
// another route type, a Length that disagrees with the octets and a short NLRI do not.
// PREVENTS: a route in the current form being rewritten, or a peer's short NLRI
// reading past its end.
func TestCanonicalT1STGrow(t *testing.T) {
	old := t1stWire()
	isd := append([]byte{byte(MUPArch3GPP5G), 0, byte(MUPISD), 13}, make([]byte, 8)...)
	isd = append(isd, 32, 10, 0, 0, 1)
	lengthLies := append([]byte{}, old...)
	lengthLies[3]++
	cases := []struct {
		name    string
		nlri    []byte
		addPath bool
		grow    int
	}{
		{"-02 route", old, false, 1},
		{"-02 route with path id", append([]byte{0, 0, 0, 7}, old...), true, 1},
		{"current form", t1stWire(0x00), false, 0},
		{"current form with a TLV", t1stWire(0x00, 0xf0, 0x00), false, 0},
		{"ISD route", isd, false, 0},
		{"Length disagrees", lengthLies, false, 0},
		{"header only", old[:mupHeaderLen], false, 0},
		{"short path id", []byte{0, 0}, true, 0},
		{"empty", nil, false, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := canonicalT1STGrow(tc.nlri, tc.addPath); got != tc.grow {
				t.Fatalf("canonicalT1STGrow(%x) = %d, want %d", tc.nlri, got, tc.grow)
			}
		})
	}
}

// TestWriteCanonicalT1ST proves the rewrite writes the current encoding.
//
// VALIDATES: draft-ietf-bess-mup-safi Section 3.1.3.1, "The Source Address Length has
// a fixed length of 1 octet.": the -02 route is written with the Length one larger
// and a Source Address Length of 0, at the offset asked, the path identifier kept.
// PREVENTS: a relayed -02 route leaving without the octet ExaBGP sends.
func TestWriteCanonicalT1ST(t *testing.T) {
	for _, addPath := range []bool{false, true} {
		in, want := t1stWire(), t1stWire(0x00)
		if addPath {
			in = append([]byte{0, 0, 0, 7}, in...)
			want = append([]byte{0, 0, 0, 7}, want...)
		}
		buf := make([]byte, 2+len(want))
		n := writeCanonicalT1ST(buf, 2, in, addPath)
		if n != len(want) {
			t.Fatalf("addPath %v: wrote %d octets, want %d", addPath, n, len(want))
		}
		if !bytes.Equal(buf[2:], want) {
			t.Fatalf("addPath %v: wrote %x, want %x", addPath, buf[2:], want)
		}
		if _, _, err := ParseMUP(AFIIPv4, want[len(want)-len(t1stWire(0x00)):]); err != nil {
			t.Fatalf("the rewritten route does not parse: %v", err)
		}
	}
}
