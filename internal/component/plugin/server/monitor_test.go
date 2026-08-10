package server

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bgpevents "github.com/ze-software/ze/internal/core/bgp/events"
	"github.com/ze-software/ze/internal/core/events"
)

// =============================================================================
// MonitorManager Add/Remove Tests
// =============================================================================

// TestMonitorManagerAddRemove verifies add and remove operations.
//
// VALIDATES: Monitor clients can be added and removed by ID.
// PREVENTS: Memory leaks or orphaned monitor entries.
func TestMonitorManagerAddRemove(t *testing.T) {
	mm := NewMonitorManager()

	mc := NewMonitorClient(t.Context(), "test-1", []*Subscription{
		{Namespace: events.LookupNamespaceID("bgp"), EventType: events.LookupEventTypeID(bgpevents.EventUpdate), Direction: events.DirBoth},
	}, 256)

	mm.Add(mc)
	assert.Equal(t, 1, mm.Count())

	mm.Remove("test-1")
	assert.Equal(t, 0, mm.Count())

	// Remove again should be no-op.
	mm.Remove("test-1")
	assert.Equal(t, 0, mm.Count())
}

// TestMonitorManagerGetMatching verifies subscription matching for monitors.
//
// VALIDATES: GetMatching returns only monitors whose subscriptions match the event.
// PREVENTS: Events delivered to wrong monitors or missed by matching monitors.
func TestMonitorManagerGetMatching(t *testing.T) {
	mm := NewMonitorManager()
	ctx := t.Context()

	// Monitor 1: subscribes to updates from all peers.
	mc1 := NewMonitorClient(ctx, "all-updates", []*Subscription{
		{Namespace: events.LookupNamespaceID("bgp"), EventType: events.LookupEventTypeID(bgpevents.EventUpdate), Direction: events.DirBoth},
	}, 256)

	// Monitor 2: subscribes to state events only.
	mc2 := NewMonitorClient(ctx, "state-only", []*Subscription{
		{Namespace: events.LookupNamespaceID("bgp"), EventType: events.LookupEventTypeID(bgpevents.EventState), Direction: events.DirBoth},
	}, 256)

	// Monitor 3: subscribes to updates from specific peer.
	mc3 := NewMonitorClient(ctx, "peer-updates", []*Subscription{
		{Namespace: events.LookupNamespaceID("bgp"), EventType: events.LookupEventTypeID(bgpevents.EventUpdate), Direction: events.DirBoth,
			PeerFilter: &PeerFilter{Selector: "10.0.0.1"}},
	}, 256)

	mm.Add(mc1)
	mm.Add(mc2)
	mm.Add(mc3)

	// Update from 10.0.0.1 should match mc1 and mc3.
	matches := mm.GetMatching(bgpevents.Namespace, bgpevents.EventUpdate, events.DirectionReceived, "10.0.0.1", "")
	assert.Len(t, matches, 2)
	ids := matchIDs(matches)
	assert.Contains(t, ids, "all-updates")
	assert.Contains(t, ids, "peer-updates")

	// Update from 10.0.0.2 should match mc1 only.
	matches = mm.GetMatching(bgpevents.Namespace, bgpevents.EventUpdate, events.DirectionReceived, "10.0.0.2", "")
	assert.Len(t, matches, 1)
	assert.Equal(t, "all-updates", matches[0].id)

	// State event should match mc2 only.
	matches = mm.GetMatching(bgpevents.Namespace, bgpevents.EventState, "", "10.0.0.1", "")
	assert.Len(t, matches, 1)
	assert.Equal(t, "state-only", matches[0].id)

	// Notification event should match nobody.
	matches = mm.GetMatching(bgpevents.Namespace, bgpevents.EventNotification, events.DirectionReceived, "10.0.0.1", "")
	assert.Len(t, matches, 0)
}

// matchIDs extracts IDs from a slice of MonitorClients.
func matchIDs(clients []*MonitorClient) []string {
	ids := make([]string, len(clients))
	for i, c := range clients {
		ids[i] = c.id
	}
	return ids
}

