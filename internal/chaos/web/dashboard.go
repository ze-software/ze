// Design: docs/architecture/chaos-web-dashboard.md — web dashboard UI

package web

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/chaos/peer"
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
	// (pause, resume, rate, trigger, stop) to the chaos scheduler.
	// When nil, chaos control UI elements are hidden.
	Control chan ControlCommand

	// RouteControl is an optional channel for sending control commands
	// to the route dynamics scheduler. When nil, route control UI is hidden.
	RouteControl chan ControlCommand

	// Seed is the deterministic seed for this run (displayed in header).
	Seed uint64

	// ChaosRate is the initial chaos rate (for UI display).
	ChaosRate float64

	// RouteRate is the initial route dynamics rate (for UI display).
	RouteRate float64

	// WarmupDuration is the chaos warmup period (for timeline rendering).
	WarmupDuration time.Duration

	// ConvergenceDeadline is the convergence deadline (for histogram marker).
	ConvergenceDeadline time.Duration

	// ControlLogger logs control events (pause/resume/rate/trigger/stop)
	// to the NDJSON event log. When nil, control events are not logged.
	ControlLogger ControlLogger

	// RestartCh receives a new seed when the user requests a restart.
	// When nil, the restart button is hidden.
	RestartCh chan<- uint64

	// OnStop is called when the dashboard triggers a stop or restart.
	// It should cancel the current run's context.
	OnStop func()

	// InitialSpeedFactor is the initial time acceleration factor (1, 10, 100, 1000).
	// When non-zero, enables the speed control UI in the dashboard.
	// Only meaningful in in-process mode where virtual clock pacing is adjustable.
	InitialSpeedFactor int

	// PeerFamilyTargets maps peer index → family → expected route count.
	// Used to show sent progress (nb/max) in the Families tab.
	// Computed from profile data: unicast families get full RouteCount,
	// non-unicast (VPN, EVPN, FlowSpec) get RouteCount/4.
	PeerFamilyTargets map[int]map[string]int
}

