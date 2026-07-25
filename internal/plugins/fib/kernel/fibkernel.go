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
// producer-specific rtm_protocol ID to identify fib-kernel routes.
// Monitors kernel route changes to detect external modifications and
// re-asserts ze routes when overwritten.
package fibkernel

import (
	"context"
	"encoding/json"
	"log/slog"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/core/bgp/routeaction"

	sysctlevents "github.com/ze-software/ze/internal/component/sysctl/events"
	sysribevents "github.com/ze-software/ze/internal/component/sysrib/events"
	"github.com/ze-software/ze/internal/core/metrics"
	mplsfibevents "github.com/ze-software/ze/internal/core/mplsfib"
	"github.com/ze-software/ze/internal/core/replay"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/pkg/ze"
)

// fibMetrics holds Prometheus metrics for the fib-kernel plugin.
type fibMetrics struct {
	routesInstalled     metrics.Gauge      // current installed route count
	routeInstalls       metrics.Counter    // routes successfully added
	routeUpdates        metrics.Counter    // routes successfully replaced
	routeRemovals       metrics.Counter    // routes successfully withdrawn
	errors              metrics.CounterVec // backend operation failures (labels: operation)
	mplsRoutesInstalled metrics.Gauge      // current MPLS labeled route count
	mplsInstalls        metrics.Counter    // MPLS routes successfully programmed
}

// fibMetricsPtr stores fib-kernel metrics, set by SetMetricsRegistry.
var fibMetricsPtr atomic.Pointer[fibMetrics]

