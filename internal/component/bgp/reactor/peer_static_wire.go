// RFC: rfc/short/rfc4271.md -- Section 4.3, a withdrawn route is identified in the context of the connection it was advertised on
// Design: docs/architecture/core-design.md -- BGP peer reload: swap or restart
// Overview: peer_initial_sync.go -- the establishment-time send of the same set
// Related: peer_settings_apply.go -- the reload swap that calls deliverStaticRouteDelta
package reactor

import (
	"encoding/binary"
	"reflect"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
)

// staticWireSet is the static route set one connection has been sent.
//
// The session pointer is half of the value rather than a detail: RFC 4271
// Section 4.3 scopes "previously advertised" to one BGP speaker connection, so a
// set recorded on a connection that has gone says nothing about the one running
// now. Comparing the pointer is what retires the record, and it needs no hook on
// teardown to do it.
type staticWireSet struct {
	session *Session
	routes  []StaticRoute
}

// sendStaticRoutes writes routes to this peer and answers the ones that reached
// the wire, in the order they were sent.
//
// The answer is what the caller records as this connection's static set
// (Peer.staticWire), so it holds only the routes a peer really received: a route
// whose next hop could not be resolved, and a route whose build or write failed,
// are both left out. Recording the configured set instead would have a later
// reload withdraw a route the peer never held, and suppress the re-announcement
// of one it never got.
//
// group is the peer's group-updates setting. It is read by the caller rather than
// here because it is immutable on a running peer -- a change to it forces a
// session restart (hotSwappableSettings, peer_settings_apply.go).
//
// The returned slice is one allocation per call. Both callers run once per
// session establishment or once per reload, so this is a control-plane cost and
// not a per-UPDATE one (ai/rules/performance.md).
func (p *Peer) sendStaticRoutes(routes []StaticRoute, group bool, maxMsgSize int, prefixSIDAllowed bool) []StaticRoute {
	if len(routes) == 0 {
		return nil
	}
	if group {
		return p.sendStaticRoutesGrouped(routes, maxMsgSize, prefixSIDAllowed)
	}

	addr := p.addrString
	sent := make([]StaticRoute, 0, len(routes))
	for i := range routes {
		route := &routes[i]
		fam := routeFamily(route)
		// Resolve next-hop from RouteNextHop policy.
		nextHop, nhErr := p.resolveNextHop(route.NextHop, fam)
		if nhErr != nil {
			routesLogger().Debug("next-hop resolution failed", "peer", addr, "prefix", route.Prefix, "error", nhErr)
			continue
		}
		addPath := p.addPathFor(fam)
		ub := message.GetUpdateBuilder(p.settings.LocalAS, p.IsIBGP(), p.asn4(), addPath)
		update := buildStaticRouteUpdateNew(ub, route, nextHop, p.linkLocalNextHopFor(nextHop), p.sendCtx.Load(), prefixSIDAllowed)
		err := p.sendUpdateWithSplit(update, maxMsgSize, addPath)
		message.PutUpdateBuilder(ub)
		if err != nil {
			routesLogger().Debug("send error", "peer", addr, "error", err)
			return sent
		}
		sent = append(sent, *route)
		routesLogger().Debug("route sent", "peer", addr, "prefix", route.Prefix.String(), "nextHop", route.NextHop.String())
	}
	return sent
}

