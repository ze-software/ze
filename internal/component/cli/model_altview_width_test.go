package cli

import (
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/ze-software/ze/internal/component/cli/contract"
	"github.com/ze-software/ze/internal/core/bgp/asn"
)

// altViewTerminalWidth is the terminal width every test in this file renders
// at. It is the width of the published `monitor bgp` recording, where the
// defect was seen.
const altViewTerminalWidth = 137

// assertAltViewFits fails when any line of a full-screen view, as View() hands
// it to the terminal, is wider than the terminal. A wider line is clipped by
// the terminal, so its last column is lost.
func assertAltViewFits(t *testing.T, name, content string) {
	t.Helper()
	for i, line := range strings.Split(content, "\n") {
		if got := lipgloss.Width(line); got > altViewTerminalWidth {
			t.Errorf("%s: line %d is %d columns wide on a %d-column terminal, so the terminal clips it:\n%q",
				name, i, got, altViewTerminalWidth, line)
		}
	}
}

// VALIDATES: the `monitor bgp` dashboard, as View() renders it through
// paddedAltView, fits the terminal and its footer keeps "Last update: 0s ago"
// whole.
//
// PREVENTS: the dashboard sizing its content to the full terminal width while
// paddedAltView prepends a one-column margin. The right-aligned footer then
// ended one column past the terminal edge, and the terminal clipped it to
// "Last update: 0s ag". The test renders through View(), the path a terminal
// reads, and measures every line against the terminal width.
func TestDashboardAltViewFitsTerminal(t *testing.T) {
	t.Parallel()
	m := NewCommandModel(FilesystemAuthorityOperatorLocal)
	m.width = altViewTerminalWidth
	m.height = 40
	m.activeView = &dashboardView{st: &dashboardState{
		snapshot: &dashboardSnapshot{
			LocalAS:          asn.Of(65000),
			RouterID:         "10.0.0.254",
			Uptime:           "1h2m",
			PeersEstablished: 1,
			PeersConfigured:  1,
			Peers:            []dashboardPeer{{Address: "10.0.0.1", RemoteAS: asn.Of(65001), State: "established", Uptime: "1h2m"}},
		},
		sortAsc:      true,
		rates:        map[string]*peerRateEntry{},
		lastPollTime: time.Now(),
	}}

	content := m.View().Content
	assertAltViewFits(t, "dashboard", content)

	lines := strings.Split(strings.TrimRight(content, "\n"), "\n")
	footer := zzAnsiRe.ReplaceAllString(lines[len(lines)-1], "")
	if !strings.HasSuffix(footer, "Last update: 0s ago") {
		t.Errorf("dashboard footer does not end with %q:\n%q", "Last update: 0s ago", footer)
	}
}

// VALIDATES: the ping and traceroute live views, and their piped (replace
// mode) variants, fit the terminal as View() renders them. Each pads its footer
// to the content width, so each overflowed the same way the dashboard did.
func TestLiveViewsAltViewFitTerminal(t *testing.T) {
	t.Parallel()
	views := []struct {
		name string
		view activeView
	}{
		{"ping", &pingView{st: &pingState{target: "8.8.8.8", interval: defaultPingMonitorInterval}}},
		{"ping piped", &pingPipedView{st: &pingPipedState{target: "8.8.8.8", interval: defaultPingMonitorInterval}}},
		{"traceroute", &tracerouteView{st: &tracerouteState{target: "8.8.8.8", maxHops: 30, rounds: 1, lastPollTime: time.Now()}}},
		{"traceroute piped", &traceroutePipedView{st: &traceroutePipedState{target: "8.8.8.8", maxHops: 30}}},
	}
	for _, tc := range views {
		m := NewCommandModel(FilesystemAuthorityOperatorLocal)
		m.width = altViewTerminalWidth
		m.height = 40
		m.activeView = tc.view
		content := m.View().Content
		if content == "" {
			t.Fatalf("%s: View rendered nothing", tc.name)
		}
		assertAltViewFits(t, tc.name, content)
	}
}

// VALIDATES: a monitor session's RenderFunc that fills the width it is given
// still fits the terminal once View() wraps it in paddedAltView. The width the
// Model passes is the content width, not the terminal width.
func TestMonitorRenderFuncAltViewFitsTerminal(t *testing.T) {
	t.Parallel()
	m := NewCommandModel(FilesystemAuthorityOperatorLocal)
	m.width = altViewTerminalWidth
	m.height = 40
	m.monitorSession = &contract.MonitorSession{
		RenderFunc: func(width, _ int) string { return strings.Repeat("x", width) },
	}
	assertAltViewFits(t, "monitor", m.View().Content)
}
