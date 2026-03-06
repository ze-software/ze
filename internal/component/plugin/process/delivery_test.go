package process

import (
	"context"
	"errors"
	"strings"
	"testing"
	"unsafe"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
)

// TestProcessDrainBatchReusesBuffer verifies that drainBatch reuses the
// caller-provided buffer across calls — no new backing array allocation on the second call.
//
// VALIDATES: AC-3 from spec-alloc-1-batch-pooling.md
// PREVENTS: Per-burst slice allocations in per-process delivery goroutine.
func TestProcessDrainBatchReusesBuffer(t *testing.T) {
	proc := NewProcess(plugin.PluginConfig{Name: "test-drain-reuse"})
	proc.ctx, proc.cancel = context.WithCancel(context.Background())
	defer proc.cancel()

	proc.eventChan = make(chan EventDelivery, 4)
	proc.eventChan <- EventDelivery{Output: "a"}
	proc.eventChan <- EventDelivery{Output: "b"}

	first := EventDelivery{Output: "first"}

	// First call: buffer grows from nil.
	var buf []EventDelivery
	buf = proc.drainBatch(buf, first)

	if len(buf) != 3 {
		t.Fatalf("expected 3 items, got %d", len(buf))
	}
	firstPtr := unsafe.SliceData(buf)

	// Second call: reuse existing buffer.
	proc.eventChan <- EventDelivery{Output: "c"}
	first2 := EventDelivery{Output: "second"}
	buf = proc.drainBatch(buf, first2)

	if len(buf) != 2 {
		t.Fatalf("expected 2 items, got %d", len(buf))
	}
	secondPtr := unsafe.SliceData(buf)

	if firstPtr != secondPtr {
		t.Error("second call allocated a new backing array instead of reusing buffer")
	}
}

// TestDeliverBatchReusesEventsSlice verifies that deliverBatch reuses the
// caller-provided eventsBuf across calls — no new backing array allocation.
//
// VALIDATES: AC-4 from spec-alloc-1-batch-pooling.md
// PREVENTS: Per-batch string slice allocations in delivery pipeline.
func TestDeliverBatchReusesEventsSlice(t *testing.T) {
	proc := NewProcess(plugin.PluginConfig{Name: "test-events-reuse"})
	proc.ctx, proc.cancel = context.WithCancel(context.Background())
	defer proc.cancel()

	// No bridge or connB set — deliverBatch will error with "connection closed",
	// but the eventsBuf slice is still constructed and returned.
	batch1 := []EventDelivery{
		{Output: "event-1"},
		{Output: "event-2"},
		{Output: "event-3"},
	}

	// First call: eventsBuf grows from nil.
	var eventsBuf []string
	eventsBuf = proc.deliverBatch(batch1, eventsBuf, defaultDeliveryTimeout)

	if len(eventsBuf) != 3 {
		t.Fatalf("expected 3 events, got %d", len(eventsBuf))
	}
	firstPtr := unsafe.SliceData(eventsBuf)

	// Second call: reuse existing eventsBuf.
	batch2 := []EventDelivery{
		{Output: "event-a"},
		{Output: "event-b"},
	}
	eventsBuf = proc.deliverBatch(batch2, eventsBuf, defaultDeliveryTimeout)

	if len(eventsBuf) != 2 {
		t.Fatalf("expected 2 events, got %d", len(eventsBuf))
	}
	secondPtr := unsafe.SliceData(eventsBuf)

	if firstPtr != secondPtr {
		t.Error("second call allocated a new backing array instead of reusing eventsBuf")
	}
}

// TestSafeBridgeCallRecoversPanic verifies that safeBridgeCall catches panics
// from DirectBridge handlers and returns them as errors.
//
// VALIDATES: H1 — DirectBridge panic does not crash delivery loop.
// PREVENTS: Internal plugin panic propagating to engine event loop.
func TestSafeBridgeCallRecoversPanic(t *testing.T) {
	err := safeBridgeCall(func() error {
		panic("plugin handler exploded")
	})
	if err == nil {
		t.Fatal("expected error from panicking bridge call, got nil")
	}
	if !strings.Contains(err.Error(), "plugin panic") {
		t.Errorf("error should mention 'plugin panic', got: %v", err)
	}
}

// TestSafeBridgeCallPassesError verifies that safeBridgeCall passes through
// normal errors without interference.
//
// VALIDATES: H1 — normal errors unaffected by panic recovery.
// PREVENTS: Panic recovery swallowing legitimate errors.
func TestSafeBridgeCallPassesError(t *testing.T) {
	want := errors.New("normal failure")
	got := safeBridgeCall(func() error {
		return want
	})
	if !errors.Is(got, want) {
		t.Errorf("expected %v, got %v", want, got)
	}
}

// TestSafeBridgeCallSuccess verifies that safeBridgeCall returns nil on success.
func TestSafeBridgeCallSuccess(t *testing.T) {
	err := safeBridgeCall(func() error {
		return nil
	})
	if err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}