// sendStaticRoutesGrouped is sendStaticRoutes for a peer that asks for grouping:
// routes with identical attributes travel in one UPDATE.
//
// A multi-route group is all or nothing. BuildGroupedUnicast hands the whole
// group to one send, so a failure says nothing about which of its prefixes
// reached the peer, and the safe reading of "unknown" is that none did: the
// group is then left out of the answer, and a later reload announces it again
// rather than suppressing a route the peer may not hold.
func (p *Peer) sendStaticRoutesGrouped(routes []StaticRoute, maxMsgSize int, prefixSIDAllowed bool) []StaticRoute {
	addr := p.addrString
	sent := make([]StaticRoute, 0, len(routes))

	for _, grouped := range groupRoutesByAttributes(routes) {
		addPath := p.addPathFor(routeFamily(&grouped[0]))
		if len(grouped) == 1 {
			// Single-route group (IPv6, VPN, LabeledUnicast, or solo IPv4).
			nextHop, nhErr := p.resolveNextHop(grouped[0].NextHop, routeFamily(&grouped[0]))
			if nhErr != nil {
				routesLogger().Debug("next-hop resolution failed", "peer", addr, "error", nhErr)
				continue
			}
			ub := message.GetUpdateBuilder(p.settings.LocalAS, p.IsIBGP(), p.asn4(), addPath)
			update := buildStaticRouteUpdateNew(ub, &grouped[0], nextHop, p.linkLocalNextHopFor(nextHop), p.sendCtx.Load(), prefixSIDAllowed)
			err := p.sendUpdateWithSplit(update, maxMsgSize, addPath)
			message.PutUpdateBuilder(ub)
			if err != nil {
				routesLogger().Debug("send error", "peer", addr, "error", err)
				return sent
			}
			sent = append(sent, grouped[0])
			routesLogger().Debug("route sent", "peer", addr, "prefix", grouped[0].Prefix.String(), "nextHop", grouped[0].NextHop.String())
			continue
		}

		// Multi-route group -- IPv4 unicast only (routeGroupKey ensures this).
		// The size-aware builder respects the peer's maximum message size.
		ub := message.GetUpdateBuilder(p.settings.LocalAS, p.IsIBGP(), p.asn4(), addPath)
		params := make([]message.UnicastParams, 0, len(grouped))
		included := make([]StaticRoute, 0, len(grouped))
		for i := range grouped {
			r := &grouped[i]
			nextHop, nhErr := p.resolveNextHop(r.NextHop, routeFamily(r))
			if nhErr != nil {
				routesLogger().Debug("next-hop resolution failed", "peer", addr, "prefix", r.Prefix, "error", nhErr)
				continue
			}
			params = append(params, toStaticRouteUnicastParams(r, nextHop, p.linkLocalNextHopFor(nextHop), p.sendCtx.Load(), prefixSIDAllowed))
			included = append(included, *r)
		}
		if len(params) == 0 {
			message.PutUpdateBuilder(ub)
			continue
		}
		err := ub.BuildGroupedUnicast(params, maxMsgSize, p.SendUpdate)
		message.PutUpdateBuilder(ub)
		if err != nil {
			routesLogger().Debug("grouped unicast error", "peer", addr, "error", err)
			return sent
		}
		sent = append(sent, included...)
		for i := range included {
			routesLogger().Debug("route sent", "peer", addr, "prefix", included[i].Prefix.String(), "nextHop", included[i].NextHop.String())
		}
	}
	return sent
}

// deliverStaticRouteDelta moves a running session from the static route set it
// holds to the set wanted, by announcing what it does not have and withdrawing
// what the configuration no longer names.
//
// It is the reason StaticRoutes is a hot-swappable field: without it the new set
// would sit on the peer and reach no wire until the next connection, which is the
// silent-discard failure hotSwappableSettings exists to prevent
// (peer_settings_apply.go).
//
// Called from applyHotSwappableSettings AFTER p.mu is released, on the config
// reload goroutine. p.staticMu is what makes that safe: the establishment
// goroutine holds the same lock across "read the set, send it, record it"
// (sendInitialRoutes), so the two never interleave on one connection and a
// withdrawal can never precede the announcement it takes back.
//
// A connection that has not been sent its static set yet is left alone. Its
// initial send reads the settings under p.mu after this function's caller wrote
// them, so it sends the new set itself, and a delta as well would send every
// route twice.
func (p *Peer) deliverStaticRouteDelta(wanted []StaticRoute) {
	p.staticMu.Lock()
	defer p.staticMu.Unlock()

	session := p.currentSession()
	if session == nil || p.staticWire.session != session {
		return
	}

	announce, withdraw := staticRouteDelta(p.staticWire.routes, wanted)
	if len(announce) == 0 && len(withdraw) == 0 {
		return
	}

	// A peer with no negotiated capabilities has no established session to
	// deliver on. Answering rather than assuming a default keeps this path from
	// building an UPDATE against an encoding nobody agreed.
	nc := p.negotiated.Load()
	if nc == nil {
		return
	}
	maxMsgSize := int(message.MaxMessageLength(msgtype.TypeUPDATE, nc.ExtendedMessage))

	// RFC 8669 Section 8: "The propagation to other ASes MUST be explicitly
	// configured." One answer for this whole delta, because it is a property of
	// the session rather than of a route (Peer.prefixSIDAllowed,
	// forward_prefix_sid.go).
	prefixSIDAllowed := p.prefixSIDAllowed()

	// The same marker the initial send raises, for the same reason: these routes
	// come from the configuration and are re-sent from it on every reconnection,
	// so the RIB plugin must not store them in ribOut (handleSentStructured,
	// plugins/rib/rib_structured.go). The previous value is restored rather than
	// cleared, because the initial send's own span can still be open: it clears
	// the marker after its default-originate routes, which is outside staticMu.
	restore := p.sendingConfigStatic.Swap(true)
	p.withdrawStaticRoutes(withdraw, maxMsgSize, prefixSIDAllowed)
	sent := p.sendStaticRoutes(announce, p.settings.GroupUpdates, maxMsgSize, prefixSIDAllowed)
	p.sendingConfigStatic.Store(restore)

	p.staticWire.routes = staticWireAfterDelta(wanted, announce, sent)

	routesLogger().Debug("static routes delivered on the running session",
		"peer", p.addrString,
		"announced", len(sent),
		"withdrawn", len(withdraw),
		"held", len(p.staticWire.routes))
}

