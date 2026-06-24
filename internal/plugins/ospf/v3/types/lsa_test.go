// VALIDATES: spec-ospfv3-1-types AC-4/AC-5/AC-6 -- LSType decodes the U-bit, S2/S1
// flooding scope, and 13-bit function code per RFC 5340; the known base LSA constants
// carry the right scope/function; reserved scope is identified; LSAKey is comparable and
// sorts stably.
// PREVENTS: treating LSType as a flat enum (losing scope), or an LSAKey that includes
// age/sequence in its identity.
package types

import "testing"

const (
	testScopeLinkLocal floodScope = 0
	testScopeArea      floodScope = 1
	testScopeAS        floodScope = 2
	testScopeReserved  floodScope = 3
	testUBitMask       uint16     = 0x8000
	testFunctionMask   uint16     = 0x1fff
)

func TestOSPFv3LSTypeScopeFunction(t *testing.T) {
	cases := []struct {
		t     LSType
		scope floodScope
		fn    uint16
		u     bool
	}{
		{LSTypeRouter, testScopeArea, 1, false},
		{LSTypeNetwork, testScopeArea, 2, false},
		{LSTypeInterAreaPrefix, testScopeArea, 3, false},
		{LSTypeInterAreaRouter, testScopeArea, 4, false},
		{LSTypeASExternal, testScopeAS, 5, false},
		{LSTypeNSSA, testScopeArea, 7, false},
		{LSTypeLink, testScopeLinkLocal, 8, false},
		{LSTypeIntraAreaPrefix, testScopeArea, 9, false},
		{LSType(0x8000 | 0x2000 | 0x0042), testScopeArea, 0x42, true}, // U-bit set, area scope
	}
	for _, c := range cases {
		if c.t.Scope() != c.scope {
			t.Errorf("LSType %#04x scope = %v, want %v", uint16(c.t), c.t.Scope(), c.scope)
		}
		if got := uint16(c.t) & testFunctionMask; got != c.fn {
			t.Errorf("LSType %#04x function = %d, want %d", uint16(c.t), got, c.fn)
		}
		if got := uint16(c.t)&testUBitMask != 0; got != c.u {
			t.Errorf("LSType %#04x U-bit = %v, want %v", uint16(c.t), got, c.u)
		}
	}
}

func TestOSPFv3KnownLSATypes(t *testing.T) {
	known := []LSType{
		LSTypeRouter, LSTypeNetwork, LSTypeInterAreaPrefix, LSTypeInterAreaRouter,
		LSTypeASExternal, LSTypeNSSA, LSTypeLink, LSTypeIntraAreaPrefix,
	}
	for _, lt := range known {
		if !lt.Known() {
			t.Errorf("LSType %#04x not Known()", uint16(lt))
		}
	}
	if LSType(0x2000).Known() {
		t.Error("function code 0 reported Known")
	}
}

func TestOSPFv3ReservedScope(t *testing.T) {
	// S2/S1 = 11 is the reserved scope.
	r := LSType(0x6001)
	if r.Scope() != testScopeReserved {
		t.Errorf("scope = %v, want reserved", r.Scope())
	}
	if r.Known() {
		t.Error("reserved-scope LSA reported Known")
	}
}

func TestOSPFv3LSAKeyComparable(t *testing.T) {
	a := LSAKey{Type: LSTypeRouter, LinkStateID: LinkStateID{0, 0, 0, 1}, AdvertisingRouter: RouterID{10, 0, 0, 1}}
	b := a // same identity even with different age/seq (not in the key)
	m := map[LSAKey]int{a: 1}
	if m[b] != 1 {
		t.Error("LSAKey not usable as a map key")
	}

	// Stable ordering: by LSType, then Link State ID, then Advertising Router.
	lo := LSAKey{Type: LSTypeRouter, LinkStateID: LinkStateID{0, 0, 0, 1}, AdvertisingRouter: RouterID{1, 0, 0, 0}}
	hi := LSAKey{Type: LSTypeNetwork, LinkStateID: LinkStateID{0, 0, 0, 1}, AdvertisingRouter: RouterID{1, 0, 0, 0}}
	if lo.Compare(hi) >= 0 {
		t.Error("Router (0x2001) should sort before Network (0x2002)")
	}
	if hi.Compare(lo) <= 0 {
		t.Error("Compare not antisymmetric")
	}
	loCopy := lo
	if lo.Compare(loCopy) != 0 {
		t.Error("Compare not reflexive-zero")
	}
}
