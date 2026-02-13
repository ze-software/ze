package plugin

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Subscription Parsing Tests
// =============================================================================

// TestParseSubscriptionBasic verifies basic subscription parsing.
//
// VALIDATES: Simple "bgp event update" parses correctly.
// PREVENTS: Basic subscription parsing failures.
func TestParseSubscriptionBasic(t *testing.T) {
	sub, err := ParseSubscription([]string{"bgp", "event", "update"})
	require.NoError(t, err)
	assert.Equal(t, "bgp", sub.Namespace)
	assert.Equal(t, "update", sub.EventType)
	assert.Equal(t, "both", sub.Direction)
	assert.Nil(t, sub.PeerFilter)
	assert.Empty(t, sub.PluginFilter)
}

// TestParseSubscriptionWithPeer verifies peer-filtered subscription.
//
// VALIDATES: "peer 10.0.0.1 bgp event update" extracts peer filter.
// PREVENTS: Peer filter not parsed.
func TestParseSubscriptionWithPeer(t *testing.T) {
	sub, err := ParseSubscription([]string{"peer", "10.0.0.1", "bgp", "event", "update"})
	require.NoError(t, err)
	assert.Equal(t, "bgp", sub.Namespace)
	assert.Equal(t, "update", sub.EventType)
	require.NotNil(t, sub.PeerFilter)
	assert.Equal(t, "10.0.0.1", sub.PeerFilter.Selector)
}

// TestParseSubscriptionWithPeerGlob verifies glob peer selector.
//
// VALIDATES: "peer * bgp event state" uses glob selector.
// PREVENTS: Glob selector rejected.
func TestParseSubscriptionWithPeerGlob(t *testing.T) {
	sub, err := ParseSubscription([]string{"peer", "*", "bgp", "event", "state"})
	require.NoError(t, err)
	require.NotNil(t, sub.PeerFilter)
	assert.Equal(t, "*", sub.PeerFilter.Selector)
}

// TestParseSubscriptionWithPeerExclude verifies exclusion selector.
//
// VALIDATES: "peer !10.0.0.1 bgp event update" uses exclusion.
// PREVENTS: Exclusion selector rejected.
func TestParseSubscriptionWithPeerExclude(t *testing.T) {
	sub, err := ParseSubscription([]string{"peer", "!10.0.0.1", "bgp", "event", "update"})
	require.NoError(t, err)
	require.NotNil(t, sub.PeerFilter)
	assert.Equal(t, "!10.0.0.1", sub.PeerFilter.Selector)
}

// TestParseSubscriptionWithPlugin verifies plugin-filtered subscription.
//
// VALIDATES: "plugin rib-cache rib event cache" extracts plugin filter.
// PREVENTS: Plugin filter not parsed.
func TestParseSubscriptionWithPlugin(t *testing.T) {
	sub, err := ParseSubscription([]string{"plugin", "rib-cache", "rib", "event", "cache"})
	require.NoError(t, err)
	assert.Equal(t, "rib", sub.Namespace)
	assert.Equal(t, "cache", sub.EventType)
	assert.Equal(t, "rib-cache", sub.PluginFilter)
	assert.Nil(t, sub.PeerFilter)
}

// TestParseSubscriptionWithDirection verifies direction filter.
//
// VALIDATES: "bgp event update direction received" extracts direction.
// PREVENTS: Direction filter not parsed.
func TestParseSubscriptionWithDirection(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		direction string
	}{
		{"received", []string{"bgp", "event", "update", "direction", "received"}, "received"},
		{"sent", []string{"bgp", "event", "update", "direction", "sent"}, "sent"},
		{"both", []string{"bgp", "event", "update", "direction", "both"}, "both"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sub, err := ParseSubscription(tt.args)
			require.NoError(t, err)
			assert.Equal(t, tt.direction, sub.Direction)
		})
	}
}