// withdrawStaticRoutes takes routes back from a running session.
//
// The withdrawal is built from the ANNOUNCEMENT the peer received, and then
// rewritten with message.SynthesizeWithdraw: the NLRI moves into the Withdrawn
// Routes field for IPv4 unicast, and MP_REACH_NLRI becomes MP_UNREACH_NLRI for
// every other family (RFC 4760 Section 4). That is what makes one code path serve
// a VPN route, a labeled-unicast route and a plain prefix alike, without this
// package naming an NLRI type that lives in a plugin.
//
// RFC 4271 Section 4.3 identifies a withdrawn route "in the context of the BGP
// speaker - BGP speaker connection to which it has been previously advertised",
// so a connection that has advertised nothing is written nothing, exactly as the
// API rail decides it (withdrawBatchFromPeers, reactor_api_batch.go). The caller
// only ever passes routes taken from Peer.staticWire, which is what this
// connection was sent, so the guard fires for a connection whose initial send
// reached no socket at all.
func (p *Peer) withdrawStaticRoutes(routes []StaticRoute, maxMsgSize int, prefixSIDAllowed bool) {
	if len(routes) == 0 {
		return
	}
	if !p.hasAdvertised() {
		routesLogger().Warn("withdrawal withheld: this session has advertised no route to the peer",
			"peer", p.addrString,
			"routes", len(routes),
			"rfc", "RFC 4271 Section 4.3")
		return
	}

	handle := getBuildBuf()
	defer putBuildBuf(handle)

	for i := range routes {
		route := &routes[i]
		fam := routeFamily(route)
		nextHop, nhErr := p.resolveNextHop(route.NextHop, fam)
		if nhErr != nil {
			routesLogger().Warn("static route not withdrawn: its next hop no longer resolves",
				"peer", p.addrString, "prefix", route.Prefix, "error", nhErr)
			continue
		}
		addPath := p.addPathFor(fam)

		ub := message.GetUpdateBuilder(p.settings.LocalAS, p.IsIBGP(), p.asn4(), addPath)
		update := buildStaticRouteUpdateNew(ub, route, nextHop, p.linkLocalNextHopFor(nextHop), p.sendCtx.Load(), prefixSIDAllowed)
		body, ok := updateBody(handle.Buf, update)
		message.PutUpdateBuilder(ub)
		if !ok {
			routesLogger().Warn("static route not withdrawn: its announcement does not fit the build buffer",
				"peer", p.addrString, "prefix", route.Prefix)
			continue
		}
		withdrawn, changed := message.SynthesizeWithdraw(body)
		if !changed {
			routesLogger().Warn("static route not withdrawn: its announcement carries no reachable route",
				"peer", p.addrString, "prefix", route.Prefix)
			continue
		}
		if err := p.sendBodyWithSplit(withdrawn, maxMsgSize, addPath); err != nil {
			routesLogger().Debug("withdraw send error", "peer", p.addrString, "prefix", route.Prefix, "error", err)
			return
		}
		routesLogger().Debug("route withdrawn", "peer", p.addrString, "prefix", route.Prefix.String())
	}
}

