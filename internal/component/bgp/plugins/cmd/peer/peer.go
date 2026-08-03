// Design: docs/architecture/api/commands.md — BGP peer lifecycle and introspection handlers
// Detail: summary.go — BGP summary and capabilities handlers
// Detail: session.go — BGP peer session handlers

package peer

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"strconv"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	peersel "github.com/ze-software/ze/internal/core/selector"
	"github.com/ze-software/ze/internal/core/textbuf"
)

var (
	errReactorNotAvailable = errors.New("reactor not available")
	errMissingCeaseSubcode = errors.New("missing cease subcode")
	errEmptyString         = errors.New("empty string")
)

func notifDirection(recv bool) string {
	if recv {
		return "received"
	}
	return "sent"
}

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: "ze-bgp:peer-list", Handler: handleBgpPeerList},
		pluginserver.RPCRegistration{WireMethod: "ze-bgp:peer-detail", Handler: HandleBgpPeerDetail},
		pluginserver.RPCRegistration{WireMethod: "ze-bgp:peer-teardown", Handler: handleTeardown, RequiresSelector: true},
		pluginserver.RPCRegistration{WireMethod: "ze-bgp:peer-pause", Handler: handleBgpPeerPause, RequiresSelector: true},
		pluginserver.RPCRegistration{WireMethod: "ze-bgp:peer-resume", Handler: handleBgpPeerResume, RequiresSelector: true},
		pluginserver.RPCRegistration{WireMethod: "ze-bgp:peer-flush", Handler: handleBgpPeerFlush, RequiresSelector: true},
		// Additional owner-registered BGP peer commands.
		pluginserver.RPCRegistration{WireMethod: "ze-bgp:peer-history", Handler: handlePeerHistory, RequiresSelector: true},
		pluginserver.RPCRegistration{WireMethod: "ze-delete:bgp-peer", Handler: HandleBgpPeerRemove, RequiresSelector: true},
		pluginserver.RPCRegistration{WireMethod: "ze-update:bgp-peer-prefix", Handler: handleBgpPeerPrefixUpdate, RequiresSelector: true},
	)
}

func filterPeersByArgs(ctx *pluginserver.CommandContext, args []string) ([]plugin.PeerInfo, *plugin.Response, error) {
	selector := ctx.PeerSelector()
	if len(args) > 0 && args[0] != "" {
		selector = args[0]
	}
	return filterPeersBySelectorValue(ctx, selector)
}

func filterPeersBySelectorValue(ctx *pluginserver.CommandContext, selectorStr string) ([]plugin.PeerInfo, *plugin.Response, error) {
	if ctx.Reactor() == nil {
		return nil, &plugin.Response{Status: plugin.StatusError, Error: "reactor not available"}, errReactorNotAvailable
	}
	allPeers := ctx.Reactor().Peers()

	sel := peersel.ParseDefault(selectorStr)

	switch sel.SelectorKind() {
	case peersel.KindAll:
		return allPeers, nil, nil

	case peersel.KindAddr:
		ip := sel.IP()
		for i := range allPeers {
			if allPeers[i].Address == ip {
				if sel.IsExclude() {
					return excludePeer(allPeers, i), nil, nil
				}
				return []plugin.PeerInfo{allPeers[i]}, nil, nil
			}
		}
		if sel.IsExclude() {
			return allPeers, nil, nil
		}
		return nil, nil, nil

	case peersel.KindName:
		name := sel.NameValue()
		for i := range allPeers {
			if allPeers[i].Name == name {
				if sel.IsExclude() {
					return excludePeer(allPeers, i), nil, nil
				}
				return []plugin.PeerInfo{allPeers[i]}, nil, nil
			}
		}
		if sel.IsExclude() {
			return allPeers, nil, nil
		}
		return nil, nil, nil

	case peersel.KindASN:
		asn := sel.ASNValue()
		var matched []plugin.PeerInfo
		for i := range allPeers {
			asnMatch := allPeers[i].PeerAS == asn
			if sel.IsExclude() {
				asnMatch = !asnMatch
			}
			if asnMatch {
				matched = append(matched, allPeers[i])
			}
		}
		return matched, nil, nil

	case peersel.KindAddrs:
		var matched []plugin.PeerInfo
		for i := range allPeers {
			if sel.Matches(allPeers[i].Address) {
				matched = append(matched, allPeers[i])
			}
		}
		return matched, nil, nil

	case peersel.KindGlob:
		var matched []plugin.PeerInfo
		for i := range allPeers {
			if sel.Matches(allPeers[i].Address) {
				matched = append(matched, allPeers[i])
			}
		}
		return matched, nil, nil
	}

	return nil, nil, nil
}

