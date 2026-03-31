// Design: docs/architecture/api/commands.md — BGP peer lifecycle and introspection handlers
// Detail: summary.go — BGP summary and capabilities handlers
// Detail: session.go — BGP peer session handlers
// Detail: save.go — BGP peer config persistence
// Detail: prefix_update.go — PeeringDB prefix update command

package peer

import (
	"context"
	"fmt"
	"net/netip"
	"strconv"
	"strings"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/message"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: "ze-bgp:peer-list", Handler: handleBgpPeerList},
		pluginserver.RPCRegistration{WireMethod: "ze-bgp:peer-detail", Handler: HandleBgpPeerDetail},
		pluginserver.RPCRegistration{WireMethod: "ze-bgp:peer-teardown", Handler: handleTeardown, RequiresSelector: true},
		pluginserver.RPCRegistration{WireMethod: "ze-bgp:peer-pause", Handler: handleBgpPeerPause, RequiresSelector: true},
		pluginserver.RPCRegistration{WireMethod: "ze-bgp:peer-resume", Handler: handleBgpPeerResume, RequiresSelector: true},
		pluginserver.RPCRegistration{WireMethod: "ze-bgp:peer-flush", Handler: handleBgpPeerFlush, RequiresSelector: true},
	)
}

// filterPeersBySelector returns peers matching the context's peer selector.
// If the selector is "*", all peers are returned. Otherwise, filters by IP,
// peer name, or ASN ("as<N>" format).
func filterPeersBySelector(ctx *pluginserver.CommandContext) ([]plugin.PeerInfo, *plugin.Response, error) {
	if ctx.Reactor() == nil {
		return nil, &plugin.Response{Status: plugin.StatusError, Data: "reactor not available"}, fmt.Errorf("reactor not available")
	}
	allPeers := ctx.Reactor().Peers()
	selector := ctx.PeerSelector()

	if selector == "*" {
		return allPeers, nil, nil
	}

	// Try IP address match first.
	filterIP, err := netip.ParseAddr(selector)
	if err == nil {
		for i := range allPeers {
			if allPeers[i].Address == filterIP {
				return []plugin.PeerInfo{allPeers[i]}, nil, nil
			}
		}
		return nil, nil, nil
	}

	// Not a valid IP -- try peer name match.
	for i := range allPeers {
		if allPeers[i].Name == selector {
			return []plugin.PeerInfo{allPeers[i]}, nil, nil
		}
	}

	// Try ASN selector: "as<N>" (case-insensitive) matches all peers with that remote AS.
	if len(selector) > 2 && (selector[0] == 'a' || selector[0] == 'A') && (selector[1] == 's' || selector[1] == 'S') {
		if asn, err := strconv.ParseUint(selector[2:], 10, 32); err == nil {
			var matched []plugin.PeerInfo
			for i := range allPeers {
				if uint64(allPeers[i].PeerAS) == asn {
					matched = append(matched, allPeers[i])
				}
			}
			return matched, nil, nil
		}
	}

	return nil, nil, nil
}

