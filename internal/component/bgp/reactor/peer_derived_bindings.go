// Design: docs/architecture/core-design.md — config reload delivers changed peer settings
// Related: peer_settings_apply.go — hotSwappableSettings carries feed-only bus changes, applyHotSwappableSettings delivers them in order
// Related: reactor_dynamic.go — reconcileDynamicGroupsJournaled swaps a dynamic peer's bus block through the same ordered delivery
// Related: delivery_graph.go — the peer-to-process index a delivered bus block republishes
// Related: config.go — parsePeerBusFromTree, the parser of a peer's bus filter block
// RFC: rfc/short/rfc2918.md — ROUTE-REFRESH, how a new consumer learns what the peer sent
package reactor

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/component/bgp/format"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	bgpevents "github.com/ze-software/ze/internal/core/bgp/events"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

const (
	bgpMsgOpen       = bgpevents.Namespace + ":" + bgpevents.EventOpen
	bgpMsgState      = bgpevents.Namespace + ":" + bgpevents.EventState
	bgpMsgNegotiated = bgpevents.Namespace + ":" + bgpevents.EventNegotiated
	bgpMsgPrefix     = bgpevents.Namespace + ":"
)

// peerBindEvents is what the ordered bind delivery tells the plugins when a
// reload attaches processes to, or detaches them from, a session that stays up.
// *bgpserver.EventDispatcher satisfies it, and Reactor.bindEvents holds it as
// this interface so a test can observe each event against the delivery graph
// and the wire.
//
// OnPeerBound MUST return only once every named process acknowledged its up:
// the caller publishes the processes' delivery edges after it returns, and that
// order is what keeps a message of the session from reaching a process before
// its up. OnPeerUnbound is called after the edges are gone.
type peerBindEvents interface {
	OnPeerBound(peer *plugin.PeerInfo, neg any, stateProcesses, negotiatedProcesses []string)
	OnPeerUnbound(peer *plugin.PeerInfo, state rpc.SessionState, reason string, processes []string)
}

// routeFreeReceiveTypes are the event types that carry no route. A process
// fed only these from a running session has nothing to catch up on.
// Every other type, a plugin-registered one included, counts as route-bearing:
// answering "carries routes" for a type that does not costs one catch-up, where
// the opposite answer would leave a consumer without routes. The OPEN is not
// listed because a bus change touching bgp:open is never hot-swappable.
var routeFreeReceiveTypes = map[string]bool{
	bgpevents.EventState:        true,
	bgpevents.EventNegotiated:   true,
	bgpevents.EventKeepalive:    true,
	bgpevents.EventNotification: true,
	bgpevents.EventRefresh:      true,
}

// endpointFeedSpec summarizes the messages and directions one peer's bus send
// block feeds to a specific endpoint.
type endpointFeedSpec struct {
	allMsgs bool
	msgs    map[string]events.Direction
}

func (f endpointFeedSpec) equal(other endpointFeedSpec) bool {
	if f.allMsgs != other.allMsgs {
		return false
	}
	return maps.Equal(f.msgs, other.msgs)
}

func (f endpointFeedSpec) receivesState() bool {
	if f.allMsgs {
		return true
	}
	return f.msgs[bgpevents.EventState] != events.DirUnspecified
}

func (f endpointFeedSpec) receivesNegotiated() bool {
	if f.allMsgs {
		return true
	}
	return f.msgs[bgpevents.EventNegotiated] != events.DirUnspecified
}

func (f endpointFeedSpec) routeDirections() (received, sent bool) {
	if f.allMsgs {
		return true, true
	}
	for eventType, dir := range f.msgs {
		if routeFreeReceiveTypes[eventType] {
			continue
		}
		received = received || dir&events.DirReceived != 0
		sent = sent || dir&events.DirSent != 0
	}
	return received, sent
}

// busSendOpenSpec returns the effective bgp:open feed rule of a peer's bus send
// block so onlyReceiveOnlyBusDiffers can detect any OPEN delivery change.
func busSendOpenSpec(d plugin.BusDirectionBlock) (allowed, targetsNone bool, targets []string, tags map[string][]string) {
	if d.AllowPresent {
		if d.AllowNone {
			return false, false, nil, nil
		}
		if !slices.Contains(d.Allow, bgpMsgOpen) {
			return false, false, nil, nil
		}
	}
	if rule := d.Messages[bgpMsgOpen]; rule != nil {
		tags = rule.Tags
		if rule.TargetsPresent {
			return true, rule.TargetsNone, rule.Targets, tags
		}
	}
	if d.TargetsPresent {
		return true, d.TargetsNone, d.Targets, tags
	}
	return true, false, nil, tags
}

