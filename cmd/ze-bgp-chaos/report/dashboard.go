package report

import (
	"fmt"
	"io"

	"codeberg.org/thomas-mangin/ze/cmd/ze-bgp-chaos/peer"
)

// DashboardConfig configures the live terminal dashboard.
type DashboardConfig struct {
	// IsTTY enables ANSI escape codes for in-place updates.
	// When false, falls back to one-line-per-event output.
	IsTTY bool

	// PeerCount is the total number of peers being simulated.
	PeerCount int
}

// peerState tracks the display state of a single peer.
type peerState struct {
	status     string // "idle", "established", "disconnected"
	routesSent int
	routesRecv int
	lastEvent  string
}

// Dashboard renders a live per-peer status table to a writer.
// In TTY mode, it uses ANSI escape codes to update in-place.
// In non-TTY mode, it prints one line per event.
// Implements the Consumer interface.
type Dashboard struct {
	rw    reportWriter
	cfg   DashboardConfig
	peers []peerState
}

// NewDashboard creates a Dashboard writing to w.
func NewDashboard(w io.Writer, cfg DashboardConfig) *Dashboard {
	peers := make([]peerState, cfg.PeerCount)
	for i := range peers {
		peers[i].status = "idle"
	}
	return &Dashboard{rw: reportWriter{w: w}, cfg: cfg, peers: peers}
}

// ProcessEvent updates peer state and renders the dashboard.
func (d *Dashboard) ProcessEvent(ev peer.Event) {
	if ev.PeerIndex >= 0 && ev.PeerIndex < len(d.peers) {
		d.updatePeer(ev)
	}

	if d.cfg.IsTTY {
		d.renderTTY()
	} else {
		d.renderLine(ev)
	}
}

// updatePeer updates the internal state for the event's peer.
func (d *Dashboard) updatePeer(ev peer.Event) {
	p := &d.peers[ev.PeerIndex]
	p.lastEvent = ev.Type.String()

	switch ev.Type {
	case peer.EventEstablished:
		p.status = "established"
	case peer.EventDisconnected:
		p.status = "disconnected"
	case peer.EventRouteSent:
		p.routesSent++
	case peer.EventRouteReceived:
		p.routesRecv++
	case peer.EventChaosExecuted:
		p.lastEvent = ev.ChaosAction
	case peer.EventReconnecting:
		p.status = "reconnecting"
	case peer.EventRouteWithdrawn, peer.EventEORSent, peer.EventError, peer.EventWithdrawalSent:
		// These events update lastEvent only (set above).
	}
}

// renderTTY redraws the entire dashboard using ANSI escape codes.
func (d *Dashboard) renderTTY() {
	// Cursor home + clear below.
	d.rw.printf("\033[H\033[J")
	d.rw.printf("── ze-bgp-chaos dashboard (%d peers) ──\n", d.cfg.PeerCount)

	for i, p := range d.peers {
		d.rw.printf("  peer %d  %-14s  sent:%-6d recv:%-6d  %s\n",
			i, p.status, p.routesSent, p.routesRecv, p.lastEvent)
	}
}

// renderLine prints a single-line event summary (non-TTY fallback).
func (d *Dashboard) renderLine(ev peer.Event) {
	line := fmt.Sprintf("peer %d | %s", ev.PeerIndex, ev.Type.String())

	if ev.Prefix.IsValid() {
		line += " | " + ev.Prefix.String()
	}

	if ev.ChaosAction != "" {
		line += " | " + ev.ChaosAction
	}

	if ev.Err != nil {
		line += " | error: " + ev.Err.Error()
	}

	d.rw.printf("%s\n", line)
}

// Close clears the dashboard area in TTY mode.
func (d *Dashboard) Close() error {
	if d.cfg.IsTTY {
		// Move cursor to home and clear the screen area used by the dashboard.
		d.rw.printf("\033[H\033[J")
	}
	return d.rw.err
}
