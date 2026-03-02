// Design: docs/architecture/hub-architecture.md — hub coordination
//
// Package hub provides the hub/orchestrator process for ze.
//
// The hub forks and coordinates plugins (ze bgp, ze rib, ze gr) using
// the 5-stage protocol. It routes commands and events between plugins
// and provides config management.
//
// This package composes existing infrastructure from internal/plugin:
//   - plugin.SubsystemManager - manages forked processes
//   - plugin.SchemaRegistry - routes by handler path
//   - plugin.Hub - command routing
package hub

import (
	"context"
	"sync"

	pluginserver "codeberg.org/thomas-mangin/ze/internal/plugin/server"
)

// PluginDef defines a plugin to be forked by the hub.
type PluginDef struct {
	Name string // Plugin name (e.g., "bgp", "rib", "gr")
	Run  string // Command to execute (e.g., "ze bgp --child")
}

// HubConfig holds hub orchestrator configuration.
type HubConfig struct {
	Plugins    []PluginDef       // Plugins to fork
	Env        map[string]string // Environment settings
	APISocket  string            // Unix socket path for CLI
	Blocks     map[string]any    // Remaining config blocks (bgp, rib, etc.)
	ConfigPath string            // Original config file path (for child processes)
}

// Orchestrator manages plugin lifecycle and communication.
// It composes existing plugin infrastructure rather than duplicating it.
type Orchestrator struct {
	config     *HubConfig
	subsystems *pluginserver.SubsystemManager
	registry   *pluginserver.SchemaRegistry
	pluginHub  *pluginserver.Hub

	ctx    context.Context
	cancel context.CancelFunc
	mu     sync.Mutex
}

// NewOrchestrator creates a new hub orchestrator with the given configuration.
func NewOrchestrator(cfg *HubConfig) *Orchestrator {
	if cfg == nil {
		cfg = &HubConfig{}
	}

	registry := pluginserver.NewSchemaRegistry()
	subsystems := pluginserver.NewSubsystemManager()

	// Register subsystems from config
	for _, p := range cfg.Plugins {
		subsystems.Register(pluginserver.SubsystemConfig{
			Name:       p.Name,
			Binary:     p.Run,
			ConfigPath: cfg.ConfigPath,
		})
	}

	return &Orchestrator{
		config:     cfg,
		subsystems: subsystems,
		registry:   registry,
		pluginHub:  pluginserver.NewHub(registry, subsystems),
	}
}

// Start starts all plugins and the hub event loop.
func (o *Orchestrator) Start(ctx context.Context) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	o.ctx, o.cancel = context.WithCancel(ctx)

	// Start all subsystems (fork processes, complete 5-stage protocol)
	if err := o.subsystems.StartAll(o.ctx); err != nil {
		return err
	}

	// Register schemas from all subsystems
	if err := o.subsystems.RegisterSchemas(o.registry); err != nil {
		o.subsystems.StopAll()
		return err
	}

	return nil
}

// Stop gracefully shuts down all plugins.
func (o *Orchestrator) Stop() {
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.cancel != nil {
		o.cancel()
	}

	o.subsystems.StopAll()
}

// Registry returns the schema registry for handler lookups.
func (o *Orchestrator) Registry() *pluginserver.SchemaRegistry {
	return o.registry
}

// Subsystems returns the subsystem manager.
func (o *Orchestrator) Subsystems() *pluginserver.SubsystemManager {
	return o.subsystems
}
