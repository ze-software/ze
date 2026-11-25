// Design: docs/architecture/chaos-web-dashboard.md -- chaos test orchestrator types

package orchestrator

import (
	"sync"
	"time"

	"github.com/ze-software/ze/internal/chaos/engine"
	"github.com/ze-software/ze/internal/chaos/peer"
	"github.com/ze-software/ze/internal/chaos/scenario"
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

	// EnabledActions is the set of v2 action types enabled for scheduling.
	// Parsed from --chaos-actions flag. Nil means only legacy actions.
	EnabledActions []engine.ActionType
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

// orchestratorConfig holds all parameters for RunOrchestrator.
type orchestratorConfig struct {
	Profiles            []scenario.PeerProfile
	Target              scenario.Target
	Seed                uint64
	LocalAddr           string
	ZePort              int
	Verbose             bool
	Quiet               bool
	Start               time.Time
	ChaosCfg            ChaosConfig
	RouteCfg            RouteConfig
	ZePID               int
	EventLog            string
	MRTFile             string
	MetricsAddr         string
	WebAddr             string
	McpAddr             string
	Properties          string
	ConvergenceDeadline time.Duration

	// RestartCh receives a new seed from the web dashboard when restart is requested.
	// When nil, restart is not supported.
	RestartCh chan uint64

	// OnStop is called by the web dashboard to cancel the current run's context.
	OnStop func()
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
// dashboard output.
func isLifecycleEvent(t peer.EventType) bool {
	// Only events needing immediate terminal output satisfy this predicate.
	//exhaustive:ignore
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
