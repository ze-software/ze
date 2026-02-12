package plugin

import (
	"fmt"

	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/nlri"
)

// refreshRPCs returns RPC registrations for handlers defined in this file.
func refreshRPCs() []RPCRegistration {
	return []RPCRegistration{
		{"ze-bgp:peer-borr", "bgp peer borr", handleBoRR, "Send Beginning of Route Refresh"},
		{"ze-bgp:peer-eorr", "bgp peer eorr", handleEoRR, "Send End of Route Refresh"},
	}
}

// handleBoRR sends a Beginning of Route Refresh marker.
// RFC 7313 Section 4: "Before the speaker starts a route refresh...
// the speaker MUST send a BoRR message.".
func handleBoRR(ctx *CommandContext, args []string) (*Response, error) {
	r, errResp, err := requireReactor(ctx)
	if err != nil {
		return errResp, err
	}
	return handleRefreshMarker(ctx, args, "borr", r.SendBoRR)
}

// handleEoRR sends an End of Route Refresh marker.
// RFC 7313 Section 4: "After the speaker completes the re-advertisement
// of the entire Adj-RIB-Out to the peer, it MUST send an EoRR message.".
func handleEoRR(ctx *CommandContext, args []string) (*Response, error) {
	r, errResp, err := requireReactor(ctx)
	if err != nil {
		return errResp, err
	}
	return handleRefreshMarker(ctx, args, "eorr", r.SendEoRR)
}

// handleRefreshMarker implements the shared logic for borr/eorr commands.
// Usage: bgp peer <selector> {borr|eorr} <family>.
func handleRefreshMarker(
	ctx *CommandContext,
	args []string,
	cmd string,
	send func(string, uint16, uint8) error,
) (*Response, error) {
	if len(args) < 1 {
		return &Response{
			Status: statusError,
			Data:   fmt.Sprintf("usage: bgp peer <selector> %s <family>", cmd),
		}, fmt.Errorf("missing family")
	}

	// Parse family (e.g., "ipv4/unicast")
	family, ok := nlri.ParseFamily(args[0])
	if !ok {
		return &Response{
			Status: statusError,
			Data:   fmt.Sprintf("invalid family: %s", args[0]),
		}, fmt.Errorf("invalid family: %s", args[0])
	}

	peerSelector := ctx.PeerSelector()

	if err := send(peerSelector, uint16(family.AFI), uint8(family.SAFI)); err != nil {
		return &Response{
			Status: statusError,
			Data:   fmt.Sprintf("%s failed: %v", cmd, err),
		}, err
	}

	return &Response{
		Status: statusDone,
		Data: map[string]any{
			"selector": peerSelector,
			"family":   family.String(),
		},
	}, nil
}
