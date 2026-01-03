package api

import (
	"fmt"
	"net/netip"
)

// Version is the ZeBGP version string.
const Version = "0.1.0"

// RegisterDefaultHandlers registers all command handlers.
func RegisterDefaultHandlers(d *Dispatcher) {
	// Daemon control
	d.Register("daemon shutdown", handleDaemonShutdown, "Gracefully shutdown the daemon")
	d.Register("daemon status", handleDaemonStatus, "Show daemon status")
	d.Register("daemon reload", handleDaemonReload, "Reload the configuration")

	// Peer operations
	d.Register("peer list", handlePeerList, "List all peers (brief)")
	d.Register("peer show", handlePeerShow, "Show peer details")
	d.Register("peer teardown", handlePeerTeardown, "Teardown a peer session")

	// Teardown command (for "neighbor <ip> teardown <subcode>" syntax)
	d.Register("teardown", handleTeardown, "Teardown peer session with cease subcode")

	// System commands
	d.Register("system help", handleSystemHelp, "Show available commands")
	d.Register("system version", handleSystemVersion, "Show version")

	// RIB operations
	d.Register("rib show in", handleRIBShowIn, "Show Adj-RIB-In")
	d.Register("rib clear in", handleRIBClearIn, "Clear Adj-RIB-In")
	// Note: rib show/clear/flush out removed - Adj-RIB-Out tracking delegated to external API

	// Route operations
	RegisterRouteHandlers(d)

	// Commit operations (transaction-based batching)
	RegisterCommitHandlers(d)

	// Session operations (per-process API connection state)
	RegisterSessionHandlers(d)

	// Forward operations (route reflection via update-id)
	RegisterForwardHandlers(d)
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

// handleDaemonReload reloads the configuration.
func handleDaemonReload(ctx *CommandContext, _ []string) (*Response, error) {
	if err := ctx.Reactor.Reload(); err != nil {
		return &Response{
			Status: "error",
			Data:   fmt.Sprintf("reload failed: %v", err),
		}, err
	}
	return &Response{
		Status: "done",
		Data: map[string]any{
			"message": "configuration reloaded",
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
				Data:   fmt.Sprintf("invalid IP address: %s", args[0]),
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
func handleSystemHelp(_ *CommandContext, _ []string) (*Response, error) {
	// We need access to the dispatcher to list commands
	// For now, return a static list
	commands := []string{
		"daemon shutdown - Gracefully shutdown the daemon",
		"daemon status - Show daemon status",
		"daemon reload - Reload the configuration",
		"peer list - List all peers (brief)",
		"peer show [<ip>] - Show peer details",
		"peer teardown <ip> [reason] - Teardown a peer session",
		"rib show in - Show Adj-RIB-In",
		"rib clear in - Clear Adj-RIB-In",
		"session ack enable - Enable ACK responses (default)",
		"session ack disable - Disable ACK responses",
		"session ack silence - Disable ACK immediately (no response)",
		"session sync enable - Wait for wire transmission before ACK",
		"session sync disable - ACK immediately after RIB update",
		"session reset - Reset session state to defaults",
		"session ping - Health check (returns pong)",
		"session bye - Client disconnect cleanup",
		"announce route <prefix> next-hop <addr> - Announce a route",
		"announce eor [<afi> <safi>] - Send End-of-RIB marker",
		"announce flow match <spec> then <action> - Announce a FlowSpec route",
		"announce vpls rd <rd> ... - Announce a VPLS route",
		"announce l2vpn <type> rd <rd> ... - Announce an L2VPN/EVPN route",
		"withdraw route <prefix> - Withdraw a route",
		"withdraw flow match <spec> - Withdraw a FlowSpec route",
		"withdraw vpls rd <rd> - Withdraw a VPLS route",
		"withdraw l2vpn <type> rd <rd> ... - Withdraw an L2VPN/EVPN route",
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
		},
	}, nil
}

// handlePeerTeardown closes a peer session.
// Usage: peer teardown <ip> [subcode]
// Subcode defaults to 3 (Peer De-configured) if not specified.
func handlePeerTeardown(ctx *CommandContext, args []string) (*Response, error) {
	if len(args) < 1 {
		return &Response{
			Status: "error",
			Data:   "usage: peer teardown <ip> [subcode]",
		}, fmt.Errorf("missing peer address")
	}

	addr, err := netip.ParseAddr(args[0])
	if err != nil {
		return &Response{
			Status: "error",
			Data:   fmt.Sprintf("invalid IP address: %s", args[0]),
		}, err
	}

	// Optional subcode (default: 3 = Peer De-configured)
	subcode := uint8(3)
	if len(args) > 1 {
		var code uint64
		code, err = parseUint(args[1])
		if err != nil || code > 255 {
			return &Response{
				Status: "error",
				Data:   fmt.Sprintf("invalid subcode: %s", args[1]),
			}, fmt.Errorf("invalid subcode: %s", args[1])
		}
		subcode = uint8(code)
	}

	if err := ctx.Reactor.TeardownPeer(addr, subcode); err != nil {
		return &Response{
			Status: "error",
			Data:   fmt.Sprintf("teardown failed: %v", err),
		}, err
	}

	return &Response{
		Status: "done",
		Data: map[string]any{
			"peer":    addr.String(),
			"subcode": subcode,
		},
	}, nil
}

// handleTeardown handles "neighbor <ip> teardown <subcode>" command.
// The neighbor IP is extracted by the dispatcher into ctx.Peer.
// Subcode is the Cease subcode per RFC 4486.
func handleTeardown(ctx *CommandContext, args []string) (*Response, error) {
	if len(args) < 1 {
		return &Response{
			Status: "error",
			Data:   "usage: neighbor <ip> teardown <subcode>",
		}, fmt.Errorf("missing cease subcode")
	}

	// Parse peer address from context
	peer := ctx.PeerSelector()
	if peer == "*" || peer == "" {
		return &Response{
			Status: "error",
			Data:   "teardown requires specific peer: neighbor <ip> teardown <subcode>",
		}, fmt.Errorf("no peer specified")
	}

	addr, err := netip.ParseAddr(peer)
	if err != nil {
		return &Response{
			Status: "error",
			Data:   fmt.Sprintf("invalid peer address: %s", peer),
		}, err
	}

	// Parse subcode
	code, err := parseUint(args[0])
	if err != nil || code > 255 {
		return &Response{
			Status: "error",
			Data:   fmt.Sprintf("invalid subcode: %s", args[0]),
		}, fmt.Errorf("invalid subcode: %s", args[0])
	}
	subcode := uint8(code)

	if err := ctx.Reactor.TeardownPeer(addr, subcode); err != nil {
		return &Response{
			Status: "error",
			Data:   fmt.Sprintf("teardown failed: %v", err),
		}, err
	}

	return &Response{
		Status: "done",
		Data: map[string]any{
			"peer":    addr.String(),
			"subcode": subcode,
		},
	}, nil
}

// parseUint parses a string as unsigned integer.
func parseUint(s string) (uint64, error) {
	if len(s) == 0 {
		return 0, fmt.Errorf("empty string")
	}
	var n uint64
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("invalid digit: %c", c)
		}
		n = n*10 + uint64(c-'0')
	}
	return n, nil
}

// handleRIBShowIn returns Adj-RIB-In contents.
func handleRIBShowIn(ctx *CommandContext, args []string) (*Response, error) {
	// Optional peer filter
	peerID := ""
	if len(args) > 0 {
		peerID = args[0]
	}

	routes := ctx.Reactor.RIBInRoutes(peerID)
	stats := ctx.Reactor.RIBStats()

	return &Response{
		Status: "done",
		Data: map[string]any{
			"routes":      routes,
			"route_count": len(routes),
			"peer_count":  stats.InPeerCount,
		},
	}, nil
}

// handleRIBClearIn clears all routes from Adj-RIB-In.
func handleRIBClearIn(ctx *CommandContext, _ []string) (*Response, error) {
	count := ctx.Reactor.ClearRIBIn()

	return &Response{
		Status: "done",
		Data: map[string]any{
			"routes_cleared": count,
		},
	}, nil
}
