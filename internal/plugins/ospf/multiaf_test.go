// VALIDATES: spec-ospf-ext-15 AC-1 -- the RFC 5838 §2.1 Instance-ID-range to address-family
// mapping for all four ranges, the >127 invalid boundary, the per-AF Loc-RIB install family,
// the prefix address width, and the default-AF AF-bit rule.
// PREVENTS: mis-classifying an Instance ID into the wrong AF (which would form an adjacency on
// the wrong family) or installing an AF's routes into the wrong Loc-RIB family.
package ospf

import (
	"testing"

	"github.com/ze-software/ze/internal/core/family"
)

func TestAFFromInstanceID(t *testing.T) {
	cases := []struct {
		id     uint8
		wantAF addressFamily
		wantOK bool
	}{
		// IPv6 unicast 0-31.
		{0, afIPv6Unicast, true},
		{31, afIPv6Unicast, true},
		// IPv6 multicast 32-63.
		{32, afIPv6Multicast, true},
		{63, afIPv6Multicast, true},
		// IPv4 unicast 64-95.
		{64, afIPv4Unicast, true},
		{95, afIPv4Unicast, true},
		// IPv4 multicast 96-127.
		{96, afIPv4Multicast, true},
		{127, afIPv4Multicast, true},
		// Out of the AF space (>127).
		{128, 0, false},
		{200, 0, false},
		{255, 0, false},
	}
	for _, c := range cases {
		got, ok := afFromInstanceID(c.id)
		if ok != c.wantOK || (ok && got != c.wantAF) {
			t.Errorf("afFromInstanceID(%d) = (%s, %v), want (%s, %v)", c.id, got, ok, c.wantAF, c.wantOK)
		}
	}
}

func TestAFInstallFamily(t *testing.T) {
	cases := []struct {
		af   addressFamily
		want family.Family
	}{
		{afIPv6Unicast, family.IPv6Unicast},
		{afIPv6Multicast, family.IPv6Multicast},
		{afIPv4Unicast, family.IPv4Unicast},
		{afIPv4Multicast, family.IPv4Multicast},
	}
	for _, c := range cases {
		if got := c.af.family(); got != c.want {
			t.Errorf("%s.family() = %s, want %s", c.af, got, c.want)
		}
	}
}

func TestAFPrefixWidth(t *testing.T) {
	for _, af := range []addressFamily{afIPv6Unicast, afIPv6Multicast} {
		if got := af.prefixWidth(); got != 16 {
			t.Errorf("%s.prefixWidth() = %d, want 16", af, got)
		}
		if af.isIPv4() {
			t.Errorf("%s.isIPv4() = true, want false", af)
		}
	}
	for _, af := range []addressFamily{afIPv4Unicast, afIPv4Multicast} {
		if got := af.prefixWidth(); got != 4 {
			t.Errorf("%s.prefixWidth() = %d, want 4", af, got)
		}
		if !af.isIPv4() {
			t.Errorf("%s.isIPv4() = false, want true", af)
		}
	}
}

func TestAFIsDefault(t *testing.T) {
	if !afIPv6Unicast.isDefault() {
		t.Error("IPv6-unicast must be the default AF (RFC 5838 §2.6)")
	}
	for _, af := range []addressFamily{afIPv6Multicast, afIPv4Unicast, afIPv4Multicast} {
		if af.isDefault() {
			t.Errorf("%s.isDefault() = true, want false (only IPv6-unicast is default)", af)
		}
	}
}

func TestAFInstanceIDInRange(t *testing.T) {
	// Boundary matrix from the spec's Boundary Tests table.
	cases := []struct {
		af   addressFamily
		id   uint8
		want bool
	}{
		{afIPv6Unicast, 31, true}, {afIPv6Unicast, 32, false},
		{afIPv6Multicast, 32, true}, {afIPv6Multicast, 31, false}, {afIPv6Multicast, 63, true}, {afIPv6Multicast, 64, false},
		{afIPv4Unicast, 64, true}, {afIPv4Unicast, 63, false}, {afIPv4Unicast, 95, true}, {afIPv4Unicast, 96, false},
		{afIPv4Multicast, 96, true}, {afIPv4Multicast, 95, false}, {afIPv4Multicast, 127, true},
	}
	for _, c := range cases {
		if got := afInstanceIDInRange(c.af, c.id); got != c.want {
			t.Errorf("afInstanceIDInRange(%s, %d) = %v, want %v", c.af, c.id, got, c.want)
		}
	}
}

func TestAFFromName(t *testing.T) {
	cases := []struct {
		name   string
		wantAF addressFamily
		wantOK bool
	}{
		{"ipv6", afIPv6Unicast, true},
		{"ipv6-unicast", afIPv6Unicast, true},
		{"ipv6-multicast", afIPv6Multicast, true},
		{"ipv4-unicast", afIPv4Unicast, true},
		{"ipv4-multicast", afIPv4Multicast, true},
		{"bogus", 0, false},
	}
	for _, c := range cases {
		got, ok := afFromName(c.name)
		if ok != c.wantOK || (ok && got != c.wantAF) {
			t.Errorf("afFromName(%q) = (%s, %v), want (%s, %v)", c.name, got, ok, c.wantAF, c.wantOK)
		}
	}
}
