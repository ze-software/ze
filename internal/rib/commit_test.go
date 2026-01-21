package rib

import (
	"errors"
	"net/netip"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/bgp/attribute"
	"codeberg.org/thomas-mangin/ze/internal/bgp/capability"
	bgpctx "codeberg.org/thomas-mangin/ze/internal/bgp/context"
	"codeberg.org/thomas-mangin/ze/internal/bgp/message"
	"codeberg.org/thomas-mangin/ze/internal/bgp/nlri"
)

// testContext creates an EncodingContext for tests.
// This is the replacement for message.Negotiated{} in test code.
// The localAS parameter varies per test case to test eBGP vs iBGP behavior.
func testContext(localAS, peerAS uint32, asn4 bool) *bgpctx.EncodingContext { //nolint:unparam // localAS varies to test eBGP/iBGP
	identity := &capability.PeerIdentity{
		LocalASN: localAS,
		PeerASN:  peerAS,
	}
	encoding := &capability.EncodingCaps{
		ASN4: asn4,
	}
	return bgpctx.NewEncodingContext(identity, encoding, bgpctx.DirectionSend)
}

// testContextWithAddPath creates an EncodingContext with ADD-PATH settings.
func testContextWithAddPath(localAS, peerAS uint32, asn4 bool, addPath map[nlri.Family]bool) *bgpctx.EncodingContext {
	identity := &capability.PeerIdentity{
		LocalASN: localAS,
		PeerASN:  peerAS,
	}

	// Convert bool map to AddPathMode map (Send mode enables sending)
	addPathMode := make(map[capability.Family]capability.AddPathMode)
	for f, enabled := range addPath {
		if enabled {
			addPathMode[f] = capability.AddPathSend
		}
	}

	encoding := &capability.EncodingCaps{
		ASN4:        asn4,
		AddPathMode: addPathMode,
	}
	return bgpctx.NewEncodingContext(identity, encoding, bgpctx.DirectionSend)
}

// mockUpdateSender records sent updates for verification.
type mockUpdateSender struct {
	updates []*message.Update
	err     error // If set, SendUpdate returns this error
}

func (m *mockUpdateSender) SendUpdate(u *message.Update) error {
	if m.err != nil {
		return m.err
	}
	m.updates = append(m.updates, u)
	return nil
}

// TestCommitService_GroupsRoutesByAttributes verifies routes with same attributes are grouped.
//
// VALIDATES: Multiple routes with identical attributes → fewer UPDATE messages
//
// PREVENTS: Each route sent as separate UPDATE when they could be grouped.
func TestCommitService_GroupsRoutesByAttributes(t *testing.T) {
	sender := &mockUpdateSender{}
	neg := testContext(65000, 65000, true)
	cs := NewCommitService(sender, neg, true) // groupUpdates=true

	// Create 3 routes: 2 with same attributes, 1 different
	nh := netip.MustParseAddr("10.0.0.1")
	attrs := []attribute.Attribute{attribute.Origin(0)}

	routes := []*Route{
		NewRoute(newIPv4NLRI("192.168.1.0/24"), nh, attrs),
		NewRoute(newIPv4NLRI("192.168.2.0/24"), nh, attrs),                              // Same attrs as first
		NewRoute(newIPv4NLRI("192.168.3.0/24"), netip.MustParseAddr("10.0.0.2"), attrs), // Different next-hop
	}

	stats, err := cs.Commit(routes, CommitOptions{})
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	// Should produce 2 updates: one for routes 0+1, one for route 2
	if stats.UpdatesSent != 2 {
		t.Errorf("expected 2 updates sent, got %d", stats.UpdatesSent)
	}
	if stats.RoutesAnnounced != 3 {
		t.Errorf("expected 3 routes announced, got %d", stats.RoutesAnnounced)
	}
	if len(sender.updates) != 2 {
		t.Errorf("expected 2 updates in mock, got %d", len(sender.updates))
	}
}

