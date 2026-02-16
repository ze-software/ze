package shrink

import (
	"bytes"
	"fmt"
	"net/netip"
	"testing"
	"time"

	"codeberg.org/thomas-mangin/ze/cmd/ze-bgp-chaos/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var defaultCfg = Config{PeerCount: 2, Deadline: 5 * time.Second}

// TestShrinkAlreadyMinimal verifies that a 3-event minimal failure is returned unchanged.
//
// VALIDATES: Already-minimal input is not shrunk further.
// PREVENTS: Over-shrinking that loses the violation.
func TestShrinkAlreadyMinimal(t *testing.T) {
	// route-consistency: peer 0 announces, peer 1 never receives → missing route.
	events := []peer.Event{
		{Type: peer.EventEstablished, PeerIndex: 0, Time: t0},
		{Type: peer.EventEstablished, PeerIndex: 1, Time: t0},
		{Type: peer.EventRouteSent, PeerIndex: 0, Prefix: prefix1, Time: t1},
	}

	result, err := Run(events, defaultCfg)
	require.NoError(t, err)
	assert.Equal(t, "route-consistency", result.Property)
	assert.Len(t, result.Events, 3, "already minimal — nothing to remove")
	assert.Equal(t, 3, result.Original)
}

// TestShrinkRemovesUnnecessary verifies that extra events are eliminated.
//
// VALIDATES: Shrinking removes events not needed for the violation.
// PREVENTS: Shrink engine keeping unnecessary events.
func TestShrinkRemovesUnnecessary(t *testing.T) {
	// Minimal violation needs: Established(0), Established(1), RouteSent(0, P).
	// Extra events: RouteSent(0, prefix2), RouteSent(1, prefix2).
	events := []peer.Event{
		{Type: peer.EventEstablished, PeerIndex: 0, Time: t0},
		{Type: peer.EventEstablished, PeerIndex: 1, Time: t0},
		{Type: peer.EventRouteSent, PeerIndex: 0, Prefix: prefix1, Time: t1},
		{Type: peer.EventRouteSent, PeerIndex: 0, Prefix: prefix2, Time: t2}, // extra
		{Type: peer.EventRouteSent, PeerIndex: 1, Prefix: prefix2, Time: t3}, // extra
	}

	result, err := Run(events, defaultCfg)
	require.NoError(t, err)
	assert.Equal(t, "route-consistency", result.Property)
	assert.Less(t, len(result.Events), 5, "should have removed at least one event")
	assert.Equal(t, 5, result.Original)
}

// TestShrinkNoViolation verifies error when the input log has no violation.
//
// VALIDATES: Non-failing input produces descriptive error.
// PREVENTS: Silent success on non-failing input.
func TestShrinkNoViolation(t *testing.T) {
	// All routes received — no violation.
	events := []peer.Event{
		{Type: peer.EventEstablished, PeerIndex: 0, Time: t0},
		{Type: peer.EventEstablished, PeerIndex: 1, Time: t0},
		{Type: peer.EventRouteSent, PeerIndex: 0, Prefix: prefix1, Time: t1},
		{Type: peer.EventRouteReceived, PeerIndex: 1, Prefix: prefix1, Time: t2},
	}

	_, err := Run(events, defaultCfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no violation")
}

// TestShrinkEmpty verifies error on empty event list.
//
// VALIDATES: Empty input produces descriptive error.
// PREVENTS: Panic on nil/empty slice.
func TestShrinkEmpty(t *testing.T) {
	_, err := Run(nil, defaultCfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no events")
}

// TestShrinkPreservesViolation verifies that the shrunk result still triggers
// the same property violation.
//
// VALIDATES: Shrunk result reproduces the original violation.
// PREVENTS: Shrinking that loses the failure.
func TestShrinkPreservesViolation(t *testing.T) {
	events := []peer.Event{
		{Type: peer.EventEstablished, PeerIndex: 0, Time: t0},
		{Type: peer.EventEstablished, PeerIndex: 1, Time: t0},
		{Type: peer.EventRouteSent, PeerIndex: 0, Prefix: prefix1, Time: t1},
		{Type: peer.EventRouteSent, PeerIndex: 0, Prefix: prefix2, Time: t2},
		{Type: peer.EventRouteSent, PeerIndex: 1, Prefix: prefix1, Time: t2},
		{Type: peer.EventRouteSent, PeerIndex: 1, Prefix: prefix2, Time: t3},
	}

	result, err := Run(events, defaultCfg)
	require.NoError(t, err)

	// Verify the shrunk result still triggers the violation.
	assert.True(t, hasViolation(result.Events, result.Property, defaultCfg),
		"shrunk result must still trigger the violation")
}

// TestShrinkDeterministic verifies that the same input produces the same output.
//
// VALIDATES: Shrinking is deterministic (no randomness).
// PREVENTS: Non-reproducible shrink results.
func TestShrinkDeterministic(t *testing.T) {
	events := []peer.Event{
		{Type: peer.EventEstablished, PeerIndex: 0, Time: t0},
		{Type: peer.EventEstablished, PeerIndex: 1, Time: t0},
		{Type: peer.EventRouteSent, PeerIndex: 0, Prefix: prefix1, Time: t1},
		{Type: peer.EventRouteSent, PeerIndex: 0, Prefix: prefix2, Time: t2},
	}

	r1, err1 := Run(events, defaultCfg)
	require.NoError(t, err1)

	r2, err2 := Run(events, defaultCfg)
	require.NoError(t, err2)

	assert.Equal(t, r1.Events, r2.Events, "same input should produce identical output")
	assert.Equal(t, r1.Property, r2.Property)
}

// TestShrinkMessageOrdering verifies shrinking with a message-ordering violation.
//
// VALIDATES: Shrink works with different property types.
// PREVENTS: Shrink engine only working with route-consistency.
func TestShrinkMessageOrdering(t *testing.T) {
	// RouteSent before Established → message-ordering violation.
	// Receives satisfy route-consistency so only message-ordering fires.
	events := []peer.Event{
		{Type: peer.EventRouteSent, PeerIndex: 0, Prefix: prefix1, Time: t0},
		{Type: peer.EventEstablished, PeerIndex: 0, Time: t1},
		{Type: peer.EventEstablished, PeerIndex: 1, Time: t1},
		{Type: peer.EventRouteSent, PeerIndex: 0, Prefix: prefix2, Time: t2},
		{Type: peer.EventRouteReceived, PeerIndex: 1, Prefix: prefix1, Time: t2},
		{Type: peer.EventRouteReceived, PeerIndex: 1, Prefix: prefix2, Time: t3},
	}

	result, err := Run(events, defaultCfg)
	require.NoError(t, err)
	assert.Equal(t, "message-ordering", result.Property)
	// Minimal: just the RouteSent(0, prefix1) before any Established.
	assert.Equal(t, 1, len(result.Events), "should shrink to just the violating event")
}

// TestShrinkVerboseOutput verifies that verbose mode produces progress output.
//
// VALIDATES: Verbose output shows shrinking progress.
// PREVENTS: Silent shrinking when user expects progress.
func TestShrinkVerboseOutput(t *testing.T) {
	events := []peer.Event{
		{Type: peer.EventEstablished, PeerIndex: 0, Time: t0},
		{Type: peer.EventEstablished, PeerIndex: 1, Time: t0},
		{Type: peer.EventRouteSent, PeerIndex: 0, Prefix: prefix1, Time: t1},
		{Type: peer.EventRouteSent, PeerIndex: 0, Prefix: prefix2, Time: t2},
	}

	var buf bytes.Buffer
	cfg := Config{PeerCount: 2, Deadline: 5 * time.Second, Verbose: &buf}

	_, err := Run(events, cfg)
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "shrink:")
}

// TestShrinkBinarySearchEffective verifies that binary search reduces
// a large event list before single-step elimination.
//
// VALIDATES: Binary search coarsely narrows the event list.
// PREVENTS: O(n²) single-step elimination on large inputs.
func TestShrinkBinarySearchEffective(t *testing.T) {
	// Build a large event list where the violation is in the first few events.
	events := make([]peer.Event, 0, 102)
	events = append(events,
		peer.Event{Type: peer.EventEstablished, PeerIndex: 0, Time: t0},
		peer.Event{Type: peer.EventEstablished, PeerIndex: 1, Time: t0},
	)
	// Add 100 routes from peer 0 — all cause route-consistency violations
	// (peer 1 never receives any).
	for i := range 100 {
		pfx := netip.MustParsePrefix(fmt.Sprintf("10.0.%d.0/24", i))
		events = append(events, peer.Event{
			Type: peer.EventRouteSent, PeerIndex: 0, Prefix: pfx,
			Time: t0.Add(time.Duration(i+1) * time.Millisecond),
		})
	}

	result, err := Run(events, defaultCfg)
	require.NoError(t, err)
	assert.Equal(t, "route-consistency", result.Property)
	// Should shrink to 3: Established(0), Established(1), RouteSent(0, any).
	assert.Equal(t, 3, len(result.Events), "should minimize to 3 events")
	// Binary search should have been effective — fewer iterations than 102.
	assert.Less(t, result.Iterations, 50, "binary search should reduce iteration count")
}