// TestParseSubscriptionFull verifies full subscription with all options.
//
// VALIDATES: "peer 10.0.0.1 bgp event update direction sent" parses fully.
// PREVENTS: Complex subscriptions failing.
func TestParseSubscriptionFull(t *testing.T) {
	sub, err := ParseSubscription([]string{"peer", "10.0.0.1", "bgp", "event", "update", "direction", "sent"})
	require.NoError(t, err)
	assert.Equal(t, "bgp", sub.Namespace)
	assert.Equal(t, "update", sub.EventType)
	assert.Equal(t, "sent", sub.Direction)
	require.NotNil(t, sub.PeerFilter)
	assert.Equal(t, "10.0.0.1", sub.PeerFilter.Selector)
}

// =============================================================================
// Subscription Validation Tests (Invalid Input)
// =============================================================================

// TestParseSubscriptionInvalidNamespace verifies unknown namespace rejected.
//
// VALIDATES: "bmp event update" fails.
// PREVENTS: Invalid namespace accepted.
func TestParseSubscriptionInvalidNamespace(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"bmp", []string{"bmp", "event", "update"}},
		{"rpki", []string{"rpki", "event", "update"}},
		{"empty", []string{"", "event", "update"}},
		{"BGP_case", []string{"BGP", "event", "update"}}, // Case sensitive
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseSubscription(tt.args)
			require.Error(t, err)
		})
	}
}

// TestParseSubscriptionInvalidDirection verifies invalid direction rejected.
//
// VALIDATES: "direction recv" fails (must be "received").
// PREVENTS: Invalid direction accepted.
func TestParseSubscriptionInvalidDirection(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"recv", []string{"bgp", "event", "update", "direction", "recv"}},
		{"send", []string{"bgp", "event", "update", "direction", "send"}},
		{"in", []string{"bgp", "event", "update", "direction", "in"}},
		{"out", []string{"bgp", "event", "update", "direction", "out"}},
		{"empty_after_keyword", []string{"bgp", "event", "update", "direction"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseSubscription(tt.args)
			require.Error(t, err)
		})
	}
}

// TestParseSubscriptionInvalidEventType verifies invalid event type rejected.
//
// VALIDATES: Unknown event types fail.
// PREVENTS: Invalid event type accepted.
func TestParseSubscriptionInvalidEventType(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"bgp_sent", []string{"bgp", "event", "sent"}},       // sent is direction, not event
		{"bgp_unknown", []string{"bgp", "event", "unknown"}}, // not a valid bgp event
		{"bgp_empty", []string{"bgp", "event"}},              // missing event type
		{"rib_update", []string{"rib", "event", "update"}},   // update is bgp, not rib
		{"rib_unknown", []string{"rib", "event", "unknown"}}, // not a valid rib event
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseSubscription(tt.args)
			require.Error(t, err)
		})
	}
}

// TestParseSubscriptionInvalidPeerSelector verifies invalid peer selector rejected.
//
// VALIDATES: Invalid peer selectors fail.
// PREVENTS: Malformed selector accepted.
func TestParseSubscriptionInvalidPeerSelector(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"double_glob", []string{"peer", "**", "bgp", "event", "update"}},
		{"double_exclude", []string{"peer", "!!10.0.0.1", "bgp", "event", "update"}},
		{"invalid_ip", []string{"peer", "999.999.999.999", "bgp", "event", "update"}},
		{"missing_selector", []string{"peer", "bgp", "event", "update"}}, // bgp looks like selector but isn't valid
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseSubscription(tt.args)
			require.Error(t, err)
		})
	}
}

// TestParseSubscriptionMissingKeyword verifies missing "event" keyword rejected.
//
// VALIDATES: "bgp update" fails (missing "event" keyword).
// PREVENTS: Command without keyword accepted.
func TestParseSubscriptionMissingKeyword(t *testing.T) {
	_, err := ParseSubscription([]string{"bgp", "update"})
	require.Error(t, err)
}

// =============================================================================
// Subscription Matching Tests
// =============================================================================

