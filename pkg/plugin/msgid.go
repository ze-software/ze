package plugin

import (
	"fmt"
	"strconv"
)

// RegisterMsgIDHandlers registers msg-id cache control command handlers.
func RegisterMsgIDHandlers(d *Dispatcher) {
	d.Register("msg-id retain", handleMsgIDRetain,
		"Retain a cached msg-id (prevent eviction): msg-id <id> retain")
	d.Register("msg-id release", handleMsgIDRelease,
		"Release a retained msg-id (allow eviction): msg-id <id> release")
	d.Register("msg-id expire", handleMsgIDExpire,
		"Expire a cached msg-id immediately: msg-id <id> expire")
	d.Register("msg-id list", handleMsgIDList,
		"List all cached msg-ids: msg-id list")
}

// handleMsgIDRetain retains a cached UPDATE to prevent eviction.
// Usage: msg-id <id> retain
//
// Used by API for graceful restart - retain routes for replay.
func handleMsgIDRetain(ctx *CommandContext, args []string) (*Response, error) {
	if len(args) < 1 {
		return &Response{
			Status: statusError,
			Data:   "usage: msg-id retain <id>",
		}, fmt.Errorf("missing msg-id")
	}

	// Parse msg-id
	msgID, err := strconv.ParseUint(args[0], 10, 64)
	if err != nil {
		return &Response{
			Status: statusError,
			Data:   fmt.Sprintf("invalid msg-id: %s", args[0]),
		}, fmt.Errorf("invalid msg-id: %w", err)
	}

	// Retain the update
	if err := ctx.Reactor.RetainUpdate(msgID); err != nil {
		return &Response{
			Status: statusError,
			Data:   fmt.Sprintf("retain failed: %v", err),
		}, err
	}

	return &Response{
		Status: statusDone,
		Data: map[string]any{
			"msg_id":   msgID,
			"retained": true,
		},
	}, nil
}

// handleMsgIDRelease releases a retained UPDATE to allow eviction.
// Usage: msg-id <id> release
//
// Resets TTL to default expiration time.
func handleMsgIDRelease(ctx *CommandContext, args []string) (*Response, error) {
	if len(args) < 1 {
		return &Response{
			Status: statusError,
			Data:   "usage: msg-id release <id>",
		}, fmt.Errorf("missing msg-id")
	}

	// Parse msg-id
	msgID, err := strconv.ParseUint(args[0], 10, 64)
	if err != nil {
		return &Response{
			Status: statusError,
			Data:   fmt.Sprintf("invalid msg-id: %s", args[0]),
		}, fmt.Errorf("invalid msg-id: %w", err)
	}

	// Release the update
	if err := ctx.Reactor.ReleaseUpdate(msgID); err != nil {
		return &Response{
			Status: statusError,
			Data:   fmt.Sprintf("release failed: %v", err),
		}, err
	}

	return &Response{
		Status: statusDone,
		Data: map[string]any{
			"msg_id":   msgID,
			"released": true,
		},
	}, nil
}

// handleMsgIDExpire removes a cached UPDATE immediately.
// Usage: msg-id <id> expire
//
// Same as DeleteUpdate but with explicit "expire" command name.
func handleMsgIDExpire(ctx *CommandContext, args []string) (*Response, error) {
	if len(args) < 1 {
		return &Response{
			Status: statusError,
			Data:   "usage: msg-id expire <id>",
		}, fmt.Errorf("missing msg-id")
	}

	// Parse msg-id
	msgID, err := strconv.ParseUint(args[0], 10, 64)
	if err != nil {
		return &Response{
			Status: statusError,
			Data:   fmt.Sprintf("invalid msg-id: %s", args[0]),
		}, fmt.Errorf("invalid msg-id: %w", err)
	}

	// Delete the update
	if err := ctx.Reactor.DeleteUpdate(msgID); err != nil {
		return &Response{
			Status: statusError,
			Data:   fmt.Sprintf("expire failed: %v", err),
		}, err
	}

	return &Response{
		Status: statusDone,
		Data: map[string]any{
			"msg_id":  msgID,
			"expired": true,
		},
	}, nil
}

// handleMsgIDList returns all cached msg-ids.
// Usage: msg-id list.
func handleMsgIDList(ctx *CommandContext, _ []string) (*Response, error) {
	ids := ctx.Reactor.ListUpdates()

	return &Response{
		Status: statusDone,
		Data: map[string]any{
			"msg_ids": ids,
			"count":   len(ids),
		},
	}, nil
}
