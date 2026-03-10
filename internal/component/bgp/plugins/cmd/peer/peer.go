// Design: docs/architecture/api/commands.md — BGP peer lifecycle and introspection handlers
// Detail: summary.go — BGP summary and capabilities handlers
// Detail: session.go — BGP peer session handlers

package peer

import (
	"fmt"
	"net/netip"
	"strings"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: "ze-bgp:peer-list", CLICommand: "bgp peer list", Handler: handleBgpPeerList, Help: "List peer(s) (brief)", ReadOnly: true},
		pluginserver.RPCRegistration{WireMethod: "ze-bgp:peer-show", CLICommand: "bgp peer show", Handler: handleBgpPeerShow, Help: "Show peer(s) details", ReadOnly: true},
		pluginserver.RPCRegistration{WireMethod: "ze-bgp:peer-teardown", CLICommand: "bgp peer teardown", Handler: handleTeardown, Help: "Teardown peer session with cease subcode", RequiresSelector: true},
		pluginserver.RPCRegistration{WireMethod: "ze-bgp:peer-add", CLICommand: "bgp peer add", Handler: handleBgpPeerAdd, Help: "Add a peer dynamically", RequiresSelector: true},
		pluginserver.RPCRegistration{WireMethod: "ze-bgp:peer-remove", CLICommand: "bgp peer remove", Handler: handleBgpPeerRemove, Help: "Remove a peer dynamically", RequiresSelector: true},
		pluginserver.RPCRegistration{WireMethod: "ze-bgp:peer-pause", CLICommand: "bgp peer pause", Handler: handleBgpPeerPause, Help: "Pause peer read loop (flow control)", RequiresSelector: true},
		pluginserver.RPCRegistration{WireMethod: "ze-bgp:peer-resume", CLICommand: "bgp peer resume", Handler: handleBgpPeerResume, Help: "Resume peer read loop (flow control)", RequiresSelector: true},
	)
}

// filterPeersBySelector returns peers matching the context's peer selector.
// If the selector is "*", all peers are returned. Otherwise, filters by IP.
func filterPeersBySelector(ctx *pluginserver.CommandContext) ([]plugin.PeerInfo, *plugin.Response, error) {
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
		}, fmt.Errorf("invalid peer address %s: %w", selector, err)
	}

	for _, p := range allPeers {
		if p.Address == filterIP {
			return []plugin.PeerInfo{p}, nil, nil
		}
	}

	return nil, nil, nil
}

// handleBgpPeerList returns a brief list of peer(s) indexed by IP.
// Used by "bgp peer <selector> list" - filters to matching peers.
// The selector is extracted by dispatcher into ctx.Peer.
func handleBgpPeerList(ctx *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	peers, errResp, err := filterPeersBySelector(ctx)
	if errResp != nil {
		return errResp, err
	}

	result := make(map[string]any, len(peers))
	for _, p := range peers {
		result[p.Address.String()] = map[string]any{
			"peer-as": p.PeerAS,
			"state":   p.State,
			"uptime":  p.Uptime.String(),
		}
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: map[string]any{
			"peers": result,
		},
	}, nil
}

// handleBgpPeerShow returns detailed peer information indexed by IP.
// Used by "bgp peer <selector> show" - filters to matching peers.
// The selector is extracted by dispatcher into ctx.Peer.
func handleBgpPeerShow(ctx *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	peers, errResp, err := filterPeersBySelector(ctx)
	if errResp != nil {
		return errResp, err
	}

	result := make(map[string]any, len(peers))
	for _, p := range peers {
		rid := p.RouterID
		routerID := netip.AddrFrom4([4]byte{byte(rid >> 24), byte(rid >> 16), byte(rid >> 8), byte(rid)}).String()

		row := map[string]any{
			"peer-as":             p.PeerAS,
			"local-as":            p.LocalAS,
			"router-id":           routerID,
			"state":               p.State,
			"uptime":              p.Uptime.String(),
			"updates-received":    p.UpdatesReceived,
			"updates-sent":        p.UpdatesSent,
			"keepalives-received": p.KeepalivesReceived,
			"keepalives-sent":     p.KeepalivesSent,
			"eor-received":        p.EORReceived,
			"eor-sent":            p.EORSent,
		}
		if p.LocalAddress.IsValid() {
			row["local-address"] = p.LocalAddress.String()
		}
		result[p.Address.String()] = row
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: map[string]any{
			"peers": result,
		},
	}, nil
}