// TestSubscriptionMatches verifies subscription matching logic.
//
// VALIDATES: Matching works correctly for namespace/event/direction/peer.
// PREVENTS: Wrong events delivered to processes.
func TestSubscriptionMatches(t *testing.T) {
	tests := []struct {
		name      string
		sub       *Subscription
		namespace string
		eventType string
		direction string
		peer      string
		want      bool
	}{
		{
			name:      "exact_match",
			sub:       &Subscription{Namespace: "bgp", EventType: "update", Direction: "both"},
			namespace: "bgp", eventType: "update", direction: "received", peer: "10.0.0.1",
			want: true,
		},
		{
			name:      "direction_received_match",
			sub:       &Subscription{Namespace: "bgp", EventType: "update", Direction: "received"},
			namespace: "bgp", eventType: "update", direction: "received", peer: "10.0.0.1",
			want: true,
		},
		{
			name:      "direction_received_no_match",
			sub:       &Subscription{Namespace: "bgp", EventType: "update", Direction: "received"},
			namespace: "bgp", eventType: "update", direction: "sent", peer: "10.0.0.1",
			want: false,
		},
		{
			name:      "namespace_mismatch",
			sub:       &Subscription{Namespace: "bgp", EventType: "update", Direction: "both"},
			namespace: "rib", eventType: "update", direction: "received", peer: "10.0.0.1",
			want: false,
		},
		{
			name:      "event_mismatch",
			sub:       &Subscription{Namespace: "bgp", EventType: "update", Direction: "both"},
			namespace: "bgp", eventType: "state", direction: "received", peer: "10.0.0.1",
			want: false,
		},
		{
			name:      "peer_filter_match",
			sub:       &Subscription{Namespace: "bgp", EventType: "update", Direction: "both", PeerFilter: &PeerFilter{Selector: "10.0.0.1"}},
			namespace: "bgp", eventType: "update", direction: "received", peer: "10.0.0.1",
			want: true,
		},
		{
			name:      "peer_filter_no_match",
			sub:       &Subscription{Namespace: "bgp", EventType: "update", Direction: "both", PeerFilter: &PeerFilter{Selector: "10.0.0.1"}},
			namespace: "bgp", eventType: "update", direction: "received", peer: "10.0.0.2",
			want: false,
		},
		{
			name:      "peer_glob_match",
			sub:       &Subscription{Namespace: "bgp", EventType: "update", Direction: "both", PeerFilter: &PeerFilter{Selector: "*"}},
			namespace: "bgp", eventType: "update", direction: "received", peer: "10.0.0.1",
			want: true,
		},
		{
			name:      "peer_exclude_match",
			sub:       &Subscription{Namespace: "bgp", EventType: "update", Direction: "both", PeerFilter: &PeerFilter{Selector: "!10.0.0.1"}},
			namespace: "bgp", eventType: "update", direction: "received", peer: "10.0.0.2",
			want: true,
		},
		{
			name:      "peer_exclude_no_match",
			sub:       &Subscription{Namespace: "bgp", EventType: "update", Direction: "both", PeerFilter: &PeerFilter{Selector: "!10.0.0.1"}},
			namespace: "bgp", eventType: "update", direction: "received", peer: "10.0.0.1",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.sub.Matches(tt.namespace, tt.eventType, tt.direction, tt.peer)
			assert.Equal(t, tt.want, got)
		})
	}
}

// =============================================================================
// SubscriptionManager Tests
// =============================================================================

// TestSubscriptionManagerAddRemove verifies add/remove operations.
//
// VALIDATES: Subscriptions can be added and removed.
// PREVENTS: Memory leaks or incorrect tracking.
func TestSubscriptionManagerAddRemove(t *testing.T) {
	sm := NewSubscriptionManager()
	proc := NewProcess(PluginConfig{Name: "test"})

	sub := &Subscription{Namespace: "bgp", EventType: "update", Direction: "both"}

	// Add subscription
	sm.Add(proc, sub)
	assert.Equal(t, 1, sm.Count(proc))

	// Remove subscription
	removed := sm.Remove(proc, sub)
	assert.True(t, removed)
	assert.Equal(t, 0, sm.Count(proc))

	// Remove again should return false
	removed = sm.Remove(proc, sub)
	assert.False(t, removed)
}

