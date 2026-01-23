package plugin

import (
	"errors"
	"os"
)

// ErrSilent is returned when a command should produce no response.
var ErrSilent = errors.New("silent")

// handlePluginSessionPing returns a pong response for health checking.
// Returns daemon PID for identification.
func handlePluginSessionPing(_ *CommandContext, _ []string) (*Response, error) {
	return &Response{
		Status: "done",
		Data: map[string]any{
			"pong": os.Getpid(),
		},
	}, nil
}

// handlePluginSessionBye handles client disconnect cleanup.
// Called when a client is disconnecting from the API.
func handlePluginSessionBye(_ *CommandContext, _ []string) (*Response, error) {
	// Currently just acknowledges the disconnect.
	// Future: could clean up client-specific state.
	return &Response{
		Status: "done",
		Data: map[string]any{
			"status": "goodbye",
		},
	}, nil
}

// handlePluginSessionReady signals that an API process has completed initialization.
// If called with a peer prefix ("peer <addr> plugin session ready"), signals that
// peer-specific API initialization is complete (e.g., route replay after reconnect).
// If called without peer prefix, unblocks reactor startup.
func handlePluginSessionReady(ctx *CommandContext, _ []string) (*Response, error) {
	if ctx.Reactor != nil {
		// Check if this is a peer-specific ready signal
		if ctx.Peer != "" && ctx.Peer != "*" {
			ctx.Reactor.SignalPeerAPIReady(ctx.Peer)
		} else {
			// Global ready signal for startup
			ctx.Reactor.SignalAPIReady()
		}
	}
	return &Response{
		Status: "done",
		Data: map[string]any{
			"api": "ready acknowledged",
		},
	}, nil
}