func tagsEqual(a, b map[string][]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, va := range a {
		vb, ok := b[k]
		if !ok {
			return false
		}
		if !plugin.StringSetEqual(va, vb) {
			return false
		}
	}
	return true
}

func busTargetsCover(targets []string, epPath string) bool {
	for _, target := range targets {
		if plugin.PathCovers(target, epPath) {
			return true
		}
	}
	return false
}

// busReceivePermitsEndpoint reports whether a peer's Bus.Receive block permits
// endpointName (at /bus/endpoint/<name>) to send at least one message to the peer.
func busReceivePermitsEndpoint(d plugin.BusDirectionBlock, endpointName string) bool {
	if d.AllowPresent && d.AllowNone {
		return false
	}
	epPath := plugin.EndpointPath(endpointName)
	for _, rule := range d.Messages {
		if rule == nil {
			continue
		}
		if rule.TargetsPresent {
			if !rule.TargetsNone && busTargetsCover(rule.Targets, epPath) {
				return true
			}
			continue
		}
		if !d.TargetsPresent {
			return true
		}
		if !d.TargetsNone && busTargetsCover(d.Targets, epPath) {
			return true
		}
	}
	if !d.TargetsPresent {
		return true
	}
	if d.TargetsNone {
		return false
	}
	return busTargetsCover(d.Targets, epPath)
}

// onlyReceiveOnlyBusDiffers reports whether current and next differ only in
// non-OPEN peer-to-endpoint feed rules (Bus.Send) for endpoints that hold no
// send grant toward this peer. Any change to Bus.Receive, to bgp:open delivery,
// or to the feed of an endpoint that holds a send grant forces a session restart.
func onlyReceiveOnlyBusDiffers(current, next plugin.BusBlock) bool {
	if !plugin.BusDirectionBlockEqual(current.Receive, next.Receive) {
		return false
	}
	if plugin.BusDirectionBlockEqual(current.Send, next.Send) {
		return true
	}
	cOpen, cNone, cTargets, cTags := busSendOpenSpec(current.Send)
	nOpen, nNone, nTargets, nTags := busSendOpenSpec(next.Send)
	if cOpen != nOpen {
		return false
	}
	if cOpen {
		if cNone != nNone {
			return false
		}
		if !plugin.StringSetEqual(cTargets, nTargets) {
			return false
		}
		if !tagsEqual(cTags, nTags) {
			return false
		}
	}
	curFeeds, curWild := busSendEndpointFeeds(current.Send)
	nextFeeds, nextWild := busSendEndpointFeeds(next.Send)
	if curWild != nextWild {
		if !(current.Receive.AllowPresent && current.Receive.AllowNone) &&
			!(current.Receive.TargetsPresent && current.Receive.TargetsNone && len(current.Receive.Messages) == 0) {
			return false
		}
	}
	for ep, cf := range curFeeds {
		if nf, ok := nextFeeds[ep]; !ok || !cf.equal(nf) {
			if busReceivePermitsEndpoint(current.Receive, ep) {
				return false
			}
		}
	}
	for ep := range nextFeeds {
		if _, ok := curFeeds[ep]; !ok {
			if busReceivePermitsEndpoint(current.Receive, ep) {
				return false
			}
		}
	}
	return true
}

func directionFromRule(rule *plugin.BusMessageRule) events.Direction {
	if rule == nil {
		return events.DirBoth
	}
	vals := rule.Tags[events.TagDirection]
	if len(vals) == 0 {
		return events.DirBoth
	}
	var dir events.Direction
	for _, v := range vals {
		switch v {
		case events.DirectionReceived:
			dir |= events.DirReceived
		case events.DirectionSent:
			dir |= events.DirSent
		}
	}
	return dir
}

func bareBGPEventName(msg string) string {
	return strings.TrimPrefix(msg, bgpMsgPrefix)
}

