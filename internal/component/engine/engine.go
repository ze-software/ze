// Design: plan/spec-arch-0-system-boundaries.md — Engine supervisor
// Design: plan/spec-arch-5-engine.md — Engine spec

package engine

import (
	"context"
	"fmt"
	"sync"

	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

// Engine implements ze.Engine.
// It composes Bus, ConfigProvider, and PluginManager, and manages
// subsystem registration and lifecycle in correct startup/shutdown order.
// The engine has no knowledge of BGP or any specific protocol.
type Engine struct {
	mu         sync.RWMutex
	bus        ze.Bus
	config     ze.ConfigProvider
	plugins    ze.PluginManager
	subsystems []ze.Subsystem
	names      map[string]struct{}
	started    bool
	stopped    bool
}

// NewEngine creates a new Engine with the given components.
func NewEngine(bus ze.Bus, config ze.ConfigProvider, plugins ze.PluginManager) *Engine {
	return &Engine{
		bus:     bus,
		config:  config,
		plugins: plugins,
		names:   make(map[string]struct{}),
	}
}

// RegisterSubsystem adds a subsystem to be started by Start.
// Returns error if the name is already registered or if Start has been called.
func (e *Engine) RegisterSubsystem(sub ze.Subsystem) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.started {
		return fmt.Errorf("cannot register subsystem %q: already started", sub.Name())
	}

	name := sub.Name()
	if _, exists := e.names[name]; exists {
		return fmt.Errorf("subsystem %q already registered", name)
	}

	e.names[name] = struct{}{}
	e.subsystems = append(e.subsystems, sub)
	return nil
}

// Start launches all components in order:
// plugins start → subsystems start (in registration order).
// Bus and config are provided at construction time.
func (e *Engine) Start(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.started {
		return fmt.Errorf("already started")
	}

	// Start plugins via PluginManager.
	if err := e.plugins.StartAll(ctx, e.bus, e.config); err != nil {
		return fmt.Errorf("start plugins: %w", err)
	}

	// Start subsystems in registration order.
	for i, sub := range e.subsystems {
		if err := sub.Start(ctx, e.bus, e.config); err != nil {
			// Stop already-started subsystems in reverse order.
			for j := i - 1; j >= 0; j-- {
				_ = e.subsystems[j].Stop(ctx)
			}
			_ = e.plugins.StopAll(ctx)
			return fmt.Errorf("start subsystem %q: %w", sub.Name(), err)
		}
	}

	e.started = true
	e.stopped = false
	return nil
}

// Stop gracefully shuts down all components in reverse order:
// subsystems stop (reverse registration order) → plugins stop.
// Idempotent — second call returns nil.
func (e *Engine) Stop(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.stopped || !e.started {
		return nil
	}

	// Stop subsystems in reverse registration order.
	var firstErr error
	for i := len(e.subsystems) - 1; i >= 0; i-- {
		if err := e.subsystems[i].Stop(ctx); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("stop subsystem %q: %w", e.subsystems[i].Name(), err)
		}
	}

	// Stop plugins.
	if err := e.plugins.StopAll(ctx); err != nil && firstErr == nil {
		firstErr = fmt.Errorf("stop plugins: %w", err)
	}

	e.stopped = true
	return firstErr
}

// Reload re-reads config and notifies all subsystems.
func (e *Engine) Reload(ctx context.Context) error {
	e.mu.RLock()
	defer e.mu.RUnlock()

	for _, sub := range e.subsystems {
		if err := sub.Reload(ctx, e.config); err != nil {
			return fmt.Errorf("reload subsystem %q: %w", sub.Name(), err)
		}
	}
	return nil
}

// Bus returns the message bus.
func (e *Engine) Bus() ze.Bus {
	return e.bus
}

// Config returns the config provider.
func (e *Engine) Config() ze.ConfigProvider {
	return e.config
}

// Plugins returns the plugin manager.
func (e *Engine) Plugins() ze.PluginManager {
	return e.plugins
}
