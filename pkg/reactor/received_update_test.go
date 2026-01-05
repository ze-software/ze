package reactor

import (
	"encoding/binary"
	"net/netip"
	"testing"
	"time"

	"github.com/exa-networks/zebgp/pkg/api"
	bgpctx "github.com/exa-networks/zebgp/pkg/bgp/context"
	"github.com/exa-networks/zebgp/pkg/bgp/nlri"
)

// buildUpdatePayload builds an UPDATE message body from components.
// Format: WithdrawnLen(2) + Withdrawn + AttrLen(2) + Attrs + NLRI.
func buildUpdatePayload(attrs, nlriBytes []byte) []byte {
	attrLen := len(attrs)
	payload := make([]byte, 2+0+2+attrLen+len(nlriBytes))

	binary.BigEndian.PutUint16(payload[0:2], 0)               // No withdrawals in tests
	binary.BigEndian.PutUint16(payload[2:4], uint16(attrLen)) //nolint:gosec // G115: test data
	copy(payload[4:], attrs)
	copy(payload[4+attrLen:], nlriBytes)

	return payload
}

// TestReceivedUpdateFields verifies ReceivedUpdate stores all fields correctly.
//
// VALIDATES: All fields are accessible and correctly stored.
// PREVENTS: Missing or incorrect field storage.
func TestReceivedUpdateFields(t *testing.T) {
	now := time.Now()
	sourcePeer := netip.MustParseAddr("10.0.0.1")
	ctxID := bgpctx.ContextID(1)

	// Create simple NLRI for testing
	announceNLRI := nlri.NewINET(nlri.IPv4Unicast, netip.MustParsePrefix("192.168.1.0/24"), 0)
	withdrawNLRI := nlri.NewINET(nlri.IPv4Unicast, netip.MustParsePrefix("172.16.0.0/16"), 0)

	// Build UPDATE payload
	attrBytes := []byte{0x40, 0x01, 0x01, 0x00} // ORIGIN IGP
	payload := buildUpdatePayload(attrBytes, nil)
	wireUpdate := api.NewWireUpdate(payload, ctxID)

	update := &ReceivedUpdate{
		UpdateID:     12345,
		WireUpdate:   wireUpdate,
		Announces:    []nlri.NLRI{announceNLRI},
		Withdraws:    []nlri.NLRI{withdrawNLRI},
		AnnounceWire: [][]byte{announceNLRI.Bytes()},
		WithdrawWire: [][]byte{withdrawNLRI.Bytes()},
		SourcePeerIP: sourcePeer,
		ReceivedAt:   now,
	}

	if update.UpdateID != 12345 {
		t.Errorf("UpdateID = %d, want 12345", update.UpdateID)
	}
	if update.WireUpdate.Attrs() == nil {
		t.Error("WireUpdate.Attrs() should not be nil")
	}
	if len(update.Announces) != 1 {
		t.Errorf("Announces len = %d, want 1", len(update.Announces))
	}
	if len(update.Withdraws) != 1 {
		t.Errorf("Withdraws len = %d, want 1", len(update.Withdraws))
	}
	if update.SourcePeerIP != sourcePeer {
		t.Errorf("SourcePeerIP = %v, want %v", update.SourcePeerIP, sourcePeer)
	}
	if update.WireUpdate.SourceCtxID() != ctxID {
		t.Errorf("SourceCtxID = %d, want %d", update.WireUpdate.SourceCtxID(), ctxID)
	}
	if !update.ReceivedAt.Equal(now) {
		t.Errorf("ReceivedAt = %v, want %v", update.ReceivedAt, now)
	}
}

// TestReceivedUpdateWithdrawOnly verifies withdraw-only UPDATEs work correctly.
//
// VALIDATES: Updates can have nil attrs (withdraw-only).
// PREVENTS: Nil pointer panic on withdraw-only UPDATEs.
func TestReceivedUpdateWithdrawOnly(t *testing.T) {
	withdrawNLRI := nlri.NewINET(nlri.IPv4Unicast, netip.MustParsePrefix("10.0.0.0/8"), 0)

	// Withdraw-only: no attributes
	payload := buildUpdatePayload(nil, nil)
	wireUpdate := api.NewWireUpdate(payload, bgpctx.ContextID(1))

	update := &ReceivedUpdate{
		UpdateID:     1,
		WireUpdate:   wireUpdate,
		Announces:    nil,
		Withdraws:    []nlri.NLRI{withdrawNLRI},
		WithdrawWire: [][]byte{withdrawNLRI.Bytes()},
		SourcePeerIP: netip.MustParseAddr("10.0.0.1"),
		ReceivedAt:   time.Now(),
	}

	if update.WireUpdate.Attrs() != nil {
		t.Error("withdraw-only UPDATE should have nil Attrs")
	}
	if len(update.Withdraws) != 1 {
		t.Errorf("Withdraws len = %d, want 1", len(update.Withdraws))
	}
}