func endpointNameFromBusPath(path string) (string, bool) {
	ep, ok := strings.CutPrefix(path, plugin.BusEndpointPathPrefix)
	if !ok {
		return "", false
	}
	if ep == "" {
		return "", false
	}
	if strings.Contains(ep, "/") {
		return "", false
	}
	return ep, true
}

// busSendEndpointFeeds projects a peer's bus send block into per-endpoint feed
// specs for every exact /bus/endpoint/<name> target named in the block, and
// reports whether the block also covers wildcard or prefix targets.
func busSendEndpointFeeds(d plugin.BusDirectionBlock) (feeds map[string]endpointFeedSpec, wildcard bool) {
	if d.AllowPresent && d.AllowNone {
		return nil, false
	}
	feeds = make(map[string]endpointFeedSpec)
	addMsg := func(ep, bareMsg string, dir events.Direction) {
		if dir == events.DirUnspecified {
			return
		}
		spec := feeds[ep]
		if spec.msgs == nil {
			spec.msgs = make(map[string]events.Direction)
		}
		spec.msgs[bareMsg] |= dir
		feeds[ep] = spec
	}
	addAll := func(ep string) {
		spec := feeds[ep]
		spec.allMsgs = true
		feeds[ep] = spec
	}

	if d.AllowPresent {
		for _, msg := range d.Allow {
			bare := bareBGPEventName(msg)
			rule := d.Messages[msg]
			dir := directionFromRule(rule)
			targetsPresent := d.TargetsPresent
			targetsNone := d.TargetsNone
			targets := d.Targets
			if rule != nil && rule.TargetsPresent {
				targetsPresent = true
				targetsNone = rule.TargetsNone
				targets = rule.Targets
			}
			if targetsNone {
				continue
			}
			if !targetsPresent {
				wildcard = true
				continue
			}
			for _, tp := range targets {
				if ep, ok := endpointNameFromBusPath(tp); ok {
					addMsg(ep, bare, dir)
				} else {
					wildcard = true
				}
			}
		}
		return feeds, wildcard
	}

	// !AllowPresent: all declared messages allowed unless narrowed per message.
	if d.TargetsPresent {
		if !d.TargetsNone {
			for _, tp := range d.Targets {
				if ep, ok := endpointNameFromBusPath(tp); ok {
					addAll(ep)
				} else {
					wildcard = true
				}
			}
		}
	} else {
		wildcard = true
	}
	for msg, rule := range d.Messages {
		if rule == nil {
			continue
		}
		if !rule.TargetsPresent {
			continue
		}
		if rule.TargetsNone {
			continue
		}
		bare := bareBGPEventName(msg)
		dir := directionFromRule(rule)
		for _, tp := range rule.Targets {
			if ep, ok := endpointNameFromBusPath(tp); ok {
				addMsg(ep, bare, dir)
			} else {
				wildcard = true
			}
		}
	}
	return feeds, wildcard
}

// busFeedChange returns the sorted endpoint names that gained or lost (or
// changed) their peer-to-endpoint feed when moving from before to after.
// An endpoint whose feed spec changed appears in both lists (rebound).
func busFeedChange(before, after plugin.BusBlock) (gained, lost []string) {
	beforeFeeds, _ := busSendEndpointFeeds(before.Send)
	afterFeeds, _ := busSendEndpointFeeds(after.Send)

	for _, name := range slices.Sorted(maps.Keys(afterFeeds)) {
		bSpec, existed := beforeFeeds[name]
		if !existed || !bSpec.equal(afterFeeds[name]) {
			gained = append(gained, name)
		}
	}
	for _, name := range slices.Sorted(maps.Keys(beforeFeeds)) {
		aSpec, exists := afterFeeds[name]
		if !exists || !aSpec.equal(beforeFeeds[name]) {
			lost = append(lost, name)
		}
	}
	return gained, lost
}

