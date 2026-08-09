// Design: docs/architecture/ldp/mpls-ldp.md -- dynamic LDP interface reload (AC-9)
// Related: register.go -- OnStarted seeds it, OnConfigure reconciles on reload
// Related: discovery.go -- discoverOnInterface is the per-interface worker
//
// RFC 5036 Section 2.4.1 runs Basic Discovery per link. The manager owns the set
// of per-interface discovery goroutines so a config reload that adds or removes
// an LDP interface starts or stops discovery on it without restarting the engine.
// Stopping discovery stops the Hellos on that link; the neighbor's adjacency then
// ages out through the existing hold-timer path, which tears its session down.
package ldp

import (
	"context"
	"log/slog"
	"sync"
)

// discoveryStartFunc starts discovery on one interface and blocks until ctx is
// canceled. ifName is "" for the system-assigned multicast interface.
type discoveryStartFunc func(ctx context.Context, ifName string, cfg ldpConfig)

// discoveryManager tracks the running per-interface discovery goroutines and
// reconciles them against the configured interface set.
type discoveryManager struct {
	ctx     context.Context
	log     *slog.Logger
	startFn discoveryStartFunc

	mu      sync.Mutex
	running map[string]context.CancelFunc // interface name ("" = system default) -> cancel
}

func newDiscoveryManager(ctx context.Context, log *slog.Logger, startFn discoveryStartFunc) *discoveryManager {
	return &discoveryManager{
		ctx:     ctx,
		log:     log,
		startFn: startFn,
		running: make(map[string]context.CancelFunc),
	}
}

// reconcile starts discovery on newly-configured interfaces and stops it on
// removed ones (AC-9). With no interfaces configured it runs a single
// system-assigned multicast listener, keyed by the empty interface name.
func (m *discoveryManager) reconcile(cfg ldpConfig) {
	desired := make(map[string]struct{})
	if len(cfg.Interfaces) == 0 {
		desired[""] = struct{}{}
	} else {
		for _, name := range cfg.Interfaces {
			desired[name] = struct{}{}
		}
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	for name, cancel := range m.running {
		if _, ok := desired[name]; !ok {
			cancel()
			delete(m.running, name)
			m.log.Info("ldp: discovery stopped on interface", "interface", name)
		}
	}
	for name := range desired {
		if _, ok := m.running[name]; ok {
			continue
		}
		ifctx, cancel := context.WithCancel(m.ctx)
		m.running[name] = cancel
		go m.startFn(ifctx, name, cfg)
		if name != "" {
			m.log.Info("ldp: discovery started on interface", "interface", name)
		}
	}
}

// stopAll cancels every running discovery goroutine.
func (m *discoveryManager) stopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for name, cancel := range m.running {
		cancel()
		delete(m.running, name)
	}
}

// runningCount returns the number of interfaces discovery is currently running on.
func (m *discoveryManager) runningCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.running)
}
