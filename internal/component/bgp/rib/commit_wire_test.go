package rib

import (
	"net/netip"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/attribute"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/nlri"
)

// TestCommitService_IPv4_HasNextHop verifies IPv4 unicast UPDATE has NEXT_HOP attribute.
//
// VALIDATES: IPv4 unicast routes → NEXT_HOP attribute (code 3) in path attributes
//
// PREVENTS: Missing mandatory NEXT_HOP attribute in IPv4 unicast UPDATEs.
func TestCommitService_IPv4_HasNextHop(t *testing.T) {
	sender := &mockUpdateSender{}
	neg := testContext(65000, 65000, true)
	cs := NewCommitService(sender, neg, true)

	nh := netip.MustParseAddr("10.0.0.1")
	attrs := []attribute.Attribute{attribute.Origin(0)}

	routes := []*Route{
		NewRoute(newIPv4NLRI("192.168.1.0/24"), nh, attrs),
	}

	_, err := cs.Commit(routes, CommitOptions{})
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	if len(sender.updates) != 1 {
		t.Fatalf("expected 1 update, got %d", len(sender.updates))
	}

	update := sender.updates[0]

	// Verify NLRI is in UPDATE.NLRI (for IPv4 unicast)
	if len(update.NLRI) == 0 {
		t.Error("expected NLRI in UPDATE.NLRI for IPv4 unicast")
	}

	// Verify path attributes contain NEXT_HOP (attribute code 3)
	hasNextHop := false
	offset := 0
	for offset < len(update.PathAttributes) {
		if offset+2 > len(update.PathAttributes) {
			break
		}
		code := update.PathAttributes[offset+1]
		flags := update.PathAttributes[offset]
		var attrLen int
		if flags&0x10 != 0 { // Extended length
			if offset+4 > len(update.PathAttributes) {
				break
			}
			attrLen = int(update.PathAttributes[offset+2])<<8 | int(update.PathAttributes[offset+3])
			offset += 4
		} else {
			if offset+3 > len(update.PathAttributes) {
				break
			}
			attrLen = int(update.PathAttributes[offset+2])
			offset += 3
		}

		if code == 3 { // NEXT_HOP
			hasNextHop = true
			// Verify next-hop value is 10.0.0.1
			if attrLen != 4 {
				t.Errorf("expected NEXT_HOP length 4, got %d", attrLen)
			}
			if offset+attrLen <= len(update.PathAttributes) {
				nhBytes := update.PathAttributes[offset : offset+attrLen]
				if nhBytes[0] != 10 || nhBytes[1] != 0 || nhBytes[2] != 0 || nhBytes[3] != 1 {
					t.Errorf("expected next-hop 10.0.0.1, got %v", nhBytes)
				}
			}
		}
		offset += attrLen
	}

	if !hasNextHop {
		t.Error("UPDATE missing mandatory NEXT_HOP attribute for IPv4 unicast")
	}
}

