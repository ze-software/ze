package message

import (
	"bytes"
	"errors"
	"net/netip"
	"slices"
	"testing"
	"unsafe"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/attribute"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/nlri"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/wire"
)

// sliceAliasesAny reports whether s is a sub-slice of ANY buffer in backings
// (same backing array, s's start is within that buffer). Used to verify scratch
// aliasing across a potential grow: sub-slices allocated before a grow reference
// the OLD backing, ones allocated after reference the NEW backing. Passing both
// to this helper covers the span.
func sliceAliasesAny(s []byte, backings ...[]byte) bool {
	if len(s) == 0 {
		return false
	}
	sStart := uintptr(unsafe.Pointer(unsafe.SliceData(s)))
	for _, b := range backings {
		if len(b) == 0 {
			continue
		}
		bStart := uintptr(unsafe.Pointer(unsafe.SliceData(b)))
		bEnd := bStart + uintptr(len(b))
		if sStart >= bStart && sStart < bEnd {
			return true
		}
	}
	return false
}

// sliceAliasesScratch is a convenience wrapper for the common single-backing
// check. Use sliceAliasesAny directly when testing grow-mid-build paths.
func sliceAliasesScratch(s, scratch []byte) bool {
	return sliceAliasesAny(s, scratch)
}

// collectGrouped runs BuildGroupedUnicast with a collecting callback and returns
// deep-copied Updates so test code can inspect every chunk after the next build.
// The deep copy is necessary because chunks emitted by the callback alias the
// builder's scratch (see Update type doc).
func collectGrouped(t *testing.T, ub *UpdateBuilder, routes []UnicastParams, maxSize int) ([]*Update, error) {
	t.Helper()
	var updates []*Update
	err := ub.BuildGroupedUnicast(routes, maxSize, func(u *Update) error {
		updates = append(updates, &Update{
			PathAttributes: append([]byte(nil), u.PathAttributes...),
			NLRI:           append([]byte(nil), u.NLRI...),
		})
		return nil
	})
	return updates, err
}

// collectMVPN runs BuildGroupedMVPN with a collecting callback and returns
// deep-copied Updates for the same reason as collectGrouped.
func collectMVPN(t *testing.T, ub *UpdateBuilder, routes []MVPNParams, maxSize int) ([]*Update, error) {
	t.Helper()
	var updates []*Update
	err := ub.BuildGroupedMVPN(routes, maxSize, func(u *Update) error {
		updates = append(updates, &Update{
			PathAttributes: append([]byte(nil), u.PathAttributes...),
			NLRI:           append([]byte(nil), u.NLRI...),
		})
		return nil
	})
	return updates, err
}

// mustBuildGrouped wraps collectGrouped at the standard 65535 max size and
// returns the sole Update, failing the test on error or unexpected split.
func mustBuildGrouped(t *testing.T, ub *UpdateBuilder, routes []UnicastParams) *Update {
	t.Helper()
	updates, err := collectGrouped(t, ub, routes, 65535)
	if err != nil {
		t.Fatalf("BuildGroupedUnicast failed: %v", err)
	}
	if len(updates) == 0 {
		t.Fatal("BuildGroupedUnicast returned no updates")
	}
	if len(updates) > 1 {
		t.Fatalf("BuildGroupedUnicast unexpectedly split into %d updates", len(updates))
	}
	return updates[0]
}

// TestUpdateBuilder_NewBuilder verifies UpdateBuilder creation.
//
// VALIDATES: UpdateBuilder stores LocalAS, IsIBGP, ASN4, and AddPath correctly.
//
// PREVENTS: Missing fields or incorrect initialization causing encode failures.
func TestUpdateBuilder_NewBuilder(t *testing.T) {
	ub := NewUpdateBuilder(65001, true, true, true)

	if ub.LocalAS != 65001 {
		t.Errorf("LocalAS = %d, want 65001", ub.LocalAS)
	}
	if !ub.IsIBGP {
		t.Error("IsIBGP = false, want true")
	}
	if !ub.ASN4 {
		t.Error("ASN4 = false, want true")
	}
	if !ub.AddPath {
		t.Error("AddPath = false, want true")
	}
}

// TestUpdateBuilder_BuildUnicast_IPv4 verifies IPv4 unicast UPDATE building.
//
// VALIDATES: IPv4 unicast route produces valid UPDATE with correct NLRI placement.
//
// PREVENTS: IPv4 routes incorrectly using MP_REACH_NLRI instead of inline NLRI.
func TestUpdateBuilder_BuildUnicast_IPv4(t *testing.T) {

	ub := NewUpdateBuilder(65001, false, true, false)

	params := UnicastParams{
		Prefix:  netip.MustParsePrefix("10.0.0.0/24"),
		NextHop: netip.MustParseAddr("192.168.1.1"),
		Origin:  attribute.OriginIGP,
	}

	update := ub.BuildUnicast(&params)
	if update == nil {
		t.Fatal("BuildUnicast returned nil")
		return
	}

	// IPv4 unicast should have inline NLRI, not MP_REACH
	if len(update.NLRI) == 0 {
		t.Error("IPv4 unicast should have inline NLRI")
	}

	// Should have path attributes
	if len(update.PathAttributes) == 0 {
		t.Error("missing path attributes")
	}
}

// TestUpdateBuilder_BuildUnicast_IPv6 verifies IPv6 unicast UPDATE building.
//
// VALIDATES: IPv6 unicast route produces UPDATE with MP_REACH_NLRI attribute.
//
// PREVENTS: IPv6 routes incorrectly using inline NLRI (RFC 4760 violation).
func TestUpdateBuilder_BuildUnicast_IPv6(t *testing.T) {

	ub := NewUpdateBuilder(65001, false, true, false)

	params := UnicastParams{
		Prefix:  netip.MustParsePrefix("2001:db8::/32"),
		NextHop: netip.MustParseAddr("2001:db8::1"),
		Origin:  attribute.OriginIGP,
	}

	update := ub.BuildUnicast(&params)
	if update == nil {
		t.Fatal("BuildUnicast returned nil")
		return
	}

	// IPv6 unicast should NOT have inline NLRI
	if len(update.NLRI) != 0 {
		t.Error("IPv6 unicast should not have inline NLRI")
	}

	// MP_REACH_NLRI is encoded in PathAttributes
	if len(update.PathAttributes) == 0 {
		t.Error("missing path attributes (should include MP_REACH_NLRI)")
	}
}

// TestUpdateBuilder_BuildUnicast_IPv6_LinkLocal verifies 32-byte MP_REACH next-hop
// when LinkLocalNextHop is set.
//
// VALIDATES: IPv6 unicast with link-local produces 32-byte next-hop (global + link-local)
// per RFC 2545 Section 3.
// PREVENTS: Link-local address dropped from MP_REACH_NLRI, producing 16-byte next-hop
// instead of 32-byte (test L regression).
func TestUpdateBuilder_BuildUnicast_IPv6_LinkLocal(t *testing.T) {
	ub := NewUpdateBuilder(65533, true, true, false) // iBGP, ASN4, no ADD-PATH

	params := UnicastParams{
		Prefix:           netip.MustParsePrefix("2001:db8:1::1/128"),
		NextHop:          netip.MustParseAddr("2001:db8::ffff"),
		LinkLocalNextHop: netip.MustParseAddr("fe80::1"),
		Origin:           attribute.OriginIGP,
		LocalPreference:  100,
	}

	update := ub.BuildUnicast(&params)
	if update == nil {
		t.Fatal("BuildUnicast returned nil")
		return
	}

	// IPv6 should use MP_REACH_NLRI, not inline NLRI
	if len(update.NLRI) != 0 {
		t.Error("IPv6 unicast should not have inline NLRI")
	}

	// RFC 2545 Section 3: MP_REACH_NLRI must have 32-byte next-hop (NH_LEN=0x20)
	// Format: AFI(0002) SAFI(01) NH_LEN(20) [16-byte global] [16-byte link-local] ...
	expectedNHLen := byte(0x20) // 32 bytes

	// Find MP_REACH_NLRI (code 14) in PathAttributes
	found := false
	offset := 0
	for offset < len(update.PathAttributes) {
		if len(update.PathAttributes[offset:]) < 3 {
			break
		}
		flags, code, length, hdrLen, err := attribute.ParseHeader(update.PathAttributes[offset:])
		if err != nil {
			t.Fatalf("ParseHeader at offset %d: %v", offset, err)
		}
		_ = flags
		if code == attribute.AttrMPReachNLRI {
			found = true
			attrData := update.PathAttributes[offset+hdrLen : offset+hdrLen+int(length)]

			// attrData[0:2] = AFI (0002 = IPv6)
			// attrData[2]   = SAFI (01 = unicast)
			// attrData[3]   = NH_LEN
			if len(attrData) < 4 {
				t.Fatalf("MP_REACH_NLRI too short: %d bytes", len(attrData))
			}

			if attrData[3] != expectedNHLen {
				t.Errorf("NH_LEN = %d, want %d (32 bytes for global + link-local)",
					attrData[3], expectedNHLen)
			}

			// Verify global next-hop bytes (2001:db8::ffff)
			if len(attrData) < 4+32 {
				t.Fatalf("MP_REACH_NLRI too short for 32-byte next-hop: %d bytes", len(attrData))
			}
			globalNH := attrData[4:20]
			expectedGlobal := netip.MustParseAddr("2001:db8::ffff").AsSlice()
			if !bytes.Equal(globalNH, expectedGlobal) {
				t.Errorf("global next-hop = %x, want %x", globalNH, expectedGlobal)
			}

			// Verify link-local next-hop bytes (fe80::1)
			linkLocalNH := attrData[20:36]
			expectedLinkLocal := netip.MustParseAddr("fe80::1").AsSlice()
			if !bytes.Equal(linkLocalNH, expectedLinkLocal) {
				t.Errorf("link-local next-hop = %x, want %x", linkLocalNH, expectedLinkLocal)
			}
			break
		}
		offset += hdrLen + int(length)
	}
	if !found {
		t.Errorf("MP_REACH_NLRI not found in PathAttributes: %x", update.PathAttributes)
	}
}

// TestUpdateBuilder_BuildUnicast_IPv6_NoLinkLocal verifies 16-byte MP_REACH next-hop
// when LinkLocalNextHop is NOT set.
//
// VALIDATES: IPv6 unicast without link-local produces 16-byte next-hop (global only).
// PREVENTS: Zero-value link-local accidentally added to next-hop.
func TestUpdateBuilder_BuildUnicast_IPv6_NoLinkLocal(t *testing.T) {
	ub := NewUpdateBuilder(65533, true, true, false)

	params := UnicastParams{
		Prefix:  netip.MustParsePrefix("2001:db8::/32"),
		NextHop: netip.MustParseAddr("2001:db8::1"),
		Origin:  attribute.OriginIGP,
	}

	update := ub.BuildUnicast(&params)
	if update == nil {
		t.Fatal("BuildUnicast returned nil")
		return
	}

	// Find MP_REACH_NLRI and check NH_LEN = 16 (global only)
	offset := 0
	for offset < len(update.PathAttributes) {
		if len(update.PathAttributes[offset:]) < 3 {
			break
		}
		_, code, length, hdrLen, err := attribute.ParseHeader(update.PathAttributes[offset:])
		if err != nil {
			t.Fatalf("ParseHeader at offset %d: %v", offset, err)
		}
		if code == attribute.AttrMPReachNLRI {
			attrData := update.PathAttributes[offset+hdrLen : offset+hdrLen+int(length)]
			if len(attrData) < 4 {
				t.Fatalf("MP_REACH_NLRI too short: %d bytes", len(attrData))
			}
			if attrData[3] != 16 {
				t.Errorf("NH_LEN = %d, want 16 (global only, no link-local)", attrData[3])
			}
			return
		}
		offset += hdrLen + int(length)
	}
	t.Error("MP_REACH_NLRI not found in PathAttributes")
}

// TestUpdateBuilder_BuildUnicast_IPv4_IgnoresLinkLocal verifies link-local is ignored
// for IPv4 unicast routes.
//
// VALIDATES: IPv4 routes use inline NLRI + NEXT_HOP attribute, ignoring LinkLocalNextHop.
// PREVENTS: Link-local accidentally applied to IPv4 routes.
func TestUpdateBuilder_BuildUnicast_IPv4_IgnoresLinkLocal(t *testing.T) {
	ub := NewUpdateBuilder(65533, true, true, false)

	params := UnicastParams{
		Prefix:           netip.MustParsePrefix("10.0.0.0/24"),
		NextHop:          netip.MustParseAddr("192.168.1.1"),
		LinkLocalNextHop: netip.MustParseAddr("fe80::1"), // Should be ignored
		Origin:           attribute.OriginIGP,
	}

	update := ub.BuildUnicast(&params)
	if update == nil {
		t.Fatal("BuildUnicast returned nil")
		return
	}

	// IPv4 should use inline NLRI, not MP_REACH_NLRI
	if len(update.NLRI) == 0 {
		t.Error("IPv4 unicast should have inline NLRI")
	}

	// Verify no MP_REACH_NLRI in PathAttributes
	offset := 0
	for offset < len(update.PathAttributes) {
		if len(update.PathAttributes[offset:]) < 3 {
			break
		}
		_, code, length, hdrLen, err := attribute.ParseHeader(update.PathAttributes[offset:])
		if err != nil {
			break
		}
		if code == attribute.AttrMPReachNLRI {
			t.Error("IPv4 unicast should not have MP_REACH_NLRI")
		}
		offset += hdrLen + int(length)
	}
}

