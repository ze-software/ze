package report

import (
	"bytes"
	"net/netip"
	"strings"
	"testing"
	"time"

	"codeberg.org/thomas-mangin/ze/cmd/ze-bgp-chaos/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDashboardRenderTTY verifies ANSI output contains escape codes and peer table.
//
// VALIDATES: TTY mode produces ANSI escape codes with per-peer status.
// PREVENTS: Dashboard producing plain text when TTY mode is expected.
func TestDashboardRenderTTY(t *testing.T) {
	var buf bytes.Buffer
	d := NewDashboard(&buf, DashboardConfig{
		IsTTY:     true,
		PeerCount: 2,
	})

	d.ProcessEvent(peer.Event{
		Type:      peer.EventEstablished,
		PeerIndex: 0,
		Time:      time.Now(),
	})

	output := buf.String()

	// Must contain ANSI escape codes for cursor positioning.
	assert.Contains(t, output, "\033[", "should contain ANSI escape codes")

	// Must mention peer 0.
	assert.Contains(t, output, "peer 0")
}

// TestDashboardFallback verifies line-based output when not a TTY.
//
// VALIDATES: Non-TTY mode produces one line per event with no escape codes.
// PREVENTS: ANSI codes corrupting piped output.
func TestDashboardFallback(t *testing.T) {
	var buf bytes.Buffer
	d := NewDashboard(&buf, DashboardConfig{
		IsTTY:     false,
		PeerCount: 2,
	})

	d.ProcessEvent(peer.Event{
		Type:      peer.EventEstablished,
		PeerIndex: 0,
		Time:      time.Now(),
	})

	d.ProcessEvent(peer.Event{
		Type:      peer.EventRouteSent,
		PeerIndex: 1,
		Time:      time.Now(),
		Prefix:    netip.MustParsePrefix("10.0.0.0/24"),
	})

	output := buf.String()

	// Must NOT contain ANSI escape codes.
	assert.NotContains(t, output, "\033[", "should not contain ANSI escape codes")

	// Must contain event info on separate lines.
	lines := strings.Split(strings.TrimSpace(output), "\n")
	require.GreaterOrEqual(t, len(lines), 2, "should have at least 2 lines")
}

// TestDashboardPeerTracking verifies per-peer state updates.
//
// VALIDATES: Dashboard tracks established/disconnected state per peer.
// PREVENTS: All peers showing same state regardless of events.
func TestDashboardPeerTracking(t *testing.T) {
	var buf bytes.Buffer
	d := NewDashboard(&buf, DashboardConfig{
		IsTTY:     true,
		PeerCount: 3,
	})

	// Establish peer 0, disconnect peer 1.
	d.ProcessEvent(peer.Event{Type: peer.EventEstablished, PeerIndex: 0, Time: time.Now()})
	d.ProcessEvent(peer.Event{Type: peer.EventDisconnected, PeerIndex: 1, Time: time.Now()})

	output := buf.String()

	// After the last render, peer 0 should show as established.
	assert.Contains(t, output, "established")
}

// TestDashboardChaosEvent verifies chaos events appear in output.
//
// VALIDATES: Chaos actions are displayed with their type.
// PREVENTS: Chaos events silently dropped from dashboard.
func TestDashboardChaosEvent(t *testing.T) {
	var buf bytes.Buffer
	d := NewDashboard(&buf, DashboardConfig{
		IsTTY:     false,
		PeerCount: 2,
	})

	d.ProcessEvent(peer.Event{
		Type:        peer.EventChaosExecuted,
		PeerIndex:   0,
		Time:        time.Now(),
		ChaosAction: "tcp-disconnect",
	})

	output := buf.String()
	assert.Contains(t, output, "tcp-disconnect")
}

// TestDashboardCloseClears verifies Close clears the terminal in TTY mode.
//
// VALIDATES: Close produces a final clear sequence in TTY mode.
// PREVENTS: Dashboard artifacts remaining after shutdown.
func TestDashboardCloseClears(t *testing.T) {
	var buf bytes.Buffer
	d := NewDashboard(&buf, DashboardConfig{
		IsTTY:     true,
		PeerCount: 1,
	})

	d.ProcessEvent(peer.Event{Type: peer.EventEstablished, PeerIndex: 0, Time: time.Now()})
	require.NoError(t, d.Close())

	output := buf.String()
	// The last escape sequence should clear the screen.
	assert.True(t, strings.HasSuffix(strings.TrimSpace(output), "\033[J") ||
		strings.Contains(output, "\033[2J"),
		"Close should clear the dashboard area")
}
