// Design: docs/research/vpp-deployment-reference.md -- VPP FIB route programming
// Detail: backend.go -- GoVPP backend and mock for testing
//
// fib-vpp subscribes to (system-rib, best-change) on the EventBus and programs
// VPP's FIB directly via GoVPP binary API. Mirrors the fib-p4/fib-kernel pattern
// with a GoVPP backend instead of noop/netlink.
package fibvpp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/netip"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"codeberg.org/thomas-mangin/ze/internal/core/events"
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

func setFibVPPLogger(l *slog.Logger) {
	if l != nil {
		loggerPtr.Store(l)
	}
}

// eventBusPtr stores the EventBus instance.
var eventBusPtr atomic.Pointer[ze.EventBus]

func setFibVPPEventBus(eb ze.EventBus) {
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

// fibVPPConfig holds parsed fib.vpp config values.
type fibVPPConfig struct {
	tableID uint32
}

// parseFibVPPConfig extracts fib-vpp settings from config section JSON.
func parseFibVPPConfig(data string) (*fibVPPConfig, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(data), &raw); err != nil {
		return nil, fmt.Errorf("fib-vpp config: %w", err)
	}
	known := map[string]bool{"enabled": true, "table-id": true, "batch-size": true, "batch-interval-ms": true}
	for k := range raw {
		if !known[k] {
			return nil, fmt.Errorf("fib-vpp config: unknown key %q", k)
		}
	}

	cfg := &fibVPPConfig{}
	if v, ok := raw["table-id"]; ok {
		s := strings.Trim(string(v), `"`)
		n, err := strconv.ParseUint(s, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("fib-vpp table-id: %w", err)
		}
		cfg.tableID = uint32(n)
	}
	return cfg, nil
}

// incomingBatch is the JSON payload received from (system-rib, best-change).
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

// fibVPP manages VPP FIB route programming.
type fibVPP struct {
	installed map[string]string // prefix -> next-hop
	backend   vppBackend
	mu        sync.RWMutex
}

func newFibVPP(backend vppBackend) *fibVPP {
	return &fibVPP{
		installed: make(map[string]string),
		backend:   backend,
	}
}

// processEvent handles a single (system-rib, best-change) payload.
func (f *fibVPP) processEvent(payload string) {
	var batch incomingBatch
	if err := json.Unmarshal([]byte(payload), &batch); err != nil {
		logger().Warn("fib-vpp: failed to unmarshal batch", "error", err)
		return
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	for _, c := range batch.Changes {
		if c.Prefix == "" {
			logger().Warn("fib-vpp: skipping change with empty prefix")
			continue
		}
		if c.Action != "add" && c.Action != "update" && c.Action != "withdraw" {
			logger().Warn("fib-vpp: unrecognized action", "action", c.Action, "prefix", c.Prefix)
			continue
		}

		prefix, err := netip.ParsePrefix(c.Prefix)
		if err != nil {
			logger().Warn("fib-vpp: invalid prefix", "prefix", c.Prefix, "error", err)
			continue
		}

		switch c.Action {
		case "add":
			nextHop, nhErr := netip.ParseAddr(c.NextHop)
			if nhErr != nil {
				logger().Warn("fib-vpp: invalid next-hop", "next-hop", c.NextHop, "error", nhErr)
				continue
			}
			if err := f.backend.addRoute(prefix, nextHop); err != nil {
				logger().Error("fib-vpp: add route failed", "prefix", c.Prefix, "error", err)
				continue
			}
			f.installed[c.Prefix] = c.NextHop
		case "update":
			nextHop, nhErr := netip.ParseAddr(c.NextHop)
			if nhErr != nil {
				logger().Warn("fib-vpp: invalid next-hop", "next-hop", c.NextHop, "error", nhErr)
				continue
			}
			if err := f.backend.replaceRoute(prefix, nextHop); err != nil {
				logger().Error("fib-vpp: replace route failed", "prefix", c.Prefix, "error", err)
				continue
			}
			f.installed[c.Prefix] = c.NextHop
		case "withdraw":
			if err := f.backend.delRoute(prefix); err != nil {
				logger().Error("fib-vpp: del route failed", "prefix", c.Prefix, "error", err)
				continue
			}
			delete(f.installed, c.Prefix)
		}
	}
}

// flushRoutes removes all installed entries from VPP FIB.
func (f *fibVPP) flushRoutes() {
	f.mu.Lock()
	defer f.mu.Unlock()

	for prefixStr := range f.installed {
		prefix, err := netip.ParsePrefix(prefixStr)
		if err != nil {
			continue
		}
		if err := f.backend.delRoute(prefix); err != nil {
			logger().Warn("fib-vpp: flush del failed", "prefix", prefixStr, "error", err)
		}
	}
	f.installed = make(map[string]string)
}

// showInstalled returns the currently installed routes as JSON.
func (f *fibVPP) showInstalled() string {
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

// run subscribes to (system-rib, best-change) on the EventBus and blocks until
// ctx is canceled.
func (f *fibVPP) run(ctx context.Context, flushOnStop bool) {
	eb := getEventBus()
	if eb == nil {
		logger().Warn("fib-vpp: no event bus configured")
		return
	}

	unsub := eb.Subscribe(events.NamespaceSystemRIB, events.EventSystemRIBBestChange, func(payload string) {
		f.processEvent(payload)
	})
	defer unsub()

	// Request full-table replay from sysrib.
	if _, err := eb.Emit(events.NamespaceSystemRIB, events.EventSystemRIBReplayRequest, ""); err != nil {
		logger().Warn("fib-vpp: replay-request emit failed", "error", err)
	}

	logger().Info("fib-vpp: running")
	<-ctx.Done()

	if flushOnStop {
		f.flushRoutes()
	}
	logger().Info("fib-vpp: stopped")
}