// TestCommitService_IPv6_UsesMPReachNLRI verifies IPv6 routes use MP_REACH_NLRI.
//
// VALIDATES: IPv6 routes → MP_REACH_NLRI attribute with next-hop and NLRI inside
//
// PREVENTS: IPv6 NLRIs incorrectly placed in UPDATE.NLRI field.
func TestCommitService_IPv6_UsesMPReachNLRI(t *testing.T) {
	sender := &mockUpdateSender{}
	neg := testContext(65000, 65000, true)
	cs := NewCommitService(sender, neg, true)

	nh := netip.MustParseAddr("2001:db8::1")
	attrs := []attribute.Attribute{attribute.Origin(0)}

	routes := []*Route{
		NewRoute(newIPv6NLRI("2001:db8:1::/48"), nh, attrs),
	}

	_, err := cs.Commit(routes, CommitOptions{})
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	if len(sender.updates) != 1 {
		t.Fatalf("expected 1 update, got %d", len(sender.updates))
	}

	update := sender.updates[0]

	// For IPv6, UPDATE.NLRI MUST be empty
	if len(update.NLRI) != 0 {
		t.Errorf("expected empty UPDATE.NLRI for IPv6, got %d bytes", len(update.NLRI))
	}

	// Verify path attributes contain MP_REACH_NLRI (attribute code 14)
	hasMPReach := false
	offset := 0
	for offset < len(update.PathAttributes) {
		if offset+2 > len(update.PathAttributes) {
			break
		}
		code := update.PathAttributes[offset+1]
		flags := update.PathAttributes[offset]
		var attrLen int
		var hdrLen int
		if flags&0x10 != 0 { // Extended length
			if offset+4 > len(update.PathAttributes) {
				break
			}
			attrLen = int(update.PathAttributes[offset+2])<<8 | int(update.PathAttributes[offset+3])
			hdrLen = 4
		} else {
			if offset+3 > len(update.PathAttributes) {
				break
			}
			attrLen = int(update.PathAttributes[offset+2])
			hdrLen = 3
		}

		if code == 14 { // MP_REACH_NLRI
			hasMPReach = true
			valueStart := offset + hdrLen
			if valueStart+5 > len(update.PathAttributes) {
				t.Fatal("MP_REACH_NLRI too short")
			}

			// Parse AFI/SAFI
			afi := uint16(update.PathAttributes[valueStart])<<8 | uint16(update.PathAttributes[valueStart+1])
			safi := update.PathAttributes[valueStart+2]

			if afi != 2 {
				t.Errorf("expected AFI 2 (IPv6), got %d", afi)
			}
			if safi != 1 {
				t.Errorf("expected SAFI 1 (unicast), got %d", safi)
			}

			// Verify next-hop length (should be 16 for single IPv6)
			nhLen := update.PathAttributes[valueStart+3]
			if nhLen != 16 {
				t.Errorf("expected next-hop length 16, got %d", nhLen)
			}
		}
		offset += hdrLen + attrLen
	}

	if !hasMPReach {
		t.Error("UPDATE missing MP_REACH_NLRI attribute for IPv6")
	}
}

// TestCommitService_ASN4_EncodesASPath verifies AS_PATH uses 4-byte encoding when ASN4=true.
//
// VALIDATES: ASN4=true → 4-byte AS numbers in AS_PATH
//
// PREVENTS: Wrong AS_PATH encoding based on capability negotiation.
func TestCommitService_ASN4_EncodesASPath(t *testing.T) {
	sender := &mockUpdateSender{}
	// eBGP session with ASN4
	neg := testContext(65000, 65001, true)
	cs := NewCommitService(sender, neg, true)

	nh := netip.MustParseAddr("10.0.0.1")
	attrs := []attribute.Attribute{attribute.Origin(0)}

	routes := []*Route{
		NewRoute(newIPv4NLRI("192.168.1.0/24"), nh, attrs),
	}

	_, err := cs.Commit(routes, CommitOptions{})
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	if len(sender.updates) != 1 {
		t.Fatalf("expected 1 update, got %d", len(sender.updates))
	}

	update := sender.updates[0]

	// Verify AS_PATH attribute exists (code 2)
	hasASPath := false
	offset := 0
	for offset < len(update.PathAttributes) {
		if offset+2 > len(update.PathAttributes) {
			break
		}
		code := update.PathAttributes[offset+1]
		flags := update.PathAttributes[offset]
		var attrLen int
		var hdrLen int
		if flags&0x10 != 0 {
			if offset+4 > len(update.PathAttributes) {
				break
			}
			attrLen = int(update.PathAttributes[offset+2])<<8 | int(update.PathAttributes[offset+3])
			hdrLen = 4
		} else {
			if offset+3 > len(update.PathAttributes) {
				break
			}
			attrLen = int(update.PathAttributes[offset+2])
			hdrLen = 3
		}

		if code == 2 { // AS_PATH
			hasASPath = true
			// For eBGP with 1 AS, expect: segment type(1) + count(1) + ASN(4) = 6 bytes
			if attrLen < 6 {
				t.Errorf("expected AS_PATH length >= 6 for 4-byte AS, got %d", attrLen)
			}
		}
		offset += hdrLen + attrLen
	}

	if !hasASPath {
		t.Error("UPDATE missing AS_PATH attribute")
	}
}

