package handler

import (
	"fmt"
	"strconv"

	"codeberg.org/thomas-mangin/ze/internal/plugin"
	"codeberg.org/thomas-mangin/ze/internal/selector"
)

// CacheRPCs returns RPC registrations for cache handlers.
func CacheRPCs() []plugin.RPCRegistration {
	return []plugin.RPCRegistration{
		{WireMethod: "ze-bgp:cache", CLICommand: "bgp cache", Handler: handleBgpCache, Help: "BGP message cache operations"},
	}
}

// handleBgpCache handles all bgp cache subcommands.
// Usage:
//   - bgp cache list
//   - bgp cache <id> retain
//   - bgp cache <id> release
//   - bgp cache <id> expire
//   - bgp cache <id> forward <selector>
func handleBgpCache(ctx *plugin.CommandContext, args []string) (*plugin.Response, error) {
	if len(args) == 0 {
		return bgpCacheHelp()
	}

	// Guard reactor access (BGP-specific: cache operations)
	_, errResp, err := requireBGPReactor(ctx)
	if err != nil {
		return errResp, err
	}

	// Check for "list" command (no ID needed)
	if args[0] == "list" {
		return handleBgpCacheList(ctx)
	}

	// All other commands need <id> <action> [args...]
	if len(args) < 2 {
		return &plugin.Response{
			Status: plugin.StatusError,
			Data:   "usage: bgp cache <id> retain|release|expire|forward <sel>",
		}, fmt.Errorf("missing action")
	}

	// Parse cache ID
	cacheID, err := strconv.ParseUint(args[0], 10, 64)
	if err != nil {
		return &plugin.Response{
			Status: plugin.StatusError,
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
	default: // unknown cache action — return explicit error
		return &plugin.Response{
			Status: plugin.StatusError,
			Data:   fmt.Sprintf("unknown cache action: %s", action),
		}, fmt.Errorf("unknown action: %s", action)
	}
}

// bgpCacheHelp returns help for bgp cache command.
func bgpCacheHelp() (*plugin.Response, error) {
	return &plugin.Response{
		Status: plugin.StatusDone,
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
func handleBgpCacheList(ctx *plugin.CommandContext) (*plugin.Response, error) {
	r, errResp, err := requireBGPReactor(ctx)
	if err != nil {
		return errResp, err
	}
	ids := r.ListUpdates()

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: map[string]any{
			"ids":   ids,
			"count": len(ids),
		},
	}, nil
}

// handleBgpCacheRetain prevents eviction of a cached message.
func handleBgpCacheRetain(ctx *plugin.CommandContext, id uint64) (*plugin.Response, error) {
	r, errResp, err := requireBGPReactor(ctx)
	if err != nil {
		return errResp, err
	}
	if err := r.RetainUpdate(id); err != nil {
		return &plugin.Response{
			Status: plugin.StatusError,
			Data:   fmt.Sprintf("retain failed: %v", err),
		}, err
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: map[string]any{
			"id":       id,
			"retained": true,
		},
	}, nil
}

// handleBgpCacheRelease allows eviction of a cached message.
func handleBgpCacheRelease(ctx *plugin.CommandContext, id uint64) (*plugin.Response, error) {
	r, errResp, err := requireBGPReactor(ctx)
	if err != nil {
		return errResp, err
	}
	if err := r.ReleaseUpdate(id); err != nil {
		return &plugin.Response{
			Status: plugin.StatusError,
			Data:   fmt.Sprintf("release failed: %v", err),
		}, err
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: map[string]any{
			"id":       id,
			"released": true,
		},
	}, nil
}

// handleBgpCacheExpire removes a cached message immediately.
func handleBgpCacheExpire(ctx *plugin.CommandContext, id uint64) (*plugin.Response, error) {
	r, errResp, err := requireBGPReactor(ctx)
	if err != nil {
		return errResp, err
	}
	if err := r.DeleteUpdate(id); err != nil {
		return &plugin.Response{
			Status: plugin.StatusError,
			Data:   fmt.Sprintf("expire failed: %v", err),
		}, err
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: map[string]any{
			"id":      id,
			"expired": true,
		},
	}, nil
}

// handleBgpCacheForward forwards a cached UPDATE to peers.
func handleBgpCacheForward(ctx *plugin.CommandContext, id uint64, args []string) (*plugin.Response, error) {
	if len(args) < 1 {
		return &plugin.Response{
			Status: plugin.StatusError,
			Data:   "usage: bgp cache <id> forward <selector>",
		}, fmt.Errorf("missing selector")
	}

	sel, err := selector.Parse(args[0])
	if err != nil {
		return &plugin.Response{
			Status: plugin.StatusError,
			Data:   fmt.Sprintf("invalid selector: %v", err),
		}, err
	}

	r, errResp, bgpErr := requireBGPReactor(ctx)
	if bgpErr != nil {
		return errResp, bgpErr
	}
	if err := r.ForwardUpdate(sel, id); err != nil {
		return &plugin.Response{
			Status: plugin.StatusError,
			Data:   fmt.Sprintf("forward failed: %v", err),
		}, err
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: map[string]any{
			"id":       id,
			"selector": sel.String(),
		},
	}, nil
}