// TestSubscriptionManagerGetMatching verifies event routing.
//
// VALIDATES: GetMatching returns processes with matching subscriptions.
// PREVENTS: Events sent to wrong processes.
func TestSubscriptionManagerGetMatching(t *testing.T) {
	sm := NewSubscriptionManager()

	proc1 := NewProcess(PluginConfig{Name: "proc1"})
	proc2 := NewProcess(PluginConfig{Name: "proc2"})
	proc3 := NewProcess(PluginConfig{Name: "proc3"})

	// proc1 subscribes to updates
	sm.Add(proc1, &Subscription{Namespace: "bgp", EventType: "update", Direction: "both"})

	// proc2 subscribes to state
	sm.Add(proc2, &Subscription{Namespace: "bgp", EventType: "state", Direction: "both"})

	// proc3 subscribes to updates from specific peer
	sm.Add(proc3, &Subscription{Namespace: "bgp", EventType: "update", Direction: "both", PeerFilter: &PeerFilter{Selector: "10.0.0.1"}})

	// Update from 10.0.0.1 should match proc1 and proc3
	matches := sm.GetMatching("bgp", "update", "received", "10.0.0.1")
	assert.Len(t, matches, 2)

	// Update from 10.0.0.2 should only match proc1
	matches = sm.GetMatching("bgp", "update", "received", "10.0.0.2")
	assert.Len(t, matches, 1)
	assert.Equal(t, "proc1", matches[0].Name())

	// State event should only match proc2
	matches = sm.GetMatching("bgp", "state", "", "10.0.0.1")
	assert.Len(t, matches, 1)
	assert.Equal(t, "proc2", matches[0].Name())
}

// TestSubscriptionManagerConcurrency verifies thread-safe operations.
//
// VALIDATES: Concurrent add/remove/get operations are safe.
// PREVENTS: Race conditions.
func TestSubscriptionManagerConcurrency(t *testing.T) {
	sm := NewSubscriptionManager()
	proc := NewProcess(PluginConfig{Name: "test"})

	var wg sync.WaitGroup
	const goroutines = 10
	const iterations = 100

	// Concurrent adds
	for range goroutines {
		wg.Go(func() {
			for range iterations {
				sm.Add(proc, &Subscription{
					Namespace: "bgp",
					EventType: "update",
					Direction: "both",
				})
			}
		})
	}

	// Concurrent reads
	for range goroutines {
		wg.Go(func() {
			for range iterations {
				_ = sm.GetMatching("bgp", "update", "received", "10.0.0.1")
			}
		})
	}

	wg.Wait()
	// No panic = success
}

// TestSubscriptionManagerClearProcess verifies cleanup on process death.
//
// VALIDATES: ClearProcess removes all subscriptions for a process.
// PREVENTS: Memory leaks when process dies.
func TestSubscriptionManagerClearProcess(t *testing.T) {
	sm := NewSubscriptionManager()
	proc := NewProcess(PluginConfig{Name: "test"})

	sm.Add(proc, &Subscription{Namespace: "bgp", EventType: "update", Direction: "both"})
	sm.Add(proc, &Subscription{Namespace: "bgp", EventType: "state", Direction: "both"})
	assert.Equal(t, 2, sm.Count(proc))

	sm.ClearProcess(proc)
	assert.Equal(t, 0, sm.Count(proc))
}

// =============================================================================
// Valid Event Types Tests
// =============================================================================

// TestValidBgpEventTypes verifies all valid BGP event types.
//
// VALIDATES: All documented BGP event types are accepted.
// PREVENTS: Valid event types being rejected.
func TestValidBgpEventTypes(t *testing.T) {
	validTypes := []string{"update", "open", "notification", "keepalive", "refresh", "state", "negotiated"}

	for _, eventType := range validTypes {
		t.Run(eventType, func(t *testing.T) {
			sub, err := ParseSubscription([]string{"bgp", "event", eventType})
			require.NoError(t, err)
			assert.Equal(t, eventType, sub.EventType)
		})
	}
}

// TestValidRibEventTypes verifies all valid RIB event types.
//
// VALIDATES: All documented RIB event types are accepted.
// PREVENTS: Valid event types being rejected.
func TestValidRibEventTypes(t *testing.T) {
	validTypes := []string{"cache", "route"}

	for _, eventType := range validTypes {
		t.Run(eventType, func(t *testing.T) {
			sub, err := ParseSubscription([]string{"rib", "event", eventType})
			require.NoError(t, err)
			assert.Equal(t, eventType, sub.EventType)
		})
	}
}
