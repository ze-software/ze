// Design: docs/plan/spec-arch-0-system-boundaries.md — PluginManager interface

package ze

import "context"

// PluginManager handles plugin lifecycle: registration, 5-stage startup
// protocol, process management, DirectBridge setup, and shutdown.
//
// After Stage 5, plugins receive a Bus reference for runtime communication.
// The PluginManager does not handle event delivery — that is the Bus's job.
type PluginManager interface {
	// Register adds a plugin for startup.
	Register(config PluginConfig) error

	// StartAll runs the 5-stage protocol for all registered plugins.
	StartAll(ctx context.Context, bus Bus, config ConfigProvider) error

	// StopAll gracefully shuts down all running plugins.
	StopAll(ctx context.Context) error

	// Plugin looks up a running plugin by name.
	Plugin(name string) (PluginProcess, bool)

	// Plugins lists all running plugins.
	Plugins() []PluginProcess

	// Capabilities returns capabilities collected during Stage 3.
	Capabilities() []Capability
}

// PluginConfig describes a plugin to be started by the PluginManager.
type PluginConfig struct {
	// Name is the plugin identifier (e.g., "bgp-rib").
	Name string

	// Internal indicates an in-process plugin (goroutine + DirectBridge)
	// vs an external plugin (forked subprocess + sockets).
	Internal bool

	// Dependencies lists plugin names that must also be loaded.
	Dependencies []string
}

// PluginProcess represents a running plugin.
type PluginProcess struct {
	// Name is the plugin identifier.
	Name string

	// Running indicates whether the plugin is currently active.
	Running bool
}

// Capability is a capability declared by a plugin during Stage 3.
type Capability struct {
	// Plugin is the name of the plugin that declared this capability.
	Plugin string

	// Code is the BGP capability code (e.g., 65 for 4-byte ASN).
	Code uint8

	// Value is the raw capability value bytes.
	Value []byte
}
