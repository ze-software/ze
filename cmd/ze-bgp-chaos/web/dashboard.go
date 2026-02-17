package web

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"codeberg.org/thomas-mangin/ze/cmd/ze-bgp-chaos/peer"
)

// Config holds configuration for the web dashboard.
type Config struct {
	// Addr is the listen address (e.g., ":8080").
	Addr string

	// PeerCount is the total number of simulated peers.
	PeerCount int

	// MaxVisible is the maximum number of peers in the active set (default 40).
	MaxVisible int

	// EventBufSize is the global event ring buffer capacity (default 500).
	EventBufSize int

	// DebounceInterval is the SSE debounce interval (default 200ms).
	DebounceInterval time.Duration

	// Logger for dashboard messages. Nil means discard.
	Logger *slog.Logger

	// Mux is an optional existing ServeMux to register routes on.
	// When non-nil, the dashboard shares an existing HTTP server
	// (e.g., with --metrics). When nil, a new server is created.
	Mux *http.ServeMux

	// Control is an optional channel for sending control commands
	// (pause, resume, rate, trigger, stop) to the orchestrator.
	// When nil, control UI elements are hidden.
	Control chan ControlCommand

	// ChaosRate is the initial chaos rate (for UI display).
	ChaosRate float64

	// WarmupDuration is the chaos warmup period (for timeline rendering).
	WarmupDuration time.Duration

	// ConvergenceDeadline is the convergence deadline (for histogram marker).
	ConvergenceDeadline time.Duration

	// ControlLogger logs control events (pause/resume/rate/trigger/stop)
	// to the NDJSON event log. When nil, control events are not logged.
	ControlLogger ControlLogger
}

func (c *Config) defaults() {
	if c.MaxVisible <= 0 {
		c.MaxVisible = 40
	}
	if c.EventBufSize <= 0 {
		c.EventBufSize = 500
	}
	if c.DebounceInterval <= 0 {
		c.DebounceInterval = 200 * time.Millisecond
	}
	if c.Logger == nil {
		c.Logger = slog.New(slog.DiscardHandler)
	}
}

const (
	// maxChaosHistory is the maximum number of chaos history entries retained.
	// When exceeded, the oldest half is discarded.
	maxChaosHistory = 10_000

	// maxPeerTransitions is the maximum number of state transitions per peer.
	// When exceeded, the oldest half is discarded.
	maxPeerTransitions = 1_000
)

// Dashboard implements report.Consumer for the web dashboard.
// It tracks per-peer state, manages the active set, and drives an SSE broker
// for live updates.
type Dashboard struct {
	state   *DashboardState
	broker  *SSEBroker
	server  *http.Server
	logger  *slog.Logger
	cancel  context.CancelFunc
	control chan ControlCommand

	// controlLogger logs control events to NDJSON (nil when --event-log not set).
	controlLogger ControlLogger

	// ownServer is true when the Dashboard created its own HTTP server
	// (as opposed to sharing one via Config.Mux).
	ownServer bool

	closeOnce sync.Once
	closeErr  error
}

// New creates a Dashboard and starts the HTTP server and SSE broadcast loop.
// The returned Dashboard implements report.Consumer.
func New(cfg Config) (*Dashboard, error) {
	cfg.defaults()

	state := NewDashboardState(cfg.PeerCount, cfg.MaxVisible, cfg.EventBufSize)
	broker := NewSSEBroker(cfg.DebounceInterval)

	mux := cfg.Mux
	ownServer := mux == nil
	if ownServer {
		mux = http.NewServeMux()
	}

	d := &Dashboard{
		state:         state,
		broker:        broker,
		logger:        cfg.Logger,
		control:       cfg.Control,
		controlLogger: cfg.ControlLogger,
		ownServer:     ownServer,
	}

	// Initialize control state from config.
	if cfg.Control != nil {
		state.Control = ControlState{Rate: cfg.ChaosRate, Status: "running"}
	}
	state.WarmupDuration = cfg.WarmupDuration
	state.ConvergenceDeadline = cfg.ConvergenceDeadline

	if err := registerRoutes(mux, d); err != nil {
		return nil, fmt.Errorf("register routes: %w", err)
	}

	if ownServer {
		lc := net.ListenConfig{}
		ln, err := lc.Listen(context.Background(), "tcp", cfg.Addr)
		if err != nil {
			return nil, err
		}
		d.server = &http.Server{
			Handler:           mux,
			ReadHeaderTimeout: 10 * time.Second,
		}
		go func() {
			if err := d.server.Serve(ln); err != nil && err != http.ErrServerClosed {
				cfg.Logger.Error("http server error", "error", err)
			}
		}()
		cfg.Logger.Info("web dashboard started", "addr", ln.Addr().String())
	}

	ctx, cancel := context.WithCancel(context.Background())
	d.cancel = cancel
	go d.runBroadcastLoop(ctx)

	return d, nil
}

