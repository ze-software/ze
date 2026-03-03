package report

import (
	"bytes"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/chaos/peer"
)

// TestDashboardLifecycleEvents verifies lifecycle events are printed immediately.
//
// VALIDATES: Established, disconnected, error, eor events produce output lines.
// PREVENTS: Lifecycle events silently dropped.
func TestDashboardLifecycleEvents(t *testing.T) {
	var buf bytes.Buffer
	d := NewDashboard(&buf, DashboardConfig{PeerCount: 4})

	now := time.Now()
	d.ProcessEvent(peer.Event{Type: peer.EventEstablished, PeerIndex: 0, Time: now})
	d.ProcessEvent(peer.Event{Type: peer.EventEstablished, PeerIndex: 1, Time: now})
	d.ProcessEvent(peer.Event{Type: peer.EventEORSent, PeerIndex: 0, Time: now})
	d.ProcessEvent(peer.Event{Type: peer.EventError, PeerIndex: 2, Time: now, Err: assert.AnError})

	output := buf.String()
	assert.Contains(t, output, "peer 0 | established (1/4)")
	assert.Contains(t, output, "peer 1 | established (2/4)")
	assert.Contains(t, output, "peer 0 | eor-sent")
	assert.Contains(t, output, "peer 2 | error")
	assert.Contains(t, output, assert.AnError.Error())
}

// TestDashboardRoutesSuppressed verifies individual route events are not printed.
//
// VALIDATES: Route events update counters without per-route output lines.
// PREVENTS: Thousands of route-sent lines drowning lifecycle events.
func TestDashboardRoutesSuppressed(t *testing.T) {
	var buf bytes.Buffer
	d := NewDashboard(&buf, DashboardConfig{PeerCount: 2, StatusInterval: time.Hour})

	now := time.Now()
	d.ProcessEvent(peer.Event{Type: peer.EventRouteSent, PeerIndex: 0, Time: now, Prefix: mustPrefix("10.0.0.0/24")})
	d.ProcessEvent(peer.Event{Type: peer.EventRouteSent, PeerIndex: 0, Time: now, Prefix: mustPrefix("10.0.1.0/24")})

	output := buf.String()
	assert.NotContains(t, output, "10.0.0.0/24", "individual routes should not be printed")
	assert.Equal(t, 2, d.sent, "sent counter should be updated")
}

// TestDashboardPeriodicStatus verifies the aggregate status line is emitted periodically.
//
// VALIDATES: Status line shows sent/received/withdrawn counts at intervals.
// PREVENTS: No feedback during long route-sending phases.
func TestDashboardPeriodicStatus(t *testing.T) {
	var buf bytes.Buffer
	d := NewDashboard(&buf, DashboardConfig{PeerCount: 2, StatusInterval: 100 * time.Millisecond})

	t0 := time.Now()
	// First route — triggers status (no previous status).
	d.ProcessEvent(peer.Event{Type: peer.EventEstablished, PeerIndex: 0, Time: t0})
	d.ProcessEvent(peer.Event{Type: peer.EventRouteSent, PeerIndex: 0, Time: t0})
	// Route within interval — no status.
	d.ProcessEvent(peer.Event{Type: peer.EventRouteSent, PeerIndex: 0, Time: t0.Add(50 * time.Millisecond)})
	// Route after interval — triggers status.
	d.ProcessEvent(peer.Event{Type: peer.EventRouteSent, PeerIndex: 0, Time: t0.Add(200 * time.Millisecond)})

	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")

	// Should have: established, first status, second status = 3+ lines.
	statusCount := 0
	for _, l := range lines {
		if strings.Contains(l, "routes:") {
			statusCount++
		}
	}
	require.GreaterOrEqual(t, statusCount, 2, "expected at least 2 status lines, got:\n%s", output)
	assert.Contains(t, output, "3 sent")
}

// TestDashboardEstablishedCount tracks the running peer count.
//
// VALIDATES: Established/disconnected events increment/decrement the counter.
// PREVENTS: Stale peer count in status output.
func TestDashboardEstablishedCount(t *testing.T) {
	var buf bytes.Buffer
	d := NewDashboard(&buf, DashboardConfig{PeerCount: 3})

	now := time.Now()
	d.ProcessEvent(peer.Event{Type: peer.EventEstablished, PeerIndex: 0, Time: now})
	d.ProcessEvent(peer.Event{Type: peer.EventEstablished, PeerIndex: 1, Time: now})
	d.ProcessEvent(peer.Event{Type: peer.EventDisconnected, PeerIndex: 0, Time: now})

	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")
	require.Len(t, lines, 3)
	assert.Contains(t, lines[0], "(1/3)")
	assert.Contains(t, lines[1], "(2/3)")
	assert.Contains(t, lines[2], "(1/3)")
}

// TestDashboardChaosEvent verifies chaos events appear in output.
//
// VALIDATES: Chaos actions are displayed with their type.
// PREVENTS: Chaos events silently dropped from output.
func TestDashboardChaosEvent(t *testing.T) {
	var buf bytes.Buffer
	d := NewDashboard(&buf, DashboardConfig{PeerCount: 2})

	d.ProcessEvent(peer.Event{
		Type:        peer.EventChaosExecuted,
		PeerIndex:   0,
		Time:        time.Now(),
		ChaosAction: "tcp-disconnect",
	})

	output := buf.String()
	assert.Contains(t, output, "tcp-disconnect")
}

// TestDashboardCloseFinalStatus verifies Close prints a final status line.
//
// VALIDATES: Final aggregate counts are printed on close.
// PREVENTS: Missing summary when run ends.
func TestDashboardCloseFinalStatus(t *testing.T) {
	var buf bytes.Buffer
	d := NewDashboard(&buf, DashboardConfig{PeerCount: 2, StatusInterval: time.Hour})

	now := time.Now()
	d.ProcessEvent(peer.Event{Type: peer.EventRouteSent, PeerIndex: 0, Time: now})
	d.ProcessEvent(peer.Event{Type: peer.EventRouteReceived, PeerIndex: 1, Time: now})

	buf.Reset()
	require.NoError(t, d.Close())

	output := buf.String()
	assert.Contains(t, output, "1 sent")
	assert.Contains(t, output, "1 received")
	assert.Contains(t, output, "(final)")
}

func mustPrefix(s string) netip.Prefix {
	return netip.MustParsePrefix(s)
}