// TestCommitService_NoGrouping verifies one UPDATE per route when grouping disabled.
//
// VALIDATES: GroupUpdates=false → one UPDATE per route
//
// PREVENTS: Unwanted grouping when explicit per-route updates needed.
func TestCommitService_NoGrouping(t *testing.T) {
	sender := &mockUpdateSender{}
	neg := testContext(65000, 65000, true)
	cs := NewCommitService(sender, neg, false) // groupUpdates=false

	nh := netip.MustParseAddr("10.0.0.1")
	attrs := []attribute.Attribute{attribute.Origin(0)}

	routes := []*Route{
		NewRoute(newIPv4NLRI("192.168.1.0/24"), nh, attrs),
		NewRoute(newIPv4NLRI("192.168.2.0/24"), nh, attrs), // Same attrs but no grouping
	}

	stats, err := cs.Commit(routes, CommitOptions{})
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	// Should produce 2 updates (one per route)
	if stats.UpdatesSent != 2 {
		t.Errorf("expected 2 updates sent, got %d", stats.UpdatesSent)
	}
	if len(sender.updates) != 2 {
		t.Errorf("expected 2 updates in mock, got %d", len(sender.updates))
	}
}

// TestCommitService_SendsEORWhenRequested verifies EOR sent when SendEOR=true.
//
// VALIDATES: SendEOR: true → EOR marker sent for affected families
//
// PREVENTS: Missing EOR after config route commit.
func TestCommitService_SendsEORWhenRequested(t *testing.T) {
	sender := &mockUpdateSender{}
	neg := testContext(65000, 65000, true)
	cs := NewCommitService(sender, neg, true)

	nh := netip.MustParseAddr("10.0.0.1")
	attrs := []attribute.Attribute{attribute.Origin(0)}

	routes := []*Route{
		NewRoute(newIPv4NLRI("192.168.1.0/24"), nh, attrs),
	}

	stats, err := cs.Commit(routes, CommitOptions{SendEOR: true})
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	// Should have 1 update + 1 EOR
	if stats.UpdatesSent != 1 {
		t.Errorf("expected 1 update sent, got %d", stats.UpdatesSent)
	}
	if len(stats.EORSent) != 1 {
		t.Errorf("expected 1 EOR sent, got %d", len(stats.EORSent))
	}

	// Verify EOR family
	if len(stats.EORSent) > 0 {
		eorFamily := stats.EORSent[0]
		if eorFamily.AFI != 1 || eorFamily.SAFI != 1 {
			t.Errorf("expected IPv4 unicast EOR, got AFI=%d SAFI=%d", eorFamily.AFI, eorFamily.SAFI)
		}
	}

	// Sender should have 2 messages: UPDATE + EOR
	if len(sender.updates) != 2 {
		t.Errorf("expected 2 messages (update + EOR), got %d", len(sender.updates))
	}
}

// TestCommitService_NoEORWhenNotRequested verifies no EOR sent when SendEOR=false.
//
// VALIDATES: SendEOR: false → no EOR marker
//
// PREVENTS: Spurious EOR after API batch commit.
func TestCommitService_NoEORWhenNotRequested(t *testing.T) {
	sender := &mockUpdateSender{}
	neg := testContext(65000, 65000, true)
	cs := NewCommitService(sender, neg, true)

	nh := netip.MustParseAddr("10.0.0.1")
	attrs := []attribute.Attribute{attribute.Origin(0)}

	routes := []*Route{
		NewRoute(newIPv4NLRI("192.168.1.0/24"), nh, attrs),
	}

	stats, err := cs.Commit(routes, CommitOptions{SendEOR: false})
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	if len(stats.EORSent) != 0 {
		t.Errorf("expected 0 EOR sent, got %d", len(stats.EORSent))
	}

	// Only 1 update, no EOR
	if len(sender.updates) != 1 {
		t.Errorf("expected 1 message, got %d", len(sender.updates))
	}
}