// TestCommitService_iBGP_NoASPrepend verifies iBGP sessions don't prepend local AS.
//
// VALIDATES: LocalAS == PeerAS → empty AS_PATH (no prepending)
//
// PREVENTS: Incorrect AS prepending on iBGP sessions.
func TestCommitService_iBGP_NoASPrepend(t *testing.T) {
	sender := &mockUpdateSender{}
	// iBGP session (same AS)
	neg := testContext(65000, 65000, true)
	cs := NewCommitService(sender, neg, true)

	nh := netip.MustParseAddr("10.0.0.1")
	attrs := []attribute.Attribute{attribute.Origin(0)}

	routes := []*Route{
		NewRoute(newIPv4NLRI("192.168.1.0/24"), nh, attrs),
	}

	_, err := cs.Commit(routes, CommitOptions{})
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	if len(sender.updates) != 1 {
		t.Fatalf("expected 1 update, got %d", len(sender.updates))
	}

	update := sender.updates[0]

	// Find AS_PATH and verify it's empty (no segments)
	offset := 0
	for offset < len(update.PathAttributes) {
		if offset+2 > len(update.PathAttributes) {
			break
		}
		code := update.PathAttributes[offset+1]
		flags := update.PathAttributes[offset]
		var attrLen int
		var hdrLen int
		if flags&0x10 != 0 {
			attrLen = int(update.PathAttributes[offset+2])<<8 | int(update.PathAttributes[offset+3])
			hdrLen = 4
		} else {
			attrLen = int(update.PathAttributes[offset+2])
			hdrLen = 3
		}

		if code == 2 { // AS_PATH
			// iBGP should have empty AS_PATH (length 0)
			if attrLen != 0 {
				t.Errorf("expected empty AS_PATH for iBGP, got length %d", attrLen)
			}
		}
		offset += hdrLen + attrLen
	}
}

// TestCommitService_EVPN_UsesMPReachNLRI verifies EVPN routes use MP_REACH_NLRI.
//
// VALIDATES: L2VPN/EVPN (AFI=25, SAFI=70) routes → MP_REACH_NLRI attribute
//
// PREVENTS: EVPN routes incorrectly placed in UPDATE.NLRI field.
func TestCommitService_EVPN_UsesMPReachNLRI(t *testing.T) {
	sender := &mockUpdateSender{}
	neg := testContext(65000, 65000, true)
	cs := NewCommitService(sender, neg, true)

	nh := netip.MustParseAddr("10.0.0.1")
	attrs := []attribute.Attribute{attribute.Origin(0)}

	// Create EVPN route (AFI=25, SAFI=70)
	routes := []*Route{
		NewRoute(newEVPNNLRI(), nh, attrs),
	}

	_, err := cs.Commit(routes, CommitOptions{})
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	if len(sender.updates) != 1 {
		t.Fatalf("expected 1 update, got %d", len(sender.updates))
	}

	update := sender.updates[0]

	// For EVPN, UPDATE.NLRI MUST be empty
	if len(update.NLRI) != 0 {
		t.Errorf("expected empty UPDATE.NLRI for EVPN, got %d bytes", len(update.NLRI))
	}

	// Verify MP_REACH_NLRI contains AFI=25, SAFI=70
	hasMPReach := false
	offset := 0
	for offset < len(update.PathAttributes) {
		if offset+2 > len(update.PathAttributes) {
			break
		}
		code := update.PathAttributes[offset+1]
		flags := update.PathAttributes[offset]
		var attrLen int
		var hdrLen int
		if flags&0x10 != 0 {
			attrLen = int(update.PathAttributes[offset+2])<<8 | int(update.PathAttributes[offset+3])
			hdrLen = 4
		} else {
			attrLen = int(update.PathAttributes[offset+2])
			hdrLen = 3
		}

		if code == 14 { // MP_REACH_NLRI
			hasMPReach = true
			valueStart := offset + hdrLen
			if valueStart+3 > len(update.PathAttributes) {
				t.Fatal("MP_REACH_NLRI too short")
			}

			// Parse AFI/SAFI
			afi := uint16(update.PathAttributes[valueStart])<<8 | uint16(update.PathAttributes[valueStart+1])
			safi := update.PathAttributes[valueStart+2]

			if afi != 25 {
				t.Errorf("expected AFI 25 (L2VPN), got %d", afi)
			}
			if safi != 70 {
				t.Errorf("expected SAFI 70 (EVPN), got %d", safi)
			}
		}
		offset += hdrLen + attrLen
	}

	if !hasMPReach {
		t.Error("UPDATE missing MP_REACH_NLRI attribute for EVPN")
	}
}

