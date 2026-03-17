// Design: docs/architecture/api/commands.md — BGP soft clear handler
// Overview: ../register.go — bgp-route-refresh SDK plugin registration
// Related: refresh.go — BGP route refresh handlers

package handler

import (
	"fmt"
	"net/netip"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: "ze-bgp:peer-clear-soft", Handler: handleBgpPeerClearSoft, Help: "Soft-clear peer (send ROUTE-REFRESH for all families)", RequiresSelector: true},
	)
}

// handleBgpPeerClearSoft performs a soft clear by sending ROUTE-REFRESH
// for all negotiated families of the specified peer.
// RFC 2918 Section 3: soft reset via route refresh.
func handleBgpPeerClearSoft(ctx *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	r, errResp, err := requireBGPReactor(ctx)
	if err != nil {
		return errResp, err
	}

	peer := ctx.PeerSelector()
	if peer == "*" || peer == "" {
		return &plugin.Response{
			Status: plugin.StatusError,
			Data:   "clear soft requires specific peer: bgp peer <ip> clear soft",
		}, fmt.Errorf("no peer specified")
	}

	addr, err := netip.ParseAddr(peer)
	if err != nil {
		return &plugin.Response{
			Status: plugin.StatusError,
			Data:   fmt.Sprintf("invalid peer address: %s", peer),
		}, fmt.Errorf("invalid peer address %s: %w", peer, err)
	}

	families, err := r.SoftClearPeer(addr.String())
	if err != nil {
		return &plugin.Response{
			Status: plugin.StatusError,
			Data:   fmt.Sprintf("soft clear failed: %v", err),
		}, fmt.Errorf("soft clear peer %s: %w", addr, err)
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: map[string]any{
			"peer":               addr.String(),
			"action":             "soft-clear",
			"families-refreshed": families,
		},
	}, nil
}
