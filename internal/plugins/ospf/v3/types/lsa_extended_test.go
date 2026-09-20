// VALIDATES: spec-ospf-ext-5 AC-18/AC-23, R-11 -- the RFC 8362 Extended-LSA type
// constants carry the RFC 8362 Section 2 values, U-bit and scope bits included, are
// recognized by Known() so the shared LSDB stores and floods them by scope, and the base
// RFC 5340 types stay Known.
// PREVENTS: wrong Extended-LSA type codes; an Extended LSA originated with the U-bit clear,
// which RFC 5340 Appendix A.4.2.1 confines to the originating link; a new type silently
// dropped by the LSDB; an additive change regressing base-type recognition.
package types

import "testing"

func TestExtendedLSATypeConstants(t *testing.T) {
	// The LS Type table of RFC 8362 Section 2, which sets the U-bit on every row:
	// "For backward compatibility, the U-bit MUST be set in the LS Type so that the LSAs
	// will be flooded by OSPFv3 routers that do not understand them."
	cases := []struct {
		name string
		got  LSType
		want LSType
	}{
		{"E-Router", LSTypeERouter, 0xA021},
		{"E-Network", LSTypeENetwork, 0xA022},
		{"E-Inter-Area-Prefix", LSTypeEInterAreaPrefix, 0xA023},
		{"E-AS-External", LSTypeEASExternal, 0xC025},
		{"E-Type-7", LSTypeEType7, 0xA027},
		{"E-Link", LSTypeELink, 0x8028},
		{"E-Intra-Area-Prefix", LSTypeEIntraAreaPrefix, 0xA029},
	}
	for _, c := range cases {
		if c.got&lsTypeUBit == 0 {
			t.Errorf("%s = 0x%04X has the U-bit clear; RFC 8362 Section 2 requires it set", c.name, uint16(c.got))
		}
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Fatalf("%s = 0x%04X want 0x%04X", c.name, uint16(c.got), uint16(c.want))
		}
	}
}

func TestExtendedLSAKnownAndScope(t *testing.T) {
	// The new types are recognized (so the LSDB stores + floods them by scope).
	for _, lt := range []LSType{LSTypeERouter, LSTypeEInterAreaPrefix, LSTypeEASExternal, LSTypeEType7, LSTypeELink, LSTypeEIntraAreaPrefix} {
		if !lt.Known() {
			t.Fatalf("Extended-LSA type 0x%04X must be Known()", uint16(lt))
		}
	}
	// Scope falls out of the high bits: E-Router area, E-AS-External AS, E-Link link-local.
	if LSTypeERouter.Scope() != LSTypeRouter.Scope() {
		t.Fatalf("E-Router must be area-scoped like Router-LSA")
	}
	if LSTypeEASExternal.Scope() != LSTypeASExternal.Scope() {
		t.Fatalf("E-AS-External must be AS-scoped")
	}
	if LSTypeELink.Scope() != LSTypeLink.Scope() {
		t.Fatalf("E-Link must be link-local scoped")
	}
}

func TestBaseLSAsStillKnown(t *testing.T) {
	// Additive change: the RFC 5340 base types remain recognized (R-11).
	for _, lt := range []LSType{LSTypeRouter, LSTypeNetwork, LSTypeInterAreaPrefix, LSTypeInterAreaRouter, LSTypeASExternal, LSTypeNSSA, LSTypeLink, LSTypeIntraAreaPrefix} {
		if !lt.Known() {
			t.Fatalf("base LSA type 0x%04X must remain Known()", uint16(lt))
		}
	}
}
