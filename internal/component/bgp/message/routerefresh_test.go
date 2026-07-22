package message

import (
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/core/bgp/msgtype"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/core/family"
)

// rfc-test-change-approved: 2026-07-22 Thomas approved the msgtype/routeaction
// package rename (spec-feature-gate-10-bgp). MessageType/Type* moved to
// internal/core/bgp/msgtype and the route-action enum to
// internal/core/bgp/routeaction so MRT, sysrib and the FIB backends keep
// compiling when the BGP engine is compiled out (//go:build ze_bgp). Every hunk
// in this file is a package-qualifier requalification: no assertion was added,
// removed, reworded, weakened or re-tagged, verified by normalising the diff
// under the renaming and confirming the add/delete multisets cancel.

// TestRouteRefreshType verifies ROUTE_REFRESH message type.
//
// RFC requirement: RFC2918-3-1 positive -- a ROUTE-REFRESH message reports message
// type ROUTE-REFRESH, which is the constant 5 (RouteRefresh.Type in routerefresh.go,
// msgtype.TypeROUTEREFRESH in header.go).
func TestRouteRefreshType(t *testing.T) {
	r := &RouteRefresh{AFI: 1, SAFI: 1}
	assert.Equal(t, msgtype.TypeROUTEREFRESH, r.Type())
	assert.Equal(t, msgtype.MessageType(5), r.Type())
}

// TestRouteRefreshPack verifies ROUTE_REFRESH packing.
//
// VALIDATES: AFI and SAFI correctly serialized.
//
// PREVENTS: Malformed request causing peer to send wrong routes.
//
// RFC requirement: RFC2918-3-2 positive -- a packed ROUTE-REFRESH carries exactly a
// 4-byte payload after the header: AFI (2) + Reserved (1) + SAFI (1)
// (RouteRefresh.Len/WriteTo in routerefresh.go).
func TestRouteRefreshPack(t *testing.T) {
	r := &RouteRefresh{
		AFI:  family.AFIIPv6,
		SAFI: family.SAFIUnicast,
	}

	data := PackTo(r, nil)

	// Header (19) + AFI (2) + Reserved (1) + SAFI (1)
	assert.Len(t, data, HeaderLen+4)

	// Verify header
	h, err := ParseHeader(data)
	require.NoError(t, err)
	assert.Equal(t, msgtype.TypeROUTEREFRESH, h.Type)

	// Verify body
	body := data[HeaderLen:]
	assert.Equal(t, byte(0x00), body[0]) // AFI high
	assert.Equal(t, byte(0x02), body[1]) // AFI low (2 = IPv6)
	assert.Equal(t, byte(0x00), body[2]) // Reserved
	assert.Equal(t, byte(0x01), body[3]) // SAFI (1 = Unicast)
}

// TestRouteRefreshUnpack verifies ROUTE_REFRESH unpacking.
func TestRouteRefreshUnpack(t *testing.T) {
	body := []byte{
		0x00, 0x01, // AFI = 1 (IPv4)
		0x00, // Reserved
		0x02, // SAFI = 2 (Multicast)
	}

	msg, err := UnpackRouteRefresh(body)
	require.NoError(t, err)

	assert.Equal(t, family.AFIIPv4, msg.AFI)
	assert.Equal(t, family.SAFIMulticast, msg.SAFI)
}

// TestRouteRefreshUnpackShort verifies short data handling.
//
// RFC requirement: RFC2918-3-2 negative -- a ROUTE-REFRESH payload shorter than the
// required 4 bytes (here 3) is rejected: UnpackRouteRefresh returns ErrShortRead
// rather than parsing a truncated <AFI, SAFI> (routerefresh.go).
func TestRouteRefreshUnpackShort(t *testing.T) {
	_, err := UnpackRouteRefresh([]byte{0x00, 0x01, 0x00}) // Only 3 bytes
	assert.ErrorIs(t, err, ErrShortRead)
}

