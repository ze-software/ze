// Design: docs/plan/spec-arch-0-system-boundaries.md — PluginManager implementation
// Design: docs/plan/spec-arch-3-plugin-manager.md — PluginManager spec

package plugin

import (
	"context"
	"fmt"
	"sync"

	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

// Manager implements ze.PluginManager.
// It tracks plugin registration, lifecycle state, and capabilities.
// Phase 5 will wire this to actual process management and 5-stage protocol.
type Manager struct {
	mu      sync.RWMutex
	plugins map[string]*pluginState
	caps    []ze.Capability
	started bool

	// Stored for Phase 5 integration — Bus and ConfigProvider
	// received during StartAll for use during 5-stage protocol.
	bus    ze.Bus
	config ze.ConfigProvider
}

// pluginState tracks a single plugin's registration and lifecycle.
type pluginState struct {
	config  ze.PluginConfig
	running bool
}

// NewManager creates a new Manager.
func NewManager() *Manager {
	return &Manager{
		plugins: make(map[string]*pluginState),
	}
}

// Register adds a plugin for startup. Returns error if the name
// is already registered or if StartAll has already been called.
func (m *Manager) Register(config ze.PluginConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.started {
		return fmt.Errorf("cannot register plugin %q: already started", config.Name)
	}

	if _, exists := m.plugins[config.Name]; exists {
		return fmt.Errorf("plugin %q already registered", config.Name)
	}

	m.plugins[config.Name] = &pluginState{config: config}
	return nil
}

// StartAll marks all registered plugins as running.
// Stores bus and config references for Phase 5 (5-stage protocol integration).
func (m *Manager) StartAll(ctx context.Context, bus ze.Bus, config ze.ConfigProvider) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.started {
		return fmt.Errorf("already started")
	}

	m.bus = bus
	m.config = config
	m.started = true

	for _, ps := range m.plugins {
		ps.running = true
	}

	return nil
}

// StopAll marks all plugins as stopped.
func (m *Manager) StopAll(_ context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, ps := range m.plugins {
		ps.running = false
	}

	m.started = false
	return nil
}

// Plugin looks up a plugin by name. Returns the plugin state and
// true if found, or a zero PluginProcess and false if not found.
func (m *Manager) Plugin(name string) (ze.PluginProcess, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ps, ok := m.plugins[name]
	if !ok {
		return ze.PluginProcess{}, false
	}

	return ze.PluginProcess{
		Name:    ps.config.Name,
		Running: ps.running,
	}, true
}

// Plugins returns all registered plugins.
func (m *Manager) Plugins() []ze.PluginProcess {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]ze.PluginProcess, 0, len(m.plugins))
	for _, ps := range m.plugins {
		result = append(result, ze.PluginProcess{
			Name:    ps.config.Name,
			Running: ps.running,
		})
	}
	return result
}

// Capabilities returns all capabilities collected from plugins.
func (m *Manager) Capabilities() []ze.Capability {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]ze.Capability, len(m.caps))
	copy(result, m.caps)
	return result
}

// AddCapability adds a capability to the manager.
// Used during Stage 3 of the 5-stage protocol and in tests.
func (m *Manager) AddCapability(cap ze.Capability) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.caps = append(m.caps, cap)
}
