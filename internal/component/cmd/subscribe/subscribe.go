// Design: docs/architecture/api/commands.md — event subscription handlers
// Overview: doc.go — bgp-cmd-subscribe plugin registration

package subscribe

import (
	"errors"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
)

// Sentinel errors for subscribe/unsubscribe handlers.
var (
	// ErrNoProcessContext is returned when the command context has no process.
	ErrNoProcessContext = errors.New("no process context")

	// ErrNoSubscriptionManager is returned when no subscription manager is available.
	ErrNoSubscriptionManager = errors.New("no subscription manager")
)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: "ze-bgp:subscribe", Handler: handleSubscribe},
		pluginserver.RPCRegistration{WireMethod: "ze-bgp:unsubscribe", Handler: handleUnsubscribe},
	)
}

// handleSubscribe handles the "subscribe" command.
func handleSubscribe(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	sub, err := pluginserver.ParseSubscription(args)
	if err != nil {
		return &plugin.Response{
			Status: plugin.StatusError,
			Data:   err.Error(),
		}, err
	}

	if ctx.Process == nil {
		return &plugin.Response{
			Status: plugin.StatusError,
			Data:   "subscribe requires a process context",
		}, ErrNoProcessContext
	}

	if ctx.Subscriptions() == nil {
		return &plugin.Response{
			Status: plugin.StatusError,
			Data:   "subscription manager not available",
		}, ErrNoSubscriptionManager
	}

	ctx.Subscriptions().Add(ctx.Process, sub)

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: map[string]any{
			"namespace": sub.Namespace.String(),
			"event":     sub.EventType.String(),
			"direction": sub.Direction.String(),
		},
	}, nil
}

// handleUnsubscribe handles the "unsubscribe" command.
func handleUnsubscribe(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	sub, err := pluginserver.ParseSubscription(args)
	if err != nil {
		return &plugin.Response{
			Status: plugin.StatusError,
			Data:   err.Error(),
		}, err
	}

	if ctx.Process == nil {
		return &plugin.Response{
			Status: plugin.StatusError,
			Data:   "unsubscribe requires a process context",
		}, ErrNoProcessContext
	}

	if ctx.Subscriptions() == nil {
		return &plugin.Response{
			Status: plugin.StatusError,
			Data:   "subscription manager not available",
		}, ErrNoSubscriptionManager
	}

	removed := ctx.Subscriptions().Remove(ctx.Process, sub)

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: map[string]any{
			"removed":   removed,
			"namespace": sub.Namespace.String(),
			"event":     sub.EventType.String(),
		},
	}, nil
}