// TestCommitService_iBGP_PreservesASPath verifies iBGP preserves existing AS_PATH.
//
// VALIDATES: Routes with existing AS_PATH → iBGP preserves it unchanged
//
// PREVENTS: Dropping or modifying AS_PATH on iBGP sessions.
func TestCommitService_iBGP_PreservesASPath(t *testing.T) {
	sender := &mockUpdateSender{}
	neg := testContext(65000, 65000, true)
	cs := NewCommitService(sender, neg, true)

	nh := netip.MustParseAddr("10.0.0.1")
	// Route with existing AS_PATH [65001, 65002]
	existingASPath := &attribute.ASPath{
		Segments: []attribute.ASPathSegment{
			{Type: attribute.ASSequence, ASNs: []uint32{65001, 65002}},
		},
	}
	attrs := []attribute.Attribute{attribute.Origin(0), existingASPath}

	routes := []*Route{
		NewRoute(newIPv4NLRI("192.168.1.0/24"), nh, attrs),
	}

	_, err := cs.Commit(routes, CommitOptions{})
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	if len(sender.updates) != 1 {
		t.Fatalf("expected 1 update, got %d", len(sender.updates))
	}

	update := sender.updates[0]

	// Find AS_PATH and verify it contains the original ASNs
	offset := 0
	foundASPath := false
	for offset < len(update.PathAttributes) {
		if offset+2 > len(update.PathAttributes) {
			break
		}
		code := update.PathAttributes[offset+1]
		flags := update.PathAttributes[offset]
		var attrLen int
		var hdrLen int
		if flags&0x10 != 0 {
			attrLen = int(update.PathAttributes[offset+2])<<8 | int(update.PathAttributes[offset+3])
			hdrLen = 4
		} else {
			attrLen = int(update.PathAttributes[offset+2])
			hdrLen = 3
		}

		if code == 2 { // AS_PATH
			foundASPath = true
			// For 4-byte ASNs with 2 ASNs: segment_type(1) + count(1) + 2*ASN(8) = 10 bytes
			if attrLen < 10 {
				t.Errorf("expected AS_PATH with 2 ASNs (>= 10 bytes), got %d bytes", attrLen)
			}
			// Verify first ASN is 65001 (not our local AS)
			if attrLen >= 10 {
				valueStart := offset + hdrLen
				// Skip segment type and count
				firstASN := uint32(update.PathAttributes[valueStart+2])<<24 |
					uint32(update.PathAttributes[valueStart+3])<<16 |
					uint32(update.PathAttributes[valueStart+4])<<8 |
					uint32(update.PathAttributes[valueStart+5])
				if firstASN != 65001 {
					t.Errorf("expected first ASN 65001, got %d", firstASN)
				}
			}
		}
		offset += hdrLen + attrLen
	}

	if !foundASPath {
		t.Error("UPDATE missing AS_PATH attribute")
	}
}

