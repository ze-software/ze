// Design: plan/learned/742-ipsec-8-ikev2-child-xfrm.md -- established SA lifecycle
// RFC: rfc/short/rfc7296.md -- Child SA, DPD, rekeying after IKE_AUTH

package engine

import (
	"log/slog"
	"time"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/pkg/ze"
)

// runEstablished handles the post-IKE_AUTH lifecycle: child SA, DPD, rekey.
func (ps *PeerSession) runEstablished(
	sa *SA,
	peer ipsec.SiteToSitePeer,
	ikeGroup ipsec.IKEGroup,
	table *SATable,
	tr *transport.UDPTransport,
	bus ze.EventBus,
	log *slog.Logger,
) error {
	dp := dataplane.Get()

	// Drain any stale supersede token left by a previous cycle: it belongs to the SA
	// that just relinquished, not this one. Without this, a token signaled while the
	// prior owner loop was already exiting (e.g. DPD) would fire on THIS SA's first
	// select and tear it down with nothing to promote. maintainSA is the sole receiver
	// of ps.supersede, so a len>0 check guarantees the receive cannot block.
	if len(ps.supersede) > 0 {
		<-ps.supersede
		log.Debug("ike: drained stale supersede token", "peer", ps.peerName)
	}

	// maintainSA now owns this exact SA: routeInbound hands established-SA packets to
	// the owner loop by SA identity (ownedSA == packet's SA). Cleared on return so a
	// reconnect handshake is handled inline again. RFC 7296 Section 2.4: a parallel
	// half-open SA of the same peer has a different identity and is never routed here.
	ps.ownedSA.Store(sa)
	defer ps.ownedSA.Store(nil)

	ifID := resolveIfID(peer)

	var child *ChildSA
	if sa.IsInitiator {
		var err error
		child, err = createFirstChildSA(sa, ps.espGroup, peer.LocalAddress, peer.RemoteAddress, ifID, dp, log)
		if err != nil {
			log.Warn("ike: child SA creation failed", "peer", ps.peerName, "error", err)
			return err
		}
		ps.setChildSA(child)
	} else {
		// Responder: the first Child SA was already negotiated and installed during
		// handleAuthRequest (it had to answer with SAr2/TSr), so adopt it here rather
		// than creating a second one (spec-ipsec-14 R-6).
		child = ps.getChildSA()
		if child == nil {
			log.Warn("ike: responder established without a child SA", "peer", ps.peerName)
			return errInvalidMessage
		}
	}

	if child.TSRemote != nil {
		log.Debug("ike: tunnel route", "peer", ps.peerName, "ts_remote", child.TSRemote.String(), "bus_set", bus != nil)
	} else {
		log.Debug("ike: tunnel route nil tsRemote", "peer", ps.peerName)
	}
	emitChildUp(bus, ps.peerName, child, log)
	emitRouteAdd(bus, child.TSRemote, log)

	// RFC 3948 Section 2.3: start NAT keepalive when NAT is detected.
	if sa.NATDetected && tr != nil {
		remote := sa.remoteUDPAddr()
		if remote != nil {
			remote.Port = transport.NATTPort
			ka := transport.NewKeepalive(tr.Conn(), remote, transport.DefaultKeepaliveInterval, log)
			go ka.Run()
			defer ka.Stop()
			log.Info("ike: NAT keepalive started", "peer", ps.peerName, "remote", remote)
		}
	}

	dpd := newDPDState(ikeGroup.DPD)
	childLT := newLifetimeState(ps.espGroup.Lifetime)
	ikeLT := newLifetimeState(ikeGroup.Lifetime)

	return ps.maintainSA(sa, dpd, childLT, ikeLT, ikeGroup, table, dp, tr, bus, log)
}

