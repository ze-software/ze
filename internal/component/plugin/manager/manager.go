// Design: plan/spec-arch-0-system-boundaries.md — PluginManager implementation
// Design: docs/architecture/plugin-manager-wiring.md — two-phase startup
// Related: ../server/startup.go — Server calls SpawnMore/GetProcessManager during handshake

package plugin

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"

	parent "codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginipc "codeberg.org/thomas-mangin/ze/internal/component/plugin/ipc"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/process"
	"codeberg.org/thomas-mangin/ze/internal/core/metrics"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

// logger is the plugin manager subsystem logger (lazy initialization).
var logger = slogutil.LazyLogger("plugin.manager")

// Manager implements ze.PluginManager.
// It owns plugin process lifecycle: registration, spawning, and shutdown.
// The 5-stage protocol handshake stays in pluginserver.Server (Phase 2).
//
// Two-phase startup:
//   - Phase 1 (StartAll): spawn processes — no Server needed
//   - Phase 2 (Server.StartWithContext): handshake with spawned processes
//
// MUST call SetHubConfig before StartAll if external plugins exist.
type Manager struct {
	mu      sync.RWMutex
	plugins map[string]*pluginState
	caps    []ze.Capability
	started bool

	// Hub config for TLS acceptor (external plugins).
	hubConfig *parent.HubConfig

	// Single shared ProcessManager — every spawn phase appends to the same
	// pm.processes map so AllProcesses returns every running plugin regardless
	// of which phase started it. Created on the first spawnProcesses call.
	procManager *process.ProcessManager

	// TLS acceptor for external plugin connect-back (shared across phases).
	acceptor *pluginipc.PluginAcceptor

	// Context for process management.
	ctx    context.Context
	cancel context.CancelFunc

	// Stored references.
	eventBus        ze.EventBus
	config          ze.ConfigProvider
	metricsRegistry any // metrics.Registry, stored as any to avoid import
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

// SetHubConfig sets the TLS transport config for external plugins.
// Must be called before StartAll if external plugins are configured.
func (m *Manager) SetHubConfig(cfg *parent.HubConfig) {
	m.hubConfig = cfg
}

// SetMetricsRegistry stores the metrics registry for plugin health metrics.
// Must be called before SpawnMore. The registry is passed to each ProcessManager.
func (m *Manager) SetMetricsRegistry(reg any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.metricsRegistry = reg
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

// StartAll initializes the PluginManager for process management (Phase 1).
// Stores context and references. Does NOT spawn processes — that happens
// when Server calls SpawnMore during Phase 2 (after Server is created).
// Plugins are discovered from reactor config by Server, not registered here.
func (m *Manager) StartAll(ctx context.Context, eventBus ze.EventBus, config ze.ConfigProvider) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.started {
		return fmt.Errorf("already started")
	}

	m.ctx, m.cancel = context.WithCancel(ctx)
	m.eventBus = eventBus
	m.config = config
	m.started = true

	// Mark registered plugins as running (for query API).
	for _, ps := range m.plugins {
		ps.running = true
	}

	return nil
}

// SpawnMore spawns additional processes for auto-loaded plugins.
// Called by Server during Phase 2 when auto-load discovers new plugins.
func (m *Manager) SpawnMore(configs []parent.PluginConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.started {
		return fmt.Errorf("not started")
	}
	if len(configs) == 0 {
		return nil
	}

	return m.spawnProcesses(configs)
}

// spawnProcesses appends configs to the shared ProcessManager and starts them.
// On the first call it creates the pm; subsequent calls reuse it via StartMore
// so AllProcesses returns every plugin from every phase. Must be called with
// m.mu held.
func (m *Manager) spawnProcesses(configs []parent.PluginConfig) error {
	first := m.procManager == nil
	if first {
		m.procManager = process.NewProcessManager(configs)

		// Thread metrics registry to ProcessManager for plugin health metrics.
		if m.metricsRegistry != nil {
			if reg, ok := m.metricsRegistry.(metrics.Registry); ok {
				m.procManager.SetMetricsRegistry(reg)
			} else {
				logger().Warn("metrics registry type mismatch, plugin health metrics disabled",
					"type", fmt.Sprintf("%T", m.metricsRegistry))
			}
		}
	}

	// Set up TLS acceptor for external plugins (shared across all spawn phases).
	if err := m.ensureAcceptor(configs); err != nil {
		return err
	}
	if m.acceptor != nil {
		m.procManager.SetAcceptor(m.acceptor)
	}

	if first {
		if err := m.procManager.StartWithContext(m.ctx); err != nil {
			return fmt.Errorf("start processes: %w", err)
		}
	} else {
		if err := m.procManager.StartMore(configs); err != nil {
			return fmt.Errorf("start more processes: %w", err)
		}
	}

	// Mark spawned plugins as running. Auto-loaded plugins may not have been
	// pre-registered via Register() — add them to the map so Plugin()/Plugins()
	// queries return them.
	for _, cfg := range configs {
		if ps, ok := m.plugins[cfg.Name]; ok {
			ps.running = true
		} else {
			m.plugins[cfg.Name] = &pluginState{
				config:  ze.PluginConfig{Name: cfg.Name, Internal: cfg.Internal},
				running: true,
			}
		}
	}

	logger().Debug("processes spawned", "count", len(configs))
	return nil
}

// ensureAcceptor creates a TLS acceptor if external plugins exist and no acceptor yet.
// Must be called with m.mu held.
func (m *Manager) ensureAcceptor(configs []parent.PluginConfig) error {
	if m.acceptor != nil {
		return nil // Already created
	}

	hasExternal := false
	for _, cfg := range configs {
		if !cfg.Internal {
			hasExternal = true
			break
		}
	}
	if !hasExternal {
		return nil
	}

	hubConf := m.hubConfig
	if hubConf == nil || len(hubConf.Servers) == 0 {
		// Auto-generate hub config for external plugins without explicit config.
		var tokenBytes [32]byte
		if _, err := rand.Read(tokenBytes[:]); err != nil {
			return fmt.Errorf("generate hub token: %w", err)
		}
		hubConf = &parent.HubConfig{
			Servers: []parent.HubServerConfig{{
				Name:   "auto",
				Host:   "127.0.0.1",
				Port:   0,
				Secret: hex.EncodeToString(tokenBytes[:]),
			}},
		}
	}

	// Use the first server block for the plugin acceptor.
	server := hubConf.Servers[0]

	cert, err := pluginipc.GenerateSelfSignedCert()
	if err != nil {
		return fmt.Errorf("generate TLS cert: %w", err)
	}

	listeners, err := pluginipc.StartListeners([]string{server.Address()}, cert)
	if err != nil {
		return fmt.Errorf("start TLS listeners: %w", err)
	}

	m.acceptor = pluginipc.NewPluginAcceptor(listeners[0], server.Secret, pluginipc.CertFingerprint(cert))

	// Wire per-client secrets if the server block has any.
	if len(server.Clients) > 0 {
		clients := server.Clients // capture for closure
		m.acceptor.SetSecretLookup(func(name string) (string, bool) {
			s, ok := clients[name]
			return s, ok
		})
	}

	m.acceptor.Start()

	// Close extra listeners (acceptor owns the first one).
	for _, ln := range listeners[1:] {
		ln.Close() //nolint:errcheck,gosec // extra listeners not used yet
	}

	return nil
}

// GetProcessManager returns the shared ProcessManager that holds every plugin
// process from every spawn phase.
// Returns nil if no processes have been spawned.
// Returns any to satisfy plugin.ProcessSpawner interface (Server type-asserts).
func (m *Manager) GetProcessManager() any {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.procManager == nil {
		return nil
	}
	return m.procManager
}

// StopAll stops all spawned processes and cleans up.
func (m *Manager) StopAll(_ context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.procManager != nil {
		m.procManager.Stop()
		m.procManager = nil
	}

	if m.acceptor != nil {
		m.acceptor.Stop()
		m.acceptor = nil
	}

	if m.cancel != nil {
		m.cancel()
	}

	for _, ps := range m.plugins {
		ps.running = false
	}

	m.started = false
	return nil
}

// Plugin looks up a plugin by name.
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
func (m *Manager) AddCapability(cap ze.Capability) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.caps = append(m.caps, cap)
}
