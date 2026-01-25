package plugin

import (
	"fmt"

	"codeberg.org/thomas-mangin/ze/internal/plugin/bgp/nlri"
)

// RFC 7313: Enhanced Route Refresh Capability for BGP-4.
func init() {
	RegisterBuiltin("bgp peer borr", handleBoRR,
		"Send Beginning of Route Refresh: bgp peer <selector> borr <family>")
	RegisterBuiltin("bgp peer eorr", handleEoRR,
		"Send End of Route Refresh: bgp peer <selector> eorr <family>")
}

// handleBoRR sends a Beginning of Route Refresh marker.
// Usage: bgp peer <selector> borr <family>
//
// RFC 7313 Section 4: "Before the speaker starts a route refresh...
// the speaker MUST send a BoRR message.".
func handleBoRR(ctx *CommandContext, args []string) (*Response, error) {
	if len(args) < 1 {
		return &Response{
			Status: statusError,
			Data:   "usage: bgp peer <selector> borr <family>",
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

	// Send BoRR to matching peers
	if err := ctx.Reactor.SendBoRR(peerSelector, uint16(family.AFI), uint8(family.SAFI)); err != nil {
		return &Response{
			Status: statusError,
			Data:   fmt.Sprintf("borr failed: %v", err),
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

// handleEoRR sends an End of Route Refresh marker.
// Usage: bgp peer <selector> eorr <family>
//
// RFC 7313 Section 4: "After the speaker completes the re-advertisement
// of the entire Adj-RIB-Out to the peer, it MUST send an EoRR message.".
func handleEoRR(ctx *CommandContext, args []string) (*Response, error) {
	if len(args) < 1 {
		return &Response{
			Status: statusError,
			Data:   "usage: bgp peer <selector> eorr <family>",
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

	// Send EoRR to matching peers
	if err := ctx.Reactor.SendEoRR(peerSelector, uint16(family.AFI), uint8(family.SAFI)); err != nil {
		return &Response{
			Status: statusError,
			Data:   fmt.Sprintf("eorr failed: %v", err),
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
