// Design: docs/architecture/core-design.md -- FIB P4 plugin
// Detail: backend.go -- P4 backend interface and noop implementation
//
// fib-p4 subscribes to (sysrib, best-change) on the EventBus and programs
// a P4 switch via gRPC/P4Runtime. Cross-OS plugin (generic Go, no
// build tags). The backend interface abstracts P4Runtime so the
// plugin logic is testable without gRPC dependencies.
package fibp4

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"sync/atomic"

	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	sysribevents "codeberg.org/thomas-mangin/ze/internal/plugins/sysrib/events"
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

// p4Backend abstracts P4 switch programming via P4Runtime gRPC.
type p4Backend interface {
	// addRoute installs a forwarding entry in the P4 switch.
	addRoute(prefix, nextHop string) error
	// delRoute removes a forwarding entry from the P4 switch.
	delRoute(prefix string) error
	// replaceRoute atomically replaces a forwarding entry.
	replaceRoute(prefix, nextHop string) error
	// close releases the gRPC connection.
	close() error
}

// incomingBatch aliases the (system-rib, best-change) payload type.
type incomingBatch = sysribevents.BestChangeBatch

// incomingChange aliases a single entry in an incoming batch.
type incomingChange = sysribevents.BestChangeEntry

// fibP4 manages P4 switch route programming.
type fibP4 struct {
	installed map[string]string // prefix -> next-hop
	backend   p4Backend
	mu        sync.RWMutex
}

func newFIBP4(backend p4Backend) *fibP4 {
	return &fibP4{
		installed: make(map[string]string),
		backend:   backend,
	}
}

// processEvent handles a single (system-rib, best-change) payload received
// via the typed BestChange handle.
func (f *fibP4) processEvent(batch *incomingBatch) {
	if batch == nil {
		return
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	for _, c := range batch.Changes {
		if c.Prefix == "" {
			logger().Warn("fib-p4: skipping change with empty prefix")
			continue
		}
		if c.Action != "add" && c.Action != "update" && c.Action != "withdraw" {
			logger().Warn("fib-p4: unrecognized action", "action", c.Action, "prefix", c.Prefix)
			continue
		}
		switch c.Action {
		case "add":
			if err := f.backend.addRoute(c.Prefix, c.NextHop); err != nil {
				logger().Error("fib-p4: add route failed", "prefix", c.Prefix, "error", err)
				continue
			}
			f.installed[c.Prefix] = c.NextHop
		case "update":
			if err := f.backend.replaceRoute(c.Prefix, c.NextHop); err != nil {
				logger().Error("fib-p4: replace route failed", "prefix", c.Prefix, "error", err)
				continue
			}
			f.installed[c.Prefix] = c.NextHop
		case "withdraw":
			if err := f.backend.delRoute(c.Prefix); err != nil {
				logger().Error("fib-p4: del route failed", "prefix", c.Prefix, "error", err)
				continue
			}
			delete(f.installed, c.Prefix)
		}
	}
}

// flushRoutes removes all installed entries from the P4 switch.
func (f *fibP4) flushRoutes() {
	f.mu.Lock()
	defer f.mu.Unlock()

	for prefix := range f.installed {
		if err := f.backend.delRoute(prefix); err != nil {
			logger().Warn("fib-p4: flush del failed", "prefix", prefix, "error", err)
		}
	}
	f.installed = make(map[string]string)
}

// showInstalled returns the currently installed routes as JSON.
func (f *fibP4) showInstalled() string {
	f.mu.RLock()
	defer f.mu.RUnlock()

	type entry struct {
		Prefix  string `json:"prefix"`
		NextHop string `json:"next-hop"`
	}

	entries := make([]entry, 0, len(f.installed))
	for prefix, nextHop := range f.installed {
		entries = append(entries, entry{Prefix: prefix, NextHop: nextHop})
	}

	data, err := json.Marshal(entries)
	if err != nil {
		return "[]"
	}
	return string(data)
}

// run subscribes to (sysrib, best-change) on the EventBus and blocks until
// ctx is canceled.
func (f *fibP4) run(ctx context.Context, flushOnStop bool) {
	eb := getEventBus()
	if eb == nil {
		logger().Warn("fib-p4: no event bus configured")
		return
	}

	unsub := sysribevents.BestChange.Subscribe(eb, f.processEvent)
	defer unsub()

	// Request full-table replay from sysrib.
	if _, err := sysribevents.ReplayRequest.Emit(eb); err != nil {
		logger().Warn("fib-p4: replay-request emit failed", "error", err)
	}

	logger().Info("fib-p4: running")
	<-ctx.Done()

	if flushOnStop {
		f.flushRoutes()
	}
	logger().Info("fib-p4: stopped")
}
