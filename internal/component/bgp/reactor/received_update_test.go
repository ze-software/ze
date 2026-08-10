package reactor

import (
	"encoding/binary"
	"net/netip"
	"sync"
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

// extractFirstASN extracts the first ASN from AS_PATH in an UPDATE payload.
// Returns (asn, true) if found, (0, false) otherwise.
func extractFirstASN(payload []byte) (uint32, bool) {
	if len(payload) < 4 {
		return 0, false
	}
	wdLen := int(binary.BigEndian.Uint16(payload[0:2]))
	if len(payload) < 2+wdLen+2 {
		return 0, false
	}
	attrLenOff := 2 + wdLen
	attrLen := int(binary.BigEndian.Uint16(payload[attrLenOff : attrLenOff+2]))
	attrsStart := attrLenOff + 2

	off := attrsStart
	for off < attrsStart+attrLen {
		if off+3 > len(payload) {
			return 0, false
		}
		code := payload[off+1]
		length := int(payload[off+2])
		hdrLen := 3
		if payload[off]&0x10 != 0 { // Extended Length
			if off+4 > len(payload) {
				return 0, false
			}
			length = int(binary.BigEndian.Uint16(payload[off+2 : off+4]))
			hdrLen = 4
		}
		if code == 2 { // AS_PATH
			value := payload[off+hdrLen : off+hdrLen+length]
			if len(value) < 6 { // type(1) + count(1) + at least one 4-byte ASN
				return 0, false
			}
			// First segment, first ASN (4-byte)
			asn := binary.BigEndian.Uint32(value[2:6])
			return asn, true
		}
		off += hdrLen + length
	}
	return 0, false
}

// TestReceivedUpdate_EBGPWireLazyASN4 verifies lazy generation of EBGP wire
// with ASN4 encoding.
//
// VALIDATES: AC-7 — Gets pool buffer, generates patched WireUpdate, caches as ebgpWireASN4.
// PREVENTS: Missing lazy generation or incorrect caching.
func TestReceivedUpdate_EBGPWireLazyASN4(t *testing.T) {
	ctx := bgpctx.EncodingContextForASN4(true)
	ctxID, _ := bgpctx.Registry.Register(ctx)

	payload := testUpdatePayloadWithASPath([]uint32{64512, 64513})
	wu := wireu.NewWireUpdate(payload, ctxID)
	wu.SetMessageID(nextMsgID())

	update := &ReceivedUpdate{
		WireUpdate:   wu,
		SourcePeerIP: netip.MustParseAddr("10.0.0.1"),
		ReceivedAt:   time.Now(),
	}

	ebgpWire, err := update.EBGPWire(65000, true, true)
	if err != nil {
		t.Fatalf("EBGPWire() error = %v", err)
	}
	if ebgpWire == nil {
		t.Fatal("EBGPWire() returned nil")
		return
	}

	// Verify the patched wire has localASN prepended
	firstASN, ok := extractFirstASN(ebgpWire.Payload())
	if !ok {
		t.Fatal("could not extract first ASN from EBGP wire")
	}
	if firstASN != 65000 {
		t.Errorf("first ASN = %d, want 65000", firstASN)
	}

	// Verify cached
	if s := update.ebgpSlotASN4.Load(); s == nil {
		t.Error("ebgpSlotASN4 should be cached")
	} else if s.handle.Buf == nil {
		t.Error("ebgpSlotASN4 handle should be stored")
	}
}

// TestReceivedUpdate_EBGPWireCachedASN4 verifies that second call returns
// the same cached pointer without re-generation.
//
// VALIDATES: AC-8 — Pointer equality on second call (no re-patch, no pool get).
// PREVENTS: Redundant pool allocations per ForwardUpdate.
func TestReceivedUpdate_EBGPWireCachedASN4(t *testing.T) {
	ctx := bgpctx.EncodingContextForASN4(true)
	ctxID, _ := bgpctx.Registry.Register(ctx)

	payload := testUpdatePayloadWithASPath([]uint32{64512})
	wu := wireu.NewWireUpdate(payload, ctxID)
	wu.SetMessageID(nextMsgID())

	update := &ReceivedUpdate{
		WireUpdate:   wu,
		SourcePeerIP: netip.MustParseAddr("10.0.0.1"),
		ReceivedAt:   time.Now(),
	}

	first, err := update.EBGPWire(65000, true, true)
	if err != nil {
		t.Fatalf("first EBGPWire() error = %v", err)
	}

	second, err := update.EBGPWire(65000, true, true)
	if err != nil {
		t.Fatalf("second EBGPWire() error = %v", err)
	}

	if first != second {
		t.Error("second call should return same pointer")
	}
}

// TestReceivedUpdate_EBGPWireLazyASN2 verifies that ASN2 variant is cached
// separately from ASN4.
//
// VALIDATES: AC-9 — Generates separate ASN2 version; caches as ebgpWireASN2.
// PREVENTS: ASN4/ASN2 variants overwriting each other.
func TestReceivedUpdate_EBGPWireLazyASN2(t *testing.T) {
	ctx := bgpctx.EncodingContextForASN4(true)
	ctxID, _ := bgpctx.Registry.Register(ctx)

	payload := testUpdatePayloadWithASPath([]uint32{64512})
	wu := wireu.NewWireUpdate(payload, ctxID)
	wu.SetMessageID(nextMsgID())

	update := &ReceivedUpdate{
		WireUpdate:   wu,
		SourcePeerIP: netip.MustParseAddr("10.0.0.1"),
		ReceivedAt:   time.Now(),
	}

	asn4Wire, err := update.EBGPWire(65000, true, true)
	if err != nil {
		t.Fatalf("EBGPWire(asn4) error = %v", err)
	}

	asn2Wire, err := update.EBGPWire(65000, true, false)
	if err != nil {
		t.Fatalf("EBGPWire(asn2) error = %v", err)
	}

	if asn4Wire == asn2Wire {
		t.Error("ASN4 and ASN2 should be different objects")
	}
	if s := update.ebgpSlotASN4.Load(); s == nil {
		t.Error("ebgpSlotASN4 should be cached")
	} else if s.handle.Buf == nil {
		t.Error("ebgpSlotASN4 handle should be stored")
	}
	if s := update.ebgpSlotASN2.Load(); s == nil {
		t.Error("ebgpSlotASN2 should be cached")
	} else if s.handle.Buf == nil {
		t.Error("ebgpSlotASN2 handle should be stored")
	}
}

// TestReceivedUpdate_EBGPWireConcurrent verifies thread safety of concurrent
// EBGPWire calls.
//
// VALIDATES: Concurrent calls safe (no data race).
// PREVENTS: Race conditions from concurrent lazy initialization.
func TestReceivedUpdate_EBGPWireConcurrent(t *testing.T) {
	ctx := bgpctx.EncodingContextForASN4(true)
	ctxID, _ := bgpctx.Registry.Register(ctx)

	payload := testUpdatePayloadWithASPath([]uint32{64512, 64513})
	wu := wireu.NewWireUpdate(payload, ctxID)
	wu.SetMessageID(nextMsgID())

	update := &ReceivedUpdate{
		WireUpdate:   wu,
		SourcePeerIP: netip.MustParseAddr("10.0.0.1"),
		ReceivedAt:   time.Now(),
	}

	const goroutines = 10
	var wg sync.WaitGroup
	results := make([]*wireu.WireUpdate, goroutines)
	errs := make([]error, goroutines)

	wg.Add(goroutines)
	for i := range goroutines {
		go func(idx int) {
			defer wg.Done()
			dstASN4 := idx%2 == 0
			results[idx], errs[idx] = update.EBGPWire(65000, true, dstASN4)
		}(i)
	}
	wg.Wait()

	for i := range goroutines {
		if errs[i] != nil {
			t.Errorf("goroutine %d error = %v", i, errs[i])
		}
		if results[i] == nil {
			t.Errorf("goroutine %d returned nil", i)
		}
	}

	// All ASN4 results should be the same pointer
	var asn4Result *wireu.WireUpdate
	for i := range goroutines {
		if i%2 == 0 {
			if asn4Result == nil {
				asn4Result = results[i]
			} else if results[i] != asn4Result {
				t.Error("all ASN4 results should be same pointer")
			}
		}
	}
}

// TestReceivedUpdate_EBGPWireEvictionReturnsBuffers verifies that cache
// eviction returns exactly the published EBGP variant pool buffers.
//
// VALIDATES: AC-5 — eviction of entries with 0, 1, or 2 published variants
// returns the correct buffers.
// PREVENTS: Pool buffer leaks when cache entries with EBGP variants are evicted.
func TestReceivedUpdate_EBGPWireEvictionReturnsBuffers(t *testing.T) {
	ctx := bgpctx.EncodingContextForASN4(true)
	ctxID, _ := bgpctx.Registry.Register(ctx)

	cases := []struct {
		name     string
		variants int // 0, 1, or 2
	}{
		{"no variants", 0},
		{"one variant (ASN4)", 1},
		{"both variants", 2},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, stdBefore := bufMuxStd.Stats()

			payload := testUpdatePayloadWithASPath([]uint32{64512, 64513})
			wu := wireu.NewWireUpdate(payload, ctxID)
			id := nextMsgID()
			wu.SetMessageID(id)

			update := &ReceivedUpdate{
				WireUpdate:   wu,
				poolBuf:      BufHandle{ID: noPoolBufID, Buf: make([]byte, 4096)},
				SourcePeerIP: netip.MustParseAddr("10.0.0.1"),
				ReceivedAt:   time.Now(),
			}

			if tc.variants >= 1 {
				_, err := update.EBGPWire(65000, true, true)
				if err != nil {
					t.Fatalf("EBGPWire(ASN4) error = %v", err)
				}
			}
			if tc.variants >= 2 {
				_, err := update.EBGPWire(65000, true, false)
				if err != nil {
					t.Fatalf("EBGPWire(ASN2) error = %v", err)
				}
			}

			_, stdAfterGen := bufMuxStd.Stats()
			variantsAllocated := stdAfterGen - stdBefore
			if variantsAllocated != tc.variants {
				t.Errorf("expected %d variant buffers in use, got delta %d", tc.variants, variantsAllocated)
			}

			cache := newRecentUpdateCache(100)
			cache.RegisterConsumer("test-plugin")
			cache.Add(update)
			cache.Activate(id, 1)
			_ = cache.Ack(id, "test-plugin")

			_, stdAfterEvict := bufMuxStd.Stats()
			leaked := stdAfterEvict - stdBefore
			if leaked != 0 {
				t.Errorf("expected 0 buffers leaked after eviction, got delta %d", leaked)
			}
		})
	}
}

