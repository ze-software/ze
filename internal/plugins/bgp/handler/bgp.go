package handler

import (
	"fmt"
	"net/netip"
	"strings"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/plugin"
)

// PeerOpsRPCs returns BGP RPCs for peer operations.
func PeerOpsRPCs() []plugin.RPCRegistration {
	return []plugin.RPCRegistration{
		{WireMethod: "ze-bgp:peer-list", CLICommand: "bgp peer list", Handler: handleBgpPeerList, Help: "List peer(s) (brief)"},
		{WireMethod: "ze-bgp:peer-show", CLICommand: "bgp peer show", Handler: handleBgpPeerShow, Help: "Show peer(s) details"},
		{WireMethod: "ze-bgp:peer-teardown", CLICommand: "bgp peer teardown", Handler: handleTeardown, Help: "Teardown peer session with cease subcode"},
		{WireMethod: "ze-bgp:peer-add", CLICommand: "bgp peer add", Handler: handleBgpPeerAdd, Help: "Add a peer dynamically"},
		{WireMethod: "ze-bgp:peer-remove", CLICommand: "bgp peer remove", Handler: handleBgpPeerRemove, Help: "Remove a peer dynamically"},
	}
}

// IntrospectionRPCs returns RPC registrations for BGP introspection and plugin config.
func IntrospectionRPCs() []plugin.RPCRegistration {
	return []plugin.RPCRegistration{
		{WireMethod: "ze-bgp:help", CLICommand: "bgp help", Handler: handleBgpHelp, Help: "List bgp subcommands"},
		{WireMethod: "ze-bgp:command-list", CLICommand: "bgp command list", Handler: handleBgpCommandList, Help: "List bgp commands"},
		{WireMethod: "ze-bgp:command-help", CLICommand: "bgp command help", Handler: handleBgpCommandHelp, Help: "Show command details"},
		{WireMethod: "ze-bgp:command-complete", CLICommand: "bgp command complete", Handler: handleBgpCommandComplete, Help: "Complete command/args"},
		{WireMethod: "ze-bgp:event-list", CLICommand: "bgp event list", Handler: handleBgpEventList, Help: "List available BGP event types"},
		{WireMethod: "ze-bgp:plugin-encoding", CLICommand: "bgp plugin encoding", Handler: handleBgpPluginEncoding, Help: "Set event encoding (json|text)"},
		{WireMethod: "ze-bgp:plugin-format", CLICommand: "bgp plugin format", Handler: handleBgpPluginFormat, Help: "Set wire format (hex|base64|parsed|full)"},
		{WireMethod: "ze-bgp:plugin-ack", CLICommand: "bgp plugin ack", Handler: handleBgpPluginAck, Help: "Set ACK timing (sync|async)"},
	}
}

// BGP event types.
var bgpEventTypes = []string{
	"update", "open", "notification", "keepalive",
	"refresh", "state", "negotiated",
}

// filterPeersBySelector returns peers matching the context's peer selector.
// If the selector is "*", all peers are returned. Otherwise, filters by IP.
func filterPeersBySelector(ctx *plugin.CommandContext) ([]plugin.PeerInfo, *plugin.Response, error) {
	if ctx.Reactor() == nil {
		return nil, &plugin.Response{Status: plugin.StatusError, Data: "reactor not available"}, fmt.Errorf("reactor not available")
	}
	allPeers := ctx.Reactor().Peers()
	selector := ctx.PeerSelector()

	if selector == "*" {
		return allPeers, nil, nil
	}

	filterIP, err := netip.ParseAddr(selector)
	if err != nil {
		return nil, &plugin.Response{
			Status: plugin.StatusError,
			Data:   fmt.Sprintf("invalid IP address: %s", selector),
		}, err
	}

	for _, p := range allPeers {
		if p.Address == filterIP {
			return []plugin.PeerInfo{p}, nil, nil
		}
	}

	return nil, nil, nil
}

// handleBgpPeerList returns a brief list of peer(s).
// Used by "bgp peer <selector> list" - filters to matching peers.
// The selector is extracted by dispatcher into ctx.Peer.
func handleBgpPeerList(ctx *plugin.CommandContext, _ []string) (*plugin.Response, error) {
	peers, errResp, err := filterPeersBySelector(ctx)
	if errResp != nil {
		return errResp, err
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: map[string]any{
			"peers": peers,
		},
	}, nil
}