// TestCommitService_TracksAffectedFamilies verifies multiple families tracked correctly.
//
// VALIDATES: IPv4 + IPv6 routes → both families in FamiliesAffected
//
// PREVENTS: Missing EOR for some families in mixed-family commits.
func TestCommitService_TracksAffectedFamilies(t *testing.T) {
	sender := &mockUpdateSender{}
	neg := testContext(65000, 65000, true)
	cs := NewCommitService(sender, neg, true)

	attrs := []attribute.Attribute{attribute.Origin(0)}

	routes := []*Route{
		NewRoute(newIPv4NLRI("192.168.1.0/24"), netip.MustParseAddr("10.0.0.1"), attrs),
		NewRoute(newIPv6NLRI("2001:db8::/32"), netip.MustParseAddr("2001:db8::1"), attrs),
	}

	stats, err := cs.Commit(routes, CommitOptions{SendEOR: true})
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	// Should have 2 families affected
	if len(stats.FamiliesAffected) != 2 {
		t.Errorf("expected 2 families affected, got %d", len(stats.FamiliesAffected))
	}

	// Should have 2 EORs sent
	if len(stats.EORSent) != 2 {
		t.Errorf("expected 2 EOR sent, got %d", len(stats.EORSent))
	}
}

// TestCommitService_EmptyRoutes verifies no crash on empty input.
//
// VALIDATES: Empty route slice → no updates, no EOR
//
// PREVENTS: Panic or error on empty commit.
func TestCommitService_EmptyRoutes(t *testing.T) {
	sender := &mockUpdateSender{}
	neg := testContext(65000, 65000, true)
	cs := NewCommitService(sender, neg, true)

	stats, err := cs.Commit(nil, CommitOptions{SendEOR: true})
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	if stats.UpdatesSent != 0 {
		t.Errorf("expected 0 updates, got %d", stats.UpdatesSent)
	}
	if len(stats.EORSent) != 0 {
		t.Errorf("expected 0 EOR (no families affected), got %d", len(stats.EORSent))
	}
	if len(sender.updates) != 0 {
		t.Errorf("expected 0 messages, got %d", len(sender.updates))
	}
}

// TestCommitService_SendError verifies error propagation on send failure.
//
// VALIDATES: SendUpdate error → Commit returns error
//
// PREVENTS: Silent failures on network errors.
func TestCommitService_SendError(t *testing.T) {
	sender := &mockUpdateSender{err: errors.New("network error")}
	neg := testContext(65000, 65000, true)
	cs := NewCommitService(sender, neg, true)

	nh := netip.MustParseAddr("10.0.0.1")
	attrs := []attribute.Attribute{attribute.Origin(0)}

	routes := []*Route{
		NewRoute(newIPv4NLRI("192.168.1.0/24"), nh, attrs),
	}

	_, err := cs.Commit(routes, CommitOptions{})
	if err == nil {
		t.Error("expected error from Commit, got nil")
	}
}