// TestMsgIDAssignment verifies unique ID generation.
//
// VALIDATES: Each message gets unique ID.
// PREVENTS: ID collisions causing wrong forwarding.
func TestMsgIDAssignment(t *testing.T) {
	// Reset counter for test
	msgIDCounter.Store(0)

	ids := make(map[uint64]bool)
	for i := 0; i < 1000; i++ {
		id := nextMsgID()
		if ids[id] {
			t.Fatalf("duplicate ID %d at iteration %d", id, i)
		}
		ids[id] = true
	}

	if len(ids) != 1000 {
		t.Errorf("expected 1000 unique IDs, got %d", len(ids))
	}
}

// TestMsgIDMonotonic verifies IDs are monotonically increasing.
//
// VALIDATES: IDs increase sequentially.
// PREVENTS: Out-of-order IDs confusing API consumers.
func TestMsgIDMonotonic(t *testing.T) {
	msgIDCounter.Store(0)

	var prev uint64
	for i := 0; i < 100; i++ {
		id := nextMsgID()
		if id <= prev {
			t.Fatalf("ID %d not greater than previous %d", id, prev)
		}
		prev = id
	}
}

// TestConvertToRoutesIPv4 verifies conversion for IPv4 unicast routes.
//
// VALIDATES: IPv4 routes extracted with correct NextHop from NextHop attribute.
// PREVENTS: Missing or wrong next-hop in adj-rib-out entries.
func TestConvertToRoutesIPv4(t *testing.T) {
	// Register a context for attribute parsing
	ctx := &bgpctx.EncodingContext{ASN4: true}
	ctxID := bgpctx.Registry.Register(ctx)
	nextHopAddr := netip.MustParseAddr("10.0.0.1")

	// Build attributes: ORIGIN + NEXT_HOP + AS_PATH
	attrBytes := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN IGP
		0x40, 0x03, 0x04, 10, 0, 0, 1, // NEXT_HOP 10.0.0.1
		0x40, 0x02, 0x00, // AS_PATH empty
	}

	payload := buildUpdatePayload(attrBytes, nil)
	wireUpdate := api.NewWireUpdate(payload, ctxID)

	announceNLRI := nlri.NewINET(nlri.IPv4Unicast, netip.MustParsePrefix("192.168.1.0/24"), 0)

	update := &ReceivedUpdate{
		UpdateID:     1,
		WireUpdate:   wireUpdate,
		Announces:    []nlri.NLRI{announceNLRI},
		AnnounceWire: [][]byte{announceNLRI.Bytes()},
	}

	routes, err := update.ConvertToRoutes()
	if err != nil {
		t.Fatalf("ConvertToRoutes failed: %v", err)
	}

	if len(routes) != 1 {
		t.Fatalf("expected 1 route, got %d", len(routes))
	}

	route := routes[0]
	if route.NextHop() != nextHopAddr {
		t.Errorf("NextHop = %v, want %v", route.NextHop(), nextHopAddr)
	}
	if route.NLRI().String() != announceNLRI.String() {
		t.Errorf("NLRI = %v, want %v", route.NLRI(), announceNLRI)
	}
}