// newEVPNNLRI creates a mock EVPN NLRI for testing.
func newEVPNNLRI() *mockEVPNNLRI {
	return &mockEVPNNLRI{}
}

// mockEVPNNLRI is a minimal EVPN NLRI for testing.
type mockEVPNNLRI struct{}

func (m *mockEVPNNLRI) Family() nlri.Family   { return nlri.L2VPNEVPN }
func (m *mockEVPNNLRI) Bytes() []byte         { return []byte{0x02, 0x21} } // Type 2, length 33
func (m *mockEVPNNLRI) Len() int              { return 2 }
func (m *mockEVPNNLRI) String() string        { return "evpn-type2" }
func (m *mockEVPNNLRI) PathID() uint32        { return 0 }
func (m *mockEVPNNLRI) SupportsAddPath() bool { return true }
func (m *mockEVPNNLRI) WriteTo(buf []byte, off int) int {
	return copy(buf[off:], m.Bytes())
}

// ==============================================================
// Two-Level Grouping Wire Format Tests
// ==============================================================

// TestCommitService_TwoLevel_WireFormat_DiffASPaths verifies different AS_PATHs produce separate UPDATEs.
//
// VALIDATES: Routes with different AS_PATHs → separate UPDATEs with correct AS_PATH bytes
//
// PREVENTS: Routes with different AS_PATHs sharing UPDATE (RFC 4271 violation).
func TestCommitService_TwoLevel_WireFormat_DiffASPaths(t *testing.T) {
	sender := &mockUpdateSender{}
	neg := testContext(65000, 65000, true) // iBGP (no prepend)
	cs := NewCommitService(sender, neg, true)

	nh := netip.MustParseAddr("10.0.0.1")
	attrs := []attribute.Attribute{attribute.Origin(0)}

	// Two routes with same attrs but different AS_PATHs
	asPath1 := &attribute.ASPath{
		Segments: []attribute.ASPathSegment{
			{Type: attribute.ASSequence, ASNs: []uint32{65001}},
		},
	}
	asPath2 := &attribute.ASPath{
		Segments: []attribute.ASPathSegment{
			{Type: attribute.ASSequence, ASNs: []uint32{65001, 65002}},
		},
	}

	routes := []*Route{
		NewRouteWithASPath(newIPv4NLRI("192.168.1.0/24"), nh, attrs, asPath1),
		NewRouteWithASPath(newIPv4NLRI("192.168.2.0/24"), nh, attrs, asPath2),
	}

	_, err := cs.Commit(routes, CommitOptions{})
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	if len(sender.updates) != 2 {
		t.Fatalf("expected 2 updates (different AS_PATHs), got %d", len(sender.updates))
	}

	// Extract AS_PATH lengths from both updates
	asPathLens := make([]int, 0, len(sender.updates))
	for _, update := range sender.updates {
		asPathLen := extractASPathLength(update.PathAttributes)
		asPathLens = append(asPathLens, asPathLen)
	}

	// One should have 1 ASN (6 bytes: type+count+4byte), other should have 2 ASNs (10 bytes)
	// Sort to check: {6, 10}
	if len(asPathLens) != 2 {
		t.Fatalf("expected 2 AS_PATH lengths")
	}

	// Check that we have two different AS_PATH lengths
	if asPathLens[0] == asPathLens[1] {
		t.Errorf("expected different AS_PATH lengths, got both %d", asPathLens[0])
	}
}

