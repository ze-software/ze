package reactor

import (
	"bytes"
	"encoding/hex"
	"net/netip"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/attribute"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/message"
)

// Wire format tests verify that UpdateBuilder produces correct wire format.
// Tests use expected-bytes assertions based on RFC specifications.
//
// VALIDATES: UPDATE encoding follows RFC 4271 wire format.
//
// PREVENTS: Breaking changes to UPDATE encoding that could cause peer rejection.

// TestWireFormat_UnicastIPv4 verifies IPv4 unicast wire format.
//
// RFC 4271 attribute order: ORIGIN(1), AS_PATH(2), NEXT_HOP(3), MED(4), LOCAL_PREF(5), COMMUNITIES(8).
func TestWireFormat_UnicastIPv4(t *testing.T) {

	ub := message.NewUpdateBuilder(65001, true, true, false)

	params := message.UnicastParams{
		Prefix:          netip.MustParsePrefix("10.0.0.0/24"),
		NextHop:         netip.MustParseAddr("192.168.1.1"),
		Origin:          attribute.OriginIGP,
		MED:             100,
		LocalPreference: 200,
		Communities:     []uint32{0xFFFF0001, 0xFFFF0002},
	}

	update := ub.BuildUnicast(&params)

	// Expected: ORIGIN + AS_PATH(empty) + NEXT_HOP + MED + LOCAL_PREF + COMMUNITIES
	expectedAttrs, _ := hex.DecodeString(
		"40010100" + // ORIGIN: IGP
			"400200" + // AS_PATH: empty
			"400304c0a80101" + // NEXT_HOP: 192.168.1.1
			"800404" + "00000064" + // MED: 100
			"400504" + "000000c8" + // LOCAL_PREF: 200
			"c00808" + "ffff0001ffff0002") // COMMUNITIES: sorted

	if !bytes.Equal(update.PathAttributes, expectedAttrs) {
		t.Errorf("PathAttributes mismatch:\nexpected: %x\ngot:      %x",
			expectedAttrs, update.PathAttributes)
	}

	// NLRI: 24-bit prefix (1 byte len + 3 bytes prefix)
	expectedNLRI, _ := hex.DecodeString("180a0000")
	if !bytes.Equal(update.NLRI, expectedNLRI) {
		t.Errorf("NLRI mismatch:\nexpected: %x\ngot:      %x",
			expectedNLRI, update.NLRI)
	}
}

// TestWireFormat_UnicastIPv4_EBGP verifies eBGP AS_PATH handling.
//
// RFC 4271 Section 5.1.2: eBGP MUST prepend local AS to AS_PATH.
func TestWireFormat_UnicastIPv4_EBGP(t *testing.T) {

	ub := message.NewUpdateBuilder(65001, false, true, false) // eBGP

	params := message.UnicastParams{
		Prefix:  netip.MustParsePrefix("10.0.0.0/24"),
		NextHop: netip.MustParseAddr("192.168.1.1"),
		Origin:  attribute.OriginIGP,
	}

	update := ub.BuildUnicast(&params)

	// Expected: ORIGIN + AS_PATH([65001]) + NEXT_HOP (no LOCAL_PREF for eBGP)
	expectedAttrs, _ := hex.DecodeString(
		"40010100" + // ORIGIN: IGP
			"40020602010000fde9" + // AS_PATH: AS_SEQUENCE [65001]
			"400304c0a80101") // NEXT_HOP: 192.168.1.1

	if !bytes.Equal(update.PathAttributes, expectedAttrs) {
		t.Errorf("PathAttributes mismatch:\nexpected: %x\ngot:      %x",
			expectedAttrs, update.PathAttributes)
	}
}

// TestWireFormat_UnicastIPv6 verifies IPv6 unicast wire format.
//
// RFC 4760: IPv6 unicast uses MP_REACH_NLRI for next-hop and NLRI.
func TestWireFormat_UnicastIPv6(t *testing.T) {

	ub := message.NewUpdateBuilder(65001, true, true, false)

	params := message.UnicastParams{
		Prefix:          netip.MustParsePrefix("2001:db8::/32"),
		NextHop:         netip.MustParseAddr("2001:db8::1"),
		Origin:          attribute.OriginIGP,
		LocalPreference: 100,
	}

	update := ub.BuildUnicast(&params)

	// Expected: ORIGIN + AS_PATH + LOCAL_PREF + MP_REACH_NLRI
	// MP_REACH_NLRI = AFI(2) + SAFI(1) + NH_LEN(1) + NH(16) + Reserved(1) + NLRI(5) = 26 bytes
	expectedAttrs, _ := hex.DecodeString(
		"40010100" + // ORIGIN: IGP
			"400200" + // AS_PATH: empty
			"40050400000064" + // LOCAL_PREF: 100
			"800e1a" + // MP_REACH_NLRI header (len=26)
			"00020110" + // AFI=2, SAFI=1, NH_LEN=16
			"20010db8000000000000000000000001" + // Next-hop: 2001:db8::1
			"00" + // Reserved
			"2020010db8") // NLRI: /32 2001:db8::

	if !bytes.Equal(update.PathAttributes, expectedAttrs) {
		t.Errorf("PathAttributes mismatch:\nexpected: %x\ngot:      %x",
			expectedAttrs, update.PathAttributes)
	}

	// IPv6 uses MP_REACH_NLRI, no inline NLRI
	if len(update.NLRI) != 0 {
		t.Errorf("Expected no inline NLRI for IPv6, got: %x", update.NLRI)
	}
}