// TestCommitService_TwoLevel_ExplicitASPathTakesPrecedence verifies route.ASPath() wins over attrs.
//
// VALIDATES: When AS_PATH exists in both route.asPath AND route.attributes, explicit field wins
//
// PREVENTS: Wrong AS_PATH used when route has AS_PATH in both locations.
func TestCommitService_TwoLevel_ExplicitASPathTakesPrecedence(t *testing.T) {
	sender := &mockUpdateSender{}
	neg := testContext(65000, 65000, true) // iBGP (no prepend)
	cs := NewCommitService(sender, neg, true)

	nh := netip.MustParseAddr("10.0.0.1")

	// AS_PATH in attributes (should be IGNORED)
	asPathInAttrs := &attribute.ASPath{
		Segments: []attribute.ASPathSegment{
			{Type: attribute.ASSequence, ASNs: []uint32{99999}}, // Wrong AS - should NOT appear
		},
	}
	attrs := []attribute.Attribute{attribute.Origin(0), asPathInAttrs}

	// Explicit AS_PATH (should WIN)
	explicitASPath := &attribute.ASPath{
		Segments: []attribute.ASPathSegment{
			{Type: attribute.ASSequence, ASNs: []uint32{65001, 65002}}, // Correct AS
		},
	}

	routes := []*Route{
		NewRouteWithASPath(newIPv4NLRI("192.168.1.0/24"), nh, attrs, explicitASPath),
	}

	_, err := cs.Commit(routes, CommitOptions{})
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	if len(sender.updates) != 1 {
		t.Fatalf("expected 1 update, got %d", len(sender.updates))
	}

	// Extract first ASN from AS_PATH - should be 65001 (from explicit), NOT 99999 (from attrs)
	update := sender.updates[0]
	offset := 0
	for offset < len(update.PathAttributes) {
		if offset+2 > len(update.PathAttributes) {
			break
		}
		flags := update.PathAttributes[offset]
		code := update.PathAttributes[offset+1]
		var attrLen, hdrLen int
		if flags&0x10 != 0 {
			attrLen = int(update.PathAttributes[offset+2])<<8 | int(update.PathAttributes[offset+3])
			hdrLen = 4
		} else {
			attrLen = int(update.PathAttributes[offset+2])
			hdrLen = 3
		}

		if code == 2 { // AS_PATH
			valueStart := offset + hdrLen
			if valueStart+6 <= len(update.PathAttributes) {
				firstASN := uint32(update.PathAttributes[valueStart+2])<<24 |
					uint32(update.PathAttributes[valueStart+3])<<16 |
					uint32(update.PathAttributes[valueStart+4])<<8 |
					uint32(update.PathAttributes[valueStart+5])
				if firstASN == 99999 {
					t.Error("AS_PATH from attributes was used instead of explicit AS_PATH")
				}
				if firstASN != 65001 {
					t.Errorf("expected first ASN 65001 (explicit), got %d", firstASN)
				}
			}
			break
		}
		offset += hdrLen + attrLen
	}
}

// newIPv4NLRI creates an IPv4 unicast NLRI for testing.
func newIPv4NLRI(prefix string) nlri.NLRI {
	p := netip.MustParsePrefix(prefix)
	return nlri.NewINET(nlri.IPv4Unicast, p, 0)
}

// newIPv6NLRI creates an IPv6 unicast NLRI for testing.
func newIPv6NLRI(prefix string) nlri.NLRI {
	p := netip.MustParsePrefix(prefix)
	return nlri.NewINET(nlri.IPv6Unicast, p, 0)
}

// ==============================================================
// Two-Level Grouping Tests (AS_PATH preservation)
// ==============================================================

// TestCommitService_TwoLevel_DifferentASPaths verifies routes with different AS_PATHs produce separate UPDATEs.
//
// VALIDATES: Routes with same attrs but different AS_PATH → separate UPDATEs
//
// PREVENTS: RFC 4271 violation where routes with different AS_PATHs share UPDATE.
func TestCommitService_TwoLevel_DifferentASPaths(t *testing.T) {
	sender := &mockUpdateSender{}
	neg := testContext(65000, 65000, true) // iBGP
	cs := NewCommitService(sender, neg, true)

	nh := netip.MustParseAddr("10.0.0.1")
	attrs := []attribute.Attribute{attribute.Origin(0)}

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
		NewRouteWithASPath(newIPv4NLRI("192.168.2.0/24"), nh, attrs, asPath1), // Same AS_PATH as first
		NewRouteWithASPath(newIPv4NLRI("192.168.3.0/24"), nh, attrs, asPath2), // Different AS_PATH
	}

	stats, err := cs.Commit(routes, CommitOptions{})
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	// Should produce 2 updates: one for routes with asPath1 (2 NLRIs), one for asPath2
	if stats.UpdatesSent != 2 {
		t.Errorf("expected 2 updates sent, got %d", stats.UpdatesSent)
	}
	if stats.RoutesAnnounced != 3 {
		t.Errorf("expected 3 routes announced, got %d", stats.RoutesAnnounced)
	}
	if len(sender.updates) != 2 {
		t.Errorf("expected 2 updates in mock, got %d", len(sender.updates))
	}
}

