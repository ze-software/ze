// Design: docs/architecture/api/commands.md — BGP cache operation handlers

package cache

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/selector"
	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	actionRetain  = "retain"
	actionRelease = "release"
	actionExpire  = "expire"
	actionForward = "forward"
)

var (
	errMissingID       = errors.New("missing id")
	errMissingSelector = errors.New("missing selector")
)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: "ze-bgp:cache-list", Handler: handleCacheListRPC},
		pluginserver.RPCRegistration{WireMethod: "ze-bgp:cache-retain", Handler: handleCacheRetainRPC},
		pluginserver.RPCRegistration{WireMethod: "ze-bgp:cache-release", Handler: handleCacheReleaseRPC},
		pluginserver.RPCRegistration{WireMethod: "ze-bgp:cache-expire", Handler: handleCacheExpireRPC},
		pluginserver.RPCRegistration{WireMethod: "ze-bgp:cache-forward", Handler: handleCacheForwardRPC},
	)
}

func handleCacheListRPC(ctx *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	return handleBgpCacheList(ctx)
}

func handleCacheRetainRPC(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	if len(args) < 1 {
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  "usage: request cache retain <id>",
		}, errMissingID
	}
	return dispatchCacheByID(ctx, actionRetain, args[0], args[1:])
}

func handleCacheReleaseRPC(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	if len(args) < 1 {
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  "usage: request cache release <id>",
		}, errMissingID
	}
	return dispatchCacheByID(ctx, actionRelease, args[0], args[1:])
}

func handleCacheExpireRPC(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	if len(args) < 1 {
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  "usage: request cache expire <id>",
		}, errMissingID
	}
	return dispatchCacheByID(ctx, actionExpire, args[0], args[1:])
}

func handleCacheForwardRPC(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	if len(args) < 2 {
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  "usage: request cache forward <id> <selector>",
		}, errMissingSelector
	}
	return dispatchCacheByID(ctx, actionForward, args[0], args[1:])
}

// dispatchCacheByID routes a cache action to the right handler after parsing the ID.
func dispatchCacheByID(ctx *pluginserver.CommandContext, action, idStr string, extraArgs []string) (*plugin.Response, error) {
	if strings.Contains(idStr, ",") {
		return handleBgpCacheBatch(ctx, idStr, action, extraArgs)
	}

	var tb textbuf.Buffer
	cacheID, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  tb.Str("invalid cache id: ").Str(idStr).String(),
		}, fmt.Errorf("invalid cache id: %w", err)
	}

	switch action {
	case actionRetain:
		return handleBgpCacheRetain(ctx, cacheID)
	case actionRelease:
		return handleBgpCacheRelease(ctx, cacheID)
	case actionExpire:
		return handleBgpCacheExpire(ctx, cacheID)
	case actionForward:
		return handleBgpCacheForward(ctx, cacheID, extraArgs)
	default:
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  tb.Reset().Str("unknown cache action: ").Str(action).String(),
		}, fmt.Errorf("unknown action: %s", action)
	}
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
	// ctx.Sender is the authority and cacheConsumerNameFromCtx is the accounting
	// name; they answer different questions and the second is empty for a process
	// that did not declare cache-consumer. Forwarding puts a whole UPDATE on each
	// matched peer's wire, so the peers that do not attach this process with
	// `send [ update ]` are dropped by the reactor and reported.
	if err := r.ForwardUpdate(sel, id, cacheConsumerNameFromCtx(ctx), ctx.Sender); err != nil {
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
