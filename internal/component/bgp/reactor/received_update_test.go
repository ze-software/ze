package reactor

import (
	"encoding/binary"
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/wireu"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	wireUpdate := wireu.NewWireUpdate(payload, ctxID)
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
	wireUpdate := wireu.NewWireUpdate(payload, bgpctx.ContextID(1))
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
	for i := range 1000 {
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
	for range 100 {
		id := nextMsgID()
		if id <= prev {
			t.Fatalf("ID %d not greater than previous %d", id, prev)
		}
		prev = id
	}
}

// testUpdatePayloadWithASPath builds an UPDATE payload with ORIGIN + AS_PATH.
// Constructs the AS_PATH attribute inline using raw bytes to avoid importing attribute package.
func testUpdatePayloadWithASPath(asns []uint32) []byte {
	// ORIGIN attribute: flags=0x40, code=1, len=1, value=0 (IGP)
	origin := []byte{0x40, 0x01, 0x01, 0x00}

	// AS_PATH attribute (ASN4 encoding): flags=0x40, code=2
	// Segment: type=2 (AS_SEQUENCE), count=len(asns), then 4-byte ASNs
	segLen := 2 + len(asns)*4 // type(1) + count(1) + ASNs
	aspValue := make([]byte, segLen)
	aspValue[0] = 2               // AS_SEQUENCE
	aspValue[1] = byte(len(asns)) //nolint:gosec // test data, count < 256
	for i, asn := range asns {
		binary.BigEndian.PutUint32(aspValue[2+i*4:], asn)
	}

	// Header: flags=0x40, code=2, len
	aspAttr := make([]byte, 3+len(aspValue))
	aspAttr[0] = 0x40                // Transitive
	aspAttr[1] = 0x02                // AS_PATH
	aspAttr[2] = byte(len(aspValue)) //nolint:gosec // test data
	copy(aspAttr[3:], aspValue)

	attrs := make([]byte, 0, len(origin)+len(aspAttr))
	attrs = append(attrs, origin...)
	attrs = append(attrs, aspAttr...)

	return buildUpdatePayload(attrs, nil)
}

// TestReceivedUpdateAdoptedHandlesReturnedOnce verifies the adopt-list mechanism
// shared by all six forward borrow sites: handles adopted onto the entry are
// returned to the pool exactly once when the cache evicts the entry (via ack or
// Delete), an entry with no adopted handles is a no-op, and a second drain (the
// hypothetical double-evict) returns nothing twice.
//
// VALIDATES: A-4 / AC-1 -- adoptFwdHandle + eviction drain return each handle once.
// PREVENTS: leaked forward buffers, or a double-return corrupting the pool.
func TestReceivedUpdateAdoptedHandlesReturnedOnce(t *testing.T) {
	ctx := bgpctx.EncodingContextForASN4(true)
	ctxID, _ := bgpctx.Registry.Register(ctx)

	// newEntry builds a cache-resident ReceivedUpdate with a no-op poolBuf so only
	// adopted handles move the shared pool counter.
	// nlri is a real NLRI section (192.0.2.0/24) so the update carries reachable
	// routes like a forwarded UPDATE would.
	nlri := []byte{24, 192, 0, 2}
	newEntry := func(t *testing.T) (*RecentUpdateCache, *ReceivedUpdate, uint64) {
		t.Helper()
		wu := wireu.NewWireUpdate(buildUpdatePayload([]byte{0x40, 0x01, 0x01, 0x00}, nlri), ctxID)
		id := nextMsgID()
		wu.SetMessageID(id)
		update := &ReceivedUpdate{
			WireUpdate:   wu,
			poolBuf:      BufHandle{ID: noPoolBufID, Buf: make([]byte, 4096)},
			SourcePeerIP: netip.MustParseAddr("10.0.0.1"),
			ReceivedAt:   time.Now(),
		}
		cache := newRecentUpdateCache(100)
		cache.RegisterConsumer("test-plugin")
		cache.Add(update)
		cache.Activate(id, 1)
		return cache, update, id
	}

	t.Run("empty list is a no-op on eviction", func(t *testing.T) {
		_, before := bufMuxStd.Stats()
		cache, _, id := newEntry(t)
		require.NoError(t, cache.Ack(id, "test-plugin"))
		_, after := bufMuxStd.Stats()
		assert.Equal(t, before, after, "eviction with no adopted handles must not touch the pool")
	})

	for _, k := range []int{1, 2, 3} {
		t.Run("evict returns each of K handles once", func(t *testing.T) {
			_, before := bufMuxStd.Stats()
			cache, update, id := newEntry(t)

			for range k {
				h := bufMuxStd.Get()
				require.NotNil(t, h.Buf, "pool must hand out a buffer")
				update.adoptFwdHandle(h)
			}
			_, afterAdopt := bufMuxStd.Stats()
			require.Equal(t, before+k, afterAdopt, "K adopted handles must be in use")

			require.NoError(t, cache.Ack(id, "test-plugin"))
			_, ok := cache.Get(id)
			require.False(t, ok, "entry must be evicted after ack")

			_, after := bufMuxStd.Stats()
			assert.Equal(t, before, after, "all K adopted handles must be returned exactly once at eviction")
		})
	}

	t.Run("Delete also drains the adopted handles", func(t *testing.T) {
		_, before := bufMuxStd.Stats()
		cache, update, id := newEntry(t)
		h := bufMuxStd.Get()
		require.NotNil(t, h.Buf)
		update.adoptFwdHandle(h)

		require.True(t, cache.Delete(id), "Delete must find and remove the entry")
		_, after := bufMuxStd.Stats()
		assert.Equal(t, before, after, "Delete must return the adopted handle to the pool")
	})

	t.Run("second drain returns nothing twice", func(t *testing.T) {
		_, before := bufMuxStd.Stats()
		update := &ReceivedUpdate{
			WireUpdate: wireu.NewWireUpdate(buildUpdatePayload([]byte{0x40, 0x01, 0x01, 0x00}, nil), ctxID),
		}
		h := bufMuxStd.Get()
		require.NotNil(t, h.Buf)
		update.adoptFwdHandle(h)

		update.returnFwdHandles()
		_, afterFirst := bufMuxStd.Stats()
		require.Equal(t, before, afterFirst, "first drain returns the handle")

		// A second drain (idempotent) must be a pure no-op: no double-return.
		update.returnFwdHandles()
		_, afterSecond := bufMuxStd.Stats()
		assert.Equal(t, before, afterSecond, "second drain must return nothing (idempotent)")
	})
}