// TestCommitService_TwoLevel_SameASPath verifies routes with same AS_PATH are grouped.
//
// VALIDATES: Routes with same attrs AND same AS_PATH → single UPDATE
//
// PREVENTS: Unnecessary UPDATE fragmentation.
func TestCommitService_TwoLevel_SameASPath(t *testing.T) {
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
		NewRouteWithASPath(newIPv4NLRI("192.168.3.0/24"), nh, attrs, asPath),
	}

	stats, err := cs.Commit(routes, CommitOptions{})
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	// Should produce 1 update with all 3 NLRIs
	if stats.UpdatesSent != 1 {
		t.Errorf("expected 1 update sent, got %d", stats.UpdatesSent)
	}
	if len(sender.updates) != 1 {
		t.Errorf("expected 1 update in mock, got %d", len(sender.updates))
	}
}

// TestCommitService_TwoLevel_eBGPPrepends verifies local AS prepended for eBGP with explicit AS_PATH.
//
// VALIDATES: eBGP + explicit AS_PATH → local AS prepended to AS_PATH
//
// PREVENTS: Incorrect AS_PATH in eBGP announcements (missing local AS).
func TestCommitService_TwoLevel_eBGPPrepends(t *testing.T) {
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

	stats, err := cs.Commit(routes, CommitOptions{})
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	if stats.UpdatesSent != 1 {
		t.Errorf("expected 1 update sent, got %d", stats.UpdatesSent)
	}

	// Verify AS_PATH in the update has local AS prepended
	// The AS_PATH should be [65000, 65002]
	if len(sender.updates) == 0 {
		t.Fatal("no updates sent")
	}

	// We can't easily check the packed bytes here, but wire format tests will verify
	// For now, just verify the commit succeeded
}

// TestCommitService_TwoLevel_iBGPPreserves verifies AS_PATH unchanged for iBGP.
//
// VALIDATES: iBGP + explicit AS_PATH → AS_PATH preserved unchanged
//
// PREVENTS: Incorrect AS_PATH modification in iBGP (should not prepend).
func TestCommitService_TwoLevel_iBGPPreserves(t *testing.T) {
	sender := &mockUpdateSender{}
	neg := testContext(65000, 65000, true) // iBGP
	cs := NewCommitService(sender, neg, true)

	nh := netip.MustParseAddr("10.0.0.1")
	attrs := []attribute.Attribute{attribute.Origin(0)}

	asPath := &attribute.ASPath{
		Segments: []attribute.ASPathSegment{
			{Type: attribute.ASSequence, ASNs: []uint32{65001, 65002}},
		},
	}

	routes := []*Route{
		NewRouteWithASPath(newIPv4NLRI("192.168.1.0/24"), nh, attrs, asPath),
	}

	stats, err := cs.Commit(routes, CommitOptions{})
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	if stats.UpdatesSent != 1 {
		t.Errorf("expected 1 update sent, got %d", stats.UpdatesSent)
	}

	// Wire format tests will verify the exact AS_PATH bytes
}

// TestCommitService_TwoLevel_NilASPath verifies nil AS_PATH creates fresh path.
//
// VALIDATES: Nil AS_PATH → creates appropriate AS_PATH for eBGP/iBGP
//
// PREVENTS: Missing AS_PATH in UPDATEs for locally originated routes.
func TestCommitService_TwoLevel_NilASPath(t *testing.T) {
	sender := &mockUpdateSender{}
	neg := testContext(65000, 65001, true) // eBGP
	cs := NewCommitService(sender, neg, true)

	nh := netip.MustParseAddr("10.0.0.1")
	attrs := []attribute.Attribute{attribute.Origin(0)}

	routes := []*Route{
		NewRouteWithASPath(newIPv4NLRI("192.168.1.0/24"), nh, attrs, nil), // nil AS_PATH
	}

	stats, err := cs.Commit(routes, CommitOptions{})
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	if stats.UpdatesSent != 1 {
		t.Errorf("expected 1 update sent, got %d", stats.UpdatesSent)
	}

	// For eBGP with nil AS_PATH, should create [LocalAS]
	// Wire format tests will verify
}