// TestCommitService_TwoLevel_WireFormat_SameASPath verifies same AS_PATH routes grouped.
//
// VALIDATES: Routes with same AS_PATH → single UPDATE with multiple NLRIs
//
// PREVENTS: Unnecessary UPDATE fragmentation.
func TestCommitService_TwoLevel_WireFormat_SameASPath(t *testing.T) {
	sender := &mockUpdateSender{}
	neg := testContext(65000, 65000, true)
	cs := NewCommitService(sender, neg, true)

	nh := netip.MustParseAddr("10.0.0.1")
	attrs := []attribute.Attribute{attribute.Origin(0)}

	asPath := &attribute.ASPath{
		Segments: []attribute.ASPathSegment{
			{Type: attribute.ASSequence, ASNs: []uint32{65001}},
		},
	}

	routes := []*Route{
		NewRouteWithASPath(newIPv4NLRI("192.168.1.0/24"), nh, attrs, asPath),
		NewRouteWithASPath(newIPv4NLRI("192.168.2.0/24"), nh, attrs, asPath),
	}

	_, err := cs.Commit(routes, CommitOptions{})
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	if len(sender.updates) != 1 {
		t.Fatalf("expected 1 update (same AS_PATH), got %d", len(sender.updates))
	}

	// Verify UPDATE has 2 NLRIs (two /24 prefixes = 2 * 4 bytes)
	update := sender.updates[0]
	expectedNLRILen := 8 // 2 prefixes * (1 byte prefix len + 3 bytes for /24)
	if len(update.NLRI) != expectedNLRILen {
		t.Errorf("expected NLRI length %d, got %d", expectedNLRILen, len(update.NLRI))
	}
}

// TestCommitService_TwoLevel_WireFormat_eBGP_Prepends verifies eBGP prepends local AS.
//
// VALIDATES: eBGP with explicit AS_PATH → local AS prepended in wire format
//
// PREVENTS: Missing local AS in eBGP announcements.
func TestCommitService_TwoLevel_WireFormat_eBGP_Prepends(t *testing.T) {
	sender := &mockUpdateSender{}
	neg := testContext(65000, 65001, true) // eBGP
	cs := NewCommitService(sender, neg, true)

	nh := netip.MustParseAddr("10.0.0.1")
	attrs := []attribute.Attribute{attribute.Origin(0)}

	asPath := &attribute.ASPath{
		Segments: []attribute.ASPathSegment{
			{Type: attribute.ASSequence, ASNs: []uint32{65002}},
		},
	}

	routes := []*Route{
		NewRouteWithASPath(newIPv4NLRI("192.168.1.0/24"), nh, attrs, asPath),
	}

	_, err := cs.Commit(routes, CommitOptions{})
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	if len(sender.updates) != 1 {
		t.Fatalf("expected 1 update, got %d", len(sender.updates))
	}

	// Extract first ASN from AS_PATH
	firstASN := extractFirstASN(sender.updates[0].PathAttributes)
	if firstASN != 65000 {
		t.Errorf("expected first ASN 65000 (local AS), got %d", firstASN)
	}

	// Verify we have 2 ASNs total (65000, 65002)
	asPathLen := extractASPathLength(sender.updates[0].PathAttributes)
	expectedLen := 10 // type(1) + count(1) + 2*ASN(8) = 10 bytes
	if asPathLen != expectedLen {
		t.Errorf("expected AS_PATH length %d (2 ASNs), got %d", expectedLen, asPathLen)
	}
}

// extractASPathLength returns the length of the AS_PATH attribute value.
func extractASPathLength(attrs []byte) int {
	offset := 0
	for offset < len(attrs) {
		if offset+2 > len(attrs) {
			break
		}
		flags := attrs[offset]
		code := attrs[offset+1]
		var attrLen, hdrLen int
		if flags&0x10 != 0 {
			if offset+4 > len(attrs) {
				break
			}
			attrLen = int(attrs[offset+2])<<8 | int(attrs[offset+3])
			hdrLen = 4
		} else {
			if offset+3 > len(attrs) {
				break
			}
			attrLen = int(attrs[offset+2])
			hdrLen = 3
		}

		if code == 2 { // AS_PATH
			return attrLen
		}
		offset += hdrLen + attrLen
	}
	return 0
}

