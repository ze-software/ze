// VALIDATES: spec-ospfv3-1-types AC-1 -- Router ID / Area ID / Link State ID parse from
// dotted-quad (Area ID also from integer), round-trip to canonical text and big-endian
// bytes, and FromBytes enforces the exact 4-octet width.
// PREVENTS: identifier parse/format drift or a wrong-width wire read.
package types

import (
	"bytes"
	"errors"
	"testing"
)

func TestOSPFv3IdentifierParseFormat(t *testing.T) {
	rid, err := ParseRouterID("1.2.3.4")
	if err != nil {
		t.Fatalf("ParseRouterID: %v", err)
	}
	if rid.String() != "1.2.3.4" {
		t.Errorf("RouterID round-trip = %q, want 1.2.3.4", rid.String())
	}
	if rid != (RouterID{1, 2, 3, 4}) {
		t.Errorf("RouterID bytes = %v", rid)
	}

	// Area ID accepts dotted-quad and a plain integer; both render canonically.
	for _, in := range []string{"0", "0.0.0.0"} {
		a, err := ParseAreaID(in)
		if err != nil {
			t.Fatalf("ParseAreaID(%q): %v", in, err)
		}
		if a.String() != "0.0.0.0" {
			t.Errorf("AreaID(%q) = %q, want 0.0.0.0", in, a.String())
		}
	}
	a, err := ParseAreaID("16")
	if err != nil {
		t.Fatalf("ParseAreaID(16): %v", err)
	}
	if a.String() != "0.0.0.16" {
		t.Errorf("AreaID(16) = %q, want 0.0.0.16", a.String())
	}

	lsid, err := LinkStateIDFromBytes([]byte{255, 255, 255, 255})
	if err != nil {
		t.Fatalf("LinkStateIDFromBytes: %v", err)
	}
	if lsid.String() != "255.255.255.255" {
		t.Errorf("LinkStateID = %q", lsid.String())
	}

	// Width enforcement.
	if _, err := RouterIDFromBytes([]byte{1, 2, 3}); err == nil {
		t.Error("RouterIDFromBytes accepted 3 bytes")
	}
	if _, err := RouterIDFromBytes([]byte{1, 2, 3, 4, 5}); err == nil {
		t.Error("RouterIDFromBytes accepted 5 bytes")
	}
	if _, err := ParseRouterID("1.2.3"); err == nil {
		t.Error("ParseRouterID accepted a 3-octet string")
	}
	if _, err := ParseRouterID("1.2.3.256"); err == nil {
		t.Error("ParseRouterID accepted an out-of-range octet")
	}

	// WriteTo emits big-endian octets into a caller buffer.
	buf := make([]byte, 4)
	if n := rid.WriteTo(buf, 0); n != 4 {
		t.Fatalf("WriteTo returned %d, want 4", n)
	}
	if buf[0] != 1 || buf[3] != 4 {
		t.Errorf("WriteTo bytes = %v", buf)
	}
}

