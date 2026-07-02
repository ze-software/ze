// VALIDATES: spec-ospfv3-1-types AC-4/AC-5/AC-6 -- LSType decodes the U-bit, S2/S1
// flooding scope, and 13-bit function code per RFC 5340; the known base LSA constants
// carry the right scope/function; reserved scope is identified; LSAKey is comparable and
// sorts stably.
// PREVENTS: treating LSType as a flat enum (losing scope), or an LSAKey that includes
// age/sequence in its identity.
package types

import (
	"errors"
	"testing"
)

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

func TestGraceLSATypeRegistered(t *testing.T) {
	// spec-ospf-ext-9 AC-2: the OSPFv3 Grace-LSA (function code 11, LS Type 0x000B) is a
	// recognized, link-local-scope LSA type (RFC 5187 sec 2.1).
	if !LSTypeGrace.Known() {
		t.Fatalf("LSTypeGrace %#04x not Known()", uint16(LSTypeGrace))
	}
	if LSTypeGrace.Scope() != testScopeLinkLocal {
		t.Fatalf("LSTypeGrace scope = %v, want link-local", LSTypeGrace.Scope())
	}
	if got := uint16(LSTypeGrace) & testFunctionMask; got != 11 {
		t.Fatalf("LSTypeGrace function code = %d, want 11", got)
	}
	if uint16(LSTypeGrace)&testUBitMask != 0 {
		t.Fatalf("LSTypeGrace U-bit set, want clear (link-local)")
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

// VALIDATES: spec-ospfv3-1-types AC-4 -- LSTypeFromBytes reads a 16-bit big-endian LS Type at
// an offset and round-trips WriteTo; it rejects a negative offset or a read past the buffer end
// with ErrWrongLength instead of panicking, and accepts the last in-bounds 2-octet window.
// PREVENTS: an attacker-controlled offset causing a slice panic, or an endian-swapped LS Type.
func TestOSPFv3LSTypeFromBytes(t *testing.T) {
	// Big-endian decode at offset 0 and at a non-zero offset.
	got, err := LSTypeFromBytes([]byte{0x20, 0x01}, 0)
	if err != nil {
		t.Fatalf("LSTypeFromBytes returned error: %v", err)
	}
	if got != LSTypeRouter {
		t.Errorf("LSTypeFromBytes = %#04x, want %#04x (Router)", uint16(got), uint16(LSTypeRouter))
	}
	got, err = LSTypeFromBytes([]byte{0xff, 0x40, 0x05}, 1)
	if err != nil {
		t.Fatalf("LSTypeFromBytes at offset 1 returned error: %v", err)
	}
	if got != LSTypeASExternal {
		t.Errorf("LSTypeFromBytes(off=1) = %#04x, want %#04x (AS-External)", uint16(got), uint16(LSTypeASExternal))
	}

	// Round-trip: WriteTo then LSTypeFromBytes recovers the value.
	var buf [2]byte
	if n := LSTypeIntraAreaPrefix.WriteTo(buf[:], 0); n != 2 {
		t.Fatalf("LSType.WriteTo returned %d, want 2", n)
	}
	back, err := LSTypeFromBytes(buf[:], 0)
	if err != nil {
		t.Fatalf("round-trip decode error: %v", err)
	}
	if back != LSTypeIntraAreaPrefix {
		t.Errorf("round-trip = %#04x, want %#04x", uint16(back), uint16(LSTypeIntraAreaPrefix))
	}

	// Boundary: the last valid 2-octet window is off == len-2; off == len-1 is out of range.
	b := []byte{0x00, 0x08, 0xde}
	if _, err := LSTypeFromBytes(b, 1); err != nil {
		t.Errorf("LSTypeFromBytes(off=len-2) error = %v, want nil", err)
	}
	if _, err := LSTypeFromBytes(b, 2); !errors.Is(err, ErrWrongLength) {
		t.Errorf("LSTypeFromBytes(off=len-1) err = %v, want ErrWrongLength", err)
	}

	// Truncated buffer and negative offset both return ErrWrongLength, never a panic.
	for _, tc := range []struct {
		b   []byte
		off int
	}{
		{[]byte{0x20}, 0},  // one byte, needs two
		{[]byte{}, 0},      // empty
		{[]byte{0, 0}, -1}, // negative offset
	} {
		if _, err := LSTypeFromBytes(tc.b, tc.off); !errors.Is(err, ErrWrongLength) {
			t.Errorf("LSTypeFromBytes(%v, %d) err = %v, want ErrWrongLength", tc.b, tc.off, err)
		}
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

// VALIDATES: spec-ospfv3-1-types AC-6 -- with equal LS Types, Compare breaks ties first on
// Link State ID and then on Advertising Router.
// PREVENTS: an LSDB ordering that ignores the Link State ID or Advertising Router tiebreak and
// so mis-sorts two LSAs of the same type.
func TestOSPFv3LSAKeyCompareTiebreak(t *testing.T) {
	base := LSAKey{Type: LSTypeRouter, LinkStateID: LinkStateID{0, 0, 0, 1}, AdvertisingRouter: RouterID{10, 0, 0, 1}}

	// Same type, larger Link State ID sorts after.
	byLSID := base
	byLSID.LinkStateID = LinkStateID{0, 0, 0, 2}
	if base.Compare(byLSID) != -1 || byLSID.Compare(base) != 1 {
		t.Errorf("Link State ID tiebreak wrong: base<byLSID expected")
	}

	// Same type and Link State ID, larger Advertising Router sorts after.
	byRouter := base
	byRouter.AdvertisingRouter = RouterID{10, 0, 0, 2}
	if base.Compare(byRouter) != -1 || byRouter.Compare(base) != 1 {
		t.Errorf("Advertising Router tiebreak wrong: base<byRouter expected")
	}
}