func excludePeer(all []plugin.PeerInfo, idx int) []plugin.PeerInfo {
	result := make([]plugin.PeerInfo, 0, len(all)-1)
	for i := range all {
		if i != idx {
			result = append(result, all[i])
		}
	}
	return result
}

// handleBgpPeerList returns a brief list of peer(s) indexed by IP.
// Used by "peer <selector> list" - filters to matching peers.
// The selector is extracted by dispatcher into ctx.Peer.
func handleBgpPeerList(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	peers, errResp, err := filterPeersByArgs(ctx, args)
	if errResp != nil {
		return errResp, err
	}

	result := make(map[string]any, len(peers))
	for i := range peers {
		p := &peers[i]
		row := map[string]any{
			"remote-as": p.PeerAS,
			"state":     p.State.String(),
			"uptime":    p.Uptime.Truncate(time.Second).String(),
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
		Data: plugin.Map{
			"peers": result,
		},
	}, nil
}

// HandleBgpPeerDetail returns detailed peer information indexed by IP.
// Used by "show bgp peer <selector>" - filters to matching peers.
// The selector is extracted by dispatcher into ctx.Peer.
func HandleBgpPeerDetail(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	peers, errResp, err := filterPeersByArgs(ctx, args)
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
			"keepalive":         int(p.KeepaliveTime.Seconds()),
			"connect-retry":     int(p.ConnectRetry.Seconds()),
		}
		row := map[string]any{
			"remote-as":           p.PeerAS,
			"local-as":            p.LocalAS,
			"router-id":           routerID,
			"peer-type":           p.PeerType,
			"timer":               timer,
			"connect":             p.Connect,
			"accept":              p.Accept,
			"state":               p.State.String(),
			"uptime":              p.Uptime.Truncate(time.Second).String(),
			"updates-received":    p.UpdatesReceived,
			"updates-sent":        p.UpdatesSent,
			"keepalives-received": p.KeepalivesReceived,
			"keepalives-sent":     p.KeepalivesSent,
			"eor-received":        p.EORReceived,
			"eor-sent":            p.EORSent,
			"messages": map[string]any{
				"received": map[string]any{
					"opens":         p.OpensReceived,
					"updates":       p.UpdatesReceived,
					"notifications": p.NotificationsReceived,
					"keepalives":    p.KeepalivesReceived,
					"route-refresh": p.RefreshReceived,
					"eor":           p.EORReceived,
					"total":         p.OpensReceived + p.UpdatesReceived + p.NotificationsReceived + p.KeepalivesReceived + p.RefreshReceived + p.EORReceived,
				},
				"sent": map[string]any{
					"opens":         p.OpensSent,
					"updates":       p.UpdatesSent,
					"notifications": p.NotificationsSent,
					"keepalives":    p.KeepalivesSent,
					"route-refresh": p.RefreshSent,
					"eor":           p.EORSent,
					"total":         p.OpensSent + p.UpdatesSent + p.NotificationsSent + p.KeepalivesSent + p.RefreshSent + p.EORSent,
				},
			},
			"connections-established": p.ConnectionsEstablished,
			"connections-dropped":     p.ConnectionsDropped,
			"flap-count":              p.FlapCount,
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
		if p.LocalPort != 0 {
			row["local-port"] = p.LocalPort
		}
		if p.RemotePort != 0 {
			row["remote-port"] = p.RemotePort
		}
		if p.MD5Enabled {
			row["md5"] = true
		}
		if p.BFDEnabled {
			row["bfd"] = true
		}
		if p.GTSMOutTTL != 0 {
			row["gtsm-ttl-out"] = p.GTSMOutTTL
		}
		if p.GTSMMinTTL != 0 {
			row["gtsm-ttl-min"] = p.GTSMMinTTL
		}
		if p.RouteReflectorClient {
			row["route-reflector-client"] = true
		}
		if p.ClusterID != 0 {
			cid := p.ClusterID
			row["cluster-id"] = netip.AddrFrom4([4]byte{byte(cid >> 24), byte(cid >> 16), byte(cid >> 8), byte(cid)}).String()
		}
		nhModes := [4]string{"auto", "self", "unchanged", "explicit"}
		if p.NextHopMode < 4 && p.NextHopMode != 0 {
			row["next-hop"] = nhModes[p.NextHopMode]
			if p.NextHopMode == 3 && p.NextHopAddress.IsValid() {
				row["next-hop-address"] = p.NextHopAddress.String()
			}
		}
		if p.NegotiatedHoldTime > 0 {
			row["negotiated-hold-time"] = int(p.NegotiatedHoldTime.Seconds())
			row["negotiated-keepalive-time"] = int(p.NegotiatedKeepaliveTime.Seconds())
		}
		if !p.LastNotifTime.IsZero() {
			row["last-notification"] = map[string]any{
				"code":      p.LastNotifCode,
				"subcode":   p.LastNotifSubcode,
				"direction": notifDirection(p.LastNotifRecv),
				"time":      p.LastNotifTime.UTC().Format(time.RFC3339),
			}
		}
		if !p.LastReadTime.IsZero() {
			row["last-read"] = p.LastReadTime.UTC().Format(time.RFC3339)
		}
		if !p.LastWriteTime.IsZero() {
			row["last-write"] = p.LastWriteTime.UTC().Format(time.RFC3339)
		}
		if len(p.ImportFilters) > 0 {
			row["import-policy"] = p.ImportFilters
		}
		if len(p.ExportFilters) > 0 {
			row["export-policy"] = p.ExportFilters
		}
		caps := map[string]any{
			"negotiation-complete":   p.NegotiationComplete,
			"asn4":                   p.NegotiatedASN4,
			"extended-message":       p.NegotiatedExtMsg,
			"route-refresh":          p.NegotiatedRouteRefresh,
			"enhanced-route-refresh": p.NegotiatedEnhancedRR,
		}
		if p.NegotiationComplete {
			famStrs := make([]string, len(p.NegotiatedFamilies))
			for j, f := range p.NegotiatedFamilies {
				famStrs[j] = f.String()
			}
			caps["families"] = famStrs
			if p.NegotiatedAddPath != nil {
				caps["add-path"] = p.NegotiatedAddPath
			}
			if p.GracefulRestart {
				caps["graceful-restart"] = map[string]any{
					"restart-time": p.GRRestartTime,
				}
			}
		}
		row["capabilities"] = caps
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
		Data: plugin.Map{
			"peers": result,
		},
	}, nil
}

