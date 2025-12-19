package api

import (
	"fmt"
	"net/netip"
)

// Version is the ZeBGP version string.
const Version = "0.1.0"

// RegisterDefaultHandlers registers all P0 command handlers.
func RegisterDefaultHandlers(d *Dispatcher) {
	// Daemon control
	d.Register("daemon shutdown", handleDaemonShutdown, "Gracefully shutdown the daemon")
	d.Register("daemon status", handleDaemonStatus, "Show daemon status")

	// Peer operations
	d.Register("peer list", handlePeerList, "List all peers (brief)")
	d.Register("peer show", handlePeerShow, "Show peer details")

	// System commands
	d.Register("system help", handleSystemHelp, "Show available commands")
	d.Register("system version", handleSystemVersion, "Show version")

	// RIB operations (placeholder - needs RIB integration)
	d.Register("rib show in", handleRIBShowIn, "Show Adj-RIB-In")
	d.Register("rib show out", handleRIBShowOut, "Show Adj-RIB-Out")
}

// handleDaemonShutdown signals the reactor to stop.
func handleDaemonShutdown(ctx *CommandContext, _ []string) (*Response, error) {
	ctx.Reactor.Stop()
	return &Response{
		Status: "done",
		Data: map[string]any{
			"message": "shutdown initiated",
		},
	}, nil
}

// handleDaemonStatus returns daemon status.
func handleDaemonStatus(ctx *CommandContext, _ []string) (*Response, error) {
	stats := ctx.Reactor.Stats()
	return &Response{
		Status: "done",
		Data: map[string]any{
			"uptime":     stats.Uptime.String(),
			"peer_count": stats.PeerCount,
			"start_time": stats.StartTime.Format("2006-01-02T15:04:05Z07:00"),
		},
	}, nil
}

// handlePeerList returns a brief list of all peers.
func handlePeerList(ctx *CommandContext, _ []string) (*Response, error) {
	peers := ctx.Reactor.Peers()
	return &Response{
		Status: "done",
		Data: map[string]any{
			"peers": peers,
		},
	}, nil
}

// handlePeerShow returns detailed peer information.
// If args contains an IP, filters to that specific peer.
func handlePeerShow(ctx *CommandContext, args []string) (*Response, error) {
	allPeers := ctx.Reactor.Peers()

	var peers []PeerInfo

	if len(args) > 0 {
		// Filter to specific peer
		filterIP, err := netip.ParseAddr(args[0])
		if err != nil {
			return &Response{
				Status: "error",
				Error:  fmt.Sprintf("invalid IP address: %s", args[0]),
			}, err
		}

		for _, p := range allPeers {
			if p.Address == filterIP {
				peers = append(peers, p)
				break
			}
		}
	} else {
		peers = allPeers
	}

	return &Response{
		Status: "done",
		Data: map[string]any{
			"peers": peers,
		},
	}, nil
}

// handleSystemHelp returns list of available commands.
func handleSystemHelp(ctx *CommandContext, _ []string) (*Response, error) {
	// We need access to the dispatcher to list commands
	// For now, return a static list
	commands := []string{
		"daemon shutdown - Gracefully shutdown the daemon",
		"daemon status - Show daemon status",
		"peer list - List all peers (brief)",
		"peer show [<ip>] - Show peer details",
		"rib show in - Show Adj-RIB-In",
		"rib show out - Show Adj-RIB-Out",
		"system help - Show available commands",
		"system version - Show version",
	}

	return &Response{
		Status: "done",
		Data: map[string]any{
			"commands": commands,
		},
	}, nil
}

// handleSystemVersion returns version information.
func handleSystemVersion(_ *CommandContext, _ []string) (*Response, error) {
	return &Response{
		Status: "done",
		Data: map[string]any{
			"version": Version,
			"api":     "v6",
		},
	}, nil
}

// handleRIBShowIn returns Adj-RIB-In contents.
// TODO: Implement when RIB is integrated.
func handleRIBShowIn(ctx *CommandContext, _ []string) (*Response, error) {
	return &Response{
		Status: "done",
		Data: map[string]any{
			"routes": []any{},
			"note":   "RIB integration pending",
		},
	}, nil
}

// handleRIBShowOut returns Adj-RIB-Out contents.
// TODO: Implement when RIB is integrated.
func handleRIBShowOut(ctx *CommandContext, _ []string) (*Response, error) {
	return &Response{
		Status: "done",
		Data: map[string]any{
			"routes": []any{},
			"note":   "RIB integration pending",
		},
	}, nil
}