// ProcessEvent implements report.Consumer. It updates per-peer state,
// promotes peers to the active set on noteworthy events, and sets dirty flags
// for the SSE broadcast loop. Must be fast — runs on the main event loop.
func (d *Dashboard) ProcessEvent(ev peer.Event) {
	d.state.mu.Lock()
	defer d.state.mu.Unlock()

	ps, ok := d.state.Peers[ev.PeerIndex]
	if !ok {
		// Unknown peer — ignore.
		return
	}

	// Track previous status for transition recording.
	prevStatus := ps.Status

	// Update per-peer state.
	switch ev.Type {
	case peer.EventEstablished:
		ps.Status = PeerUp
		if prevStatus != PeerUp {
			d.state.PeersUp++
		}
	case peer.EventDisconnected:
		ps.Status = PeerDown
		if prevStatus == PeerUp {
			d.state.PeersUp--
		}
	case peer.EventReconnecting:
		ps.Status = PeerReconnecting
		if prevStatus == PeerUp {
			d.state.PeersUp--
		}
		ps.Reconnects++
		d.state.TotalReconnects++
	case peer.EventRouteSent:
		ps.RoutesSent++
		d.state.TotalAnnounced++
		if ev.Prefix.IsValid() {
			d.state.RouteMatrix.RecordSent(ev.PeerIndex, ev.Prefix, ev.Time)
		}
	case peer.EventRouteReceived:
		ps.RoutesRecv++
		d.state.TotalReceived++
		if ev.Prefix.IsValid() {
			d.state.RouteMatrix.RecordReceived(ev.PeerIndex, ev.Prefix, ev.Time)
		}
	case peer.EventRouteWithdrawn:
		d.state.TotalWithdrawn++
	case peer.EventChaosExecuted:
		ps.ChaosCount++
		d.state.TotalChaos++
		d.state.ChaosHistory = append(d.state.ChaosHistory, ChaosHistoryEntry{
			Time:      ev.Time,
			PeerIndex: ev.PeerIndex,
			Action:    ev.ChaosAction,
		})
		if len(d.state.ChaosHistory) > maxChaosHistory {
			d.state.ChaosHistory = d.state.ChaosHistory[len(d.state.ChaosHistory)-maxChaosHistory/2:]
		}
	case peer.EventWithdrawalSent:
		d.state.TotalWithdrawn += ev.Count
	case peer.EventEORSent:
		// No specific counter.
	case peer.EventError:
		ps.Status = PeerDown
		if prevStatus == PeerUp {
			d.state.PeersUp--
		}
	}

	// Record peer state transitions for timeline visualization.
	if ps.Status != prevStatus {
		trans := d.state.PeerTransitions[ev.PeerIndex]
		trans = append(trans, PeerStateTransition{Time: ev.Time, Status: ps.Status})
		if len(trans) > maxPeerTransitions {
			trans = trans[len(trans)-maxPeerTransitions/2:]
		}
		d.state.PeerTransitions[ev.PeerIndex] = trans
	}

	ps.LastEvent = ev.Type
	ps.LastEventAt = ev.Time
	ps.Events.Push(ev)
	d.state.GlobalEvents.Push(ev)

	// Auto-promote to active set on noteworthy events.
	if prio, ok := PromotionPriorityForEvent(ev.Type); ok {
		d.state.Active.Promote(ev.PeerIndex, prio, ev.Time)
	}

	d.state.MarkDirty(ev.PeerIndex)
}

// Close implements report.Consumer. It stops the SSE broadcast loop,
// shuts down the HTTP server (if owned), and closes the broker.
// Safe to call multiple times — only the first call takes effect.
func (d *Dashboard) Close() error {
	d.closeOnce.Do(func() {
		d.cancel()
		d.broker.Close()
		if d.ownServer && d.server != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			d.closeErr = d.server.Shutdown(ctx)
		}
	})
	return d.closeErr
}

// State returns the dashboard state for read access by handlers.
func (d *Dashboard) State() *DashboardState {
	return d.state
}

// Broker returns the SSE broker.
func (d *Dashboard) Broker() *SSEBroker {
	return d.broker
}

// runBroadcastLoop runs in a goroutine, periodically checking dirty flags
// and broadcasting SSE events with rendered HTML fragments.
func (d *Dashboard) runBroadcastLoop(ctx context.Context) {
	ticker := time.NewTicker(d.broker.Interval())
	defer ticker.Stop()

	// Convergence broadcasts at a lower frequency (~every 2s).
	convergenceInterval := 10 // ticks between convergence updates (10 * 200ms = 2s)
	convergenceTick := 0

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			convergenceTick++
			d.broadcastDirty(convergenceTick%convergenceInterval == 0)
		}
	}
}