// VALIDATES: spec-ospfv3-1-types AC-1 -- Bytes returns a fresh (non-aliasing) copy, AppendTo
// renders canonical dotted-quad text, and Compare orders identifiers by big-endian octets.
// PREVENTS: a caller mutating a returned slice through the identifier's storage, format drift,
// or an ordering that ignores a low-order octet.
func TestOSPFv3IdentifierAccessors(t *testing.T) {
	rid := RouterID{10, 0, 0, 1}
	area := AreaID{0, 0, 0, 5}
	lsid := LinkStateID{192, 0, 2, 7}

	// Bytes returns a copy: mutating it must not change the identifier.
	rb := rid.Bytes()
	rb[0] = 0xff
	if rid[0] != 10 {
		t.Errorf("RouterID.Bytes aliased storage")
	}
	if !bytes.Equal(area.Bytes(), []byte{0, 0, 0, 5}) {
		t.Errorf("AreaID.Bytes = %v, want [0 0 0 5]", area.Bytes())
	}
	if !bytes.Equal(lsid.Bytes(), []byte{192, 0, 2, 7}) {
		t.Errorf("LinkStateID.Bytes = %v, want [192 0 2 7]", lsid.Bytes())
	}

	// AppendTo renders canonical dotted-quad text without a leading allocation.
	if got := string(rid.AppendTo(nil)); got != "10.0.0.1" {
		t.Errorf("RouterID.AppendTo = %q, want 10.0.0.1", got)
	}
	if got := string(area.AppendTo(nil)); got != "0.0.0.5" {
		t.Errorf("AreaID.AppendTo = %q, want 0.0.0.5", got)
	}
	if got := string(lsid.AppendTo(nil)); got != "192.0.2.7" {
		t.Errorf("LinkStateID.AppendTo = %q, want 192.0.2.7", got)
	}

	// Compare orders by big-endian octets: less, greater, equal.
	if rid.Compare(RouterID{10, 0, 0, 2}) != -1 {
		t.Errorf("RouterID.Compare(smaller-than-arg) != -1")
	}
	if rid.Compare(RouterID{10, 0, 0, 0}) != 1 {
		t.Errorf("RouterID.Compare(greater-than-arg) != 1")
	}
	if rid.Compare(RouterID{10, 0, 0, 1}) != 0 {
		t.Errorf("RouterID.Compare(equal) != 0")
	}
	if area.Compare(AreaID{0, 0, 0, 6}) != -1 || area.Compare(AreaID{0, 0, 0, 4}) != 1 {
		t.Errorf("AreaID.Compare ordering wrong")
	}
	if lsid.Compare(LinkStateID{192, 0, 2, 8}) != -1 || lsid.Compare(LinkStateID{192, 0, 2, 7}) != 0 {
		t.Errorf("LinkStateID.Compare ordering wrong")
	}

	// LinkStateID.WriteTo emits big-endian octets at the offset.
	wb := make([]byte, 6)
	if n := lsid.WriteTo(wb, 1); n != 4 || !bytes.Equal(wb, []byte{0, 192, 0, 2, 7, 0}) {
		t.Errorf("LinkStateID.WriteTo n=%d bytes=%v", n, wb)
	}
}

// VALIDATES: spec-ospfv3-1-types AC-1 -- AreaID.IsBackbone matches the all-zero area,
// AreaIDFromBytes enforces the 4-octet width, and ParseLinkStateID parses dotted-quad text.
// PREVENTS: a non-zero area reporting as backbone, a wrong-width Area ID decode, or a
// Link State ID parse that accepts malformed text.
func TestOSPFv3AreaBackboneAndFromBytes(t *testing.T) {
	if !BackboneArea.IsBackbone() {
		t.Errorf("BackboneArea.IsBackbone() = false, want true")
	}
	if (AreaID{0, 0, 0, 1}).IsBackbone() {
		t.Errorf("non-zero AreaID reported IsBackbone")
	}

	area, err := AreaIDFromBytes([]byte{0, 0, 0, 9})
	if err != nil {
		t.Fatalf("AreaIDFromBytes returned error: %v", err)
	}
	if area != (AreaID{0, 0, 0, 9}) {
		t.Errorf("AreaIDFromBytes = %v, want [0 0 0 9]", area)
	}
	for _, bad := range [][]byte{{1, 2, 3}, {1, 2, 3, 4, 5}} {
		if _, err := AreaIDFromBytes(bad); !errors.Is(err, ErrWrongLength) {
			t.Errorf("AreaIDFromBytes(%v) err = %v, want ErrWrongLength", bad, err)
		}
	}

	lsid, err := ParseLinkStateID("255.255.255.255")
	if err != nil {
		t.Fatalf("ParseLinkStateID returned error: %v", err)
	}
	if lsid != (LinkStateID{255, 255, 255, 255}) {
		t.Errorf("ParseLinkStateID = %v, want [255 255 255 255]", lsid)
	}
	if _, err := ParseLinkStateID("1.2.3"); err == nil {
		t.Errorf("ParseLinkStateID accepted malformed text")
	}
}
