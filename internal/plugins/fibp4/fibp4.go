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

// incomingBatch is the JSON payload received from (sysrib, best-change).
// Family is carried in-band so the EventBus stays metadata-free.
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

// processEvent handles a single (sysrib, best-change) payload received on
// the EventBus.
func (f *fibP4) processEvent(payload string) {
	var batch incomingBatch
	if err := json.Unmarshal([]byte(payload), &batch); err != nil {
		logger().Warn("fib-p4: failed to unmarshal batch", "error", err)
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

	unsub := eb.Subscribe(sysribevents.Namespace, sysribevents.EventBestChange, func(payload string) {
		f.processEvent(payload)
	})
	defer unsub()

	// Request full-table replay from sysrib. Empty payload by convention.
	if _, err := eb.Emit(sysribevents.Namespace, sysribevents.EventReplayRequest, ""); err != nil {
		logger().Warn("fib-p4: replay-request emit failed", "error", err)
	}

	logger().Info("fib-p4: running")
	<-ctx.Done()

	if flushOnStop {
		f.flushRoutes()
	}
	logger().Info("fib-p4: stopped")
}
