// Design: docs/architecture/core-design.md -- FIB kernel plugin
// Detail: backend.go -- OS backend abstraction, showInstalled
// Detail: backend_linux.go -- Linux netlink backend
// Detail: backend_other.go -- noop backend for non-Linux
// Detail: monitor.go -- external route change handling
// Detail: monitor_linux.go -- Linux netlink route monitor
// Detail: monitor_other.go -- noop monitor for non-Linux
//
// fib-kernel subscribes to (sysrib, best-change) on the EventBus and
// programs OS routes via netlink (Linux) or route socket (Darwin). Uses a
// custom rtm_protocol ID (RTPROT_ZE=250) to identify ze-installed routes.
// Monitors kernel route changes to detect external modifications and
// re-asserts ze routes when overwritten.
package fibkernel

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"sync/atomic"

	bgptypes "codeberg.org/thomas-mangin/ze/internal/component/bgp/types"
	"codeberg.org/thomas-mangin/ze/internal/core/metrics"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	sysctlevents "codeberg.org/thomas-mangin/ze/internal/plugins/sysctl/events"
	sysribevents "codeberg.org/thomas-mangin/ze/internal/plugins/sysrib/events"
	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

// fibMetrics holds Prometheus metrics for the fib-kernel plugin.
type fibMetrics struct {
	routesInstalled metrics.Gauge      // current installed route count
	routeInstalls   metrics.Counter    // routes successfully added
	routeUpdates    metrics.Counter    // routes successfully replaced
	routeRemovals   metrics.Counter    // routes successfully withdrawn
	errors          metrics.CounterVec // backend operation failures (labels: operation)
}

// fibMetricsPtr stores fib-kernel metrics, set by SetMetricsRegistry.
var fibMetricsPtr atomic.Pointer[fibMetrics]

// SetMetricsRegistry creates fib-kernel metrics from the given registry.
// Called via ConfigureMetrics callback before RunEngine.
func SetMetricsRegistry(reg metrics.Registry) {
	m := &fibMetrics{
		routesInstalled: reg.Gauge("ze_fibkernel_routes_installed", "Current number of ze-installed kernel routes."),
		routeInstalls:   reg.Counter("ze_fibkernel_route_installs_total", "Routes successfully added to kernel."),
		routeUpdates:    reg.Counter("ze_fibkernel_route_updates_total", "Routes successfully replaced in kernel."),
		routeRemovals:   reg.Counter("ze_fibkernel_route_removals_total", "Routes successfully removed from kernel."),
		errors:          reg.CounterVec("ze_fibkernel_errors_total", "Backend operation failures.", []string{"operation"}),
	}
	fibMetricsPtr.Store(m)
}

// loggerPtr is the package-level logger, disabled by default.
var loggerPtr atomic.Pointer[slog.Logger]

func init() {
	d := slogutil.DiscardLogger()
	loggerPtr.Store(d)
}

func logger() *slog.Logger { return loggerPtr.Load() }

func setLogger(l *slog.Logger) {
	if l != nil {
		loggerPtr.Store(l)
	}
}

// eventBusPtr stores the EventBus instance.
var eventBusPtr atomic.Pointer[ze.EventBus]

func setEventBus(eb ze.EventBus) {
	if eb != nil {
		eventBusPtr.Store(&eb)
	}
}

func getEventBus() ze.EventBus {
	p := eventBusPtr.Load()
	if p == nil {
		return nil
	}
	return *p
}

// emitForwardingDefaults publishes forwarding sysctl defaults on the EventBus.
// The sysctl plugin receives these and writes them to the kernel unless
// the user has overridden them via config.
func emitForwardingDefaults() {
	eb := getEventBus()
	if eb == nil {
		logger().Warn("fib-kernel: no event bus, cannot emit forwarding defaults")
		return
	}
	type sysctlDefault struct {
		Key    string `json:"key"`
		Value  string `json:"value"`
		Source string `json:"source"`
	}
	for _, key := range []string{
		"net.ipv4.conf.all.forwarding",
		"net.ipv6.conf.all.forwarding",
	} {
		payload, _ := json.Marshal(sysctlDefault{Key: key, Value: "1", Source: "fib-kernel"})
		if _, err := eb.Emit(sysctlevents.Namespace, sysctlevents.EventDefault, string(payload)); err != nil {
			logger().Warn("fib-kernel: emit sysctl default failed", "key", key, "err", err)
		}
	}
	logger().Info("fib-kernel: emitted forwarding defaults via sysctl")
}

// routeBackend abstracts OS-specific route programming.
type routeBackend interface {
	// addRoute installs a route in the OS routing table.
	addRoute(prefix, nextHop string) error
	// delRoute removes a route from the OS routing table.
	delRoute(prefix string) error
	// replaceRoute atomically replaces a route.
	replaceRoute(prefix, nextHop string) error
	// listZeRoutes returns all routes installed by ze (matching rtprotZE).
	listZeRoutes() ([]installedRoute, error)
	// close releases backend resources.
	close() error
}

// installedRoute represents a route installed in the OS kernel.
type installedRoute struct {
	prefix  string
	nextHop string
}

// incomingBatch aliases the (system-rib, best-change) payload type.
// sysrib publishes these; fib-kernel consumes them to program the kernel FIB.
type incomingBatch = sysribevents.BestChangeBatch

// incomingChange aliases a single entry in an incoming batch.
type incomingChange = sysribevents.BestChangeEntry

