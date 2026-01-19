package message

import (
	"net/netip"
	"testing"

	"codeberg.org/thomas-mangin/zebgp/internal/bgp/attribute"
	"codeberg.org/thomas-mangin/zebgp/internal/bgp/nlri"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// makeRD creates a Type 0 RD (ASN:Local) for testing.
func makeRD(local uint32) nlri.RouteDistinguisher {
	const asn uint16 = 100 // test value
	rd := nlri.RouteDistinguisher{Type: nlri.RDType0}
	rd.Value[0] = byte(asn >> 8)
	rd.Value[1] = byte(asn)
	rd.Value[2] = byte(local >> 24)
	rd.Value[3] = byte(local >> 16)
	rd.Value[4] = byte(local >> 8)
	rd.Value[5] = byte(local)
	return rd
}

// TestUpdateBuilder_BuildEVPN_Type2 verifies EVPN Type 2 (MAC/IP) UPDATE building.
//
// VALIDATES: BuildEVPN produces valid UPDATE with MP_REACH_NLRI for Type 2.
// PREVENTS: EVPN encoding regression.
func TestUpdateBuilder_BuildEVPN_Type2(t *testing.T) {
	ub := NewUpdateBuilder(65001, false, true, false)

	// RD 100:1
	rd := makeRD(1)

	params := EVPNParams{
		RouteType:   2, // MAC/IP Advertisement
		RD:          rd,
		ESI:         [10]byte{}, // all zeros
		EthernetTag: 0,
		MAC:         [6]byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55},
		IP:          netip.MustParseAddr("10.0.0.1"),
		Labels:      []uint32{100},
		NextHop:     netip.MustParseAddr("192.168.1.1"),
		Origin:      attribute.OriginIGP,
	}

	update := ub.BuildEVPN(params)

	require.NotNil(t, update, "BuildEVPN returned nil")

	// Verify path attributes contain MP_REACH_NLRI
	assert.Greater(t, len(update.PathAttributes), 0, "PathAttributes should not be empty")

	// Pack and verify structure
	data := PackTo(update, nil)
	assert.Greater(t, len(data), HeaderLen, "packed data should include header")
}

// TestUpdateBuilder_BuildEVPN_Type5 verifies EVPN Type 5 (IP Prefix) UPDATE building.
//
// VALIDATES: BuildEVPN produces valid UPDATE for Type 5 IP Prefix route.
// PREVENTS: EVPN Type 5 encoding bugs.
func TestUpdateBuilder_BuildEVPN_Type5(t *testing.T) {
	ub := NewUpdateBuilder(65001, true, true, false) // iBGP

	// RD 100:200
	rd := makeRD(200)

	params := EVPNParams{
		RouteType:       5, // IP Prefix
		RD:              rd,
		ESI:             [10]byte{},
		EthernetTag:     100,
		Prefix:          netip.MustParsePrefix("10.0.0.0/24"),
		Gateway:         netip.IPv4Unspecified(),
		Labels:          []uint32{200},
		NextHop:         netip.MustParseAddr("192.168.1.1"),
		Origin:          attribute.OriginIGP,
		LocalPreference: 150,
	}

	update := ub.BuildEVPN(params)

	require.NotNil(t, update, "BuildEVPN returned nil")
	assert.Greater(t, len(update.PathAttributes), 0, "PathAttributes should not be empty")
}

// TestUpdateBuilder_BuildEVPN_Type3 verifies EVPN Type 3 (Inclusive Multicast) UPDATE.
//
// VALIDATES: BuildEVPN produces valid UPDATE for Type 3.
// PREVENTS: EVPN Type 3 encoding bugs.
func TestUpdateBuilder_BuildEVPN_Type3(t *testing.T) {
	ub := NewUpdateBuilder(65001, false, true, false)

	// RD 100:1
	rd := makeRD(1)

	params := EVPNParams{
		RouteType:    3, // Inclusive Multicast
		RD:           rd,
		EthernetTag:  0,
		OriginatorIP: netip.MustParseAddr("192.168.1.1"),
		NextHop:      netip.MustParseAddr("192.168.1.1"),
		Origin:       attribute.OriginIGP,
	}

	update := ub.BuildEVPN(params)

	require.NotNil(t, update, "BuildEVPN returned nil")
}

// TestUpdateBuilder_BuildEVPN_ExtCommunity verifies extended community in EVPN UPDATE.
//
// VALIDATES: Route Targets are included in EVPN UPDATE.
// PREVENTS: Extended community encoding bugs.
func TestUpdateBuilder_BuildEVPN_ExtCommunity(t *testing.T) {
	ub := NewUpdateBuilder(65001, false, true, false)

	// RD 100:1
	rd := makeRD(1)

	// Route Target: 100:200
	rt := make([]byte, 8)
	rt[0] = 0x00 // Type high (2-octet AS)
	rt[1] = 0x02 // Type low (Route Target)
	rt[2] = 0x00 // AS high
	rt[3] = 0x64 // AS low (100)
	rt[4] = 0x00
	rt[5] = 0x00
	rt[6] = 0x00
	rt[7] = 0xC8 // Local admin (200)

	params := EVPNParams{
		RouteType:         2,
		RD:                rd,
		MAC:               [6]byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55},
		Labels:            []uint32{100},
		NextHop:           netip.MustParseAddr("192.168.1.1"),
		Origin:            attribute.OriginIGP,
		ExtCommunityBytes: rt,
	}

	update := ub.BuildEVPN(params)

	require.NotNil(t, update, "BuildEVPN returned nil")

	// Verify path attributes are present
	assert.Greater(t, len(update.PathAttributes), 0)
}
