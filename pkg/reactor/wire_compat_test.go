package reactor

import (
	"bytes"
	"net/netip"
	"testing"

	"github.com/exa-networks/zebgp/pkg/bgp/attribute"
	"github.com/exa-networks/zebgp/pkg/bgp/message"
	"github.com/exa-networks/zebgp/pkg/bgp/nlri"
)

// Wire format compatibility tests verify that the new message.UpdateBuilder
// produces byte-identical output to the old buildXxx functions in peer.go.
//
// VALIDATES: New builders produce same wire format as old code.
//
// PREVENTS: Breaking changes to UPDATE encoding that could cause peer rejection.

// TestWireCompat_UnicastIPv4 verifies IPv4 unicast wire format compatibility.
func TestWireCompat_UnicastIPv4(t *testing.T) {
	route := StaticRoute{
		Prefix:          netip.MustParsePrefix("10.0.0.0/24"),
		NextHop:         netip.MustParseAddr("192.168.1.1"),
		Origin:          0, // IGP
		MED:             100,
		LocalPreference: 200,
		Communities:     []uint32{0xFFFF0001, 0xFFFF0002},
	}

	ctx := &nlri.PackContext{ASN4: true}
	nf := &NegotiatedFamilies{IPv4Unicast: true}

	// Old implementation
	oldUpdate := buildStaticRouteUpdate(route, 65001, true, ctx, nf)

	// New implementation
	ub := message.NewUpdateBuilder(65001, true, ctx)
	params := message.UnicastParams{
		Prefix:          route.Prefix,
		NextHop:         route.NextHop,
		Origin:          attribute.Origin(route.Origin),
		MED:             route.MED,
		LocalPreference: route.LocalPreference,
		Communities:     route.Communities,
	}
	newUpdate := ub.BuildUnicast(params)

	// Compare wire format
	if !bytes.Equal(oldUpdate.PathAttributes, newUpdate.PathAttributes) {
		t.Errorf("PathAttributes mismatch:\nold: %x\nnew: %x",
			oldUpdate.PathAttributes, newUpdate.PathAttributes)
	}
	if !bytes.Equal(oldUpdate.NLRI, newUpdate.NLRI) {
		t.Errorf("NLRI mismatch:\nold: %x\nnew: %x",
			oldUpdate.NLRI, newUpdate.NLRI)
	}
}

// TestWireCompat_UnicastIPv4_EBGP verifies eBGP AS_PATH handling.
func TestWireCompat_UnicastIPv4_EBGP(t *testing.T) {
	route := StaticRoute{
		Prefix:  netip.MustParsePrefix("10.0.0.0/24"),
		NextHop: netip.MustParseAddr("192.168.1.1"),
		Origin:  0,
	}

	ctx := &nlri.PackContext{ASN4: true}
	nf := &NegotiatedFamilies{IPv4Unicast: true}

	// Old implementation (eBGP)
	oldUpdate := buildStaticRouteUpdate(route, 65001, false, ctx, nf)

	// New implementation
	ub := message.NewUpdateBuilder(65001, false, ctx)
	params := message.UnicastParams{
		Prefix:  route.Prefix,
		NextHop: route.NextHop,
		Origin:  attribute.Origin(route.Origin),
	}
	newUpdate := ub.BuildUnicast(params)

	if !bytes.Equal(oldUpdate.PathAttributes, newUpdate.PathAttributes) {
		t.Errorf("PathAttributes mismatch:\nold: %x\nnew: %x",
			oldUpdate.PathAttributes, newUpdate.PathAttributes)
	}
}

// TestWireCompat_UnicastIPv6 verifies IPv6 unicast wire format compatibility.
func TestWireCompat_UnicastIPv6(t *testing.T) {
	route := StaticRoute{
		Prefix:          netip.MustParsePrefix("2001:db8::/32"),
		NextHop:         netip.MustParseAddr("2001:db8::1"),
		Origin:          0,
		LocalPreference: 100,
	}

	ctx := &nlri.PackContext{ASN4: true}
	nf := &NegotiatedFamilies{IPv6Unicast: true}

	// Old implementation
	oldUpdate := buildStaticRouteUpdate(route, 65001, true, ctx, nf)

	// New implementation
	ub := message.NewUpdateBuilder(65001, true, ctx)
	params := message.UnicastParams{
		Prefix:          route.Prefix,
		NextHop:         route.NextHop,
		Origin:          attribute.Origin(route.Origin),
		LocalPreference: route.LocalPreference,
	}
	newUpdate := ub.BuildUnicast(params)

	if !bytes.Equal(oldUpdate.PathAttributes, newUpdate.PathAttributes) {
		t.Errorf("PathAttributes mismatch:\nold: %x\nnew: %x",
			oldUpdate.PathAttributes, newUpdate.PathAttributes)
	}
}

