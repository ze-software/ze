// Design: docs/architecture/rsvpte/mpls-rsvp-te.md -- configured explicit routing.
package rsvpte

import (
	"fmt"
	"net/netip"
)

type pathSelection struct {
	Route RouteInfo
	ERO []eroHop
	ErrorValue uint16
}

// resolveReceivedPath implements RFC 3209 Section 4.3.4.1 before either transit
// or egress acts on a PATH. An ERO names this node first, not merely some node
// reachable through it. Absent and present-but-empty EROs are different cases.
func (e *engine) resolveReceivedPath(msg *ParsedMessage) (pathSelection, error) {
	if !msg.HasERO {
		return e.resolveImplicitPath(msg.Session.TunnelEndpoint)
	}
	if len(msg.ERO) == 0 {
		return pathSelection{ErrorValue: ErrValueBadEROObject}, fmt.Errorf("empty explicit route")
	}
	if !e.isLocalPrefix(msg.ERO[0].Address) {
		return pathSelection{ErrorValue: ErrValueBadInitialSubobject}, fmt.Errorf("first explicit node %s does not contain this node", msg.ERO[0].Address)
	}
	return e.resolveExplicitPath(msg.ERO[0], msg.ERO[1:], msg.Session.TunnelEndpoint)
}

// An originator's configured list names the downstream nodes; its own node is
// the implicit preceding abstract node, not an extra configured requirement.
func (e *engine) resolveOriginatingPath(psb *pathStateBlock) (pathSelection, error) {
	return e.resolveExplicitPath(eroHop{Address: netip.PrefixFrom(e.cfg().RouterID, 32)}, psb.ERO, psb.Session.TunnelEndpoint)
}

func (e *engine) resolveImplicitPath(endpoint netip.Addr) (pathSelection, error) {
	if e.isLocalAddress(endpoint) {
		return pathSelection{}, nil
	}
	route, err := e.transport.ResolveRoute(netip.PrefixFrom(endpoint, endpoint.BitLen()), endpoint, 0)
	return pathSelection{Route: route, ErrorValue: ErrValueNoRouteAvailable}, err
}

func (e *engine) resolveExplicitPath(first eroHop, rest []eroHop, endpoint netip.Addr) (pathSelection, error) {
	// Sections 4.3.4.1 steps 2 and 3: repeated membership can consume more
	// than one subobject, including prefixes wider than a single address.
	for len(rest) > 0 && e.isLocalPrefix(rest[0].Address) {
		first, rest = rest[0], rest[1:]
	}
	if len(rest) == 0 {
		return e.resolveImplicitPath(endpoint)
	}
	target := rest[0]
	code := ErrValueBadStrictNode
	if target.Loose {
		code = ErrValueBadLooseNode
	}
	route, err := e.transport.ResolveRoute(target.Address, endpoint, 0)
	if err != nil {
		return pathSelection{ErrorValue: code}, err
	}
	if e.isLocalAddress(route.NextHop) {
		return pathSelection{ErrorValue: code}, fmt.Errorf("explicit route resolves back to this node")
	}
	if e.peerInPrefix(route.NextHop, target.Address) {
		// Step 4: the next node is a member of the second abstract node.
		return pathSelection{Route: route, ERO: rest}, nil
	}
	inside := e.peerInPrefix(route.NextHop, first.Address)
	if !inside && !target.Loose {
		return pathSelection{ErrorValue: code}, fmt.Errorf("strict node %s is not adjacent through %s", target.Address, first.Address)
	}
	// Steps 5 and 6: stay inside the current abstract node for a strict
	// transition, or follow native routing toward a loose node. In the latter
	// case replace the current subobject with the actual adjacent next hop.
	// This is local next-hop resolution, not TE path computation or CSPF.
	if !inside {
		first = eroHop{Address: netip.PrefixFrom(route.NextHop, 32)}
	}
	if len(rest) >= maxExplicitRouteHops {
		return pathSelection{ErrorValue: ErrValueBadEROObject}, fmt.Errorf("expanded explicit route exceeds %d hops", maxExplicitRouteHops)
	}
	forward := make([]eroHop, len(rest)+1)
	forward[0] = first
	copy(forward[1:], rest)
	return pathSelection{Route: route, ERO: forward}, nil
}
