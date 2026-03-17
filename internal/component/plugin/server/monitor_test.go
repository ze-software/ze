package server

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
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
		{Namespace: plugin.NamespaceBGP, EventType: plugin.EventUpdate, Direction: plugin.DirectionBoth},
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
		{Namespace: plugin.NamespaceBGP, EventType: plugin.EventUpdate, Direction: plugin.DirectionBoth},
	}, 256)

	// Monitor 2: subscribes to state events only.
	mc2 := NewMonitorClient(ctx, "state-only", []*Subscription{
		{Namespace: plugin.NamespaceBGP, EventType: plugin.EventState, Direction: plugin.DirectionBoth},
	}, 256)

	// Monitor 3: subscribes to updates from specific peer.
	mc3 := NewMonitorClient(ctx, "peer-updates", []*Subscription{
		{Namespace: plugin.NamespaceBGP, EventType: plugin.EventUpdate, Direction: plugin.DirectionBoth,
			PeerFilter: &PeerFilter{Selector: "10.0.0.1"}},
	}, 256)

	mm.Add(mc1)
	mm.Add(mc2)
	mm.Add(mc3)

	// Update from 10.0.0.1 should match mc1 and mc3.
	matches := mm.GetMatching(plugin.NamespaceBGP, plugin.EventUpdate, plugin.DirectionReceived, "10.0.0.1")
	assert.Len(t, matches, 2)
	ids := matchIDs(matches)
	assert.Contains(t, ids, "all-updates")
	assert.Contains(t, ids, "peer-updates")

	// Update from 10.0.0.2 should match mc1 only.
	matches = mm.GetMatching(plugin.NamespaceBGP, plugin.EventUpdate, plugin.DirectionReceived, "10.0.0.2")
	assert.Len(t, matches, 1)
	assert.Equal(t, "all-updates", matches[0].id)

	// State event should match mc2 only.
	matches = mm.GetMatching(plugin.NamespaceBGP, plugin.EventState, "", "10.0.0.1")
	assert.Len(t, matches, 1)
	assert.Equal(t, "state-only", matches[0].id)

	// Notification event should match nobody.
	matches = mm.GetMatching(plugin.NamespaceBGP, plugin.EventNotification, plugin.DirectionReceived, "10.0.0.1")
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
		{Namespace: plugin.NamespaceBGP, EventType: plugin.EventUpdate, Direction: plugin.DirectionBoth},
	}, 256)

	mm.Add(mc)
	matches := mm.GetMatching(plugin.NamespaceBGP, plugin.EventUpdate, plugin.DirectionReceived, "10.0.0.1")
	require.Len(t, matches, 1)

	mm.Remove("cleanup-test")
	matches = mm.GetMatching(plugin.NamespaceBGP, plugin.EventUpdate, plugin.DirectionReceived, "10.0.0.1")
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
		{Namespace: plugin.NamespaceBGP, EventType: plugin.EventUpdate, Direction: plugin.DirectionBoth},
	}, 256)

	mc2 := NewMonitorClient(ctx, "non-receiver", []*Subscription{
		{Namespace: plugin.NamespaceBGP, EventType: plugin.EventState, Direction: plugin.DirectionBoth},
	}, 256)

	mm.Add(mc1)
	mm.Add(mc2)

	testEvent := `{"type":"bgp","bgp":{"message":{"type":"update"}}}`
	mm.Deliver(plugin.NamespaceBGP, plugin.EventUpdate, plugin.DirectionReceived, "10.0.0.1", testEvent)

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
		{Namespace: plugin.NamespaceBGP, EventType: plugin.EventUpdate, Direction: plugin.DirectionBoth},
	}, 2) // tiny buffer

	mm.Add(mc)

	// Send 5 events; buffer holds 2, so 3 should be dropped.
	for i := range 5 {
		mm.Deliver(plugin.NamespaceBGP, plugin.EventUpdate, plugin.DirectionReceived, "10.0.0.1",
			fmt.Sprintf(`{"event":%d}`, i))
	}

	assert.Equal(t, uint64(3), mc.Dropped.Load(), "expected 3 dropped events")

	// The 2 buffered events should still be readable.
	assert.Len(t, mc.EventChan, 2)
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
				{Namespace: plugin.NamespaceBGP, EventType: plugin.EventUpdate, Direction: plugin.DirectionBoth},
			}, 256)
			mm.Add(mc)
		})
	}
	wg.Wait()

	// Concurrent reads + delivers.
	for range goroutines {
		wg.Go(func() {
			for range 100 {
				_ = mm.GetMatching(plugin.NamespaceBGP, plugin.EventUpdate, plugin.DirectionReceived, "10.0.0.1")
				mm.Deliver(plugin.NamespaceBGP, plugin.EventUpdate, plugin.DirectionReceived, "10.0.0.1", `{"test":true}`)
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