// handleTeardown handles "request peer <sel> teardown <subcode> [message]" command.
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
			Error:  "usage: peer <ip> teardown <subcode> [message]",
		}, errMissingCeaseSubcode
	}

	// One resolver for every peer-scoped command. This used to be a hand-rolled
	// "not an IP, try the name" loop that knew only two of the six selector
	// forms the YANG leaf advertises, so `request peer as65001 teardown` failed
	// with "unknown peer" while `request peer as65001 pause` -- the same
	// selector, the same peer, the adjacent verb -- succeeded. The wildcard and
	// exclusion refusals come with it (ai/rules/evidence.md).
	addr, errResp, err := pluginserver.ResolveSinglePeer(ctx, "teardown")
	if err != nil {
		return errResp, err
	}

	// Parse subcode
	code, err := parseUint(args[0])
	if err != nil {
		var tb textbuf.Buffer
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  tb.Str("invalid subcode: ").Str(args[0]).String(),
		}, fmt.Errorf("invalid subcode %s: %w", args[0], err)
	}
	if code > 255 {
		var tb textbuf.Buffer
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  tb.Str("invalid subcode: ").Str(args[0]).Str(" (must be 0-255)").String(),
		}, fmt.Errorf("subcode out of range: %d", code)
	}
	subcode := uint8(code)

	// RFC 8203: optional shutdown communication message (remaining args joined).
	var shutdownMsg string
	if len(args) > 1 {
		shutdownMsg = textbuf.Join(args[1:], " ")
	}

	if err := ctx.Reactor().TeardownPeer(addr, subcode, shutdownMsg); err != nil {
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  fmt.Sprintf("teardown failed: %v", err),
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
		Data:   plugin.Map(resp),
	}, nil
}

// parseUint parses a string as unsigned integer.
// Uses strconv.ParseUint for correct overflow detection.
func parseUint(s string) (uint64, error) {
	if s == "" {
		return 0, errEmptyString
	}
	return strconv.ParseUint(s, 10, 64)
}

// HandleBgpPeerRemove handles "delete bgp peer <ip>" command.
// Removes a peer dynamically at runtime.
func HandleBgpPeerRemove(ctx *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	_, errResp, err := pluginserver.RequireReactor(ctx)
	if err != nil {
		return errResp, err
	}

	// Accepts an address OR a configured peer name; the wildcard is refused,
	// since removing every peer at once is never what a bare selector means.
	addr, errResp, err := pluginserver.ResolveSinglePeer(ctx, "remove")
	if err != nil {
		return errResp, err
	}

	// Remove peer via reactor
	if err := ctx.Reactor().RemovePeer(addr); err != nil {
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  fmt.Sprintf("failed to remove peer: %v", err),
		}, fmt.Errorf("remove peer %s: %w", addr, err)
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: plugin.Map{
			"peer":    addr.String(),
			"message": "peer removed",
		},
	}, nil
}

