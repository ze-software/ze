package plugin

import (
	"encoding/json"
	"fmt"
	"net/netip"
	"strings"
	"time"
)

// Version is the ZeBGP version string.
const Version = "0.1.0"

// APIVersion is the IPC protocol version.
const APIVersion = "0.1.0"

// Command source constants.
const (
	sourceBuiltin = "builtin"
	argVerbose    = "verbose"
)

func init() {
	// BGP daemon control (moved from daemon * to bgp daemon *)
	RegisterBuiltin("bgp daemon shutdown", handleDaemonShutdown, "Gracefully shutdown the daemon")
	RegisterBuiltin("bgp daemon status", handleDaemonStatus, "Show daemon status")
	RegisterBuiltin("bgp daemon reload", handleDaemonReload, "Reload the configuration")

	// BGP peer operations (use "bgp peer <selector> <cmd>" syntax)
	// The selector is extracted by dispatcher, handlers receive remaining args
	RegisterBuiltin("bgp peer list", handleBgpPeerList, "List peer(s) (brief)")
	RegisterBuiltin("bgp peer show", handleBgpPeerShow, "Show peer(s) details")
	RegisterBuiltin("bgp peer teardown", handleTeardown, "Teardown peer session with cease subcode")
	RegisterBuiltin("bgp peer add", handleBgpPeerAdd, "Add a peer dynamically")
	RegisterBuiltin("bgp peer remove", handleBgpPeerRemove, "Remove a peer dynamically")

	// System commands
	RegisterBuiltin("system help", handleSystemHelp, "Show available commands")
	RegisterBuiltin("system version software", handleSystemVersionSoftware, "Show ZeBGP version")
	RegisterBuiltin("system version api", handleSystemVersionAPI, "Show IPC protocol version")
	RegisterBuiltin("system shutdown", handleSystemShutdown, "Graceful application shutdown")
	RegisterBuiltin("system subsystem list", handleSystemSubsystemList, "List available subsystems")
	RegisterBuiltin("system command list", handleSystemCommandList, "List all commands")
	RegisterBuiltin("system command help", handleSystemCommandHelp, "Show command details")
	RegisterBuiltin("system command complete", handleSystemCommandComplete, "Complete command/args")

	// RIB namespace (introspection + operations)
	RegisterBuiltin("rib help", handleRibHelp, "Show RIB subcommands")
	RegisterBuiltin("rib command list", handleRibCommandList, "List RIB commands")
	RegisterBuiltin("rib command help", handleRibCommandHelp, "Show RIB command details")
	RegisterBuiltin("rib command complete", handleRibCommandComplete, "Complete RIB command/args")
	RegisterBuiltin("rib event list", handleRibEventList, "List RIB event types")
	RegisterBuiltin("rib show in", handleRIBShowIn, "Show Adj-RIB-In")
	RegisterBuiltin("rib clear in", handleRIBClearIn, "Clear Adj-RIB-In")
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

// handleBgpPeerList returns a brief list of peer(s).
// Used by "bgp peer <selector> list" - filters to matching peers.
// The selector is extracted by dispatcher into ctx.Peer.
func handleBgpPeerList(ctx *CommandContext, _ []string) (*Response, error) {
	allPeers := ctx.Reactor.Peers()
	var peers []PeerInfo

	selector := ctx.PeerSelector()
	if selector == "*" {
		peers = allPeers
	} else {
		// Filter to specific peer(s) matching selector
		filterIP, err := netip.ParseAddr(selector)
		if err != nil {
			return &Response{
				Status: "error",
				Data:   fmt.Sprintf("invalid IP address: %s", selector),
			}, err
		}

		for _, p := range allPeers {
			if p.Address == filterIP {
				peers = append(peers, p)
				break
			}
		}
	}

	return &Response{
		Status: "done",
		Data: map[string]any{
			"peers": peers,
		},
	}, nil
}

// handleBgpPeerShow returns detailed peer information.
// Used by "bgp peer <selector> show" - filters to matching peers.
// The selector is extracted by dispatcher into ctx.Peer.
func handleBgpPeerShow(ctx *CommandContext, _ []string) (*Response, error) {
	allPeers := ctx.Reactor.Peers()
	var peers []PeerInfo

	selector := ctx.PeerSelector()
	if selector == "*" {
		peers = allPeers
	} else {
		// Filter to specific peer(s) matching selector
		filterIP, err := netip.ParseAddr(selector)
		if err != nil {
			return &Response{
				Status: "error",
				Data:   fmt.Sprintf("invalid IP address: %s", selector),
			}, err
		}

		for _, p := range allPeers {
			if p.Address == filterIP {
				peers = append(peers, p)
				break
			}
		}
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
			"bgp daemon shutdown - Gracefully shutdown the daemon",
			"bgp daemon status - Show daemon status",
			"bgp peer <selector> list - List peer(s) (brief)",
			"bgp peer <selector> show - Show peer(s) details",
			"system help - Show available commands",
			"system version software - Show ZeBGP version",
			"system version api - Show IPC protocol version",
		}
	}

	return &Response{
		Status: statusDone,
		Data: map[string]any{
			"commands": commands,
		},
	}, nil
}