// TestMonitorManagerCleanup verifies that Remove cleans up all state.
//
// VALIDATES: After Remove, monitor is not returned by GetMatching.
// PREVENTS: Stale monitor entries causing event delivery to dead clients.
func TestMonitorManagerCleanup(t *testing.T) {
	mm := NewMonitorManager()

	mc := NewMonitorClient(t.Context(), "cleanup-test", []*Subscription{
		{Namespace: events.LookupNamespaceID("bgp"), EventType: events.LookupEventTypeID(bgpevents.EventUpdate), Direction: events.DirBoth},
	}, 256)

	mm.Add(mc)
	matches := mm.GetMatching(bgpevents.Namespace, bgpevents.EventUpdate, events.DirectionReceived, "10.0.0.1", "")
	require.Len(t, matches, 1)

	mm.Remove("cleanup-test")
	matches = mm.GetMatching(bgpevents.Namespace, bgpevents.EventUpdate, events.DirectionReceived, "10.0.0.1", "")
	assert.Len(t, matches, 0)
}

// =============================================================================
// MonitorManager Delivery Tests
// =============================================================================

// TestMonitorDelivery verifies events are delivered to matching monitors.
//
// VALIDATES: Deliver sends formatted output to matching monitors' EventChan.
// PREVENTS: Events lost or delivered to non-matching monitors.
func TestMonitorDelivery(t *testing.T) {
	mm := NewMonitorManager()
	ctx := t.Context()

	mc1 := NewMonitorClient(ctx, "receiver", []*Subscription{
		{Namespace: events.LookupNamespaceID("bgp"), EventType: events.LookupEventTypeID(bgpevents.EventUpdate), Direction: events.DirBoth},
	}, 256)

	mc2 := NewMonitorClient(ctx, "non-receiver", []*Subscription{
		{Namespace: events.LookupNamespaceID("bgp"), EventType: events.LookupEventTypeID(bgpevents.EventState), Direction: events.DirBoth},
	}, 256)

	mm.Add(mc1)
	mm.Add(mc2)

	testEvent := `{"type":"bgp","bgp":{"message":{"type":"update"}}}`
	mm.Deliver(bgpevents.Namespace, bgpevents.EventUpdate, events.DirectionReceived, "10.0.0.1", "", testEvent)

	// mc1 should receive it.
	select {
	case got := <-mc1.EventChan:
		assert.Equal(t, testEvent, got)
	default:
		t.Fatal("mc1 should have received the event")
	}

	// mc2 should not.
	select {
	case <-mc2.EventChan:
		t.Fatal("mc2 should not have received the event")
	default:
		// expected
	}
}

// TestMonitorBackpressure verifies dropped events when channel is full.
//
// VALIDATES: Full channel drops events and increments dropped counter.
// PREVENTS: Blocking the event pipeline on a slow monitor client.
func TestMonitorBackpressure(t *testing.T) {
	mm := NewMonitorManager()

	// Use small buffer to test backpressure.
	mc := NewMonitorClient(t.Context(), "slow-client", []*Subscription{
		{Namespace: events.LookupNamespaceID("bgp"), EventType: events.LookupEventTypeID(bgpevents.EventUpdate), Direction: events.DirBoth},
	}, 2) // tiny buffer

	mm.Add(mc)

	// Send 5 events; buffer holds 2, so 3 should be dropped.
	for i := range 5 {
		mm.Deliver(bgpevents.Namespace, bgpevents.EventUpdate, events.DirectionReceived, "10.0.0.1", "",
			fmt.Sprintf(`{"event":%d}`, i))
	}

	assert.Equal(t, uint64(3), mc.Dropped.Load(), "expected 3 dropped events")

	// The 2 buffered events should still be readable.
	assert.Len(t, mc.EventChan, 2)
}

// TestMonitorDeliverLazyNoMonitors verifies DeliverLazy skips build when empty.
//
// VALIDATES: With no monitors registered, the build callback is never invoked.
// PREVENTS: Structured plugin consumers paying JSON formatting cost when no
// CLI monitor is attached.
func TestMonitorDeliverLazyNoMonitors(t *testing.T) {
	mm := NewMonitorManager()

	called := 0
	mm.deliverLazy(bgpevents.Namespace, bgpevents.EventUpdate, events.DirectionReceived, "10.0.0.1", "", func() string {
		called++
		return `{"should":"not appear"}`
	})

	assert.Equal(t, 0, called, "build must not run when no monitors are registered")
}