// maintainSA runs the DPD + rekey loop until stopped or peer dies.
func (ps *PeerSession) maintainSA(
	sa *SA,
	dpd *dpdState,
	childLT *lifetimeState,
	ikeLT *lifetimeState,
	ikeGroup ipsec.IKEGroup,
	table *SATable,
	dp dataplane.Dataplane,
	tr *transport.UDPTransport,
	bus ze.EventBus,
	log *slog.Logger,
) error {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	// Re-announce routes periodically so the redistribute-orchestrator
	// (which subscribes asynchronously) catches routes that were emitted
	// before its subscription was active.
	routeReannounce := time.NewTicker(30 * time.Second)
	defer routeReannounce.Stop()

	for {
		select {
		case <-ps.stopCh:
			// RFC 7296 Section 1.4: on an operator `clear` (graceful) say goodbye so the
			// peer tears its SA down at once instead of waiting for the DPD timeout. Built
			// and sent HERE, on the owner goroutine, because sendDeleteIKE mutates
			// sa.NextMsgID and reads sa.SKKeys -- state owned solely by maintainSA (A-5);
			// TerminateAllSAs runs on the RPC goroutine and must not touch it. Best-effort
			// UDP: a lost Delete falls back to the DPD self-heal path (R-4).
			if ps.graceful.Load() {
				ps.sendDeleteIKE(sa, tr, log)
			}
			ps.cleanupChild(dp, bus, log)
			return nil
		case <-ps.supersede:
			// RFC 7296 Section 2.4: a parallel IKE_SA_INIT authenticated (the new SA
			// reached IKE_AUTH), so relinquish this old SA and let runResponder promote
			// the new one. The new Child SA is already installed in ps.pendingChild;
			// remove only ours (make-before-break, so traffic is not dropped before the
			// new tunnel is up, R-2).
			ps.cleanupChild(dp, bus, log)
			log.Info("ike: superseded by a re-initiated SA, relinquishing owner loop", "peer", ps.peerName)
			return nil
		case <-routeReannounce.C:
			if child := ps.getChildSA(); child != nil {
				emitRouteAdd(bus, child.TSRemote, log)
			}
		case pkt := <-ps.inbound:
			out := ps.handleOwnedInbound(sa, pkt, tr, dp, log)
			// Clear the DPD wait on an in-window authenticated inbound (peerAlive), or
			// on an authenticated INFORMATIONAL response whose message ID matches the
			// outstanding probe (matchesProbe rejects replays / out-of-window acks).
			if out.peerAlive || (out.dpdResp && dpd.matchesProbe(out.dpdRespMsgID)) {
				handleDPDResponse(dpd, log, ps.peerName)
			}
			if out.newSA != nil {
				// RFC 7296 §2.8: rekeyed IKE SA has new SPIs. Point routing at the new SA
				// BEFORE it is discoverable in the table, so a packet for it is never
				// briefly handled inline instead of on this owner loop; then re-key the
				// table and swap the loop's SA.
				oldSA := sa
				sa = out.newSA
				ps.ownedSA.Store(sa)
				table.Insert(sa)
				table.Remove(oldSA.InitiatorSPI, oldSA.ResponderSPI)
				oldSA.SKKeys.Clear()
				ikeLT = newLifetimeState(ikeGroup.Lifetime)
				ps.incRekeyCount()
			}
			if out.newChild != nil {
				childLT = newLifetimeState(ps.espGroup.Lifetime)
				ps.incRekeyCount()
				emitChildRekey(bus, ps.peerName, out.newChild, log)
				emitRouteAdd(bus, out.newChild.TSRemote, log)
			}
		case now := <-ticker.C:
			// Drop a parallel half-open handshake the peer abandoned before it could
			// authenticate, so responderBusy and its SATable slot free up (AC-6 for the
			// parallel path). A pending SA that DID authenticate returns us via the
			// supersede case above, so anything still pending past the timeout is dead.
			ps.reapStalePending(now, table, dp, log)
			if sa.State == StateDead {
				log.Info("ike: SA marked dead by peer", "peer", ps.peerName)
				ps.cleanupChild(dp, bus, log)
				return nil
			}

			if dpd != nil && dpd.timedOut(now) {
				log.Warn("dpd: peer dead", "peer", ps.peerName, "action", dpd.action)
				ps.cleanupChild(dp, bus, log)
				return errTimeout
			}

			ps.serviceRequestWindow(sa, dpd, now, log)

			if dpd != nil && dpd.shouldSend(now) {
				sendDPD(sa, tr, dpd, log)
			}

			// RFC 7296 §1.3.2: on soft-lifetime expiry, initiate a Child SA
			// rekey via a CREATE_CHILD_SA wire exchange. Completion (key install,
			// old-SA delete, childLT reset) happens in handleOwnedInbound when the
			// response arrives; here we only start it and manage retransmission.
			if childLT != nil && childLT.softExpired(now) && ps.pendingRekey == nil {
				ps.startChildRekey(sa, tr, log)
			}

			if ps.pendingRekey != nil {
				if err := ps.serviceRekeyRetransmit(sa, tr, now, dp, bus, log); err != nil {
					return err
				}
			}

			// Hard-expire only when no rekey is in flight: a rekey initiated at
			// soft-expiry (which can be within jitter of the hard time) must be
			// allowed to complete or exhaust its own retransmits, not be torn down
			// the same tick it started.
			if childLT != nil && childLT.hardExpired(now) && ps.pendingRekey == nil {
				log.Warn("child-sa: hard lifetime expired", "peer", ps.peerName)
				ps.cleanupChild(dp, bus, log)
				return errTimeout
			}

			// RFC 7296 §1.3.3: on soft-lifetime expiry, initiate an IKE SA rekey via
			// a CREATE_CHILD_SA wire exchange. Completion (new SA, table re-key, SA
			// swap) happens in the inbound case when the response arrives.
			if ikeLT != nil && ikeLT.softExpired(now) && ps.pendingRekey == nil {
				ps.startIKERekey(sa, ikeGroup, tr, log)
			}

			if ikeLT != nil && ikeLT.hardExpired(now) && ps.pendingRekey == nil {
				log.Warn("ike-sa: hard lifetime expired", "peer", ps.peerName)
				ps.cleanupChild(dp, bus, log)
				return errTimeout
			}
		}
	}
}

