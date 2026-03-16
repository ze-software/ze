package reactor

import (
	"testing"
	"unsafe"
)

// TestDrainDeliveryBatchReusesBuffer verifies that drainDeliveryBatch reuses the
// caller-provided buffer across calls — no new backing array allocation on the second call.
//
// VALIDATES: AC-1 from spec-alloc-1-batch-pooling.md
// PREVENTS: Per-burst slice allocations in per-peer delivery goroutine.
func TestDrainDeliveryBatchReusesBuffer(t *testing.T) {
	ch := make(chan deliveryItem, 4)
	ch <- deliveryItem{}
	ch <- deliveryItem{}

	first := deliveryItem{}

	// First call: buffer grows from nil.
	var buf []deliveryItem
	buf = drainDeliveryBatch(buf, &first, ch)

	if len(buf) != 3 {
		t.Fatalf("expected 3 items, got %d", len(buf))
	}
	firstPtr := unsafe.SliceData(buf)

	// Second call: reuse existing buffer.
	ch <- deliveryItem{}
	first2 := deliveryItem{}
	buf = drainDeliveryBatch(buf, &first2, ch)

	if len(buf) != 2 {
		t.Fatalf("expected 2 items, got %d", len(buf))
	}
	secondPtr := unsafe.SliceData(buf)

	if firstPtr != secondPtr {
		t.Error("second call allocated a new backing array instead of reusing buffer")
	}
}

// TestDrainDeliveryBatchChannelClose verifies correct behavior when the channel
// closes mid-drain.
//
// VALIDATES: AC-1 from spec-alloc-1-batch-pooling.md — channel close detection preserved
// PREVENTS: Dropped items or panic on closed channel.
func TestDrainDeliveryBatchChannelClose(t *testing.T) {
	ch := make(chan deliveryItem, 2)
	ch <- deliveryItem{}
	close(ch)

	first := deliveryItem{}
	buf := drainDeliveryBatch(nil, &first, ch)

	if len(buf) != 2 {
		t.Fatalf("expected 2 items (first + 1 from channel before close), got %d", len(buf))
	}
}
