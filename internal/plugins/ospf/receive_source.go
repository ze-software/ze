// Design: docs/architecture/ospf/ospf-3-ip-transport.md -- RFC 2328 section 8.2 receive
// checks, the source-network clause of case (1).
// Related: instance.go -- acceptsArea, the dispatcher's area gate that calls this check.
// RFC: rfc/short/rfc2328.md (RFC2328-8.2-2)

package ospf

import (
	"net/netip"

	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// sourceOnInterfaceNetworkLocked reports whether a packet whose Area ID matched the
// receiving interface's area may be processed: its IP source must be on that
// interface's network, except on a point-to-point network. The interface's address and
// mask are the ones its runtime was started with (interfaceRuntimeConfigLocked), the pair
// its own Hellos carry. The caller MUST hold e.mu.
func (e *engine) sourceOnInterfaceNetworkLocked(ic interfaceConfig, source netip.Addr) bool {
	// The source-network clause is OSPFv2's: an OSPFv3 engine runs per link, and its
	// packets come from link-local addresses, so the check is not applied there.
	if e.dispatch.codec.IsV6() {
		return true
	}
	// RFC 2328 Section 8.2: "This comparison should not be performed on point-to-point
	// networks. On point-to-point networks, the interface addresses of each end of the
	// link are assigned independently, if they are assigned at all."
	if ic.NetworkType == types.NetworkPointToPoint {
		return true
	}
	rt := e.interfaces[ic.Name]
	if rt == nil {
		// No runtime interface: nothing is receiving on this network.
		return false
	}
	address, mask := rt.Network()
	// Guard: an interface with no IPv4 address has no network, so no source is on it.
	// Masking with the zero mask would put every source on 0.0.0.0/0 and accept it.
	if mask == ([4]byte{}) {
		return false
	}
	if !source.Is4() {
		return false
	}
	// RFC 2328 Section 8.2: "Therefore, the packet's IP source address is required to be
	// on the same network as the receiving interface. This can be verified by comparing
	// the packet's IP source address to the interface's IP address, after masking both
	// addresses with the interface mask."
	return sameNetworkV4(source.As4(), address, mask)
}

// sameNetworkV4 reports whether a and b are equal under mask.
func sameNetworkV4(a, b, mask [4]byte) bool {
	for idx := range mask {
		if a[idx]&mask[idx] != b[idx]&mask[idx] {
			return false
		}
	}
	return true
}
