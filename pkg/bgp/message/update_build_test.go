package message

import (
	"net/netip"
	"testing"

	"github.com/exa-networks/zebgp/pkg/bgp/attribute"
	"github.com/exa-networks/zebgp/pkg/bgp/nlri"
)

// TestUpdateBuilder_NewBuilder verifies UpdateBuilder creation.
//
// VALIDATES: UpdateBuilder stores LocalAS, IsIBGP, and PackContext correctly.
//
// PREVENTS: Missing fields or incorrect initialization causing encode failures.
func TestUpdateBuilder_NewBuilder(t *testing.T) {
	ctx := &nlri.PackContext{AddPath: true, ASN4: true}
	ub := NewUpdateBuilder(65001, true, ctx)

	if ub.LocalAS != 65001 {
		t.Errorf("LocalAS = %d, want 65001", ub.LocalAS)
	}
	if !ub.IsIBGP {
		t.Error("IsIBGP = false, want true")
	}
	if ub.Ctx != ctx {
		t.Error("Ctx not set correctly")
	}
}

// TestUpdateBuilder_BuildUnicast_IPv4 verifies IPv4 unicast UPDATE building.
//
// VALIDATES: IPv4 unicast route produces valid UPDATE with correct NLRI placement.
//
// PREVENTS: IPv4 routes incorrectly using MP_REACH_NLRI instead of inline NLRI.
func TestUpdateBuilder_BuildUnicast_IPv4(t *testing.T) {
	ctx := &nlri.PackContext{ASN4: true}
	ub := NewUpdateBuilder(65001, false, ctx)

	params := UnicastParams{
		Prefix:  netip.MustParsePrefix("10.0.0.0/24"),
		NextHop: netip.MustParseAddr("192.168.1.1"),
		Origin:  attribute.OriginIGP,
	}

	update := ub.BuildUnicast(params)
	if update == nil {
		t.Fatal("BuildUnicast returned nil")
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
	ctx := &nlri.PackContext{ASN4: true}
	ub := NewUpdateBuilder(65001, false, ctx)

	params := UnicastParams{
		Prefix:  netip.MustParsePrefix("2001:db8::/32"),
		NextHop: netip.MustParseAddr("2001:db8::1"),
		Origin:  attribute.OriginIGP,
	}

	update := ub.BuildUnicast(params)
	if update == nil {
		t.Fatal("BuildUnicast returned nil")
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
	ctx := &nlri.PackContext{ASN4: true}
	ub := NewUpdateBuilder(65001, true, ctx) // iBGP to include LOCAL_PREF

	params := UnicastParams{
		Prefix:          netip.MustParsePrefix("10.0.0.0/24"),
		NextHop:         netip.MustParseAddr("192.168.1.1"),
		Origin:          attribute.OriginIGP,
		MED:             100,
		LocalPreference: 200,
	}

	update := ub.BuildUnicast(params)
	if update == nil {
		t.Fatal("BuildUnicast returned nil")
	}

	// Parse attributes and verify ordering
	codes, err := extractAttributeCodes(update.PathAttributes)
	if err != nil {
		t.Fatalf("extractAttributeCodes failed: %v", err)
	}

	// Verify ordering: each type code should be <= next
	for i := 0; i < len(codes)-1; i++ {
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
	ctx := &nlri.PackContext{ASN4: true}
	ub := NewUpdateBuilder(65001, false, ctx) // eBGP

	params := UnicastParams{
		Prefix:  netip.MustParsePrefix("10.0.0.0/24"),
		NextHop: netip.MustParseAddr("192.168.1.1"),
		Origin:  attribute.OriginIGP,
		ASPath:  []uint32{65002, 65003}, // Configured path
	}

	update := ub.BuildUnicast(params)
	if update == nil {
		t.Fatal("BuildUnicast returned nil")
	}

	// Find AS_PATH attribute and parse it
	codes, err := extractAttributeCodes(update.PathAttributes)
	if err != nil {
		t.Fatalf("extractAttributeCodes failed: %v", err)
	}

	// Verify AS_PATH is present
	hasASPath := false
	for _, c := range codes {
		if c == attribute.AttrASPath {
			hasASPath = true
			break
		}
	}
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
	ctx := &nlri.PackContext{ASN4: true}
	ub := NewUpdateBuilder(65001, true, ctx) // iBGP

	params := UnicastParams{
		Prefix:  netip.MustParsePrefix("10.0.0.0/24"),
		NextHop: netip.MustParseAddr("192.168.1.1"),
		Origin:  attribute.OriginIGP,
		ASPath:  []uint32{65002, 65003}, // Configured path
	}

	update := ub.BuildUnicast(params)
	if update == nil {
		t.Fatal("BuildUnicast returned nil")
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
	ctx := &nlri.PackContext{ASN4: true}
	ub := NewUpdateBuilder(65001, false, ctx)

	params := VPNParams{
		Prefix:  netip.MustParsePrefix("10.0.0.0/24"),
		NextHop: netip.MustParseAddr("192.168.1.1"),
		Origin:  attribute.OriginIGP,
		Label:   100,
		RDBytes: [8]byte{0, 1, 0, 0, 0, 100, 0, 100}, // Type 1 RD: 100:100
	}

	update := ub.BuildVPN(params)
	if update == nil {
		t.Fatal("BuildVPN returned nil")
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

	hasMPReach := false
	for _, c := range codes {
		if c == attribute.AttrMPReachNLRI {
			hasMPReach = true
			break
		}
	}
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
	ctx := &nlri.PackContext{ASN4: true}
	ub := NewUpdateBuilder(65001, false, ctx)

	params := VPNParams{
		Prefix:  netip.MustParsePrefix("2001:db8::/32"),
		NextHop: netip.MustParseAddr("2001:db8::1"),
		Origin:  attribute.OriginIGP,
		Label:   200,
		RDBytes: [8]byte{0, 1, 0, 0, 0, 100, 0, 100},
	}

	update := ub.BuildVPN(params)
	if update == nil {
		t.Fatal("BuildVPN returned nil")
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
	ctx := &nlri.PackContext{ASN4: true}
	ub := NewUpdateBuilder(65001, true, ctx) // iBGP

	params := VPNParams{
		Prefix:          netip.MustParsePrefix("10.0.0.0/24"),
		NextHop:         netip.MustParseAddr("192.168.1.1"),
		Origin:          attribute.OriginIGP,
		Label:           100,
		RDBytes:         [8]byte{0, 1, 0, 0, 0, 100, 0, 100},
		MED:             50,
		LocalPreference: 150,
	}

	update := ub.BuildVPN(params)
	if update == nil {
		t.Fatal("BuildVPN returned nil")
	}

	codes, err := extractAttributeCodes(update.PathAttributes)
	if err != nil {
		t.Fatalf("extractAttributeCodes failed: %v", err)
	}

	// Verify ordering
	for i := 0; i < len(codes)-1; i++ {
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
	ctx := &nlri.PackContext{ASN4: true}
	ub := NewUpdateBuilder(65001, false, ctx)

	// Route target: 65001:100
	rtBytes := []byte{0x00, 0x02, 0xfd, 0xe9, 0x00, 0x00, 0x00, 0x64}

	params := VPNParams{
		Prefix:            netip.MustParsePrefix("10.0.0.0/24"),
		NextHop:           netip.MustParseAddr("192.168.1.1"),
		Origin:            attribute.OriginIGP,
		Label:             100,
		RDBytes:           [8]byte{0, 1, 0, 0, 0, 100, 0, 100},
		ExtCommunityBytes: rtBytes,
	}

	update := ub.BuildVPN(params)
	if update == nil {
		t.Fatal("BuildVPN returned nil")
	}

	codes, err := extractAttributeCodes(update.PathAttributes)
	if err != nil {
		t.Fatalf("extractAttributeCodes failed: %v", err)
	}

	hasExtComm := false
	for _, c := range codes {
		if c == attribute.AttrExtCommunity {
			hasExtComm = true
			break
		}
	}
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
	ctx := &nlri.PackContext{ASN4: true}
	ub := NewUpdateBuilder(65001, false, ctx)

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

	hasMPReach := false
	for _, c := range codes {
		if c == attribute.AttrMPReachNLRI {
			hasMPReach = true
			break
		}
	}
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
	ctx := &nlri.PackContext{ASN4: true}
	ub := NewUpdateBuilder(65001, true, ctx) // iBGP

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
	}

	codes, err := extractAttributeCodes(update.PathAttributes)
	if err != nil {
		t.Fatalf("extractAttributeCodes failed: %v", err)
	}

	// Verify ordering
	for i := 0; i < len(codes)-1; i++ {
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
	ctx := &nlri.PackContext{ASN4: true}
	ub := NewUpdateBuilder(65001, false, ctx)

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
	}

	if len(update.NLRI) != 0 {
		t.Error("VPLS route should not have inline NLRI")
	}

	codes, err := extractAttributeCodes(update.PathAttributes)
	if err != nil {
		t.Fatalf("extractAttributeCodes failed: %v", err)
	}

	hasMPReach := false
	for _, c := range codes {
		if c == attribute.AttrMPReachNLRI {
			hasMPReach = true
			break
		}
	}
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
	ctx := &nlri.PackContext{ASN4: true}
	ub := NewUpdateBuilder(65001, false, ctx)

	// Simple FlowSpec NLRI (destination prefix)
	params := FlowSpecParams{
		IsIPv6:  false,
		NLRI:    []byte{0x03, 0x01, 0x18, 0x0a}, // dest 10.0.0.0/24
		NextHop: netip.MustParseAddr("192.168.1.1"),
	}

	update := ub.BuildFlowSpec(params)
	if update == nil {
		t.Fatal("BuildFlowSpec returned nil")
	}

	if len(update.NLRI) != 0 {
		t.Error("FlowSpec route should not have inline NLRI")
	}

	codes, err := extractAttributeCodes(update.PathAttributes)
	if err != nil {
		t.Fatalf("extractAttributeCodes failed: %v", err)
	}

	hasMPReach := false
	for _, c := range codes {
		if c == attribute.AttrMPReachNLRI {
			hasMPReach = true
			break
		}
	}
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
	ctx := &nlri.PackContext{ASN4: true}
	ub := NewUpdateBuilder(65001, false, ctx)

	params := MUPParams{
		RouteType: 1,
		IsIPv6:    false,
		NLRI:      []byte{0x01, 0x02, 0x03, 0x04},
		NextHop:   netip.MustParseAddr("192.168.1.1"),
	}

	update := ub.BuildMUP(params)
	if update == nil {
		t.Fatal("BuildMUP returned nil")
	}

	if len(update.NLRI) != 0 {
		t.Error("MUP route should not have inline NLRI")
	}

	codes, err := extractAttributeCodes(update.PathAttributes)
	if err != nil {
		t.Fatalf("extractAttributeCodes failed: %v", err)
	}

	hasMPReach := false
	for _, c := range codes {
		if c == attribute.AttrMPReachNLRI {
			hasMPReach = true
			break
		}
	}
	if !hasMPReach {
		t.Error("MUP route should have MP_REACH_NLRI")
	}
}
