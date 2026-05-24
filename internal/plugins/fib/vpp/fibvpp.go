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
	"errors"
	"fmt"
	"log/slog"
	"net/netip"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	bgptypes "codeberg.org/thomas-mangin/ze/internal/component/bgp/types"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	sysribevents "codeberg.org/thomas-mangin/ze/internal/plugins/sysrib/events"
	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

var (
	errFibVppConfigSectionMissingFib  = errors.New("fib-vpp: config section missing 'fib' root")
	errFibVppConfigSectionMissingFib2 = errors.New("fib-vpp: config section missing 'fib/vpp' subtree")
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

// fibVPPConfig holds parsed fib/vpp config values.
// batch-size and batch-interval-ms are accepted by the parser (YANG leaves
// exist) but not consumed: sysRIB already delivers per-family batches and
// VPP's IPRouteAddDel is per-route, so cross-emission accumulation adds
// complexity for zero benefit today.
type fibVPPConfig struct {
	tableID uint32
}

// parseFibVPPConfigSection parses a wrapped fib-vpp config section delivered
// by the plugin-server ExtractConfigSubtree helper. For ConfigRoots "fib/vpp"
// the helper wraps the subtree as `{"fib":{"vpp":{...}}}`. This function
// unwraps both levels and delegates to parseFibVPPConfig.
//
// Use this from the plugin OnConfigure callback. Use parseFibVPPConfig
// directly from tests or callers that already hold the inner subtree.
func parseFibVPPConfigSection(data string) (*fibVPPConfig, error) {
	var outer map[string]json.RawMessage
	if err := json.Unmarshal([]byte(data), &outer); err != nil {
		return nil, fmt.Errorf("fib-vpp: parse wrapped config: %w", err)
	}
	fibRaw, ok := outer["fib"]
	if !ok {
		return nil, errFibVppConfigSectionMissingFib
	}
	var fib map[string]json.RawMessage
	if err := json.Unmarshal(fibRaw, &fib); err != nil {
		return nil, fmt.Errorf("fib-vpp: parse fib container: %w", err)
	}
	vppRaw, ok := fib["vpp"]
	if !ok {
		return nil, errFibVppConfigSectionMissingFib2
	}
	return parseFibVPPConfig(string(vppRaw))
}

// parseFibVPPConfig extracts fib-vpp settings from config section JSON.
func parseFibVPPConfig(data string) (*fibVPPConfig, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(data), &raw); err != nil {
		return nil, fmt.Errorf("fib-vpp config: %w", err)
	}
	known := map[string]bool{"enabled": true, "table-id": true}
	deferred := map[string]bool{"batch-size": true, "batch-interval-ms": true}
	for k := range raw {
		if deferred[k] {
			return nil, fmt.Errorf("fib-vpp config: %q is not yet implemented (deferred)", k)
		}
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

// incomingBatch aliases the (system-rib, best-change) payload type.
type incomingBatch = sysribevents.BestChangeBatch

// incomingChange aliases a single entry in an incoming batch.
type incomingChange = sysribevents.BestChangeEntry

// fibVPP manages VPP FIB route programming.
type fibVPP struct {
	installed     map[string]string // prefix -> next-hop (IP routes)
	mplsInstalled map[string]bool   // prefix -> true (MPLS labeled routes)
	srv6Installed map[string]bool   // prefix -> true (SRv6 steered routes)
	backend       vppBackend
	mplsBackend   mplsBackend
	srv6Backend   srv6Backend
	mu            sync.RWMutex
}

func newFibVPP(backend vppBackend) *fibVPP {
	return &fibVPP{
		installed:     make(map[string]string),
		mplsInstalled: make(map[string]bool),
		srv6Installed: make(map[string]bool),
		backend:       backend,
	}
}

// ecmpNextHops collects all next-hops (primary + ECMP siblings) from a change.
func ecmpNextHops(c *incomingChange) []netip.Addr {
	nextHops := make([]netip.Addr, 0, len(c.ECMPPaths)+1)
	if c.NextHop.IsValid() {
		nextHops = append(nextHops, c.NextHop)
	}
	for _, p := range c.ECMPPaths {
		if p.NextHop.IsValid() {
			nextHops = append(nextHops, p.NextHop)
		}
	}
	return nextHops
}

// addVPPRoute dispatches to multi-path or single-path based on ECMP presence.
func (f *fibVPP) addVPPRoute(c *incomingChange) error {
	if len(c.ECMPPaths) > 0 {
		return f.backend.addMultiPath(c.Prefix, ecmpNextHops(c), c.TableID)
	}
	return f.backend.addRoute(c.Prefix, c.NextHop)
}

// replaceVPPRoute dispatches to multi-path or single-path replace.
func (f *fibVPP) replaceVPPRoute(c *incomingChange) error {
	if len(c.ECMPPaths) > 0 {
		return f.backend.addMultiPath(c.Prefix, ecmpNextHops(c), c.TableID)
	}
	return f.backend.replaceRoute(c.Prefix, c.NextHop)
}

// processEvent handles a single (system-rib, best-change) payload received
// via the typed BestChange handle.
func (f *fibVPP) processEvent(batch *incomingBatch) {
	if batch == nil {
		return
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	for i := range batch.Changes {
		c := &batch.Changes[i]
		if !c.Prefix.IsValid() {
			logger().Warn("fib-vpp: skipping change with empty prefix")
			continue
		}
		if c.SRv6SID.IsValid() || f.srv6Installed[c.Prefix.String()] {
			f.processSRv6Change(c)
			continue
		}
		if len(c.Labels) > 0 || f.mplsInstalled[c.Prefix.String()] {
			f.processMPLSChange(c)
			continue
		}
		switch c.Action {
		case bgptypes.RouteActionAdd:
			if err := f.addVPPRoute(c); err != nil {
				logger().Error("fib-vpp: add route failed", "prefix", c.Prefix, "error", err)
				continue
			}
			f.installed[c.Prefix.String()] = c.NextHop.String()
			if m := fibVPPMetricsPtr.Load(); m != nil {
				m.routeInstalls.Inc()
				m.routesInstalled.Set(float64(len(f.installed)))
			}
		case bgptypes.RouteActionUpdate:
			if err := f.replaceVPPRoute(c); err != nil {
				logger().Error("fib-vpp: replace route failed", "prefix", c.Prefix, "error", err)
				continue
			}
			f.installed[c.Prefix.String()] = c.NextHop.String()
			if m := fibVPPMetricsPtr.Load(); m != nil {
				m.routeUpdates.Inc()
			}
		case bgptypes.RouteActionWithdraw, bgptypes.RouteActionDel:
			if err := f.backend.delRoute(c.Prefix); err != nil {
				logger().Error("fib-vpp: del route failed", "prefix", c.Prefix, "error", err)
				continue
			}
			delete(f.installed, c.Prefix.String())
			if m := fibVPPMetricsPtr.Load(); m != nil {
				m.routeRemovals.Inc()
				m.routesInstalled.Set(float64(len(f.installed)))
			}
		case bgptypes.RouteActionUnspecified:
			logger().Warn("fib-vpp: skipping change with unspecified action", "prefix", c.Prefix)
		}
	}
}

// processMPLSChange handles a single best-change entry with MPLS labels.
// Caller must hold f.mu.
func (f *fibVPP) processMPLSChange(c *incomingChange) {
	if f.mplsBackend == nil {
		logger().Warn("fib-vpp: MPLS change but no MPLS backend configured", "prefix", c.Prefix)
		return
	}
	pfxStr := c.Prefix.String()
	switch c.Action { //nolint:exhaustive // Unspecified is a no-op for MPLS
	case bgptypes.RouteActionAdd, bgptypes.RouteActionUpdate:
		if err := f.mplsBackend.addMPLSRoute(c.Prefix, c.NextHop, c.Labels); err != nil {
			logger().Error("fib-vpp: MPLS add failed", "prefix", c.Prefix, "error", err)
			return
		}
		f.mplsInstalled[pfxStr] = true
		if m := fibVPPMetricsPtr.Load(); m != nil {
			m.routeInstalls.Inc()
		}
	case bgptypes.RouteActionWithdraw, bgptypes.RouteActionDel:
		if err := f.mplsBackend.delMPLSRoute(c.Prefix, c.Labels); err != nil {
			logger().Error("fib-vpp: MPLS del failed", "prefix", c.Prefix, "error", err)
			return
		}
		delete(f.mplsInstalled, pfxStr)
		if m := fibVPPMetricsPtr.Load(); m != nil {
			m.routeRemovals.Inc()
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

	if f.mplsBackend != nil {
		for prefixStr := range f.mplsInstalled {
			prefix, err := netip.ParsePrefix(prefixStr)
			if err != nil {
				continue
			}
			if err := f.mplsBackend.delMPLSRoute(prefix, nil); err != nil {
				logger().Warn("fib-vpp: flush MPLS del failed", "prefix", prefixStr, "error", err)
			}
		}
	}
	f.mplsInstalled = make(map[string]bool)

	if f.srv6Backend != nil {
		for prefixStr := range f.srv6Installed {
			prefix, err := netip.ParsePrefix(prefixStr)
			if err != nil {
				continue
			}
			if err := f.srv6Backend.delSRv6Steer(prefix, 0); err != nil {
				logger().Warn("fib-vpp: flush SRv6 del failed", "prefix", prefixStr, "error", err)
			}
		}
	}
	f.srv6Installed = make(map[string]bool)

	if m := fibVPPMetricsPtr.Load(); m != nil {
		m.routesInstalled.Set(0)
	}
}

// showInstalled returns the currently installed routes as JSON.
func (f *fibVPP) showInstalled() string {
	f.mu.RLock()
	defer f.mu.RUnlock()

	type entry struct {
		Prefix  string `json:"prefix"`
		NextHop string `json:"next-hop,omitempty"`
		MPLS    bool   `json:"mpls,omitempty"`
	}

	entries := make([]entry, 0, len(f.installed)+len(f.mplsInstalled))
	for prefix, nextHop := range f.installed {
		entries = append(entries, entry{Prefix: prefix, NextHop: nextHop})
	}
	for prefix := range f.mplsInstalled {
		entries = append(entries, entry{Prefix: prefix, MPLS: true})
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

	unsub := sysribevents.BestChange.Subscribe(eb, f.processEvent)
	defer unsub()

	// Request full-table replay from sysrib.
	if _, err := sysribevents.ReplayRequest.Emit(eb); err != nil {
		logger().Warn("fib-vpp: replay-request emit failed", "error", err)
	}

	logger().Info("fib-vpp: running")
	<-ctx.Done()

	if flushOnStop {
		f.flushRoutes()
	}
	logger().Info("fib-vpp: stopped")
}