// updateBody lays a built UPDATE out as one RFC 4271 Section 4.3 message body:
// the withdrawn-routes length and field, the path-attributes length and field,
// and the NLRI. It answers false when buf cannot hold the result, which is the
// caller's signal that nothing was written.
//
// The header is left off because the two readers of the answer, SynthesizeWithdraw
// and ParseUpdateSections, both take a body.
func updateBody(buf []byte, update *message.Update) ([]byte, bool) {
	if update == nil {
		return nil, false
	}
	total := 2 + len(update.WithdrawnRoutes) + 2 + len(update.PathAttributes) + len(update.NLRI)
	if total > len(buf) || len(update.WithdrawnRoutes) > 0xFFFF || len(update.PathAttributes) > 0xFFFF {
		return nil, false
	}

	//nolint:gosec // G115: both lengths are bounded above.
	binary.BigEndian.PutUint16(buf[0:2], uint16(len(update.WithdrawnRoutes)))
	n := 2
	n += copy(buf[n:], update.WithdrawnRoutes)
	//nolint:gosec // G115: both lengths are bounded above.
	binary.BigEndian.PutUint16(buf[n:n+2], uint16(len(update.PathAttributes)))
	n += 2
	n += copy(buf[n:], update.PathAttributes)
	n += copy(buf[n:], update.NLRI)
	return buf[:n], true
}

// staticRouteDelta answers what a session holding held must be sent to hold
// wanted instead: the routes to announce, and the routes to withdraw.
//
// Identity is StaticRoute.RouteKey, which is prefix, route distinguisher and path
// identifier -- what the peer keys its own table on. A route whose key stays and
// whose attributes change is announced and NOT withdrawn: RFC 4271 Section 3.1
// makes a second advertisement of one destination replace the first, so a
// withdrawal beside it would take the new route away again.
func staticRouteDelta(held, wanted []StaticRoute) (announce, withdraw []StaticRoute) {
	wantedByKey := make(map[string]int, len(wanted))
	for i := range wanted {
		wantedByKey[wanted[i].RouteKey()] = i
	}
	heldByKey := make(map[string]int, len(held))
	for i := range held {
		heldByKey[held[i].RouteKey()] = i
	}

	for i := range held {
		if _, keeps := wantedByKey[held[i].RouteKey()]; !keeps {
			withdraw = append(withdraw, held[i])
		}
	}
	for i := range wanted {
		at, has := heldByKey[wanted[i].RouteKey()]
		if has && reflect.DeepEqual(held[at], wanted[i]) {
			continue
		}
		announce = append(announce, wanted[i])
	}
	return announce, withdraw
}

// staticWireAfterDelta is the set the connection holds once a delta has been
// delivered: every wanted route, minus the ones the announce failed to send.
//
// A route the announce did not reach the wire with is left out so a later reload
// announces it again. A route the withdrawal failed on is left out too, because
// it is not in wanted: the peer may still hold it, and re-announcing a route it
// has costs one UPDATE where the other reading blackholes it (adj_rib_out.go,
// forget).
func staticWireAfterDelta(wanted, announce, sent []StaticRoute) []StaticRoute {
	if len(announce) == len(sent) {
		return wanted
	}

	sentKeys := make(map[string]struct{}, len(sent))
	for i := range sent {
		sentKeys[sent[i].RouteKey()] = struct{}{}
	}
	announced := make(map[string]struct{}, len(announce))
	for i := range announce {
		announced[announce[i].RouteKey()] = struct{}{}
	}

	after := make([]StaticRoute, 0, len(wanted))
	for i := range wanted {
		key := wanted[i].RouteKey()
		if _, owed := announced[key]; owed {
			if _, ok := sentKeys[key]; !ok {
				continue
			}
		}
		after = append(after, wanted[i])
	}
	return after
}

// sameStaticRouteSet reports whether two headers name the same routes without
// comparing them: every writer of PeerSettings.StaticRoutes REPLACES the slice
// and none mutates the backing array, so one header is one set.
//
// It is how applyHotSwappableSettings knows whether the copier it was given
// delivered this field. Reading the answer off the write itself is what keeps a
// field from being classified swappable and then not delivered, which is the
// invariant hotSwappableSettings guards (peer_settings_apply.go).
func sameStaticRouteSet(a, b []StaticRoute) bool {
	if len(a) != len(b) {
		return false
	}
	if len(a) == 0 {
		return true
	}
	return &a[0] == &b[0]
}
