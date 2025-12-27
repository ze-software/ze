package api

import (
	"errors"
	"os"
)

// ErrSilent is returned when a command should produce no response.
// Used by "session ack silence" to suppress output.
var ErrSilent = errors.New("silent")

// RegisterSessionHandlers registers session API commands.
// These commands control per-process API connection state.
func RegisterSessionHandlers(d *Dispatcher) {
	// ACK control
	d.Register("session ack enable", handleSessionAckEnable, "Enable ACK responses")
	d.Register("session ack disable", handleSessionAckDisable, "Disable ACK responses")
	d.Register("session ack silence", handleSessionAckSilence, "Silence ACK immediately (no response)")

	// Sync control
	d.Register("session sync enable", handleSessionSyncEnable, "Enable sync mode (wait for wire)")
	d.Register("session sync disable", handleSessionSyncDisable, "Disable sync mode")

	// Session control
	d.Register("session reset", handleSessionReset, "Reset session state")
	d.Register("session ping", handleSessionPing, "Health check")
	d.Register("session bye", handleSessionBye, "Client disconnect")
}

// handleSessionAckEnable enables ACK responses for this process.
// After this, "done" responses are sent after each command.
func handleSessionAckEnable(ctx *CommandContext, _ []string) (*Response, error) {
	if ctx.Process != nil {
		ctx.Process.SetAck(true)
	}
	return &Response{
		Status: "done",
		Data: map[string]any{
			"ack": "enabled",
		},
	}, nil
}

// handleSessionAckDisable disables ACK responses for this process.
// A response IS sent for this command, but subsequent commands don't get responses.
func handleSessionAckDisable(ctx *CommandContext, _ []string) (*Response, error) {
	if ctx.Process != nil {
		ctx.Process.SetAck(false)
	}
	return &Response{
		Status: "done",
		Data: map[string]any{
			"ack": "disabled",
		},
	}, nil
}

// handleSessionAckSilence disables ACK responses immediately.
// Unlike disable, no response is sent for this command either.
// Returns ErrSilent to indicate the caller should not send any response.
func handleSessionAckSilence(ctx *CommandContext, _ []string) (*Response, error) {
	if ctx.Process != nil {
		ctx.Process.SetAck(false)
	}
	// Return ErrSilent to suppress response
	return nil, ErrSilent
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
		ctx.Process.SetAck(true)
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
