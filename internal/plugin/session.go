package plugin

import (
	"errors"
	"fmt"
	"os"
)

// ErrSilent is returned when a command should produce no response.
var ErrSilent = errors.New("silent")

// RegisterSessionHandlers registers session API commands.
// These commands control per-process API connection state.
// Note: ACK is controlled by serial prefix (#N), not session commands.
// Note: Session lifecycle commands (ready/ping/bye) moved to plugin namespace.
// Note: Session sync/encoding commands move to bgp plugin namespace in Step 4.
func RegisterSessionHandlers(d *Dispatcher) {
	// Sync control (Step 4 moves to bgp plugin ack sync/async)
	d.Register("session sync enable", handleSessionSyncEnable, "Enable sync mode (wait for wire)")
	d.Register("session sync disable", handleSessionSyncDisable, "Disable sync mode")

	// Wire encoding control (Step 4 moves to bgp plugin encoding)
	d.Register("session api encoding", handleSessionAPIEncoding, "Set wire encoding (hex|b64|cbor|text)")
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

// handleSessionAPIEncoding sets wire encoding for this process session.
// Syntax:
//   - session api encoding <format>          - sets both inbound and outbound
//   - session api encoding inbound <format>  - sets inbound only
//   - session api encoding outbound <format> - sets outbound only
//
// Formats: hex (default), b64, cbor, text.
func handleSessionAPIEncoding(ctx *CommandContext, args []string) (*Response, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("missing encoding: session api encoding <hex|b64|cbor|text>")
	}

	// Check for direction specifier
	direction := "both"
	encodingArg := args[0]

	if args[0] == "inbound" || args[0] == "outbound" {
		if len(args) < 2 {
			return nil, fmt.Errorf("missing encoding after %s", args[0])
		}
		direction = args[0]
		encodingArg = args[1]
	}

	// Parse encoding
	enc, err := ParseWireEncoding(encodingArg)
	if err != nil {
		return nil, err
	}

	// Apply to process
	if ctx.Process != nil {
		switch direction {
		case "inbound":
			ctx.Process.SetWireEncodingIn(enc)
		case "outbound":
			ctx.Process.SetWireEncodingOut(enc)
		default: // both
			ctx.Process.SetWireEncoding(enc)
		}
	}

	// Build response data
	data := map[string]any{
		"encoding": enc.String(),
	}
	if direction != "both" {
		data["direction"] = direction
	}

	return &Response{
		Status: "done",
		Data:   data,
	}, nil
}
