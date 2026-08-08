// Design: docs/architecture/core-design.md — the config leaves that decide the MP_REACH next-hop field
// RFC: rfc/short/rfc2545.md — Section 3, the first address of the Next Hop field is the global one
// Overview: config.go — parsePeerFromTree, the peer config tree parser
// Related: link_scope.go — Section 3's inclusion condition, decided against the host interface table

package reactor

import (
	"fmt"
	"net/netip"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// ValidatePeerGlobalNextHop refuses a peer config value that would occupy the
// FIRST address of the MP_REACH_NLRI Network Address of Next Hop field.
//
// RFC 2545 Section 3: "A BGP speaker shall advertise to its peer in the Network
// Address of Next Hop field the global IPv6 address of the next hop, potentially
// followed by the link-local IPv6 address of the next hop." A link-local address
// in the first slot is the shape that sentence excludes, whatever the second slot
// holds. The `session > link-local` leaf is where a link-local address belongs:
// ze appends it after the global one, and only when Section 3's condition holds
// (linkScope.linkLocalNextHop, link_scope.go).
//
// The three route-level entry points guard the same field with the same helper:
// ParseRouteAttributes (../config/routeattr.go), handleAnnounceUnicast
// (../plugins/cmd/announce/announce.go) and parseNhopFlat
// (../plugins/cmd/update/update_text.go).
//
// leaf names the config leaf for the operator; peer names the peer.
func ValidatePeerGlobalNextHop(peer, leaf string, addr netip.Addr) error {
	if err := attribute.ValidateGlobalNextHop(addr); err != nil {
		return fmt.Errorf("peer %s: %s: %w", peer, leaf, err)
	}
	return nil
}

// applyLocalAddress parses `connection > local > ip`, which is required and takes
// either an address or "auto".
//
// The address is the session's local endpoint AND the value `next-hop self` and
// the default-originate rail write into the global next-hop slot
// (precomputeNextHop, peer_forward_facts.go; sendDefaultOriginateRoutes,
// peer_initial_sync.go), so RFC 2545 Section 3 governs it.
func applyLocalAddress(ps *PeerSettings, name string, connMap map[string]any) error {
	var localAddrStr string
	if connMap != nil {
		if connLocalMap, ok := mapMap(connMap, "local"); ok {
			localAddrStr, _ = mapString(connLocalMap, "ip")
		}
	}
	if localAddrStr == "" {
		return fmt.Errorf("peer %s: local ip is required (use IP address or \"auto\")", name)
	}
	if localAddrStr == valAuto {
		return nil
	}
	la, err := netip.ParseAddr(localAddrStr)
	if err != nil {
		return fmt.Errorf("peer %s: invalid local ip: %w", name, err)
	}
	if err := ValidatePeerGlobalNextHop(name, "local ip", la); err != nil {
		return err
	}
	ps.LocalAddress = la
	return nil
}

// applyLinkLocal parses `session > link-local`, the address ze appends AFTER the
// global one when RFC 2545 Section 3's condition holds. The leaf supplies the
// address; it does not decide inclusion (link_scope.go).
func applyLinkLocal(ps *PeerSettings, name string, sessionMap map[string]any) error {
	if sessionMap == nil {
		return nil
	}
	v, ok := mapString(sessionMap, "link-local")
	if !ok {
		return nil
	}
	ll, err := netip.ParseAddr(v)
	if err != nil {
		return fmt.Errorf("peer %s: invalid link-local: %w", name, err)
	}
	ps.LinkLocal = ll
	return nil
}

// applyNextHopMode parses `session > next-hop`: one of the three rewriting modes
// of RFC 4271 Section 5.1.3, or an explicit address.
//
// An explicit address goes straight into the global next-hop slot, so RFC 2545
// Section 3 governs it exactly as it governs the local address.
func applyNextHopMode(ps *PeerSettings, name string, sessionMap map[string]any) error {
	if sessionMap == nil {
		return nil
	}
	nhVal, ok := mapString(sessionMap, "next-hop")
	if !ok {
		return nil
	}
	switch nhVal {
	case "auto", "": // empty treated as auto
		ps.NextHopMode = NextHopAuto
	case "self":
		ps.NextHopMode = NextHopSelf
	case "unchanged":
		ps.NextHopMode = NextHopUnchanged
	default: // must be an IP address
		nhAddr, err := netip.ParseAddr(nhVal)
		if err != nil {
			return fmt.Errorf("peer %s: invalid next-hop %q: expected auto, self, unchanged, or IP address", name, nhVal)
		}
		if err := ValidatePeerGlobalNextHop(name, "next-hop", nhAddr); err != nil {
			return err
		}
		ps.NextHopMode = NextHopExplicit
		ps.NextHopAddress = nhAddr
	}
	return nil
}