// handleBgpPeerPause handles "request peer <sel> pause" command.
// Pauses the peer's read loop for flow control (backpressure from plugins).
func handleBgpPeerPause(ctx *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	return peerFlowControl(ctx, "pause", func(r plugin.ReactorLifecycle, addr netip.Addr) error {
		return r.PausePeer(addr)
	})
}

// handleBgpPeerResume handles "request peer <sel> resume" command.
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

	// Accepts an address OR a configured peer name; the wildcard is refused,
	// since flow control acts on a single peer's read loop.
	addr, errResp, err := pluginserver.ResolveSinglePeer(ctx, action)
	if err != nil {
		return errResp, err
	}

	if err := fn(ctx.Reactor(), addr); err != nil {
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  fmt.Sprintf("%s failed: %v", action, err),
		}, fmt.Errorf("%s peer %s: %w", action, addr, err)
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: plugin.Map{
			"peer":   addr.String(),
			"action": action,
		},
	}, nil
}

// handleBgpPeerFlush handles "request peer <sel> flush" command.
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
				Error:  fmt.Sprintf("flush failed: %v", err),
			}, fmt.Errorf("flush forward pool: %w", err)
		}
		return &plugin.Response{
			Status: plugin.StatusDone,
			Data: plugin.Map{
				"action": "flush",
				"peer":   "*",
			},
		}, nil
	}

	// Specific peer: the same resolver every other peer-scoped command uses.
	//
	// This replaces a second hand-rolled name loop whose miss branch FAILED OPEN:
	// an unresolvable selector was handed to the forward pool verbatim, the pool
	// found no worker for it, returned immediately, and the handler reported
	// StatusDone. A typo'd peer name therefore claimed to have drained a queue it
	// never looked at -- a silent no-op reported as success, which is the one
	// thing a barrier must never do (ai/rules/evidence.md). It now
	// errors with the selector quoted.
	//
	// An unmatched ADDRESS still passes through unchanged, which is
	// ResolveSinglePeer's documented contract and keeps flushing a peer that is
	// configured but has no live worker a successful no-op rather than an error.
	addr, errResp, err := pluginserver.ResolveSinglePeer(ctx, "flush")
	if err != nil {
		return errResp, err
	}
	peerAddr := addr.String()

	if err := ctx.Reactor().FlushForwardPoolPeer(flushCtx, peerAddr); err != nil {
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  fmt.Sprintf("flush failed: %v", err),
		}, fmt.Errorf("flush forward pool peer %s: %w", peerAddr, err)
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: plugin.Map{
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

func handlePeerHistory(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	if len(args) == 0 && ctx.PeerSelector() == "*" {
		return &plugin.Response{Status: plugin.StatusError, Error: "no peer specified"}, nil
	}
	peers, errResp, err := filterPeersByArgs(ctx, args)
	if err != nil {
		return errResp, nil //nolint:nilerr // operational error in Response
	}
	if len(peers) == 0 {
		return &plugin.Response{Status: plugin.StatusError, Error: "peer not found"}, nil
	}
	hp, ok := ctx.Reactor().(plugin.FSMHistoryProvider)
	if !ok || hp == nil {
		return &plugin.Response{Status: plugin.StatusError, Error: "FSM history not available"}, nil
	}
	addr := peers[0].Address.String()
	transitions := hp.PeerFSMHistory(addr)
	if transitions == nil {
		var tb textbuf.Buffer
		return &plugin.Response{Status: plugin.StatusError, Error: tb.Str("no history for peer ").Str(addr).String()}, nil
	}
	out := make([]map[string]any, 0, len(transitions))
	for i := range transitions {
		t := &transitions[i]
		m := map[string]any{
			"from": t.From,
			"to":   t.To,
		}
		if !t.Timestamp.IsZero() {
			m["timestamp"] = t.Timestamp.UTC().Format("2006-01-02T15:04:05Z07:00")
		}
		if t.Reason != "" {
			m["reason"] = t.Reason
		}
		out = append(out, m)
	}
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: plugin.Map{
			"peer":        addr,
			"transitions": out,
			"count":       len(out),
		},
	}, nil
}
