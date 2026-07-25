// Design: docs/architecture/api/commands.md — BGP soft clear handler
// Overview: ../register.go — bgp-route-refresh SDK plugin registration
// Related: refresh.go — BGP route refresh handlers

package handler

import (
	"errors"
	"fmt"
	"net/netip"

	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/selector"
)

var errNoPeerSpecified = errors.New("no peer specified")

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: "ze-bgp:peer-clear-soft", Handler: handleBgpPeerClearSoft, RequiresSelector: true},
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
			Error:  "clear soft requires specific peer: bgp peer <ip> clear soft",
		}, errNoPeerSpecified
	}

	addr, err := netip.ParseAddr(peer)
	if err != nil {
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  "invalid peer address: " + peer,
		}, fmt.Errorf("invalid peer address %s: %w", peer, err)
	}

	families, err := r.SoftClearPeer(selector.Addr(addr))
	if err != nil {
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  fmt.Sprintf("soft clear failed: %v", err),
		}, fmt.Errorf("soft clear peer %s: %w", addr, err)
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: plugin.Map{
			"peer":               addr.String(),
			"action":             "soft-clear",
			"families-refreshed": families,
		},
	}, nil
}