// handleBgpPeerList returns a brief list of peer(s) indexed by IP.
// Used by "peer <selector> list" - filters to matching peers.
// The selector is extracted by dispatcher into ctx.Peer.
func handleBgpPeerList(ctx *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	peers, errResp, err := filterPeersBySelector(ctx)
	if errResp != nil {
		return errResp, err
	}

	result := make(map[string]any, len(peers))
	for i := range peers {
		p := &peers[i]
		row := map[string]any{
			"remote-as": p.PeerAS,
			"state":     p.State,
			"uptime":    p.Uptime.String(),
		}
		if p.Name != "" {
			row["name"] = p.Name
		}
		if p.GroupName != "" {
			row["group"] = p.GroupName
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

// HandleBgpPeerDetail returns detailed peer information indexed by IP.
// Used by "show bgp peer <selector>" - filters to matching peers.
// The selector is extracted by dispatcher into ctx.Peer.
func HandleBgpPeerDetail(ctx *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	peers, errResp, err := filterPeersBySelector(ctx)
	if errResp != nil {
		return errResp, err
	}

	result := make(map[string]any, len(peers))
	for i := range peers {
		p := &peers[i]
		rid := p.RouterID
		routerID := netip.AddrFrom4([4]byte{byte(rid >> 24), byte(rid >> 16), byte(rid >> 8), byte(rid)}).String()

		timer := map[string]any{
			"receive-hold-time": int(p.ReceiveHoldTime.Seconds()),
			"send-hold-time":    int(p.SendHoldTime.Seconds()),
			"connect-retry":     int(p.ConnectRetry.Seconds()),
		}
		row := map[string]any{
			"remote-as":           p.PeerAS,
			"local-as":            p.LocalAS,
			"router-id":           routerID,
			"timer":               timer,
			"connect":             p.Connect,
			"accept":              p.Accept,
			"state":               p.State,
			"uptime":              p.Uptime.String(),
			"updates-received":    p.UpdatesReceived,
			"updates-sent":        p.UpdatesSent,
			"keepalives-received": p.KeepalivesReceived,
			"keepalives-sent":     p.KeepalivesSent,
			"eor-received":        p.EORReceived,
			"eor-sent":            p.EORSent,
		}
		if p.Name != "" {
			row["name"] = p.Name
		}
		if p.GroupName != "" {
			row["group"] = p.GroupName
		}
		if p.LocalAddress.IsValid() {
			row["local-ip"] = p.LocalAddress.String()
		}
		if p.PrefixUpdated != "" {
			row["prefix-updated"] = p.PrefixUpdated
			if t, err := time.Parse(time.DateOnly, p.PrefixUpdated); err == nil {
				if time.Since(t) > 180*24*time.Hour {
					row["prefix-stale"] = true
				}
			}
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

// handleTeardown handles "peer <ip> teardown <subcode> [message]" command.
// The peer IP is extracted by the dispatcher into ctx.Peer.
// Subcode is the Cease subcode per RFC 4486.
// RFC 8203: optional message is included in the NOTIFICATION for subcodes 2/4.
func handleTeardown(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	_, errResp, err := pluginserver.RequireReactor(ctx)
	if err != nil {
		return errResp, err
	}

	if len(args) < 1 {
		return &plugin.Response{
			Status: plugin.StatusError,
			Data:   "usage: peer <ip> teardown <subcode> [message]",
		}, fmt.Errorf("missing cease subcode")
	}

	// Parse peer selector from context (name or IP).
	peer := ctx.PeerSelector()
	if peer == "*" || peer == "" {
		return &plugin.Response{
			Status: plugin.StatusError,
			Data:   "teardown requires specific peer: peer <name> teardown <subcode>",
		}, fmt.Errorf("no peer specified")
	}

	// Resolve peer selector to address (supports both name and IP).
	addr, err := netip.ParseAddr(peer)
	if err != nil {
		// Not an IP -- try resolving as a name via peer list.
		found := false
		peers := ctx.Reactor().Peers()
		for i := range peers {
			if peers[i].Name == peer {
				addr = peers[i].Address
				found = true
				break
			}
		}
		if !found {
			return &plugin.Response{
				Status: plugin.StatusError,
				Data:   fmt.Sprintf("unknown peer: %s", peer),
			}, fmt.Errorf("unknown peer %s", peer)
		}
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

	// RFC 8203: optional shutdown communication message (remaining args joined).
	var shutdownMsg string
	if len(args) > 1 {
		shutdownMsg = strings.Join(args[1:], " ")
	}

	if err := ctx.Reactor().TeardownPeer(addr, subcode, shutdownMsg); err != nil {
		return &plugin.Response{
			Status: plugin.StatusError,
			Data:   fmt.Sprintf("teardown failed: %v", err),
		}, fmt.Errorf("teardown peer %s: %w", addr, err)
	}

	resp := map[string]any{
		"peer":    addr.String(),
		"subcode": subcode,
	}
	if shutdownMsg != "" && (subcode == message.NotifyCeaseAdminShutdown || subcode == message.NotifyCeaseAdminReset) {
		// Show the truncated message that was actually sent on the wire (RFC 8203).
		wireData := message.BuildShutdownData(shutdownMsg)
		if wireData[0] > 0 {
			resp["shutdown-message"] = string(wireData[1:])
		}
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data:   resp,
	}, nil
}

// parseUint parses a string as unsigned integer.
// Uses strconv.ParseUint for correct overflow detection.
func parseUint(s string) (uint64, error) {
	if s == "" {
		return 0, fmt.Errorf("empty string")
	}
	return strconv.ParseUint(s, 10, 64)
}

// HandleBgpPeerWith handles "set bgp peer <ip> with <config>" command.
// Delegates to pluginserver.HandleNodeWith with peer-specific validation and apply.
func HandleBgpPeerWith(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	return pluginserver.HandleNodeWith(ctx, args, "bgp.peer", "peer", preparePeerTree,
		func(selector string, tree map[string]any) error {
			addr, _ := netip.ParseAddr(selector) // already validated in preparePeerTree
			return ctx.Reactor().AddDynamicPeer(addr, tree)
		},
	)
}

// preparePeerTree validates and injects peer-specific defaults into the parsed tree.
// selector is the peer IP from the dispatcher; nodeTree is the YANG-parsed config.
func preparePeerTree(selector string, nodeTree map[string]any) (*plugin.Response, error) {
	addr, err := netip.ParseAddr(selector)
	if err != nil {
		return &plugin.Response{
			Status: plugin.StatusError,
			Data:   fmt.Sprintf("invalid peer address: %s", selector),
		}, fmt.Errorf("invalid peer address %s: %w", selector, err)
	}

	// Validate required session.asn.remote and inject connection.remote.ip from selector.
	session, _ := nodeTree["session"].(map[string]any)
	var asn map[string]any
	if session != nil {
		asn, _ = session["asn"].(map[string]any)
	}
	if asn == nil || asn["remote"] == nil {
		return &plugin.Response{
			Status: plugin.StatusError,
			Data:   "remote as is required: set bgp peer <ip> with session asn remote <asn>",
		}, fmt.Errorf("missing required remote as")
	}

	// Inject connection.remote.ip from the peer selector address.
	conn, ok := nodeTree["connection"].(map[string]any)
	if !ok {
		conn = map[string]any{}
		nodeTree["connection"] = conn
	}
	remote, ok := conn["remote"].(map[string]any)
	if !ok {
		remote = map[string]any{}
		conn["remote"] = remote
	}
	remote["ip"] = addr.String()

	return nil, nil //nolint:nilnil // success
}

// HandleBgpPeerRemove handles "del bgp peer <ip>" command.
// Removes a peer dynamically at runtime.
func HandleBgpPeerRemove(ctx *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	_, errResp, err := pluginserver.RequireReactor(ctx)
	if err != nil {
		return errResp, err
	}

	// Parse peer address from context (extracted by dispatcher)
	peer := ctx.PeerSelector()
	if peer == "*" || peer == "" {
		return &plugin.Response{
			Status: plugin.StatusError,
			Data:   "remove requires specific peer: peer <ip> remove",
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

// handleBgpPeerPause handles "peer <ip> pause" command.
// Pauses the peer's read loop for flow control (backpressure from plugins).
func handleBgpPeerPause(ctx *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	return peerFlowControl(ctx, "pause", func(r plugin.ReactorLifecycle, addr netip.Addr) error {
		return r.PausePeer(addr)
	})
}

// handleBgpPeerResume handles "peer <ip> resume" command.
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
			Data:   fmt.Sprintf("%s requires specific peer: peer <ip> %s", action, action),
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

// handleBgpPeerFlush handles "peer <selector> flush" command.
// Blocks until the forward pool has drained all queued items for the targeted peers.
// If selector is "*", flushes all peers. If a specific peer, flushes only that peer.
func handleBgpPeerFlush(ctx *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	_, errResp, err := pluginserver.RequireReactor(ctx)
	if err != nil {
		return errResp, err
	}

	selector := ctx.PeerSelector()
	flushCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if selector == "*" || selector == "" {
		// Flush all peers.
		if err := ctx.Reactor().FlushForwardPool(flushCtx); err != nil {
			return &plugin.Response{
				Status: plugin.StatusError,
				Data:   fmt.Sprintf("flush failed: %v", err),
			}, fmt.Errorf("flush forward pool: %w", err)
		}
		return &plugin.Response{
			Status: plugin.StatusDone,
			Data: map[string]any{
				"action": "flush",
				"peer":   "*",
			},
		}, nil
	}

	// Specific peer: resolve selector to address (supports both name and IP).
	peerAddr := selector
	if _, parseErr := netip.ParseAddr(selector); parseErr != nil {
		// Not an IP -- try resolving as a name via peer list.
		peers := ctx.Reactor().Peers()
		found := false
		for i := range peers {
			if peers[i].Name == selector {
				peerAddr = peers[i].Address.String()
				found = true
				break
			}
		}
		if !found {
			// Unknown selector -- flush by selector string anyway.
			// The forward pool will return immediately if no worker exists.
			peerAddr = selector
		}
	}

	if err := ctx.Reactor().FlushForwardPoolPeer(flushCtx, peerAddr); err != nil {
		return &plugin.Response{
			Status: plugin.StatusError,
			Data:   fmt.Sprintf("flush failed: %v", err),
		}, fmt.Errorf("flush forward pool peer %s: %w", peerAddr, err)
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: map[string]any{
			"action": "flush",
			"peer":   peerAddr,
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
