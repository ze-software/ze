// Design: docs/architecture/config/syntax.md — peer transport and next-hop configuration
// RFC: rfc/short/rfc2545.md — Section 3, the first address of the Next Hop field is the global one
// Overview: config.go — parsePeerFromTree, the peer config tree parser
// Related: link_scope.go — Section 3's inclusion condition, decided against the host interface table

package reactor

import (
	"fmt"
	"net/netip"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/policychain"
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
// This is a TCP endpoint, which can be IPv6 link-local (RFC 2545 Section 4).
// Keep its zone for the socket. Next-hop self selects the connected endpoint,
// and the route's final next-hop form and negotiated capabilities decide whether
// that address can be advertised (resolveNextHop, peer.go; egressNextHopWithheld,
// forward_next_hop.go).
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

// staticExportNextHop answers the folded next-hop intent and true when p can
// take the zero-allocation static fast path without running an export policy
// chain per route: either p's compiled library chain holds ONLY built-in
// next-hop:* steps, or p carries no compiled library and no active filters at
// all (which preserves the RFC 4271 Section 5.1.3 default next-hop:auto for
// direct reactor callers that build PeerSettings without bgp/config).
//
// Templated dynamic policies and policies requiring the ingress stage never
// take the static fast path.
func staticExportNextHop(p filterapi.Policy) (policychain.NextHopIntent, bool) {
	if p.RequiresIngress() {
		return policychain.NextHopIntent{}, false
	}
	if p.Templated() {
		return policychain.NextHopIntent{}, false
	}
	if lib := p.Library(); lib != nil {
		chain, ok := lib.(*policychain.Chain)
		if !ok {
			return policychain.NextHopIntent{}, false
		}
		return chain.StaticNextHop()
	}
	for _, ref := range p.Filters() {
		if !ref.Inactive {
			return policychain.NextHopIntent{}, false
		}
	}
	return policychain.NextHopIntent{Mode: policychain.NextHopModeAuto, AutoDefault: true}, true
}