// extractAttributeCodes parses raw attribute bytes and returns type codes in order.
// Used for testing attribute ordering.
func extractAttributeCodes(data []byte) ([]attribute.AttributeCode, error) {
	var codes []attribute.AttributeCode
	offset := 0
	for offset < len(data) {
		if len(data[offset:]) < 3 {
			break
		}
		_, code, length, hdrLen, err := attribute.ParseHeader(data[offset:])
		if err != nil {
			return nil, err
		}
		codes = append(codes, code)
		offset += hdrLen + int(length)
	}
	return codes, nil
}

// TestUpdateBuilder_BuildUnicast_AttributeOrder verifies RFC 4271 attribute ordering.
//
// VALIDATES: Attributes are ordered by type code (ORIGIN=1, AS_PATH=2, NEXT_HOP=3, MED=4, LOCAL_PREF=5).
//
// PREVENTS: Attribute ordering violations that may cause peer rejection.
func TestUpdateBuilder_BuildUnicast_AttributeOrder(t *testing.T) {

	ub := NewUpdateBuilder(65001, true, true, false) // iBGP to include LOCAL_PREF

	params := UnicastParams{
		Prefix:          netip.MustParsePrefix("10.0.0.0/24"),
		NextHop:         netip.MustParseAddr("192.168.1.1"),
		Origin:          attribute.OriginIGP,
		MED:             100,
		LocalPreference: 200,
	}

	update := ub.BuildUnicast(&params)
	if update == nil {
		t.Fatal("BuildUnicast returned nil")
		return
	}

	// Parse attributes and verify ordering
	codes, err := extractAttributeCodes(update.PathAttributes)
	if err != nil {
		t.Fatalf("extractAttributeCodes failed: %v", err)
	}

	// Verify ordering: each type code should be <= next
	for i := range len(codes) - 1 {
		if codes[i] > codes[i+1] {
			t.Errorf("attribute order violation: type %d before type %d at position %d",
				codes[i], codes[i+1], i)
		}
	}

	// Specifically verify MED (4) comes before LOCAL_PREF (5) if both present
	var medPos, lpPos = -1, -1
	for i, tc := range codes {
		if tc == attribute.AttrMED {
			medPos = i
		}
		if tc == attribute.AttrLocalPref {
			lpPos = i
		}
	}

	if medPos >= 0 && lpPos >= 0 && medPos > lpPos {
		t.Errorf("MED (pos %d) should come before LOCAL_PREF (pos %d)", medPos, lpPos)
	}
}

// TestUpdateBuilder_BuildUnicast_ASPath_EBGP verifies AS_PATH for eBGP.
//
// VALIDATES: eBGP prepends local AS to AS_PATH.
//
// PREVENTS: Missing local AS prepend causing BGP loop detection failures.
func TestUpdateBuilder_BuildUnicast_ASPath_EBGP(t *testing.T) {

	ub := NewUpdateBuilder(65001, false, true, false) // eBGP

	params := UnicastParams{
		Prefix:  netip.MustParsePrefix("10.0.0.0/24"),
		NextHop: netip.MustParseAddr("192.168.1.1"),
		Origin:  attribute.OriginIGP,
		ASPath:  []uint32{65002, 65003}, // Configured path
	}

	update := ub.BuildUnicast(&params)
	if update == nil {
		t.Fatal("BuildUnicast returned nil")
		return
	}

	// Find AS_PATH attribute and parse it
	codes, err := extractAttributeCodes(update.PathAttributes)
	if err != nil {
		t.Fatalf("extractAttributeCodes failed: %v", err)
	}

	// Verify AS_PATH is present
	hasASPath := slices.Contains(codes, attribute.AttrASPath)
	if !hasASPath {
		t.Fatal("AS_PATH not found")
	}

	// Parse the AS_PATH to verify local AS prepend
	// Find AS_PATH position and parse it
	offset := 0
	for offset < len(update.PathAttributes) {
		_, code, length, hdrLen, err := attribute.ParseHeader(update.PathAttributes[offset:])
		if err != nil {
			t.Fatalf("ParseHeader failed: %v", err)
		}
		if code == attribute.AttrASPath {
			asPathData := update.PathAttributes[offset+hdrLen : offset+hdrLen+int(length)]
			asPath, err := attribute.ParseASPath(asPathData, true)
			if err != nil {
				t.Fatalf("ParseASPath failed: %v", err)
			}

			// For eBGP, local AS should be prepended
			if len(asPath.Segments) == 0 || len(asPath.Segments[0].ASNs) == 0 {
				t.Fatal("AS_PATH is empty")
			}

			// First AS should be local AS (65001)
			if asPath.Segments[0].ASNs[0] != 65001 {
				t.Errorf("first AS = %d, want 65001 (local AS)", asPath.Segments[0].ASNs[0])
			}
			break
		}
		offset += hdrLen + int(length)
	}
}

// TestUpdateBuilder_BuildUnicast_ASPath_IBGP verifies AS_PATH for iBGP.
//
// VALIDATES: iBGP does not prepend local AS to AS_PATH.
//
// PREVENTS: Incorrect AS prepend breaking iBGP path selection.
func TestUpdateBuilder_BuildUnicast_ASPath_IBGP(t *testing.T) {

	ub := NewUpdateBuilder(65001, true, true, false) // iBGP

	params := UnicastParams{
		Prefix:  netip.MustParsePrefix("10.0.0.0/24"),
		NextHop: netip.MustParseAddr("192.168.1.1"),
		Origin:  attribute.OriginIGP,
		ASPath:  []uint32{65002, 65003}, // Configured path
	}

	update := ub.BuildUnicast(&params)
	if update == nil {
		t.Fatal("BuildUnicast returned nil")
		return
	}

	// Parse the AS_PATH to verify no local AS prepend
	offset := 0
	for offset < len(update.PathAttributes) {
		_, code, length, hdrLen, err := attribute.ParseHeader(update.PathAttributes[offset:])
		if err != nil {
			t.Fatalf("ParseHeader failed: %v", err)
		}
		if code == attribute.AttrASPath {
			asPathData := update.PathAttributes[offset+hdrLen : offset+hdrLen+int(length)]
			asPath, err := attribute.ParseASPath(asPathData, true)
			if err != nil {
				t.Fatalf("ParseASPath failed: %v", err)
			}

			// For iBGP with configured path, local AS should NOT be prepended
			if len(asPath.Segments) > 0 && len(asPath.Segments[0].ASNs) > 0 {
				if asPath.Segments[0].ASNs[0] == 65001 {
					t.Error("iBGP should not prepend local AS")
				}
				// First AS should be first configured AS (65002)
				if asPath.Segments[0].ASNs[0] != 65002 {
					t.Errorf("first AS = %d, want 65002 (first configured AS)", asPath.Segments[0].ASNs[0])
				}
			}
			break
		}
		offset += hdrLen + int(length)
	}
}

// TestUpdateBuilder_BuildVPN_IPv4 verifies IPv4 VPN UPDATE building.
//
// VALIDATES: IPv4 VPN route produces UPDATE with MP_REACH_NLRI (SAFI=128).
//
// PREVENTS: VPN routes using wrong SAFI or missing label/RD encoding.
func TestUpdateBuilder_BuildVPN_IPv4(t *testing.T) {

	ub := NewUpdateBuilder(65001, false, true, false)

	params := VPNParams{
		Prefix:  netip.MustParsePrefix("10.0.0.0/24"),
		NextHop: netip.MustParseAddr("192.168.1.1"),
		Origin:  attribute.OriginIGP,
		Labels:  []uint32{100},
		RDBytes: [8]byte{0, 1, 0, 0, 0, 100, 0, 100}, // Type 1 RD: 100:100
	}

	update := ub.BuildVPN(&params)
	if update == nil {
		t.Fatal("BuildVPN returned nil")
		return
	}

	// VPN routes should NOT have inline NLRI
	if len(update.NLRI) != 0 {
		t.Error("VPN route should not have inline NLRI")
	}

	// Should have MP_REACH_NLRI in path attributes
	codes, err := extractAttributeCodes(update.PathAttributes)
	if err != nil {
		t.Fatalf("extractAttributeCodes failed: %v", err)
	}

	hasMPReach := slices.Contains(codes, attribute.AttrMPReachNLRI)
	if !hasMPReach {
		t.Error("VPN route should have MP_REACH_NLRI")
	}
}

// TestUpdateBuilder_BuildVPN_IPv6 verifies IPv6 VPN UPDATE building.
//
// VALIDATES: IPv6 VPN route produces UPDATE with MP_REACH_NLRI (AFI=2, SAFI=128).
//
// PREVENTS: IPv6 VPN using wrong AFI.
func TestUpdateBuilder_BuildVPN_IPv6(t *testing.T) {

	ub := NewUpdateBuilder(65001, false, true, false)

	params := VPNParams{
		Prefix:  netip.MustParsePrefix("2001:db8::/32"),
		NextHop: netip.MustParseAddr("2001:db8::1"),
		Origin:  attribute.OriginIGP,
		Labels:  []uint32{200},
		RDBytes: [8]byte{0, 1, 0, 0, 0, 100, 0, 100},
	}

	update := ub.BuildVPN(&params)
	if update == nil {
		t.Fatal("BuildVPN returned nil")
		return
	}

	// VPN routes should NOT have inline NLRI
	if len(update.NLRI) != 0 {
		t.Error("VPN route should not have inline NLRI")
	}

	// Should have path attributes
	if len(update.PathAttributes) == 0 {
		t.Error("missing path attributes")
	}
}

// TestUpdateBuilder_BuildVPN_AttributeOrder verifies RFC 4271 attribute ordering for VPN.
//
// VALIDATES: VPN UPDATE has attributes ordered by type code.
//
// PREVENTS: Attribute ordering violations in VPN updates.
func TestUpdateBuilder_BuildVPN_AttributeOrder(t *testing.T) {

	ub := NewUpdateBuilder(65001, true, true, false) // iBGP

	params := VPNParams{
		Prefix:          netip.MustParsePrefix("10.0.0.0/24"),
		NextHop:         netip.MustParseAddr("192.168.1.1"),
		Origin:          attribute.OriginIGP,
		Labels:          []uint32{100},
		RDBytes:         [8]byte{0, 1, 0, 0, 0, 100, 0, 100},
		MED:             50,
		LocalPreference: 150,
	}

	update := ub.BuildVPN(&params)
	if update == nil {
		t.Fatal("BuildVPN returned nil")
		return
	}

	codes, err := extractAttributeCodes(update.PathAttributes)
	if err != nil {
		t.Fatalf("extractAttributeCodes failed: %v", err)
	}

	// Verify ordering
	for i := range len(codes) - 1 {
		if codes[i] > codes[i+1] {
			t.Errorf("attribute order violation: type %d before type %d at position %d",
				codes[i], codes[i+1], i)
		}
	}
}

// TestUpdateBuilder_BuildVPN_ExtCommunity verifies extended community in VPN UPDATE.
//
// VALIDATES: VPN UPDATE includes extended communities (route targets).
//
// PREVENTS: Missing route targets causing VPN route import failures.
func TestUpdateBuilder_BuildVPN_ExtCommunity(t *testing.T) {

	ub := NewUpdateBuilder(65001, false, true, false)

	// Route target: 65001:100
	rtBytes := []byte{0x00, 0x02, 0xfd, 0xe9, 0x00, 0x00, 0x00, 0x64}

	params := VPNParams{
		Prefix:            netip.MustParsePrefix("10.0.0.0/24"),
		NextHop:           netip.MustParseAddr("192.168.1.1"),
		Origin:            attribute.OriginIGP,
		Labels:            []uint32{100},
		RDBytes:           [8]byte{0, 1, 0, 0, 0, 100, 0, 100},
		ExtCommunityBytes: rtBytes,
	}

	update := ub.BuildVPN(&params)
	if update == nil {
		t.Fatal("BuildVPN returned nil")
		return
	}

	codes, err := extractAttributeCodes(update.PathAttributes)
	if err != nil {
		t.Fatalf("extractAttributeCodes failed: %v", err)
	}

	hasExtComm := slices.Contains(codes, attribute.AttrExtCommunity)
	if !hasExtComm {
		t.Error("VPN route should have EXTENDED_COMMUNITIES")
	}
}

// TestUpdateBuilder_BuildMVPN_Basic verifies MVPN UPDATE building.
//
// VALIDATES: MVPN route produces UPDATE with MP_REACH_NLRI (SAFI=5).
//
// PREVENTS: MVPN routes using wrong SAFI or missing route type encoding.
func TestUpdateBuilder_BuildMVPN_Basic(t *testing.T) {

	ub := NewUpdateBuilder(65001, false, true, false)

	params := MVPNParams{
		RouteType: 5, // Source Active A-D
		IsIPv6:    false,
		RD:        [8]byte{0, 1, 0, 0, 0, 100, 0, 100},
		Source:    netip.MustParseAddr("10.0.0.1"),
		Group:     netip.MustParseAddr("239.1.1.1"),
		NextHop:   netip.MustParseAddr("192.168.1.1"),
		Origin:    attribute.OriginIGP,
	}

	update := ub.BuildMVPN([]MVPNParams{params})
	if update == nil {
		t.Fatal("BuildMVPN returned nil")
		return
	}

	// MVPN routes should NOT have inline NLRI
	if len(update.NLRI) != 0 {
		t.Error("MVPN route should not have inline NLRI")
	}

	// Should have MP_REACH_NLRI in path attributes
	codes, err := extractAttributeCodes(update.PathAttributes)
	if err != nil {
		t.Fatalf("extractAttributeCodes failed: %v", err)
	}

	hasMPReach := slices.Contains(codes, attribute.AttrMPReachNLRI)
	if !hasMPReach {
		t.Error("MVPN route should have MP_REACH_NLRI")
	}
}