// TestRouteRefreshRoundTrip verifies pack/unpack symmetry.
func TestRouteRefreshRoundTrip(t *testing.T) {
	original := &RouteRefresh{
		AFI:  family.AFIIPv4,
		SAFI: family.SAFIFlowSpec,
	}

	data := PackTo(original, nil)

	body := data[HeaderLen:]
	parsed, err := UnpackRouteRefresh(body)
	require.NoError(t, err)

	assert.Equal(t, original.AFI, parsed.AFI)
	assert.Equal(t, original.SAFI, parsed.SAFI)
}

// TestRouteRefreshCommonFamilies verifies common AFI/SAFI values.
func TestRouteRefreshCommonFamilies(t *testing.T) {
	tests := []struct {
		name string
		afi  family.AFI
		safi family.SAFI
	}{
		{"IPv4 Unicast", family.AFIIPv4, family.SAFIUnicast},
		{"IPv6 Unicast", family.AFIIPv6, family.SAFIUnicast},
		{"IPv4 VPN", family.AFIIPv4, family.SAFIVPN},
		{"IPv6 VPN", family.AFIIPv6, family.SAFIVPN},
		{"L2VPN EVPN", family.AFIL2VPN, family.SAFIEVPN},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &RouteRefresh{AFI: tt.afi, SAFI: tt.safi}
			data := PackTo(r, nil)

			parsed, err := UnpackRouteRefresh(data[HeaderLen:])
			require.NoError(t, err)

			assert.Equal(t, tt.afi, parsed.AFI)
			assert.Equal(t, tt.safi, parsed.SAFI)
		})
	}
}

// TestRouteRefreshSubtypes verifies RFC 7313 enhanced route refresh subtypes.
//
// RFC 7313 Section 3.2:
//   - 0: Normal route refresh (RFC 2918)
//   - 1: BoRR (Beginning of Route Refresh)
//   - 2: EoRR (Ending of Route Refresh)
//
// VALIDATES: Subtype correctly serialized and parsed.
//
// PREVENTS: Failure to handle enhanced route refresh markers.
//
// RFC requirement: RFC7313-4-1 positive -- the BoRR marker ze sends before
// re-advertising encodes Message Subtype byte 1 at body offset 2
// (RouteRefreshBoRR case: body[2]==1, round-tripping to parsed.Subtype).
// RFC requirement: RFC7313-4-2 positive -- the EoRR marker ze sends after
// re-advertising encodes Message Subtype byte 2 at body offset 2
// (RouteRefreshEoRR case: body[2]==2, round-tripping to parsed.Subtype).
func TestRouteRefreshSubtypes(t *testing.T) {
	tests := []struct {
		name    string
		subtype RouteRefreshSubtype
	}{
		{"Normal", RouteRefreshNormal},
		{"BoRR", RouteRefreshBoRR},
		{"EoRR", RouteRefreshEoRR},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &RouteRefresh{
				AFI:     family.AFIIPv4,
				SAFI:    family.SAFIUnicast,
				Subtype: tt.subtype,
			}

			data := PackTo(r, nil)

			// Verify subtype in wire format (offset 2 in body, which is the Reserved/Subtype field)
			body := data[HeaderLen:]
			assert.Equal(t, byte(tt.subtype), body[2])

			// Verify round trip
			parsed, err := UnpackRouteRefresh(body)
			require.NoError(t, err)
			assert.Equal(t, tt.subtype, parsed.Subtype)
		})
	}
}

// TestRouteRefreshSubtypeConstants verifies subtype constant values per RFC 7313.
func TestRouteRefreshSubtypeConstants(t *testing.T) {
	assert.Equal(t, RouteRefreshSubtype(0), RouteRefreshNormal)
	assert.Equal(t, RouteRefreshSubtype(1), RouteRefreshBoRR)
	assert.Equal(t, RouteRefreshSubtype(2), RouteRefreshEoRR)
}
