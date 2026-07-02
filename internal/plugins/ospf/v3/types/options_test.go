// VALIDATES: spec-ospfv3-1-types AC-9 -- the OSPFv3 Options field is a 24-bit bitset that
// round-trips through 3 wire octets and the common bit predicates (V6/E/R/N) decode
// correctly.
// PREVENTS: truncating OSPFv3's 24-bit Options to the OSPFv2 8-bit width.
package types

import "testing"

func TestOSPFv3Options24BitRoundTrip(t *testing.T) {
	o := Options(0xABCDEF)

	buf := make([]byte, 3)
	if n := o.WriteTo(buf, 0); n != 3 {
		t.Fatalf("WriteTo = %d, want 3", n)
	}
	if buf[0] != 0xAB || buf[1] != 0xCD || buf[2] != 0xEF {
		t.Errorf("WriteTo bytes = %v", buf)
	}
	back, err := OptionsFromBytes(buf, 0)
	if err != nil || back != o {
		t.Fatalf("OptionsFromBytes = %#06x, %v", uint32(back), err)
	}
	if _, err := OptionsFromBytes([]byte{1, 2}, 0); err == nil {
		t.Error("OptionsFromBytes accepted < 3 bytes")
	}

	// Bit predicates: V6 (0x01), E (0x02), R (0x10), N (0x08).
	all := OptV6 | OptE | OptR | OptN
	if !all.V6() || !all.External() || !all.Router() || !all.NSSA() {
		t.Errorf("option predicates: %#06x", uint32(all))
	}
	if Options(0).V6() {
		t.Error("empty options reported V6")
	}
}

// TestAFBitDistinct pins the RFC 5838 §2.4 AF-bit at a position that does not alias
// V6/E/N/R, and checks the AF()/SetAF round-trip and the 3-octet wire carriage.
func TestAFBitDistinct(t *testing.T) {
	if OptAF&(OptV6|OptE|OptN|OptR) != 0 {
		t.Fatalf("OptAF %#06x aliases a V6/E/N/R bit", uint32(OptAF))
	}
	if OptAF != 0x000100 {
		t.Fatalf("OptAF = %#06x, want 0x000100 (IANA OSPFv3 Options bit 8)", uint32(OptAF))
	}
	// SetAF adds the bit without disturbing others; AF() reports it.
	base := OptV6 | OptR
	withAF := base.SetAF()
	if !withAF.AF() {
		t.Error("SetAF did not set the AF-bit")
	}
	if base.AF() {
		t.Error("base options without SetAF reported AF")
	}
	if withAF&base != base {
		t.Error("SetAF disturbed the pre-existing bits")
	}
	// The AF-bit survives the 3-octet wire round-trip (it lives in the middle octet).
	buf := make([]byte, 3)
	withAF.WriteTo(buf, 0)
	back, err := OptionsFromBytes(buf, 0)
	if err != nil || !back.AF() {
		t.Fatalf("AF-bit lost on wire round-trip: %#06x, %v", uint32(back), err)
	}
}