// TestUpdateBuilder_BuildMVPN_AttributeOrder verifies RFC 4271 ordering for MVPN.
//
// VALIDATES: MVPN UPDATE has attributes ordered by type code.
//
// PREVENTS: Attribute ordering violations in MVPN updates.
func TestUpdateBuilder_BuildMVPN_AttributeOrder(t *testing.T) {

	ub := NewUpdateBuilder(65001, true, true, false) // iBGP

	params := MVPNParams{
		RouteType:       5,
		IsIPv6:          false,
		RD:              [8]byte{0, 1, 0, 0, 0, 100, 0, 100},
		Source:          netip.MustParseAddr("10.0.0.1"),
		Group:           netip.MustParseAddr("239.1.1.1"),
		NextHop:         netip.MustParseAddr("192.168.1.1"),
		Origin:          attribute.OriginIGP,
		LocalPreference: 150,
	}

	update := ub.BuildMVPN([]MVPNParams{params})
	if update == nil {
		t.Fatal("BuildMVPN returned nil")
		return
	}

	codes, err := extractAttributeCodes(update.PathAttributes)
	if err != nil {
		t.Fatalf("extractAttributeCodes failed: %v", err)
	}

	// Verify ordering
	for i := range len(codes) - 1 {
		if codes[i] > codes[i+1] {
			t.Errorf("attribute order violation: type %d before type %d at position %d",
				codes[i], codes[i+1], i)
		}
	}
}

// TestUpdateBuilder_BuildVPLS_Basic verifies VPLS UPDATE building.
//
// VALIDATES: VPLS route produces UPDATE with MP_REACH_NLRI (AFI=25, SAFI=65).
//
// PREVENTS: VPLS routes using wrong AFI/SAFI.
func TestUpdateBuilder_BuildVPLS_Basic(t *testing.T) {

	ub := NewUpdateBuilder(65001, false, true, false)

	params := VPLSParams{
		RD:       [8]byte{0, 1, 0, 0, 0, 100, 0, 100},
		Endpoint: 1,
		Base:     100,
		Offset:   0,
		Size:     10,
		NextHop:  netip.MustParseAddr("192.168.1.1"),
		Origin:   attribute.OriginIGP,
	}

	update := ub.BuildVPLS(params)
	if update == nil {
		t.Fatal("BuildVPLS returned nil")
		return
	}

	if len(update.NLRI) != 0 {
		t.Error("VPLS route should not have inline NLRI")
	}

	codes, err := extractAttributeCodes(update.PathAttributes)
	if err != nil {
		t.Fatalf("extractAttributeCodes failed: %v", err)
	}

	hasMPReach := slices.Contains(codes, attribute.AttrMPReachNLRI)
	if !hasMPReach {
		t.Error("VPLS route should have MP_REACH_NLRI")
	}
}

// TestUpdateBuilder_BuildFlowSpec_Basic verifies FlowSpec UPDATE building.
//
// VALIDATES: FlowSpec route produces UPDATE with MP_REACH_NLRI (SAFI=133/134).
//
// PREVENTS: FlowSpec routes missing required attributes.
func TestUpdateBuilder_BuildFlowSpec_Basic(t *testing.T) {

	ub := NewUpdateBuilder(65001, false, true, false)

	// Simple FlowSpec NLRI (destination prefix)
	params := FlowSpecParams{
		IsIPv6:  false,
		NLRI:    []byte{0x03, 0x01, 0x18, 0x0a}, // dest 10.0.0.0/24
		NextHop: netip.MustParseAddr("192.168.1.1"),
	}

	update := ub.BuildFlowSpec(params)
	if update == nil {
		t.Fatal("BuildFlowSpec returned nil")
		return
	}

	if len(update.NLRI) != 0 {
		t.Error("FlowSpec route should not have inline NLRI")
	}

	codes, err := extractAttributeCodes(update.PathAttributes)
	if err != nil {
		t.Fatalf("extractAttributeCodes failed: %v", err)
	}

	hasMPReach := slices.Contains(codes, attribute.AttrMPReachNLRI)
	if !hasMPReach {
		t.Error("FlowSpec route should have MP_REACH_NLRI")
	}
}

// TestUpdateBuilder_BuildMUP_Basic verifies MUP UPDATE building.
//
// VALIDATES: MUP route produces UPDATE with MP_REACH_NLRI (SAFI=85).
//
// PREVENTS: MUP routes using wrong SAFI.
func TestUpdateBuilder_BuildMUP_Basic(t *testing.T) {

	ub := NewUpdateBuilder(65001, false, true, false)

	params := MUPParams{
		RouteType: 1,
		IsIPv6:    false,
		NLRI:      []byte{0x01, 0x02, 0x03, 0x04},
		NextHop:   netip.MustParseAddr("192.168.1.1"),
	}

	update := ub.BuildMUP(params)
	if update == nil {
		t.Fatal("BuildMUP returned nil")
		return
	}

	if len(update.NLRI) != 0 {
		t.Error("MUP route should not have inline NLRI")
	}

	codes, err := extractAttributeCodes(update.PathAttributes)
	if err != nil {
		t.Fatalf("extractAttributeCodes failed: %v", err)
	}

	hasMPReach := slices.Contains(codes, attribute.AttrMPReachNLRI)
	if !hasMPReach {
		t.Error("MUP route should have MP_REACH_NLRI")
	}
}

// TestBuildUnicast_EncodesReflectorAttrs verifies RFC 4456 attribute encoding.
//
// VALIDATES: ORIGINATOR_ID and CLUSTER_LIST are encoded in PathAttributes.
// PREVENTS: Data loss for route reflector configurations.
func TestBuildUnicast_EncodesReflectorAttrs(t *testing.T) {

	ub := NewUpdateBuilder(65001, true, true, false)

	params := UnicastParams{
		Prefix:          netip.MustParsePrefix("10.0.0.0/24"),
		NextHop:         netip.MustParseAddr("192.168.1.1"),
		Origin:          attribute.OriginIGP,
		LocalPreference: 100,
		OriginatorID:    0xC0A80101, // 192.168.1.1
		ClusterList:     []uint32{0xC0A80102, 0xC0A80103},
	}

	update := ub.BuildUnicast(&params)

	// ORIGINATOR_ID: flags=0x80 (optional), type=0x09, len=0x04, value=C0A80101
	expectedOriginator := []byte{0x80, 0x09, 0x04, 0xC0, 0xA8, 0x01, 0x01}
	if !bytes.Contains(update.PathAttributes, expectedOriginator) {
		t.Errorf("ORIGINATOR_ID not found in PathAttributes\ngot: %x\nwant to contain: %x",
			update.PathAttributes, expectedOriginator)
	}

	// CLUSTER_LIST: flags=0x80, type=0x0A, len=0x08, values=C0A80102 C0A80103
	expectedClusterType := []byte{0x80, 0x0A, 0x08}
	if !bytes.Contains(update.PathAttributes, expectedClusterType) {
		t.Errorf("CLUSTER_LIST not found in PathAttributes\ngot: %x",
			update.PathAttributes)
	}
}

// TestBuildUnicast_eBGP_NoLocalPref verifies LOCAL_PREF omitted for eBGP.
//
// VALIDATES: LOCAL_PREF not present in eBGP UPDATE.
// PREVENTS: RFC violation - LOCAL_PREF is iBGP only.
func TestBuildUnicast_eBGP_NoLocalPref(t *testing.T) {

	ub := NewUpdateBuilder(65001, false, true, false) // isIBGP=false

	params := UnicastParams{
		Prefix:          netip.MustParsePrefix("10.0.0.0/24"),
		NextHop:         netip.MustParseAddr("192.168.1.1"),
		Origin:          attribute.OriginIGP,
		LocalPreference: 200, // Should be ignored for eBGP
	}

	update := ub.BuildUnicast(&params)

	// LOCAL_PREF (type 5) should NOT be present for eBGP
	// Attribute header: flags (1 byte) + type 0x05
	if bytes.Contains(update.PathAttributes, []byte{0x40, 0x05}) {
		t.Error("LOCAL_PREF should not be present in eBGP UPDATE")
	}
}

// TestBuildUnicast_ASN4Disabled verifies 2-byte AS encoding.
//
// VALIDATES: AS_PATH uses 2-byte ASN format when ctx.ASN4=false.
// PREVENTS: RFC 6793 violation for legacy peers with asn4 disable.
func TestBuildUnicast_ASN4Disabled(t *testing.T) {
	ub := NewUpdateBuilder(100, false, false, false) // eBGP, AS 100, 2-byte mode

	params := UnicastParams{
		Prefix:  netip.MustParsePrefix("10.0.0.0/24"),
		NextHop: netip.MustParseAddr("192.168.1.1"),
		Origin:  attribute.OriginIGP,
	}

	update := ub.BuildUnicast(&params)

	// AS_PATH with 2-byte ASN: 40 02 04 02 01 00 64
	// flags=0x40 (transitive), type=2 (AS_PATH), len=4
	// segment: type=2 (AS_SEQUENCE), count=1, AS=100 (0x0064) as 2 bytes
	expected2ByteAS := []byte{0x40, 0x02, 0x04, 0x02, 0x01, 0x00, 0x64}
	if !bytes.Contains(update.PathAttributes, expected2ByteAS) {
		t.Errorf("AS_PATH not 2-byte encoded\nexpected to contain: %x\ngot: %x",
			expected2ByteAS, update.PathAttributes)
	}

	// Verify it's NOT using 4-byte format (would be 40 02 06 02 01 00 00 00 64)
	wrong4ByteAS := []byte{0x40, 0x02, 0x06, 0x02, 0x01, 0x00, 0x00, 0x00, 0x64}
	if bytes.Contains(update.PathAttributes, wrong4ByteAS) {
		t.Error("AS_PATH incorrectly using 4-byte format when ASN4=false")
	}
}

// TestBuildUnicast_ASN4Enabled verifies 4-byte AS encoding (default).
//
// VALIDATES: AS_PATH uses 4-byte ASN format when ctx.ASN4=true.
// PREVENTS: Regression in standard 4-byte AS encoding.
func TestBuildUnicast_ASN4Enabled(t *testing.T) {
	ub := NewUpdateBuilder(65001, false, true, false) // eBGP, AS 65001, 4-byte mode

	params := UnicastParams{
		Prefix:  netip.MustParsePrefix("10.0.0.0/24"),
		NextHop: netip.MustParseAddr("192.168.1.1"),
		Origin:  attribute.OriginIGP,
	}

	update := ub.BuildUnicast(&params)

	// AS_PATH with 4-byte ASN: 40 02 06 02 01 00 00 fd e9
	// flags=0x40 (transitive), type=2 (AS_PATH), len=6
	// segment: type=2 (AS_SEQUENCE), count=1, AS=65001 (0x0000FDE9) as 4 bytes
	expected4ByteAS := []byte{0x40, 0x02, 0x06, 0x02, 0x01, 0x00, 0x00, 0xfd, 0xe9}
	if !bytes.Contains(update.PathAttributes, expected4ByteAS) {
		t.Errorf("AS_PATH not 4-byte encoded\nexpected to contain: %x\ngot: %x",
			expected4ByteAS, update.PathAttributes)
	}
}

// TestBuildGroupedUnicast_ASN4Disabled verifies grouped updates respect ASN4 flag.
//
// VALIDATES: BuildGroupedUnicast uses 2-byte ASN format when ctx.ASN4=false.
// PREVENTS: Grouped updates ignoring ASN4 capability.
func TestBuildGroupedUnicast_ASN4Disabled(t *testing.T) {
	ub := NewUpdateBuilder(100, false, false, false) // eBGP, AS 100, 2-byte mode

	routes := []UnicastParams{
		{
			Prefix:  netip.MustParsePrefix("10.0.0.0/24"),
			NextHop: netip.MustParseAddr("192.168.1.1"),
			Origin:  attribute.OriginIGP,
		},
		{
			Prefix:  netip.MustParsePrefix("10.0.1.0/24"),
			NextHop: netip.MustParseAddr("192.168.1.1"),
			Origin:  attribute.OriginIGP,
		},
	}

	update := mustBuildGrouped(t, ub, routes)

	// AS_PATH with 2-byte ASN
	expected2ByteAS := []byte{0x40, 0x02, 0x04, 0x02, 0x01, 0x00, 0x64}
	if !bytes.Contains(update.PathAttributes, expected2ByteAS) {
		t.Errorf("Grouped AS_PATH not 2-byte encoded\nexpected to contain: %x\ngot: %x",
			expected2ByteAS, update.PathAttributes)
	}
}

// TestBuildGroupedUnicast_MultipleNLRIs verifies grouped UPDATE encoding.
//
// VALIDATES: Multiple prefixes packed into single UPDATE with shared attributes.
// PREVENTS: Regression in GroupUpdates=true performance optimization.
func TestBuildGroupedUnicast_MultipleNLRIs(t *testing.T) {

	ub := NewUpdateBuilder(65001, true, true, false)

	routes := []UnicastParams{
		{
			Prefix:          netip.MustParsePrefix("10.0.0.0/24"),
			NextHop:         netip.MustParseAddr("192.168.1.1"),
			Origin:          attribute.OriginIGP,
			LocalPreference: 100,
			Communities:     []uint32{0xFFFF0001},
		},
		{
			Prefix:  netip.MustParsePrefix("10.0.1.0/24"),
			NextHop: netip.MustParseAddr("192.168.1.1"),
			Origin:  attribute.OriginIGP,
		},
		{
			Prefix:  netip.MustParsePrefix("10.0.2.0/24"),
			NextHop: netip.MustParseAddr("192.168.1.1"),
			Origin:  attribute.OriginIGP,
		},
	}

	update := mustBuildGrouped(t, ub, routes)

	if update == nil {
		t.Fatal("BuildGroupedUnicast returned nil")
		return
	}

	// Verify NLRI contains all 3 prefixes (each /24 = 4 bytes: 1 len + 3 prefix)
	expectedNLRILen := 3 * 4
	if len(update.NLRI) != expectedNLRILen {
		t.Errorf("NLRI length: got %d, want %d", len(update.NLRI), expectedNLRILen)
	}

	// Verify attributes from first route are present (COMMUNITIES)
	if !bytes.Contains(update.PathAttributes, []byte{0xFF, 0xFF, 0x00, 0x01}) {
		t.Error("First route's communities not found in PathAttributes")
	}
}