// broadcastDirty reads dirty flags and sends SSE events for changed state.
// When broadcastConvergence is true, also pushes convergence histogram updates.
func (d *Dashboard) broadcastDirty(broadcastConvergence bool) {
	d.state.mu.Lock()
	dirtyPeers, dirtyGlobal := d.state.ConsumeDirty()

	// Run active set decay.
	removed := d.state.Active.Decay(time.Now())
	d.state.mu.Unlock()

	if !dirtyGlobal && len(removed) == 0 && !broadcastConvergence {
		return
	}

	// Broadcast stats and events updates.
	if dirtyGlobal {
		d.state.mu.RLock()
		stats := d.renderStats()
		events := d.renderRecentEvents()
		d.state.mu.RUnlock()
		d.broker.Broadcast(SSEEvent{Event: "stats", Data: stats})
		d.broker.Broadcast(SSEEvent{Event: "events", Data: events})
	}

	// Broadcast convergence histogram (~every 2s).
	if broadcastConvergence {
		d.state.mu.RLock()
		convergence := d.renderConvergence()
		d.state.mu.RUnlock()
		d.broker.Broadcast(SSEEvent{Event: "convergence", Data: convergence})
	}

	// Broadcast peer updates for dirty peers in the active set.
	d.state.mu.RLock()
	for idx := range dirtyPeers {
		if !d.state.Active.Contains(idx) {
			continue
		}
		row := d.renderPeerRow(idx)
		d.broker.Broadcast(SSEEvent{Event: "peer-update", Data: row})
	}
	d.state.mu.RUnlock()

	// Broadcast removals for decayed peers.
	for _, idx := range removed {
		d.broker.Broadcast(SSEEvent{
			Event: "peer-remove",
			Data:  renderPeerRemoval(idx),
		})
	}
}

// renderStats returns a minimal stats HTML fragment for SSE.
// Must preserve sse-swap and hx-swap attributes so future SSE events continue to work.
func (d *Dashboard) renderStats() string {
	return `<div id="stats" sse-swap="stats" hx-swap="outerHTML">` +
		`<span class="stat"><span class="stat-label">Peers </span><span class="stat-value">` + itoa(d.state.PeersUp) + `/` + itoa(d.state.PeerCount) + `</span></span>` +
		`<span class="stat"><span class="stat-label">Announced </span><span class="stat-value">` + itoa(d.state.TotalAnnounced) + `</span></span>` +
		`<span class="stat"><span class="stat-label">Received </span><span class="stat-value">` + itoa(d.state.TotalReceived) + `</span></span>` +
		`<span class="stat"><span class="stat-label">Withdrawn </span><span class="stat-value">` + itoa(d.state.TotalWithdrawn) + `</span></span>` +
		`<span class="stat"><span class="stat-label">Chaos </span><span class="stat-value">` + itoa(d.state.TotalChaos) + `</span></span>` +
		`<span class="stat"><span class="stat-label">Reconnects </span><span class="stat-value">` + itoa(d.state.TotalReconnects) + `</span></span>` +
		`</div>`
}

// renderConvergence returns the convergence histogram HTML fragment for SSE.
// Must preserve sse-swap and hx-swap attributes so future SSE events continue to work.
func (d *Dashboard) renderConvergence() string {
	var b strings.Builder
	b.WriteString(`<div id="viz-convergence" sse-swap="convergence" hx-swap="outerHTML">`)
	writeConvergenceHistogram(&b, d.state.Convergence, d.state.ConvergenceDeadline)
	b.WriteString(`</div>`)
	return b.String()
}

// renderRecentEvents returns the recent events HTML fragment for SSE.
// Must preserve sse-swap and hx-swap attributes so future SSE events continue to work.
func (d *Dashboard) renderRecentEvents() string {
	var b strings.Builder
	b.WriteString(`<div id="events" class="event-list" sse-swap="events" hx-swap="outerHTML">`)
	writeRecentEvents(&b, d.state)
	b.WriteString(`</div>`)
	return b.String()
}

// renderPeerRow returns a table row HTML fragment for a single peer.
func (d *Dashboard) renderPeerRow(idx int) string {
	ps := d.state.Peers[idx]
	if ps == nil {
		return ""
	}
	pinned := d.state.Active.IsPinned(idx)
	pinClass := "pin"
	if pinned {
		pinClass = "pin pinned"
	}
	return "<tr id=\"peer-" + itoa(idx) + "\" hx-swap-oob=\"outerHTML\">" +
		"<td><span class=\"" + pinClass + "\" hx-post=\"/peers/" + itoa(idx) + "/pin\" hx-swap=\"none\"></span></td>" +
		"<td>" + itoa(idx) + "</td>" +
		"<td><span class=\"dot " + ps.Status.CSSClass() + "\"></span> " + ps.Status.String() + "</td>" +
		"<td>" + itoa(ps.RoutesSent) + "</td>" +
		"<td>" + itoa(ps.RoutesRecv) + "</td>" +
		"<td>" + itoa(ps.ChaosCount) + "</td>" +
		"</tr>"
}

// renderPeerRemoval returns an empty element to remove a peer row via hx-swap-oob.
func renderPeerRemoval(idx int) string {
	return "<tr id=\"peer-" + itoa(idx) + "\" hx-swap-oob=\"delete\"></tr>"
}

// itoa is a simple int-to-string helper to avoid importing strconv for HTML rendering.
func itoa(n int) string {
	return fmt.Sprintf("%d", n)
}