// fibKernel manages route installation and monitoring.
type fibKernel struct {
	// installed tracks routes currently installed by ze in the kernel.
	installed map[string]string // prefix -> next-hop
	backend   routeBackend
	mu        sync.RWMutex
}

func newFIBKernel(backend routeBackend) *fibKernel {
	return &fibKernel{
		installed: make(map[string]string),
		backend:   backend,
	}
}

// processEvent handles a single (system-rib, best-change) payload. The
// sysrib publisher emits one event per family with the typed
// *BestChangeBatch payload.
func (f *fibKernel) processEvent(batch *incomingBatch) {
	if batch == nil {
		return
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	for _, c := range batch.Changes {
		if c.Prefix == "" {
			logger().Warn("fib-kernel: skipping change with empty prefix")
			continue
		}
		switch c.Action {
		case bgptypes.RouteActionAdd:
			if err := f.backend.addRoute(c.Prefix, c.NextHop); err != nil {
				logger().Error("fib-kernel: add route failed", "prefix", c.Prefix, "error", err)
				if m := fibMetricsPtr.Load(); m != nil {
					m.errors.With("add").Inc()
				}
				continue
			}
			f.installed[c.Prefix] = c.NextHop
			if m := fibMetricsPtr.Load(); m != nil {
				m.routeInstalls.Inc()
				m.routesInstalled.Set(float64(len(f.installed)))
			}
		case bgptypes.RouteActionUpdate:
			if err := f.backend.replaceRoute(c.Prefix, c.NextHop); err != nil {
				logger().Error("fib-kernel: replace route failed", "prefix", c.Prefix, "error", err)
				if m := fibMetricsPtr.Load(); m != nil {
					m.errors.With("replace").Inc()
				}
				continue
			}
			f.installed[c.Prefix] = c.NextHop
			if m := fibMetricsPtr.Load(); m != nil {
				m.routeUpdates.Inc()
			}
		case bgptypes.RouteActionWithdraw, bgptypes.RouteActionDel:
			if err := f.backend.delRoute(c.Prefix); err != nil {
				logger().Error("fib-kernel: del route failed", "prefix", c.Prefix, "error", err)
				if m := fibMetricsPtr.Load(); m != nil {
					m.errors.With("delete").Inc()
				}
				continue
			}
			delete(f.installed, c.Prefix)
			if m := fibMetricsPtr.Load(); m != nil {
				m.routeRemovals.Inc()
				m.routesInstalled.Set(float64(len(f.installed)))
			}
		case bgptypes.RouteActionUnspecified:
			logger().Warn("fib-kernel: skipping change with unspecified action", "prefix", c.Prefix)
		}
	}
}

// flushRoutes removes all ze-installed routes from the kernel.
func (f *fibKernel) flushRoutes() {
	f.mu.Lock()
	defer f.mu.Unlock()

	for prefix := range f.installed {
		if err := f.backend.delRoute(prefix); err != nil {
			logger().Warn("fib-kernel: flush del failed", "prefix", prefix, "error", err)
		}
	}
	f.installed = make(map[string]string)

	if m := fibMetricsPtr.Load(); m != nil {
		m.routesInstalled.Set(0)
	}
}

// startupSweep implements stale-mark-then-sweep for crash recovery.
// Marks existing ze routes as stale, then removes any not refreshed
// by incoming sysrib events within the sweep window.
func (f *fibKernel) startupSweep() map[string]string {
	routes, err := f.backend.listZeRoutes()
	if err != nil {
		logger().Warn("fib-kernel: list ze routes failed", "error", err)
		return nil
	}

	stale := make(map[string]string, len(routes))
	for _, r := range routes {
		stale[r.prefix] = r.nextHop
	}

	logger().Info("fib-kernel: startup sweep", "stale-routes", len(stale))
	return stale
}

// sweepStale removes routes that are still stale (not refreshed by sysrib).
// Uses write lock to keep f.installed consistent with kernel state.
func (f *fibKernel) sweepStale(stale map[string]string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	for prefix := range stale {
		if _, refreshed := f.installed[prefix]; refreshed {
			continue // Route was refreshed by sysrib.
		}
		if err := f.backend.delRoute(prefix); err != nil {
			logger().Warn("fib-kernel: sweep del failed", "prefix", prefix, "error", err)
		}
		// Ensure installed map stays consistent -- stale route is gone from kernel.
		delete(f.installed, prefix)
	}

	if m := fibMetricsPtr.Load(); m != nil {
		m.routesInstalled.Set(float64(len(f.installed)))
	}
}

// run subscribes to (sysrib, best-change) on the EventBus and blocks until
// ctx is canceled.
func (f *fibKernel) run(ctx context.Context, flushOnStop bool) {
	eb := getEventBus()
	if eb == nil {
		logger().Warn("fib-kernel: no event bus configured")
		return
	}

	unsub := sysribevents.BestChange.Subscribe(eb, f.processEvent)
	defer unsub()

	// Request full-table replay from sysrib so we populate even if sysrib
	// started before us. Signal event, no payload.
	if _, err := sysribevents.ReplayRequest.Emit(eb); err != nil {
		logger().Warn("fib-kernel: replay-request emit failed", "error", err)
	}

	// Start kernel route monitor for external change detection.
	var monitorDone sync.WaitGroup
	monitorDone.Go(func() {
		f.runMonitor(ctx)
	})

	logger().Info("fib-kernel: running")
	<-ctx.Done()

	// Wait for monitor to exit before closing backend.
	monitorDone.Wait()

	if flushOnStop {
		f.flushRoutes()
	}
	logger().Info("fib-kernel: stopped")
}