// SetMetricsRegistry creates fib-kernel metrics from the given registry.
// Called via ConfigureMetrics callback before RunEngine.
func SetMetricsRegistry(reg metrics.Registry) {
	m := &fibMetrics{
		routesInstalled:     reg.Gauge("ze_fibkernel_routes_installed", "Current number of ze-installed kernel routes."),
		routeInstalls:       reg.Counter("ze_fibkernel_route_installs_total", "Routes successfully added to kernel."),
		routeUpdates:        reg.Counter("ze_fibkernel_route_updates_total", "Routes successfully replaced in kernel."),
		routeRemovals:       reg.Counter("ze_fibkernel_route_removals_total", "Routes successfully removed from kernel."),
		errors:              reg.CounterVec("ze_fibkernel_errors_total", "Backend operation failures.", []string{"operation"}),
		mplsRoutesInstalled: reg.Gauge("ze_fibkernel_mpls_routes_installed", "Current number of MPLS labeled routes installed."),
		mplsInstalls:        reg.Counter("ze_fibkernel_mpls_installs_total", "MPLS routes successfully programmed."),
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
	// listZeRoutes returns all routes installed by fib-kernel.
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

const reportSourceFIB = "fib"
const reportCodeFIBSyncFailure = "fib-sync-failure"
const reportCodeFIBOrphan = "fib-orphan"
const reportCodeFIBProgrammingLag = "fib-programming-lag"

const fibLagTimeout = 30 * time.Second
const maxPendingEntries = 10000

// fibKernel manages route installation and monitoring.
type fibKernel struct {
	// installed tracks routes currently installed by ze in the kernel.
	installed map[string]string // prefix -> next-hop
	// mplsInstalled tracks which installed prefixes carry MPLS labels (push).
	mplsInstalled map[string]bool
	// mplsSwaps tracks installed AF_MPLS swap/pop entries by incoming label.
	mplsSwaps map[uint32]bool
	// pending tracks routes that failed FIB programming with their first
	// failure time. Used for fib-programming-lag detection (AC-14).
	pending map[string]time.Time
	backend routeBackend
	mu      sync.RWMutex
}

// asRichBackend returns the richRouteBackend if the backend supports it.
func (f *fibKernel) asRichBackend() richRouteBackend {
	if rb, ok := f.backend.(richRouteBackend); ok {
		return rb
	}
	return nil
}

func newFIBKernel(backend routeBackend) *fibKernel {
	return &fibKernel{
		installed:     make(map[string]string),
		mplsInstalled: make(map[string]bool),
		mplsSwaps:     make(map[uint32]bool),
		pending:       make(map[string]time.Time),
		backend:       backend,
	}
}

// hasRichFields reports whether a change carries attributes beyond prefix+next-hop.
func hasRichFields(c *incomingChange) bool {
	return c.RouteType != 0 || c.Metric != 0 || c.TableID != 0 ||
		len(c.ECMPPaths) > 0 || len(c.Labels) > 0 || c.SRv6SID.IsValid() || len(c.Backup) > 0
}

func changeToRichRoute(c *incomingChange) RichRoute {
	return RichRoute{
		Prefix:    c.Prefix,
		NextHop:   c.NextHop,
		RouteType: c.RouteType,
		Metric:    c.Metric,
		TableID:   c.TableID,
		Labels:    c.Labels,
		SRv6SID:   c.SRv6SID,
		ECMPPaths: c.ECMPPaths,
		Backup:    c.Backup,
	}
}

func (f *fibKernel) addChange(c *incomingChange, pfx string, rb richRouteBackend) error {
	if len(c.Labels) > 0 {
		if err := validateMPLSLabels(c.Labels); err != nil {
			return err
		}
	}
	if rb != nil && hasRichFields(c) {
		return rb.addRichRoute(changeToRichRoute(c))
	}
	return f.backend.addRoute(pfx, c.NextHop.String())
}

func (f *fibKernel) replaceChange(c *incomingChange, pfx string, rb richRouteBackend) error {
	if len(c.Labels) > 0 {
		if err := validateMPLSLabels(c.Labels); err != nil {
			return err
		}
	}
	if rb != nil && hasRichFields(c) {
		return rb.replaceRichRoute(changeToRichRoute(c))
	}
	return f.backend.replaceRoute(pfx, c.NextHop.String())
}

func (f *fibKernel) delChange(c *incomingChange, pfx string, rb richRouteBackend) error {
	if rb != nil {
		return rb.delRichRoute(c.Prefix, c.TableID)
	}
	return f.backend.delRoute(pfx)
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

	now := time.Now()
	rb := f.asRichBackend()

	for i := range batch.Changes {
		c := &batch.Changes[i]
		if !c.Prefix.IsValid() {
			logger().Warn("fib-kernel: skipping change with empty prefix")
			continue
		}
		pfx := c.Prefix.String()
		switch c.Action.Verb() {
		case routeaction.VerbInstall:
			if err := f.addChange(c, pfx, rb); err != nil {
				logger().Error("fib-kernel: add route failed", "prefix", c.Prefix, "error", err)
				if m := fibMetricsPtr.Load(); m != nil {
					m.errors.With("add").Inc()
				}
				report.RaiseError(reportSourceFIB, reportCodeFIBSyncFailure, pfx,
					"FIB add failed: "+err.Error(), map[string]any{"operation": "add", "prefix": pfx})
				f.trackPendingLocked(pfx, now)
				continue
			}
			f.installed[pfx] = c.NextHop.String()
			delete(f.pending, pfx)
			if len(c.Labels) > 0 {
				f.mplsInstalled[pfx] = true
			}
			if m := fibMetricsPtr.Load(); m != nil {
				m.routeInstalls.Inc()
				m.routesInstalled.Set(float64(len(f.installed)))
				if len(c.Labels) > 0 {
					m.mplsInstalls.Inc()
					m.mplsRoutesInstalled.Set(float64(f.mplsCountLocked()))
				}
			}
		case routeaction.VerbReplace:
			if err := f.replaceChange(c, pfx, rb); err != nil {
				logger().Error("fib-kernel: replace route failed", "prefix", c.Prefix, "error", err)
				if m := fibMetricsPtr.Load(); m != nil {
					m.errors.With("replace").Inc()
				}
				report.RaiseError(reportSourceFIB, reportCodeFIBSyncFailure, pfx,
					"FIB replace failed: "+err.Error(), map[string]any{"operation": "replace", "prefix": pfx})
				f.trackPendingLocked(pfx, now)
				continue
			}
			f.installed[pfx] = c.NextHop.String()
			delete(f.pending, pfx)
			// A replace may toggle a prefix between labeled and unlabeled;
			// keep the MPLS membership set in sync so the gauge is accurate.
			wasMPLS := f.mplsInstalled[pfx]
			if len(c.Labels) > 0 {
				f.mplsInstalled[pfx] = true
			} else {
				delete(f.mplsInstalled, pfx)
			}
			if m := fibMetricsPtr.Load(); m != nil {
				m.routeUpdates.Inc()
				if len(c.Labels) > 0 && !wasMPLS {
					m.mplsInstalls.Inc()
				}
				m.mplsRoutesInstalled.Set(float64(f.mplsCountLocked()))
			}
		case routeaction.VerbRemove:
			if err := f.delChange(c, pfx, rb); err != nil {
				logger().Error("fib-kernel: del route failed", "prefix", c.Prefix, "error", err)
				if m := fibMetricsPtr.Load(); m != nil {
					m.errors.With("delete").Inc()
				}
				report.RaiseError(reportSourceFIB, reportCodeFIBSyncFailure, pfx,
					"FIB delete failed: "+err.Error(), map[string]any{"operation": "delete", "prefix": pfx})
				continue
			}
			delete(f.installed, pfx)
			delete(f.pending, pfx)
			wasMPLS := f.mplsInstalled[pfx]
			delete(f.mplsInstalled, pfx)
			if m := fibMetricsPtr.Load(); m != nil {
				m.routeRemovals.Inc()
				m.routesInstalled.Set(float64(len(f.installed)))
				if wasMPLS {
					m.mplsRoutesInstalled.Set(float64(f.mplsCountLocked()))
				}
			}
		case routeaction.VerbSkip:
			logger().Warn("fib-kernel: skipping change with unspecified action", "prefix", c.Prefix)
		}
	}

	f.checkPendingLagLocked(now)
}

// trackPendingLocked records a failed route's first-failure time.
// Only sets the timestamp on the first failure; subsequent failures
// for the same prefix preserve the original time.
func (f *fibKernel) trackPendingLocked(pfx string, now time.Time) {
	if _, exists := f.pending[pfx]; !exists {
		if len(f.pending) >= maxPendingEntries {
			return
		}
		f.pending[pfx] = now
	}
}

// checkPendingLagLocked raises or clears the fib-programming-lag warning
// based on whether any pending routes have been failing for >30 seconds.
func (f *fibKernel) checkPendingLagLocked(now time.Time) {
	lagging := 0
	for _, firstFail := range f.pending {
		if now.Sub(firstFail) > fibLagTimeout {
			lagging++
		}
	}
	if lagging > 0 {
		report.RaiseWarning(reportSourceFIB, reportCodeFIBProgrammingLag, "pending",
			strconv.Itoa(lagging)+" routes pending FIB install for >30s",
			map[string]any{"lagging": lagging, "total_pending": len(f.pending)})
	} else {
		report.ClearWarning(reportSourceFIB, reportCodeFIBProgrammingLag, "pending")
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
// Raises a fib-orphan warning if any orphan routes are swept (AC-13).
func (f *fibKernel) sweepStale(stale map[string]string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	orphanCount := 0
	for prefix := range stale {
		if _, refreshed := f.installed[prefix]; refreshed {
			continue // Route was refreshed by sysrib.
		}
		if err := f.backend.delRoute(prefix); err != nil {
			logger().Warn("fib-kernel: sweep del failed", "prefix", prefix, "error", err)
		}
		// Ensure installed map stays consistent -- stale route is gone from kernel.
		delete(f.installed, prefix)
		orphanCount++
	}

	if orphanCount > 0 {
		report.RaiseWarning(reportSourceFIB, reportCodeFIBOrphan, "sweep",
			strconv.Itoa(orphanCount)+" orphan routes removed from FIB (no RIB entry)",
			map[string]any{"orphan_count": orphanCount})
	} else {
		report.ClearWarning(reportSourceFIB, reportCodeFIBOrphan, "sweep")
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

	// MPLS label-switching entries from label-distribution sources (RSVP-TE,
	// LDP). fib-kernel is the single kernel-FIB owner, so it programs these too.
	unsubMPLS := mplsfibevents.EntryChange.Subscribe(eb, f.handleMPLSEntry)
	defer unsubMPLS()

	// Request full-table replay from sysrib so we populate even if sysrib
	// started before us. Broadcast hop: the token addresses every consumer.
	if _, err := sysribevents.ReplayRequest.Emit(eb, &replay.Request{ReplayID: replay.Broadcast}); err != nil {
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