// handleTeardown handles "bgp peer <ip> teardown <subcode>" command.
// The peer IP is extracted by the dispatcher into ctx.Peer.
// Subcode is the Cease subcode per RFC 4486.
func handleTeardown(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	_, errResp, err := pluginserver.RequireReactor(ctx)
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
		}, fmt.Errorf("invalid peer address %s: %w", peer, err)
	}

	// Parse subcode
	code, err := parseUint(args[0])
	if err != nil {
		return &plugin.Response{
			Status: plugin.StatusError,
			Data:   fmt.Sprintf("invalid subcode: %s", args[0]),
		}, fmt.Errorf("invalid subcode %s: %w", args[0], err)
	}
	if code > 255 {
		return &plugin.Response{
			Status: plugin.StatusError,
			Data:   fmt.Sprintf("invalid subcode: %s (must be 0-255)", args[0]),
		}, fmt.Errorf("subcode out of range: %d", code)
	}
	subcode := uint8(code)

	if err := ctx.Reactor().TeardownPeer(addr, subcode); err != nil {
		return &plugin.Response{
			Status: plugin.StatusError,
			Data:   fmt.Sprintf("teardown failed: %v", err),
		}, fmt.Errorf("teardown peer %s: %w", addr, err)
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
	if s == "" {
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
func handleBgpPeerAdd(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	_, errResp, err := pluginserver.RequireReactor(ctx)
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
		}, fmt.Errorf("invalid peer address %s: %w", peer, err)
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
			if err != nil {
				return &plugin.Response{Status: plugin.StatusError, Data: fmt.Sprintf("invalid asn: %s", args[i])}, fmt.Errorf("invalid asn %s: %w", args[i], err)
			}
			if asn > 0xFFFFFFFF {
				return &plugin.Response{Status: plugin.StatusError, Data: fmt.Sprintf("invalid asn: %s (out of range)", args[i])}, fmt.Errorf("asn out of range: %d", asn)
			}
			config.PeerAS = uint32(asn)

		case "local-as":
			if i+1 >= len(args) {
				return &plugin.Response{Status: plugin.StatusError, Data: "missing value for local-as"}, fmt.Errorf("missing local-as value")
			}
			i++
			asn, err := parseUint(args[i])
			if err != nil {
				return &plugin.Response{Status: plugin.StatusError, Data: fmt.Sprintf("invalid local-as: %s", args[i])}, fmt.Errorf("invalid local-as %s: %w", args[i], err)
			}
			if asn > 0xFFFFFFFF {
				return &plugin.Response{Status: plugin.StatusError, Data: fmt.Sprintf("invalid local-as: %s (out of range)", args[i])}, fmt.Errorf("local-as out of range: %d", asn)
			}
			config.LocalAS = uint32(asn)

		case "local-address":
			if i+1 >= len(args) {
				return &plugin.Response{Status: plugin.StatusError, Data: "missing value for local-address"}, fmt.Errorf("missing local-address value")
			}
			i++
			localAddr, err := netip.ParseAddr(args[i])
			if err != nil {
				return &plugin.Response{Status: plugin.StatusError, Data: fmt.Sprintf("invalid local-address: %s", args[i])}, fmt.Errorf("invalid local-address %s: %w", args[i], err)
			}
			config.LocalAddress = localAddr

		case "router-id":
			if i+1 >= len(args) {
				return &plugin.Response{Status: plugin.StatusError, Data: "missing value for router-id"}, fmt.Errorf("missing router-id value")
			}
			i++
			rid, err := parseRouterID(args[i])
			if err != nil {
				return &plugin.Response{Status: plugin.StatusError, Data: fmt.Sprintf("invalid router-id: %s", args[i])}, fmt.Errorf("invalid router-id %s: %w", args[i], err)
			}
			config.RouterID = rid

		case "hold-time":
			if i+1 >= len(args) {
				return &plugin.Response{Status: plugin.StatusError, Data: "missing value for hold-time"}, fmt.Errorf("missing hold-time value")
			}
			i++
			seconds, err := parseUint(args[i])
			if err != nil {
				return &plugin.Response{Status: plugin.StatusError, Data: fmt.Sprintf("invalid hold-time: %s", args[i])}, fmt.Errorf("invalid hold-time %s: %w", args[i], err)
			}
			// RFC 4271: hold time 0 is valid (no keepalives), 3-65535 are valid
			// Cap at reasonable maximum to prevent overflow (1 day = 86400s)
			const maxHoldTime = 86400
			if seconds > maxHoldTime {
				return &plugin.Response{Status: plugin.StatusError, Data: fmt.Sprintf("hold-time too large: %d (max %d)", seconds, maxHoldTime)}, fmt.Errorf("hold-time out of range: %d (max %d)", seconds, maxHoldTime)
			}
			config.HoldTime = time.Duration(seconds) * time.Second

		case "connection":
			if i+1 >= len(args) {
				return &plugin.Response{Status: plugin.StatusError, Data: "connection requires a value (both, passive, active)"}, fmt.Errorf("missing connection value")
			}
			i++
			v := args[i] //nolint:gosec // bounds checked by i+1 >= len(args) guard above
			if v != "both" && v != "passive" && v != "active" {
				return &plugin.Response{Status: plugin.StatusError, Data: fmt.Sprintf("invalid connection mode: %s", v)}, fmt.Errorf("invalid connection mode: %s", v)
			}
			config.Connection = v

		default: // unknown option -> return error
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
		}, fmt.Errorf("add peer %s: %w", addr, err)
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
func handleBgpPeerRemove(ctx *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	_, errResp, err := pluginserver.RequireReactor(ctx)
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
		}, fmt.Errorf("invalid peer address %s: %w", peer, err)
	}

	// Remove peer via reactor
	if err := ctx.Reactor().RemovePeer(addr); err != nil {
		return &plugin.Response{
			Status: plugin.StatusError,
			Data:   fmt.Sprintf("failed to remove peer: %v", err),
		}, fmt.Errorf("remove peer %s: %w", addr, err)
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: map[string]any{
			"peer":    addr.String(),
			"message": "peer removed",
		},
	}, nil
}

