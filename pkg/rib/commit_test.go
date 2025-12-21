package rib

import (
	"errors"
	"net/netip"
	"testing"

	"github.com/exa-networks/zebgp/pkg/bgp/attribute"
	"github.com/exa-networks/zebgp/pkg/bgp/message"
	"github.com/exa-networks/zebgp/pkg/bgp/nlri"
)

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
	neg := &message.Negotiated{ASN4: true, LocalAS: 65000, PeerAS: 65000}
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
	neg := &message.Negotiated{ASN4: true, LocalAS: 65000, PeerAS: 65000}
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
	neg := &message.Negotiated{ASN4: true, LocalAS: 65000, PeerAS: 65000}
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
	neg := &message.Negotiated{ASN4: true, LocalAS: 65000, PeerAS: 65000}
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
	neg := &message.Negotiated{ASN4: true, LocalAS: 65000, PeerAS: 65000}
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
	neg := &message.Negotiated{ASN4: true, LocalAS: 65000, PeerAS: 65000}
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
	neg := &message.Negotiated{ASN4: true, LocalAS: 65000, PeerAS: 65000}
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