// TestMonitorDeliverLazyNoMatch verifies DeliverLazy skips build on no match.
//
// VALIDATES: When monitors exist but none match the event, build is not called.
// PREVENTS: Formatting JSON for events that no subscription cares about.
func TestMonitorDeliverLazyNoMatch(t *testing.T) {
	mm := NewMonitorManager()

	mc := NewMonitorClient(t.Context(), "state-only", []*Subscription{
		{Namespace: events.LookupNamespaceID("bgp"), EventType: events.LookupEventTypeID(bgpevents.EventState), Direction: events.DirBoth},
	}, 256)
	mm.Add(mc)

	called := 0
	mm.deliverLazy(bgpevents.Namespace, bgpevents.EventUpdate, events.DirectionReceived, "10.0.0.1", "", func() string {
		called++
		return `{"should":"not appear"}`
	})

	assert.Equal(t, 0, called, "build must not run when no subscription matches")
	assert.Empty(t, mc.EventChan)
}

// TestMonitorDeliverLazyMatch verifies DeliverLazy invokes build once and fans out.
//
// VALIDATES: build is invoked exactly once even when multiple monitors match,
// and all matching monitors receive the same output string.
// PREVENTS: Per-monitor formatting cost or stale/missed delivery.
func TestMonitorDeliverLazyMatch(t *testing.T) {
	mm := NewMonitorManager()
	ctx := t.Context()

	mc1 := NewMonitorClient(ctx, "one", []*Subscription{
		{Namespace: events.LookupNamespaceID("bgp"), EventType: events.LookupEventTypeID(bgpevents.EventUpdate), Direction: events.DirBoth},
	}, 256)
	mc2 := NewMonitorClient(ctx, "two", []*Subscription{
		{Namespace: events.LookupNamespaceID("bgp"), EventType: events.LookupEventTypeID(bgpevents.EventUpdate), Direction: events.DirBoth},
	}, 256)
	mm.Add(mc1)
	mm.Add(mc2)

	called := 0
	const payload = `{"ok":true}`
	mm.deliverLazy(bgpevents.Namespace, bgpevents.EventUpdate, events.DirectionReceived, "10.0.0.1", "", func() string {
		called++
		return payload
	})

	assert.Equal(t, 1, called, "build must run exactly once even with two matching monitors")

	for _, mc := range []*MonitorClient{mc1, mc2} {
		select {
		case got := <-mc.EventChan:
			assert.Equal(t, payload, got)
		default:
			t.Fatalf("monitor %s did not receive event", mc.id)
		}
	}
}

// TestMonitorManagerConcurrency verifies thread-safe operations.
//
// VALIDATES: Concurrent Add/Remove/GetMatching/Deliver don't race.
// PREVENTS: Race conditions under concurrent access.
func TestMonitorManagerConcurrency(t *testing.T) {
	mm := NewMonitorManager()
	ctx := t.Context()

	var wg sync.WaitGroup
	const goroutines = 10

	// Concurrent adds.
	for i := range goroutines {
		wg.Go(func() {
			mc := NewMonitorClient(ctx, fmt.Sprintf("client-%d", i), []*Subscription{
				{Namespace: events.LookupNamespaceID("bgp"), EventType: events.LookupEventTypeID(bgpevents.EventUpdate), Direction: events.DirBoth},
			}, 256)
			mm.Add(mc)
		})
	}
	wg.Wait()

	// Concurrent reads + delivers (Deliver and DeliverLazy paths both exercised).
	for range goroutines {
		wg.Go(func() {
			for range 100 {
				_ = mm.GetMatching(bgpevents.Namespace, bgpevents.EventUpdate, events.DirectionReceived, "10.0.0.1", "")
				mm.Deliver(bgpevents.Namespace, bgpevents.EventUpdate, events.DirectionReceived, "10.0.0.1", "", `{"test":true}`)
				mm.deliverLazy(bgpevents.Namespace, bgpevents.EventUpdate, events.DirectionReceived, "10.0.0.1", "", func() string {
					return `{"test":"lazy"}`
				})
			}
		})
	}
	wg.Wait()

	// Concurrent removes.
	for i := range goroutines {
		wg.Go(func() {
			mm.Remove(fmt.Sprintf("client-%d", i))
		})
	}
	wg.Wait()

	assert.Equal(t, 0, mm.Count())
}