// handleBgpPeerPause handles "bgp peer <ip> pause" command.
// Pauses the peer's read loop for flow control (backpressure from plugins).
func handleBgpPeerPause(ctx *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	return peerFlowControl(ctx, "pause", func(r plugin.ReactorLifecycle, addr netip.Addr) error {
		return r.PausePeer(addr)
	})
}

// handleBgpPeerResume handles "bgp peer <ip> resume" command.
// Resumes the peer's read loop after a flow-control pause.
func handleBgpPeerResume(ctx *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	return peerFlowControl(ctx, "resume", func(r plugin.ReactorLifecycle, addr netip.Addr) error {
		return r.ResumePeer(addr)
	})
}

// peerFlowControl is the shared implementation for pause/resume handlers.
func peerFlowControl(ctx *pluginserver.CommandContext, action string, fn func(plugin.ReactorLifecycle, netip.Addr) error) (*plugin.Response, error) {
	_, errResp, err := pluginserver.RequireReactor(ctx)
	if err != nil {
		return errResp, err
	}

	peer := ctx.PeerSelector()
	if peer == "*" || peer == "" {
		return &plugin.Response{
			Status: plugin.StatusError,
			Data:   fmt.Sprintf("%s requires specific peer: bgp peer <ip> %s", action, action),
		}, fmt.Errorf("no peer specified")
	}

	addr, err := netip.ParseAddr(peer)
	if err != nil {
		return &plugin.Response{
			Status: plugin.StatusError,
			Data:   fmt.Sprintf("invalid peer address: %s", peer),
		}, fmt.Errorf("invalid peer address %s: %w", peer, err)
	}

	if err := fn(ctx.Reactor(), addr); err != nil {
		return &plugin.Response{
			Status: plugin.StatusError,
			Data:   fmt.Sprintf("%s failed: %v", action, err),
		}, fmt.Errorf("%s peer %s: %w", action, addr, err)
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: map[string]any{
			"peer":   addr.String(),
			"action": action,
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
