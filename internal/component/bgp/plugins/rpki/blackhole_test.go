// VALIDATES: RFC 7999 Section 3.3's fourth requirement, that origin validation
// does not inadvertently block a legitimate BLACKHOLE announcement. The
// exemption fires only for the case the RFC names: a covering VRP that names
// the route's origin AS and disagrees on nothing but prefix length.
// PREVENTS: an operator running origin-invalid-action reject getting a honoring
// switch that silently does nothing, because a /32 under a maxLength-24 ROA is
// RFC 6811 Invalid and is dropped before the honoring path sees it. Also
// prevents that exemption widening into a general escape from origin validation.

package rpki

import (
	"testing"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
)

func lengthOnlyCache(t *testing.T) *ROACache {
	t.Helper()
	c := newROACache()
	// One VRP: 192.0.2.0/24, maxLength 24, origin AS 65001. A /32 inside it is
	// RFC 6811 Invalid, because prefixLen 32 exceeds MaxLength 24.
	c.Add(makeVRP("192.0.2.0/24", 24, 65001))
	return c
}

// The exact case RFC 7999 Section 3.3 names. The origin AS is authorized for
// the covering prefix, and only the length disagrees. Length is what
// blackholing changes by design.
func TestInvalidByLengthOnlyForABlackholeMoreSpecific(t *testing.T) {
	c := lengthOnlyCache(t)

	if got := c.Validate("192.0.2.1/32", 65001); got != ValidationInvalid {
		t.Fatalf("Validate = %d, want Invalid: the premise of this test is gone", got)
	}
	if !c.invalidByLengthOnly("192.0.2.1/32", 65001) {
		t.Error("invalidByLengthOnly = false: a legitimate blackhole is blocked with no way out")
	}
}

// A WRONG origin is not a length problem. The announcement is a hijack shape,
// and the exemption must not reach it whatever community it carries.
func TestInvalidByLengthOnlyRefusesAWrongOrigin(t *testing.T) {
	c := lengthOnlyCache(t)

	if got := c.Validate("192.0.2.1/32", 65999); got != ValidationInvalid {
		t.Fatalf("Validate = %d, want Invalid", got)
	}
	if c.invalidByLengthOnly("192.0.2.1/32", 65999) {
		t.Error("invalidByLengthOnly = true for an unauthorized origin: the exemption is a hijack path")
	}
}

// A route that is not Invalid at all has nothing to be exempted from, and a
// route with no covering VRP is NotFound rather than Invalid.
func TestInvalidByLengthOnlyRefusesNonInvalidStates(t *testing.T) {
	c := lengthOnlyCache(t)

	if c.invalidByLengthOnly("192.0.2.0/24", 65001) {
		t.Error("a Valid route was reported as invalid-by-length")
	}
	if c.invalidByLengthOnly("198.51.100.1/32", 65001) {
		t.Error("a NotFound route was reported as invalid-by-length")
	}
}

// AS_SET and an empty AS_PATH give OriginNone, which can never match a VRP.
// Exempting it would let a route with no usable origin through.
func TestInvalidByLengthOnlyRefusesOriginNone(t *testing.T) {
	if lengthOnlyCache(t).invalidByLengthOnly("192.0.2.1/32", OriginNone) {
		t.Error("a route with no usable origin AS was exempted")
	}
}

// A prefix the cache cannot parse was never validated. Validate fails closed on
// it, and so must this.
func TestInvalidByLengthOnlyRefusesAnUnparseablePrefix(t *testing.T) {
	if lengthOnlyCache(t).invalidByLengthOnly("not-a-prefix", 65001) {
		t.Error("an unparseable prefix was exempted")
	}
}

// The community read, over the indexed attribute wire the plugin actually
// holds. Every fixture is real path-attribute bytes, so the index build is part
// of what is under test.
func TestRPKICarriesBlackhole(t *testing.T) {
	origin := []byte{0x40, 0x01, 0x01, 0x00}
	asPath := []byte{0x40, 0x02, 0x06, 0x02, 0x01, 0x00, 0x00, 0xFD, 0xE9}
	commBlackhole := []byte{0xC0, 0x08, 0x04, 0xFF, 0xFF, 0x02, 0x9A}
	commNoExport := []byte{0xC0, 0x08, 0x04, 0xFF, 0xFF, 0xFF, 0x01}
	commBoth := []byte{0xC0, 0x08, 0x08, 0xFF, 0xFF, 0xFF, 0x01, 0xFF, 0xFF, 0x02, 0x9A}
	// Extended-length form: flags carry 0x10, and the length is two octets.
	commExtLen := []byte{0xD0, 0x08, 0x00, 0x04, 0xFF, 0xFF, 0x02, 0x9A}

	cases := []struct {
		name  string
		attrs []byte
		want  bool
	}{
		{"no attributes", nil, false},
		{"no community attribute", concatBytes(origin, asPath), false},
		{"community present, blackhole absent", concatBytes(origin, commNoExport), false},
		{"blackhole alone", concatBytes(origin, commBlackhole), true},
		{"blackhole after another value", concatBytes(origin, asPath, commBoth), true},
		{"blackhole in an extended-length attribute", concatBytes(origin, commExtLen), true},
		{"truncated attribute header", []byte{0xC0}, false},
		{"attribute length runs past the buffer", []byte{0xC0, 0x08, 0x40, 0xFF, 0xFF, 0x02, 0x9A}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			wire := attribute.NewAttributesWire(c.attrs, bgpctx.APIContextID)
			if got := rpkiCarriesBlackhole(wire); got != c.want {
				t.Errorf("rpkiCarriesBlackhole = %v, want %v", got, c.want)
			}
		})
	}
}

// A nil wire is what an event with no attributes at all delivers. It must read
// as "no blackhole" rather than panicking or guessing.
func TestRPKICarriesBlackholeNilWire(t *testing.T) {
	if rpkiCarriesBlackhole(nil) {
		t.Error("a nil attribute wire was read as carrying BLACKHOLE")
	}
}

func concatBytes(parts ...[]byte) []byte {
	var out []byte
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}
