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

	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

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

// incomingBatch is the JSON payload received from (sysrib, best-change).
// The sysrib publisher carries the family in-band so the EventBus stays
// metadata-free.
type incomingBatch struct {
	Family  string           `json:"family"`
	Replay  bool             `json:"replay,omitempty"`
	Changes []incomingChange `json:"changes"`
}

type incomingChange struct {
	Action   string `json:"action"`
	Prefix   string `json:"prefix"`
	NextHop  string `json:"next-hop"`
	Protocol string `json:"protocol"`
}

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

// processEvent handles a single (sysrib, best-change) payload received on
// the EventBus. The sysrib publisher emits one event per family.
func (f *fibKernel) processEvent(payload string) {
	var batch incomingBatch
	if err := json.Unmarshal([]byte(payload), &batch); err != nil {
		logger().Warn("fib-kernel: failed to unmarshal batch", "error", err)
		return
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	for _, c := range batch.Changes {
		if c.Prefix == "" {
			logger().Warn("fib-kernel: skipping change with empty prefix")
			continue
		}
		if c.Action != "add" && c.Action != "update" && c.Action != "withdraw" {
			logger().Warn("fib-kernel: unrecognized action", "action", c.Action, "prefix", c.Prefix)
			continue
		}
		switch c.Action {
		case "add":
			if err := f.backend.addRoute(c.Prefix, c.NextHop); err != nil {
				logger().Error("fib-kernel: add route failed", "prefix", c.Prefix, "error", err)
				continue
			}
			f.installed[c.Prefix] = c.NextHop
		case "update":
			if err := f.backend.replaceRoute(c.Prefix, c.NextHop); err != nil {
				logger().Error("fib-kernel: replace route failed", "prefix", c.Prefix, "error", err)
				continue
			}
			f.installed[c.Prefix] = c.NextHop
		case "withdraw":
			if err := f.backend.delRoute(c.Prefix); err != nil {
				logger().Error("fib-kernel: del route failed", "prefix", c.Prefix, "error", err)
				continue
			}
			delete(f.installed, c.Prefix)
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
}

// run subscribes to (sysrib, best-change) on the EventBus and blocks until
// ctx is canceled.
func (f *fibKernel) run(ctx context.Context, flushOnStop bool) {
	eb := getEventBus()
	if eb == nil {
		logger().Warn("fib-kernel: no event bus configured")
		return
	}

	unsub := eb.Subscribe(plugin.NamespaceSystemRIB, plugin.EventSystemRIBBestChange, func(payload string) {
		f.processEvent(payload)
	})
	defer unsub()

	// Request full-table replay from sysrib so we populate even if sysrib
	// started before us. Empty payload by convention.
	if _, err := eb.Emit(plugin.NamespaceSystemRIB, plugin.EventSystemRIBReplayRequest, ""); err != nil {
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
