package report

import (
	"fmt"
	"io"
	"strings"
	"time"

	"codeberg.org/thomas-mangin/ze/cmd/ze-chaos/peer"
)

// DashboardConfig configures the live terminal dashboard.
type DashboardConfig struct {
	// IsTTY is retained for API compatibility but no longer affects behavior.
	// All output uses scrolling one-line-per-event format.
	IsTTY bool

	// PeerCount is the total number of peers being simulated.
	PeerCount int

	// StatusInterval controls how often the aggregate status line is printed.
	// Zero defaults to 2 seconds.
	StatusInterval time.Duration
}

// Dashboard prints lifecycle events immediately and a periodic aggregate
// status line for route activity. Implements the Consumer interface.
type Dashboard struct {
	rw  reportWriter
	cfg DashboardConfig

	// Aggregate counters for the periodic status line.
	sent      int
	received  int
	withdrawn int

	// Per-peer state for the status line.
	established int

	// Timer for periodic status output.
	lastStatus time.Time
	interval   time.Duration
}

// NewDashboard creates a Dashboard writing to w.
func NewDashboard(w io.Writer, cfg DashboardConfig) *Dashboard {
	interval := cfg.StatusInterval
	if interval == 0 {
		interval = 2 * time.Second
	}
	return &Dashboard{
		rw:       reportWriter{w: w},
		cfg:      cfg,
		interval: interval,
	}
}

// ProcessEvent handles a single event. Lifecycle events are printed
// immediately; route events update aggregate counters and trigger a
// periodic status line.
func (d *Dashboard) ProcessEvent(ev peer.Event) {
	switch ev.Type {
	case peer.EventRouteSent:
		d.sent++
		d.maybeStatus(ev.Time)
	case peer.EventRouteReceived:
		d.received++
		d.maybeStatus(ev.Time)
	case peer.EventRouteWithdrawn:
		d.withdrawn++
		d.maybeStatus(ev.Time)
	case peer.EventWithdrawalSent:
		d.withdrawn += ev.Count
		d.maybeStatus(ev.Time)
	case peer.EventEstablished:
		d.established++
		d.rw.printf("peer %d | established (%d/%d)\n", ev.PeerIndex, d.established, d.cfg.PeerCount)
	case peer.EventDisconnected:
		d.established--
		d.rw.printf("peer %d | disconnected (%d/%d)\n", ev.PeerIndex, d.established, d.cfg.PeerCount)
	case peer.EventEORSent:
		if len(ev.Families) > 0 {
			d.rw.printf("peer %d | eor-sent | %d routes [%s]\n", ev.PeerIndex, ev.Count, strings.Join(ev.Families, ", "))
		} else {
			d.rw.printf("peer %d | eor-sent | %d routes\n", ev.PeerIndex, ev.Count)
		}
	case peer.EventError, peer.EventChaosExecuted, peer.EventReconnecting:
		d.printLifecycle(ev)
	}
}

// printLifecycle prints a single-line event for non-route events.
func (d *Dashboard) printLifecycle(ev peer.Event) {
	line := fmt.Sprintf("peer %d | %s", ev.PeerIndex, ev.Type.String())

	if ev.ChaosAction != "" {
		line += " | " + ev.ChaosAction
	}

	if ev.Err != nil {
		line += " | " + ev.Err.Error()
	}

	d.rw.printf("%s\n", line)
}

// maybeStatus prints an aggregate status line if enough time has passed.
func (d *Dashboard) maybeStatus(now time.Time) {
	if now.Sub(d.lastStatus) < d.interval {
		return
	}
	d.lastStatus = now
	d.rw.printf("  routes: %d sent, %d received, %d withdrawn | peers: %d/%d\n",
		d.sent, d.received, d.withdrawn, d.established, d.cfg.PeerCount)
}

// Close prints a final status line.
func (d *Dashboard) Close() error {
	d.rw.printf("  routes: %d sent, %d received, %d withdrawn | peers: %d/%d (final)\n",
		d.sent, d.received, d.withdrawn, d.established, d.cfg.PeerCount)
	return d.rw.err
}
