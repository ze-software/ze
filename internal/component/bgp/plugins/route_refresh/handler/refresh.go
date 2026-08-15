// Design: docs/architecture/api/commands.md — BGP route refresh handlers
// Overview: ../register.go — bgp-route-refresh SDK plugin registration
// Related: clear_soft.go — BGP soft clear handler

package handler

import (
	"errors"
	"fmt"

	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/selector"
)

var errMissingFamily = errors.New("missing family")

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: "ze-bgp:peer-refresh", Handler: handleRefresh, RequiresSelector: true},
		pluginserver.RPCRegistration{WireMethod: "ze-bgp:peer-borr", Handler: handleBoRR, RequiresSelector: true},
		pluginserver.RPCRegistration{WireMethod: "ze-bgp:peer-eorr", Handler: handleEoRR, RequiresSelector: true},
	)
}

// handleRefresh sends a normal ROUTE-REFRESH message.
// RFC 2918 Section 3: "A BGP speaker may send a ROUTE-REFRESH message to
// its peer only if it has received the Route Refresh Capability from its peer.".
func handleRefresh(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	r, errResp, err := requireBGPReactor(ctx)
	if err != nil {
		return errResp, err
	}
	return handleRefreshMarker(ctx, args, "refresh", r.SendRefresh)
}

// handleBoRR sends a Beginning of Route Refresh marker.
// RFC 7313 Section 4: "Before the speaker starts a route refresh...
// the speaker MUST send a BoRR message.".
func handleBoRR(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	r, errResp, err := requireBGPReactor(ctx)
	if err != nil {
		return errResp, err
	}
	return handleRefreshMarker(ctx, args, "borr", r.SendBoRR)
}

// handleEoRR sends an End of Route Refresh marker.
// RFC 7313 Section 4: "After the speaker completes the re-advertisement
// of the entire Adj-RIB-Out to the peer, it MUST send an EoRR message.".
func handleEoRR(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	r, errResp, err := requireBGPReactor(ctx)
	if err != nil {
		return errResp, err
	}
	return handleRefreshMarker(ctx, args, "eorr", r.SendEoRR)
}

// handleRefreshMarker implements the shared logic for borr/eorr commands.
// Usage: bgp peer <selector> {borr|eorr} <family>.
func handleRefreshMarker(
	ctx *pluginserver.CommandContext,
	args []string,
	cmd string,
	send func(*selector.Selector, uint16, uint8, plugin.Sender) error,
) (*plugin.Response, error) {
	if len(args) < 1 {
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  "usage: bgp peer <selector> " + cmd + " <family>",
		}, errMissingFamily
	}

	// Parse family (e.g., "ipv4/unicast")
	fam, ok := family.LookupFamily(args[0])
	if !ok {
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  "invalid family: " + args[0],
		}, fmt.Errorf("invalid family: %s", args[0])
	}

	sel := selector.ParseDefault(ctx.PeerSelector())

	if err := send(sel, uint16(fam.AFI), uint8(fam.SAFI), ctx.Sender); err != nil {
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  fmt.Sprintf("%s failed: %v", cmd, err),
		}, err
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: plugin.Map{
			"selector": sel.String(),
			"family":   fam.String(),
		},
	}, nil
}
