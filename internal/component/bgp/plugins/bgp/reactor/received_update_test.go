package reactor

import (
	"encoding/binary"
	"net/netip"
	"sync"
	"testing"
	"time"

	bgpctx "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/bgp/context"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/bgp/wireu"
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
	ctxID := bgpctx.Registry.Register(ctx)

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
	if update.ebgpWireASN4 == nil {
		t.Error("ebgpWireASN4 should be cached")
	}
	if update.ebgpPoolBuf4 == nil {
		t.Error("ebgpPoolBuf4 should be stored")
	}
}

// TestReceivedUpdate_EBGPWireCachedASN4 verifies that second call returns
// the same cached pointer without re-generation.
//
// VALIDATES: AC-8 — Pointer equality on second call (no re-patch, no pool get).
// PREVENTS: Redundant pool allocations per ForwardUpdate.
func TestReceivedUpdate_EBGPWireCachedASN4(t *testing.T) {
	ctx := bgpctx.EncodingContextForASN4(true)
	ctxID := bgpctx.Registry.Register(ctx)

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
	ctxID := bgpctx.Registry.Register(ctx)

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
	if update.ebgpWireASN4 == nil {
		t.Error("ebgpWireASN4 should be cached")
	}
	if update.ebgpWireASN2 == nil {
		t.Error("ebgpWireASN2 should be cached")
	}
	if update.ebgpPoolBuf4 == nil {
		t.Error("ebgpPoolBuf4 should be stored")
	}
	if update.ebgpPoolBuf2 == nil {
		t.Error("ebgpPoolBuf2 should be stored")
	}
}

// TestReceivedUpdate_EBGPWireConcurrent verifies thread safety of concurrent
// EBGPWire calls.
//
// VALIDATES: Concurrent calls safe (no data race).
// PREVENTS: Race conditions from concurrent lazy initialization.
func TestReceivedUpdate_EBGPWireConcurrent(t *testing.T) {
	ctx := bgpctx.EncodingContextForASN4(true)
	ctxID := bgpctx.Registry.Register(ctx)

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
			dstAsn4 := idx%2 == 0
			results[idx], errs[idx] = update.EBGPWire(65000, true, dstAsn4)
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
