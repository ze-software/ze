// Design: docs/architecture/api/process-protocol.md — plugin process management

package server

import (
	"errors"
	"os"

	plugin "codeberg.org/thomas-mangin/ze/internal/component/plugin"
)

// ErrSilent is returned when a command should produce no response.
var ErrSilent = errors.New("silent")

// sessionRPCs returns RPC registrations for handlers defined in this file.
func sessionRPCs() []RPCRegistration {
	return []RPCRegistration{
		{"ze-plugin:session-ready", "plugin session ready", handlePluginSessionReady, "Signal plugin init complete"},
		{"ze-plugin:peer-session-ready", "bgp peer plugin session ready", handlePluginSessionReady, "Signal peer-specific plugin init complete"},
		{"ze-plugin:session-ping", "plugin session ping", handlePluginSessionPing, "Health check (returns PID)"},
		{"ze-plugin:session-bye", "plugin session bye", handlePluginSessionBye, "Disconnect"},
	}
}

// handlePluginSessionPing returns a pong response for health checking.
// Returns daemon PID for identification.
func handlePluginSessionPing(_ *CommandContext, _ []string) (*plugin.Response, error) {
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: map[string]any{
			"pong": os.Getpid(),
		},
	}, nil
}

// handlePluginSessionBye handles client disconnect cleanup.
// Called when a client is disconnecting from the API.
func handlePluginSessionBye(_ *CommandContext, _ []string) (*plugin.Response, error) {
	// Currently just acknowledges the disconnect.
	// Future: could clean up client-specific state.
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: map[string]any{
			"status": "goodbye",
		},
	}, nil
}

// handlePluginSessionReady signals that an API process has completed initialization.
// If called with a peer prefix ("peer <addr> plugin session ready"), signals that
// peer-specific API initialization is complete (e.g., route replay after reconnect).
// If called without peer prefix, unblocks reactor startup.
func handlePluginSessionReady(ctx *CommandContext, _ []string) (*plugin.Response, error) {
	if ctx.Reactor() != nil {
		// Check if this is a peer-specific ready signal
		if ctx.Peer != "" && ctx.Peer != "*" {
			ctx.Reactor().SignalPeerAPIReady(ctx.Peer)
		} else {
			// Global ready signal for startup
			ctx.Reactor().SignalAPIReady()
		}
	}
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: map[string]any{
			"api": "ready acknowledged",
		},
	}, nil
}