// =============================================================================
// HasMonitors (atomic count) Tests
// =============================================================================

// TestHasMonitorsEmpty verifies HasMonitors returns false when no monitors exist.
func TestHasMonitorsEmpty(t *testing.T) {
	mm := NewMonitorManager()
	assert.False(t, mm.HasMonitors())
}

// TestHasMonitorsAddRemove verifies HasMonitors tracks Add and Remove.
func TestHasMonitorsAddRemove(t *testing.T) {
	mm := NewMonitorManager()
	ctx := t.Context()

	mc1 := NewMonitorClient(ctx, "a", []*Subscription{
		{Namespace: events.LookupNamespaceID("bgp"), EventType: events.LookupEventTypeID(bgpevents.EventUpdate), Direction: events.DirBoth},
	}, 256)
	mc2 := NewMonitorClient(ctx, "b", []*Subscription{
		{Namespace: events.LookupNamespaceID("bgp"), EventType: events.LookupEventTypeID(bgpevents.EventUpdate), Direction: events.DirBoth},
	}, 256)

	mm.Add(mc1)
	assert.True(t, mm.HasMonitors())

	mm.Add(mc2)
	assert.True(t, mm.HasMonitors())

	mm.Remove("a")
	assert.True(t, mm.HasMonitors())

	mm.Remove("b")
	assert.False(t, mm.HasMonitors())
}

// TestHasMonitorsDuplicateAdd verifies that re-adding the same ID does not double-count.
func TestHasMonitorsDuplicateAdd(t *testing.T) {
	mm := NewMonitorManager()
	ctx := t.Context()

	mc := NewMonitorClient(ctx, "dup", []*Subscription{
		{Namespace: events.LookupNamespaceID("bgp"), EventType: events.LookupEventTypeID(bgpevents.EventUpdate), Direction: events.DirBoth},
	}, 256)

	mm.Add(mc)
	mm.Add(mc)
	assert.True(t, mm.HasMonitors())
	assert.Equal(t, 1, mm.Count())

	mm.Remove("dup")
	assert.False(t, mm.HasMonitors())
}

// TestHasMonitorsRemoveNonexistent verifies removing an unknown ID does not underflow.
func TestHasMonitorsRemoveNonexistent(t *testing.T) {
	mm := NewMonitorManager()
	mm.Remove("does-not-exist")
	assert.False(t, mm.HasMonitors())
	assert.Equal(t, 0, mm.Count())
}

// =============================================================================
// Typed Delivery Tests
// =============================================================================

// TestDeliverLazyTypedNoMonitors verifies the atomic fast path skips build.
func TestDeliverLazyTypedNoMonitors(t *testing.T) {
	mm := NewMonitorManager()

	called := 0
	mm.DeliverLazyTyped(
		events.LookupNamespaceID("bgp"),
		events.LookupEventTypeID(bgpevents.EventUpdate),
		events.DirReceived,
		"10.0.0.1", "",
		func() string {
			called++
			return `{"should":"not appear"}`
		},
	)

	assert.Equal(t, 0, called, "build must not run when no monitors are registered")
}

// TestDeliverLazyTypedNoMatch verifies build is skipped when monitors exist but none match.
func TestDeliverLazyTypedNoMatch(t *testing.T) {
	mm := NewMonitorManager()

	mc := NewMonitorClient(t.Context(), "state-only", []*Subscription{
		{Namespace: events.LookupNamespaceID("bgp"), EventType: events.LookupEventTypeID(bgpevents.EventState), Direction: events.DirBoth},
	}, 256)
	mm.Add(mc)

	called := 0
	mm.DeliverLazyTyped(
		events.LookupNamespaceID("bgp"),
		events.LookupEventTypeID(bgpevents.EventUpdate),
		events.DirReceived,
		"10.0.0.1", "",
		func() string {
			called++
			return `{"should":"not appear"}`
		},
	)

	assert.Equal(t, 0, called, "build must not run when no subscription matches")
	assert.Empty(t, mc.EventChan)
}