// rekeyRetransmitTimeout is how long the owner loop waits for a rekey response
// before retransmitting the request. RFC 7296 §2.1 (retransmission).
const rekeyRetransmitTimeout = 3 * time.Second

// startChildRekey begins a Child SA rekey. RFC 7296 §2.3 allows one self-initiated
// request per SA, so the window is reserved before initiateChildRekey reads
// NextMsgID. A held window defers the rekey: pendingRekey stays nil and the soft
// lifetime still stands, so the next tick raises the rekey again. RFC 7296 §1.3.2.
func (ps *PeerSession) startChildRekey(sa *SA, tr *transport.UDPTransport, log *slog.Logger) {
	old := ps.getChildSA()
	if old == nil {
		return
	}
	if !sa.reserveRequestWindow() {
		log.Debug("child-sa: rekey deferred, a request is outstanding", "peer", ps.peerName)
		return
	}
	msg, pending, err := initiateChildRekey(sa, old)
	if err != nil {
		sa.releaseRequestWindow()
		log.Warn("child-sa: rekey init failed", "peer", ps.peerName, "error", err)
		return
	}
	sendRaw(sa, tr, msg, log)
	ps.pendingRekey = pending
	log.Info("child-sa: rekey initiated", "peer", ps.peerName, "msgid", pending.messageID)
}

// startIKERekey begins an IKE SA rekey under the same RFC 7296 §2.3 window as
// startChildRekey. A held window defers it to a later tick. RFC 7296 §1.3.3.
func (ps *PeerSession) startIKERekey(sa *SA, ikeGroup ipsec.IKEGroup, tr *transport.UDPTransport, log *slog.Logger) {
	if !sa.reserveRequestWindow() {
		log.Debug("ike-sa: rekey deferred, a request is outstanding", "peer", ps.peerName)
		return
	}
	msg, pending, err := initiateIKERekey(sa, ikeGroup)
	if err != nil {
		sa.releaseRequestWindow()
		log.Warn("ike-sa: rekey init failed", "peer", ps.peerName, "error", err)
		return
	}
	sendRaw(sa, tr, msg, log)
	ps.pendingRekey = pending
	log.Info("ike-sa: rekey initiated", "peer", ps.peerName, "msgid", pending.messageID)
}

