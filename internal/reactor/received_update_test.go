package reactor

import (
	"encoding/binary"
	"net/netip"
	"testing"
	"time"

	bgpctx "codeberg.org/thomas-mangin/zebgp/internal/bgp/context"
	"codeberg.org/thomas-mangin/zebgp/internal/plugin"
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

	// Build UPDATE payload
	attrBytes := []byte{0x40, 0x01, 0x01, 0x00} // ORIGIN IGP
	payload := buildUpdatePayload(attrBytes, nil)
	wireUpdate := plugin.NewWireUpdate(payload, ctxID)
	wireUpdate.SetMessageID(12345)

	update := &ReceivedUpdate{
		WireUpdate:   wireUpdate,
		SourcePeerIP: sourcePeer,
		ReceivedAt:   now,
	}

	if update.WireUpdate.MessageID() != 12345 {
		t.Errorf("MessageID = %d, want 12345", update.WireUpdate.MessageID())
	}
	attrs, err := update.WireUpdate.Attrs()
	if err != nil {
		t.Errorf("WireUpdate.Attrs() error = %v", err)
	}
	if attrs == nil {
		t.Error("WireUpdate.Attrs() should not be nil")
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
	// Withdraw-only: no attributes
	payload := buildUpdatePayload(nil, nil)
	wireUpdate := plugin.NewWireUpdate(payload, bgpctx.ContextID(1))
	wireUpdate.SetMessageID(1)

	update := &ReceivedUpdate{
		WireUpdate:   wireUpdate,
		SourcePeerIP: netip.MustParseAddr("10.0.0.1"),
		ReceivedAt:   time.Now(),
	}

	attrs, err := update.WireUpdate.Attrs()
	if err != nil {
		t.Errorf("WireUpdate.Attrs() error = %v", err)
	}
	if attrs != nil {
		t.Error("withdraw-only UPDATE should have nil Attrs")
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