// TestDeliverLazyTypedMatch verifies build runs once and fans out to all matching monitors.
func TestDeliverLazyTypedMatch(t *testing.T) {
	mm := NewMonitorManager()
	ctx := t.Context()

	mc1 := NewMonitorClient(ctx, "one", []*Subscription{
		{Namespace: events.LookupNamespaceID("bgp"), EventType: events.LookupEventTypeID(bgpevents.EventUpdate), Direction: events.DirBoth},
	}, 256)
	mc2 := NewMonitorClient(ctx, "two", []*Subscription{
		{Namespace: events.LookupNamespaceID("bgp"), EventType: events.LookupEventTypeID(bgpevents.EventUpdate), Direction: events.DirBoth},
	}, 256)
	mm.Add(mc1)
	mm.Add(mc2)

	called := 0
	const payload = `{"typed":true}`
	mm.DeliverLazyTyped(
		events.LookupNamespaceID("bgp"),
		events.LookupEventTypeID(bgpevents.EventUpdate),
		events.DirReceived,
		"10.0.0.1", "",
		func() string {
			called++
			return payload
		},
	)

	assert.Equal(t, 1, called, "build must run exactly once even with two matching monitors")

	for _, mc := range []*MonitorClient{mc1, mc2} {
		select {
		case got := <-mc.EventChan:
			assert.Equal(t, payload, got)
		default:
			t.Fatalf("monitor %s did not receive event", mc.id)
		}
	}
}

// TestGetMatchingTyped verifies typed matching produces identical results to string matching.
func TestGetMatchingTyped(t *testing.T) {
	mm := NewMonitorManager()
	ctx := t.Context()

	mc1 := NewMonitorClient(ctx, "all-updates", []*Subscription{
		{Namespace: events.LookupNamespaceID("bgp"), EventType: events.LookupEventTypeID(bgpevents.EventUpdate), Direction: events.DirBoth},
	}, 256)
	mc2 := NewMonitorClient(ctx, "state-only", []*Subscription{
		{Namespace: events.LookupNamespaceID("bgp"), EventType: events.LookupEventTypeID(bgpevents.EventState), Direction: events.DirBoth},
	}, 256)
	mm.Add(mc1)
	mm.Add(mc2)

	stringResult := mm.GetMatching(bgpevents.Namespace, bgpevents.EventUpdate, events.DirectionReceived, "10.0.0.1", "")
	typedResult := mm.getMatchingTyped(
		events.LookupNamespaceID("bgp"),
		events.LookupEventTypeID(bgpevents.EventUpdate),
		events.DirReceived,
		"10.0.0.1", "",
	)

	assert.Equal(t, len(stringResult), len(typedResult), "typed and string matching must return the same count")
	assert.Equal(t, matchIDs(stringResult), matchIDs(typedResult), "typed and string matching must return the same monitors")
}

// TestHasMonitorsConcurrency verifies atomic count under concurrent Add/Remove.
func TestHasMonitorsConcurrency(t *testing.T) {
	mm := NewMonitorManager()
	ctx := t.Context()

	var wg sync.WaitGroup
	const goroutines = 50

	for i := range goroutines {
		wg.Go(func() {
			mc := NewMonitorClient(ctx, fmt.Sprintf("c-%d", i), []*Subscription{
				{Namespace: events.LookupNamespaceID("bgp"), EventType: events.LookupEventTypeID(bgpevents.EventUpdate), Direction: events.DirBoth},
			}, 256)
			mm.Add(mc)
		})
	}
	wg.Wait()

	assert.True(t, mm.HasMonitors())
	assert.Equal(t, goroutines, mm.Count())

	for i := range goroutines {
		wg.Go(func() {
			mm.Remove(fmt.Sprintf("c-%d", i))
		})
	}
	wg.Wait()

	assert.False(t, mm.HasMonitors())
	assert.Equal(t, 0, mm.Count())
}
