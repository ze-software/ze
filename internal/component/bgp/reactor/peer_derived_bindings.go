// Design: docs/architecture/core-design.md — config reload delivers changed peer settings
// Related: peer_settings_apply.go — hotSwappableSettings carries receive-only bindings, applyHotSwappableSettings delivers them in order
// Related: reactor_dynamic.go — reconcileDynamicGroupsJournaled swaps a dynamic peer's bindings through the same ordered delivery
// Related: delivery_graph.go — the peer-to-process index a delivered binding republishes
// Related: config.go — parseProcessBindingsFromTree and EnsureProcessBinding, the writers of a binding
// RFC: rfc/short/rfc2918.md — ROUTE-REFRESH, how a new consumer learns what the peer sent
package reactor

import (
	"fmt"
	"reflect"
	"slices"

	"github.com/ze-software/ze/internal/component/bgp/format"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	bgpevents "github.com/ze-software/ze/internal/core/bgp/events"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/pkg/plugin/rpc"
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

// routeFreeReceiveTypes are the receive types that carry no route. A process
// attached with only these to a running session has nothing to catch up on.
// Every other type, a plugin-registered one included, counts as route-bearing:
// answering "carries routes" for a type that does not costs one catch-up, where
// the opposite answer would leave a consumer without routes. The OPEN is not
// listed because a binding receiving it is never receive-only.
var routeFreeReceiveTypes = map[string]bool{
	bgpevents.EventState:        true,
	bgpevents.EventNegotiated:   true,
	bgpevents.EventKeepalive:    true,
	bgpevents.EventNotification: true,
	bgpevents.EventRefresh:      true,
}

// receiveOnly reports whether b grants the process no send type and does not
// show it the OPEN exchange. Such a binding only feeds a consumer: the process
// can put nothing on this peer's wire through it, so it is outside the initial
// routing update and its End-of-RIB barrier (ProcessBinding.MayPushRoutes,
// Peer.resetAPISync), and a running session can take it on or let it go.
//
// The rule reads the grant, never who wrote it: an operator's attach block and
// a binding the config builder adds (EnsureProcessBinding) are judged alike.
// The OPEN refuses it because only a new session produces an OPEN the process
// can be shown, and the wildcard grants the OPEN.
func (b *ProcessBinding) receiveOnly() bool {
	if b.SendAll {
		return false
	}
	if len(b.Send) > 0 {
		return false
	}
	if b.ReceiveAll {
		return false
	}
	return b.Receive[bgpevents.EventOpen] == events.DirUnspecified
}

// routeDirections reports whether b's grant carries routes the peer sent
// (received) and routes this speaker sent the peer (sent). A bare type is
// granted in both directions (events.SplitTypeToken), so `update` sets both.
func (b *ProcessBinding) routeDirections() (received, sent bool) {
	for eventType, dir := range b.Receive {
		if routeFreeReceiveTypes[eventType] {
			continue
		}
		received = received || dir&events.DirReceived != 0
		sent = sent || dir&events.DirSent != 0
	}
	return received, sent
}

// onlyReceiveOnlyBindingsDiffer reports whether current and next hold the same
// bindings, in the same order, once the receive-only bindings on each side are
// set aside.
//
// A binding that may send, or that receives the OPEN, still restarts the
// session when it changes. So does a binding that moves between the two kinds,
// because it is set aside on one side only.
//
// Each pass of the loop consumes one binding from each list, so it runs at most
// len(current)+len(next) times.
func onlyReceiveOnlyBindingsDiffer(current, next []ProcessBinding) bool {
	i, j := 0, 0
	for {
		for i < len(current) && current[i].receiveOnly() {
			i++
		}
		for j < len(next) && next[j].receiveOnly() {
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

// receiveBindingChange names the processes a peer starts and stops feeding
// through a receive-only binding when its bindings move from before to after.
//
// A process that keeps a receive-only binding with a different grant is in
// BOTH lists: its consumer's picture under the old grant cannot be told apart
// from the new one, so it is told the peer is down and then up again, and is
// caught up like a process attached for the first time.
func receiveBindingChange(before, after []ProcessBinding) (gained, lost []string) {
	for i := range after {
		if after[i].receiveOnly() && !holdsSameBinding(before, &after[i]) {
			gained = append(gained, after[i].PluginName)
		}
	}
	for i := range before {
		if before[i].receiveOnly() && !holdsSameBinding(after, &before[i]) {
			lost = append(lost, before[i].PluginName)
		}
	}
	return gained, lost
}

// holdsSameBinding reports whether bindings holds b's process with b's grant.
func holdsSameBinding(bindings []ProcessBinding, b *ProcessBinding) bool {
	for i := range bindings {
		if bindings[i].PluginName == b.PluginName {
			return reflect.DeepEqual(bindings[i], *b)
		}
	}
	return false
}

// bindingFor returns the binding bindings holds for process, or nil.
func bindingFor(bindings []ProcessBinding, process string) *ProcessBinding {
	for i := range bindings {
		if bindings[i].PluginName == process {
			return &bindings[i]
		}
	}
	return nil
}

// withoutProcesses returns a copy of bindings with the named processes' bindings
// left out.
func withoutProcesses(bindings []ProcessBinding, processes []string) []ProcessBinding {
	kept := make([]ProcessBinding, 0, len(bindings))
	for i := range bindings {
		if !slices.Contains(processes, bindings[i].PluginName) {
			kept = append(kept, bindings[i])
		}
	}
	return kept
}

// grantedProcesses returns the processes among names whose binding in bindings
// passes granted.
func grantedProcesses(bindings []ProcessBinding, names []string, granted func(*ProcessBinding) bool) []string {
	var out []string
	for _, name := range names {
		if b := bindingFor(bindings, name); b != nil && granted(b) {
			out = append(out, name)
		}
	}
	return out
}

// receivesNegotiated reports whether b grants the negotiated event.
func receivesNegotiated(b *ProcessBinding) bool {
	return b.ReceiveAll || b.Receive[bgpevents.EventNegotiated] != events.DirUnspecified
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

// receiveBindingsDeliverable reports whether a running session can take the
// change from current to next in place. The caller has already established that
// only receive-only bindings differ (onlyReceiveOnlyBindingsDiffer).
//
// Every process the change attaches, or re-attaches with a new grant
// (receiveBindingChange), must also learn what the session already carried in
// each direction its grant covers, or it holds half of that peer's routes.
// Session.bindingCatchUp says whether it can; any process that cannot restarts
// the whole change.
func receiveBindingsDeliverable(current, next []ProcessBinding, s *Session) bool {
	gained, _ := receiveBindingChange(current, next)
	for _, name := range gained {
		switch s.bindingCatchUp(bindingFor(next, name)) {
		case feedCatchUpNothing, feedCatchUpRouteRefresh:
		case feedCatchUpUnspecified, feedCatchUpImpossible:
			return false
		default:
			panic("BUG: invalid binding catch-up outcome")
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

// feedCatchUp answers Session.bindingCatchUp's question for the routes the
// peer sent. No session, and a session that announced no route, owe a new
// consumer nothing. Otherwise the peer's Route Refresh capability decides.
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

// bindingCatchUp says how a process attached to session s through binding b
// learns the routes the session carried before, in each direction b covers.
//
// A grant that carries no route needs nothing. Routes this speaker sent cannot
// be asked for again: nothing re-sends them, so once the session advertised one
// (Session.advertised, the state Peer.hasAdvertised reads) a grant covering the
// sent direction is impossible. Routes the peer sent are feedCatchUp's answer.
func (s *Session) bindingCatchUp(b *ProcessBinding) feedCatchUp {
	if b == nil {
		return feedCatchUpImpossible
	}
	received, sent := b.routeDirections()
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

// writeBindings replaces the peer's running bindings and republishes the
// delivery graph. It takes r.mu then p.mu, the order every writer uses, because
// the graph build and maySend read the bindings under r.mu alone.
func (p *Peer) writeBindings(r *Reactor, bindings []ProcessBinding) {
	if r != nil {
		r.mu.Lock()
	}
	p.mu.Lock()
	p.settings.ProcessBindings = bindings
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
	r.notifyPeerNegotiated(p, neg)
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
	r.forgetClosedPeer(p)
}

// announceDown tells the plugins that the session is down: the state down, to
// the processes the delivery graph feeds when it is read. announceClosed MUST
// have marked the session closing first.
// The caller MUST hold neither r.mu nor p.mu.
func (p *Peer) announceDown(r *Reactor, closed sessionClose) {
	r.notifyPeerClosed(p, closed)
}

// bindProcesses tells the named processes, whose bindings bindings holds, that
// the running session is up for them: a state up with reason
// rpc.ReasonAttached to the ones granted the state event, then the negotiated
// event to the ones granted it.
//
// The up carries InitialReplay 0. The session's initial routing update is not
// the attached process's to replay, and a token on it would authorize a replay
// for an update the session already sent.
//
// The caller MUST hold neither r.mu nor p.mu, and MUST NOT have published the
// processes' delivery edges yet: the delivery waits for every process
// (peerBindEvents), and the graph without the edges is what keeps the
// session's messages away from a process that has not been told it is up.
func (p *Peer) bindProcesses(r *Reactor, session *Session, bindings []ProcessBinding, names []string) {
	sink := bindEventsOf(r)
	if sink == nil {
		return
	}
	if len(names) == 0 {
		return
	}
	stateNames := grantedProcesses(bindings, names, (*ProcessBinding).ReceivesPeerState)
	negotiatedNames := grantedProcesses(bindings, names, receivesNegotiated)
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
	info := establishedPeerInfo(p)
	info.InitialReplay = 0
	sink.OnPeerBound(&info, neg, stateNames, negotiatedNames)
}

// unbindProcesses tells the named processes that the peer is down for them
// alone. The caller MUST have republished the delivery graph without their
// edges and MUST NOT hold r.mu or p.mu. Every other consumer of the peer hears
// nothing and keeps the peer's routes, and the detached one drops them rather
// than select over routes nobody updates.
//
// The down is not narrowed to the processes granted the state event, where the
// up is: a process holds the peer's routes by its update grant, and the down is
// what makes it let them go. The plugin server still delivers it only to a
// process subscribed to state events.
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
	// The revoked binding's released receipts can never be restored.
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
// The answer is decided again here, per process (Session.bindingCatchUp),
// because the peer can have announced its first route, or this speaker
// advertised one, since the swap plan asked. The read path marks a route
// received before it looks the consumers up in the graph, so a route this
// check does not see reaches the new consumer live. One process that cannot
// catch up restarts the session; otherwise one ROUTE-REFRESH per negotiated
// family serves every process that needs the peer's routes.
//
// The caller MUST NOT hold r.mu or p.mu: the calls below can block on the
// peer's socket.
func (p *Peer) catchUpBoundProcesses(bindings []ProcessBinding, gained []string) {
	session := p.currentSession()
	refresh := false
	for _, name := range gained {
		switch session.bindingCatchUp(bindingFor(bindings, name)) {
		case feedCatchUpNothing:
		case feedCatchUpRouteRefresh:
			refresh = true
		case feedCatchUpUnspecified, feedCatchUpImpossible:
			p.restartForBinding(session, gained, "process bound without the routes the session already carried: restarting the session")
			return
		default:
			panic("BUG: invalid binding catch-up outcome")
		}
	}
	if refresh {
		if err := p.requestRouteRefreshAll(session); err != nil {
			peerLogger().Warn("bound process catch-up: route refresh not sent",
				"peer", p.settings.Address, "processes", gained, "error", err)
		}
	}
}

// restartForBinding ends target because a binding change could not be
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
		peerLogger().Warn("binding change: session restart not queued",
			"peer", p.settings.Address, "error", err)
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
