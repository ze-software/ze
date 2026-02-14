package plugin

import (
	"errors"

	bgptypes "codeberg.org/thomas-mangin/ze/internal/plugins/bgp/types"
)

// ErrMissingWatchdog is returned when watchdog name is not provided.
var ErrMissingWatchdog = errors.New("missing watchdog name")

// routeRPCs returns RPC registrations for watchdog handlers.
// Part of the ze-bgp module — aggregated by BgpPluginRPCs().
func routeRPCs() []RPCRegistration {
	return []RPCRegistration{
		{"ze-bgp:watchdog-announce", "bgp watchdog announce", handleWatchdogAnnounce, "Announce routes in watchdog group"},
		{"ze-bgp:watchdog-withdraw", "bgp watchdog withdraw", handleWatchdogWithdraw, "Withdraw routes in watchdog group"},
	}
}

// handleWatchdogAnnounce handles: watchdog announce <name>
// Announces all routes in the named watchdog group that are currently withdrawn.
func handleWatchdogAnnounce(ctx *CommandContext, args []string) (*Response, error) {
	return handleWatchdogAction(ctx, args, func(r bgptypes.BGPReactor, peer, name string) error {
		return r.AnnounceWatchdog(peer, name)
	})
}

// handleWatchdogWithdraw handles: watchdog withdraw <name>
// Withdraws all routes in the named watchdog group that are currently announced.
func handleWatchdogWithdraw(ctx *CommandContext, args []string) (*Response, error) {
	return handleWatchdogAction(ctx, args, func(r bgptypes.BGPReactor, peer, name string) error {
		return r.WithdrawWatchdog(peer, name)
	})
}

// handleWatchdogAction implements the shared logic for watchdog announce/withdraw.
func handleWatchdogAction(
	ctx *CommandContext,
	args []string,
	action func(bgptypes.BGPReactor, string, string) error,
) (*Response, error) {
	r, errResp, err := RequireBGPReactor(ctx)
	if err != nil {
		return errResp, err
	}

	if len(args) < 1 {
		return &Response{
			Status: "error",
			Data:   "missing watchdog name",
		}, ErrMissingWatchdog
	}

	name := args[0]
	peerSelector := ctx.PeerSelector()

	if err := action(r, peerSelector, name); err != nil {
		return &Response{
			Status: "error",
			Data:   err.Error(),
		}, err
	}

	return &Response{
		Status: "done",
		Data: map[string]any{
			"peer":     peerSelector,
			"watchdog": name,
		},
	}, nil
}
