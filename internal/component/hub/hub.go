// Design: docs/architecture/hub-architecture.md — hub coordination
// Related: ../plugin/acceptor.go — NewHubAcceptor, the shared TLS acceptor lifecycle ensureAcceptor uses
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
	"errors"
	"fmt"
	"slices"
	"sync"

	"github.com/ze-software/ze/internal/component/config/storage"
	zeplugin "github.com/ze-software/ze/internal/component/plugin"
	pluginipc "github.com/ze-software/ze/internal/component/plugin/ipc"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/textbuf"
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
	Blocks     map[string]any    // Remaining config blocks (bgp, rib, etc.)
	ConfigPath string            // Original config file path (for child processes)
}

// Orchestrator manages plugin lifecycle and communication.
// It composes existing plugin infrastructure rather than duplicating it.
type Orchestrator struct {
	config     *HubConfig
	store      storage.Storage
	subsystems *pluginserver.SubsystemManager
	registry   *pluginserver.SchemaRegistry
	pluginHub  *pluginserver.Hub

	// acceptor is the TLS listener every forked subsystem connects back to.
	// Created lazily by ensureAcceptor (only when a plugin is declared) and
	// owned here: Stop closes it.
	acceptor *pluginipc.PluginAcceptor

	// stopped records that Stop has run. A stopped orchestrator refuses to
	// reload: without this, a post-Stop Reload passes the "was it started"
	// (ctx != nil) check, mints a fresh TLS listener through ensureAcceptor,
	// then fails to fork on the canceled context -- leaving a bound listener
	// nothing will ever close. Cleared by Start so a restart works.
	stopped bool

	ctx    context.Context
	cancel context.CancelFunc
	mu     sync.Mutex
}

// ErrOrchestratorStopped reports an operation attempted after Stop.
var ErrOrchestratorStopped = errors.New("hub orchestrator is stopped")

// NewOrchestrator creates a new hub orchestrator with the given configuration.
// Uses filesystem storage by default; call SetStorage to use blob storage.
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
		store:      storage.NewFilesystem(),
		subsystems: subsystems,
		registry:   registry,
		pluginHub:  pluginserver.NewHub(registry, subsystems),
	}
}

// SetStorage overrides the default filesystem storage.
// Must be called before Start or Reload.
func (o *Orchestrator) SetStorage(store storage.Storage) {
	o.store = store
}

// Start starts all plugins and the hub event loop.
func (o *Orchestrator) Start(ctx context.Context) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	o.stopped = false
	o.ctx, o.cancel = context.WithCancel(ctx)

	// Every subsystem is an external fork, so the TLS connect-back listener
	// must exist before the first one is spawned.
	if err := o.ensureAcceptor(); err != nil {
		return err
	}

	// Start all subsystems (fork processes, complete 5-stage protocol).
	// StartAll already stops any handler it managed to start; the acceptor is
	// this function's own allocation, so a failed Start closes it here rather
	// than leaving a listener bound for a hub that never came up.
	if err := o.subsystems.StartAll(o.ctx); err != nil {
		o.closeAcceptorLocked()
		return err
	}

	// Register schemas from all subsystems
	if err := o.subsystems.RegisterSchemas(o.registry); err != nil {
		o.subsystems.StopAll()
		o.closeAcceptorLocked()
		return err
	}

	// Freeze registries for lock-free dispatch.
	// All registrations are complete; no writers after this point.
	o.registry.Freeze()
	o.subsystems.Freeze()

	return nil
}

// ensureAcceptor creates the shared TLS acceptor once, when at least one
// subsystem is declared. A hub config that forks nothing opens no listener.
// Must be called with o.mu held.
//
// Fail-closed: when the listener cannot be created the error names the plugins
// that will not start because of it, and the caller aborts startup rather than
// spawning children that have nowhere to connect back to.
func (o *Orchestrator) ensureAcceptor() error {
	if o.acceptor != nil {
		return nil
	}
	names := o.subsystems.Names()
	if len(names) == 0 {
		return nil
	}
	// Names() iterates a map, so sort before the name list reaches an error
	// message: an unsorted list reorders run to run and is not greppable.
	slices.Sort(names)

	// nil hub config: ParseHubConfig accepts only `external <name> { ... }`
	// entries inside `plugin {}` (internal/component/hub/config.go
	// parsePluginBlock), so a hub config can never carry an explicit
	// `hub { server ... }` block. NewHubAcceptor auto-generates a loopback
	// listener with a random secret; each child authenticates with the
	// per-plugin token startExternal hands it. Pass the parsed config here if
	// the hub parser ever learns server blocks.
	acceptor, err := zeplugin.NewHubAcceptor(nil)
	if err != nil {
		return fmt.Errorf("hub: plugin TLS acceptor required by external plugin(s) %s: %w",
			textbuf.Join(names, ", "), err)
	}

	o.acceptor = acceptor
	o.subsystems.SetAcceptor(acceptor)
	return nil
}

// closeAcceptorLocked stops and clears the shared acceptor if one exists.
// Idempotent. Must be called with o.mu held. A later Start creates a fresh
// acceptor through ensureAcceptor.
func (o *Orchestrator) closeAcceptorLocked() {
	if o.acceptor == nil {
		return
	}
	o.acceptor.Stop()
	o.acceptor = nil
	o.subsystems.SetAcceptor(nil)
}

// releaseAcceptorIfIdleLocked restores ensureAcceptor's invariant -- a hub that
// forks nothing opens no listener -- after a reload leaves the registry empty,
// either by removing the last plugin or by rolling a failed one back. Without
// it a reload that adds the first plugin and fails to start it leaves a bound
// TLS listener with no subsystem behind it. Must be called with o.mu held.
func (o *Orchestrator) releaseAcceptorIfIdleLocked() {
	if o.acceptor == nil || len(o.subsystems.Names()) > 0 {
		return
	}
	o.closeAcceptorLocked()
}

// Stop gracefully shuts down all plugins.
func (o *Orchestrator) Stop() {
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.cancel != nil {
		o.cancel()
	}

	o.subsystems.StopAll()
	o.closeAcceptorLocked()
	o.stopped = true
}

// Registry returns the schema registry for handler lookups.
func (o *Orchestrator) Registry() *pluginserver.SchemaRegistry {
	return o.registry
}

// Subsystems returns the subsystem manager.
func (o *Orchestrator) Subsystems() *pluginserver.SubsystemManager {
	return o.subsystems
}
