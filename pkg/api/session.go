package api

import (
	"errors"
	"os"
)

// ErrSilent is returned when a command should produce no response.
var ErrSilent = errors.New("silent")

// RegisterSessionHandlers registers session API commands.
// These commands control per-process API connection state.
// Note: ACK is controlled by serial prefix (#N), not session commands.
func RegisterSessionHandlers(d *Dispatcher) {
	// Sync control
	d.Register("session sync enable", handleSessionSyncEnable, "Enable sync mode (wait for wire)")
	d.Register("session sync disable", handleSessionSyncDisable, "Disable sync mode")

	// API startup synchronization
	d.Register("session api ready", handleSessionAPIReady, "Signal API initialization complete")

	// Session control
	d.Register("session reset", handleSessionReset, "Reset session state")
	d.Register("session ping", handleSessionPing, "Health check")
	d.Register("session bye", handleSessionBye, "Client disconnect")
}

// handleSessionSyncEnable enables sync mode for this process.
// When enabled, announce/withdraw waits for wire transmission before ACK.
func handleSessionSyncEnable(ctx *CommandContext, _ []string) (*Response, error) {
	if ctx.Process != nil {
		ctx.Process.SetSync(true)
	}
	return &Response{
		Status: "done",
		Data: map[string]any{
			"sync": "enabled",
		},
	}, nil
}

// handleSessionSyncDisable disables sync mode for this process.
// ACK is sent immediately after RIB update (default behavior).
func handleSessionSyncDisable(ctx *CommandContext, _ []string) (*Response, error) {
	if ctx.Process != nil {
		ctx.Process.SetSync(false)
	}
	return &Response{
		Status: "done",
		Data: map[string]any{
			"sync": "disabled",
		},
	}, nil
}

// handleSessionReset resets session state for this process.
// Clears any pending async operations.
func handleSessionReset(ctx *CommandContext, _ []string) (*Response, error) {
	// Reset to defaults
	if ctx.Process != nil {
		ctx.Process.SetSync(false)
	}
	return &Response{
		Status: "done",
		Data: map[string]any{
			"status": "session reset",
		},
	}, nil
}

// handleSessionPing returns a pong response for health checking.
// Returns daemon PID for identification.
func handleSessionPing(ctx *CommandContext, _ []string) (*Response, error) {
	return &Response{
		Status: "done",
		Data: map[string]any{
			"pong": os.Getpid(),
		},
	}, nil
}

// handleSessionBye handles client disconnect cleanup.
// Called when a client is disconnecting from the API.
func handleSessionBye(ctx *CommandContext, _ []string) (*Response, error) {
	// Currently just acknowledges the disconnect.
	// Future: could clean up client-specific state.
	return &Response{
		Status: "done",
		Data: map[string]any{
			"status": "goodbye",
		},
	}, nil
}

// handleSessionAPIReady signals that an API process has completed initialization.
// Unblocks the reactor startup, allowing BGP peer connections to begin.
func handleSessionAPIReady(ctx *CommandContext, _ []string) (*Response, error) {
	if ctx.Reactor != nil {
		ctx.Reactor.SignalAPIReady()
	}
	return &Response{
		Status: "done",
		Data: map[string]any{
			"api": "ready acknowledged",
		},
	}, nil
}