// serviceRequestWindow frees a request window that no other timer can free. A rekey
// ends the session once its retransmissions run out, and a DPD probe ends it once
// the peer stays silent past the DPD timeout. Both therefore bound their own hold.
// A Delete has neither a retransmission nor a deadline, so only a Delete reaches
// requestWindowTimeout. RFC 7296 §1.4, §2.3.
func (ps *PeerSession) serviceRequestWindow(sa *SA, dpd *dpdState, now time.Time, log *slog.Logger) {
	if ps.pendingRekey != nil || dpd.awaitingReply() {
		return
	}
	if !sa.requestWindowStale(now) {
		return
	}
	sa.releaseRequestWindow()
	log.Debug("ike: freed the request window, the answer never arrived", "peer", ps.peerName)
}

// sendRaw sends already-built wire bytes to the peer's IKE address.
func sendRaw(sa *SA, tr *transport.UDPTransport, msg []byte, log *slog.Logger) {
	if tr == nil {
		return
	}
	remote := sa.remoteUDPAddr()
	if remote == nil {
		return
	}
	if err := tr.Send(msg, remote); err != nil {
		log.Debug("ike: send failed", "peer", sa.PeerName, "error", err)
	}
}

// serviceRekeyRetransmit retransmits an outstanding rekey request whose response
// has not arrived, and tears the SA down once retransmissions are exhausted so a
// stalled rekey cannot leave the tunnel running on soon-to-expire keys (AC-8).
func (ps *PeerSession) serviceRekeyRetransmit(sa *SA, tr *transport.UDPTransport, now time.Time, dp dataplane.Dataplane, bus ze.EventBus, log *slog.Logger) error {
	p := ps.pendingRekey
	if now.Sub(p.sentAt) < rekeyRetransmitTimeout {
		return nil
	}
	if p.retransmits >= maxRetransmissions {
		log.Warn("ike: rekey unanswered, tearing down", "peer", ps.peerName)
		// The exchange is over, so free the request window it held (RFC 7296 §2.3).
		sa.releaseRequestWindow()
		ps.pendingRekey = nil
		ps.cleanupChild(dp, bus, log)
		return errTimeout
	}
	p.retransmits++
	p.sentAt = now
	sendRaw(sa, tr, p.sentMsg, log)
	log.Debug("ike: rekey retransmit", "peer", ps.peerName, "attempt", p.retransmits)
	return nil
}

// reapStalePending drops a parallel responder handshake (pendingSA) that the peer
// abandoned before authenticating -- stuck past responderHandshakeTimeout -- so
// responderBusy and the SATable slot free up for a future re-initiation. It reads
// only pendingSA.CreatedAt (immutable) and never pendingSA.State: a pending SA that
// authenticated would have returned the owner loop via the supersede case before this
// runs, so a pending still present past the timeout is dead. Runs on the owner loop.
// RFC 7296 Section 2.4.
func (ps *PeerSession) reapStalePending(now time.Time, table *SATable, dp dataplane.Dataplane, log *slog.Logger) {
	pending := ps.getPendingSA()
	if pending == nil || now.Sub(pending.CreatedAt) <= responderHandshakeTimeout {
		return
	}
	// The dispatch goroutine may have authenticated this handshake between the select
	// picking the ticker and here (finishResponderEstablish sets State=Established +
	// pendingChild + signals supersede, but select is random when both ticker and
	// supersede are ready). Never reap an established pending: that would destroy the
	// freshly installed make-before-break child and, with the supersede token still
	// buffered, tear the old SA down too with nothing to promote. Mirrors
	// reapStaleHandshake (fsm.go). The supersede case adopts it on the next cycle.
	if pending.State == StateEstablished {
		return
	}
	log.Warn("ike: parallel responder handshake timed out, dropping", "peer", ps.peerName)
	if table != nil {
		table.Remove(pending.InitiatorSPI, pending.ResponderSPI)
	}
	ps.setPendingSA(nil)
	if pc := ps.getPendingChild(); pc != nil {
		removeChildSA(pc, dp, log)
		ps.setPendingChild(nil)
	}
	ps.responderBusy.Store(false)
}

