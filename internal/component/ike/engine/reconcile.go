// Design: plan/learned/740-ipsec-7-ikev2-engine.md -- config reconciliation
// RFC: rfc/short/rfc7296.md -- Section 1.4 (Delete), Section 2.4 (state sync / re-init)
package engine

import (
	"log/slog"
	"maps"
	"sync"
	"sync/atomic"

	"codeberg.org/thomas-mangin/ze/internal/component/ike/dataplane"
	"codeberg.org/thomas-mangin/ze/internal/component/ike/ipsec"
	"codeberg.org/thomas-mangin/ze/internal/component/ike/transport"
	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

// PeerSession manages a single IKE peer's goroutine lifecycle.
type PeerSession struct {
	peerName string
	peerCfg  ipsec.SiteToSitePeer
	ikeGroup ipsec.IKEGroup // retained so the responder can negotiate on inbound
	espGroup ipsec.ESPGroup
	sa       *SA

	// responderBusy gates a `respond` peer to ONE in-flight half-open handshake --
	// its documented meaning. The shared dispatchInbound goroutine CAS-sets it true
	// when it creates a responder SA on an unsolicited IKE_SA_INIT; it is cleared
	// once that SA reaches StateEstablished (runResponder adoption, or promotion of a
	// parallel SA) or dies/is reaped. RFC 7296 Section 2.4: it is NOT held across the
	// established lifetime, so a fresh IKE_SA_INIT that arrives while an SA is up is
	// accepted in parallel (pendingSA) rather than dropped. A concurrent second
	// half-open attempt while busy is dropped (AC-6); a genuine retransmit finds the
	// SA already in the SATable and never reaches the creation path. Initiator
	// sessions never touch it.
	responderBusy atomic.Bool

	// ownedSA is the SA maintainSA currently owns (runEstablished sets it, clears it
	// on return, and updates it on an IKE-SA rekey swap). routeInbound keys the
	// owner-loop hand-off on SA identity (ownedSA == packet's SA), not the peer name,
	// so a parallel half-open SA's handshake packets are handled inline on the
	// dispatch goroutine while the established SA's traffic goes to the owner loop
	// (spec-fixit-ipsec-clear-reestablish, coupling #1). Atomic: read on the shared
	// dispatch goroutine, written by the session goroutine.
	ownedSA atomic.Pointer[SA]

	// graceful marks a session that operator `clear` is bouncing (vs a config-change
	// stop). Set by StopGraceful before Stop; the owner loop reads it in its stopCh
	// case and sends an authenticated INFORMATIONAL Delete (RFC 7296 Section 1.4) so
	// the peer tears down at once instead of waiting for DPD. Kept distinct from
	// Stop()'s meaning (R-6).
	graceful atomic.Bool

	mu         sync.Mutex
	childSA    *ChildSA
	rekeyCount uint64

	// pendingSA / pendingChild are the single explicit second slot for a parallel
	// responder handshake accepted while an established SA still owns the loop
	// (RFC 7296 Section 2.4 coexist-then-supersede-on-authentication). Guarded by mu.
	// The new SA supersedes the old only once it authenticates (finishResponderEstablish
	// signals supersede); the new Child SA lives in pendingChild until promotion so the
	// old owner loop's cleanupChild removes only its own child (make-before-break).
	pendingSA    *SA
	pendingChild *ChildSA

	// inbound carries packets for an ESTABLISHED SA from the shared
	// dispatchInbound goroutine to this session's maintainSA owner loop, so
	// all post-establishment SA/childSA mutation happens on one goroutine
	// (rekey design, spec-ipsec-13). Sends are non-blocking: dispatchInbound
	// serves every peer, so it must never block on one slow owner.
	inbound chan transport.Packet

	// pendingRekey and supersededChild are owned exclusively by the maintainSA
	// loop (no lock): the CREATE_CHILD_SA exchange we initiated and await a
	// response for, and (as rekey responder) the old Child SA kept installed
	// until the peer's INFORMATIONAL Delete arrives (make-before-break).
	pendingRekey    *pendingRekey
	supersededChild *ChildSA

	// pendingIKESwap holds the new IKE SA we built while responding to a peer's
	// IKE-SA rekey; the owner loop swaps to it when the peer's INFORMATIONAL Delete
	// of the old IKE SA arrives (make-before-break, RFC 7296 Section 2.8). Owned by
	// the maintainSA loop.
	pendingIKESwap *SA

	stopCh   chan struct{}
	done     chan struct{}
	stopOnce sync.Once

	// supersede signals the owner loop (maintainSA) to relinquish the established SA
	// because a parallel IKE_SA_INIT authenticated (RFC 7296 Section 2.4). Buffered 1.
	supersede chan struct{}
}

// setSA / getSA guard the ps.sa pointer for the responder handoff: the shared
// dispatchInbound goroutine publishes the responder SA it created, and runResponder
// (a different goroutine) reads it while polling for establishment. Guarding the
// pointer avoids a data race on the handoff; the SA's own fields during the
// handshake follow the same single-writer (dispatch) model the initiator uses.
func (ps *PeerSession) setSA(sa *SA) {
	ps.mu.Lock()
	ps.sa = sa
	ps.mu.Unlock()
}

func (ps *PeerSession) getSA() *SA {
	ps.mu.Lock()
	sa := ps.sa
	ps.mu.Unlock()
	return sa
}

// setPendingSA / getPendingSA guard the second SA slot for a parallel responder
// handshake accepted while an established SA still owns the loop. Written by the
// dispatch goroutine (tryResponderSAInit), read by the session goroutine
// (runResponder promotion) and the owner loop's stale-pending reaper.
func (ps *PeerSession) setPendingSA(sa *SA) {
	ps.mu.Lock()
	ps.pendingSA = sa
	ps.mu.Unlock()
}

func (ps *PeerSession) getPendingSA() *SA {
	ps.mu.Lock()
	sa := ps.pendingSA
	ps.mu.Unlock()
	return sa
}

// setPendingChild / getPendingChild guard the parallel handshake's installed Child
// SA until the old SA is superseded and the pending SA is promoted (make-before-break).
func (ps *PeerSession) setPendingChild(c *ChildSA) {
	ps.mu.Lock()
	ps.pendingChild = c
	ps.mu.Unlock()
}

func (ps *PeerSession) getPendingChild() *ChildSA {
	ps.mu.Lock()
	c := ps.pendingChild
	ps.mu.Unlock()
	return c
}

// signalSupersede tells the owner loop that a parallel IKE_SA_INIT authenticated, so
// it should relinquish the old SA and let runResponder promote the new one. RFC 7296
// Section 2.4: this happens only on an AUTHENTICATED message (IKE_AUTH), never on the
// unauthenticated IKE_SA_INIT. Non-blocking: the channel is buffered 1 and the owner
// loop consumes at most one supersede per established SA.
func (ps *PeerSession) signalSupersede(log *slog.Logger) {
	if ps.supersede == nil {
		return
	}
	select {
	case ps.supersede <- struct{}{}:
	default:
		log.Debug("ike: supersede already pending", "peer", ps.peerName)
	}
}

// setPendingIKESwap records the new IKE SA built while responding to a peer's IKE
// rekey, clearing any prior unconfirmed pending SA's key material first so a peer
// that re-initiates before Deleting the old SA cannot leak keys (Finding 4). Owned
// by the maintainSA loop; no lock.
func (ps *PeerSession) setPendingIKESwap(newSA *SA) {
	if ps.pendingIKESwap != nil && ps.pendingIKESwap.SKKeys != nil {
		ps.pendingIKESwap.SKKeys.Clear()
	}
	ps.pendingIKESwap = newSA
}

func (ps *PeerSession) setChildSA(c *ChildSA) {
	ps.mu.Lock()
	ps.childSA = c
	ps.mu.Unlock()
}

func (ps *PeerSession) getChildSA() *ChildSA {
	ps.mu.Lock()
	c := ps.childSA
	ps.mu.Unlock()
	return c
}

func (ps *PeerSession) incRekeyCount() {
	ps.mu.Lock()
	ps.rekeyCount++
	ps.mu.Unlock()
}

// PeerInfo is a snapshot of a peer session's state for display.
type PeerInfo struct {
	PeerName      string
	RemoteAddress string
	LocalAddress  string
	AuthMode      string
	ChildInSPI    uint32
	ChildOutSPI   uint32
	ChildIfID     uint32
	TSLocal       string
	TSRemote      string
	ESPEncryption string
	ESPIntegrity  string
	Lifetime      uint32
	RekeyCount    uint64
	HasChild      bool
}

// Info returns a snapshot of the peer session for display.
func (ps *PeerSession) Info() PeerInfo {
	ps.mu.Lock()
	child := ps.childSA
	rekeys := ps.rekeyCount
	ps.mu.Unlock()

	info := PeerInfo{
		PeerName:      ps.peerName,
		RemoteAddress: ps.peerCfg.RemoteAddress,
		LocalAddress:  ps.peerCfg.LocalAddress,
		AuthMode:      ps.peerCfg.Auth.Mode.String(),
		Lifetime:      ps.espGroup.Lifetime,
		RekeyCount:    rekeys,
	}
	if child != nil {
		info.HasChild = true
		info.ChildInSPI = child.InboundSPI
		info.ChildOutSPI = child.OutboundSPI
		info.ChildIfID = child.IfID
		if child.TSLocal != nil {
			info.TSLocal = child.TSLocal.String()
		}
		if child.TSRemote != nil {
			info.TSRemote = child.TSRemote.String()
		}
		if len(ps.espGroup.Proposals) > 0 {
			info.ESPEncryption = ps.espGroup.Proposals[0].Encryption.String()
			info.ESPIntegrity = ps.espGroup.Proposals[0].Hash.String()
		}
	}
	return info
}

// Stop signals the peer session to shut down and waits for cleanup.
// Safe to call multiple times.
func (ps *PeerSession) Stop() {
	ps.stopOnce.Do(func() { close(ps.stopCh) })
	<-ps.done
}

// StopGraceful stops the session like Stop but first marks it graceful, so the owner
// loop sends an authenticated INFORMATIONAL Delete on its way out (RFC 7296 Section
// 1.4). Used only by the operator `clear` path (TerminateAllSAs / TerminatePeerSA) so
// the peer tears its SA down at once instead of waiting for the DPD timeout; a plain
// config-change Stop stays silent (R-6).
func (ps *PeerSession) StopGraceful() {
	ps.graceful.Store(true)
	ps.Stop()
}

// reconcilePeers diffs the new config against running peers and starts/stops
// peer sessions as needed. Follows the PPPoE reconciliation pattern.
func reconcilePeers(
	newCfg, _ *ipsec.IPsecConfig,
	active map[string]*PeerSession,
	table *SATable,
	tr *transport.UDPTransport,
	bus ze.EventBus,
	log *slog.Logger,
) {
	desired := make(map[string]ipsec.SiteToSitePeer, len(newCfg.Peers))
	maps.Copy(desired, newCfg.Peers)

	dp := dataplane.Get()

	// Build removal list under read lock, then stop outside the lock.
	type toStop struct {
		name string
		ps   *PeerSession
	}
	var removing []toStop

	peersMu.RLock()
	for name, ps := range active {
		newPeer, ok := desired[name]
		if !ok || peerConfigChanged(ps, newPeer) {
			removing = append(removing, toStop{name, ps})
		}
	}
	peersMu.RUnlock()

	for _, r := range removing {
		log.Info("ike: stopping peer", "peer", r.name)
		r.ps.Stop()
		child := r.ps.getChildSA()
		if child != nil {
			removeChildSA(child, dp, log)
			emitChildDown(bus, r.name, child, log)
			emitRouteRemove(bus, child.TSRemote, log)
			r.ps.setChildSA(nil)
		}
		// getSA (mutex-guarded): a responder's ps.sa is written by the dispatch
		// goroutine, which ps.Stop() does not join, so read it under the lock (Finding 3).
		if sa := r.ps.getSA(); sa != nil {
			table.Remove(sa.InitiatorSPI, sa.ResponderSPI)
			emitSADown(bus, sa, log)
		}
		// A parallel responder handshake in flight at reconcile time has its own SATable
		// entry and possibly an installed Child SA in the second slot; free them too.
		r.ps.cleanupPendingSA(table, dp, bus, log)
		peersMu.Lock()
		delete(active, r.name)
		peersMu.Unlock()
	}

	// Start new or restarted peers.
	for name := range desired {
		peersMu.RLock()
		_, running := active[name]
		peersMu.RUnlock()
		if running {
			continue
		}
		peer := desired[name]
		ikeGroup, ok := newCfg.IKEGroups[peer.IKEGroup]
		if !ok {
			log.Warn("ike: peer references unknown ike-group", "peer", name, "ike-group", peer.IKEGroup)
			continue
		}
		espGroup, ok := newCfg.ESPGroups[peer.ESPGroup]
		if !ok {
			log.Warn("ike: peer references unknown esp-group", "peer", name, "esp-group", peer.ESPGroup)
			continue
		}
		ps := startPeerSession(name, peer, ikeGroup, espGroup, table, tr, bus, log)
		peersMu.Lock()
		active[name] = ps
		peersMu.Unlock()
		log.Info("ike: started peer", "peer", name, "connection-type", peer.ConnectionType)
	}
}

func peerConfigChanged(ps *PeerSession, newPeer ipsec.SiteToSitePeer) bool {
	old := ps.peerCfg
	return old.RemoteAddress != newPeer.RemoteAddress ||
		old.LocalAddress != newPeer.LocalAddress ||
		old.IKEGroup != newPeer.IKEGroup ||
		old.ESPGroup != newPeer.ESPGroup ||
		old.ConnectionType != newPeer.ConnectionType ||
		old.Auth.Mode != newPeer.Auth.Mode ||
		old.Auth.PSK != newPeer.Auth.PSK ||
		old.Auth.Certificate != newPeer.Auth.Certificate
}

func startPeerSession(
	name string,
	peer ipsec.SiteToSitePeer,
	ikeGroup ipsec.IKEGroup,
	espGroup ipsec.ESPGroup,
	table *SATable,
	tr *transport.UDPTransport,
	bus ze.EventBus,
	log *slog.Logger,
) *PeerSession {
	ps := &PeerSession{
		peerName:  name,
		peerCfg:   peer,
		ikeGroup:  ikeGroup,
		espGroup:  espGroup,
		inbound:   make(chan transport.Packet, inboundQueueDepth),
		stopCh:    make(chan struct{}),
		done:      make(chan struct{}),
		supersede: make(chan struct{}, 1),
	}
	go ps.run(peer, ikeGroup, table, tr, bus, log)
	return ps
}

// run is the main goroutine for a peer session with reconnection on failure.
func (ps *PeerSession) run(
	peer ipsec.SiteToSitePeer,
	ikeGroup ipsec.IKEGroup,
	table *SATable,
	tr *transport.UDPTransport,
	bus ze.EventBus,
	log *slog.Logger,
) {
	defer close(ps.done)

	for {
		err := ps.runOnce(peer, ikeGroup, table, tr, bus, log)
		if err == nil || ps.stopped() {
			return
		}

		delay := reconnectDelay(ps)
		log.Info("ike: peer session ended, reconnecting",
			"peer", ps.peerName, "error", err, "delay", delay)

		select {
		case <-ps.stopCh:
			return
		case <-afterFunc(delay):
		}
	}
}

func (ps *PeerSession) stopped() bool {
	select {
	case <-ps.stopCh:
		return true
	default:
		return false
	}
}

func emitSADown(bus ze.EventBus, sa *SA, log *slog.Logger) {
	if bus == nil || sa == nil {
		return
	}
	if _, err := SADown.Emit(bus, &SAEvent{
		PeerName:      sa.PeerName,
		InitiatorSPI:  SPIHex(sa.InitiatorSPI),
		ResponderSPI:  SPIHex(sa.ResponderSPI),
		RemoteAddress: sa.PeerCfg.RemoteAddress,
		AuthMethod:    sa.PeerCfg.Auth.Mode.String(),
	}); err != nil {
		log.Warn("ike: emit sa-down failed", "error", err)
	}
}
