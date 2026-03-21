// Design: plan/spec-arch-0-system-boundaries.md — Subsystem interface

package ze

import "context"

// Subsystem is a first-class daemon that owns external I/O.
//
// A subsystem listens on TCP sockets, runs protocol state machines,
// parses/encodes wire bytes, and publishes/subscribes events on the bus.
// It is supervised by the Engine and receives Bus + ConfigProvider at startup.
//
// The BGP daemon is a subsystem. Plugins (bgp-rib, bgp-rs, bgp-gr) are not —
// they extend subsystem behavior by reacting to bus events.
type Subsystem interface {
	// Name returns the subsystem identifier (e.g., "bgp").
	Name() string

	// Start launches the subsystem with access to the bus and config.
	// The subsystem creates its own topics, starts listeners, and
	// begins publishing events.
	Start(ctx context.Context, bus Bus, config ConfigProvider) error

	// Stop gracefully shuts down the subsystem.
	Stop(ctx context.Context) error

	// Reload applies configuration changes from the config provider.
	Reload(ctx context.Context, config ConfigProvider) error
}
