package api

import (
	"fmt"
	"strconv"
)

// RegisterForwardHandlers registers forward-related command handlers.
func RegisterForwardHandlers(d *Dispatcher) {
	d.Register("forward update-id", handleForwardUpdateID,
		"Forward a cached UPDATE by ID: peer <selector> forward update-id <id>")
	d.Register("delete update-id", handleDeleteUpdateID,
		"Delete a cached UPDATE without forwarding: delete update-id <id>")
}

// handleForwardUpdateID forwards a cached UPDATE to peers.
// Usage: peer <selector> forward update-id <id>
//
// The peer selector is extracted by the dispatcher into ctx.Peer.
// Supports: specific IP, *, !IP (all except).
func handleForwardUpdateID(ctx *CommandContext, args []string) (*Response, error) {
	if len(args) < 1 {
		return &Response{
			Status: "error",
			Error:  "usage: peer <selector> forward update-id <id>",
		}, fmt.Errorf("missing update-id")
	}

	// Parse update ID
	updateID, err := strconv.ParseUint(args[0], 10, 64)
	if err != nil {
		return &Response{
			Status: "error",
			Error:  fmt.Sprintf("invalid update-id: %s", args[0]),
		}, fmt.Errorf("invalid update-id: %w", err)
	}

	// Parse peer selector from context
	selectorStr := ctx.PeerSelector()
	selector, err := ParseSelector(selectorStr)
	if err != nil {
		return &Response{
			Status: "error",
			Error:  fmt.Sprintf("invalid peer selector: %v", err),
		}, err
	}

	// Forward the update
	if err := ctx.Reactor.ForwardUpdate(selector, updateID); err != nil {
		return &Response{
			Status: "error",
			Error:  fmt.Sprintf("forward failed: %v", err),
		}, err
	}

	return &Response{
		Status: "done",
		Data: map[string]any{
			"update_id": updateID,
			"selector":  selector.String(),
		},
	}, nil
}

// handleDeleteUpdateID deletes a cached UPDATE without forwarding.
// Usage: delete update-id <id>
//
// Used when controller decides not to forward (filtering/policy).
func handleDeleteUpdateID(ctx *CommandContext, args []string) (*Response, error) {
	if len(args) < 1 {
		return &Response{
			Status: "error",
			Error:  "usage: delete update-id <id>",
		}, fmt.Errorf("missing update-id")
	}

	// Parse update ID
	updateID, err := strconv.ParseUint(args[0], 10, 64)
	if err != nil {
		return &Response{
			Status: "error",
			Error:  fmt.Sprintf("invalid update-id: %s", args[0]),
		}, fmt.Errorf("invalid update-id: %w", err)
	}

	// Delete the update
	if err := ctx.Reactor.DeleteUpdate(updateID); err != nil {
		return &Response{
			Status: "error",
			Error:  fmt.Sprintf("delete failed: %v", err),
		}, err
	}

	return &Response{
		Status: "done",
		Data: map[string]any{
			"update_id": updateID,
		},
	}, nil
}