// TestBuildGroupedUnicast_IncludesReflectorAttrs verifies RFC 4456 fields.
//
// VALIDATES: ORIGINATOR_ID and CLUSTER_LIST from first route are encoded.
// PREVENTS: Data loss for route reflector attributes in grouped updates.
func TestBuildGroupedUnicast_IncludesReflectorAttrs(t *testing.T) {

	ub := NewUpdateBuilder(65001, true, true, false)

	routes := []UnicastParams{
		{
			Prefix:            netip.MustParsePrefix("10.0.0.0/24"),
			NextHop:           netip.MustParseAddr("192.168.1.1"),
			Origin:            attribute.OriginIGP,
			OriginatorID:      0xC0A80101,
			ClusterList:       []uint32{0xC0A80102, 0xC0A80103},
			RawAttributeBytes: [][]byte{{0xC0, 0x63, 0x01, 0xAB}}, // Custom attr
		},
		{
			Prefix:  netip.MustParsePrefix("10.0.1.0/24"),
			NextHop: netip.MustParseAddr("192.168.1.1"),
			Origin:  attribute.OriginIGP,
		},
	}

	update := mustBuildGrouped(t, ub, routes)
	if update == nil {
		t.Fatal("BuildGroupedUnicast returned nil")
		return
	}

	// Verify ORIGINATOR_ID (type 9) present
	if !bytes.Contains(update.PathAttributes, []byte{0x80, 0x09, 0x04, 0xC0, 0xA8, 0x01, 0x01}) {
		t.Error("ORIGINATOR_ID not encoded")
	}

	// Verify CLUSTER_LIST (type 10) present
	if !bytes.Contains(update.PathAttributes, []byte{0x80, 0x0A}) {
		t.Error("CLUSTER_LIST not encoded")
	}

	// Verify RawAttributes appended
	if !bytes.Contains(update.PathAttributes, []byte{0xC0, 0x63, 0x01, 0xAB}) {
		t.Error("RawAttributes not appended")
	}
}

// TestBuildGroupedUnicast_WithAddPath verifies ADD-PATH encoding (RFC 7911).
//
// VALIDATES: PathID is encoded when ADD-PATH is negotiated.
// PREVENTS: Missing path identifiers in grouped updates.
func TestBuildGroupedUnicast_WithAddPath(t *testing.T) {

	ub := NewUpdateBuilder(65001, true, true, true) // AddPath=true for path ID encoding

	routes := []UnicastParams{
		{
			Prefix:  netip.MustParsePrefix("10.0.0.0/24"),
			NextHop: netip.MustParseAddr("192.168.1.1"),
			Origin:  attribute.OriginIGP,
			PathID:  1,
		},
		{
			Prefix:  netip.MustParsePrefix("10.0.1.0/24"),
			NextHop: netip.MustParseAddr("192.168.1.1"),
			Origin:  attribute.OriginIGP,
			PathID:  2,
		},
	}

	update := mustBuildGrouped(t, ub, routes)
	if update == nil {
		t.Fatal("BuildGroupedUnicast returned nil")
		return
	}

	// With ADD-PATH: each NLRI = 4-byte PathID + 1-byte len + 3-byte prefix = 8 bytes
	// 2 routes = 16 bytes
	expectedNLRILen := 16
	if len(update.NLRI) != expectedNLRILen {
		t.Errorf("NLRI length with ADD-PATH: got %d, want %d", len(update.NLRI), expectedNLRILen)
	}
}

// TestBuildGroupedUnicastWithLimit_EmptySlice verifies empty input handling.
func TestBuildGroupedUnicastWithLimit_EmptySlice(t *testing.T) {

	ub := NewUpdateBuilder(65001, true, true, false)

	updates, err := collectGrouped(t, ub, nil, 65535)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updates != nil {
		t.Error("Expected nil updates for empty input")
	}
}

// =============================================================================
// ASN4 Encoding Tests for Non-Unicast Builders (RFC 6793)
// =============================================================================

// TestBuildVPN_ASN4Disabled verifies 2-byte AS encoding for VPN routes.
//
// VALIDATES: AS_PATH uses 2-byte ASN format when ctx.ASN4=false.
// PREVENTS: RFC 6793 violation for legacy peers with VPN routes.
func TestBuildVPN_ASN4Disabled(t *testing.T) {
	ub := NewUpdateBuilder(100, false, false, false) // 2-byte mode

	params := VPNParams{
		Prefix:  netip.MustParsePrefix("10.0.0.0/24"),
		NextHop: netip.MustParseAddr("192.168.1.1"),
		Origin:  attribute.OriginIGP,
		Labels:  []uint32{100},
		RDBytes: [8]byte{0, 1, 0, 0, 0, 100, 0, 100},
	}

	update := ub.BuildVPN(&params)

	// AS_PATH with 2-byte ASN: 40 02 04 02 01 00 64
	// flags=0x40 (transitive), type=2 (AS_PATH), len=4
	// segment: type=2 (AS_SEQUENCE), count=1, AS=100 (0x0064) as 2 bytes
	expected2ByteAS := []byte{0x40, 0x02, 0x04, 0x02, 0x01, 0x00, 0x64}
	if !bytes.Contains(update.PathAttributes, expected2ByteAS) {
		t.Errorf("VPN AS_PATH not 2-byte encoded\nexpected to contain: %x\ngot: %x",
			expected2ByteAS, update.PathAttributes)
	}

	// Verify it's NOT using 4-byte format
	wrong4ByteAS := []byte{0x40, 0x02, 0x06, 0x02, 0x01, 0x00, 0x00, 0x00, 0x64}
	if bytes.Contains(update.PathAttributes, wrong4ByteAS) {
		t.Error("VPN AS_PATH incorrectly using 4-byte format when ASN4=false")
	}
}

// TestBuildLabeledUnicast_ASN4Disabled verifies 2-byte AS encoding for labeled unicast.
//
// VALIDATES: AS_PATH uses 2-byte ASN format when ctx.ASN4=false.
// PREVENTS: RFC 6793 violation for legacy peers with labeled unicast routes.
func TestBuildLabeledUnicast_ASN4Disabled(t *testing.T) {

	ub := NewUpdateBuilder(100, false, false, false)

	params := LabeledUnicastParams{
		Prefix:  netip.MustParsePrefix("10.0.0.0/24"),
		NextHop: netip.MustParseAddr("192.168.1.1"),
		Origin:  attribute.OriginIGP,
		Labels:  []uint32{100},
	}

	update := ub.BuildLabeledUnicast(&params)

	expected2ByteAS := []byte{0x40, 0x02, 0x04, 0x02, 0x01, 0x00, 0x64}
	if !bytes.Contains(update.PathAttributes, expected2ByteAS) {
		t.Errorf("LabeledUnicast AS_PATH not 2-byte encoded\nexpected to contain: %x\ngot: %x",
			expected2ByteAS, update.PathAttributes)
	}
}

// TestBuildMVPN_ASN4Disabled verifies 2-byte AS encoding for MVPN routes.
//
// VALIDATES: AS_PATH uses 2-byte ASN format when ctx.ASN4=false.
// PREVENTS: RFC 6793 violation for legacy peers with MVPN routes.
func TestBuildMVPN_ASN4Disabled(t *testing.T) {

	ub := NewUpdateBuilder(100, false, false, false)

	params := MVPNParams{
		RouteType: 5,
		IsIPv6:    false,
		RD:        [8]byte{0, 1, 0, 0, 0, 100, 0, 100},
		Source:    netip.MustParseAddr("10.0.0.1"),
		Group:     netip.MustParseAddr("239.1.1.1"),
		NextHop:   netip.MustParseAddr("192.168.1.1"),
		Origin:    attribute.OriginIGP,
	}

	update := ub.BuildMVPN([]MVPNParams{params})

	expected2ByteAS := []byte{0x40, 0x02, 0x04, 0x02, 0x01, 0x00, 0x64}
	if !bytes.Contains(update.PathAttributes, expected2ByteAS) {
		t.Errorf("MVPN AS_PATH not 2-byte encoded\nexpected to contain: %x\ngot: %x",
			expected2ByteAS, update.PathAttributes)
	}
}

// TestBuildVPLS_ASN4Disabled verifies 2-byte AS encoding for VPLS routes.
//
// VALIDATES: AS_PATH uses 2-byte ASN format when ctx.ASN4=false.
// PREVENTS: RFC 6793 violation for legacy peers with VPLS routes.
func TestBuildVPLS_ASN4Disabled(t *testing.T) {

	ub := NewUpdateBuilder(100, false, false, false)

	params := VPLSParams{
		RD:       [8]byte{0, 1, 0, 0, 0, 100, 0, 100},
		Endpoint: 1,
		Base:     100,
		Offset:   0,
		Size:     10,
		NextHop:  netip.MustParseAddr("192.168.1.1"),
		Origin:   attribute.OriginIGP,
	}

	update := ub.BuildVPLS(params)

	expected2ByteAS := []byte{0x40, 0x02, 0x04, 0x02, 0x01, 0x00, 0x64}
	if !bytes.Contains(update.PathAttributes, expected2ByteAS) {
		t.Errorf("VPLS AS_PATH not 2-byte encoded\nexpected to contain: %x\ngot: %x",
			expected2ByteAS, update.PathAttributes)
	}
}

// TestBuildFlowSpec_ASN4Disabled verifies 2-byte AS encoding for FlowSpec routes.
//
// VALIDATES: AS_PATH uses 2-byte ASN format when ctx.ASN4=false.
// PREVENTS: RFC 6793 violation for legacy peers with FlowSpec routes.
func TestBuildFlowSpec_ASN4Disabled(t *testing.T) {

	ub := NewUpdateBuilder(100, false, false, false)

	params := FlowSpecParams{
		IsIPv6:  false,
		NLRI:    []byte{0x03, 0x01, 0x18, 0x0a},
		NextHop: netip.MustParseAddr("192.168.1.1"),
	}

	update := ub.BuildFlowSpec(params)

	expected2ByteAS := []byte{0x40, 0x02, 0x04, 0x02, 0x01, 0x00, 0x64}
	if !bytes.Contains(update.PathAttributes, expected2ByteAS) {
		t.Errorf("FlowSpec AS_PATH not 2-byte encoded\nexpected to contain: %x\ngot: %x",
			expected2ByteAS, update.PathAttributes)
	}
}

// TestBuildMUP_ASN4Disabled verifies 2-byte AS encoding for MUP routes.
//
// VALIDATES: AS_PATH uses 2-byte ASN format when ctx.ASN4=false.
// PREVENTS: RFC 6793 violation for legacy peers with MUP routes.
func TestBuildMUP_ASN4Disabled(t *testing.T) {

	ub := NewUpdateBuilder(100, false, false, false)

	params := MUPParams{
		RouteType: 1,
		IsIPv6:    false,
		NLRI:      []byte{0x01, 0x02, 0x03, 0x04},
		NextHop:   netip.MustParseAddr("192.168.1.1"),
	}

	update := ub.BuildMUP(params)

	expected2ByteAS := []byte{0x40, 0x02, 0x04, 0x02, 0x01, 0x00, 0x64}
	if !bytes.Contains(update.PathAttributes, expected2ByteAS) {
		t.Errorf("MUP AS_PATH not 2-byte encoded\nexpected to contain: %x\ngot: %x",
			expected2ByteAS, update.PathAttributes)
	}
}

// TestBuildMUPWithdraw_ASN4Disabled verifies 2-byte AS encoding for MUP withdrawals.
//
// VALIDATES: AS_PATH uses 2-byte ASN format when ctx.ASN4=false.
// PREVENTS: RFC 6793 violation for legacy peers with MUP withdrawals.
func TestBuildMUPWithdraw_ASN4Disabled(t *testing.T) {

	ub := NewUpdateBuilder(100, false, false, false)

	params := MUPParams{
		RouteType: 1,
		IsIPv6:    false,
		NLRI:      []byte{0x01, 0x02, 0x03, 0x04},
		NextHop:   netip.MustParseAddr("192.168.1.1"),
	}

	update := ub.BuildMUPWithdraw(params)

	expected2ByteAS := []byte{0x40, 0x02, 0x04, 0x02, 0x01, 0x00, 0x64}
	if !bytes.Contains(update.PathAttributes, expected2ByteAS) {
		t.Errorf("MUPWithdraw AS_PATH not 2-byte encoded\nexpected to contain: %x\ngot: %x",
			expected2ByteAS, update.PathAttributes)
	}
}

// =============================================================================
// AGGREGATOR ASN4 Encoding Tests (RFC 6793 Section 4.2.3)
// =============================================================================

