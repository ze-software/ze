package api

import (
	"encoding/json"
	"fmt"
	"net/netip"
	"strings"
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
	d.Register("system command list", handleSystemCommandList, "List all commands")
	d.Register("system command help", handleSystemCommandHelp, "Show command details")
	d.Register("system command complete", handleSystemCommandComplete, "Complete command/args")

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
func handleSystemHelp(ctx *CommandContext, _ []string) (*Response, error) {
	var commands []string

	// Use dispatcher if available
	if ctx.Dispatcher != nil {
		for _, cmd := range ctx.Dispatcher.Commands() {
			commands = append(commands, cmd.Name+" - "+cmd.Help)
		}
		// Add plugin commands
		for _, cmd := range ctx.Dispatcher.Registry().All() {
			line := cmd.Name
			if cmd.Args != "" {
				line += " " + cmd.Args
			}
			line += " - " + cmd.Description
			commands = append(commands, line)
		}
	}

	// Fallback if no dispatcher
	if len(commands) == 0 {
		commands = []string{
			"daemon shutdown - Gracefully shutdown the daemon",
			"daemon status - Show daemon status",
			"peer list - List all peers",
			"system help - Show available commands",
			"system version - Show version",
		}
	}

	return &Response{
		Status: statusDone,
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

// handleSystemCommandList returns all commands (builtin + plugin).
func handleSystemCommandList(ctx *CommandContext, args []string) (*Response, error) {
	verbose := len(args) > 0 && args[0] == "verbose"

	var commands []Completion

	// Add builtin commands
	if ctx.Dispatcher != nil {
		for _, cmd := range ctx.Dispatcher.Commands() {
			c := Completion{
				Value: cmd.Name,
				Help:  cmd.Help,
			}
			if verbose {
				c.Source = "builtin"
			}
			commands = append(commands, c)
		}

		// Add plugin commands
		for _, cmd := range ctx.Dispatcher.Registry().All() {
			c := Completion{
				Value: cmd.Name,
				Help:  cmd.Description,
			}
			if verbose {
				c.Source = cmd.Process.Name()
			}
			commands = append(commands, c)
		}
	}

	return &Response{
		Status: "done",
		Data: map[string]any{
			"commands": commands,
		},
	}, nil
}

// handleSystemCommandHelp returns detailed help for a specific command.
func handleSystemCommandHelp(ctx *CommandContext, args []string) (*Response, error) {
	if len(args) < 1 {
		return &Response{
			Status: "error",
			Data:   "usage: system command help \"<name>\"",
		}, fmt.Errorf("missing command name")
	}

	name := args[0]

	// Check builtins first
	if ctx.Dispatcher != nil {
		if cmd := ctx.Dispatcher.Lookup(name); cmd != nil {
			return &Response{
				Status: "done",
				Data: map[string]any{
					"command":     cmd.Name,
					"description": cmd.Help,
					"source":      "builtin",
				},
			}, nil
		}

		// Check plugin commands
		if cmd := ctx.Dispatcher.Registry().Lookup(name); cmd != nil {
			return &Response{
				Status: "done",
				Data: map[string]any{
					"command":     cmd.Name,
					"description": cmd.Description,
					"args":        cmd.Args,
					"source":      cmd.Process.Name(),
					"timeout":     cmd.Timeout.String(),
				},
			}, nil
		}
	}

	return &Response{
		Status: "error",
		Data:   fmt.Sprintf("unknown command: %s", name),
	}, fmt.Errorf("unknown command: %s", name)
}

// handleSystemCommandComplete returns completions for partial input.
// Usage:
//
//	system command complete "<partial>"           - command completion
//	system command complete "<cmd>" args "<partial>" - arg completion
func handleSystemCommandComplete(ctx *CommandContext, args []string) (*Response, error) {
	if len(args) < 1 {
		return &Response{
			Status: "error",
			Data:   "usage: system command complete \"<partial>\"",
		}, fmt.Errorf("missing partial input")
	}

	partial := args[0]

	// Check for "args" subcommand for argument completion
	// Format: system command complete "<cmd>" args [<completed>...] "<partial>"
	if len(args) >= 3 && args[1] == "args" {
		cmdName := args[0]
		// Last arg is the partial, everything between "args" and last is completed args
		partialArg := args[len(args)-1]
		var completedArgs []string
		if len(args) > 3 {
			completedArgs = args[2 : len(args)-1]
		}
		return handleArgComplete(ctx, cmdName, completedArgs, partialArg)
	}

	var completions []Completion

	if ctx.Dispatcher != nil {
		// Complete builtins
		lowerPartial := strings.ToLower(partial)
		for _, cmd := range ctx.Dispatcher.Commands() {
			if strings.HasPrefix(strings.ToLower(cmd.Name), lowerPartial) {
				completions = append(completions, Completion{
					Value: cmd.Name,
					Help:  cmd.Help,
				})
			}
		}

		// Complete plugin commands
		completions = append(completions, ctx.Dispatcher.Registry().Complete(partial)...)
	}

	return &Response{
		Status: "done",
		Data: map[string]any{
			"completions": completions,
		},
	}, nil
}

// handleArgComplete handles argument completion for a specific command.
func handleArgComplete(ctx *CommandContext, cmdName string, completedArgs []string, partial string) (*Response, error) {
	emptyResult := &Response{
		Status: statusDone,
		Data:   map[string]any{"completions": []Completion{}},
	}

	if ctx.Dispatcher == nil {
		return emptyResult, nil
	}

	// Check if it's a plugin command with completable flag
	cmd := ctx.Dispatcher.Registry().Lookup(cmdName)
	if cmd == nil || !cmd.Completable {
		return emptyResult, nil
	}

	// Route completion request to process
	proc := cmd.Process
	if proc == nil || !proc.Running() {
		return emptyResult, nil
	}

	// Create response channel
	respCh := make(chan *Response, 1)

	// Add pending request with completion timeout
	serial := ctx.Dispatcher.Pending().Add(&PendingRequest{
		Command:  cmd.Name,
		Process:  proc,
		Timeout:  CompletionTimeout,
		RespChan: respCh,
	})

	if serial == "" {
		return emptyResult, nil
	}

	// Build completion request JSON
	request := map[string]any{
		"serial":  serial,
		"type":    "complete",
		"command": cmd.Name,
		"args":    completedArgs,
		"partial": partial,
	}
	reqJSON, _ := json.Marshal(request)

	// Send to process
	if err := proc.WriteEvent(string(reqJSON)); err != nil {
		ctx.Dispatcher.Pending().Complete(serial, emptyResult)
	}

	// Wait for response
	resp := <-respCh
	return resp, nil
}
