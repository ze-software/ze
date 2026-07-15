package vrrp

import (
	"net/netip"
	"testing"
)

// TestVIPCIDRsSubnetPrefix proves a VIP is installed with the prefix of the
// parent subnet that contains it, not a host route: the macvlan needs the
// subnet's connected route to answer ARP for the VIP (spec-vrrp-6 dataplane).
func TestVIPCIDRsSubnetPrefix(t *testing.T) {
	spec := GroupSpec{
		realPrefixes: []netip.Prefix{
			netip.MustParsePrefix("192.0.2.251/24"),
			netip.MustParsePrefix("10.0.0.1/8"),
			netip.MustParsePrefix("2001:db8::1/64"),
		},
	}
	cases := []struct {
		vip  string
		want string
	}{
		{"192.0.2.1", "192.0.2.1/24"}, // in the /24 parent subnet
		{"10.5.5.5", "10.5.5.5/8"},    // in the /8 parent subnet
		{"2001:db8::9", "2001:db8::9/64"},
	}
	for _, c := range cases {
		got := spec.vipCIDRs([]netip.Addr{netip.MustParseAddr(c.vip)})
		if len(got) != 1 || got[0] != c.want {
			t.Errorf("vipCIDRs(%s) = %v, want [%s]", c.vip, got, c.want)
		}
	}
}

// TestVIPCIDRsFallbacks proves a VIP outside every parent subnet installs as a
// HOST route, never with a non-containing parent prefix's length (which would
// add a bogus connected route for a subnet the VIP is not in).
func TestVIPCIDRsFallbacks(t *testing.T) {
	// VIP not contained by any parent subnet, but a same-family /26 exists ->
	// host route (/32), NOT /26.
	spec := GroupSpec{realPrefixes: []netip.Prefix{netip.MustParsePrefix("192.0.2.251/26")}}
	if got := spec.vipCIDRs([]netip.Addr{netip.MustParseAddr("198.51.100.9")}); got[0] != "198.51.100.9/32" {
		t.Errorf("out-of-subnet fallback = %v, want [198.51.100.9/32] (host route, not the foreign /26)", got)
	}

	// No parent prefixes at all -> host route.
	empty := GroupSpec{}
	if got := empty.vipCIDRs([]netip.Addr{netip.MustParseAddr("192.0.2.1")}); got[0] != "192.0.2.1/32" {
		t.Errorf("no-prefix v4 fallback = %v, want [192.0.2.1/32]", got)
	}
	if got := empty.vipCIDRs([]netip.Addr{netip.MustParseAddr("2001:db8::1")}); got[0] != "2001:db8::1/128" {
		t.Errorf("no-prefix v6 fallback = %v, want [2001:db8::1/128]", got)
	}
}

// TestVIPCIDRsOwnerHostRoute proves an address-owner VIP (equal to one of the
// unit's real addresses) installs as a HOST route, not the subnet prefix, so it
// does not add a second connected route competing with the parent's.
func TestVIPCIDRsOwnerHostRoute(t *testing.T) {
	spec := GroupSpec{
		realAddresses: []netip.Addr{netip.MustParseAddr("192.0.2.1"), netip.MustParseAddr("2001:db8::1")},
		realPrefixes: []netip.Prefix{
			netip.MustParsePrefix("192.0.2.1/24"),
			netip.MustParsePrefix("2001:db8::1/64"),
		},
	}
	// The owner VIP equals a real address -> /32 (not /24).
	if got := spec.vipCIDRs([]netip.Addr{netip.MustParseAddr("192.0.2.1")}); got[0] != "192.0.2.1/32" {
		t.Errorf("owner v4 VIP = %v, want [192.0.2.1/32] (host route, no duplicate connected route)", got)
	}
	if got := spec.vipCIDRs([]netip.Addr{netip.MustParseAddr("2001:db8::1")}); got[0] != "2001:db8::1/128" {
		t.Errorf("owner v6 VIP = %v, want [2001:db8::1/128]", got)
	}
	// A non-owner VIP in the same subnet still gets the subnet prefix.
	if got := spec.vipCIDRs([]netip.Addr{netip.MustParseAddr("192.0.2.9")}); got[0] != "192.0.2.9/24" {
		t.Errorf("non-owner VIP = %v, want [192.0.2.9/24]", got)
	}
}

// TestVIPCIDRsLongestMatch proves the LONGEST containing parent subnet wins when
// a VIP sits in more than one configured prefix.
func TestVIPCIDRsLongestMatch(t *testing.T) {
	spec := GroupSpec{realPrefixes: []netip.Prefix{
		netip.MustParsePrefix("192.0.2.251/24"),
		netip.MustParsePrefix("192.0.2.129/25"),
	}}
	if got := spec.vipCIDRs([]netip.Addr{netip.MustParseAddr("192.0.2.130")}); got[0] != "192.0.2.130/25" {
		t.Errorf("longest-match = %v, want [192.0.2.130/25]", got)
	}
}