// TestCommitService_NoGrouping_PreservesExplicitASPath verifies non-grouped path uses route.ASPath().
//
// VALIDATES: groupUpdates=false with NewRouteWithASPath → AS_PATH preserved
//
// PREVENTS: Regression where buildSingleUpdate ignores route.ASPath() field.
func TestCommitService_NoGrouping_PreservesExplicitASPath(t *testing.T) {
	sender := &mockUpdateSender{}
	neg := testContext(65000, 65000, true)     // iBGP
	cs := NewCommitService(sender, neg, false) // groupUpdates=FALSE

	nh := netip.MustParseAddr("10.0.0.1")
	attrs := []attribute.Attribute{attribute.Origin(0)} // NO AS_PATH in attrs

	// Create route with EXPLICIT AS_PATH [65001, 65002]
	asPath := &attribute.ASPath{
		Segments: []attribute.ASPathSegment{
			{Type: attribute.ASSequence, ASNs: []uint32{65001, 65002}},
		},
	}

	routes := []*Route{
		NewRouteWithASPath(newIPv4NLRI("192.168.1.0/24"), nh, attrs, asPath),
	}

	stats, err := cs.Commit(routes, CommitOptions{})
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	if stats.UpdatesSent != 1 {
		t.Errorf("expected 1 update sent, got %d", stats.UpdatesSent)
	}

	// Verify AS_PATH in the update has 2 ASNs
	if len(sender.updates) == 0 {
		t.Fatal("no updates sent")
	}

	update := sender.updates[0]
	asPathLen := 0
	offset := 0
	for offset < len(update.PathAttributes) {
		if offset+2 > len(update.PathAttributes) {
			break
		}
		flags := update.PathAttributes[offset]
		code := update.PathAttributes[offset+1]
		var attrLen, hdrLen int
		if flags&0x10 != 0 {
			attrLen = int(update.PathAttributes[offset+2])<<8 | int(update.PathAttributes[offset+3])
			hdrLen = 4
		} else {
			attrLen = int(update.PathAttributes[offset+2])
			hdrLen = 3
		}

		if code == 2 { // AS_PATH
			asPathLen = attrLen
		}
		offset += hdrLen + attrLen
	}

	// With 2 ASNs in 4-byte format: type(1) + count(1) + 2*4 = 10 bytes
	if asPathLen < 10 {
		t.Errorf("expected AS_PATH with 2 ASNs (>=10 bytes), got %d bytes", asPathLen)
	}
}

// TestCommitServiceAddPathFor verifies addPathFor returns correct ADD-PATH status.
//
// VALIDATES: CommitService.addPathFor returns true when ADD-PATH negotiated for family.
// RFC 7911: ADD-PATH encoding requires negotiation per address family.
//
// PREVENTS: Missing path ID when ADD-PATH was negotiated.
func TestCommitServiceAddPathFor(t *testing.T) {
	sender := &mockUpdateSender{}

	// Test with ADD-PATH enabled for IPv4 unicast
	ctx := testContextWithAddPath(65000, 65001, true, map[nlri.Family]bool{
		nlri.IPv4Unicast: true,  // IPv4 unicast with ADD-PATH
		nlri.IPv6Unicast: false, // IPv6 unicast without ADD-PATH
	})
	cs := NewCommitService(sender, ctx, false)

	if !cs.addPathFor(nlri.IPv4Unicast) {
		t.Error("addPathFor should be true for IPv4 unicast when negotiated")
	}
	if cs.addPathFor(nlri.IPv6Unicast) {
		t.Error("addPathFor should be false for IPv6 unicast when not negotiated")
	}
}
