// Design: docs/architecture/api/commands.md — BGP summary and capability handlers
// Overview: peer.go — BGP peer lifecycle and introspection handlers

package peer

import (
	"fmt"
	"net/netip"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: "ze-bgp:summary", Handler: handleBgpSummary, Help: "Show BGP summary (peer table with statistics)", ReadOnly: true},
		pluginserver.RPCRegistration{WireMethod: "ze-bgp:peer-capabilities", Handler: handleBgpPeerCapabilities, Help: "Negotiated capabilities for peer(s)", ReadOnly: true},
		pluginserver.RPCRegistration{WireMethod: "ze-bgp:peer-statistics", Handler: handleBgpPeerStatistics, Help: "Per-peer update statistics with rates", ReadOnly: true},
	)
}

// handleBgpSummary returns a BGP summary table with per-peer statistics.
// Similar to FRR's "show bgp summary" — aggregate totals plus per-peer rows.
func handleBgpSummary(ctx *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	reactor := ctx.Reactor()
	if reactor == nil {
		return &plugin.Response{
			Status: plugin.StatusError,
			Data:   "reactor not available",
		}, fmt.Errorf("reactor not available")
	}

	allPeers := reactor.Peers()
	stats := reactor.Stats()

	established := 0
	peerRows := make([]map[string]any, len(allPeers))
	for i := range allPeers {
		p := &allPeers[i]
		if p.State == "established" {
			established++
		}
		peerRows[i] = map[string]any{
			"address":             p.Address.String(),
			"peer-as":             p.PeerAS,
			"state":               p.State,
			"uptime":              p.Uptime.String(),
			"updates-received":    p.UpdatesReceived,
			"updates-sent":        p.UpdatesSent,
			"keepalives-received": p.KeepalivesReceived,
			"keepalives-sent":     p.KeepalivesSent,
			"eor-received":        p.EORReceived,
			"eor-sent":            p.EORSent,
		}
	}

	// Convert uint32 router-id to dotted-quad IP string.
	rid := stats.RouterID
	routerID := netip.AddrFrom4([4]byte{byte(rid >> 24), byte(rid >> 16), byte(rid >> 8), byte(rid)}).String()

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: map[string]any{
			"summary": map[string]any{
				"router-id":         routerID,
				"local-as":          stats.LocalAS,
				"uptime":            stats.Uptime.String(),
				"peers-configured":  len(allPeers),
				"peers-established": established,
				"peers":             peerRows,
			},
		},
	}, nil
}

// handleBgpPeerCapabilities returns negotiated capabilities for matched peers.
// If no OPEN exchange completed, returns negotiation-complete=false per peer.
// Single peer: flat object. Multiple peers: array of objects.
func handleBgpPeerCapabilities(ctx *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	peers, errResp, err := filterPeersBySelector(ctx)
	if errResp != nil {
		return errResp, err
	}

	if len(peers) == 0 {
		return &plugin.Response{
			Status: plugin.StatusError,
			Data:   "no matching peers",
		}, fmt.Errorf("no matching peers")
	}

	reactor := ctx.Reactor()
	results := make([]map[string]any, len(peers))
	for i := range peers {
		peer := &peers[i]
		caps := reactor.PeerNegotiatedCapabilities(peer.Address)

		entry := map[string]any{
			"peer":  peer.Address.String(),
			"state": peer.State,
		}

		if caps != nil {
			entry["negotiation-complete"] = true
			neg := map[string]any{
				"families":               caps.Families,
				"extended-message":       caps.ExtendedMessage,
				"enhanced-route-refresh": caps.EnhancedRouteRefresh,
				"asn4":                   caps.ASN4,
			}
			if caps.AddPath != nil {
				neg["add-path"] = caps.AddPath
			}
			entry["negotiated"] = neg
		} else {
			entry["negotiation-complete"] = false
		}
		results[i] = entry
	}

	// Single peer: flat object. Multiple: array.
	var data any = results
	if len(results) == 1 {
		data = results[0]
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data:   data,
	}, nil
}

// handleBgpPeerStatistics returns per-peer update statistics with rates.
// Rate is computed from cumulative counters and uptime: counter / uptime_seconds.
// Returns 0 for all rates when uptime is zero (peer not established).
// Single peer: flat object. Multiple peers: array.
func handleBgpPeerStatistics(ctx *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	peers, errResp, err := filterPeersBySelector(ctx)
	if errResp != nil {
		return errResp, err
	}

	if len(peers) == 0 {
		return &plugin.Response{
			Status: plugin.StatusError,
			Data:   "no matching peers",
		}, fmt.Errorf("no matching peers")
	}

	results := make([]map[string]any, len(peers))
	for i := range peers {
		p := &peers[i]
		uptimeSec := p.Uptime.Seconds()

		entry := map[string]any{
			"address":             p.Address.String(),
			"peer-as":             p.PeerAS,
			"state":               p.State,
			"uptime":              p.Uptime.String(),
			"updates-received":    p.UpdatesReceived,
			"updates-sent":        p.UpdatesSent,
			"keepalives-received": p.KeepalivesReceived,
			"keepalives-sent":     p.KeepalivesSent,
			"eor-received":        p.EORReceived,
			"eor-sent":            p.EORSent,
		}

		// Compute rates from cumulative counters / uptime.
		// Zero uptime (not established) → zero rates.
		if uptimeSec > 0 {
			entry["rate-updates-received"] = float64(p.UpdatesReceived) / uptimeSec
			entry["rate-updates-sent"] = float64(p.UpdatesSent) / uptimeSec
			entry["rate-keepalives-received"] = float64(p.KeepalivesReceived) / uptimeSec
			entry["rate-keepalives-sent"] = float64(p.KeepalivesSent) / uptimeSec
		} else {
			entry["rate-updates-received"] = 0.0
			entry["rate-updates-sent"] = 0.0
			entry["rate-keepalives-received"] = 0.0
			entry["rate-keepalives-sent"] = 0.0
		}

		results[i] = entry
	}

	// Single peer: flat object. Multiple: array.
	var data any = results
	if len(results) == 1 {
		data = results[0]
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data:   data,
	}, nil
}
