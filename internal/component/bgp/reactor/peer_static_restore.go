// Design: docs/architecture/exabgp-bridge.md -- a script that exits gives back the configured route it replaced
// Related: peer_static_wire.go -- sendStaticRoutes, the send this reuses
// Related: reactor_api_batch.go -- WithdrawNLRIBatch, the withdrawal that asks for it

package reactor

import (
	"bytes"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
)

// restoreConfigured gives back the configured routes a withdrawal names, and
// answers the NLRIs that remain to withdraw.
//
// It is ExaBGP's withdraw_owner (rib/outgoing.py): "A route which replaced one
// from the configuration gives it back rather than leaving the prefix
// unannounced". A withdrawn NLRI that this peer's static config also announces
// is taken out of the withdrawal, and the configured route is sent in its
// place, where it replaces the route the withdrawal named.
//
// A connection that has not been sent its static set yet is sent nothing here:
// its initial send reads the same configuration and announces the route
// itself, and the withdrawal no longer carries the NLRI that would take it back.
//
// Only a route the config names by its prefix is matched. A static route with
// a route distinguisher or a label stack is not, and its NLRI is withdrawn as
// asked (staticRouteNamesNLRI).
func (p *Peer) restoreConfigured(withdrawn []nlri.NLRI) []nlri.NLRI {
	configured := p.staticRoutes()
	if len(configured) == 0 {
		return withdrawn
	}

	var give []StaticRoute
	var rest []nlri.NLRI
	for i, n := range withdrawn {
		route, found := configuredRouteFor(configured, n)
		if !found {
			if give != nil {
				rest = append(rest, n)
			}
			continue
		}
		if give == nil {
			rest = append(make([]nlri.NLRI, 0, len(withdrawn)), withdrawn[:i]...)
		}
		give = append(give, route)
	}
	if give == nil {
		return withdrawn
	}

	p.staticMu.Lock()
	defer p.staticMu.Unlock()

	session := p.currentSession()
	if session == nil {
		return rest
	}
	if p.staticWire.session != session {
		return rest
	}
	nc := p.negotiated.Load()
	if nc == nil {
		return rest
	}
	maxMsgSize := int(message.MaxMessageLength(msgtype.TypeUPDATE, nc.ExtendedMessage))

	// The marker every static send raises, for its reason: the route comes from
	// the configuration, so the RIB plugin must not store it in ribOut
	// (deliverStaticRouteDelta).
	restore := p.sendingConfigStatic.Swap(true)
	sent := p.sendStaticRoutes(session, give, p.settings.GroupUpdates, maxMsgSize, p.prefixSIDAllowed())
	p.sendingConfigStatic.Store(restore)

	routesLogger().Debug("configured routes given back in place of a withdrawal",
		"peer", p.addrString,
		"given-back", len(sent),
		"asked", len(give))
	return rest
}

// configuredRouteFor answers the static route that names n, if one does.
func configuredRouteFor(configured []StaticRoute, n nlri.NLRI) (StaticRoute, bool) {
	for i := range configured {
		if staticRouteNamesNLRI(&configured[i], n) {
			return configured[i], true
		}
	}
	return StaticRoute{}, false
}

// staticRouteNamesNLRI says whether static route r and NLRI n are the same
// route: one family, one ADD-PATH identifier, one prefix. The prefix is
// compared as RFC 4271 Section 4.3 encodes it, a length in bits followed by
// the octets that hold it, which is what n.Bytes() carries for a plain prefix.
//
// A route with a route distinguisher or a label stack carries those in its
// NLRI too, so it is never matched here.
func staticRouteNamesNLRI(r *StaticRoute, n nlri.NLRI) bool {
	if r.RD != "" {
		return false
	}
	if len(r.Labels) > 0 {
		return false
	}
	if r.Family() != n.Family() {
		return false
	}
	if r.PathID != n.PathID() {
		return false
	}
	prefix := r.Prefix.Masked()
	bits := prefix.Bits()
	wire := n.Bytes()
	if len(wire) == 0 {
		return false
	}
	if int(wire[0]) != bits {
		return false
	}
	addr := prefix.Addr().AsSlice()
	octets := (bits + 7) / 8 // Rounded up: a partial octet still travels.
	return len(wire) == 1+octets && bytes.Equal(wire[1:], addr[:octets])
}
