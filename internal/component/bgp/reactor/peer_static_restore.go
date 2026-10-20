// Design: docs/architecture/exabgp-bridge.md -- a script that exits gives back the configured route it replaced
// Related: peer_static_wire.go -- sendStaticRoutes, the send this reuses
// Related: reactor_api_batch.go -- WithdrawNLRIBatch, the withdrawal that asks for it

package reactor

import (
	"bytes"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisplit"
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
// A configured route is matched by its family, ADD-PATH identifier, RD and
// prefix, never by its label (staticRouteNamesNLRI).
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

// staticRouteNamesNLRI says whether static route r and withdrawn NLRI n are
// the same route: one family, one ADD-PATH identifier, one route key. The key
// of n is the family's own (nlrisplit.GetPrefixKey, read as a withdrawal), and
// the key of r is built in the same form by staticRouteKey.
//
// RFC 8277 Section 2.4: "Upon reception, the value of the Compatibility field
// MUST be ignored." A withdrawn labeled or VPN NLRI carries that field where
// the announcement carried its label stack, so the label is not part of the
// key, and a configured route is named whatever label it was given.
func staticRouteNamesNLRI(r *StaticRoute, n nlri.NLRI) bool {
	fam := r.Family()
	if fam != n.Family() {
		return false
	}
	if r.PathID != n.PathID() {
		return false
	}
	var withdrawnScratch, configuredScratch [nlrisplit.PrefixKeyScratchSize]byte
	withdrawn, err := nlrisplit.GetPrefixKey(fam)(n.Bytes(), withdrawnScratch[:], true)
	if err != nil {
		// A withdrawn NLRI the family cannot frame names no configured route.
		return false
	}
	return bytes.Equal(withdrawn, staticRouteKey(r, configuredScratch[:]))
}

// staticRouteKey writes the route key of r into buf, in the form the family's
// prefix key answers for a withdrawal: the length in bits, then the RD of a
// VPN route, then the octets that hold the prefix (RFC 4271 Section 4.3), the
// trailing bits masked. buf must hold nlrisplit.PrefixKeyScratchSize octets.
//
// RFC 4364 Section 4.1: "The purpose of the RD is solely to allow one to
// create distinct routes to a common IPv4 address prefix." The RD is part of
// the key, so the same prefix under another RD is another route.
func staticRouteKey(r *StaticRoute, buf []byte) []byte {
	prefix := r.Prefix.Masked()
	bits := prefix.Bits()
	off := 1
	if r.IsVPN() {
		bits += 8 * len(r.RDBytes)
		off += copy(buf[off:], r.RDBytes[:])
	}
	buf[0] = byte(bits)
	addr := prefix.Addr().AsSlice()
	octets := (prefix.Bits() + 7) / 8 // Rounded up: a partial octet still travels.
	off += copy(buf[off:], addr[:octets])
	return buf[:off]
}
