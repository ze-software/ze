// VALIDATES: spec-ospfv3-1-types AC-1 -- Router ID / Area ID / Link State ID parse from
// dotted-quad (Area ID also from integer), round-trip to canonical text and big-endian
// bytes, and FromBytes enforces the exact 4-octet width.
// PREVENTS: identifier parse/format drift or a wrong-width wire read.
package types

import "testing"

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