// withoutBusEndpoints returns a copy of blk with every /bus/endpoint/<name>
// target listed in names removed from blk.Send (used for the intermediate
// unbind step of a rebound endpoint).
func withoutBusEndpoints(blk plugin.BusBlock, names []string) plugin.BusBlock {
	out := blk.Clone()
	if len(names) == 0 {
		return out
	}
	filterTargets := func(targets []string) ([]string, bool) {
		kept := make([]string, 0, len(targets))
		for _, tp := range targets {
			if ep, ok := endpointNameFromBusPath(tp); ok && slices.Contains(names, ep) {
				continue
			}
			kept = append(kept, tp)
		}
		return kept, len(kept) == 0
	}
	if out.Send.TargetsPresent && !out.Send.TargetsNone {
		kept, empty := filterTargets(out.Send.Targets)
		out.Send.Targets = kept
		if empty {
			out.Send.TargetsNone = true
		}
	}
	for _, rule := range out.Send.Messages {
		if rule == nil {
			continue
		}
		if !rule.TargetsPresent {
			continue
		}
		if rule.TargetsNone {
			continue
		}
		kept, empty := filterTargets(rule.Targets)
		rule.Targets = kept
		if empty {
			rule.TargetsNone = true
		}
	}
	return out
}

// feedCatchUp names how a consumer that starts being fed a running peer's
// routes learns the routes the session carried before the feed began. The zero
// value is no answer, and every reader treats it as feedCatchUpImpossible.
type feedCatchUp uint8

const (
	feedCatchUpUnspecified feedCatchUp = iota
	// feedCatchUpNothing: the grant carries no route, or the session carried
	// none in the direction the grant covers, so the new consumer has nothing
	// to learn.
	feedCatchUpNothing
	// feedCatchUpRouteRefresh: the peer advertised Route Refresh, so it
	// re-sends its routes on request (RFC 2918).
	feedCatchUpRouteRefresh
	// feedCatchUpImpossible: the session carried routes the consumer is owed
	// and nothing can send them again, so only a new session gives the
	// consumer the whole picture.
	feedCatchUpImpossible
)

