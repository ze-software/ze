package plugin

import (
	"fmt"
	"strconv"

	"codeberg.org/thomas-mangin/ze/internal/selector"
)

// cacheRPCs returns RPC registrations for handlers defined in this file.
func cacheRPCs() []RPCRegistration {
	return []RPCRegistration{
		{"ze-bgp:cache", "bgp cache", handleBgpCache, "BGP message cache operations"},
	}
}

// handleBgpCache handles all bgp cache subcommands.
// Usage:
//   - bgp cache list
//   - bgp cache <id> retain
//   - bgp cache <id> release
//   - bgp cache <id> expire
//   - bgp cache <id> forward <selector>
func handleBgpCache(ctx *CommandContext, args []string) (*Response, error) {
	if len(args) == 0 {
		return bgpCacheHelp()
	}

	// Check for "list" command (no ID needed)
	if args[0] == "list" {
		return handleBgpCacheList(ctx)
	}

	// All other commands need <id> <action> [args...]
	if len(args) < 2 {
		return &Response{
			Status: statusError,
			Data:   "usage: bgp cache <id> retain|release|expire|forward <sel>",
		}, fmt.Errorf("missing action")
	}

	// Parse cache ID
	cacheID, err := strconv.ParseUint(args[0], 10, 64)
	if err != nil {
		return &Response{
			Status: statusError,
			Data:   fmt.Sprintf("invalid cache id: %s", args[0]),
		}, fmt.Errorf("invalid cache id: %w", err)
	}

	action := args[1]
	actionArgs := args[2:]

	switch action {
	case "retain":
		return handleBgpCacheRetain(ctx, cacheID)
	case "release":
		return handleBgpCacheRelease(ctx, cacheID)
	case "expire":
		return handleBgpCacheExpire(ctx, cacheID)
	case "forward":
		return handleBgpCacheForward(ctx, cacheID, actionArgs)
	default:
		return &Response{
			Status: statusError,
			Data:   fmt.Sprintf("unknown cache action: %s", action),
		}, fmt.Errorf("unknown action: %s", action)
	}
}

// bgpCacheHelp returns help for bgp cache command.
func bgpCacheHelp() (*Response, error) {
	return &Response{
		Status: statusDone,
		Data: map[string]any{
			"commands": []map[string]string{
				{"command": "bgp cache list", "description": "List cached message IDs"},
				{"command": "bgp cache <id> retain", "description": "Prevent eviction of cached message"},
				{"command": "bgp cache <id> release", "description": "Allow eviction (reset TTL)"},
				{"command": "bgp cache <id> expire", "description": "Remove from cache immediately"},
				{"command": "bgp cache <id> forward <sel>", "description": "Forward cached UPDATE to peers"},
			},
		},
	}, nil
}

// handleBgpCacheList returns all cached message IDs.
func handleBgpCacheList(ctx *CommandContext) (*Response, error) {
	ids := ctx.Reactor.ListUpdates()

	return &Response{
		Status: statusDone,
		Data: map[string]any{
			"ids":   ids,
			"count": len(ids),
		},
	}, nil
}

// handleBgpCacheRetain prevents eviction of a cached message.
func handleBgpCacheRetain(ctx *CommandContext, id uint64) (*Response, error) {
	if err := ctx.Reactor.RetainUpdate(id); err != nil {
		return &Response{
			Status: statusError,
			Data:   fmt.Sprintf("retain failed: %v", err),
		}, err
	}

	return &Response{
		Status: statusDone,
		Data: map[string]any{
			"id":       id,
			"retained": true,
		},
	}, nil
}

// handleBgpCacheRelease allows eviction of a cached message.
func handleBgpCacheRelease(ctx *CommandContext, id uint64) (*Response, error) {
	if err := ctx.Reactor.ReleaseUpdate(id); err != nil {
		return &Response{
			Status: statusError,
			Data:   fmt.Sprintf("release failed: %v", err),
		}, err
	}

	return &Response{
		Status: statusDone,
		Data: map[string]any{
			"id":       id,
			"released": true,
		},
	}, nil
}

// handleBgpCacheExpire removes a cached message immediately.
func handleBgpCacheExpire(ctx *CommandContext, id uint64) (*Response, error) {
	if err := ctx.Reactor.DeleteUpdate(id); err != nil {
		return &Response{
			Status: statusError,
			Data:   fmt.Sprintf("expire failed: %v", err),
		}, err
	}

	return &Response{
		Status: statusDone,
		Data: map[string]any{
			"id":      id,
			"expired": true,
		},
	}, nil
}

// handleBgpCacheForward forwards a cached UPDATE to peers.
func handleBgpCacheForward(ctx *CommandContext, id uint64, args []string) (*Response, error) {
	if len(args) < 1 {
		return &Response{
			Status: statusError,
			Data:   "usage: bgp cache <id> forward <selector>",
		}, fmt.Errorf("missing selector")
	}

	sel, err := selector.Parse(args[0])
	if err != nil {
		return &Response{
			Status: statusError,
			Data:   fmt.Sprintf("invalid selector: %v", err),
		}, err
	}

	if err := ctx.Reactor.ForwardUpdate(sel, id); err != nil {
		return &Response{
			Status: statusError,
			Data:   fmt.Sprintf("forward failed: %v", err),
		}, err
	}

	return &Response{
		Status: statusDone,
		Data: map[string]any{
			"id":       id,
			"selector": sel.String(),
		},
	}, nil
}
