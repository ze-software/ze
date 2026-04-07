// Design: plan/spec-arch-0-system-boundaries.md — Engine interface

package ze

import "context"

// Engine is the top-level supervisor. It starts the event bus, config
// manager, plugin manager, and subsystems in the correct order. It monitors
// health, handles signals (SIGHUP for reload, SIGTERM for shutdown), and
// owns the top-level context.
//
// The engine has no knowledge of BGP or any specific protocol. It starts
// whatever subsystems are registered.
type Engine interface {
	// RegisterSubsystem adds a subsystem to be started by Start.
	RegisterSubsystem(sub Subsystem) error

	// Start launches all components in order:
	// config load → event bus create → plugins start → subsystems start.
	Start(ctx context.Context) error

	// Stop gracefully shuts down all components in reverse order.
	Stop(ctx context.Context) error

	// Reload re-reads config and notifies subsystems.
	Reload(ctx context.Context) error

	// EventBus returns the namespaced event bus.
	EventBus() EventBus

	// Config returns the config provider.
	Config() ConfigProvider

	// Plugins returns the plugin manager.
	Plugins() PluginManager
}
