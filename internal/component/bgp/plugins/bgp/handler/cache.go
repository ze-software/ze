// Design: docs/architecture/api/commands.md — API command handlers

package handler

import (
	"fmt"
	"strconv"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
	"codeberg.org/thomas-mangin/ze/internal/core/selector"
)

// CacheRPCs returns RPC registrations for cache handlers.
func CacheRPCs() []pluginserver.RPCRegistration {
	return []pluginserver.RPCRegistration{
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
//   - bgp cache <id1>,<id2>,...,<idN> forward <selector>  (batch)
//   - bgp cache <id1>,<id2>,...,<idN> release              (batch)
func handleBgpCache(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
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

	action := args[1]
	actionArgs := args[2:]

	// Batch: comma-separated IDs (e.g., "10,20,30").
	if strings.Contains(args[0], ",") {
		return handleBgpCacheBatch(ctx, args[0], action, actionArgs)
	}

	// Single ID.
	cacheID, err := strconv.ParseUint(args[0], 10, 64)
	if err != nil {
		return &plugin.Response{
			Status: plugin.StatusError,
			Data:   fmt.Sprintf("invalid cache id: %s", args[0]),
		}, fmt.Errorf("invalid cache id: %w", err)
	}

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
				{"command": "bgp cache <id> release", "description": "Ack without forwarding (plugin) or undo retain (API)"},
				{"command": "bgp cache <id> expire", "description": "Remove from cache immediately"},
				{"command": "bgp cache <id> forward <sel>", "description": "Forward cached UPDATE to peers"},
			},
		},
	}, nil
}

// handleBgpCacheList returns all cached message IDs.
func handleBgpCacheList(ctx *pluginserver.CommandContext) (*plugin.Response, error) {
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
func handleBgpCacheRetain(ctx *pluginserver.CommandContext, id uint64) (*plugin.Response, error) {
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

// handleBgpCacheRelease acks without forwarding (cache consumer) or undoes retain (non-consumer).
// Cache consumer: removes calling plugin from consumer set (FIFO validated).
// Non-consumer (including non-cache-consumer plugins): decrements API-level retain count.
func handleBgpCacheRelease(ctx *pluginserver.CommandContext, id uint64) (*plugin.Response, error) {
	r, errResp, err := requireBGPReactor(ctx)
	if err != nil {
		return errResp, err
	}
	if err := r.ReleaseUpdate(id, cacheConsumerNameFromCtx(ctx)); err != nil {
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
func handleBgpCacheExpire(ctx *pluginserver.CommandContext, id uint64) (*plugin.Response, error) {
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

// handleBgpCacheForward forwards a cached UPDATE to peers and records plugin ack.
func handleBgpCacheForward(ctx *pluginserver.CommandContext, id uint64, args []string) (*plugin.Response, error) {
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
	if err := r.ForwardUpdate(sel, id, cacheConsumerNameFromCtx(ctx)); err != nil {
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

// handleBgpCacheBatch processes a comma-separated list of cache IDs.
// Parses each ID and dispatches to the per-ID handler for the given action.
// All valid IDs are processed even if some are invalid — errors are collected
// and returned as a combined error if any ID failed.
func handleBgpCacheBatch(ctx *pluginserver.CommandContext, idList, action string, actionArgs []string) (*plugin.Response, error) {
	parts := strings.Split(idList, ",")
	var errs []string
	processed := 0

	for _, part := range parts {
		id, err := strconv.ParseUint(part, 10, 64)
		if err != nil {
			errs = append(errs, fmt.Sprintf("invalid id %q: %v", part, err))
			continue
		}

		var actionErr error
		switch action {
		case "retain":
			_, actionErr = handleBgpCacheRetain(ctx, id)
		case "release":
			_, actionErr = handleBgpCacheRelease(ctx, id)
		case "expire":
			_, actionErr = handleBgpCacheExpire(ctx, id)
		case "forward":
			_, actionErr = handleBgpCacheForward(ctx, id, actionArgs)
		default: // unknown action — reject entire batch
			return &plugin.Response{
				Status: plugin.StatusError,
				Data:   fmt.Sprintf("unknown cache action: %s", action),
			}, fmt.Errorf("unknown action: %s", action)
		}
		if actionErr != nil {
			errs = append(errs, fmt.Sprintf("id %d: %v", id, actionErr))
			continue
		}
		processed++
	}

	if len(errs) > 0 {
		return &plugin.Response{
			Status: plugin.StatusError,
			Data: map[string]any{
				"processed": processed,
				"errors":    errs,
			},
		}, fmt.Errorf("batch %s: %d errors", action, len(errs))
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: map[string]any{
			"processed": processed,
		},
	}, nil
}

// cacheConsumerNameFromCtx returns the plugin name if the caller is a cache consumer.
// Returns empty string for non-plugin callers and for plugins that did not
// declare cache-consumer: true during registration. Non-cache-consumer plugins
// are treated the same as external callers for cache operations.
func cacheConsumerNameFromCtx(ctx *pluginserver.CommandContext) string {
	if ctx.Process != nil && ctx.Process.IsCacheConsumer() {
		return ctx.Process.Name()
	}
	return ""
}
