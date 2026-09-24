// Design: docs/architecture/core-design.md — config reload delivers changed peer settings
// Related: peer_settings_apply.go — hotSwappableSettings carries the derived feeds on a swap
// Related: reactor_dynamic.go — SetDynamicGroups delivers them to a running dynamic peer
// Related: delivery_graph.go — the peer-to-process index a delivered feed republishes
// Related: config.go — EnsureProcessBinding, the one writer of a derived binding
// RFC: rfc/short/rfc2918.md — ROUTE-REFRESH, how a new consumer learns what the peer sent
package reactor

import (
	"reflect"
	"slices"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// derivedFeedsOnly reports whether b is a DERIVED binding that grants the process
// no send type. Such a binding only feeds a consumer: the process can put nothing
// on this peer's wire through it, so the session it rides on has nothing to
// renegotiate when it comes or goes.
//
// The config builder adds one to every peer when the configuration needs a
// consumer fed with every peer's candidates, for example the bgp-rib grant for
// RFC 7311 AIGP selection (wireRedistributeDelivery,
// internal/component/bgp/config/redistribute_binding.go). Adding the first iBGP
// peer adds it to EVERY peer, so a restart for it would bounce sessions the
// operator did not touch.
func (b *ProcessBinding) derivedFeedsOnly() bool {
	return b.Derived && !b.SendAll && len(b.Send) == 0
}

// onlyDerivedFeedsDiffer reports whether current and next hold the same bindings,
// in the same order, once the derived feed-only bindings are set aside.
//
// Every other binding is the operator's attach block or a derived binding that
// may send, and a change to one of those still restarts the session.
//
// Each pass of the loop consumes one binding from each list, so it runs at most
// len(current)+len(next) times.
func onlyDerivedFeedsDiffer(current, next []ProcessBinding) bool {
	i, j := 0, 0
	for {
		for i < len(current) && current[i].derivedFeedsOnly() {
			i++
		}
		for j < len(next) && next[j].derivedFeedsOnly() {
			j++
		}
		if i == len(current) || j == len(next) {
			return i == len(current) && j == len(next)
		}
		if !reflect.DeepEqual(current[i], next[j]) {
			return false
		}
		i++
		j++
	}
}

// derivedFeedChange names the processes a derived feed-only binding starts and
// stops feeding when a peer moves from before to after. A process that keeps its
// feed with a different grant is in neither list: derivedFeedsDeliverable
// refuses that change, so it never reaches a delivery.
func derivedFeedChange(before, after []ProcessBinding) (gained, lost []string) {
	for i := range after {
		if after[i].derivedFeedsOnly() && !hasBinding(before, after[i].PluginName) {
			gained = append(gained, after[i].PluginName)
		}
	}
	for i := range before {
		if before[i].derivedFeedsOnly() && !hasBinding(after, before[i].PluginName) {
			lost = append(lost, before[i].PluginName)
		}
	}
	return gained, lost
}

// hasBinding reports whether bindings holds one for the process.
func hasBinding(bindings []ProcessBinding, process string) bool {
	return slices.ContainsFunc(bindings, func(b ProcessBinding) bool { return b.PluginName == process })
}

// feedCatchUp names how a consumer that starts being fed a running peer's
// routes learns the routes the peer sent before the feed began. The zero value
// is no answer, and every reader treats it as feedCatchUpImpossible.
type feedCatchUp uint8

const (
	feedCatchUpUnspecified feedCatchUp = iota
	// feedCatchUpNothing: the session has announced no route, so the new
	// consumer has nothing to learn.
	feedCatchUpNothing
	// feedCatchUpRouteRefresh: the peer advertised Route Refresh, so it
	// re-sends its routes on request (RFC 2918).
	feedCatchUpRouteRefresh
	// feedCatchUpImpossible: the peer announced routes and nothing can send
	// them again, so only a new session gives the consumer the whole picture.
	feedCatchUpImpossible
)

// derivedFeedsDeliverable reports whether a running session can take the change
// from current to next in place. The caller has already established that only
// derived feed-only bindings differ (onlyDerivedFeedsDiffer).
//
// A consumer that starts being fed must also learn the routes the peer sent
// before, or it holds half of that peer's routes. catchUp says how it can
// (Session.feedCatchUp): a session that received no route owes it nothing, and
// a peer that advertised Route Refresh re-sends them. Any other answer restarts.
//
// A feed that stays with a different grant is refused too. The consumer's
// picture of the old grant cannot be told apart from the new one, so the
// session restarts and every consumer starts again from nothing.
func derivedFeedsDeliverable(current, next []ProcessBinding, catchUp feedCatchUp) bool {
	for i := range next {
		if !next[i].derivedFeedsOnly() {
			continue
		}
		for k := range current {
			if current[k].PluginName != next[i].PluginName {
				continue
			}
			if !reflect.DeepEqual(current[k], next[i]) {
				return false
			}
		}
	}
	gained, _ := derivedFeedChange(current, next)
	if len(gained) == 0 {
		return true
	}
	switch catchUp {
	case feedCatchUpNothing, feedCatchUpRouteRefresh:
		return true
	case feedCatchUpUnspecified, feedCatchUpImpossible:
		return false
	}
	return false
}

// noteReceivedRoute records, once for each connection, that the peer announced
// a route (Session.receivedRoute). It runs on the read goroutine for every
// UPDATE that reaches the plugins, and costs one atomic load after the first
// announcement.
//
// An MP_REACH_NLRI counts whatever it carries: answering "received" for a
// message that held no route costs one catch-up, where the opposite answer
// would leave a consumer without routes.
func (s *Session) noteReceivedRoute(wu *wireu.WireUpdate) {
	if s.receivedRoute.Load() {
		return
	}
	// RFC 4271 Section 4.3: the NLRI field lists the IPv4 prefixes the
	// UPDATE announces.
	if nlri, err := wu.NLRI(); err == nil && len(nlri) > 0 {
		s.receivedRoute.Store(true)
		return
	}
	// RFC 4760 Section 3: MP_REACH_NLRI "is used to carry the set of
	// reachable destinations" for every other family.
	if mpReach, err := wu.MPReach(); err == nil && mpReach != nil {
		s.receivedRoute.Store(true)
	}
}

// feedCatchUp answers derivedFeedsDeliverable's question for a running session.
// No session, and a session that announced no route, owe a new consumer
// nothing. Otherwise the peer's Route Refresh capability decides.
func (s *Session) feedCatchUp() feedCatchUp {
	if s == nil {
		return feedCatchUpNothing
	}
	if !s.receivedRoute.Load() {
		return feedCatchUpNothing
	}
	s.mu.RLock()
	negotiated := s.negotiated
	s.mu.RUnlock()
	if negotiated == nil {
		// A route arrives only on an Established session, which has a
		// negotiation. Without one there is nothing to ask the peer with.
		return feedCatchUpImpossible
	}
	if negotiated.RouteRefresh {
		return feedCatchUpRouteRefresh
	}
	return feedCatchUpImpossible
}

// catchUpDerivedFeeds brings the consumers of a running peer's derived feeds in
// line after a reload delivered a binding change.
//
// The caller MUST have republished the delivery graph first and MUST NOT hold
// r.mu: the calls below can block on the peer's socket or on a plugin.
//
// A consumer that lost its feed is told the peer is gone from its view. The
// "down" goes to the named processes alone (EventDispatcher.OnPeerUnbound), so
// the consumers still fed keep this peer's routes, and the one that stopped
// being fed drops them rather than selecting over routes nobody updates.
//
// A consumer that gained its feed needs the routes the peer already sent. The
// catch-up is decided again here, after the republish, because the peer can
// have announced its first route since the swap plan asked. The read path marks
// a route received before it looks the consumers up in the graph, so a route
// this check does not see reaches the new consumer live.
func (p *Peer) catchUpDerivedFeeds(gained, lost []string) {
	if p.State() != PeerStateEstablished {
		return
	}

	p.mu.RLock()
	r := p.reactor
	p.mu.RUnlock()
	if len(lost) > 0 && r != nil && r.eventDispatcher != nil {
		info := establishedPeerInfo(p)
		r.eventDispatcher.OnPeerUnbound(&info, rpc.SessionStateDown, rpc.ReasonPeerRemoved, lost)
	}

	if len(gained) == 0 {
		return
	}
	switch p.currentSession().feedCatchUp() {
	case feedCatchUpNothing:
		return
	case feedCatchUpRouteRefresh:
		p.requestRouteRefreshAll(gained)
		return
	case feedCatchUpUnspecified, feedCatchUpImpossible:
	}
	// The session re-established, or announced its first route, between the swap
	// plan and this apply, and the peer cannot re-send. A new session is the one
	// way every consumer holds the same routes.
	// RFC 4486 Section 4: subcode 6 "Other Configuration Change".
	peerLogger().Warn("derived consumer bound without the peer's earlier routes: restarting the session",
		"peer", p.settings.Address, "processes", gained)
	if err := p.Teardown(message.NotifyCeaseOtherConfigChange, ""); err != nil {
		peerLogger().Warn("derived consumer catch-up: session restart not queued",
			"peer", p.settings.Address, "error", err)
	}
}

// requestRouteRefreshAll asks the peer to re-send its routes for every
// negotiated family, so the processes that gained a feed learn them. Every
// consumer still fed receives them a second time, which is an implicit
// replacement of routes it already holds (RFC 4271 Section 3.1).
func (p *Peer) requestRouteRefreshAll(gained []string) {
	neg := p.negotiated.Load()
	// RFC 2918 Section 4: "A BGP speaker may send a ROUTE-REFRESH message to
	// its peer only if it has received the Route Refresh Capability from its
	// peer." feedCatchUp answered feedCatchUpRouteRefresh from that capability.
	if neg == nil || !neg.RouteRefresh {
		return
	}
	// RFC 2918 Section 3: the ROUTE-REFRESH names one <AFI, SAFI>, and the peer
	// re-advertises its Adj-RIB-Out for that family, so each negotiated family
	// is asked for once.
	for _, fam := range neg.Families() {
		rr := &message.RouteRefresh{
			AFI:     fam.AFI,
			SAFI:    fam.SAFI,
			Subtype: message.RouteRefreshNormal,
		}
		if err := p.SendRawMessage(0, message.PackTo(rr, nil)); err != nil {
			peerLogger().Warn("derived consumer catch-up: route refresh not sent",
				"peer", p.settings.Address, "family", fam, "processes", gained, "error", err)
			return
		}
		p.incrRefreshSent()
	}
}