// handleSystemVersionSoftware returns ZeBGP version information.
func handleSystemVersionSoftware(_ *CommandContext, _ []string) (*Response, error) {
	return &Response{
		Status: "done",
		Data: map[string]any{
			"version": Version,
		},
	}, nil
}

// handleSystemVersionAPI returns IPC protocol version.
func handleSystemVersionAPI(_ *CommandContext, _ []string) (*Response, error) {
	return &Response{
		Status: "done",
		Data: map[string]any{
			"version": APIVersion,
		},
	}, nil
}

// handleSystemShutdown triggers graceful application shutdown.
func handleSystemShutdown(ctx *CommandContext, _ []string) (*Response, error) {
	ctx.Reactor.Stop()
	return &Response{
		Status: "done",
		Data: map[string]any{
			"message": "shutdown initiated",
		},
	}, nil
}

// handleSystemSubsystemList returns available subsystems.
func handleSystemSubsystemList(_ *CommandContext, _ []string) (*Response, error) {
	// For now, bgp is always available
	// Future: query reactor for enabled subsystems
	subsystems := []string{"bgp"}
	return &Response{
		Status: "done",
		Data: map[string]any{
			"subsystems": subsystems,
		},
	}, nil
}

// handleTeardown handles "bgp peer <ip> teardown <subcode>" command.
// The peer IP is extracted by the dispatcher into ctx.Peer.
// Subcode is the Cease subcode per RFC 4486.
func handleTeardown(ctx *CommandContext, args []string) (*Response, error) {
	if len(args) < 1 {
		return &Response{
			Status: "error",
			Data:   "usage: bgp peer <ip> teardown <subcode>",
		}, fmt.Errorf("missing cease subcode")
	}

	// Parse peer address from context
	peer := ctx.PeerSelector()
	if peer == "*" || peer == "" {
		return &Response{
			Status: "error",
			Data:   "teardown requires specific peer: bgp peer <ip> teardown <subcode>",
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

// handleBgpPeerAdd handles "bgp peer <ip> add asn <asn> [options...]" command.
// Adds a peer dynamically at runtime.
//
// Options:
//
//	asn <asn>           - Required: peer AS number
//	local-as <asn>      - Optional: local AS (default: reactor's LocalAS)
//	local-address <ip>  - Optional: local IP for this session
//	router-id <id>      - Optional: router ID (default: reactor's RouterID)
//	hold-time <seconds> - Optional: hold time in seconds (default: 90)
//	passive             - Optional: listen-only mode (no outgoing connections)
func handleBgpPeerAdd(ctx *CommandContext, args []string) (*Response, error) {
	// Parse peer address from context (extracted by dispatcher)
	peer := ctx.PeerSelector()
	if peer == "*" || peer == "" {
		return &Response{
			Status: statusError,
			Data:   "add requires specific peer: bgp peer <ip> add asn <asn>",
		}, fmt.Errorf("no peer specified")
	}

	addr, err := netip.ParseAddr(peer)
	if err != nil {
		return &Response{
			Status: statusError,
			Data:   fmt.Sprintf("invalid peer address: %s", peer),
		}, err
	}

	// Parse options
	config := DynamicPeerConfig{Address: addr}

	for i := 0; i < len(args); i++ {
		switch strings.ToLower(args[i]) {
		case "asn":
			if i+1 >= len(args) {
				return &Response{Status: statusError, Data: "missing value for asn"}, fmt.Errorf("missing asn value")
			}
			i++
			asn, err := parseUint(args[i])
			if err != nil || asn > 0xFFFFFFFF {
				return &Response{Status: statusError, Data: fmt.Sprintf("invalid asn: %s", args[i])}, fmt.Errorf("invalid asn: %s", args[i])
			}
			config.PeerAS = uint32(asn)

		case "local-as":
			if i+1 >= len(args) {
				return &Response{Status: statusError, Data: "missing value for local-as"}, fmt.Errorf("missing local-as value")
			}
			i++
			asn, err := parseUint(args[i])
			if err != nil || asn > 0xFFFFFFFF {
				return &Response{Status: statusError, Data: fmt.Sprintf("invalid local-as: %s", args[i])}, fmt.Errorf("invalid local-as: %s", args[i])
			}
			config.LocalAS = uint32(asn)

		case "local-address":
			if i+1 >= len(args) {
				return &Response{Status: statusError, Data: "missing value for local-address"}, fmt.Errorf("missing local-address value")
			}
			i++
			localAddr, err := netip.ParseAddr(args[i])
			if err != nil {
				return &Response{Status: statusError, Data: fmt.Sprintf("invalid local-address: %s", args[i])}, fmt.Errorf("invalid local-address: %s", args[i])
			}
			config.LocalAddress = localAddr

		case "router-id":
			if i+1 >= len(args) {
				return &Response{Status: statusError, Data: "missing value for router-id"}, fmt.Errorf("missing router-id value")
			}
			i++
			// Router ID can be IP format (4 bytes) or numeric
			rid, err := parseRouterID(args[i])
			if err != nil {
				return &Response{Status: statusError, Data: fmt.Sprintf("invalid router-id: %s", args[i])}, err
			}
			config.RouterID = rid

		case "hold-time":
			if i+1 >= len(args) {
				return &Response{Status: statusError, Data: "missing value for hold-time"}, fmt.Errorf("missing hold-time value")
			}
			i++
			seconds, err := parseUint(args[i])
			if err != nil {
				return &Response{Status: statusError, Data: fmt.Sprintf("invalid hold-time: %s", args[i])}, err
			}
			// RFC 4271: hold time 0 is valid (no keepalives), 3-65535 are valid
			// Cap at reasonable maximum to prevent overflow (1 day = 86400s)
			const maxHoldTime = 86400
			if seconds > maxHoldTime {
				return &Response{Status: statusError, Data: fmt.Sprintf("hold-time too large: %d (max %d)", seconds, maxHoldTime)}, fmt.Errorf("hold-time too large")
			}
			config.HoldTime = time.Duration(seconds) * time.Second

		case "passive":
			config.Passive = true

		default:
			return &Response{
				Status: statusError,
				Data:   fmt.Sprintf("unknown option: %s", args[i]),
			}, fmt.Errorf("unknown option: %s", args[i])
		}
	}

	// Validate required fields
	if config.PeerAS == 0 {
		return &Response{
			Status: statusError,
			Data:   "asn is required: bgp peer <ip> add asn <asn>",
		}, fmt.Errorf("missing required asn")
	}

	// Add peer via reactor
	if err := ctx.Reactor.AddDynamicPeer(config); err != nil {
		return &Response{
			Status: statusError,
			Data:   fmt.Sprintf("failed to add peer: %v", err),
		}, err
	}

	return &Response{
		Status: statusDone,
		Data: map[string]any{
			"peer":    addr.String(),
			"asn":     config.PeerAS,
			"message": "peer added",
		},
	}, nil
}

// handleBgpPeerRemove handles "bgp peer <ip> remove" command.
// Removes a peer dynamically at runtime.
func handleBgpPeerRemove(ctx *CommandContext, _ []string) (*Response, error) {
	// Parse peer address from context (extracted by dispatcher)
	peer := ctx.PeerSelector()
	if peer == "*" || peer == "" {
		return &Response{
			Status: statusError,
			Data:   "remove requires specific peer: bgp peer <ip> remove",
		}, fmt.Errorf("no peer specified")
	}

	addr, err := netip.ParseAddr(peer)
	if err != nil {
		return &Response{
			Status: statusError,
			Data:   fmt.Sprintf("invalid peer address: %s", peer),
		}, err
	}

	// Remove peer via reactor
	if err := ctx.Reactor.RemovePeer(addr); err != nil {
		return &Response{
			Status: statusError,
			Data:   fmt.Sprintf("failed to remove peer: %v", err),
		}, err
	}

	return &Response{
		Status: statusDone,
		Data: map[string]any{
			"peer":    addr.String(),
			"message": "peer removed",
		},
	}, nil
}

// parseRouterID parses a router ID from string (IP format or numeric).
func parseRouterID(s string) (uint32, error) {
	// Try IP format first (e.g., "192.0.2.1")
	if addr, err := netip.ParseAddr(s); err == nil {
		if !addr.Is4() {
			return 0, fmt.Errorf("router-id must be IPv4: %s", s)
		}
		b := addr.As4()
		return uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3]), nil
	}

	// Try numeric
	n, err := parseUint(s)
	if err != nil {
		return 0, fmt.Errorf("invalid router-id: %s", s)
	}
	if n > 0xFFFFFFFF {
		return 0, fmt.Errorf("router-id out of range: %s", s)
	}
	return uint32(n), nil
}

// handleRibHelp returns list of RIB subcommands.
func handleRibHelp(ctx *CommandContext, _ []string) (*Response, error) {
	subcommands := []string{
		"clear",
		"command",
		"event",
		"show",
	}

	// Add plugin-provided subcommands (e.g., "adjacent" from RIB plugin)
	if ctx.Dispatcher != nil {
		seen := make(map[string]bool)
		for _, sub := range subcommands {
			seen[sub] = true
		}
		for _, cmd := range ctx.Dispatcher.Registry().All() {
			if strings.HasPrefix(cmd.Name, "rib ") {
				parts := strings.SplitN(strings.TrimPrefix(cmd.Name, "rib "), " ", 2)
				if len(parts) > 0 && !seen[parts[0]] {
					subcommands = append(subcommands, parts[0])
					seen[parts[0]] = true
				}
			}
		}
	}

	return &Response{
		Status: statusDone,
		Data: map[string]any{
			"subcommands": subcommands,
		},
	}, nil
}

// handleRibCommandList returns all RIB commands (builtin + plugin).
func handleRibCommandList(ctx *CommandContext, args []string) (*Response, error) {
	verbose := len(args) > 0 && args[0] == argVerbose

	var commands []Completion

	// Add builtin rib commands
	if ctx.Dispatcher != nil {
		for _, cmd := range ctx.Dispatcher.Commands() {
			if strings.HasPrefix(cmd.Name, "rib ") {
				c := Completion{
					Value: cmd.Name,
					Help:  cmd.Help,
				}
				if verbose {
					c.Source = sourceBuiltin
				}
				commands = append(commands, c)
			}
		}

		// Add plugin-provided rib commands
		for _, cmd := range ctx.Dispatcher.Registry().All() {
			if strings.HasPrefix(cmd.Name, "rib ") {
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
	}

	return &Response{
		Status: statusDone,
		Data: map[string]any{
			"commands": commands,
		},
	}, nil
}

// handleRibCommandHelp returns detailed help for a RIB command.
func handleRibCommandHelp(ctx *CommandContext, args []string) (*Response, error) {
	if len(args) < 1 {
		return &Response{
			Status: statusError,
			Data:   "usage: rib command help \"<name>\"",
		}, fmt.Errorf("missing command name")
	}

	name := args[0]
	// Ensure it's a rib command
	if !strings.HasPrefix(name, "rib ") {
		name = "rib " + name
	}

	// Check builtins first
	if ctx.Dispatcher != nil {
		if cmd := ctx.Dispatcher.Lookup(name); cmd != nil {
			return &Response{
				Status: statusDone,
				Data: map[string]any{
					"command":     cmd.Name,
					"description": cmd.Help,
					"source":      sourceBuiltin,
				},
			}, nil
		}

		// Check plugin commands
		if cmd := ctx.Dispatcher.Registry().Lookup(name); cmd != nil {
			return &Response{
				Status: statusDone,
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
		Status: statusError,
		Data:   fmt.Sprintf("unknown rib command: %s", name),
	}, fmt.Errorf("unknown rib command: %s", name)
}

// handleRibCommandComplete returns completions for RIB commands.
func handleRibCommandComplete(ctx *CommandContext, args []string) (*Response, error) {
	if len(args) < 1 {
		return &Response{
			Status: statusError,
			Data:   "usage: rib command complete \"<partial>\"",
		}, fmt.Errorf("missing partial input")
	}

	partial := args[0]
	// Ensure we complete within rib namespace
	if !strings.HasPrefix(partial, "rib ") {
		partial = "rib " + partial
	}

	var completions []Completion

	if ctx.Dispatcher != nil {
		// Complete builtin rib commands
		lowerPartial := strings.ToLower(partial)
		for _, cmd := range ctx.Dispatcher.Commands() {
			if strings.HasPrefix(cmd.Name, "rib ") &&
				strings.HasPrefix(strings.ToLower(cmd.Name), lowerPartial) {
				completions = append(completions, Completion{
					Value: cmd.Name,
					Help:  cmd.Help,
				})
			}
		}

		// Complete plugin rib commands
		for _, c := range ctx.Dispatcher.Registry().Complete(partial) {
			if strings.HasPrefix(c.Value, "rib ") {
				completions = append(completions, c)
			}
		}
	}

	return &Response{
		Status: statusDone,
		Data: map[string]any{
			"completions": completions,
		},
	}, nil
}

// handleRibEventList returns available RIB event types.
func handleRibEventList(_ *CommandContext, _ []string) (*Response, error) {
	// RIB event types per ipc_protocol.md
	events := []string{
		"cache",  // msg-id cache operations (new, expire, evict)
		"route",  // route state changes
		"peer",   // peer RIB state changes
		"memory", // memory pressure events
	}

	return &Response{
		Status: statusDone,
		Data: map[string]any{
			"events": events,
		},
	}, nil
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
		Status: statusDone,
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
		Status: statusDone,
		Data: map[string]any{
			"routes_cleared": count,
		},
	}, nil
}

// handleSystemCommandList returns all commands (builtin + plugin).
func handleSystemCommandList(ctx *CommandContext, args []string) (*Response, error) {
	verbose := len(args) > 0 && args[0] == argVerbose

	var commands []Completion

	// Add builtin commands
	if ctx.Dispatcher != nil {
		for _, cmd := range ctx.Dispatcher.Commands() {
			c := Completion{
				Value: cmd.Name,
				Help:  cmd.Help,
			}
			if verbose {
				c.Source = sourceBuiltin
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
					"source":      sourceBuiltin,
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
