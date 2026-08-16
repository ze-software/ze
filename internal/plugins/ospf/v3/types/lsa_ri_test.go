// VALIDATES: spec-ospf-ext-3 AC-3/AC-4, R-2 -- the OSPFv3 Router Information LSA wire types
// per flooding scope (RFC 7770 sec 2.2: function code 12, U-bit set): 0x800C link, 0xA00C
// area, 0xC00C AS. Each is Known(), IsRouterInformation(), carries the U-bit, and decodes to
// function code 12 and the correct scope.
// PREVENTS: a single hard-coded RI wire type (area flooded AS-wide or dropped), a cleared
// U-bit (a non-supporting router would confine an area/AS RI LSA to link-local scope), or
// Known() rejecting a peer's RI LSA.
package types

import "testing"

func TestRIv3LSTypePerScope(t *testing.T) {
	cases := []struct {
		name  string
		typ   LSType
		scope floodScope
	}{
		{"link", LSTypeRouterInformationLink, 0}, // S2S1 = 00
		{"area", LSTypeRouterInformationArea, 1}, // S2S1 = 01
		{"as", LSTypeRouterInformationAS, 2},     // S2S1 = 10
	}
	want := map[string]LSType{"link": 0x800C, "area": 0xA00C, "as": 0xC00C}
	for _, c := range cases {
		if c.typ != want[c.name] {
			t.Errorf("%s RI wire type = %#04x, want %#04x", c.name, uint16(c.typ), uint16(want[c.name]))
		}
		if !c.typ.Known() {
			t.Errorf("%s RI type %#04x not Known()", c.name, uint16(c.typ))
		}
		// the FunctionCode()/UBit()/IsRouterInformation() accessors were removed
		// (intra-package-only exports flagged by ze-validate); the SAME behavior is asserted
		// directly on the wire bits here -- function code 12 in the low 13 bits and the U-bit set.
		if uint16(c.typ)&0x1FFF != uint16(RIFunctionCode) {
			t.Errorf("%s RI function code = %#x, want %#x", c.name, uint16(c.typ)&0x1FFF, uint16(RIFunctionCode))
		}
		if uint16(c.typ)&0x8000 == 0 {
			t.Errorf("%s RI type %#04x U-bit not set (RFC 7770 sec 2.2)", c.name, uint16(c.typ))
		}
		if c.typ.Scope() != c.scope {
			t.Errorf("%s RI type %#04x Scope() = %d, want %d", c.name, uint16(c.typ), c.typ.Scope(), c.scope)
		}
	}
}

func TestRIv3RecognizedRegardlessOfUBit(t *testing.T) {
	// the IsRouterInformation() accessor was removed (intra-package-only export
	// flagged by ze-validate); Known() now folds the function-code RI check, so the SAME
	// behavior -- RI recognized regardless of the U-bit, a non-RI/non-base type rejected -- is
	// asserted through Known() here.
	// A peer that encodes the RI LSA without the U-bit (U=0) is still recognized by function
	// code (RFC 7770 sec 5.2: function code 12 is exclusively the RI LSA).
	for _, typ := range []LSType{0x200C, 0x400C, 0x000C} {
		if !typ.Known() {
			t.Errorf("type %#04x (U=0) not Known() -- RI must be recognized regardless of U-bit", uint16(typ))
		}
	}
	// A non-RI, non-base type (function code 13) is not mistaken for a known LSA.
	if LSType(0x200D).Known() {
		t.Errorf("type 0x200D (function code 13) must not be Known()")
	}
}