// extractFirstASN returns the first ASN in the AS_PATH.
func extractFirstASN(attrs []byte) uint32 {
	offset := 0
	for offset < len(attrs) {
		if offset+2 > len(attrs) {
			break
		}
		flags := attrs[offset]
		code := attrs[offset+1]
		var attrLen, hdrLen int
		if flags&0x10 != 0 {
			if offset+4 > len(attrs) {
				break
			}
			attrLen = int(attrs[offset+2])<<8 | int(attrs[offset+3])
			hdrLen = 4
		} else {
			if offset+3 > len(attrs) {
				break
			}
			attrLen = int(attrs[offset+2])
			hdrLen = 3
		}

		if code == 2 { // AS_PATH
			valueStart := offset + hdrLen
			// Skip segment type (1) and count (1) to get first ASN
			if valueStart+6 <= len(attrs) {
				return uint32(attrs[valueStart+2])<<24 |
					uint32(attrs[valueStart+3])<<16 |
					uint32(attrs[valueStart+4])<<8 |
					uint32(attrs[valueStart+5])
			}
		}
		offset += hdrLen + attrLen
	}
	return 0
}

// TestCommitService_TwoLevel_WireFormat_eBGP_ASSetFirst verifies eBGP creates new AS_SEQUENCE
// when first segment is AS_SET.
//
// VALIDATES: eBGP with AS_SET first segment → new AS_SEQUENCE inserted
//
// PREVENTS: Incorrect AS_PATH when first segment is not AS_SEQUENCE.
func TestCommitService_TwoLevel_WireFormat_eBGP_ASSetFirst(t *testing.T) {
	sender := &mockUpdateSender{}
	neg := testContext(65000, 65001, true) // eBGP
	cs := NewCommitService(sender, neg, true)

	nh := netip.MustParseAddr("10.0.0.1")
	attrs := []attribute.Attribute{attribute.Origin(0)}

	// AS_PATH with AS_SET as first segment
	asPath := &attribute.ASPath{
		Segments: []attribute.ASPathSegment{
			{Type: attribute.ASSet, ASNs: []uint32{65002, 65003}}, // AS_SET, not AS_SEQUENCE
		},
	}

	routes := []*Route{
		NewRouteWithASPath(newIPv4NLRI("192.168.1.0/24"), nh, attrs, asPath),
	}

	_, err := cs.Commit(routes, CommitOptions{})
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	if len(sender.updates) != 1 {
		t.Fatalf("expected 1 update, got %d", len(sender.updates))
	}

	// Extract first segment type from AS_PATH
	segType, segCount := extractFirstSegment(sender.updates[0].PathAttributes)

	// First segment should be AS_SEQUENCE (type 2) with just local AS
	if segType != byte(attribute.ASSequence) {
		t.Errorf("expected first segment AS_SEQUENCE (2), got %d", segType)
	}
	if segCount != 1 {
		t.Errorf("expected first segment with 1 ASN (local AS), got %d", segCount)
	}

	// Verify first ASN is local AS
	firstASN := extractFirstASN(sender.updates[0].PathAttributes)
	if firstASN != 65000 {
		t.Errorf("expected first ASN 65000 (local AS), got %d", firstASN)
	}
}

// extractFirstSegment returns the type and count of the first AS_PATH segment.
func extractFirstSegment(attrs []byte) (segType, segCount byte) {
	offset := 0
	for offset < len(attrs) {
		if offset+2 > len(attrs) {
			break
		}
		flags := attrs[offset]
		code := attrs[offset+1]
		var attrLen, hdrLen int
		if flags&0x10 != 0 {
			if offset+4 > len(attrs) {
				break
			}
			attrLen = int(attrs[offset+2])<<8 | int(attrs[offset+3])
			hdrLen = 4
		} else {
			if offset+3 > len(attrs) {
				break
			}
			attrLen = int(attrs[offset+2])
			hdrLen = 3
		}

		if code == 2 { // AS_PATH
			valueStart := offset + hdrLen
			if valueStart+2 <= len(attrs) {
				return attrs[valueStart], attrs[valueStart+1]
			}
		}
		offset += hdrLen + attrLen
	}
	return 0, 0
}