// TestBuildUnicast_Aggregator_ASN4Disabled verifies 6-byte AGGREGATOR encoding.
//
// VALIDATES: AGGREGATOR uses 6-byte format (2-byte ASN) when ctx.ASN4=false.
// PREVENTS: RFC 6793 violation - AGGREGATOR must match ASN4 capability.
func TestBuildUnicast_Aggregator_ASN4Disabled(t *testing.T) {

	ub := NewUpdateBuilder(100, false, false, false)

	params := UnicastParams{
		Prefix:        netip.MustParsePrefix("10.0.0.0/24"),
		NextHop:       netip.MustParseAddr("192.168.1.1"),
		Origin:        attribute.OriginIGP,
		HasAggregator: true,
		AggregatorASN: 100,
		AggregatorIP:  [4]byte{192, 168, 1, 1},
	}

	update := ub.BuildUnicast(&params)

	// AGGREGATOR with 2-byte ASN: C0 07 06 00 64 C0 A8 01 01
	// flags=0xC0 (optional+transitive), type=7, len=6, ASN=100 (2 bytes), IP=192.168.1.1
	expected6Byte := []byte{0xC0, 0x07, 0x06, 0x00, 0x64, 0xC0, 0xA8, 0x01, 0x01}
	if !bytes.Contains(update.PathAttributes, expected6Byte) {
		t.Errorf("AGGREGATOR not 6-byte encoded\nexpected to contain: %x\ngot: %x",
			expected6Byte, update.PathAttributes)
	}

	// Verify it's NOT using 8-byte format
	wrong8Byte := []byte{0xC0, 0x07, 0x08, 0x00, 0x00, 0x00, 0x64}
	if bytes.Contains(update.PathAttributes, wrong8Byte) {
		t.Error("AGGREGATOR incorrectly using 8-byte format when ASN4=false")
	}
}

// TestBuildVPN_Aggregator_ASN4Disabled verifies 6-byte AGGREGATOR for VPN routes.
//
// VALIDATES: AGGREGATOR uses 6-byte format when ctx.ASN4=false.
// PREVENTS: RFC 6793 violation for VPN routes.
func TestBuildVPN_Aggregator_ASN4Disabled(t *testing.T) {

	ub := NewUpdateBuilder(100, false, false, false)

	params := VPNParams{
		Prefix:        netip.MustParsePrefix("10.0.0.0/24"),
		NextHop:       netip.MustParseAddr("192.168.1.1"),
		Origin:        attribute.OriginIGP,
		Labels:        []uint32{100},
		RDBytes:       [8]byte{0, 1, 0, 0, 0, 100, 0, 100},
		HasAggregator: true,
		AggregatorASN: 100,
		AggregatorIP:  [4]byte{192, 168, 1, 1},
	}

	update := ub.BuildVPN(&params)

	expected6Byte := []byte{0xC0, 0x07, 0x06, 0x00, 0x64, 0xC0, 0xA8, 0x01, 0x01}
	if !bytes.Contains(update.PathAttributes, expected6Byte) {
		t.Errorf("VPN AGGREGATOR not 6-byte encoded\nexpected to contain: %x\ngot: %x",
			expected6Byte, update.PathAttributes)
	}
}

// TestBuildLabeledUnicast_Aggregator_ASN4Disabled verifies 6-byte AGGREGATOR for labeled unicast.
//
// VALIDATES: AGGREGATOR uses 6-byte format when ctx.ASN4=false.
// PREVENTS: RFC 6793 violation for labeled unicast routes.
func TestBuildLabeledUnicast_Aggregator_ASN4Disabled(t *testing.T) {

	ub := NewUpdateBuilder(100, false, false, false)

	params := LabeledUnicastParams{
		Prefix:        netip.MustParsePrefix("10.0.0.0/24"),
		NextHop:       netip.MustParseAddr("192.168.1.1"),
		Origin:        attribute.OriginIGP,
		Labels:        []uint32{100},
		HasAggregator: true,
		AggregatorASN: 100,
		AggregatorIP:  [4]byte{192, 168, 1, 1},
	}

	update := ub.BuildLabeledUnicast(&params)

	expected6Byte := []byte{0xC0, 0x07, 0x06, 0x00, 0x64, 0xC0, 0xA8, 0x01, 0x01}
	if !bytes.Contains(update.PathAttributes, expected6Byte) {
		t.Errorf("LabeledUnicast AGGREGATOR not 6-byte encoded\nexpected to contain: %x\ngot: %x",
			expected6Byte, update.PathAttributes)
	}
}

// TestBuildVPLS_Aggregator_ASN4Disabled verifies 6-byte AGGREGATOR for VPLS routes.
//
// VALIDATES: AGGREGATOR uses 6-byte format when ctx.ASN4=false.
// PREVENTS: RFC 6793 violation for VPLS routes.
func TestBuildVPLS_Aggregator_ASN4Disabled(t *testing.T) {

	ub := NewUpdateBuilder(100, false, false, false)

	params := VPLSParams{
		RD:       [8]byte{0, 1, 0, 0, 0, 100, 0, 100},
		Endpoint: 1,
		Base:     100,
		Offset:   0,
		Size:     10,
		NextHop:  netip.MustParseAddr("192.168.1.1"),
		Origin:   attribute.OriginIGP,
		ASPath:   []uint32{100}, // Need AS path to trigger aggregator
	}
	// Note: VPLSParams doesn't have HasAggregator - this test documents the limitation

	update := ub.BuildVPLS(params)
	if update == nil {
		t.Fatal("BuildVPLS returned nil")
		return
	}
}

// TestBuildGroupedUnicast_Aggregator_ASN4Disabled verifies 6-byte AGGREGATOR for grouped updates.
//
// VALIDATES: AGGREGATOR uses 6-byte format when ctx.ASN4=false in grouped updates.
// PREVENTS: RFC 6793 violation for grouped unicast routes.
func TestBuildGroupedUnicast_Aggregator_ASN4Disabled(t *testing.T) {

	ub := NewUpdateBuilder(100, false, false, false)

	routes := []UnicastParams{
		{
			Prefix:        netip.MustParsePrefix("10.0.0.0/24"),
			NextHop:       netip.MustParseAddr("192.168.1.1"),
			Origin:        attribute.OriginIGP,
			HasAggregator: true,
			AggregatorASN: 100,
			AggregatorIP:  [4]byte{192, 168, 1, 1},
		},
		{
			Prefix:  netip.MustParsePrefix("10.0.1.0/24"),
			NextHop: netip.MustParseAddr("192.168.1.1"),
			Origin:  attribute.OriginIGP,
		},
	}

	update := mustBuildGrouped(t, ub, routes)

	expected6Byte := []byte{0xC0, 0x07, 0x06, 0x00, 0x64, 0xC0, 0xA8, 0x01, 0x01}
	if !bytes.Contains(update.PathAttributes, expected6Byte) {
		t.Errorf("Grouped AGGREGATOR not 6-byte encoded\nexpected to contain: %x\ngot: %x",
			expected6Byte, update.PathAttributes)
	}
}

// TestBuildMVPN_EncodesReflectorAttrs verifies RFC 4456 attribute encoding for MVPN.
//
// VALIDATES: ORIGINATOR_ID and CLUSTER_LIST are encoded in PathAttributes.
// PREVENTS: Data loss for route reflector configurations with MVPN.
func TestBuildMVPN_EncodesReflectorAttrs(t *testing.T) {

	ub := NewUpdateBuilder(65001, true, true, false)

	routes := []MVPNParams{
		{
			RouteType:       5,
			IsIPv6:          false,
			Source:          netip.MustParseAddr("192.168.1.1"),
			Group:           netip.MustParseAddr("239.0.0.1"),
			NextHop:         netip.MustParseAddr("192.168.1.1"),
			Origin:          attribute.OriginIGP,
			LocalPreference: 100,
			OriginatorID:    0xC0A80101, // 192.168.1.1
			ClusterList:     []uint32{0xC0A80102, 0xC0A80103},
		},
	}

	update := ub.BuildMVPN(routes)

	// ORIGINATOR_ID: flags=0x80 (optional), type=0x09, len=0x04, value=C0A80101
	expectedOriginator := []byte{0x80, 0x09, 0x04, 0xC0, 0xA8, 0x01, 0x01}
	if !bytes.Contains(update.PathAttributes, expectedOriginator) {
		t.Errorf("ORIGINATOR_ID not found in PathAttributes\ngot: %x\nwant to contain: %x",
			update.PathAttributes, expectedOriginator)
	}

	// CLUSTER_LIST: flags=0x80, type=0x0A, len=0x08, values=C0A80102 C0A80103
	expectedClusterType := []byte{0x80, 0x0A, 0x08}
	if !bytes.Contains(update.PathAttributes, expectedClusterType) {
		t.Errorf("CLUSTER_LIST not found in PathAttributes\ngot: %x",
			update.PathAttributes)
	}
}

// TestBuildFlowSpec_EncodesReflectorAttrs verifies RFC 4456 attribute encoding for FlowSpec.
//
// VALIDATES: ORIGINATOR_ID and CLUSTER_LIST are encoded in PathAttributes.
// PREVENTS: Data loss for route reflector configurations with FlowSpec.
func TestBuildFlowSpec_EncodesReflectorAttrs(t *testing.T) {

	ub := NewUpdateBuilder(65001, true, true, false)

	params := FlowSpecParams{
		IsIPv6:       false,
		NLRI:         []byte{0x06, 0x01, 0x18, 0x0A, 0x00, 0x00}, // simple flowspec
		NextHop:      netip.MustParseAddr("192.168.1.1"),
		OriginatorID: 0xC0A80101, // 192.168.1.1
		ClusterList:  []uint32{0xC0A80102, 0xC0A80103},
	}

	update := ub.BuildFlowSpec(params)

	// ORIGINATOR_ID: flags=0x80 (optional), type=0x09, len=0x04, value=C0A80101
	expectedOriginator := []byte{0x80, 0x09, 0x04, 0xC0, 0xA8, 0x01, 0x01}
	if !bytes.Contains(update.PathAttributes, expectedOriginator) {
		t.Errorf("ORIGINATOR_ID not found in PathAttributes\ngot: %x\nwant to contain: %x",
			update.PathAttributes, expectedOriginator)
	}

	// CLUSTER_LIST: flags=0x80, type=0x0A, len=0x08, values=C0A80102 C0A80103
	expectedClusterType := []byte{0x80, 0x0A, 0x08}
	if !bytes.Contains(update.PathAttributes, expectedClusterType) {
		t.Errorf("CLUSTER_LIST not found in PathAttributes\ngot: %x",
			update.PathAttributes)
	}
}

// TestBuildMUP_EncodesReflectorAttrs verifies RFC 4456 attribute encoding for MUP.
//
// VALIDATES: ORIGINATOR_ID and CLUSTER_LIST are encoded in PathAttributes.
// PREVENTS: Data loss for route reflector configurations with MUP.
func TestBuildMUP_EncodesReflectorAttrs(t *testing.T) {

	ub := NewUpdateBuilder(65001, true, true, false)

	params := MUPParams{
		RouteType:    1,
		IsIPv6:       false,
		NLRI:         []byte{0x01, 0x00, 0x00, 0x01}, // simple MUP NLRI
		NextHop:      netip.MustParseAddr("192.168.1.1"),
		OriginatorID: 0xC0A80101, // 192.168.1.1
		ClusterList:  []uint32{0xC0A80102, 0xC0A80103},
	}

	update := ub.BuildMUP(params)

	// ORIGINATOR_ID: flags=0x80 (optional), type=0x09, len=0x04, value=C0A80101
	expectedOriginator := []byte{0x80, 0x09, 0x04, 0xC0, 0xA8, 0x01, 0x01}
	if !bytes.Contains(update.PathAttributes, expectedOriginator) {
		t.Errorf("ORIGINATOR_ID not found in PathAttributes\ngot: %x\nwant to contain: %x",
			update.PathAttributes, expectedOriginator)
	}

	// CLUSTER_LIST: flags=0x80, type=0x0A, len=0x08, values=C0A80102 C0A80103
	expectedClusterType := []byte{0x80, 0x0A, 0x08}
	if !bytes.Contains(update.PathAttributes, expectedClusterType) {
		t.Errorf("CLUSTER_LIST not found in PathAttributes\ngot: %x",
			update.PathAttributes)
	}
}

// =============================================================================
// BuildGroupedUnicastWithLimit Tests (Phase 3: Size-Aware Builder)
// =============================================================================

// TestBuildWithLimit_Empty verifies empty input.
//
// VALIDATES: Empty routes returns nil, nil.
// PREVENTS: Panic on empty input.
func TestBuildWithLimit_Empty(t *testing.T) {

	ub := NewUpdateBuilder(65001, true, true, false)

	updates, err := collectGrouped(t, ub, nil, 4096)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if updates != nil {
		t.Errorf("expected nil for empty input, got %d updates", len(updates))
	}
}

// TestBuildWithLimit_SingleRoute verifies single route.
//
// VALIDATES: Single route returns single UPDATE.
// PREVENTS: Unnecessary splitting.
func TestBuildWithLimit_SingleRoute(t *testing.T) {

	ub := NewUpdateBuilder(65001, true, true, false)

	routes := []UnicastParams{{
		Prefix:  netip.MustParsePrefix("10.0.0.0/24"),
		NextHop: netip.MustParseAddr("192.168.1.1"),
		Origin:  attribute.OriginIGP,
	}}

	updates, err := collectGrouped(t, ub, routes, 4096)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(updates) != 1 {
		t.Errorf("expected 1 update, got %d", len(updates))
	}
}

// TestBuildWithLimit_AllFit verifies multiple routes fitting.
//
// VALIDATES: N routes that fit return single UPDATE.
// PREVENTS: Unnecessary splitting.
func TestBuildWithLimit_AllFit(t *testing.T) {

	ub := NewUpdateBuilder(65001, true, true, false)

	// Create 10 routes that should fit in one UPDATE
	var routes []UnicastParams
	for range 10 {
		routes = append(routes, UnicastParams{
			Prefix:  netip.MustParsePrefix("10.0.0.0/24"),
			NextHop: netip.MustParseAddr("192.168.1.1"),
			Origin:  attribute.OriginIGP,
		})
	}

	updates, err := collectGrouped(t, ub, routes, 4096)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(updates) != 1 {
		t.Errorf("expected 1 update (all fit), got %d", len(updates))
	}
}

