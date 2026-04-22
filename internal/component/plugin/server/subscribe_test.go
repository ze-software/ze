package server

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bgpevents "codeberg.org/thomas-mangin/ze/internal/component/bgp/events"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/process"
	"codeberg.org/thomas-mangin/ze/internal/core/events"
)

// =============================================================================
// Subscription Parsing Tests
// =============================================================================

// TestParseSubscriptionBasic verifies basic subscription parsing.
//
// VALIDATES: Simple "event update" parses correctly.
// PREVENTS: Basic subscription parsing failures.
func TestParseSubscriptionBasic(t *testing.T) {
	sub, err := ParseSubscription([]string{"bgp", "event", "update"})
	require.NoError(t, err)
	assert.Equal(t, bgpevents.Namespace, sub.Namespace.String())
	assert.Equal(t, bgpevents.EventUpdate, sub.EventType.String())
	assert.Equal(t, events.DirBoth, sub.Direction)
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
	assert.Equal(t, bgpevents.Namespace, sub.Namespace.String())
	assert.Equal(t, bgpevents.EventUpdate, sub.EventType.String())
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
	sub, err := ParseSubscription([]string{"plugin", "rib-cache", "bgp-rib", "event", "cache"})
	require.NoError(t, err)
	assert.Equal(t, "bgp-rib", sub.Namespace.String())
	assert.Equal(t, "cache", sub.EventType.String())
	assert.Equal(t, "rib-cache", sub.PluginFilter)
	assert.Nil(t, sub.PeerFilter)
}

