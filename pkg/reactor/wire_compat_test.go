package reactor

import (
	"bytes"
	"encoding/hex"
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

// TestWireCompat_VPNIPv4 verifies VPN-IPv4 wire format matches ExaBGP.
//
// NOTE: VPN wire format differs from RFC 4271 Appendix F.3 ordering to match
// ExaBGP compatibility. ExaBGP places EXT_COMMUNITIES before MP_REACH_NLRI
// and includes NEXT_HOP attribute even for MP_REACH routes.
func TestWireCompat_VPNIPv4(t *testing.T) {
	ctx := &nlri.PackContext{ASN4: true}

	// Build VPN route UPDATE
	ub := message.NewUpdateBuilder(65001, true, ctx)
	params := message.VPNParams{
		Prefix:            netip.MustParsePrefix("10.0.0.0/24"),
		NextHop:           netip.MustParseAddr("192.168.1.1"),
		Origin:            attribute.OriginIGP,
		Label:             100,
		RDBytes:           [8]byte{0, 1, 0, 0, 0, 100, 0, 100}, // Type 1: 100:100
		LocalPreference:   150,
		ExtCommunityBytes: []byte{0x00, 0x02, 0xfd, 0xe9, 0x00, 0x00, 0x00, 0x64}, // RT 65001:100
	}
	update := ub.BuildVPN(params)

	// Expected output matching ExaBGP format:
	// ORIGIN (1) + AS_PATH (2) + NEXT_HOP (3) + LOCAL_PREF (5) + EXT_COM (16) + MP_REACH (14)
	// Note: EXT_COM before MP_REACH for ExaBGP compatibility
	expected, _ := hex.DecodeString(
		"40010100" + // ORIGIN: IGP
			"400200" + // AS_PATH: empty
			"400304c0a80101" + // NEXT_HOP: 192.168.1.1
			"40050400000096" + // LOCAL_PREF: 150
			"c010080002fde900000064" + // EXT_COMMUNITIES: RT 65001:100
			"800e20" + // MP_REACH_NLRI header (len=32)
			"0001800c" + // AFI=1, SAFI=128, NH_LEN=12
			"0000000000000000c0a80101" + // NH: RD(8 zeros) + IPv4(192.168.1.1)
			"00" + // Reserved
			"70" + // NLRI: Length=112 bits (3*8 label + 64 RD + 24 prefix)
			"000641" + // Label: 100 with BOS
			"00010000006400640a0000") // RD + prefix (10.0.0.0/24)

	if !bytes.Equal(update.PathAttributes, expected) {
		t.Errorf("PathAttributes mismatch:\nexpected: %x\ngot:      %x",
			expected, update.PathAttributes)
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