// handleBgpPeerShow returns detailed peer information.
// Used by "bgp peer <selector> show" - filters to matching peers.
// The selector is extracted by dispatcher into ctx.Peer.
func handleBgpPeerShow(ctx *plugin.CommandContext, _ []string) (*plugin.Response, error) {
	peers, errResp, err := filterPeersBySelector(ctx)
	if errResp != nil {
		return errResp, err
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: map[string]any{
			"peers": peers,
		},
	}, nil
}

// handleTeardown handles "bgp peer <ip> teardown <subcode>" command.
// The peer IP is extracted by the dispatcher into ctx.Peer.
// Subcode is the Cease subcode per RFC 4486.
func handleTeardown(ctx *plugin.CommandContext, args []string) (*plugin.Response, error) {
	_, errResp, err := plugin.RequireReactor(ctx)
	if err != nil {
		return errResp, err
	}

	if len(args) < 1 {
		return &plugin.Response{
			Status: plugin.StatusError,
			Data:   "usage: bgp peer <ip> teardown <subcode>",
		}, fmt.Errorf("missing cease subcode")
	}

	// Parse peer address from context
	peer := ctx.PeerSelector()
	if peer == "*" || peer == "" {
		return &plugin.Response{
			Status: plugin.StatusError,
			Data:   "teardown requires specific peer: bgp peer <ip> teardown <subcode>",
		}, fmt.Errorf("no peer specified")
	}

	addr, err := netip.ParseAddr(peer)
	if err != nil {
		return &plugin.Response{
			Status: plugin.StatusError,
			Data:   fmt.Sprintf("invalid peer address: %s", peer),
		}, err
	}

	// Parse subcode
	code, err := parseUint(args[0])
	if err != nil || code > 255 {
		return &plugin.Response{
			Status: plugin.StatusError,
			Data:   fmt.Sprintf("invalid subcode: %s", args[0]),
		}, fmt.Errorf("invalid subcode: %s", args[0])
	}
	subcode := uint8(code)

	if err := ctx.Reactor().TeardownPeer(addr, subcode); err != nil {
		return &plugin.Response{
			Status: plugin.StatusError,
			Data:   fmt.Sprintf("teardown failed: %v", err),
		}, err
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
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
func handleBgpPeerAdd(ctx *plugin.CommandContext, args []string) (*plugin.Response, error) {
	_, errResp, err := plugin.RequireReactor(ctx)
	if err != nil {
		return errResp, err
	}

	// Parse peer address from context (extracted by dispatcher)
	peer := ctx.PeerSelector()
	if peer == "*" || peer == "" {
		return &plugin.Response{
			Status: plugin.StatusError,
			Data:   "add requires specific peer: bgp peer <ip> add asn <asn>",
		}, fmt.Errorf("no peer specified")
	}

	addr, err := netip.ParseAddr(peer)
	if err != nil {
		return &plugin.Response{
			Status: plugin.StatusError,
			Data:   fmt.Sprintf("invalid peer address: %s", peer),
		}, err
	}

	// Parse options
	config := plugin.DynamicPeerConfig{Address: addr}

	for i := 0; i < len(args); i++ {
		switch strings.ToLower(args[i]) {
		case "asn":
			if i+1 >= len(args) {
				return &plugin.Response{Status: plugin.StatusError, Data: "missing value for asn"}, fmt.Errorf("missing asn value")
			}
			i++
			asn, err := parseUint(args[i])
			if err != nil || asn > 0xFFFFFFFF {
				return &plugin.Response{Status: plugin.StatusError, Data: fmt.Sprintf("invalid asn: %s", args[i])}, fmt.Errorf("invalid asn: %s", args[i])
			}
			config.PeerAS = uint32(asn)

		case "local-as":
			if i+1 >= len(args) {
				return &plugin.Response{Status: plugin.StatusError, Data: "missing value for local-as"}, fmt.Errorf("missing local-as value")
			}
			i++
			asn, err := parseUint(args[i])
			if err != nil || asn > 0xFFFFFFFF {
				return &plugin.Response{Status: plugin.StatusError, Data: fmt.Sprintf("invalid local-as: %s", args[i])}, fmt.Errorf("invalid local-as: %s", args[i])
			}
			config.LocalAS = uint32(asn)

		case "local-address":
			if i+1 >= len(args) {
				return &plugin.Response{Status: plugin.StatusError, Data: "missing value for local-address"}, fmt.Errorf("missing local-address value")
			}
			i++
			localAddr, err := netip.ParseAddr(args[i])
			if err != nil {
				return &plugin.Response{Status: plugin.StatusError, Data: fmt.Sprintf("invalid local-address: %s", args[i])}, fmt.Errorf("invalid local-address: %s", args[i])
			}
			config.LocalAddress = localAddr

		case "router-id":
			if i+1 >= len(args) {
				return &plugin.Response{Status: plugin.StatusError, Data: "missing value for router-id"}, fmt.Errorf("missing router-id value")
			}
			i++
			rid, err := parseRouterID(args[i])
			if err != nil {
				return &plugin.Response{Status: plugin.StatusError, Data: fmt.Sprintf("invalid router-id: %s", args[i])}, err
			}
			config.RouterID = rid

		case "hold-time":
			if i+1 >= len(args) {
				return &plugin.Response{Status: plugin.StatusError, Data: "missing value for hold-time"}, fmt.Errorf("missing hold-time value")
			}
			i++
			seconds, err := parseUint(args[i])
			if err != nil {
				return &plugin.Response{Status: plugin.StatusError, Data: fmt.Sprintf("invalid hold-time: %s", args[i])}, err
			}
			// RFC 4271: hold time 0 is valid (no keepalives), 3-65535 are valid
			// Cap at reasonable maximum to prevent overflow (1 day = 86400s)
			const maxHoldTime = 86400
			if seconds > maxHoldTime {
				return &plugin.Response{Status: plugin.StatusError, Data: fmt.Sprintf("hold-time too large: %d (max %d)", seconds, maxHoldTime)}, fmt.Errorf("hold-time too large")
			}
			config.HoldTime = time.Duration(seconds) * time.Second

		case "connection":
			if i+1 >= len(args) {
				return &plugin.Response{Status: plugin.StatusError, Data: "connection requires a value (both, passive, active)"}, fmt.Errorf("connection requires a value")
			}
			i++
			v := args[i] //nolint:gosec // bounds checked by i+1 >= len(args) guard above
			if v != "both" && v != "passive" && v != "active" {
				return &plugin.Response{Status: plugin.StatusError, Data: fmt.Sprintf("invalid connection mode: %s", v)}, fmt.Errorf("invalid connection mode: %s", v)
			}
			config.Connection = v

		default: // unknown option → return error
			return &plugin.Response{
				Status: plugin.StatusError,
				Data:   fmt.Sprintf("unknown option: %s", args[i]),
			}, fmt.Errorf("unknown option: %s", args[i])
		}
	}

	// Validate required fields
	if config.PeerAS == 0 {
		return &plugin.Response{
			Status: plugin.StatusError,
			Data:   "asn is required: bgp peer <ip> add asn <asn>",
		}, fmt.Errorf("missing required asn")
	}

	// Add peer via reactor
	if err := ctx.Reactor().AddDynamicPeer(config); err != nil {
		return &plugin.Response{
			Status: plugin.StatusError,
			Data:   fmt.Sprintf("failed to add peer: %v", err),
		}, err
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: map[string]any{
			"peer":    addr.String(),
			"asn":     config.PeerAS,
			"message": "peer added",
		},
	}, nil
}

// handleBgpPeerRemove handles "bgp peer <ip> remove" command.
// Removes a peer dynamically at runtime.
func handleBgpPeerRemove(ctx *plugin.CommandContext, _ []string) (*plugin.Response, error) {
	_, errResp, err := plugin.RequireReactor(ctx)
	if err != nil {
		return errResp, err
	}

	// Parse peer address from context (extracted by dispatcher)
	peer := ctx.PeerSelector()
	if peer == "*" || peer == "" {
		return &plugin.Response{
			Status: plugin.StatusError,
			Data:   "remove requires specific peer: bgp peer <ip> remove",
		}, fmt.Errorf("no peer specified")
	}

	addr, err := netip.ParseAddr(peer)
	if err != nil {
		return &plugin.Response{
			Status: plugin.StatusError,
			Data:   fmt.Sprintf("invalid peer address: %s", peer),
		}, err
	}

	// Remove peer via reactor
	if err := ctx.Reactor().RemovePeer(addr); err != nil {
		return &plugin.Response{
			Status: plugin.StatusError,
			Data:   fmt.Sprintf("failed to remove peer: %v", err),
		}, err
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
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

// handleBgpHelp returns list of bgp subcommands.
func handleBgpHelp(ctx *plugin.CommandContext, _ []string) (*plugin.Response, error) {
	var commands []string

	if ctx.Dispatcher() != nil {
		for _, cmd := range ctx.Dispatcher().Commands() {
			if strings.HasPrefix(cmd.Name, "bgp ") {
				commands = append(commands, cmd.Name+" - "+cmd.Help)
			}
		}
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: map[string]any{
			"commands": commands,
		},
	}, nil
}

// handleBgpCommandList returns commands in bgp namespace.
func handleBgpCommandList(ctx *plugin.CommandContext, args []string) (*plugin.Response, error) {
	verbose := len(args) > 0 && args[0] == argVerbose

	var commands []plugin.Completion

	if ctx.Dispatcher() != nil {
		for _, cmd := range ctx.Dispatcher().Commands() {
			if strings.HasPrefix(cmd.Name, "bgp ") {
				c := plugin.Completion{
					Value: cmd.Name,
					Help:  cmd.Help,
				}
				if verbose {
					c.Source = sourceBuiltin
				}
				commands = append(commands, c)
			}
		}
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: map[string]any{
			"commands": commands,
		},
	}, nil
}

// handleBgpCommandHelp returns detailed help for a bgp command.
func handleBgpCommandHelp(ctx *plugin.CommandContext, args []string) (*plugin.Response, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("usage: bgp command help \"<name>\"")
	}

	name := args[0]

	if ctx.Dispatcher() != nil {
		if cmd := ctx.Dispatcher().Lookup(name); cmd != nil {
			if strings.HasPrefix(cmd.Name, "bgp ") {
				return &plugin.Response{
					Status: plugin.StatusDone,
					Data: map[string]any{
						"command":     cmd.Name,
						"description": cmd.Help,
						"source":      sourceBuiltin,
					},
				}, nil
			}
		}
	}

	return nil, fmt.Errorf("unknown bgp command: %s", name)
}

// handleBgpCommandComplete returns completions for bgp commands.
func handleBgpCommandComplete(ctx *plugin.CommandContext, args []string) (*plugin.Response, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("usage: bgp command complete \"<partial>\"")
	}

	partial := args[0]
	var completions []plugin.Completion

	if ctx.Dispatcher() != nil {
		lowerPartial := strings.ToLower(partial)
		for _, cmd := range ctx.Dispatcher().Commands() {
			if strings.HasPrefix(cmd.Name, "bgp ") &&
				strings.HasPrefix(strings.ToLower(cmd.Name), lowerPartial) {
				completions = append(completions, plugin.Completion{
					Value: cmd.Name,
					Help:  cmd.Help,
				})
			}
		}
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: map[string]any{
			"completions": completions,
		},
	}, nil
}

// handleBgpEventList returns available BGP event types.
func handleBgpEventList(_ *plugin.CommandContext, _ []string) (*plugin.Response, error) {
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: map[string]any{
			"events": bgpEventTypes,
		},
	}, nil
}

// handleBgpPluginEncoding sets event encoding for this process.
// Syntax: bgp plugin encoding <json|text>.
func handleBgpPluginEncoding(ctx *plugin.CommandContext, args []string) (*plugin.Response, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("missing encoding: bgp plugin encoding <json|text>")
	}

	enc := strings.ToLower(args[0])
	switch enc {
	case plugin.EncodingJSON, plugin.EncodingText:
		if ctx.Process != nil {
			ctx.Process.SetEncoding(enc)
		}
	default: // invalid encoding → return error
		return nil, fmt.Errorf("invalid encoding: %s (valid: json, text)", args[0])
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: map[string]any{
			"encoding": enc,
		},
	}, nil
}

