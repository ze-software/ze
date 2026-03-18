// Design: docs/architecture/api/process-protocol.md — plugin process management
// Related: rpc.go — typed RPC over socket pairs
// Related: tls.go — TLS transport for external plugins

package ipc

import (
	"net"
)

// SocketPair represents one connected pair of sockets for IPC.
// One side belongs to the engine, the other to the plugin.
type SocketPair struct {
	EngineSide net.Conn // Engine keeps this end
	PluginSide net.Conn // Plugin gets this end (or passed via FD)
}

// DualSocketPair contains both socket pairs for plugin communication.
// Engine socket: plugin calls engine (plugin is client, engine is server).
// Callback socket: engine calls plugin (engine is client, plugin is server).
type DualSocketPair struct {
	Engine   SocketPair // Socket A: Plugin calls Engine
	Callback SocketPair // Socket B: Engine calls Plugin
}

// Close closes all four connections in the dual socket pair.
func (d *DualSocketPair) Close() {
	if d.Engine.EngineSide != nil {
		d.Engine.EngineSide.Close() //nolint:errcheck,gosec // best-effort cleanup
	}
	if d.Engine.PluginSide != nil {
		d.Engine.PluginSide.Close() //nolint:errcheck,gosec // best-effort cleanup
	}
	if d.Callback.EngineSide != nil {
		d.Callback.EngineSide.Close() //nolint:errcheck,gosec // best-effort cleanup
	}
	if d.Callback.PluginSide != nil {
		d.Callback.PluginSide.Close() //nolint:errcheck,gosec // best-effort cleanup
	}
}

// NewInternalSocketPairs creates two net.Pipe pairs for internal (goroutine) plugins.
// No real FDs are involved - these are in-memory connections.
func NewInternalSocketPairs() (*DualSocketPair, error) {
	// Socket A: Plugin calls Engine
	engineA, pluginA := net.Pipe()

	// Socket B: Engine calls Plugin
	engineB, pluginB := net.Pipe()

	return &DualSocketPair{
		Engine: SocketPair{
			EngineSide: engineA,
			PluginSide: pluginA,
		},
		Callback: SocketPair{
			EngineSide: engineB,
			PluginSide: pluginB,
		},
	}, nil
}
