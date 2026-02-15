package handler

import (
	"errors"

	"codeberg.org/thomas-mangin/ze/internal/plugin"
	bgptypes "codeberg.org/thomas-mangin/ze/internal/plugins/bgp/types"
)

// ErrMissingWatchdog is returned when watchdog name is not provided.
var ErrMissingWatchdog = errors.New("missing watchdog name")

// WatchdogRPCs returns RPC registrations for watchdog handlers.
// Part of the ze-bgp module — aggregated by BgpHandlerRPCs().
func WatchdogRPCs() []plugin.RPCRegistration {
	return []plugin.RPCRegistration{
		{WireMethod: "ze-bgp:watchdog-announce", CLICommand: "bgp watchdog announce", Handler: handleWatchdogAnnounce, Help: "Announce routes in watchdog group"},
		{WireMethod: "ze-bgp:watchdog-withdraw", CLICommand: "bgp watchdog withdraw", Handler: handleWatchdogWithdraw, Help: "Withdraw routes in watchdog group"},
	}
}

// handleWatchdogAnnounce handles: watchdog announce <name>
// Announces all routes in the named watchdog group that are currently withdrawn.
func handleWatchdogAnnounce(ctx *plugin.CommandContext, args []string) (*plugin.Response, error) {
	return handleWatchdogAction(ctx, args, func(r bgptypes.BGPReactor, peer, name string) error {
		return r.AnnounceWatchdog(peer, name)
	})
}

// handleWatchdogWithdraw handles: watchdog withdraw <name>
// Withdraws all routes in the named watchdog group that are currently announced.
func handleWatchdogWithdraw(ctx *plugin.CommandContext, args []string) (*plugin.Response, error) {
	return handleWatchdogAction(ctx, args, func(r bgptypes.BGPReactor, peer, name string) error {
		return r.WithdrawWatchdog(peer, name)
	})
}

// handleWatchdogAction implements the shared logic for watchdog announce/withdraw.
func handleWatchdogAction(
	ctx *plugin.CommandContext,
	args []string,
	action func(bgptypes.BGPReactor, string, string) error,
) (*plugin.Response, error) {
	r, errResp, err := requireBGPReactor(ctx)
	if err != nil {
		return errResp, err
	}

	if len(args) < 1 {
		return &plugin.Response{
			Status: plugin.StatusError,
			Data:   "missing watchdog name",
		}, ErrMissingWatchdog
	}

	name := args[0]
	peerSelector := ctx.PeerSelector()

	if err := action(r, peerSelector, name); err != nil {
		return &plugin.Response{
			Status: plugin.StatusError,
			Data:   err.Error(),
		}, err
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: map[string]any{
			"peer":     peerSelector,
			"watchdog": name,
		},
	}, nil
}
