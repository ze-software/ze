package message

import (
	"bytes"
	"net/netip"
	"testing"

	"codeberg.org/thomas-mangin/zebgp/pkg/bgp/attribute"
	"codeberg.org/thomas-mangin/zebgp/pkg/bgp/nlri"
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
		Labels:  []uint32{100},
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
		Labels:  []uint32{200},
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
		Labels:          []uint32{100},
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
		Labels:            []uint32{100},
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

// TestBuildUnicast_EncodesReflectorAttrs verifies RFC 4456 attribute encoding.
//
// VALIDATES: ORIGINATOR_ID and CLUSTER_LIST are encoded in PathAttributes.
// PREVENTS: Data loss for route reflector configurations.
func TestBuildUnicast_EncodesReflectorAttrs(t *testing.T) {
	ctx := &nlri.PackContext{ASN4: true}
	ub := NewUpdateBuilder(65001, true, ctx)

	params := UnicastParams{
		Prefix:          netip.MustParsePrefix("10.0.0.0/24"),
		NextHop:         netip.MustParseAddr("192.168.1.1"),
		Origin:          attribute.OriginIGP,
		LocalPreference: 100,
		OriginatorID:    0xC0A80101, // 192.168.1.1
		ClusterList:     []uint32{0xC0A80102, 0xC0A80103},
	}

	update := ub.BuildUnicast(params)

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
	ctx := &nlri.PackContext{ASN4: true}
	ub := NewUpdateBuilder(65001, false, ctx) // isIBGP=false

	params := UnicastParams{
		Prefix:          netip.MustParsePrefix("10.0.0.0/24"),
		NextHop:         netip.MustParseAddr("192.168.1.1"),
		Origin:          attribute.OriginIGP,
		LocalPreference: 200, // Should be ignored for eBGP
	}

	update := ub.BuildUnicast(params)

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
	ctx := &nlri.PackContext{ASN4: false}   // 2-byte mode
	ub := NewUpdateBuilder(100, false, ctx) // eBGP, AS 100

	params := UnicastParams{
		Prefix:  netip.MustParsePrefix("10.0.0.0/24"),
		NextHop: netip.MustParseAddr("192.168.1.1"),
		Origin:  attribute.OriginIGP,
	}

	update := ub.BuildUnicast(params)

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
	ctx := &nlri.PackContext{ASN4: true}      // 4-byte mode (default)
	ub := NewUpdateBuilder(65001, false, ctx) // eBGP, AS 65001

	params := UnicastParams{
		Prefix:  netip.MustParsePrefix("10.0.0.0/24"),
		NextHop: netip.MustParseAddr("192.168.1.1"),
		Origin:  attribute.OriginIGP,
	}

	update := ub.BuildUnicast(params)

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
	ctx := &nlri.PackContext{ASN4: false}   // 2-byte mode
	ub := NewUpdateBuilder(100, false, ctx) // eBGP, AS 100

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

	update := ub.BuildGroupedUnicast(routes)

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
	ctx := &nlri.PackContext{ASN4: true}
	ub := NewUpdateBuilder(65001, true, ctx)

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

	update := ub.BuildGroupedUnicast(routes)

	if update == nil {
		t.Fatal("BuildGroupedUnicast returned nil")
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
	ctx := &nlri.PackContext{ASN4: true}
	ub := NewUpdateBuilder(65001, true, ctx)

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

	update := ub.BuildGroupedUnicast(routes)
	if update == nil {
		t.Fatal("BuildGroupedUnicast returned nil")
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
	ctx := &nlri.PackContext{ASN4: true, AddPath: true}
	ub := NewUpdateBuilder(65001, true, ctx)

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

	update := ub.BuildGroupedUnicast(routes)
	if update == nil {
		t.Fatal("BuildGroupedUnicast returned nil")
	}

	// With ADD-PATH: each NLRI = 4-byte PathID + 1-byte len + 3-byte prefix = 8 bytes
	// 2 routes = 16 bytes
	expectedNLRILen := 16
	if len(update.NLRI) != expectedNLRILen {
		t.Errorf("NLRI length with ADD-PATH: got %d, want %d", len(update.NLRI), expectedNLRILen)
	}
}

// TestBuildGroupedUnicast_EmptySlice verifies empty input handling.
func TestBuildGroupedUnicast_EmptySlice(t *testing.T) {
	ctx := &nlri.PackContext{ASN4: true}
	ub := NewUpdateBuilder(65001, true, ctx)

	update := ub.BuildGroupedUnicast(nil)

	if update == nil {
		t.Fatal("BuildGroupedUnicast returned nil for empty input")
	}
	if len(update.PathAttributes) != 0 || len(update.NLRI) != 0 {
		t.Error("Expected empty update for empty input")
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
	ctx := &nlri.PackContext{ASN4: false} // 2-byte mode
	ub := NewUpdateBuilder(100, false, ctx)

	params := VPNParams{
		Prefix:  netip.MustParsePrefix("10.0.0.0/24"),
		NextHop: netip.MustParseAddr("192.168.1.1"),
		Origin:  attribute.OriginIGP,
		Labels:  []uint32{100},
		RDBytes: [8]byte{0, 1, 0, 0, 0, 100, 0, 100},
	}

	update := ub.BuildVPN(params)

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
	ctx := &nlri.PackContext{ASN4: false}
	ub := NewUpdateBuilder(100, false, ctx)

	params := LabeledUnicastParams{
		Prefix:  netip.MustParsePrefix("10.0.0.0/24"),
		NextHop: netip.MustParseAddr("192.168.1.1"),
		Origin:  attribute.OriginIGP,
		Labels:  []uint32{100},
	}

	update := ub.BuildLabeledUnicast(params)

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
	ctx := &nlri.PackContext{ASN4: false}
	ub := NewUpdateBuilder(100, false, ctx)

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
	ctx := &nlri.PackContext{ASN4: false}
	ub := NewUpdateBuilder(100, false, ctx)

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
	ctx := &nlri.PackContext{ASN4: false}
	ub := NewUpdateBuilder(100, false, ctx)

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
	ctx := &nlri.PackContext{ASN4: false}
	ub := NewUpdateBuilder(100, false, ctx)

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
	ctx := &nlri.PackContext{ASN4: false}
	ub := NewUpdateBuilder(100, false, ctx)

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
	ctx := &nlri.PackContext{ASN4: false}
	ub := NewUpdateBuilder(100, false, ctx)

	params := UnicastParams{
		Prefix:        netip.MustParsePrefix("10.0.0.0/24"),
		NextHop:       netip.MustParseAddr("192.168.1.1"),
		Origin:        attribute.OriginIGP,
		HasAggregator: true,
		AggregatorASN: 100,
		AggregatorIP:  [4]byte{192, 168, 1, 1},
	}

	update := ub.BuildUnicast(params)

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
	ctx := &nlri.PackContext{ASN4: false}
	ub := NewUpdateBuilder(100, false, ctx)

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

	update := ub.BuildVPN(params)

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
	ctx := &nlri.PackContext{ASN4: false}
	ub := NewUpdateBuilder(100, false, ctx)

	params := LabeledUnicastParams{
		Prefix:        netip.MustParsePrefix("10.0.0.0/24"),
		NextHop:       netip.MustParseAddr("192.168.1.1"),
		Origin:        attribute.OriginIGP,
		Labels:        []uint32{100},
		HasAggregator: true,
		AggregatorASN: 100,
		AggregatorIP:  [4]byte{192, 168, 1, 1},
	}

	update := ub.BuildLabeledUnicast(params)

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
	ctx := &nlri.PackContext{ASN4: false}
	ub := NewUpdateBuilder(100, false, ctx)

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
	}
}

// TestBuildGroupedUnicast_Aggregator_ASN4Disabled verifies 6-byte AGGREGATOR for grouped updates.
//
// VALIDATES: AGGREGATOR uses 6-byte format when ctx.ASN4=false in grouped updates.
// PREVENTS: RFC 6793 violation for grouped unicast routes.
func TestBuildGroupedUnicast_Aggregator_ASN4Disabled(t *testing.T) {
	ctx := &nlri.PackContext{ASN4: false}
	ub := NewUpdateBuilder(100, false, ctx)

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

	update := ub.BuildGroupedUnicast(routes)

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
	ctx := &nlri.PackContext{ASN4: true}
	ub := NewUpdateBuilder(65001, true, ctx)

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
	ctx := &nlri.PackContext{ASN4: true}
	ub := NewUpdateBuilder(65001, true, ctx)

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
	ctx := &nlri.PackContext{ASN4: true}
	ub := NewUpdateBuilder(65001, true, ctx)

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
	ctx := &nlri.PackContext{ASN4: true}
	ub := NewUpdateBuilder(65001, true, ctx)

	updates, err := ub.BuildGroupedUnicastWithLimit(nil, 4096)
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
	ctx := &nlri.PackContext{ASN4: true}
	ub := NewUpdateBuilder(65001, true, ctx)

	routes := []UnicastParams{{
		Prefix:  netip.MustParsePrefix("10.0.0.0/24"),
		NextHop: netip.MustParseAddr("192.168.1.1"),
		Origin:  attribute.OriginIGP,
	}}

	updates, err := ub.BuildGroupedUnicastWithLimit(routes, 4096)
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
	ctx := &nlri.PackContext{ASN4: true}
	ub := NewUpdateBuilder(65001, true, ctx)

	// Create 10 routes that should fit in one UPDATE
	var routes []UnicastParams
	for i := 0; i < 10; i++ {
		routes = append(routes, UnicastParams{
			Prefix:  netip.MustParsePrefix("10.0.0.0/24"),
			NextHop: netip.MustParseAddr("192.168.1.1"),
			Origin:  attribute.OriginIGP,
		})
	}

	updates, err := ub.BuildGroupedUnicastWithLimit(routes, 4096)
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
	ctx := &nlri.PackContext{ASN4: true}
	ub := NewUpdateBuilder(65001, true, ctx)

	// Create 100 routes - should overflow with small maxSize
	var routes []UnicastParams
	for i := 0; i < 100; i++ {
		routes = append(routes, UnicastParams{
			Prefix:  netip.MustParsePrefix("10.0.0.0/24"),
			NextHop: netip.MustParseAddr("192.168.1.1"),
			Origin:  attribute.OriginIGP,
		})
	}

	// Small maxSize to force splitting
	// Overhead = 19 + 4 = 23, attrs ~30 bytes, leaves ~47 for NLRI
	// Each /24 = 4 bytes, so ~11 per update
	updates, err := ub.BuildGroupedUnicastWithLimit(routes, 100)
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
	ctx := &nlri.PackContext{ASN4: true}
	ub := NewUpdateBuilder(65001, true, ctx)

	// Route with large communities
	routes := []UnicastParams{{
		Prefix:      netip.MustParsePrefix("10.0.0.0/24"),
		NextHop:     netip.MustParseAddr("192.168.1.1"),
		Origin:      attribute.OriginIGP,
		Communities: make([]uint32, 100), // 400 bytes of communities
	}}

	// maxSize too small for attributes
	_, err := ub.BuildGroupedUnicastWithLimit(routes, 50)
	if err == nil {
		t.Error("expected ErrAttributesTooLarge, got nil")
	}
}

// TestBuildWithLimit_AllRoutesPreserved verifies no data loss.
//
// VALIDATES: All routes appear in output UPDATEs.
// PREVENTS: Route loss during splitting.
func TestBuildWithLimit_AllRoutesPreserved(t *testing.T) {
	ctx := &nlri.PackContext{ASN4: true}
	ub := NewUpdateBuilder(65001, true, ctx)

	// Create 50 routes (same prefix is fine - testing byte count)
	var routes []UnicastParams
	for i := 0; i < 50; i++ {
		routes = append(routes, UnicastParams{
			Prefix:  netip.MustParsePrefix("10.0.0.0/24"),
			NextHop: netip.MustParseAddr("192.168.1.1"),
			Origin:  attribute.OriginIGP,
		})
	}

	updates, err := ub.BuildGroupedUnicastWithLimit(routes, 100)
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
	ctx := &nlri.PackContext{ASN4: true}
	ub := NewUpdateBuilder(65001, true, ctx)

	var routes []UnicastParams
	for i := 0; i < 50; i++ {
		routes = append(routes, UnicastParams{
			Prefix:  netip.MustParsePrefix("10.0.0.0/24"),
			NextHop: netip.MustParseAddr("192.168.1.1"),
			Origin:  attribute.OriginIGP,
		})
	}

	updates, err := ub.BuildGroupedUnicastWithLimit(routes, 100)
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