// cleanupPendingSA removes a parallel second-slot SA (and its make-before-break Child
// SA) from the SATable and dataplane when the whole session is torn down (operator
// clear / config change), so it is not leaked. Called after Stop() has joined the
// owner goroutine, so no goroutine is still advancing pendingSA.
func (ps *PeerSession) cleanupPendingSA(table *SATable, dp dataplane.Dataplane, bus ze.EventBus, log *slog.Logger) {
	if pending := ps.getPendingSA(); pending != nil {
		if table != nil {
			table.Remove(pending.InitiatorSPI, pending.ResponderSPI)
		}
		emitSADown(bus, pending, log)
		ps.setPendingSA(nil)
	}
	if pc := ps.getPendingChild(); pc != nil {
		removeChildSA(pc, dp, log)
		ps.setPendingChild(nil)
	}
}

func (ps *PeerSession) cleanupChild(dp dataplane.Dataplane, bus ze.EventBus, log *slog.Logger) {
	child := ps.getChildSA()
	if child != nil {
		removeChildSA(child, dp, log)
		emitChildDown(bus, ps.peerName, child, log)
		emitRouteRemove(bus, child.TSRemote, log)
		log.Info("ike: tunnel routes withdrawn", "peer", ps.peerName)
		ps.setChildSA(nil)
	}
}

// resolveIfID returns the XFRM if_id for SA binding.
// The if_id must match the XFRM interface created by ipsec-2.
func resolveIfID(peer ipsec.SiteToSitePeer) uint32 {
	return peer.IfID
}

func emitChildUp(bus ze.EventBus, peerName string, child *ChildSA, log *slog.Logger) {
	if bus == nil || child == nil {
		return
	}
	evt := &ChildSAEvent{
		PeerName:    peerName,
		InboundSPI:  child.InboundSPI,
		OutboundSPI: child.OutboundSPI,
		IfID:        child.IfID,
	}
	if child.TSLocal != nil {
		evt.TSLocal = child.TSLocal.String()
	}
	if child.TSRemote != nil {
		evt.TSRemote = child.TSRemote.String()
	}
	if _, err := ChildUp.Emit(bus, evt); err != nil {
		log.Debug("ike: emit child-up failed", "error", err)
	}
}

func emitChildDown(bus ze.EventBus, peerName string, child *ChildSA, log *slog.Logger) {
	if bus == nil || child == nil {
		return
	}
	evt := &ChildSAEvent{
		PeerName:    peerName,
		InboundSPI:  child.InboundSPI,
		OutboundSPI: child.OutboundSPI,
		IfID:        child.IfID,
	}
	if child.TSLocal != nil {
		evt.TSLocal = child.TSLocal.String()
	}
	if child.TSRemote != nil {
		evt.TSRemote = child.TSRemote.String()
	}
	if _, err := ChildDown.Emit(bus, evt); err != nil {
		log.Debug("ike: emit child-down failed", "error", err)
	}
}

func emitChildRekey(bus ze.EventBus, peerName string, child *ChildSA, log *slog.Logger) {
	if bus == nil || child == nil {
		return
	}
	evt := &ChildSAEvent{
		PeerName:    peerName,
		InboundSPI:  child.InboundSPI,
		OutboundSPI: child.OutboundSPI,
		IfID:        child.IfID,
	}
	if child.TSLocal != nil {
		evt.TSLocal = child.TSLocal.String()
	}
	if child.TSRemote != nil {
		evt.TSRemote = child.TSRemote.String()
	}
	if _, err := ChildRekey.Emit(bus, evt); err != nil {
		log.Debug("ike: emit child-rekey failed", "error", err)
	}
}
