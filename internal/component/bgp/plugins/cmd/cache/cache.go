// Design: docs/architecture/api/commands.md — BGP cache operation handlers

package cache

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
	"codeberg.org/thomas-mangin/ze/internal/core/selector"
)

const (
	actionRetain  = "retain"
	actionRelease = "release"
	actionExpire  = "expire"
	actionForward = "forward"
	actionList    = "list"
)

var (
	errMissingAction   = errors.New("missing action")
	errMissingID       = errors.New("missing id")
	errMissingSelector = errors.New("missing selector")
)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: "ze-bgp:cache", Handler: handleBgpCache},
	)
}

// cacheActionKeywords is the set of valid cache action keywords.
var cacheActionKeywords = map[string]bool{
	actionList: true, actionRetain: true, actionRelease: true, actionExpire: true, actionForward: true,
}

// handleBgpCache handles all bgp cache subcommands.
//
// Canonical grammar (action before identifier):
//
//	cache list
//	cache retain <id>
//	cache release <id>
//	cache expire <id>
//	cache forward <id> <selector>
//	cache forward <id1>,<id2>,... <selector>  (batch)
//
// Deprecated grammar (accepted with deprecation warning):
//
//	cache <id> retain|release|expire
//	cache <id> forward <selector>
func handleBgpCache(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	if len(args) == 0 {
		return bgpCacheHelp()
	}

	_, errResp, err := requireBGPReactor(ctx)
	if err != nil {
		return errResp, err
	}

	switch args[0] {
	case actionList:
		return handleBgpCacheList(ctx)
	case actionRetain, actionRelease, actionExpire:
		if len(args) < 2 {
			return &plugin.Response{
				Status: plugin.StatusError,
				Error:  "usage: cache " + args[0] + " <id>",
			}, errMissingID
		}
		return dispatchCacheByID(ctx, args[0], args[1], args[2:], false)
	case actionForward:
		if len(args) < 3 {
			return &plugin.Response{
				Status: plugin.StatusError,
				Error:  "usage: cache forward <id> <selector>",
			}, errMissingSelector
		}
		return dispatchCacheByID(ctx, actionForward, args[1], args[2:], false)
	}

	// Deprecated: cache <id> <action> [args...]
	if len(args) < 2 {
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  "usage: cache retain|release|expire|forward <id>",
		}, errMissingAction
	}
	if !cacheActionKeywords[args[1]] {
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  "unknown cache action: " + args[1],
		}, fmt.Errorf("unknown action: %s", args[1])
	}
	return dispatchCacheByID(ctx, args[1], args[0], args[2:], true)
}

// dispatchCacheByID routes a cache action to the right handler after parsing the ID.
func dispatchCacheByID(ctx *pluginserver.CommandContext, action, idStr string, extraArgs []string, deprecated bool) (*plugin.Response, error) {
	addDeprecation := func(resp *plugin.Response) *plugin.Response {
		if !deprecated || resp == nil || resp.Status != plugin.StatusDone {
			return resp
		}
		if data, ok := resp.Data.(plugin.Map); ok {
			data["deprecated"] = "use: cache " + action + " " + idStr
		}
		return resp
	}

	if strings.Contains(idStr, ",") {
		resp, err := handleBgpCacheBatch(ctx, idStr, action, extraArgs)
		return addDeprecation(resp), err
	}

	cacheID, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  "invalid cache id: " + idStr,
		}, fmt.Errorf("invalid cache id: %w", err)
	}

	var resp *plugin.Response
	switch action {
	case actionRetain:
		resp, err = handleBgpCacheRetain(ctx, cacheID)
	case actionRelease:
		resp, err = handleBgpCacheRelease(ctx, cacheID)
	case actionExpire:
		resp, err = handleBgpCacheExpire(ctx, cacheID)
	case actionForward:
		resp, err = handleBgpCacheForward(ctx, cacheID, extraArgs)
	default:
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  "unknown cache action: " + action,
		}, fmt.Errorf("unknown action: %s", action)
	}
	return addDeprecation(resp), err
}

// bgpCacheHelp returns help for bgp cache command.
func bgpCacheHelp() (*plugin.Response, error) {
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: plugin.Map{
			"commands": []map[string]string{
				{"command": "cache list", "description": "List cached message IDs"},
				{"command": "cache retain <id>", "description": "Prevent eviction of cached message"},
				{"command": "cache release <id>", "description": "Ack without forwarding (plugin) or undo retain (API)"},
				{"command": "cache expire <id>", "description": "Remove from cache immediately"},
				{"command": "cache forward <id> <sel>", "description": "Forward cached UPDATE to peers"},
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
		Data: plugin.Map{
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
			Error:  fmt.Sprintf("retain failed: %v", err),
		}, err
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: plugin.Map{
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
			Error:  fmt.Sprintf("release failed: %v", err),
		}, err
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: plugin.Map{
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
			Error:  fmt.Sprintf("expire failed: %v", err),
		}, err
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: plugin.Map{
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
			Error:  "usage: cache forward <id> <selector>",
		}, errMissingSelector
	}

	sel, err := selector.Parse(args[0])
	if err != nil {
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  fmt.Sprintf("invalid selector: %v", err),
		}, err
	}

	r, errResp, bgpErr := requireBGPReactor(ctx)
	if bgpErr != nil {
		return errResp, bgpErr
	}
	if err := r.ForwardUpdate(sel, id, cacheConsumerNameFromCtx(ctx)); err != nil {
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  fmt.Sprintf("forward failed: %v", err),
		}, err
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: plugin.Map{
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
	var errs []string
	processed := 0

	for part := range strings.SplitSeq(idList, ",") {
		id, err := strconv.ParseUint(part, 10, 64)
		if err != nil {
			errs = append(errs, fmt.Sprintf("invalid id %q: %v", part, err))
			continue
		}

		var actionErr error
		switch action {
		case actionRetain:
			_, actionErr = handleBgpCacheRetain(ctx, id)
		case actionRelease:
			_, actionErr = handleBgpCacheRelease(ctx, id)
		case actionExpire:
			_, actionErr = handleBgpCacheExpire(ctx, id)
		case actionForward:
			_, actionErr = handleBgpCacheForward(ctx, id, actionArgs)
		default:
			return &plugin.Response{
				Status: plugin.StatusError,
				Error:  "unknown cache action: " + action,
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
			Error:  "batch " + action + ": " + strconv.Itoa(processed) + " processed, " + strconv.Itoa(len(errs)) + " errors",
		}, fmt.Errorf("batch %s: %d errors", action, len(errs))
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: plugin.Map{
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