// TestReceivedUpdate_EBGPWireErrorDoesNotPublish verifies that a failed
// generation does not cache the variant and returns the pool buffer.
//
// VALIDATES: AC-4 — generation error leaves slot nil; buffer returned; later call retries.
// PREVENTS: Stale nil-wire cached after error; pool buffer leak on error path.
func TestReceivedUpdate_EBGPWireErrorDoesNotPublish(t *testing.T) {
	ctx := bgpctx.EncodingContextForASN4(true)
	ctxID, _ := bgpctx.Registry.Register(ctx)

	// Truncated payload: too short for RewriteASPath to parse.
	truncated := []byte{0x00, 0x00}
	wu := wireu.NewWireUpdate(truncated, ctxID)
	wu.SetMessageID(nextMsgID())

	update := &ReceivedUpdate{
		WireUpdate:   wu,
		poolBuf:      BufHandle{ID: noPoolBufID, Buf: make([]byte, 4096)},
		SourcePeerIP: netip.MustParseAddr("10.0.0.1"),
		ReceivedAt:   time.Now(),
	}

	_, stdBefore := bufMuxStd.Stats()

	_, err := update.EBGPWire(65000, true, true)
	if err == nil {
		t.Fatal("expected error from EBGPWire with truncated payload")
	}

	// test-relax: ebgpWireASN4/ebgpPoolBuf4 fields replaced by atomic ebgpSlotASN4;
	// a nil slot covers both assertions (wire==nil AND handle not stored).
	if s := update.ebgpSlotASN4.Load(); s != nil {
		t.Error("ebgpSlotASN4 should be nil after error (no wire cached, no handle stored)")
	}

	_, stdAfter := bufMuxStd.Stats()
	if stdAfter != stdBefore {
		t.Errorf("pool buffer leaked: inUse before=%d, after=%d", stdBefore, stdAfter)
	}

	// Retry should attempt generation again (not return cached nil).
	_, err2 := update.EBGPWire(65000, true, true)
	if err2 == nil {
		t.Fatal("expected error on retry with truncated payload")
	}
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
