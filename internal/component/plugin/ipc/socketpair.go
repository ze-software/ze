// Design: docs/architecture/api/process-protocol.md — plugin process management
// Related: rpc.go — typed RPC over socket pairs
// Related: fdpass.go — SCM_RIGHTS fd passing over Unix sockets

package ipc

import (
	"fmt"
	"net"
	"os"
	"syscall"
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

// NewExternalSocketPairs creates two syscall.Socketpair pairs for external (subprocess) plugins.
// The plugin-side FDs are intended to be passed via cmd.ExtraFiles.
func NewExternalSocketPairs() (*DualSocketPair, error) {
	// Socket A: Plugin calls Engine
	engineA, pluginA, err := newUnixSocketPair()
	if err != nil {
		return nil, fmt.Errorf("creating engine socket pair: %w", err)
	}

	// Socket B: Engine calls Plugin
	engineB, pluginB, err := newUnixSocketPair()
	if err != nil {
		engineA.Close() //nolint:errcheck,gosec // cleanup on error
		pluginA.Close() //nolint:errcheck,gosec // cleanup on error
		return nil, fmt.Errorf("creating callback socket pair: %w", err)
	}

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

// PluginFiles returns os.File handles for the plugin-side sockets.
// These are used with cmd.ExtraFiles to pass FDs to subprocess plugins.
// The returned files must be closed by the caller after exec.Cmd.Start().
// Returns (engineFile, callbackFile, error).
func (d *DualSocketPair) PluginFiles() (*os.File, *os.File, error) {
	engineFile, err := connToFile(d.Engine.PluginSide)
	if err != nil {
		return nil, nil, fmt.Errorf("engine plugin side to file: %w", err)
	}

	callbackFile, err := connToFile(d.Callback.PluginSide)
	if err != nil {
		engineFile.Close() //nolint:errcheck,gosec // cleanup on error
		return nil, nil, fmt.Errorf("callback plugin side to file: %w", err)
	}

	return engineFile, callbackFile, nil
}

// newUnixSocketPair creates a connected pair of Unix stream sockets.
// Returns (conn0, conn1, error) where each conn is a net.Conn.
func newUnixSocketPair() (net.Conn, net.Conn, error) {
	fds, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	if err != nil {
		return nil, nil, fmt.Errorf("socketpair: %w", err)
	}

	// Convert raw FDs to net.Conn via os.File
	f0 := os.NewFile(uintptr(fds[0]), "socketpair-0")
	f1 := os.NewFile(uintptr(fds[1]), "socketpair-1")

	conn0, err := net.FileConn(f0)
	f0.Close() //nolint:errcheck,gosec // FD ownership transferred to conn0
	if err != nil {
		f1.Close() //nolint:errcheck,gosec // cleanup on error
		return nil, nil, fmt.Errorf("file to conn (fd0): %w", err)
	}

	conn1, err := net.FileConn(f1)
	f1.Close() //nolint:errcheck,gosec // FD ownership transferred to conn1
	if err != nil {
		conn0.Close() //nolint:errcheck,gosec // cleanup on error
		return nil, nil, fmt.Errorf("file to conn (fd1): %w", err)
	}

	return conn0, conn1, nil
}

// connToFile extracts an os.File from a net.Conn.
// Works for connections backed by real FDs (net.UnixConn from socketpair).
// Returns error for net.Pipe connections (no underlying FD).
func connToFile(conn net.Conn) (*os.File, error) {
	type filer interface {
		File() (*os.File, error)
	}

	fc, ok := conn.(filer)
	if !ok {
		return nil, fmt.Errorf("connection type %T does not support File()", conn)
	}

	return fc.File()
}
