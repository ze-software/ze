package validation

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestCheckerAllMatch verifies that no discrepancies are reported when
// expected and actual match perfectly.
//
// VALIDATES: Perfect match returns zero missing and zero extra.
// PREVENTS: False positives in validation.
func TestCheckerAllMatch(t *testing.T) {
	m := NewModel(3)
	m.SetEstablished(0, true)
	m.SetEstablished(1, true)
	m.SetEstablished(2, true)

	m.Announce(0, p("10.0.0.0/24"))
	m.Announce(1, p("172.16.0.0/24"))

	tr := NewTracker(3)
	// Peer 1 received peer 0's route, peer 2 received both.
	tr.RecordReceive(1, p("10.0.0.0/24"))
	tr.RecordReceive(2, p("10.0.0.0/24"))
	// Peer 0 received peer 1's route, peer 2 received it too.
	tr.RecordReceive(0, p("172.16.0.0/24"))
	tr.RecordReceive(2, p("172.16.0.0/24"))

	result := Check(m, tr)

	assert.Equal(t, 0, result.TotalMissing)
	assert.Equal(t, 0, result.TotalExtra)
	assert.True(t, result.Pass)
}

// TestCheckerMissingRoutes verifies detection of routes in expected but
// not in actual.
//
// VALIDATES: Missing routes are reported per peer.
// PREVENTS: Silent propagation failures going undetected.
func TestCheckerMissingRoutes(t *testing.T) {
	m := NewModel(2)
	m.SetEstablished(0, true)
	m.SetEstablished(1, true)

	m.Announce(0, p("10.0.0.0/24"))
	m.Announce(0, p("10.0.1.0/24"))

	tr := NewTracker(2)
	// Peer 1 only received one of two expected routes.
	tr.RecordReceive(1, p("10.0.0.0/24"))

	result := Check(m, tr)

	assert.Equal(t, 1, result.TotalMissing)
	assert.Equal(t, 0, result.TotalExtra)
	assert.False(t, result.Pass)

	// Peer 1 should have one missing route.
	assert.Equal(t, 1, result.Peers[1].Missing.Len())
	assert.True(t, result.Peers[1].Missing.Contains(p("10.0.1.0/24")))
}

// TestCheckerExtraRoutes verifies detection of routes in actual but
// not in expected.
//
// VALIDATES: Extra (unexpected) routes are reported per peer.
// PREVENTS: Spurious route propagation going undetected.
func TestCheckerExtraRoutes(t *testing.T) {
	m := NewModel(2)
	m.SetEstablished(0, true)
	m.SetEstablished(1, true)

	m.Announce(0, p("10.0.0.0/24"))

	tr := NewTracker(2)
	tr.RecordReceive(1, p("10.0.0.0/24"))
	tr.RecordReceive(1, p("10.0.99.0/24")) // Unexpected.

	result := Check(m, tr)

	assert.Equal(t, 0, result.TotalMissing)
	assert.Equal(t, 1, result.TotalExtra)
	assert.False(t, result.Pass)

	assert.Equal(t, 1, result.Peers[1].Extra.Len())
	assert.True(t, result.Peers[1].Extra.Contains(p("10.0.99.0/24")))
}

// TestCheckerMixedDiscrepancies verifies reporting with both missing
// and extra routes across multiple peers.
//
// VALIDATES: Mixed discrepancies correctly tallied across peers.
// PREVENTS: Counting errors when multiple peers have different issues.
func TestCheckerMixedDiscrepancies(t *testing.T) {
	m := NewModel(3)
	m.SetEstablished(0, true)
	m.SetEstablished(1, true)
	m.SetEstablished(2, true)

	m.Announce(0, p("10.0.0.0/24"))

	tr := NewTracker(3)
	// Peer 1: has the route (ok).
	tr.RecordReceive(1, p("10.0.0.0/24"))
	// Peer 2: missing the route, has an extra one.
	tr.RecordReceive(2, p("10.0.99.0/24"))

	result := Check(m, tr)

	assert.Equal(t, 1, result.TotalMissing) // Peer 2 missing 10.0.0.0/24.
	assert.Equal(t, 1, result.TotalExtra)   // Peer 2 has extra 10.0.99.0/24.
	assert.False(t, result.Pass)
}

// TestCheckerDisconnectedPeerIgnored verifies that disconnected peers
// are not checked.
//
// VALIDATES: Disconnected peers excluded from validation.
// PREVENTS: False failures for peers that haven't reconnected yet.
func TestCheckerDisconnectedPeerIgnored(t *testing.T) {
	m := NewModel(2)
	m.SetEstablished(0, true)
	// Peer 1 is NOT established.

	m.Announce(0, p("10.0.0.0/24"))

	tr := NewTracker(2)
	// Peer 1 has nothing — but since it's not established, that's fine.

	result := Check(m, tr)

	assert.Equal(t, 0, result.TotalMissing)
	assert.Equal(t, 0, result.TotalExtra)
	assert.True(t, result.Pass)
}
