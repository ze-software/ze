// Design: docs/architecture/chaos-web-dashboard.md — chaos test orchestrator
// Overview: main.go — CLI entry and flag parsing
// Related: orchestrator_run.go — orchestrator run loop and reporting setup
// Related: scheduler.go — chaos and route dynamics schedulers
package main

import (
	"sync"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/chaos/peer"
	"codeberg.org/thomas-mangin/ze/internal/chaos/scenario"
	"codeberg.org/thomas-mangin/ze/internal/chaos/validation"
)

// ChaosConfig holds chaos injection parameters passed from CLI flags.
type ChaosConfig struct {
	// Rate is the probability of firing a chaos event per interval (0.0-1.0).
	// Rate=0 disables chaos entirely.
	Rate float64

	// Interval is the time between chaos scheduling checks.
	Interval time.Duration

	// Warmup is the delay before chaos events begin firing.
	Warmup time.Duration
}

// RouteConfig holds route dynamics parameters passed from CLI flags.
type RouteConfig struct {
	// Rate is the probability of firing a route dynamics event per interval (0.0-1.0).
	// Rate=0 disables route dynamics entirely.
	Rate float64

	// Interval is the time between route dynamics checks.
	Interval time.Duration

	// Warmup is the delay before route dynamics events begin firing.
	Warmup time.Duration

	// BaseRoutes is the base route count per peer (used for churn count calculation).
	BaseRoutes int
}

// orchestratorConfig holds all parameters for runOrchestrator.
type orchestratorConfig struct {
	profiles            []scenario.PeerProfile
	seed                uint64
	localAddr           string
	zePort              int
	verbose             bool
	quiet               bool
	start               time.Time
	chaosCfg            ChaosConfig
	routeCfg            RouteConfig
	zePID               int
	eventLog            string
	metricsAddr         string
	webAddr             string
	properties          string
	convergenceDeadline time.Duration

	// restartCh receives a new seed from the web dashboard when restart is requested.
	// When nil, restart is not supported.
	restartCh chan uint64

	// onStop is called by the web dashboard to cancel the current run's context.
	onStop func()
}

// establishedState tracks which peers are currently in Established state.
// It is written by the event-processing goroutine and read by the scheduler
// goroutine, so all access is mutex-protected.
type establishedState struct {
	mu    sync.RWMutex
	peers []bool
}

// newEstablishedState creates an established state tracker for n peers.
func newEstablishedState(n int) *establishedState {
	return &establishedState{peers: make([]bool, n)}
}

// Set marks peer idx as established (true) or not (false).
func (es *establishedState) Set(idx int, val bool) {
	es.mu.Lock()
	es.peers[idx] = val
	es.mu.Unlock()
}

// Snapshot returns a copy of the current established state.
func (es *establishedState) Snapshot() []bool {
	es.mu.RLock()
	snap := make([]bool, len(es.peers))
	copy(snap, es.peers)
	es.mu.RUnlock()
	return snap
}

// isLifecycleEvent returns true for event types that produce immediate
// dashboard output. Used by the orchestrator to restore terminal state
// before printing, since the ze subprocess may corrupt ONLCR at any
// point during its shutdown.
func isLifecycleEvent(t peer.EventType) bool {
	switch t {
	case peer.EventEstablished, peer.EventDisconnected, peer.EventEORSent,
		peer.EventDroppedEvents, peer.EventError, peer.EventChaosExecuted,
		peer.EventReconnecting:
		return true
	case peer.EventRouteSent, peer.EventRouteReceived, peer.EventRouteWithdrawn,
		peer.EventWithdrawalSent, peer.EventRouteAction:
		return false
	}
	return false
}

// EventProcessor routes peer events to the validation model, tracker,
// and convergence tracker. It also maintains aggregate counters.
type EventProcessor struct {
	Model       *validation.Model
	Tracker     *validation.Tracker
	Convergence *validation.Convergence

	// Aggregate counters for the exit summary.
	Announced     int
	Received      int
	ChaosEvents   int
	Reconnections int
	Withdrawn     int
	DroppedEvents int
}

// Process handles a single peer event, updating all validation components.
func (ep *EventProcessor) Process(ev peer.Event) {
	switch ev.Type {
	case peer.EventEstablished:
		ep.Model.SetEstablished(ev.PeerIndex, true)

	case peer.EventRouteSent:
		ep.Model.Announce(ev.PeerIndex, ev.Prefix)
		// Only track convergence for IPv4 unicast — the readLoop can only
		// parse IPv4 NLRI from the trailing UPDATE section, so IPv6 routes
		// would create permanently unresolved pending entries.
		if ev.Prefix.Addr().Is4() {
			ep.Convergence.RecordAnnounce(ev.PeerIndex, ev.Prefix, ev.Time)
		}
		ep.Announced++

	case peer.EventRouteReceived:
		ep.Tracker.RecordReceive(ev.PeerIndex, ev.Prefix)
		ep.Convergence.RecordReceive(ev.PeerIndex, ev.Prefix, ev.Time)
		ep.Received++

	case peer.EventRouteWithdrawn:
		ep.Tracker.RecordWithdraw(ev.PeerIndex, ev.Prefix)

	case peer.EventDisconnected:
		ep.Model.Disconnect(ev.PeerIndex)
		ep.Tracker.ClearPeer(ev.PeerIndex)

	case peer.EventEORSent, peer.EventError:
		// EOR is informational; errors are logged by the caller.

	case peer.EventChaosExecuted:
		ep.ChaosEvents++

	case peer.EventReconnecting:
		ep.Reconnections++

	case peer.EventWithdrawalSent:
		ep.Withdrawn += ev.Count

	case peer.EventRouteAction:
		// Route dynamics actions are informational — counted by the web dashboard.

	case peer.EventDroppedEvents:
		ep.DroppedEvents += ev.Count
	}
}