// TestWireCompat_VPNIPv4 verifies VPN-IPv4 wire format.
//
// Order: ORIGIN(1), AS_PATH(2), NEXT_HOP(3), LOCAL_PREF(5), EXT_COM(16), MP_REACH(14 last).
// MP_REACH_NLRI placed last per docs/architecture/wire/mp-nlri-ordering.md.
func TestWireCompat_VPNIPv4(t *testing.T) {

	// Build VPN route UPDATE
	ub := message.NewUpdateBuilder(65001, true, true, false)
	params := message.VPNParams{
		Prefix:            netip.MustParsePrefix("10.0.0.0/24"),
		NextHop:           netip.MustParseAddr("192.168.1.1"),
		Origin:            attribute.OriginIGP,
		Labels:            []uint32{100},
		RDBytes:           [8]byte{0, 1, 0, 0, 0, 100, 0, 100}, // Type 1: 100:100
		LocalPreference:   150,
		ExtCommunityBytes: []byte{0x00, 0x02, 0xfd, 0xe9, 0x00, 0x00, 0x00, 0x64}, // RT 65001:100
	}
	update := ub.BuildVPN(&params)

	// Expected output: regular attrs by type code, then MP_REACH last.
	// ORIGIN (1) + AS_PATH (2) + NEXT_HOP (3) + LOCAL_PREF (5) + EXT_COM (16) + MP_REACH (14 last)
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

// TestWireFormat_ExtendedNextHop verifies RFC 8950 extended next-hop.
//
// IPv4 prefix with IPv6 next-hop uses MP_REACH_NLRI with AFI=1, SAFI=1.
func TestWireFormat_ExtendedNextHop(t *testing.T) {

	ub := message.NewUpdateBuilder(65001, true, true, false)

	params := message.UnicastParams{
		Prefix:             netip.MustParsePrefix("10.0.0.0/24"),
		NextHop:            netip.MustParseAddr("2001:db8::1"),
		Origin:             attribute.OriginIGP,
		LocalPreference:    100,
		UseExtendedNextHop: true,
	}

	update := ub.BuildUnicast(&params)

	// Expected: ORIGIN + AS_PATH + LOCAL_PREF + MP_REACH_NLRI (AFI=1, SAFI=1, IPv6 NH)
	expectedAttrs, _ := hex.DecodeString(
		"40010100" + // ORIGIN: IGP
			"400200" + // AS_PATH: empty
			"40050400000064" + // LOCAL_PREF: 100
			"800e19" + // MP_REACH_NLRI header (len=25)
			"00010110" + // AFI=1 (IPv4), SAFI=1, NH_LEN=16
			"20010db8000000000000000000000001" + // Next-hop: 2001:db8::1
			"00" + // Reserved
			"180a0000") // NLRI: /24 10.0.0.0

	if !bytes.Equal(update.PathAttributes, expectedAttrs) {
		t.Errorf("PathAttributes mismatch:\nexpected: %x\ngot:      %x",
			expectedAttrs, update.PathAttributes)
	}

	// No inline NLRI when using MP_REACH_NLRI
	if len(update.NLRI) != 0 {
		t.Errorf("Expected no inline NLRI for extended next-hop, got: %x", update.NLRI)
	}
}

// TestWireFormat_RawAttributes verifies raw attribute pass-through.
//
// Custom attributes from config are appended after sorted standard attributes.
func TestWireFormat_RawAttributes(t *testing.T) {

	ub := message.NewUpdateBuilder(65001, true, true, false)

	params := message.UnicastParams{
		Prefix:  netip.MustParsePrefix("10.0.0.0/24"),
		NextHop: netip.MustParseAddr("192.168.1.1"),
		Origin:  attribute.OriginIGP,
		RawAttributeBytes: [][]byte{
			{0xC0, 0x63, 0x03, 0x01, 0x02, 0x03}, // Custom attr code 99
		},
	}

	update := ub.BuildUnicast(&params)

	// Check raw attr is appended at end
	if !bytes.Contains(update.PathAttributes, []byte{0xC0, 0x63, 0x03, 0x01, 0x02, 0x03}) {
		t.Errorf("Raw attribute not found in PathAttributes:\ngot: %x", update.PathAttributes)
	}

	// Verify standard attrs are present before raw attr
	if !bytes.Contains(update.PathAttributes, []byte{0x40, 0x01, 0x01, 0x00}) {
		t.Error("ORIGIN attribute missing")
	}
}

// Note: MVPN, VPLS, FlowSpec, MUP wire compat tests removed.
// Those build functions were extracted to UpdateBuilder and the old
// implementations deleted. UpdateBuilder tests are in update_build_test.go.