// handleBgpPluginFormat sets wire format for this process.
// Syntax: bgp plugin format <hex|base64|parsed|full>.
func handleBgpPluginFormat(ctx *plugin.CommandContext, args []string) (*plugin.Response, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("missing format: bgp plugin format <hex|base64|parsed|full>")
	}

	format := strings.ToLower(args[0])
	switch format {
	case plugin.FormatHex, plugin.FormatBase64, plugin.FormatParsed, plugin.FormatFull:
		if ctx.Process != nil {
			ctx.Process.SetFormat(format)
		}
	default: // invalid format → return error
		return nil, fmt.Errorf("invalid format: %s (valid: hex, base64, parsed, full)", args[0])
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: map[string]any{
			"format": format,
		},
	}, nil
}

// handleBgpPluginAck sets ACK timing for this process.
// Syntax: bgp plugin ack <sync|async>.
func handleBgpPluginAck(ctx *plugin.CommandContext, args []string) (*plugin.Response, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("missing mode: bgp plugin ack <sync|async>")
	}

	mode := strings.ToLower(args[0])
	switch mode {
	case "sync":
		if ctx.Process != nil {
			ctx.Process.SetSync(true)
		}
	case "async":
		if ctx.Process != nil {
			ctx.Process.SetSync(false)
		}
	default: // invalid mode → return error
		return nil, fmt.Errorf("invalid mode: %s (valid: sync, async)", args[0])
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: map[string]any{
			"ack": mode,
		},
	}, nil
}
