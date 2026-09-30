// The two exemptions of the RFC 5881 Section 6 destination clause
// (RFC5881-6-2), judged at egressLink.reaches, the check Send runs before it
// writes a single-hop Control packet. The Send path over a real link is in
// rfc5881_subnet_destination_linux_test.go.
package transport

import (
	"net/netip"
	"testing"
)

// subnetExemptionLink is a multiaccess interface holding one IPv4 address in
// 10.58.1.0/24 and one IPv6 address in fd00:5881:1::/64, and no link-local
// address of either family.
func subnetExemptionLink(pointToLink bool) *egressLink {
	return &egressLink{
		subnets: []netip.Prefix{
			netip.MustParsePrefix("10.58.1.0/24"),
			netip.MustParsePrefix("fd00:5881:1::/64"),
		},
		pointToLink: pointToLink,
	}
}

// RFC requirement: RFC5881-6-2 positive -- on a multiaccess interface, a peer
// inside one of the interface's subnets (10.58.1.2, fd00:5881:1::2) and an
// IPv6 link-local peer (fe80::2) are destinations Send may address a
// single-hop Control packet to; on a point-to-point interface, where the
// clause's "On a multiaccess network" does not hold, an off-subnet peer
// (10.58.3.2) is allowed too.
//
// VALIDATES: reaches answers true for an on-subnet peer of either family, for
// an IPv6 link-local peer the interface holds no subnet for, and for any peer
// of a point-to-point interface.
// PREVENTS: the subnet check refusing a session the clause does not forbid.
func TestRFC5881SubnetCheckAllowsOnSubnetAndExemptPeers(t *testing.T) {
	cases := []struct {
		name        string
		peer        string
		pointToLink bool
	}{
		{name: "IPv4 peer in the subnet", peer: "10.58.1.2"},
		{name: "IPv6 peer in the subnet", peer: "fd00:5881:1::2"},
		{name: "IPv6 link-local peer", peer: "fe80::2"},
		{name: "off-subnet peer on a point-to-point link", peer: "10.58.3.2", pointToLink: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			link := subnetExemptionLink(tc.pointToLink)
			if !link.reaches(netip.MustParseAddr(tc.peer)) {
				t.Errorf("reaches(%s) on point-to-point=%v: got false, want true", tc.peer, tc.pointToLink)
			}
		})
	}
}

// RFC requirement: RFC5881-6-2 negative -- on a multiaccess interface with no
// 169.254.0.0/16 address, an IPv4 link-local peer (169.254.5.2), an IPv4-mapped
// form of it, and a global peer off every subnet (10.58.3.2) are refused as
// destinations of a single-hop Control packet.
//
// VALIDATES: the link-local exemption covers only IPv6 fe80::/10. IPv4
// 169.254.0.0/16 is not scoped to one link the way fe80::/10 is (an interface
// need not hold an address in it, and a route through a gateway can carry it),
// so an IPv4 link-local peer is judged against the interface's subnets like
// any other IPv4 peer.
// PREVENTS: a 169.254 peer skipping the subnet check and leaving through a
// gateway with a destination outside the link's subnet.
func TestRFC5881SubnetCheckRefusesIPv4LinkLocalOffTheSubnet(t *testing.T) {
	link := subnetExemptionLink(false)
	for _, peer := range []string{"169.254.5.2", "::ffff:169.254.5.2", "10.58.3.2"} {
		if link.reaches(netip.MustParseAddr(peer)) {
			t.Errorf("reaches(%s) on a multiaccess link with no 169.254 address: got true, want false", peer)
		}
	}
	onLink := &egressLink{subnets: []netip.Prefix{netip.MustParsePrefix("169.254.5.0/24")}}
	if !onLink.reaches(netip.MustParseAddr("169.254.5.2")) {
		t.Errorf("reaches(169.254.5.2) on a link holding 169.254.5.0/24: got false, want true")
	}
}
