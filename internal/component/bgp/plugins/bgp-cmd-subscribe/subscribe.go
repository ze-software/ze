// Design: docs/architecture/api/commands.md — event subscription handlers
// Overview: doc.go — bgp-cmd-subscribe plugin registration

package bgpcmdsubscribe

import (
	"fmt"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: "ze-bgp:subscribe", CLICommand: "subscribe", Handler: handleSubscribe, Help: "Subscribe to events"},
		pluginserver.RPCRegistration{WireMethod: "ze-bgp:unsubscribe", CLICommand: "unsubscribe", Handler: handleUnsubscribe, Help: "Unsubscribe from events"},
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
		}, fmt.Errorf("no process context")
	}

	if ctx.Subscriptions() == nil {
		return &plugin.Response{
			Status: plugin.StatusError,
			Data:   "subscription manager not available",
		}, fmt.Errorf("no subscription manager")
	}

	ctx.Subscriptions().Add(ctx.Process, sub)

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: map[string]any{
			"namespace": sub.Namespace,
			"event":     sub.EventType,
			"direction": sub.Direction,
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
		}, fmt.Errorf("no process context")
	}

	if ctx.Subscriptions() == nil {
		return &plugin.Response{
			Status: plugin.StatusError,
			Data:   "subscription manager not available",
		}, fmt.Errorf("no subscription manager")
	}

	removed := ctx.Subscriptions().Remove(ctx.Process, sub)

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: map[string]any{
			"removed":   removed,
			"namespace": sub.Namespace,
			"event":     sub.EventType,
		},
	}, nil
}
