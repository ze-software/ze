package inprocess

import (
	"context"
	"fmt"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/chaos/peer"
	"codeberg.org/thomas-mangin/ze/internal/chaos/scenario"
	"codeberg.org/thomas-mangin/ze/internal/chaos/shrink"
	"codeberg.org/thomas-mangin/ze/internal/chaos/validation"
	_ "codeberg.org/thomas-mangin/ze/internal/component/plugin/all" // register YANG schemas + plugins
)

// eventTypeCounts returns a map of event type → count for determinism comparison.
func eventTypeCounts(events []peer.Event) map[peer.EventType]int {
	m := make(map[peer.EventType]int)
	for i := range events {
		m[events[i].Type]++
	}
	return m
}

// TestInProcessBasicRoute verifies a 2-peer in-process scenario where
// both peers establish BGP sessions and peer 0 announces routes.
//
// VALIDATES: Full pipeline: config → reactor → mock connections → peer simulators → session establishment → route sending.
// PREVENTS: Broken in-process wiring (wrong addresses, mock connections not accepted, sessions not establishing).
//
// NOTE: RR route forwarding is not tested here because the RR plugin has a
// pre-existing JSON format mismatch (expects ExaBGP-style events, receives
// ze-bgp envelope). That needs a separate fix in the RR plugin.
func TestInProcessBasicRoute(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	profiles := []scenario.PeerProfile{
		{
			Index:      0,
			ASN:        65000,
			RouterID:   netip.MustParseAddr("10.0.0.2"),
			IsIBGP:     true,
			RouteCount: 5,
			HoldTime:   90,
			Families:   []string{"ipv4/unicast"},
		},
		{
			Index:      1,
			ASN:        65000,
			RouterID:   netip.MustParseAddr("10.0.0.3"),
			IsIBGP:     true,
			RouteCount: 0,
			HoldTime:   90,
			Families:   []string{"ipv4/unicast"},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := Run(ctx, RunConfig{
		Profiles:  profiles,
		Seed:      42,
		Duration:  10 * time.Second,
		LocalAS:   65000,
		RouterID:  netip.MustParseAddr("10.0.0.1"),
		LocalAddr: "127.0.0.1",
	})
	require.NoError(t, err)

	// Both peers should establish BGP sessions.
	established := map[int]bool{}
	var routesSentByPeer0 int
	for _, ev := range result.Events {
		if ev.Type == peer.EventEstablished {
			established[ev.PeerIndex] = true
		}
		if ev.PeerIndex == 0 && ev.Type == peer.EventRouteSent {
			routesSentByPeer0++
		}
	}
	assert.True(t, established[0], "peer 0 should establish BGP session")
	assert.True(t, established[1], "peer 1 should establish BGP session")
	assert.Greater(t, routesSentByPeer0, 0, "peer 0 should send routes to reactor")
}

// TestInProcessHoldTimerExpiry verifies that advancing VirtualClock past the
// hold time without keepalives causes the reactor to tear down the session.
//
// VALIDATES: VirtualClock drives reactor FSM timers; hold-timer expiry detected.
// PREVENTS: Virtual time not reaching FSM timers, sessions staying up when they shouldn't.
func TestInProcessHoldTimerExpiry(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Single peer with short hold time. The runner will stop keepalives
	// and advance the clock past the hold time.
	profiles := []scenario.PeerProfile{
		{
			Index:      0,
			ASN:        65000,
			RouterID:   netip.MustParseAddr("10.0.0.2"),
			IsIBGP:     true,
			RouteCount: 1,
			HoldTime:   30,
			Families:   []string{"ipv4/unicast"},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := Run(ctx, RunConfig{
		Profiles:         profiles,
		Seed:             42,
		Duration:         60 * time.Second,
		LocalAS:          65000,
		RouterID:         netip.MustParseAddr("10.0.0.1"),
		LocalAddr:        "127.0.0.1",
		StopKeepalivesAt: 5 * time.Second, // Stop keepalives after 5s virtual time.
	})
	require.NoError(t, err)

	// Should see a disconnect event from hold timer expiry.
	var disconnects int
	for _, ev := range result.Events {
		if ev.PeerIndex == 0 && ev.Type == peer.EventDisconnected {
			disconnects++
		}
	}
	assert.Greater(t, disconnects, 0, "peer should be disconnected after hold-timer expiry")
}

// TestInProcessEventLogFormat verifies that in-process events have the same
// structure as external mode events (type, peer index, time).
//
// VALIDATES: Event format compatibility between in-process and external modes.
// PREVENTS: In-process mode producing events that break the validation pipeline.
func TestInProcessEventLogFormat(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	profiles := []scenario.PeerProfile{
		{
			Index:      0,
			ASN:        65000,
			RouterID:   netip.MustParseAddr("10.0.0.2"),
			IsIBGP:     true,
			RouteCount: 1,
			HoldTime:   90,
			Families:   []string{"ipv4/unicast"},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := Run(ctx, RunConfig{
		Profiles:  profiles,
		Seed:      42,
		Duration:  30 * time.Second,
		LocalAS:   65000,
		RouterID:  netip.MustParseAddr("10.0.0.1"),
		LocalAddr: "127.0.0.1",
	})
	require.NoError(t, err)

	// Must have at least an Established event and route events.
	var hasEstablished, hasRouteSent bool
	for _, ev := range result.Events {
		assert.False(t, ev.Time.IsZero(), "events must have non-zero timestamps")
		switch ev.Type { //nolint:exhaustive // Only checking specific event types
		case peer.EventEstablished:
			hasEstablished = true
		case peer.EventRouteSent:
			hasRouteSent = true
		}
	}
	assert.True(t, hasEstablished, "should have EventEstablished")
	assert.True(t, hasRouteSent, "should have EventRouteSent")
}

// TestInProcessSpeed verifies that a 4-peer 30s scenario completes in
// under 10 seconds wall-clock time (at least 3x speedup).
//
// VALIDATES: In-process mode provides significant speedup over real time.
// PREVENTS: In-process mode accidentally using real time for all timers.
func TestInProcessSpeed(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	profiles := make([]scenario.PeerProfile, 4)
	for i := range profiles {
		profiles[i] = scenario.PeerProfile{
			Index:      i,
			ASN:        65000,
			RouterID:   netip.MustParseAddr(fmt.Sprintf("10.0.0.%d", 2+i)),
			IsIBGP:     true,
			RouteCount: 10,
			HoldTime:   90,
			Families:   []string{"ipv4/unicast"},
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	start := time.Now()
	_, err := Run(ctx, RunConfig{
		Profiles:  profiles,
		Seed:      42,
		Duration:  30 * time.Second,
		LocalAS:   65000,
		RouterID:  netip.MustParseAddr("10.0.0.1"),
		LocalAddr: "127.0.0.1",
	})
	elapsed := time.Since(start)
	require.NoError(t, err)

	// 30s simulated in under 10s wall-clock = at least 3x speedup.
	// The 10ms real-time pauses per 1s virtual step add ~300ms for 30 steps,
	// plus 500ms handshake wait and reactor startup overhead.
	assert.Less(t, elapsed, 10*time.Second, "in-process 30s scenario should complete in <10s")
}

// TestInProcessDisconnectReconnect verifies mock connection lifecycle:
// closing a connection triggers disconnect detection, and a new connection
// may re-establish depending on timing relative to the reactor's reconnect
// backoff (DefaultReconnectMin = 5s virtual).
//
// VALIDATES: Disconnect detection, collision rejection (short gap), re-establishment (long gap).
// PREVENTS: Reactor failing to detect closed connections or accept new ones.
func TestInProcessDisconnectReconnect(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tests := []struct {
		name           string
		reconnectDelay time.Duration
		duration       time.Duration
		// minEstablished/maxEstablished define the acceptable range.
		// Short gap: collision likely → 1 established.
		// Long gap: re-establishment expected → 2 established.
		// Borderline: either outcome acceptable.
		minEstablished int
		maxEstablished int
		// minDisconnected is the expected minimum clean-shutdown events.
		// Simulators emit "disconnected" on context cancellation, not on
		// connection errors. Short gap: both sims exit via error → 0.
		// Long gap: second sim runs to completion → 1 from final cancel.
		minDisconnected int
	}{
		{
			name:            "short_gap_collision",
			reconnectDelay:  0,                // Collision mode: queue new conn before closing old.
			duration:        15 * time.Second, // Session is ESTABLISHED when new conn arrives.
			minEstablished:  1,                // Initial session only — collision rejects the new one.
			maxEstablished:  1,                // RFC 4271 §6.8: ESTABLISHED state rejects incoming.
			minDisconnected: 0,                // Disconnect events vary by timing.
		},
		{
			name:            "borderline_gap",
			reconnectDelay:  5 * time.Second,
			duration:        20 * time.Second,
			minEstablished:  1, // may or may not succeed
			maxEstablished:  2,
			minDisconnected: 0, // may or may not get clean shutdown
		},
		{
			name:            "long_gap_reestablish",
			reconnectDelay:  10 * time.Second,
			duration:        25 * time.Second,
			minEstablished:  2, // peer has fully recycled
			maxEstablished:  2,
			minDisconnected: 1, // second sim exits cleanly via context cancel
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			profiles := []scenario.PeerProfile{
				{
					Index:      0,
					ASN:        65000,
					RouterID:   netip.MustParseAddr("10.0.0.2"),
					IsIBGP:     true,
					RouteCount: 1,
					HoldTime:   90,
					Families:   []string{"ipv4/unicast"},
				},
			}

			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			result, err := Run(ctx, RunConfig{
				Profiles:       profiles,
				Seed:           42,
				Duration:       tt.duration,
				LocalAS:        65000,
				RouterID:       netip.MustParseAddr("10.0.0.1"),
				LocalAddr:      "127.0.0.1",
				DisconnectAt:   3 * time.Second,
				ReconnectDelay: tt.reconnectDelay,
			})
			require.NoError(t, err)

			var established, disconnected int
			for _, ev := range result.Events {
				if ev.PeerIndex == 0 {
					switch ev.Type { //nolint:exhaustive // Only checking specific event types
					case peer.EventEstablished:
						established++
					case peer.EventDisconnected:
						disconnected++
					}
				}
			}

			assert.GreaterOrEqual(t, disconnected, tt.minDisconnected, "peer should have at least %d disconnected events", tt.minDisconnected)
			assert.GreaterOrEqual(t, established, tt.minEstablished,
				"expected at least %d established events", tt.minEstablished)
			assert.LessOrEqual(t, established, tt.maxEstablished,
				"expected at most %d established events", tt.maxEstablished)
		})
	}
}

// TestInProcessDeterminism verifies that two runs with the same seed
// produce the same seed-controlled events (established, routes).
//
// VALIDATES: Seed-controlled execution produces consistent results.
// PREVENTS: Non-determinism from leaking into route generation.
//
// NOTE: Only compares seed-controlled event types (Established, RouteSent,
// EORSent). Timing-dependent events like Disconnected and Error depend on
// goroutine scheduling at shutdown, not the seed.
func TestInProcessDeterminism(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	cfg := RunConfig{
		Profiles: []scenario.PeerProfile{
			{
				Index:      0,
				ASN:        65000,
				RouterID:   netip.MustParseAddr("10.0.0.2"),
				IsIBGP:     true,
				RouteCount: 3,
				HoldTime:   90,
				Families:   []string{"ipv4/unicast"},
			},
			{
				Index:      1,
				ASN:        65000,
				RouterID:   netip.MustParseAddr("10.0.0.3"),
				IsIBGP:     true,
				RouteCount: 0,
				HoldTime:   90,
				Families:   []string{"ipv4/unicast"},
			},
		},
		Seed:      42,
		Duration:  30 * time.Second, // 30 virtual steps × 10ms = 300ms real time for consistent event production.
		LocalAS:   65000,
		RouterID:  netip.MustParseAddr("10.0.0.1"),
		LocalAddr: "127.0.0.1",
	}

	// Run twice with the same config.
	ctx1, cancel1 := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel1()
	result1, err := Run(ctx1, cfg)
	require.NoError(t, err)

	ctx2, cancel2 := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel2()
	result2, err := Run(ctx2, cfg)
	require.NoError(t, err)

	// Compare only seed-controlled event types. Timing-dependent events
	// (Disconnected, Error) vary with goroutine scheduling at shutdown.
	seedControlled := []peer.EventType{
		peer.EventEstablished,
		peer.EventRouteSent,
		peer.EventEORSent,
	}
	for _, et := range seedControlled {
		c1 := eventTypeCounts(result1.Events)[et]
		c2 := eventTypeCounts(result2.Events)[et]
		assert.Equal(t, c1, c2, "event type %v count should match between runs (got %d vs %d)", et, c1, c2)
	}
}

// TestInProcessProperties verifies that RFC properties pass for a correct
// in-process scenario (no chaos, all peers establish cleanly).
//
// VALIDATES: Property engine processes in-process events correctly.
// PREVENTS: In-process events having wrong format for property validation.
//
// NOTE: Uses RouteCount=0 because RouteConsistency and ConvergenceDeadline
// require RR route forwarding, which is blocked by pre-existing RR format
// mismatch (see docs/plan/spec-rr-event-format.md).
func TestInProcessProperties(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	numPeers := 3
	profiles := make([]scenario.PeerProfile, numPeers)
	for i := range profiles {
		profiles[i] = scenario.PeerProfile{
			Index:      i,
			ASN:        65000,
			RouterID:   netip.MustParseAddr(fmt.Sprintf("10.0.0.%d", 2+i)),
			IsIBGP:     true,
			RouteCount: 0, // No routes — avoids RouteConsistency violations from missing RR forwarding.
			HoldTime:   90,
			Families:   []string{"ipv4/unicast"},
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := Run(ctx, RunConfig{
		Profiles:  profiles,
		Seed:      42,
		Duration:  5 * time.Second,
		LocalAS:   65000,
		RouterID:  netip.MustParseAddr("10.0.0.1"),
		LocalAddr: "127.0.0.1",
	})
	require.NoError(t, err)

	// Feed events to property engine with all Phase 7 properties.
	engine := validation.NewPropertyEngine(
		validation.AllProperties(numPeers, 10*time.Second),
	)
	for _, ev := range result.Events {
		engine.ProcessEvent(ev)
	}

	violations := engine.AllViolations()
	assert.Empty(t, violations, "no property violations expected for correct scenario")
}

// TestInProcessScale20 verifies that 20 peers can establish sessions
// without deadlock, goroutine leak, or resource exhaustion.
//
// VALIDATES: In-process mode scales to 20 concurrent peers.
// PREVENTS: Resource exhaustion, deadlocks, goroutine leaks at scale.
func TestInProcessScale20(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	numPeers := 20
	profiles := make([]scenario.PeerProfile, numPeers)
	for i := range profiles {
		profiles[i] = scenario.PeerProfile{
			Index:      i,
			ASN:        65000,
			RouterID:   netip.MustParseAddr(fmt.Sprintf("10.0.0.%d", 2+i)),
			IsIBGP:     true,
			RouteCount: 0, // No routes — focus on session management at scale.
			HoldTime:   90,
			Families:   []string{"ipv4/unicast"},
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	result, err := Run(ctx, RunConfig{
		Profiles:  profiles,
		Seed:      42,
		Duration:  30 * time.Second, // 30 virtual steps × 10ms = 300ms real time for late establishments.
		LocalAS:   65000,
		RouterID:  netip.MustParseAddr("10.0.0.1"),
		LocalAddr: "127.0.0.1",
	})
	require.NoError(t, err)

	// Count how many peers established.
	established := map[int]bool{}
	for _, ev := range result.Events {
		if ev.Type == peer.EventEstablished {
			established[ev.PeerIndex] = true
		}
	}
	// At least 80% of peers should establish (some may fail due to timing).
	// The scaled handshake wait in Run() gives enough time even under -race,
	// and Duration=30s provides 300ms of additional real time in the virtual
	// loop for any peers that establish slightly after the handshake wait.
	minExpected := numPeers * 80 / 100
	assert.GreaterOrEqual(t, len(established), minExpected,
		"at least %d of %d peers should establish sessions", minExpected, numPeers)
}

// TestInProcessShrinkCompat verifies that events from in-process mode
// are compatible with the shrink pipeline.
//
// VALIDATES: In-process events have correct format for shrink processing.
// PREVENTS: Format incompatibility between in-process events and shrink.
func TestInProcessShrinkCompat(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	profiles := []scenario.PeerProfile{
		{
			Index:      0,
			ASN:        65000,
			RouterID:   netip.MustParseAddr("10.0.0.2"),
			IsIBGP:     true,
			RouteCount: 3,
			HoldTime:   90,
			Families:   []string{"ipv4/unicast"},
		},
		{
			Index:      1,
			ASN:        65000,
			RouterID:   netip.MustParseAddr("10.0.0.3"),
			IsIBGP:     true,
			RouteCount: 0,
			HoldTime:   90,
			Families:   []string{"ipv4/unicast"},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := Run(ctx, RunConfig{
		Profiles:  profiles,
		Seed:      42,
		Duration:  5 * time.Second,
		LocalAS:   65000,
		RouterID:  netip.MustParseAddr("10.0.0.1"),
		LocalAddr: "127.0.0.1",
	})
	require.NoError(t, err)
	require.NotEmpty(t, result.Events, "should have events to test")

	// Run shrink on the events. For a passing scenario, shrink should
	// report "no violation found" — proving events are format-compatible.
	start := time.Now()
	_, shrinkErr := shrink.Run(result.Events, shrink.Config{
		PeerCount: 2,
		Deadline:  10 * time.Second,
	})
	elapsed := time.Since(start)

	// Shrink should report no violation (correct scenario).
	require.Error(t, shrinkErr, "shrink should report no violation for passing scenario")
	assert.Contains(t, shrinkErr.Error(), "no violation", "error should indicate no violation found")

	// Shrink should complete near-instantly for a passing scenario.
	assert.Less(t, elapsed, 1*time.Second, "shrink should complete in <1s")
}