// TestParseSubscriptionWithDirection verifies direction filter.
//
// VALIDATES: "event update direction received" extracts direction.
// PREVENTS: Direction filter not parsed.
func TestParseSubscriptionWithDirection(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		direction events.Direction
	}{
		{"received", []string{"bgp", "event", "update", "direction", "received"}, events.DirReceived},
		{"sent", []string{"bgp", "event", "update", "direction", "sent"}, events.DirSent},
		{"both", []string{"bgp", "event", "update", "direction", "both"}, events.DirBoth},
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
	assert.Equal(t, bgpevents.Namespace, sub.Namespace.String())
	assert.Equal(t, bgpevents.EventUpdate, sub.EventType.String())
	assert.Equal(t, events.DirSent, sub.Direction)
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
		{"bgp_unknown", []string{"bgp", "event", "unknown"}},     // not a valid bgp event
		{"bgp_empty", []string{"bgp", "event"}},                  // missing event type
		{"rib_update", []string{"bgp-rib", "event", "update"}},   // update is bgp, not rib
		{"rib_unknown", []string{"bgp-rib", "event", "unknown"}}, // not a valid rib event
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
		{"double_exclude", []string{"peer", "!!10.0.0.1", "bgp", "event", "update"}},
		{"empty_exclusion", []string{"peer", "!", "bgp", "event", "update"}},
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
		namespace events.NamespaceID
		eventType events.EventTypeID
		direction events.Direction
		peer      string
		peerName  string
		want      bool
	}{
		{
			name:      "exact_match",
			sub:       &Subscription{Namespace: events.LookupNamespaceID(bgpevents.Namespace), EventType: events.LookupEventTypeID(bgpevents.EventUpdate), Direction: events.DirBoth},
			namespace: events.LookupNamespaceID("bgp"), eventType: events.LookupEventTypeID("update"), direction: events.DirReceived, peer: "10.0.0.1",
			want: true,
		},
		{
			name:      "direction_received_match",
			sub:       &Subscription{Namespace: events.LookupNamespaceID(bgpevents.Namespace), EventType: events.LookupEventTypeID(bgpevents.EventUpdate), Direction: events.DirReceived},
			namespace: events.LookupNamespaceID("bgp"), eventType: events.LookupEventTypeID("update"), direction: events.DirReceived, peer: "10.0.0.1",
			want: true,
		},
		{
			name:      "direction_received_no_match",
			sub:       &Subscription{Namespace: events.LookupNamespaceID(bgpevents.Namespace), EventType: events.LookupEventTypeID(bgpevents.EventUpdate), Direction: events.DirReceived},
			namespace: events.LookupNamespaceID("bgp"), eventType: events.LookupEventTypeID("update"), direction: events.DirSent, peer: "10.0.0.1",
			want: false,
		},
		{
			name:      "namespace_mismatch",
			sub:       &Subscription{Namespace: events.LookupNamespaceID(bgpevents.Namespace), EventType: events.LookupEventTypeID(bgpevents.EventUpdate), Direction: events.DirBoth},
			namespace: events.LookupNamespaceID("bgp-rib"), eventType: events.LookupEventTypeID("update"), direction: events.DirReceived, peer: "10.0.0.1",
			want: false,
		},
		{
			name:      "event_mismatch",
			sub:       &Subscription{Namespace: events.LookupNamespaceID(bgpevents.Namespace), EventType: events.LookupEventTypeID(bgpevents.EventUpdate), Direction: events.DirBoth},
			namespace: events.LookupNamespaceID("bgp"), eventType: events.LookupEventTypeID("state"), direction: events.DirReceived, peer: "10.0.0.1",
			want: false,
		},
		{
			name:      "peer_filter_match",
			sub:       &Subscription{Namespace: events.LookupNamespaceID(bgpevents.Namespace), EventType: events.LookupEventTypeID(bgpevents.EventUpdate), Direction: events.DirBoth, PeerFilter: &PeerFilter{Selector: "10.0.0.1"}},
			namespace: events.LookupNamespaceID("bgp"), eventType: events.LookupEventTypeID("update"), direction: events.DirReceived, peer: "10.0.0.1",
			want: true,
		},
		{
			name:      "peer_filter_no_match",
			sub:       &Subscription{Namespace: events.LookupNamespaceID(bgpevents.Namespace), EventType: events.LookupEventTypeID(bgpevents.EventUpdate), Direction: events.DirBoth, PeerFilter: &PeerFilter{Selector: "10.0.0.1"}},
			namespace: events.LookupNamespaceID("bgp"), eventType: events.LookupEventTypeID("update"), direction: events.DirReceived, peer: "10.0.0.2",
			want: false,
		},
		{
			name:      "peer_glob_match",
			sub:       &Subscription{Namespace: events.LookupNamespaceID(bgpevents.Namespace), EventType: events.LookupEventTypeID(bgpevents.EventUpdate), Direction: events.DirBoth, PeerFilter: &PeerFilter{Selector: "*"}},
			namespace: events.LookupNamespaceID("bgp"), eventType: events.LookupEventTypeID("update"), direction: events.DirReceived, peer: "10.0.0.1",
			want: true,
		},
		{
			name:      "peer_exclude_match",
			sub:       &Subscription{Namespace: events.LookupNamespaceID(bgpevents.Namespace), EventType: events.LookupEventTypeID(bgpevents.EventUpdate), Direction: events.DirBoth, PeerFilter: &PeerFilter{Selector: "!10.0.0.1"}},
			namespace: events.LookupNamespaceID("bgp"), eventType: events.LookupEventTypeID("update"), direction: events.DirReceived, peer: "10.0.0.2",
			want: true,
		},
		{
			name:      "peer_exclude_no_match",
			sub:       &Subscription{Namespace: events.LookupNamespaceID(bgpevents.Namespace), EventType: events.LookupEventTypeID(bgpevents.EventUpdate), Direction: events.DirBoth, PeerFilter: &PeerFilter{Selector: "!10.0.0.1"}},
			namespace: events.LookupNamespaceID("bgp"), eventType: events.LookupEventTypeID("update"), direction: events.DirReceived, peer: "10.0.0.1",
			want: false,
		},
		{
			name:      "peer_name_match",
			sub:       &Subscription{Namespace: events.LookupNamespaceID(bgpevents.Namespace), EventType: events.LookupEventTypeID(bgpevents.EventUpdate), Direction: events.DirBoth, PeerFilter: &PeerFilter{Selector: "upstream-1"}},
			namespace: events.LookupNamespaceID("bgp"), eventType: events.LookupEventTypeID("update"), direction: events.DirReceived, peer: "10.0.0.1", peerName: "upstream-1",
			want: true,
		},
		{
			name:      "peer_name_no_match",
			sub:       &Subscription{Namespace: events.LookupNamespaceID(bgpevents.Namespace), EventType: events.LookupEventTypeID(bgpevents.EventUpdate), Direction: events.DirBoth, PeerFilter: &PeerFilter{Selector: "upstream-1"}},
			namespace: events.LookupNamespaceID("bgp"), eventType: events.LookupEventTypeID("update"), direction: events.DirReceived, peer: "10.0.0.1", peerName: "downstream-2",
			want: false,
		},
		{
			name:      "peer_name_exclude_by_name",
			sub:       &Subscription{Namespace: events.LookupNamespaceID(bgpevents.Namespace), EventType: events.LookupEventTypeID(bgpevents.EventUpdate), Direction: events.DirBoth, PeerFilter: &PeerFilter{Selector: "!upstream-1"}},
			namespace: events.LookupNamespaceID("bgp"), eventType: events.LookupEventTypeID("update"), direction: events.DirReceived, peer: "10.0.0.1", peerName: "upstream-1",
			want: false,
		},
		{
			name:      "peer_name_exclude_passes",
			sub:       &Subscription{Namespace: events.LookupNamespaceID(bgpevents.Namespace), EventType: events.LookupEventTypeID(bgpevents.EventUpdate), Direction: events.DirBoth, PeerFilter: &PeerFilter{Selector: "!upstream-1"}},
			namespace: events.LookupNamespaceID("bgp"), eventType: events.LookupEventTypeID("update"), direction: events.DirReceived, peer: "10.0.0.1", peerName: "downstream-2",
			want: true,
		},
		{
			name:      "peer_ip_filter_matches_addr_not_name",
			sub:       &Subscription{Namespace: events.LookupNamespaceID(bgpevents.Namespace), EventType: events.LookupEventTypeID(bgpevents.EventUpdate), Direction: events.DirBoth, PeerFilter: &PeerFilter{Selector: "10.0.0.1"}},
			namespace: events.LookupNamespaceID("bgp"), eventType: events.LookupEventTypeID("update"), direction: events.DirReceived, peer: "10.0.0.1", peerName: "upstream-1",
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.sub.Matches(tt.namespace, tt.eventType, tt.direction, tt.peer, tt.peerName)
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
	proc := process.NewProcess(plugin.PluginConfig{Name: "test"})

	sub := &Subscription{Namespace: events.LookupNamespaceID(bgpevents.Namespace), EventType: events.LookupEventTypeID(bgpevents.EventUpdate), Direction: events.DirBoth}

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

	proc1 := process.NewProcess(plugin.PluginConfig{Name: "proc1"})
	proc2 := process.NewProcess(plugin.PluginConfig{Name: "proc2"})
	proc3 := process.NewProcess(plugin.PluginConfig{Name: "proc3"})

	// proc1 subscribes to updates
	sm.Add(proc1, &Subscription{Namespace: events.LookupNamespaceID(bgpevents.Namespace), EventType: events.LookupEventTypeID(bgpevents.EventUpdate), Direction: events.DirBoth})

	// proc2 subscribes to state
	sm.Add(proc2, &Subscription{Namespace: events.LookupNamespaceID(bgpevents.Namespace), EventType: events.LookupEventTypeID(bgpevents.EventState), Direction: events.DirBoth})

	// proc3 subscribes to updates from specific peer
	sm.Add(proc3, &Subscription{Namespace: events.LookupNamespaceID(bgpevents.Namespace), EventType: events.LookupEventTypeID(bgpevents.EventUpdate), Direction: events.DirBoth, PeerFilter: &PeerFilter{Selector: "10.0.0.1"}})

	// Update from 10.0.0.1 should match proc1 and proc3
	matches := sm.GetMatching(events.LookupNamespaceID(bgpevents.Namespace), events.LookupEventTypeID("update"), events.DirReceived, "10.0.0.1", "")
	assert.Len(t, matches, 2)

	// Update from 10.0.0.2 should only match proc1
	matches = sm.GetMatching(events.LookupNamespaceID(bgpevents.Namespace), events.LookupEventTypeID("update"), events.DirReceived, "10.0.0.2", "")
	assert.Len(t, matches, 1)
	assert.Equal(t, "proc1", matches[0].Name())

	// State event should only match proc2
	matches = sm.GetMatching(events.LookupNamespaceID(bgpevents.Namespace), events.LookupEventTypeID("state"), events.DirUnspecified, "10.0.0.1", "")
	assert.Len(t, matches, 1)
	assert.Equal(t, "proc2", matches[0].Name())
}

// TestSubscriptionManagerConcurrency verifies thread-safe operations.
//
// VALIDATES: Concurrent add/remove/get operations are safe.
// PREVENTS: Race conditions.
func TestSubscriptionManagerConcurrency(t *testing.T) {
	sm := NewSubscriptionManager()
	proc := process.NewProcess(plugin.PluginConfig{Name: "test"})

	var wg sync.WaitGroup
	const goroutines = 10
	const iterations = 100

	// Concurrent adds
	for range goroutines {
		wg.Go(func() {
			for range iterations {
				sm.Add(proc, &Subscription{
					Namespace: events.LookupNamespaceID(bgpevents.Namespace),
					EventType: events.LookupEventTypeID(bgpevents.EventUpdate),
					Direction: events.DirBoth,
				})
			}
		})
	}

	// Concurrent reads
	for range goroutines {
		wg.Go(func() {
			for range iterations {
				_ = sm.GetMatching(events.LookupNamespaceID(bgpevents.Namespace), events.LookupEventTypeID("update"), events.DirReceived, "10.0.0.1", "")
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
	proc := process.NewProcess(plugin.PluginConfig{Name: "test"})

	sm.Add(proc, &Subscription{Namespace: events.LookupNamespaceID(bgpevents.Namespace), EventType: events.LookupEventTypeID(bgpevents.EventUpdate), Direction: events.DirBoth})
	sm.Add(proc, &Subscription{Namespace: events.LookupNamespaceID(bgpevents.Namespace), EventType: events.LookupEventTypeID(bgpevents.EventState), Direction: events.DirBoth})
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
			assert.Equal(t, eventType, sub.EventType.String())
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
			sub, err := ParseSubscription([]string{"bgp-rib", "event", eventType})
			require.NoError(t, err)
			assert.Equal(t, eventType, sub.EventType.String())
		})
	}
}