// TestWireCompat_VPNIPv4 verifies VPN-IPv4 wire format compatibility.
func TestWireCompat_VPNIPv4(t *testing.T) {
	route := StaticRoute{
		Prefix:            netip.MustParsePrefix("10.0.0.0/24"),
		NextHop:           netip.MustParseAddr("192.168.1.1"),
		Origin:            0,
		Label:             100,
		RD:                "100:100",                           // IsVPN() checks this string
		RDBytes:           [8]byte{0, 1, 0, 0, 0, 100, 0, 100}, // Type 1: 100:100
		LocalPreference:   150,
		ExtCommunityBytes: []byte{0x00, 0x02, 0xfd, 0xe9, 0x00, 0x00, 0x00, 0x64}, // RT 65001:100
	}

	ctx := &nlri.PackContext{ASN4: true}
	nf := &NegotiatedFamilies{IPv4Unicast: true} // VPN detection is via route.IsVPN()

	// Old implementation
	oldUpdate := buildStaticRouteUpdate(route, 65001, true, ctx, nf)

	// New implementation
	ub := message.NewUpdateBuilder(65001, true, ctx)
	params := message.VPNParams{
		Prefix:            route.Prefix,
		NextHop:           route.NextHop,
		Origin:            attribute.Origin(route.Origin),
		Label:             route.Label,
		RDBytes:           route.RDBytes,
		LocalPreference:   route.LocalPreference,
		ExtCommunityBytes: route.ExtCommunityBytes,
	}
	newUpdate := ub.BuildVPN(params)

	if !bytes.Equal(oldUpdate.PathAttributes, newUpdate.PathAttributes) {
		t.Errorf("PathAttributes mismatch:\nold: %x\nnew: %x",
			oldUpdate.PathAttributes, newUpdate.PathAttributes)
	}
}

// TestWireCompat_ExtendedNextHop verifies RFC 8950 extended next-hop.
// IPv4 prefix with IPv6 next-hop using MP_REACH_NLRI.
func TestWireCompat_ExtendedNextHop(t *testing.T) {
	route := StaticRoute{
		Prefix:          netip.MustParsePrefix("10.0.0.0/24"),
		NextHop:         netip.MustParseAddr("2001:db8::1"),
		Origin:          0,
		LocalPreference: 100,
	}

	ctx := &nlri.PackContext{ASN4: true}
	nf := &NegotiatedFamilies{IPv4Unicast: true, IPv4UnicastExtNH: true}

	// Old implementation
	oldUpdate := buildStaticRouteUpdate(route, 65001, true, ctx, nf)

	// New implementation via helper
	newUpdate := buildStaticRouteUpdateNew(route, 65001, true, ctx, nf)

	if !bytes.Equal(oldUpdate.PathAttributes, newUpdate.PathAttributes) {
		t.Errorf("PathAttributes mismatch:\nold: %x\nnew: %x",
			oldUpdate.PathAttributes, newUpdate.PathAttributes)
	}
	if !bytes.Equal(oldUpdate.NLRI, newUpdate.NLRI) {
		t.Errorf("NLRI mismatch:\nold: %x\nnew: %x",
			oldUpdate.NLRI, newUpdate.NLRI)
	}
}

// TestWireCompat_RawAttributes verifies raw attribute pass-through.
func TestWireCompat_RawAttributes(t *testing.T) {
	route := StaticRoute{
		Prefix:  netip.MustParsePrefix("10.0.0.0/24"),
		NextHop: netip.MustParseAddr("192.168.1.1"),
		Origin:  0,
		RawAttributes: []RawAttribute{
			{Flags: 0xC0, Code: 99, Value: []byte{0x01, 0x02, 0x03}},
		},
	}

	ctx := &nlri.PackContext{ASN4: true}
	nf := &NegotiatedFamilies{IPv4Unicast: true}

	// Old implementation
	oldUpdate := buildStaticRouteUpdate(route, 65001, true, ctx, nf)

	// New implementation via helper
	newUpdate := buildStaticRouteUpdateNew(route, 65001, true, ctx, nf)

	if !bytes.Equal(oldUpdate.PathAttributes, newUpdate.PathAttributes) {
		t.Errorf("PathAttributes mismatch:\nold: %x\nnew: %x",
			oldUpdate.PathAttributes, newUpdate.PathAttributes)
	}
}

// Note: MVPN, VPLS, FlowSpec, MUP wire compat tests removed.
// Those build functions were extracted to UpdateBuilder and the old
// implementations deleted. UpdateBuilder tests are in update_build_test.go.