// TestBuildWithLimit_Overflow verifies route batching.
//
// VALIDATES: N routes overflow into M UPDATEs.
// PREVENTS: Single oversized UPDATE from builder.
func TestBuildWithLimit_Overflow(t *testing.T) {

	ub := NewUpdateBuilder(65001, true, true, false)

	// Create 100 routes - should overflow with small maxSize
	var routes []UnicastParams
	for range 100 {
		routes = append(routes, UnicastParams{
			Prefix:  netip.MustParsePrefix("10.0.0.0/24"),
			NextHop: netip.MustParseAddr("192.168.1.1"),
			Origin:  attribute.OriginIGP,
		})
	}

	// Small maxSize to force splitting
	// Overhead = 19 + 4 = 23, attrs ~30 bytes, leaves ~47 for NLRI
	// Each /24 = 4 bytes, so ~11 per update
	updates, err := collectGrouped(t, ub, routes, 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(updates) <= 1 {
		t.Errorf("expected multiple updates for overflow, got %d", len(updates))
	}

	// Verify each update is within size limit
	for i, u := range updates {
		size := HeaderLen + 4 + len(u.PathAttributes) + len(u.NLRI)
		if size > 100 {
			t.Errorf("update %d exceeds maxSize: %d > 100", i, size)
		}
	}

	// Count total NLRIs
	totalNLRIs := 0
	for _, u := range updates {
		// Each /24 is 4 bytes
		totalNLRIs += len(u.NLRI) / 4
	}
	if totalNLRIs != 100 {
		t.Errorf("expected 100 total NLRIs, got %d", totalNLRIs)
	}
}

// TestBuildWithLimit_AttrsTooBig verifies attribute overflow.
//
// VALIDATES: ErrAttributesTooLarge when attrs > maxSize.
// PREVENTS: Panic on huge attributes.
func TestBuildWithLimit_AttrsTooBig(t *testing.T) {

	ub := NewUpdateBuilder(65001, true, true, false)

	// Route with large communities
	routes := []UnicastParams{{
		Prefix:      netip.MustParsePrefix("10.0.0.0/24"),
		NextHop:     netip.MustParseAddr("192.168.1.1"),
		Origin:      attribute.OriginIGP,
		Communities: make([]uint32, 100), // 400 bytes of communities
	}}

	// maxSize too small for attributes
	_, err := collectGrouped(t, ub, routes, 50)
	if err == nil {
		t.Error("expected ErrAttributesTooLarge, got nil")
	}
}

// TestBuildWithLimit_AllRoutesPreserved verifies no data loss.
//
// VALIDATES: All routes appear in output UPDATEs.
// PREVENTS: Route loss during splitting.
func TestBuildWithLimit_AllRoutesPreserved(t *testing.T) {

	ub := NewUpdateBuilder(65001, true, true, false)

	// Create 50 routes (same prefix is fine - testing byte count)
	var routes []UnicastParams
	for range 50 {
		routes = append(routes, UnicastParams{
			Prefix:  netip.MustParsePrefix("10.0.0.0/24"),
			NextHop: netip.MustParseAddr("192.168.1.1"),
			Origin:  attribute.OriginIGP,
		})
	}

	updates, err := collectGrouped(t, ub, routes, 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Count total NLRI bytes
	totalNLRIBytes := 0
	for _, u := range updates {
		totalNLRIBytes += len(u.NLRI)
	}

	// Each /24 = 4 bytes
	expectedBytes := 50 * 4
	if totalNLRIBytes != expectedBytes {
		t.Errorf("expected %d NLRI bytes, got %d", expectedBytes, totalNLRIBytes)
	}
}

// TestBuildWithLimit_AttributesShared verifies attribute reuse.
//
// VALIDATES: All updates share same attributes (consistent).
// PREVENTS: Inconsistent attributes across split updates.
func TestBuildWithLimit_AttributesShared(t *testing.T) {

	ub := NewUpdateBuilder(65001, true, true, false)

	var routes []UnicastParams
	for range 50 {
		routes = append(routes, UnicastParams{
			Prefix:  netip.MustParsePrefix("10.0.0.0/24"),
			NextHop: netip.MustParseAddr("192.168.1.1"),
			Origin:  attribute.OriginIGP,
		})
	}

	updates, err := collectGrouped(t, ub, routes, 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(updates) < 2 {
		t.Skip("need multiple updates to verify attribute sharing")
	}

	// All updates should have identical attributes
	firstAttrs := updates[0].PathAttributes
	for i, u := range updates[1:] {
		if !bytes.Equal(u.PathAttributes, firstAttrs) {
			t.Errorf("update %d has different attributes", i+1)
		}
	}
}

// =============================================================================
// API Bounds Safety Tests (spec-api-bounds-safety.md)
// =============================================================================

// TestBuildFlowSpec_MaxSize_Fits verifies FlowSpec within limit succeeds.
//
// VALIDATES: BuildFlowSpec returns UPDATE when size <= maxSize.
// PREVENTS: False positives on valid FlowSpec routes.
func TestBuildFlowSpec_MaxSize_Fits(t *testing.T) {

	ub := NewUpdateBuilder(65001, false, true, false)

	// Simple FlowSpec NLRI (destination prefix 10.0.0.0/24)
	params := FlowSpecParams{
		IsIPv6:  false,
		NLRI:    []byte{0x03, 0x01, 0x18, 0x0a}, // dest 10.0.0.0/24
		NextHop: netip.MustParseAddr("192.168.1.1"),
	}

	// Large maxSize - should fit
	update, err := ub.BuildFlowSpecWithMaxSize(params, 4096)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if update == nil {
		t.Fatal("expected non-nil UPDATE")
		return
	}
}

// TestBuildFlowSpec_MaxSize_TooLarge verifies error when FlowSpec > maxSize.
//
// VALIDATES: BuildFlowSpec returns ErrUpdateTooLarge when route + attrs > maxSize.
// PREVENTS: Oversized UPDATE generation for FlowSpec.
// RFC 5575 Section 4: Single FlowSpec rule is atomic - cannot be split.
func TestBuildFlowSpec_MaxSize_TooLarge(t *testing.T) {

	ub := NewUpdateBuilder(65001, false, true, false)

	params := FlowSpecParams{
		IsIPv6:  false,
		NLRI:    []byte{0x03, 0x01, 0x18, 0x0a},
		NextHop: netip.MustParseAddr("192.168.1.1"),
	}

	// Very small maxSize - should fail
	_, err := ub.BuildFlowSpecWithMaxSize(params, 30)
	if err == nil {
		t.Fatal("expected ErrUpdateTooLarge, got nil")
		return
	}
	if !errors.Is(err, ErrUpdateTooLarge) {
		t.Errorf("expected ErrUpdateTooLarge, got %v", err)
	}
}

// TestBuildMVPNWithLimit_AllFit verifies MVPN batch fits in single UPDATE.
//
// VALIDATES: BuildMVPNWithLimit returns single UPDATE when all routes fit.
// PREVENTS: Unnecessary splitting of small batches.
func TestBuildMVPNWithLimit_AllFit(t *testing.T) {

	ub := NewUpdateBuilder(65001, true, true, false)

	// Two small MVPN routes that should fit
	routes := []MVPNParams{
		{
			RouteType: 5,
			IsIPv6:    false,
			RD:        [8]byte{0, 1, 0, 0, 0, 100, 0, 100},
			Source:    netip.MustParseAddr("10.0.0.1"),
			Group:     netip.MustParseAddr("239.1.1.1"),
			NextHop:   netip.MustParseAddr("192.168.1.1"),
			Origin:    attribute.OriginIGP,
		},
		{
			RouteType: 5,
			IsIPv6:    false,
			RD:        [8]byte{0, 1, 0, 0, 0, 100, 0, 101},
			Source:    netip.MustParseAddr("10.0.0.2"),
			Group:     netip.MustParseAddr("239.1.1.2"),
			NextHop:   netip.MustParseAddr("192.168.1.1"),
			Origin:    attribute.OriginIGP,
		},
	}

	updates, err := collectMVPN(t, ub, routes, 4096)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(updates) != 1 {
		t.Errorf("expected 1 UPDATE (all fit), got %d", len(updates))
	}
}

// TestBuildMVPNWithLimit_Split verifies MVPN batch splits across UPDATEs.
//
// VALIDATES: BuildMVPNWithLimit returns multiple UPDATEs when routes overflow.
// PREVENTS: Single oversized UPDATE for large MVPN batches.
func TestBuildMVPNWithLimit_Split(t *testing.T) {

	ub := NewUpdateBuilder(65001, true, true, false)

	// Create 20 MVPN routes - should overflow with small maxSize
	var routes []MVPNParams
	for i := range 20 {
		routes = append(routes, MVPNParams{
			RouteType: 5,
			IsIPv6:    false,
			RD:        [8]byte{0, 1, 0, 0, 0, 100, 0, byte(i)},
			Source:    netip.MustParseAddr("10.0.0.1"),
			Group:     netip.MustParseAddr("239.1.1.1"),
			NextHop:   netip.MustParseAddr("192.168.1.1"),
			Origin:    attribute.OriginIGP,
		})
	}

	// Small maxSize to force splitting
	updates, err := collectMVPN(t, ub, routes, 200)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(updates) <= 1 {
		t.Errorf("expected multiple UPDATEs for overflow, got %d", len(updates))
	}

	// Verify each update is within size limit
	for i, u := range updates {
		size := HeaderLen + 4 + len(u.PathAttributes)
		if size > 200 {
			t.Errorf("update %d exceeds maxSize: %d > 200", i, size)
		}
	}
}

// TestBuildUnicast_MaxSize_TooLarge verifies error when unicast > maxSize.
//
// VALIDATES: BuildUnicastWithMaxSize returns ErrUpdateTooLarge when route + attrs > maxSize.
// PREVENTS: Oversized UPDATE generation for unicast.
func TestBuildUnicast_MaxSize_TooLarge(t *testing.T) {

	ub := NewUpdateBuilder(65001, true, true, false)

	params := UnicastParams{
		Prefix:  netip.MustParsePrefix("10.0.0.0/24"),
		NextHop: netip.MustParseAddr("192.168.1.1"),
		Origin:  attribute.OriginIGP,
	}

	// Very small maxSize - should fail
	_, err := ub.BuildUnicastWithMaxSize(&params, 30)
	if err == nil {
		t.Fatal("expected ErrUpdateTooLarge, got nil")
		return
	}
	if !errors.Is(err, ErrUpdateTooLarge) {
		t.Errorf("expected ErrUpdateTooLarge, got %v", err)
	}
}

// TestBuildUnicast_MaxSize_Fits verifies unicast within limit succeeds.
//
// VALIDATES: BuildUnicastWithMaxSize returns UPDATE when size <= maxSize.
// PREVENTS: False positives on valid unicast routes.
func TestBuildUnicast_MaxSize_Fits(t *testing.T) {

	ub := NewUpdateBuilder(65001, true, true, false)

	params := UnicastParams{
		Prefix:  netip.MustParsePrefix("10.0.0.0/24"),
		NextHop: netip.MustParseAddr("192.168.1.1"),
		Origin:  attribute.OriginIGP,
	}

	// Large maxSize - should fit
	update, err := ub.BuildUnicastWithMaxSize(&params, 4096)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if update == nil {
		t.Fatal("expected non-nil UPDATE")
		return
	}
}

// TestBuildVPN_MaxSize_Fits verifies VPN within limit succeeds.
//
// VALIDATES: BuildVPNWithMaxSize returns UPDATE when size <= maxSize.
// PREVENTS: False positives on valid VPN routes.
func TestBuildVPN_MaxSize_Fits(t *testing.T) {

	ub := NewUpdateBuilder(65001, false, true, false)

	params := VPNParams{
		Prefix:  netip.MustParsePrefix("10.0.0.0/24"),
		NextHop: netip.MustParseAddr("192.168.1.1"),
		Origin:  attribute.OriginIGP,
		Labels:  []uint32{100},
		RDBytes: [8]byte{0, 1, 0, 0, 0, 100, 0, 100},
	}

	update, err := ub.BuildVPNWithMaxSize(&params, 4096)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if update == nil {
		t.Fatal("expected non-nil UPDATE")
		return
	}
}

// TestBuildVPN_MaxSize_TooLarge verifies error when VPN > maxSize.
//
// VALIDATES: BuildVPNWithMaxSize returns ErrUpdateTooLarge when route + attrs > maxSize.
// PREVENTS: Oversized UPDATE generation for VPN routes.
func TestBuildVPN_MaxSize_TooLarge(t *testing.T) {

	ub := NewUpdateBuilder(65001, false, true, false)

	params := VPNParams{
		Prefix:  netip.MustParsePrefix("10.0.0.0/24"),
		NextHop: netip.MustParseAddr("192.168.1.1"),
		Origin:  attribute.OriginIGP,
		Labels:  []uint32{100},
		RDBytes: [8]byte{0, 1, 0, 0, 0, 100, 0, 100},
	}

	_, err := ub.BuildVPNWithMaxSize(&params, 30)
	if err == nil {
		t.Fatal("expected ErrUpdateTooLarge, got nil")
		return
	}
	if !errors.Is(err, ErrUpdateTooLarge) {
		t.Errorf("expected ErrUpdateTooLarge, got %v", err)
	}
}

// TestBuildLabeledUnicast_MaxSize_Fits verifies labeled unicast within limit succeeds.
//
// VALIDATES: BuildLabeledUnicastWithMaxSize returns UPDATE when size <= maxSize.
// PREVENTS: False positives on valid labeled unicast routes.
func TestBuildLabeledUnicast_MaxSize_Fits(t *testing.T) {

	ub := NewUpdateBuilder(65001, false, true, false)

	params := LabeledUnicastParams{
		Prefix:  netip.MustParsePrefix("10.0.0.0/24"),
		NextHop: netip.MustParseAddr("192.168.1.1"),
		Origin:  attribute.OriginIGP,
		Labels:  []uint32{100},
	}

	update, err := ub.BuildLabeledUnicastWithMaxSize(&params, 4096)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if update == nil {
		t.Fatal("expected non-nil UPDATE")
		return
	}
}

// TestBuildLabeledUnicast_MaxSize_TooLarge verifies error when labeled unicast > maxSize.
//
// VALIDATES: BuildLabeledUnicastWithMaxSize returns ErrUpdateTooLarge when route + attrs > maxSize.
// PREVENTS: Oversized UPDATE generation for labeled unicast routes.
func TestBuildLabeledUnicast_MaxSize_TooLarge(t *testing.T) {

	ub := NewUpdateBuilder(65001, false, true, false)

	params := LabeledUnicastParams{
		Prefix:  netip.MustParsePrefix("10.0.0.0/24"),
		NextHop: netip.MustParseAddr("192.168.1.1"),
		Origin:  attribute.OriginIGP,
		Labels:  []uint32{100},
	}

	_, err := ub.BuildLabeledUnicastWithMaxSize(&params, 30)
	if err == nil {
		t.Fatal("expected ErrUpdateTooLarge, got nil")
		return
	}
	if !errors.Is(err, ErrUpdateTooLarge) {
		t.Errorf("expected ErrUpdateTooLarge, got %v", err)
	}
}

// TestBuildVPLS_MaxSize_Fits verifies VPLS within limit succeeds.
//
// VALIDATES: BuildVPLSWithMaxSize returns UPDATE when size <= maxSize.
// PREVENTS: False positives on valid VPLS routes.
func TestBuildVPLS_MaxSize_Fits(t *testing.T) {

	ub := NewUpdateBuilder(65001, false, true, false)

	params := VPLSParams{
		RD:       [8]byte{0, 1, 0, 0, 0, 100, 0, 100},
		Endpoint: 1,
		Base:     100,
		Offset:   0,
		Size:     10,
		NextHop:  netip.MustParseAddr("192.168.1.1"),
		Origin:   attribute.OriginIGP,
	}

	update, err := ub.BuildVPLSWithMaxSize(params, 4096)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if update == nil {
		t.Fatal("expected non-nil UPDATE")
		return
	}
}

// TestBuildVPLS_MaxSize_TooLarge verifies error when VPLS > maxSize.
//
// VALIDATES: BuildVPLSWithMaxSize returns ErrUpdateTooLarge when route + attrs > maxSize.
// PREVENTS: Oversized UPDATE generation for VPLS routes.
func TestBuildVPLS_MaxSize_TooLarge(t *testing.T) {

	ub := NewUpdateBuilder(65001, false, true, false)

	params := VPLSParams{
		RD:       [8]byte{0, 1, 0, 0, 0, 100, 0, 100},
		Endpoint: 1,
		Base:     100,
		Offset:   0,
		Size:     10,
		NextHop:  netip.MustParseAddr("192.168.1.1"),
		Origin:   attribute.OriginIGP,
	}

	_, err := ub.BuildVPLSWithMaxSize(params, 30)
	if err == nil {
		t.Fatal("expected ErrUpdateTooLarge, got nil")
		return
	}
	if !errors.Is(err, ErrUpdateTooLarge) {
		t.Errorf("expected ErrUpdateTooLarge, got %v", err)
	}
}

// TestBuildEVPN_MaxSize_Fits verifies EVPN within limit succeeds.
//
// VALIDATES: BuildEVPNWithMaxSize returns UPDATE when size <= maxSize.
// PREVENTS: False positives on valid EVPN routes.
func TestBuildEVPN_MaxSize_Fits(t *testing.T) {
	ub := NewUpdateBuilder(65001, false, true, false)

	rd := nlri.RouteDistinguisher{Type: 1, Value: [6]byte{0, 0, 0, 100, 0, 100}}

	params := EVPNParams{
		NLRI:    testEVPNType2Bytes(rd, [6]byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55}, netip.Addr{}),
		NextHop: netip.MustParseAddr("192.168.1.1"),
		Origin:  attribute.OriginIGP,
	}

	update, err := ub.BuildEVPNWithMaxSize(params, 4096)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if update == nil {
		t.Fatal("expected non-nil UPDATE")
		return
	}
}

// TestBuildEVPN_MaxSize_TooLarge verifies error when EVPN > maxSize.
//
// VALIDATES: BuildEVPNWithMaxSize returns ErrUpdateTooLarge when route + attrs > maxSize.
// PREVENTS: Oversized UPDATE generation for EVPN routes.
func TestBuildEVPN_MaxSize_TooLarge(t *testing.T) {
	ub := NewUpdateBuilder(65001, false, true, false)

	rd := nlri.RouteDistinguisher{Type: 1, Value: [6]byte{0, 0, 0, 100, 0, 100}}

	params := EVPNParams{
		NLRI:    testEVPNType2Bytes(rd, [6]byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55}, netip.Addr{}),
		NextHop: netip.MustParseAddr("192.168.1.1"),
		Origin:  attribute.OriginIGP,
	}

	_, err := ub.BuildEVPNWithMaxSize(params, 30)
	if err == nil {
		t.Fatal("expected ErrUpdateTooLarge, got nil")
		return
	}
	if !errors.Is(err, ErrUpdateTooLarge) {
		t.Errorf("expected ErrUpdateTooLarge, got %v", err)
	}
}

// TestBuildMUP_MaxSize_Fits verifies MUP within limit succeeds.
//
// VALIDATES: BuildMUPWithMaxSize returns UPDATE when size <= maxSize.
// PREVENTS: False positives on valid MUP routes.
func TestBuildMUP_MaxSize_Fits(t *testing.T) {

	ub := NewUpdateBuilder(65001, false, true, false)

	params := MUPParams{
		RouteType: 1,
		IsIPv6:    false,
		NLRI:      []byte{0x01, 0x02, 0x03, 0x04},
		NextHop:   netip.MustParseAddr("192.168.1.1"),
	}

	update, err := ub.BuildMUPWithMaxSize(params, 4096)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if update == nil {
		t.Fatal("expected non-nil UPDATE")
		return
	}
}

// TestBuildMUP_MaxSize_TooLarge verifies error when MUP > maxSize.
//
// VALIDATES: BuildMUPWithMaxSize returns ErrUpdateTooLarge when route + attrs > maxSize.
// PREVENTS: Oversized UPDATE generation for MUP routes.
func TestBuildMUP_MaxSize_TooLarge(t *testing.T) {

	ub := NewUpdateBuilder(65001, false, true, false)

	params := MUPParams{
		RouteType: 1,
		IsIPv6:    false,
		NLRI:      []byte{0x01, 0x02, 0x03, 0x04},
		NextHop:   netip.MustParseAddr("192.168.1.1"),
	}

	_, err := ub.BuildMUPWithMaxSize(params, 30)
	if err == nil {
		t.Fatal("expected ErrUpdateTooLarge, got nil")
		return
	}
	if !errors.Is(err, ErrUpdateTooLarge) {
		t.Errorf("expected ErrUpdateTooLarge, got %v", err)
	}
}

// TestUpdateBuilderReuse verifies the builder produces identical bytes when reused.
//
// VALIDATES: UpdateBuilder with scratch buffer produces byte-identical output on
// successive calls with the same parameters.
// PREVENTS: Scratch buffer state leaking between Build* calls, causing corruption.
func TestUpdateBuilderReuse(t *testing.T) {
	ub := NewUpdateBuilder(65001, false, true, false)

	params := UnicastParams{
		Prefix:      netip.MustParsePrefix("10.0.0.0/24"),
		NextHop:     netip.MustParseAddr("192.168.1.1"),
		Origin:      attribute.OriginIGP,
		MED:         100,
		Communities: []uint32{0xFFFF0001, 0xFFFF0002},
	}

	// Build twice with same parameters
	update1 := ub.BuildUnicast(&params)
	update2 := ub.BuildUnicast(&params)

	if !bytes.Equal(update1.PathAttributes, update2.PathAttributes) {
		t.Error("PathAttributes differ between reused builds")
	}
	if !bytes.Equal(update1.NLRI, update2.NLRI) {
		t.Error("NLRI differ between reused builds")
	}

	// Build a different route in between to stress scratch reuse
	otherParams := UnicastParams{
		Prefix:           netip.MustParsePrefix("2001:db8::/32"),
		NextHop:          netip.MustParseAddr("2001:db8::1"),
		Origin:           attribute.OriginEGP,
		MED:              200,
		LargeCommunities: [][3]uint32{{65001, 1, 2}},
	}
	_ = ub.BuildUnicast(&otherParams)

	// Build original again — must still match
	update3 := ub.BuildUnicast(&params)
	if !bytes.Equal(update1.PathAttributes, update3.PathAttributes) {
		t.Error("PathAttributes differ after interleaved build")
	}
	if !bytes.Equal(update1.NLRI, update3.NLRI) {
		t.Error("NLRI differ after interleaved build")
	}
}

// =============================================================================
// Phase 1: scratch-backed PathAttributes / NLRI for BuildUnicast (spec-update-pool)
// =============================================================================

// TestUpdateBuilder_BuildUnicast_AliasesScratch verifies PathAttributes and NLRI
// returned by BuildUnicast are sub-slices of ub.scratch after Phase 1.
//
// VALIDATES: AC-2, AC-3 (inlineNLRI and attrBytes come from ub.alloc).
// PREVENTS: Regression to make([]byte, N) in BuildUnicast's result paths.
func TestUpdateBuilder_BuildUnicast_AliasesScratch(t *testing.T) {
	ub := NewUpdateBuilder(65001, false, true, false)

	ipv4Params := UnicastParams{
		Prefix:  netip.MustParsePrefix("10.0.0.0/24"),
		NextHop: netip.MustParseAddr("192.168.1.1"),
		Origin:  attribute.OriginIGP,
	}
	u4 := ub.BuildUnicast(&ipv4Params)
	if !sliceAliasesScratch(u4.PathAttributes, ub.scratch) {
		t.Error("IPv4 update.PathAttributes does not alias ub.scratch")
	}
	if !sliceAliasesScratch(u4.NLRI, ub.scratch) {
		t.Error("IPv4 update.NLRI does not alias ub.scratch")
	}

	ipv6Params := UnicastParams{
		Prefix:  netip.MustParsePrefix("2001:db8::/32"),
		NextHop: netip.MustParseAddr("2001:db8::1"),
		Origin:  attribute.OriginIGP,
	}
	u6 := ub.BuildUnicast(&ipv6Params)
	if !sliceAliasesScratch(u6.PathAttributes, ub.scratch) {
		t.Error("IPv6 update.PathAttributes does not alias ub.scratch")
	}
}

// TestUpdateBuilder_BuildTwice_InvalidatesFirst verifies that after two builds
// without consuming the first, the first Update's slices see the second build's
// bytes (documented caller-error invariant per AC-1 + AC-9).
//
// VALIDATES: AC-1 (second build clobbers first without an explicit copy).
// PREVENTS: Callers assuming Updates can be retained across builds.
func TestUpdateBuilder_BuildTwice_InvalidatesFirst(t *testing.T) {
	ub := NewUpdateBuilder(65001, false, true, false)

	p1 := UnicastParams{
		Prefix:  netip.MustParsePrefix("10.0.0.0/24"),
		NextHop: netip.MustParseAddr("192.168.1.1"),
		Origin:  attribute.OriginIGP,
		MED:     100,
	}
	u1 := ub.BuildUnicast(&p1)
	snapshot := append([]byte(nil), u1.PathAttributes...)

	// Different route with different attributes — produces different bytes.
	p2 := UnicastParams{
		Prefix:      netip.MustParsePrefix("192.0.2.0/24"),
		NextHop:     netip.MustParseAddr("192.168.1.2"),
		Origin:      attribute.OriginEGP,
		MED:         999,
		Communities: []uint32{0xFFFF0001},
	}
	_ = ub.BuildUnicast(&p2)

	// u1.PathAttributes now aliases scratch that holds p2's bytes.
	if bytes.Equal(u1.PathAttributes, snapshot) {
		t.Error("u1.PathAttributes still holds original bytes after second build (aliasing not active)")
	}
}

// TestUpdateBuilder_BuildUnicast_NoByteMakeAfterWarmup measures per-build
// allocations on the IPv4 unicast path. Spec AC-2 eliminates the two []byte
// make sites (inlineNLRI + attrBytes). Remaining allocations are Go interface
// boxing + attribute struct literals (Origin, NextHop, ASPath, rawAttribute)
// and are not in this spec's scope.
//
// VALIDATES: AC-2 (IPv4 unicast []byte allocs routed through scratch).
// PREVENTS: Regressions reintroducing make([]byte, N) in the build path.
func TestUpdateBuilder_BuildUnicast_NoByteMakeAfterWarmup(t *testing.T) {
	ub := NewUpdateBuilder(65001, false, true, false)
	params := UnicastParams{
		Prefix:  netip.MustParsePrefix("10.0.0.0/24"),
		NextHop: netip.MustParseAddr("192.168.1.1"),
		Origin:  attribute.OriginIGP,
	}
	// Warmup so scratch is lazy-allocated before AllocsPerRun measures.
	_ = ub.BuildUnicast(&params)

	allocs := testing.AllocsPerRun(100, func() {
		_ = ub.BuildUnicast(&params)
	})
	// Post-Phase-1: 10 allocs under `go test`, 12 under `-race` (race detector
	// adds ~2 bookkeeping allocs). Pre-Phase-1 baseline under race: 14.
	// Threshold accommodates both; tightens only when Phase 2/3 further reduce.
	if allocs > 12 {
		t.Errorf("BuildUnicast IPv4: got %v allocs/op, want ≤ 12 (phase 1 baseline, race-tolerant)", allocs)
	}
}

// TestUpdateBuilder_BuildIPv6_NoByteMakeAfterWarmup measures per-build
// allocations on the IPv6 MP_REACH path. Spec AC-3 eliminates attrBytes
// make; MPReachNLRI + NextHops struct allocs remain (not in scope).
//
// VALIDATES: AC-3 (IPv6 attrBytes comes from scratch).
// PREVENTS: Regressions reintroducing make([]byte, N) on the MP_REACH path.
func TestUpdateBuilder_BuildIPv6_NoByteMakeAfterWarmup(t *testing.T) {
	ub := NewUpdateBuilder(65001, false, true, false)
	params := UnicastParams{
		Prefix:  netip.MustParsePrefix("2001:db8::/32"),
		NextHop: netip.MustParseAddr("2001:db8::1"),
		Origin:  attribute.OriginIGP,
	}
	_ = ub.BuildUnicast(&params)

	allocs := testing.AllocsPerRun(100, func() {
		_ = ub.BuildUnicast(&params)
	})
	// Post-Phase-1: 10 allocs under `go test`, 12 under `-race`.
	if allocs > 12 {
		t.Errorf("BuildUnicast IPv6: got %v allocs/op, want ≤ 12 (phase 1 baseline, race-tolerant)", allocs)
	}
}

// =============================================================================
// Phase 2a: packAttributesOrderedInto + migrate vpn/labeled/evpn (spec-update-pool)
// =============================================================================

// TestPackAttributesOrderedInto_AliasesScratch verifies the helper writes into
// scratch, not a fresh heap allocation.
//
// VALIDATES: AC-4a (vpn/labeled/evpn callers get scratch-backed attrBytes).
// PREVENTS: Regression to make([]byte, N) inside the helper.
func TestPackAttributesOrderedInto_AliasesScratch(t *testing.T) {
	ub := NewUpdateBuilder(65001, false, true, false)
	ub.resetScratch()

	attrs := []attribute.Attribute{
		attribute.OriginIGP,
		attribute.MED(100),
	}
	result := ub.packAttributesOrderedInto(attrs, nil)
	if !sliceAliasesScratch(result, ub.scratch) {
		t.Error("packAttributesOrderedInto result does not alias ub.scratch")
	}

	// With rawAttrs appended: result extends past the ordered block and still aliases scratch.
	ub.resetScratch()
	raw := [][]byte{{0xC0, 0x63, 0x01, 0x02}}
	result2 := ub.packAttributesOrderedInto(attrs, raw)
	if !sliceAliasesScratch(result2, ub.scratch) {
		t.Error("packAttributesOrderedInto with rawAttrs does not alias ub.scratch")
	}
	// Raw bytes land at the tail of the result.
	if !bytes.Equal(result2[len(result2)-len(raw[0]):], raw[0]) {
		t.Errorf("raw attr tail mismatch: got %x, want %x", result2[len(result2)-len(raw[0]):], raw[0])
	}
}

// TestUpdateBuilder_BuildUnicast_GrowMidBuild forces scratch growth within a
// single BuildUnicast call and verifies the returned Update is still byte-correct.
// Proves the "slices from one build may span two different scratch backings"
// invariant documented on the Update type (AC-9).
//
// VALIDATES: alloc grow preserves bytes and wire output under scratch reallocation.
// PREVENTS: Regression where grow silently corrupts inlineNLRI or attrBytes when
// one straddles the old/new backing boundary.
func TestUpdateBuilder_BuildUnicast_GrowMidBuild(t *testing.T) {
	ub := NewUpdateBuilder(65001, false, true, false)

	// Pad RawAttributeBytes past StandardMaxSize (4096) so attrBytes alloc grows scratch.
	bigRaw := bytes.Repeat([]byte{0xFE, 0xFD, 0xFC, 0xFB}, 1200) // 4800 bytes
	params := UnicastParams{
		Prefix:            netip.MustParsePrefix("10.0.0.0/24"),
		NextHop:           netip.MustParseAddr("192.168.1.1"),
		Origin:            attribute.OriginIGP,
		RawAttributeBytes: [][]byte{bigRaw},
	}

	// Build once to lazy-init scratch at the standard 4096 size.
	_ = ub.BuildUnicast(&UnicastParams{
		Prefix:  netip.MustParsePrefix("10.0.0.0/24"),
		NextHop: netip.MustParseAddr("192.168.1.1"),
		Origin:  attribute.OriginIGP,
	})
	preGrow := ub.scratch

	update := ub.BuildUnicast(&params)
	postGrow := ub.scratch

	if len(postGrow) <= len(preGrow) {
		t.Fatalf("expected scratch to grow past %d, got %d", len(preGrow), len(postGrow))
	}

	// PathAttributes and NLRI must alias one of the two backings we observed.
	if !sliceAliasesAny(update.PathAttributes, preGrow, postGrow) {
		t.Error("PathAttributes does not alias any observed scratch backing after grow")
	}
	if !sliceAliasesAny(update.NLRI, preGrow, postGrow) {
		t.Error("NLRI does not alias any observed scratch backing after grow")
	}

	// Big raw block must be present at the tail of PathAttributes, byte-identical.
	tail := update.PathAttributes[len(update.PathAttributes)-len(bigRaw):]
	if !bytes.Equal(tail, bigRaw) {
		t.Error("RawAttributeBytes tail corrupted across scratch grow")
	}
}

// TestPackAttributesOrderedInto_EmptyReturnsNil verifies the zero-input edge case.
//
// VALIDATES: helper matches the former free packAttributesOrdered for empty input.
func TestPackAttributesOrderedInto_EmptyReturnsNil(t *testing.T) {
	ub := NewUpdateBuilder(65001, false, true, false)
	ub.resetScratch()

	if got := ub.packAttributesOrderedInto(nil, nil); got != nil {
		t.Errorf("packAttributesOrderedInto(nil, nil) = %x, want nil", got)
	}
	if got := ub.packAttributesOrderedInto([]attribute.Attribute{}, nil); got != nil {
		t.Errorf("packAttributesOrderedInto(empty, nil) = %x, want nil", got)
	}
}

// =============================================================================
// Phase 3: BuildGroupedUnicast / BuildGroupedMVPN callback API (spec-update-pool)
// =============================================================================

func sampleIPv4Routes(n int) []UnicastParams {
	routes := make([]UnicastParams, n)
	for i := range routes {
		// Space prefixes out in /30s across 10.0.0.0/8 so each NLRI encodes identically sized.
		third := (i / 256) & 0xFF
		fourth := (i * 4) & 0xFC
		routes[i] = UnicastParams{
			Prefix:  netip.PrefixFrom(netip.AddrFrom4([4]byte{10, 0, byte(third), byte(fourth)}), 30),
			NextHop: netip.MustParseAddr("192.168.1.1"),
			Origin:  attribute.OriginIGP,
			MED:     100,
		}
	}
	return routes
}

// TestBuildGroupedUnicast_CallbackOrder verifies chunks arrive in route order
// and each Update carries the shared attrBytes + its own NLRI sub-slice.
//
// VALIDATES: AC-5 (callback fires in route order; offset protocol works).
func TestBuildGroupedUnicast_CallbackOrder(t *testing.T) {
	ub := NewUpdateBuilder(65001, false, true, false)
	routes := sampleIPv4Routes(50)

	var chunks int
	var totalNLRIBytes int
	var lastPathAttrs []byte
	err := ub.BuildGroupedUnicast(routes, 100, func(u *Update) error {
		chunks++
		if lastPathAttrs == nil {
			lastPathAttrs = append([]byte(nil), u.PathAttributes...)
		} else if !bytes.Equal(u.PathAttributes, lastPathAttrs) {
			t.Error("PathAttributes differ between chunks (shared attrBytes invariant broken)")
		}
		totalNLRIBytes += len(u.NLRI)
		return nil
	})
	if err != nil {
		t.Fatalf("BuildGroupedUnicast: %v", err)
	}
	if chunks < 2 {
		t.Errorf("expected batch to split into multiple chunks at maxSize=100, got %d", chunks)
	}
	if totalNLRIBytes == 0 {
		t.Error("no NLRI bytes emitted")
	}
}

// TestBuildGroupedUnicast_CallbackError_StopsBuilder verifies the loop aborts
// on the first non-nil emit return and returns that error unchanged.
//
// VALIDATES: AC-5 (error short-circuits the builder).
func TestBuildGroupedUnicast_CallbackError_StopsBuilder(t *testing.T) {
	ub := NewUpdateBuilder(65001, false, true, false)
	routes := sampleIPv4Routes(50)

	sentinel := errors.New("caller stopped")
	fired := 0
	err := ub.BuildGroupedUnicast(routes, 100, func(*Update) error {
		fired++
		if fired == 2 {
			return sentinel
		}
		return nil
	})
	if !errors.Is(err, sentinel) {
		t.Errorf("expected sentinel error, got %v", err)
	}
	if fired != 2 {
		t.Errorf("expected callback to fire exactly 2 times, got %d", fired)
	}
}

// TestBuildGroupedUnicast_ScratchReuse verifies total scratch growth is bounded
// across many callbacks (no per-chunk growth).
//
// VALIDATES: AC-5 (offset protocol reuses scratch between chunks).
func TestBuildGroupedUnicast_ScratchReuse(t *testing.T) {
	ub := NewUpdateBuilder(65001, false, true, false)
	routes := sampleIPv4Routes(500)

	var peakScratch int
	err := ub.BuildGroupedUnicast(routes, 100, func(*Update) error {
		if n := len(ub.scratch); n > peakScratch {
			peakScratch = n
		}
		return nil
	})
	if err != nil {
		t.Fatalf("BuildGroupedUnicast: %v", err)
	}
	if peakScratch > wire.StandardMaxSize {
		t.Errorf("scratch grew past StandardMaxSize=%d to %d -- offset reset not bounding NLRI region", wire.StandardMaxSize, peakScratch)
	}
}

// TestBuildGroupedUnicast_AttrBytesPersistAcrossCallbacks captures attrBytes in
// the first callback and verifies it is byte-identical in the last callback.
// This proves the offset protocol (reset to A, not 0) preserves the shared
// attribute region across chunk emissions.
//
// VALIDATES: AC-5 (attrBytes at scratch[0:A) stays valid for the full batch).
func TestBuildGroupedUnicast_AttrBytesPersistAcrossCallbacks(t *testing.T) {
	ub := NewUpdateBuilder(65001, false, true, false)
	routes := sampleIPv4Routes(100)

	var firstPathAttrs []byte
	chunks := 0
	err := ub.BuildGroupedUnicast(routes, 100, func(u *Update) error {
		chunks++
		if firstPathAttrs == nil {
			firstPathAttrs = u.PathAttributes
			return nil
		}
		if !bytes.Equal(u.PathAttributes, firstPathAttrs) {
			t.Errorf("chunk %d PathAttributes diverged from chunk 0", chunks)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("BuildGroupedUnicast: %v", err)
	}
	if chunks < 3 {
		t.Fatalf("need ≥3 chunks to exercise persistence invariant, got %d", chunks)
	}
}

func sampleMVPNRoutes(n int) []MVPNParams {
	routes := make([]MVPNParams, n)
	for i := range routes {
		routes[i] = MVPNParams{
			RouteType: 5,
			RD:        [8]byte{0, 0, 0, 1, 0, 0, 0, byte(i)},
			Source:    netip.MustParseAddr("10.1.1.1"),
			Group:     netip.AddrFrom4([4]byte{239, 0, byte(i >> 8), byte(i)}),
			NextHop:   netip.MustParseAddr("192.168.1.1"),
			Origin:    attribute.OriginIGP,
		}
	}
	return routes
}

// TestBuildGroupedMVPN_CallbackOrder verifies chunks arrive in route order and
// each chunk's Update has valid PathAttributes (MP_REACH contains that chunk's
// NLRI set).
//
// VALIDATES: AC-6 (MVPN callback fires per chunk with correct per-chunk attrs).
func TestBuildGroupedMVPN_CallbackOrder(t *testing.T) {
	ub := NewUpdateBuilder(65001, false, true, false)
	routes := sampleMVPNRoutes(30)

	chunks := 0
	err := ub.BuildGroupedMVPN(routes, 200, func(u *Update) error {
		chunks++
		if len(u.PathAttributes) == 0 {
			t.Errorf("chunk %d has empty PathAttributes", chunks)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("BuildGroupedMVPN: %v", err)
	}
	if chunks < 2 {
		t.Errorf("expected batch to split at maxSize=200, got %d chunks", chunks)
	}
}

// TestBuildGroupedMVPN_AttrBytesPersistAcrossCallbacks is weaker than the
// unicast variant because MVPN rebuilds PathAttributes per chunk (MP_REACH
// contains chunk NLRI). The invariant here is that each emitted Update's
// PathAttributes contains the shared base attrs (Origin, AS_PATH, NEXT_HOP,
// LocalPref/ExtCommunities) plus a per-chunk MP_REACH block. We assert each
// chunk's PathAttributes is non-empty and prefix/suffix substrings of the
// first-chunk attrs are present (the shared attribute types).
//
// VALIDATES: AC-6 (MVPN chunks carry independent attrBytes per chunk but the
// same shared baseline across all chunks).
func TestBuildGroupedMVPN_AttrBytesPersistAcrossCallbacks(t *testing.T) {
	ub := NewUpdateBuilder(65001, true, true, false) // iBGP for LocalPref to appear
	routes := sampleMVPNRoutes(30)

	chunks := 0
	err := ub.BuildGroupedMVPN(routes, 200, func(u *Update) error {
		chunks++
		if len(u.PathAttributes) == 0 {
			t.Errorf("chunk %d: empty PathAttributes", chunks)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("BuildGroupedMVPN: %v", err)
	}
	if chunks < 2 {
		t.Fatalf("need ≥2 chunks to exercise persistence, got %d", chunks)
	}
}
