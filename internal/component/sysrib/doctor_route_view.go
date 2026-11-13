// Design: docs/architecture/doctor-and-health-checks.md -- routing-view runtime health.
// Related: route_view_worker.go -- the consumer owns synchronization transitions.

package sysrib

import (
	"strings"
	"sync"

	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

const routeViewDoctorName = "rib-routing-view"
const routeViewDoctorCode = "doctor-rib-routing-view-unavailable"

func routeViewDoctorDeclarations() []rpc.DoctorCheckDecl {
	return []rpc.DoctorCheckDecl{{Name: routeViewDoctorName, Phase: rpc.DoctorPhaseRuntime, Codes: []string{routeViewDoctorCode}}}
}

// routeViewHealth is constructed before configuration, so a missing consumer
// reports initialization. Its zero value and nil pointer are unavailable;
// callbacks MUST retain the monitor its constructor returns.
type routeViewHealth struct {
	mu           sync.Mutex
	name         func() string
	state        routeViewHealthState
	pendingCount int
	resync       metrics.Counter
	pending      metrics.Gauge
	synchronized metrics.Gauge
}

// Each variant represents an observed consumer state. Only a successful
// worker reconciliation constructs routeViewHealthy.
type routeViewHealthState interface{ routeViewHealthState() }
type routeViewInitializing struct{}
type routeViewHealthy struct{}
type routeViewResynchronizing struct{}
type routeViewReconciling struct{}
type routeViewFailed struct{ cause error }
type routeViewHealthStopped struct{}

func (routeViewInitializing) routeViewHealthState()    {}
func (routeViewHealthy) routeViewHealthState()         {}
func (routeViewResynchronizing) routeViewHealthState() {}
func (routeViewReconciling) routeViewHealthState()     {}
func (routeViewFailed) routeViewHealthState()          {}
func (routeViewHealthStopped) routeViewHealthState()   {}

func newRouteViewHealth(reg metrics.Registry, name func() string) *routeViewHealth {
	health := &routeViewHealth{name: name, state: routeViewInitializing{},
		resync:       reg.Counter("ze_rib_view_resync_total", "System routing-view reconstructions started after initialization."),
		pending:      reg.Gauge("ze_rib_view_pending_prefixes", "Distinct prefixes awaiting system routing-view reconciliation."),
		synchronized: reg.Gauge("ze_rib_view_ready", "Whether the system routing view and its output are synchronized.")}
	health.pending.Set(0)
	health.synchronized.Set(0)
	return health
}

func (health *routeViewHealth) findings(name string) ([]rpc.DoctorCheckDiagnostic, error) {
	if name != routeViewDoctorName {
		return nil, nil
	}
	health.mu.Lock()
	defer health.mu.Unlock()
	severity := "warning"
	var message string
	switch state := health.state.(type) {
	case routeViewHealthy:
		return nil, nil
	case routeViewInitializing:
		message = "The system routing view is initializing"
	case routeViewResynchronizing:
		message = "The system routing view is resynchronizing"
	case routeViewReconciling:
		message = "The system routing view is reconciling forwarding output"
	case routeViewFailed:
		severity = "error"
		cause, _, _ := strings.Cut(state.cause.Error(), "\nfix: ")
		message = "The system routing view failed: " + cause
	case routeViewHealthStopped:
		message = "The system routing view is stopped"
	default:
		message = "The system routing-view monitor is unavailable"
	}
	fix := "ze request reload"
	if instance := health.name(); instance != "" {
		fix = "ze request plugin restart " + instance
	}
	return []rpc.DoctorCheckDiagnostic{{Code: routeViewDoctorCode, Severity: severity, Message: message, Fix: fix}}, nil
}

func (health *routeViewHealth) beginResync() {
	health.mu.Lock()
	defer health.mu.Unlock()
	if _, stopped := health.state.(routeViewHealthStopped); stopped {
		return
	}
	health.state = routeViewResynchronizing{}
	health.resync.Inc()
	health.pendingCount = 0
	health.pending.Set(0)
	health.synchronized.Set(0)
}

func (health *routeViewHealth) reset() {
	health.mu.Lock()
	defer health.mu.Unlock()
	if _, stopped := health.state.(routeViewHealthStopped); stopped {
		return
	}
	health.state = routeViewResynchronizing{}
	health.pendingCount = 0
	health.pending.Set(0)
	health.synchronized.Set(0)
}

func (health *routeViewHealth) setPending(count int) {
	health.mu.Lock()
	defer health.mu.Unlock()
	health.pendingCount = count
	health.pending.Set(float64(count))
	health.synchronized.Set(0)
	if _, ready := health.state.(routeViewHealthy); ready {
		health.state = routeViewReconciling{}
	}
}

func (health *routeViewHealth) markSynchronized() {
	health.mu.Lock()
	defer health.mu.Unlock()
	switch health.state.(type) {
	case routeViewFailed, routeViewHealthStopped:
		return
	}
	health.state = routeViewHealthy{}
	health.pendingCount = 0
	health.pending.Set(0)
	health.synchronized.Set(1)
}

func (health *routeViewHealth) fail(err error) {
	health.mu.Lock()
	defer health.mu.Unlock()
	health.state = routeViewFailed{cause: err}
	health.synchronized.Set(0)
}

func (health *routeViewHealth) stop() {
	health.mu.Lock()
	defer health.mu.Unlock()
	if _, failed := health.state.(routeViewFailed); !failed {
		health.state = routeViewHealthStopped{}
	}
	health.pendingCount = 0
	health.pending.Set(0)
	health.synchronized.Set(0)
}