// busFeedDeliverable reports whether a running session can take the feed-only
// bus change from current to next in place. The caller has already established
// that only non-OPEN peer-to-endpoint feeds differ (onlyReceiveOnlyBusDiffers).
func busFeedDeliverable(current, next plugin.BusBlock, s *Session) bool {
	if plugin.BusDirectionBlockEqual(current.Send, next.Send) {
		return true
	}
	_, beforeWild := busSendEndpointFeeds(current.Send)
	afterFeeds, afterWild := busSendEndpointFeeds(next.Send)
	if afterWild && !beforeWild {
		switch s.feedDirectionsCatchUp(true, true) {
		case feedCatchUpNothing, feedCatchUpRouteRefresh:
		case feedCatchUpUnspecified, feedCatchUpImpossible:
			return false
		default:
			panic("BUG: invalid bus feed catch-up outcome")
		}
	}
	gained, _ := busFeedChange(current, next)
	for _, name := range gained {
		received, sent := afterFeeds[name].routeDirections()
		switch s.feedDirectionsCatchUp(received, sent) {
		case feedCatchUpNothing, feedCatchUpRouteRefresh:
		case feedCatchUpUnspecified, feedCatchUpImpossible:
			return false
		default:
			panic("BUG: invalid bus feed catch-up outcome")
		}
	}
	return true
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

// feedCatchUp answers whether session s can re-deliver routes the peer already
// sent. No session, and a session that announced no route, owe a new consumer
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

// feedDirectionsCatchUp says how a newly attached consumer covering received
// and/or sent routes learns the routes session s carried before.
func (s *Session) feedDirectionsCatchUp(received, sent bool) feedCatchUp {
	if sent && s != nil && s.advertised.Load() {
		return feedCatchUpImpossible
	}
	if !received {
		return feedCatchUpNothing
	}
	return s.feedCatchUp()
}

// bindEventsOf returns the reactor's bind event sink, or nil when there is no
// reactor or the plugin server was never wired (a reactor built for a test).
func bindEventsOf(r *Reactor) peerBindEvents {
	if r == nil {
		return nil
	}
	return r.bindEvents
}

// writeBus replaces the peer's running bus block and republishes the delivery
// graph. It takes r.mu then p.mu, the order every writer uses.
func (p *Peer) writeBus(r *Reactor, bus plugin.BusBlock) {
	if r != nil {
		r.mu.Lock()
	}
	p.mu.Lock()
	p.settings.Bus = bus.Clone()
	p.mu.Unlock()
	if r != nil {
		if r.deliveryPublished {
			r.publishDeliveryGraphLocked()
		}
		r.mu.Unlock()
	}
}

// sessionUpPhase is how far the plugins were told that one session is up, and
// whether they are being told it closed. A
// reload that attaches a process reads it to decide whether the process owes a
// targeted up (Peer.applyHotSwappableSettings), because the FSM's own up reads
// the delivery graph at delivery time (onPeerStateChange) and so reaches every
// process whose edge was published before that read.
type sessionUpPhase uint8

const (
	// sessionUpUnspecified: nothing told the plugins this session is up. The
	// FSM up has not started, so it will read a graph published from here on.
	sessionUpUnspecified sessionUpPhase = iota
	// sessionUpNegotiated: the OPENs are exchanged and the negotiated event is
	// being delivered, or was, while the FSM up has not started. That event
	// read the graph at a point no reader can place, so a process attached
	// now may or may not have it, and the reload restarts the session as it
	// does for sessionUpDelivering.
	sessionUpNegotiated
	// sessionUpDelivering: the FSM up is being delivered, and it reads the graph
	// at a point no reader can place, so it may or may not see an edge
	// published now.
	sessionUpDelivering
	// sessionUpDelivered: the FSM up returned, so it read the graph before
	// anything published from here on.
	sessionUpDelivered
	// sessionUpClosing: the session left Established and the FSM down is
	// being delivered, or was. It reads the graph after this mark, at a point
	// no reader can place, so a process whose edge is published now may or may
	// not receive it. A process given a targeted up must be told down by
	// whoever gave it.
	sessionUpClosing
)

// upPhaseLocked returns how far the plugins were told that session is up. A
// session other than the one the phase was recorded for was told nothing.
// The caller MUST hold p.mu.
func (p *Peer) upPhaseLocked(session *Session) sessionUpPhase {
	if session == nil {
		return sessionUpUnspecified
	}
	if p.upSession != session {
		return sessionUpUnspecified
	}
	return p.upPhase
}

// markSessionUp records how far the plugins were told that session is up. It
// writes under r.mu as well as p.mu because the reload's graph publish holds
// r.mu while it reads the phase (applyHotSwappableSettings step 3): a phase
// that moves to sessionUpDelivering or sessionUpClosing after that read moved
// after the publish, so the FSM up or down it precedes reads the published
// graph.
func (p *Peer) markSessionUp(r *Reactor, session *Session, phase sessionUpPhase) {
	r.mu.Lock()
	p.mu.Lock()
	p.upSession = session
	p.upPhase = phase
	p.mu.Unlock()
	r.mu.Unlock()
}

// announceNegotiated tells the plugins that session exchanged its OPENs: the
// negotiated capabilities, to the processes the delivery graph feeds when it
// is read. It runs on entry to OpenConfirm, before the KEEPALIVE that answers
// the peer's OPEN is sent, so every message of the session after the OPENs
// reaches a process after the negotiated event that describes it: ExaBGP
// states that block on the packet line of the OpenConfirm KEEPALIVEs too
// (docs/architecture/exabgp-bridge.md). The mark comes first, so a reload
// racing the delivery restarts the session rather than leave an attached
// process without the event (Peer.applyHotSwappableSettings).
// The caller MUST hold neither r.mu nor p.mu.
func (p *Peer) announceNegotiated(r *Reactor, session *Session, neg *capability.Negotiated) {
	if !session.beginNegotiatedPublication() {
		return
	}
	defer session.negotiationDone.Done()
	if hooks := p.sessionEndHooks.Load(); hooks != nil && hooks.beforeNegotiated != nil {
		hooks.beforeNegotiated()
	}
	p.markSessionUp(r, session, sessionUpNegotiated)
	r.notifyPeerNegotiated(p, session, neg)
}

// announceEstablished tells the plugins that session reached Established: the
// state up, to the processes the delivery graph feeds when it is read. The
// negotiated event went out on entry to OpenConfirm (announceNegotiated). The
// phase marks around the delivery are what lets a reload racing it give each
// attached process exactly one up.
// The caller MUST hold neither r.mu nor p.mu.
func (p *Peer) announceEstablished(r *Reactor, session *Session) {
	p.markSessionUp(r, session, sessionUpDelivering)
	r.notifyPeerEstablished(p)
	p.markSessionUp(r, session, sessionUpDelivered)
}

// announceClosed records that session left Established: it marks the session
// closing and does the reactor's bookkeeping, in the FSM leave-Established
// callback. The down itself, which reads the delivery graph, is announceDown,
// run later by Peer.runOnce once the session's buffered UPDATEs are
// published. The mark is set BEFORE that read, so a reload publishing an
// attached process's edge sees the mark whenever the down could have missed
// the edge, and tells that process down itself (applyHotSwappableSettings
// step 4). A mark after the read would leave a window in which the down
// missed the process and the reload still judged the session up.
// The caller MUST hold neither r.mu nor p.mu.
func (p *Peer) announceClosed(r *Reactor, session *Session) {
	p.markSessionUp(r, session, sessionUpClosing)
	r.forgetClosedPeer(p, session)
}

// announceDown tells the plugins that the session is down: the state down, to
// the processes the delivery graph feeds when it is read. announceClosed MUST
// have marked the session closing first.
// The caller MUST hold neither r.mu nor p.mu.
func (p *Peer) announceDown(r *Reactor, closed sessionClose) {
	r.notifyPeerClosed(p, closed)
}

// bindProcesses tells the named processes, whose feeds bus holds, that the
// running session is up for them: a state up with reason rpc.ReasonAttached to
// the ones fed bgp:state, then the negotiated event to the ones fed
// bgp:negotiated.
//
// The up carries InitialReplay 0. The session's initial routing update is not
// the attached process's to replay, and a token on it would authorize a replay
// for an update the session already sent.
//
// The caller MUST hold neither r.mu nor p.mu, and MUST NOT have published the
// processes' delivery edges yet: the delivery waits for every process
// (peerBindEvents), and the graph without the edges is what keeps the
// session's messages away from a process that has not been told it is up.
func (p *Peer) bindProcesses(r *Reactor, session *Session, bus plugin.BusBlock, names []string) {
	sink := bindEventsOf(r)
	if sink == nil {
		return
	}
	if len(names) == 0 {
		return
	}
	feeds, _ := busSendEndpointFeeds(bus.Send)
	var stateNames, negotiatedNames []string
	for _, name := range names {
		spec, ok := feeds[name]
		if !ok {
			continue
		}
		if spec.receivesState() {
			stateNames = append(stateNames, name)
		}
		if spec.receivesNegotiated() {
			negotiatedNames = append(negotiatedNames, name)
		}
	}
	if len(stateNames) == 0 && len(negotiatedNames) == 0 {
		return
	}
	var neg any
	if session != nil {
		session.mu.RLock()
		negotiated := session.negotiated
		session.mu.RUnlock()
		if negotiated != nil {
			neg = format.NegotiatedToDecoded(negotiated)
		}
	}
	info := boundPeerInfo(p, session)
	sink.OnPeerBound(&info, neg, stateNames, negotiatedNames)
}

// unbindProcesses tells the named processes that the peer is down for them
// alone. The caller MUST have republished the delivery graph without their
// edges and MUST NOT hold r.mu or p.mu. Every other consumer of the peer hears
// nothing and keeps the peer's routes, and the detached one drops them rather
// than select over routes nobody updates.
//
// The down ends no session, so it advances the peer's revocation epoch first:
// every Adj-RIB-Out path the peer owns at a destination stops being current,
// and the departure the down provokes (bgp-rs handleStateDown) owes those keys
// to the RIB's election instead of finding them still owned by a live session.
func (p *Peer) unbindProcesses(r *Reactor, names []string) {
	if len(names) == 0 {
		return
	}
	p.forwardRevocation.Add(1)
	// The revoked edge's released receipts can never be restored.
	r.dropReleasedSource(p)
	sink := bindEventsOf(r)
	if sink == nil {
		return
	}
	info := establishedPeerInfo(p)
	// The departure this down provokes covers the session that stays up,
	// whose paths the revocation above made no longer current.
	info.SessionGeneration = p.forwardGeneration.Load()
	sink.OnPeerUnbound(&info, rpc.SessionStateDown, rpc.ReasonPeerRemoved, names)
}

// catchUpBoundProcesses brings the processes a reload attached to the running
// session level with the routes it already carried, after their up was
// delivered and their edges published.
//
// The caller MUST NOT hold r.mu or p.mu: the calls below can block on the
// peer's socket.
func (p *Peer) catchUpBoundProcesses(bus plugin.BusBlock, gained []string) {
	session := p.currentSession()
	feeds, _ := busSendEndpointFeeds(bus.Send)
	refresh := false
	for _, name := range gained {
		spec, ok := feeds[name]
		if !ok {
			p.restartForBinding(session, gained, "process bound without a resolved feed rule: restarting the session")
			return
		}
		received, sent := spec.routeDirections()
		switch session.feedDirectionsCatchUp(received, sent) {
		case feedCatchUpNothing:
		case feedCatchUpRouteRefresh:
			refresh = true
		case feedCatchUpUnspecified, feedCatchUpImpossible:
			p.restartForBinding(session, gained, "process bound without the routes the session already carried: restarting the session")
			return
		default:
			panic("BUG: invalid bus feed catch-up outcome")
		}
	}
	if refresh {
		if err := p.requestRouteRefreshAll(session); err != nil {
			peerLogger().Warn("bound process catch-up: route refresh not sent",
				"peer", p.settings.Address, "processes", gained, "error", err)
		}
	}
}

// restartForBinding ends target because a bus feed change could not be
// delivered to it in place: a new session is the one way every consumer holds
// the same routes and sees one up. It ends target only, never the session
// after it, so a nil target, or one the peer has already left, ends nothing.
func (p *Peer) restartForBinding(target *Session, processes []string, why string) {
	if target == nil {
		return
	}
	peerLogger().Warn(why, "peer", p.settings.Address, "processes", processes)
	// RFC 4486 Section 4: subcode 6 "Other Configuration Change".
	if err := p.teardownOf(target, ceaseNotice(message.NotifyCeaseOtherConfigChange, ""), false); err != nil {
		peerLogger().Warn("bus feed change: session restart not queued",
			"peer", p.settings.Address, "error", err, "code", diagnostic.CodeDoctorInternalDefect)
	}
}

// sessionNegotiated answers what session negotiated, read from the peer's
// published negotiation (peer_run.go stores it from the same OPEN exchange),
// and refuses when session is no longer the peer's: the published value then
// belongs to another session, or to none.
func (p *Peer) sessionNegotiated(session *Session) (*NegotiatedCapabilities, error) {
	if session == nil {
		return nil, fmt.Errorf("peer %s: no session, so nothing was negotiated", p.addrString)
	}
	neg := p.negotiated.Load()
	if p.currentSession() != session {
		return nil, fmt.Errorf("peer %s: its session was replaced, so the negotiation judged is gone; commit again", p.addrString)
	}
	if neg == nil {
		return nil, fmt.Errorf("peer %s: the session has not negotiated capabilities yet", p.addrString)
	}
	return neg, nil
}

// requestRouteRefreshAll asks the peer, on session, to re-send its routes for
// every family that session negotiated. Two callers need it: a reload that
// attached a process (catchUpBoundProcesses), and an import policy relaxation
// (swapImportRising) whose recovery must reach the session it was judged on.
// Every consumer still fed receives the routes a second time, which is an
// implicit replacement of routes it already holds (RFC 4271 Section 3.1).
//
// The request is bound to session: a replacement session is a different
// negotiation, so a write on it would answer for a session nobody judged. It
// returns the first send failure, naming the family, after the requests before
// it were written: the caller decides what a partial recovery costs. The
// caller MUST NOT hold p.mu: a write can block on the socket.
func (p *Peer) requestRouteRefreshAll(session *Session) error {
	neg, err := p.sessionNegotiated(session)
	if err != nil {
		return err
	}
	// RFC 2918 Section 4: "A BGP speaker may send a ROUTE-REFRESH message to
	// its peer only if it has received the Route Refresh Capability from its
	// peer."
	if !neg.RouteRefresh {
		return fmt.Errorf("peer %s: the session did not negotiate Route Refresh, so it cannot be sent ROUTE-REFRESH", p.addrString)
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
		if err := session.SendRawMessage(0, message.PackTo(rr, nil)); err != nil {
			return fmt.Errorf("peer %s: ROUTE-REFRESH for %s not sent: %w", p.addrString, fam, err)
		}
		p.incrRefreshSent()
	}
	return nil
}