func (c *Config) defaults() {
	if c.MaxVisible <= 0 {
		c.MaxVisible = 40
	}
	if c.PeerCount > 0 && c.MaxVisible > c.PeerCount {
		c.MaxVisible = c.PeerCount
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

	// routeControl is the channel for route dynamics control commands.
	// When nil, route dynamics control UI is hidden.
	routeControl chan ControlCommand

	// controlLogger logs control events to NDJSON (nil when --event-log not set).
	controlLogger ControlLogger

	// restartCh receives a new seed when the user requests a restart.
	// When nil, the restart UI element is hidden.
	restartCh chan<- uint64

	// onStop is called when the dashboard triggers a stop or restart.
	// It should cancel the current run's context.
	onStop func()

	// ownServer is true when the Dashboard created its own HTTP server
	// (as opposed to sharing one via Config.Mux).
	ownServer bool

	// speedNanos stores the current step delay in nanoseconds for dynamic
	// speed control. Read atomically by the in-process runner each iteration.
	// Zero means speed control is disabled (caller uses its static default).
	speedNanos atomic.Int64

	closeOnce sync.Once
	closeErr  error
}

// StepDelay returns the current step delay for the simulation runner.
// Called each iteration by the in-process runner to support dynamic speed control.
// Returns 0 if speed control is not enabled (caller should use its static default).
func (d *Dashboard) StepDelay() time.Duration {
	n := d.speedNanos.Load()
	if n <= 0 {
		return 0
	}
	return time.Duration(n)
}

// SetSpeedFactor updates the time acceleration factor and the corresponding step delay.
// Valid factors: 1, 10, 100, 1000. Returns false for invalid factors.
func (d *Dashboard) SetSpeedFactor(factor int) bool {
	switch factor {
	case 1, 10, 100, 1000:
	default:
		return false
	}
	d.speedNanos.Store(int64(time.Second) / int64(factor))
	d.state.mu.Lock()
	d.state.Control.SpeedFactor = factor
	d.state.mu.Unlock()
	return true
}

// New creates a Dashboard and starts the HTTP server and SSE broadcast loop.
// The returned Dashboard implements report.Consumer.
func New(cfg Config) (*Dashboard, error) {
	cfg.defaults()

	state := NewDashboardState(cfg.PeerCount, cfg.MaxVisible, cfg.EventBufSize)
	for idx, targets := range cfg.PeerFamilyTargets {
		if ps := state.Peers[idx]; ps != nil {
			ps.FamilySentTarget = targets
		}
	}
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
		routeControl:  cfg.RouteControl,
		controlLogger: cfg.ControlLogger,
		restartCh:     cfg.RestartCh,
		onStop:        cfg.OnStop,
		ownServer:     ownServer,
	}

	// Initialize control state from config.
	if cfg.Control != nil {
		state.Control = ControlState{Rate: cfg.ChaosRate, Status: "running"}
	}
	if cfg.RouteControl != nil {
		state.Control.RouteRate = cfg.RouteRate
		state.Control.RouteStatus = "running"
	}
	if cfg.RestartCh != nil {
		state.Control.RestartAvailable = true
	}
	state.Control.Seed = state.Seed
	switch cfg.InitialSpeedFactor {
	case 0: // disabled
	case 1, 10, 100, 1000:
		state.Control.SpeedAvailable = true
		state.Control.SpeedFactor = cfg.InitialSpeedFactor
		d.speedNanos.Store(int64(time.Second) / int64(cfg.InitialSpeedFactor))
	default:
		return nil, fmt.Errorf("invalid InitialSpeedFactor %d (must be 1, 10, 100, or 1000)", cfg.InitialSpeedFactor)
	}
	state.Seed = cfg.Seed
	state.WarmupDuration = cfg.WarmupDuration
	state.ConvergenceDeadline = cfg.ConvergenceDeadline

	// Auto-promote all peers when the total fits in the active set.
	// This ensures small deployments show all peers immediately.
	if cfg.PeerCount <= state.Active.MaxVisible {
		now := time.Now()
		for i := range cfg.PeerCount {
			state.Active.Promote(i, PriorityLow, now)
		}
	}

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
		ps.Status = PeerSyncing
		if prevStatus != PeerSyncing {
			d.state.PeersSyncing++
		}
		// Reset per-peer route counters so reconnected peers start fresh.
		// Global counters (TotalAnnounced, TotalReceived) stay cumulative.
		d.state.TotalAnnounced -= ps.RoutesSent
		d.state.TotalReceived -= ps.RoutesRecv
		d.state.TotalMissing = max(0, d.state.TotalAnnounced-d.state.TotalReceived)
		ps.RoutesSent = 0
		ps.RoutesRecv = 0
		ps.Missing = 0
		// Record negotiated families from EventEstablished.
		// Reset per-family counters so reconnected peers start fresh.
		if len(ev.Families) > 0 {
			ps.Families = ev.Families
			clear(ps.FamilySent)
			clear(ps.FamilyRecv)
			for _, f := range ev.Families {
				d.state.AllFamilies[f] = true
			}
		}
	case peer.EventDisconnected:
		ps.ChaosActive = true
		ps.Status = PeerDown
		switch prevStatus {
		case PeerUp:
			d.state.PeersUp--
		case PeerSyncing:
			d.state.PeersSyncing--
		case PeerIdle, PeerDown, PeerReconnecting:
		}
	case peer.EventReconnecting:
		ps.ChaosActive = true
		ps.Status = PeerReconnecting
		switch prevStatus {
		case PeerUp:
			d.state.PeersUp--
		case PeerSyncing:
			d.state.PeersSyncing--
		case PeerIdle, PeerDown, PeerReconnecting:
		}
		ps.Reconnects++
		d.state.TotalReconnects++
	case peer.EventRouteSent:
		ps.RoutesSent++
		d.state.TotalAnnounced++
		ps.Missing = max(0, ps.RoutesSent-ps.RoutesRecv)
		d.state.TotalMissing = max(0, d.state.TotalAnnounced-d.state.TotalReceived)
		if ev.Family != "" {
			ps.FamilySent[ev.Family]++
		}
		if ev.Prefix.IsValid() {
			d.state.RouteMatrix.RecordSent(ev.PeerIndex, ev.Prefix, ev.Time)
		} else if ev.Family != "" {
			d.state.RouteMatrix.RecordNonUnicastSent(ev.PeerIndex, ev.Family)
		}
	case peer.EventRouteReceived:
		ps.RoutesRecv++
		d.state.TotalReceived++
		ps.Missing = max(0, ps.RoutesSent-ps.RoutesRecv)
		d.state.TotalMissing = max(0, d.state.TotalAnnounced-d.state.TotalReceived)
		if ev.Family != "" {
			ps.FamilyRecv[ev.Family]++
		} else if ev.Prefix.IsValid() {
			ps.FamilyRecv[prefixFamily(ev.Prefix)]++
		}
		if ev.Prefix.IsValid() {
			if latency := d.state.RouteMatrix.RecordReceived(ev.PeerIndex, ev.Prefix, ev.Time); latency > 0 {
				d.state.Convergence.Record(latency)
				d.state.ConvergenceTrend.Push(latency)
			}
		} else if ev.Family != "" {
			d.state.RouteMatrix.RecordNonUnicastReceived(ev.PeerIndex, ev.Family)
		}
	case peer.EventRouteWithdrawn:
		d.state.TotalWithdrawn++
	case peer.EventChaosExecuted:
		ps.ChaosActive = true
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
		d.state.TotalWdrawSent += ev.Count
	case peer.EventRouteAction:
		d.state.TotalRouteActions++
	case peer.EventDroppedEvents:
		d.state.TotalDropped += ev.Count
	case peer.EventEORSent:
		// Fallback: record families from EOR if not yet set.
		if len(ps.Families) == 0 && len(ev.Families) > 0 {
			ps.Families = ev.Families
			for _, f := range ev.Families {
				d.state.AllFamilies[f] = true
			}
		}
		// Transition syncing → up on first EOR.
		if ps.Status == PeerSyncing {
			ps.Status = PeerUp
			d.state.PeersSyncing--
			d.state.PeersUp++
		}
		// Track initial EOR per peer for sync duration measurement.
		if ev.PeerIndex < len(d.state.EORSeen) && !d.state.EORSeen[ev.PeerIndex] {
			d.state.EORSeen[ev.PeerIndex] = true
			d.state.EORCount++
			if d.state.EORCount == d.state.PeerCount {
				d.state.SyncDuration = time.Since(d.state.StartTime).Truncate(time.Millisecond)
			}
		}
	case peer.EventError:
		ps.ChaosActive = true
		ps.Status = PeerDown
		switch prevStatus {
		case PeerUp:
			d.state.PeersUp--
		case PeerSyncing:
			d.state.PeersSyncing--
		case PeerIdle, PeerDown, PeerReconnecting:
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

	// Accumulate byte counters from event deltas.
	if ev.BytesSent > 0 {
		ps.BytesSent += ev.BytesSent
		d.state.TotalBytesSent += ev.BytesSent
	}
	if ev.BytesRecv > 0 {
		ps.BytesRecv += ev.BytesRecv
		d.state.TotalBytesRecv += ev.BytesRecv
	}

	// Queue toast for toast-worthy events.
	if toast, ok := toastForEvent(ev); ok {
		d.state.QueueToast(toast)
	}

	ps.LastEvent = ev.Type
	ps.LastEventAt = ev.Time
	ps.Events.Push(ev)
	d.state.GlobalEvents.Push(ev)

	// Refresh LastActive for peers already in the active set (keeps them visible).
	if e := d.state.Active.Entry(ev.PeerIndex); e != nil {
		e.LastActive = ev.Time
	}

	// Auto-promote to active set on noteworthy events.
	if prio, ok := PromotionPriorityForEvent(ev.Type); ok {
		if d.state.Active.Promote(ev.PeerIndex, prio, ev.Time) {
			d.state.newlyPromoted[ev.PeerIndex] = true
		}
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
	now := time.Now()

	d.state.mu.Lock()
	dirtyPeers, promotedPeers, dirtyGlobal := d.state.ConsumeDirty()
	pendingToasts := d.state.ConsumePendingToasts()

	// Update per-peer throughput EMA.
	d.state.UpdateThroughput(now)

	// Run active set decay — skip for small deployments where all peers fit.
	var removed []int
	if d.state.PeerCount > d.state.Active.MaxVisible {
		removed = d.state.Active.Decay(now)
	}
	d.state.mu.Unlock()

	if !dirtyGlobal && len(removed) == 0 && len(promotedPeers) == 0 && len(pendingToasts) == 0 && !broadcastConvergence {
		return
	}

	// Broadcast toast notifications.
	for _, t := range pendingToasts {
		d.broker.Broadcast(SSEEvent{Event: "toast", Data: renderToast(t)})
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

	// Broadcast convergence histogram and trend (~every 2s).
	if broadcastConvergence {
		d.state.mu.RLock()
		convergence := d.renderConvergence()
		trend := d.renderConvergenceTrend()
		d.state.mu.RUnlock()
		d.broker.Broadcast(SSEEvent{Event: "convergence", Data: convergence})
		d.broker.Broadcast(SSEEvent{Event: "convergence-trend", Data: trend})
	}

	// Broadcast new rows for newly promoted peers.
	// Remove-then-add pattern: delete any stale row, then insert fresh.
	d.state.mu.RLock()
	for idx := range promotedPeers {
		d.broker.Broadcast(SSEEvent{Event: "peer-remove", Data: renderPeerRemoval(idx)})
		row := d.renderPeerRowInsert(idx)
		d.broker.Broadcast(SSEEvent{Event: "peer-add", Data: row})
	}

	// Broadcast updates for dirty peers already in the active set.
	for idx := range dirtyPeers {
		if promotedPeers[idx] {
			continue // Already sent as peer-add above.
		}
		if !d.state.Active.Contains(idx) {
			continue
		}
		row := d.renderPeerRow(idx)
		d.broker.Broadcast(SSEEvent{Event: "peer-update", Data: row})
	}
	d.state.mu.RUnlock()

	// Clear transient ChaosActive flags for rendered peers.
	d.state.mu.Lock()
	for idx := range dirtyPeers {
		if ps := d.state.Peers[idx]; ps != nil {
			ps.ChaosActive = false
		}
	}
	d.state.mu.Unlock()

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
	var donutBuf strings.Builder
	counts := d.state.StatusCounts()
	writeDonut(&donutBuf, counts, d.state.PeerCount)
	writeDonutLegend(&donutBuf, counts)
	writeDonutEnd(&donutBuf)

	return `<div id="stats" sse-swap="stats" hx-swap="outerHTML" hx-get="/sidebar/stats" hx-trigger="every 500ms">` +
		donutBuf.String() +
		`<div class="stat-grid">` +
		`<span></span><span class="stat-grid-header">Out</span><span class="stat-grid-header">In</span>` +
		`<span class="stat-label">Msgs</span><span class="stat-value">` + itoa(d.state.TotalAnnounced) + `</span><span class="stat-value">` + itoa(d.state.TotalReceived) + `</span>` +
		`<span class="stat-label">Bytes</span><span class="stat-value">` + FormatBytes(d.state.TotalBytesSent) + `</span><span class="stat-value">` + FormatBytes(d.state.TotalBytesRecv) + `</span>` +
		`<span class="stat-label">Rate</span><span class="stat-value">` + FormatBitRate(d.state.AggregateThroughput(true)) + `</span><span class="stat-value">` + FormatBitRate(d.state.AggregateThroughput(false)) + `</span>` +
		`<span class="stat-label">Wdraw</span><span class="stat-value">` + itoa(d.state.TotalWithdrawn) + `</span><span class="stat-value">` + itoa(d.state.TotalWdrawSent) + `</span>` +
		`</div>` +
		`<span class="stat"><span class="stat-label">Churn </span><span class="stat-value">` + itoa(d.state.TotalRouteActions) + `</span></span>` +
		`<span class="stat"><span class="stat-label">Chaos </span><span class="stat-value">` + itoa(d.state.TotalChaos) + `</span></span> ` +
		`<span class="stat"><span class="stat-value ` + ChaosRateColorClass(d.state.ChaosRate()) + `">` + fmt.Sprintf("%.1f/s", d.state.ChaosRate()) + `</span></span> ` +
		`<span class="stat"><span class="stat-label">Reconn </span><span class="stat-value">` + itoa(d.state.TotalReconnects) + `</span></span>` +
		droppedStat(d.state.TotalDropped) +
		syncStat(d.state.EORCount, d.state.PeerCount, d.state.SyncDuration) +
		speedStat(d.state.Control.SpeedAvailable, d.state.Control.SpeedFactor) +
		`</div>`
}

// renderConvergence returns the convergence histogram HTML fragment for SSE.
// writeConvergenceHistogram already includes sse-swap="convergence" on its outer div,
// so no extra wrapper is needed — SSE outerHTML swap replaces the div in place.
func (d *Dashboard) renderConvergence() string {
	var b strings.Builder
	writeConvergenceHistogram(&b, d.state.Convergence, d.state.ConvergenceDeadline)
	return b.String()
}

// renderRecentEvents returns the recent events HTML fragment for SSE.
// Must preserve sse-swap and hx-swap attributes so future SSE events continue to work.
func (d *Dashboard) renderRecentEvents() string {
	var b strings.Builder
	b.WriteString(`<div id="events" class="event-list" sse-swap="events" hx-swap="outerHTML" hx-get="/sidebar/events" hx-trigger="every 500ms">`)
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
	pinClass := cssPinDefault
	if pinned {
		pinClass = cssPinPinned
	}
	return "<tr id=\"peer-" + itoa(idx) + "\" hx-swap-oob=\"outerHTML\">" +
		"<td><span class=\"" + pinClass + "\" hx-post=\"/peers/" + itoa(idx) + "/pin\" hx-swap=\"none\"></span></td>" +
		"<td>" + itoa(idx) + "</td>" +
		"<td><span class=\"dot " + ps.Status.CSSClass() + "\"></span> " + ps.Status.String() + "</td>" +
		"<td>" + itoa(ps.RoutesSent) + "</td>" +
		"<td>" + itoa(ps.RoutesRecv) + "</td>" +
		"<td>" + FormatBytes(ps.BytesSent) + "</td>" +
		"<td>" + FormatBytes(ps.BytesRecv) + "</td>" +
		"<td>" + FormatBitRate(ps.throughputOut) + "</td>" +
		"<td>" + FormatBitRate(ps.throughputIn) + "</td>" +
		"<td>" + itoa(ps.ChaosCount) + "</td>" +
		"</tr>"
}

// renderPeerRowInsert returns a table row HTML fragment for inserting a new peer
// into the table body via hx-swap-oob="beforeend". Unlike renderPeerRow (which
// uses outerHTML to update an existing row), this appends a new row to #peer-tbody.
func (d *Dashboard) renderPeerRowInsert(idx int) string {
	ps := d.state.Peers[idx]
	if ps == nil {
		return ""
	}
	pinned := d.state.Active.IsPinned(idx)
	pinClass := cssPinDefault
	if pinned {
		pinClass = cssPinPinned
	}
	return "<tr id=\"peer-" + itoa(idx) + "\" hx-swap-oob=\"beforeend:#peer-tbody\" hx-get=\"/peer/" + itoa(idx) + "\" hx-target=\"#peer-detail\" hx-swap=\"outerHTML\">" +
		"<td><span class=\"" + pinClass + "\" hx-post=\"/peers/" + itoa(idx) + "/pin\" hx-swap=\"none\" hx-trigger=\"click\" onclick=\"event.stopPropagation()\"></span></td>" +
		"<td>" + itoa(idx) + "</td>" +
		"<td><span class=\"dot " + ps.Status.CSSClass() + "\"></span> " + ps.Status.String() + "</td>" +
		"<td>" + itoa(ps.RoutesSent) + "</td>" +
		"<td>" + itoa(ps.RoutesRecv) + "</td>" +
		"<td>" + FormatBytes(ps.BytesSent) + "</td>" +
		"<td>" + FormatBytes(ps.BytesRecv) + "</td>" +
		"<td>" + FormatBitRate(ps.throughputOut) + "</td>" +
		"<td>" + FormatBitRate(ps.throughputIn) + "</td>" +
		"<td>" + itoa(ps.ChaosCount) + "</td>" +
		"</tr>"
}

// renderPeerCell returns a grid cell HTML fragment for a single peer.
// The cell is a colored div with a tooltip showing peer info and hx-get for detail.
func (d *Dashboard) renderPeerCell(idx int) string {
	ps := d.state.Peers[idx]
	if ps == nil {
		return ""
	}
	pulseClass := ""
	if ps.ChaosActive {
		pulseClass = " pulse"
	}
	return `<div id="peer-cell-` + itoa(idx) + `" class="peer-cell ` + ps.Status.CSSClass() + pulseClass +
		`" title="Peer ` + itoa(idx) + `: ` + ps.Status.String() +
		` | Sent: ` + itoa(ps.RoutesSent) + ` Recv: ` + itoa(ps.RoutesRecv) +
		` | Last: ` + eventTypeLabel(ps.LastEvent) +
		`" hx-get="/peer/` + itoa(idx) + `" hx-target="#peer-detail" hx-swap="outerHTML"></div>`
}

// renderPeerRemoval returns an empty element to remove a peer row via hx-swap-oob.
func renderPeerRemoval(idx int) string {
	return "<tr id=\"peer-" + itoa(idx) + "\" hx-swap-oob=\"delete\"></tr>"
}

// itoa is a simple int-to-string helper to avoid importing strconv for HTML rendering.
func itoa(n int) string {
	return fmt.Sprintf("%d", n)
}

// droppedStat returns a warning stat span when events were dropped, or empty string otherwise.
// Only rendered when non-zero to avoid cluttering the dashboard during normal operation.
func droppedStat(n int) string {
	if n == 0 {
		return ""
	}
	return `<span class="stat stat-warn"><span class="stat-label">Dropped </span><span class="stat-value">` + itoa(n) + `</span></span>`
}

// speedStat returns a stat showing the current speed factor, or empty when disabled.
func speedStat(available bool, factor int) string {
	if !available {
		return ""
	}
	return `<span class="stat"><span class="stat-label">Speed </span><span class="stat-value">` + itoa(factor) + `x</span></span>`
}

// syncStat returns a stat showing initial route synchronization progress or duration.
// Shows "syncing N/total" while in progress, or the completed sync duration.
func syncStat(eorCount, peerCount int, syncDuration time.Duration) string {
	if syncDuration > 0 {
		return `<span class="stat" title="Time for all peers to complete initial route announcement (EOR)"><span class="stat-label">Sync </span><span class="stat-value">` + FormatDuration(syncDuration) + `</span></span>`
	}
	if eorCount > 0 {
		return `<span class="stat" title="Peers that completed initial route announcement (EOR)"><span class="stat-label">Sync </span><span class="stat-value">` + itoa(eorCount) + `/` + itoa(peerCount) + `</span></span>`
	}
	return ""
}