// TestConvertToRoutesIPv6 verifies conversion for IPv6 routes with MP_REACH_NLRI.
//
// VALIDATES: IPv6 routes extract NextHop from MP_REACH_NLRI, not NextHop attribute.
// PREVENTS: Zero next-hop for non-IPv4 routes causing routing failures.
func TestConvertToRoutesIPv6(t *testing.T) {
	// Register a context for attribute parsing
	ctx := &bgpctx.EncodingContext{ASN4: true}
	ctxID := bgpctx.Registry.Register(ctx)
	nextHopAddr := netip.MustParseAddr("2001:db8::1")

	// Build attributes: ORIGIN + MP_REACH_NLRI with IPv6 next-hop
	mpReach := []byte{
		0x00, 0x02, // AFI = 2 (IPv6)
		0x01, // SAFI = 1 (Unicast)
		0x10, // Next-hop length = 16
		// Next-hop: 2001:db8::1
		0x20, 0x01, 0x0d, 0xb8, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01,
		0x00, // Reserved
	}

	attrBytes := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN IGP
	}
	// Add MP_REACH_NLRI (optional non-transitive)
	attrBytes = append(attrBytes, 0x80, 0x0e, byte(len(mpReach)))
	attrBytes = append(attrBytes, mpReach...)

	payload := buildUpdatePayload(attrBytes, nil)
	wireUpdate := api.NewWireUpdate(payload, ctxID)

	// IPv6 NLRI
	announceNLRI := nlri.NewINET(nlri.IPv6Unicast, netip.MustParsePrefix("2001:db8:1::/48"), 0)

	update := &ReceivedUpdate{
		UpdateID:     1,
		WireUpdate:   wireUpdate,
		Announces:    []nlri.NLRI{announceNLRI},
		AnnounceWire: [][]byte{announceNLRI.Bytes()},
	}

	routes, err := update.ConvertToRoutes()
	if err != nil {
		t.Fatalf("ConvertToRoutes failed: %v", err)
	}

	if len(routes) != 1 {
		t.Fatalf("expected 1 route, got %d", len(routes))
	}

	route := routes[0]
	if route.NextHop() != nextHopAddr {
		t.Errorf("NextHop = %v, want %v (extracted from MP_REACH_NLRI)", route.NextHop(), nextHopAddr)
	}
}

// TestConvertToRoutesWithdrawOnly verifies withdraw-only UPDATEs return nil.
//
// VALIDATES: Withdraw-only UPDATEs return nil routes (not error).
// PREVENTS: Attempting to store nil-attribute routes in adj-rib-out.
func TestConvertToRoutesWithdrawOnly(t *testing.T) {
	withdrawNLRI := nlri.NewINET(nlri.IPv4Unicast, netip.MustParsePrefix("10.0.0.0/8"), 0)

	// Withdraw-only: no attributes
	payload := buildUpdatePayload(nil, nil)
	wireUpdate := api.NewWireUpdate(payload, bgpctx.ContextID(1))

	update := &ReceivedUpdate{
		UpdateID:     1,
		WireUpdate:   wireUpdate,
		Announces:    nil,
		Withdraws:    []nlri.NLRI{withdrawNLRI},
		WithdrawWire: [][]byte{withdrawNLRI.Bytes()},
	}

	routes, err := update.ConvertToRoutes()
	if err != nil {
		t.Fatalf("ConvertToRoutes should not error for withdraw-only: %v", err)
	}
	if routes != nil {
		t.Errorf("expected nil routes for withdraw-only, got %d routes", len(routes))
	}
}

// TestConvertToRoutesMultipleNLRI verifies multiple NLRIs create multiple routes.
//
// VALIDATES: Each NLRI creates separate Route with shared attributes.
// PREVENTS: Missing routes when UPDATE has multiple NLRIs.
func TestConvertToRoutesMultipleNLRI(t *testing.T) {
	// Register a context for attribute parsing
	ctx := &bgpctx.EncodingContext{ASN4: true}
	ctxID := bgpctx.Registry.Register(ctx)
	nextHopAddr := netip.MustParseAddr("10.0.0.1")

	attrBytes := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN IGP
		0x40, 0x03, 0x04, 10, 0, 0, 1, // NEXT_HOP 10.0.0.1
	}

	payload := buildUpdatePayload(attrBytes, nil)
	wireUpdate := api.NewWireUpdate(payload, ctxID)

	nlri1 := nlri.NewINET(nlri.IPv4Unicast, netip.MustParsePrefix("192.168.1.0/24"), 0)
	nlri2 := nlri.NewINET(nlri.IPv4Unicast, netip.MustParsePrefix("192.168.2.0/24"), 0)
	nlri3 := nlri.NewINET(nlri.IPv4Unicast, netip.MustParsePrefix("192.168.3.0/24"), 0)

	update := &ReceivedUpdate{
		UpdateID:     1,
		WireUpdate:   wireUpdate,
		Announces:    []nlri.NLRI{nlri1, nlri2, nlri3},
		AnnounceWire: [][]byte{nlri1.Bytes(), nlri2.Bytes(), nlri3.Bytes()},
	}

	routes, err := update.ConvertToRoutes()
	if err != nil {
		t.Fatalf("ConvertToRoutes failed: %v", err)
	}

	if len(routes) != 3 {
		t.Fatalf("expected 3 routes, got %d", len(routes))
	}

	// All routes should have same next-hop
	for i, route := range routes {
		if route.NextHop() != nextHopAddr {
			t.Errorf("route[%d] NextHop = %v, want %v", i, route.NextHop(), nextHopAddr)
		}
	}
}
