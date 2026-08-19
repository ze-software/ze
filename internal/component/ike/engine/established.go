// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- established SA lifecycle
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

	// RFC 7296 Section 2.12 MUST: an endpoint closing a connection forgets the keys and
	// everything that could recompute them. This function's return IS that close, for
	// every reason maintainSA can end (operator clear, peer Delete, DPD timeout,
	// lifetime, Message ID exhaustion, supersede), so one erase here covers them all.
	//
	// It reads ownedSA rather than the argument, because an IKE SA rekey swaps the loop
	// onto a replacement and stores it there. Erasing the argument would leave the SA
	// that actually finished the session holding its keys. The retired SA of that swap
	// is erased at the swap itself.
	//
	// Registered AFTER the store above, so it runs BEFORE it: defers unwind last-first.
	defer func() { ps.ownedSA.Load().forgetKeys() }()

	// The same close, for the two holders forgetKeys cannot reach. Both live on the
	// SESSION rather than on the SA, so erasing the SA above leaves either one behind,
	// and the session outlives the SA: ps.run loops runOnce on this same PeerSession
	// (reconcile.go), so what survives here is carried into the next reconnect cycle.
	//
	// ps.pendingRekey holds the Diffie-Hellman private value of a rekey this session
	// started and never got an answer to. It is exactly "something that could recompute
	// the keys" in Section 2.12's sense: the private half of the exchange that would
	// have derived the replacement SA's SKEYSEED.
	//
	// ps.pendingIKESwap holds a whole IKE SA, SK_* material included, built while
	// answering the peer's IKE-SA rekey and held make-before-break until the peer
	// deletes the old SA (RFC 7296 Section 2.8, inbound.go). A session that ends before
	// that Delete arrives keeps a keyed SA nobody will promote, and the next cycle then
	// promotes it: handleInformationalOwned swaps the loop onto ps.pendingIKESwap on
	// the peer's first IKE Delete, whichever cycle built it. Releasing it here forgets
	// its keys and empties the slot in the one call.
	//
	// Safe to touch here, and only here. Both fields are owned without a lock by the
	// maintainSA goroutine (reconcile.go), and this defer runs after maintainSA has
	// returned. Every path that ENDS an exchange normally already clears them
	// (inbound.go, serviceRekeyRetransmit); this covers the paths that end the
	// SESSION with an exchange still outstanding.
	defer func() {
		ps.pendingRekey.clear()
		ps.pendingRekey = nil
		ps.setPendingIKESwap(nil)
	}()

	ifID := resolveIfID(peer)

	var child *ChildSA
	if sa.IsInitiator {
		var err error
		child, err = initiatorFirstChildSA(sa, peer, ifID, dp, log)
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
	//
	// The keepalive holds the NAT binding open, so it MUST leave from the same port
	// the SA's traffic leaves from. RFC 7296 Section 2.23 puts that at 4500. A
	// keepalive from port 500 refreshes a mapping no traffic uses.
	//
	// The destination is the SA's stored endpoint. The keepalive is self-initiated,
	// so no request corroborates any observation of its own.
	if sa.NATDetected {
		out, _ := sa.sendPath(tr)
		remote := sa.remoteUDPAddr()
		switch {
		case out == nil || remote == nil:
			log.Warn("ike: NAT detected but no keepalive path, the NAT binding will expire",
				"peer", ps.peerName, "local-port", sa.localPort)
		default:
			ka := transport.NewKeepalive(out.Conn(), remote, transport.DefaultKeepaliveInterval, log)
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
	// RFC 7296 Section 2.8 forbids USING an SA whose lifetime has expired, and the send
	// path is what has to refuse. Publishing the deadline on the SA is what lets it:
	// the loop below notices expiry only on a tick, and a send can be reached between
	// two ticks. A nil ikeLT means no configured lifetime, and clears the deadline.
	if ikeLT != nil {
		sa.setHardExpiry(ikeLT.hardTime)
	} else {
		sa.setHardExpiry(time.Time{})
	}

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
				ps.sayGoodbye(sa, tr, dp, log)
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
			// RFC 7296 Section 4: the peer refused a rekey with NO_ADDITIONAL_SAS, so
			// the required fallback is to delete the old SA and create a new one.
			// Dropping the Child SA and ending the owner loop is that fallback: the
			// caller's reconnect path runs IKE_SA_INIT and IKE_AUTH again, and those
			// build a fresh Child SA. Returning errTimeout takes the same exit the hard
			// lifetime uses, so the backoff and the teardown are the established ones.
			if out.reestablish {
				ps.cleanupChild(dp, bus, log)
				return errTimeout
			}
			// Clear the DPD wait on an in-window authenticated inbound (peerAlive), or
			// on an authenticated INFORMATIONAL response whose message ID matches the
			// outstanding probe (matchesProbe rejects replays / out-of-window acks).
			if out.peerAlive || (out.dpdResp && dpd.matchesProbe(out.dpdRespMsgID)) {
				// The probe is abandoned here, so the request window it holds is
				// retired with it. A peer REQUEST sets peerAlive too, and that path
				// frees no window of its own. RFC 7296 Section 2.3 forbids one request
				// answering another. Without this step the window stays held for the
				// whole requestWindowTimeout, while timedOut and shouldRetransmit are
				// both off. Read awaitingReply BEFORE handleDPDResponse clears it.
				if dpd.awaitingReply() {
					sa.retireRequest(dpd.probeMsgID)
				}
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
				// The session pointer follows the swap too. TerminatePeerSA and
				// TerminateAllSAs remove the SA that ps.getSA returns. A pointer
				// left on the retired SA deletes a key that is already gone, and
				// leaves the live tunnel in the table.
				ps.setSA(sa)
				table.Insert(sa)
				table.Remove(oldSA.InitiatorSPI, oldSA.ResponderSPI)
				// RFC 7296 Section 2.12: the retired SA is closed here, so it forgets
				// its keys AND the DH private value, nonces and EAP key that could
				// recompute them. Clearing SKKeys alone left SK_d's own inputs behind.
				oldSA.forgetKeys()
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

			// RFC 7296 Section 2.2 MUST: "In the unlikely event that Message IDs grow
			// too large to fit in 32 bits, the IKE SA MUST be closed or rekeyed." The
			// rekey branch further down answers the threshold. This one answers the
			// ceiling, where no id is left to carry a rekey request, so the SA closes.
			// Without it an exhausted SA would go quiet instead: reserveRequestWindow
			// refuses every request, including the DPD probe that detects a dead peer.
			if sa.msgIDExhausted && sa.State != StateDead {
				log.Warn("ike: message id space exhausted, closing the SA",
					"peer", ps.peerName, "next-msgid", sa.NextMsgID,
					"expected-msgid", sa.ExpectedMsgID)
				sa.State = StateDead
			}

			if sa.State == StateDead {
				log.Info("ike: SA marked dead by peer", "peer", ps.peerName)
				ps.cleanupChild(dp, bus, log)
				// NON-NIL ON PURPOSE. PeerSession.run (reconcile.go) reads a nil
				// return as a clean shutdown and ends the session goroutine, so this
				// path used to leave the tunnel down until the next config apply. The
				// operator `clear` above returns nil correctly, because
				// TerminateAllSAs deletes the peer and calls reEstablish to rebuild
				// it. A peer's Delete rebuilds nothing, so the reconnect has to come
				// from here. This is the exit the Section 4 fallback already takes.
				return errSADeletedByPeer
			}

			if dpd != nil && dpd.timedOut(now) {
				log.Warn("dpd: peer dead", "peer", ps.peerName,
					"action", dpd.action, "attempts", dpd.retries+1)
				ps.cleanupChild(dp, bus, log)
				return errTimeout
			}

			ps.serviceRequestRetransmit(sa, dpd, tr, now, log)
			ps.serviceRequestWindow(sa, dpd, now, log)

			// RFC 7296 Section 2.4: the peer has failed only once REPEATED attempts
			// have gone unanswered for a timeout period. One lost datagram is not
			// that, so an unanswered probe is repeated inside the liveness budget
			// before the branch above ends the SA.
			if dpd != nil && dpd.shouldRetransmit(now) {
				retransmitDPD(sa, tr, dpd, now, log)
			}

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

			// RFC 7296 Section 2.8 is unconditional: once the lifetime expires the SA
			// MUST NOT be used, and an in-flight rekey does not extend it. The rekey
			// gets its room BEFORE the hard time instead -- newLifetimeState places the
			// soft trigger at least a full retransmit budget earlier -- so a rekey that
			// is still unanswered here has already exhausted that room.
			if childLT != nil && childLT.hardExpired(now) {
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

			// RFC 7296 Section 2.2 offers two remedies for a counter that runs out.
			// The SA is "closed or rekeyed". This branch takes the second one first.
			// The counter is inside the headroom below the ceiling, so the SA is
			// rekeyed while ids remain to carry the exchange. Section 2.18 starts the
			// replacement SA at 0.
			if sa.msgIDNearExhaustion() && ps.pendingRekey == nil {
				ps.startIKERekey(sa, ikeGroup, tr, log)
			}

			// Unconditional for the same reason as the Child SA above (Section 2.8).
			if ikeLT != nil && ikeLT.hardExpired(now) {
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

// temporaryFailureBackoff is how long a rekey waits after the peer answered it with a
// TEMPORARY_FAILURE notify.
//
// RFC 7296 §2.25 states two things about that answer. The recipient MUST NOT retry
// the operation at once, and it MUST wait for the peer to finish the operation that
// caused the condition. The recipient is then free to retry over a period of several
// minutes. The checklist row RFC7296-2.25-1 carries the sentence verbatim.
//
// The RFC names no number, so this one is chosen. 60 seconds is 60 ticks of the owner
// loop rather than one tick. It also puts about five attempts inside the
// several-minute period the RFC describes.
//
// A default lifetime leaves room for it. ze-ipsec-conf.yang gives an ESP group 3600
// seconds, and lifetimeJitter takes up to 10% off the soft time. A shorter configured
// lifetime can close that gap. There hardExpired stays the backstop RFC 7296 §2.8
// already requires.
const temporaryFailureBackoff = 60 * time.Second

// rekeyHeld reports whether a TEMPORARY_FAILURE answer is still holding a rekey back.
// A zero instant means no answer has ever held it. RFC 7296 §2.25.
func rekeyHeld(until, now time.Time) bool {
	return !until.IsZero() && now.Before(until)
}

// startChildRekey begins a Child SA rekey. RFC 7296 §2.3 allows one self-initiated
// request per SA, so the window is reserved before initiateChildRekey reads
// NextMsgID. A held window defers the rekey: pendingRekey stays nil and the soft
// lifetime still stands, so the next tick raises the rekey again. RFC 7296 §1.3.2.
func (ps *PeerSession) startChildRekey(sa *SA, tr *transport.UDPTransport, log *slog.Logger) {
	old := ps.getChildSA()
	if old == nil {
		return
	}
	// RFC 7296 §2.25: the peer answered an earlier attempt with TEMPORARY_FAILURE.
	// This one waits out the hold. The soft lifetime is a level trigger and still
	// stands, so a tick after the hold raises the rekey again. That repeat is the
	// retry over several minutes the same section permits.
	if rekeyHeld(ps.childRekeyHoldUntil, time.Now()) {
		log.Debug("child-sa: rekey held, the peer answered with TEMPORARY_FAILURE",
			"peer", ps.peerName, "until", ps.childRekeyHoldUntil)
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
	// RFC 7296 §2.25, as in startChildRekey. The two holds are separate, so a peer busy
	// with a Child SA rekey does not stop an IKE SA rekey and the reverse.
	if rekeyHeld(ps.ikeRekeyHoldUntil, time.Now()) {
		log.Debug("ike-sa: rekey held, the peer answered with TEMPORARY_FAILURE",
			"peer", ps.peerName, "until", ps.ikeRekeyHoldUntil)
		return
	}
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

// serviceRequestRetransmit repeats the request that holds the window, when that request
// kept a copy of itself. It runs BEFORE serviceRequestWindow, so a repeat is made while
// the window still stands.
//
// RFC 7296 Section 2.1: a retransmission carries the Message ID of the request it
// repeats. A repeat therefore spends no further id.
//
// Only a request with no retransmission machine of its own arms the slot. That is the
// INVALID_MESSAGE_ID notify and every Delete (writeDelete, delete.go). The rekey and
// the DPD probe keep their own copies, and the guard below excludes both, so neither is
// repeated twice.
func (ps *PeerSession) serviceRequestRetransmit(sa *SA, dpd *dpdState, tr *transport.UDPTransport, now time.Time, log *slog.Logger) {
	if ps.pendingRekey != nil || dpd.awaitingReply() {
		return
	}
	if !sa.shouldRetransmitRequest(now) {
		return
	}
	sendRaw(sa, tr, sa.requestMsg, log)
	sa.noteRequestRetransmit(now)
	log.Debug("ike: repeated the outstanding request", "peer", ps.peerName,
		"msgid", sa.requestMsgID, "attempt", sa.requestAttempts)
}

// serviceRequestWindow ends an SA whose outstanding request was never answered, once
// every repeat of it has gone out and requestWindowTimeout has passed.
//
// RFC 7296 Section 2.1 MUST: "IKE is a reliable protocol: the initiator MUST retransmit
// a request until it either receives a corresponding response or deems the IKE SA to
// have failed. In the latter case, the initiator discards all state associated with the
// IKE SA and any Child SAs that were negotiated using that IKE SA." The section offers
// two exits and no third. Freeing the window and carrying on was that third exit: the
// request was forgotten while the SA it belonged to kept running.
//
// StateDead is how this loop discards that state. Its own branch runs at the top of the
// next tick, calls cleanupChild, and returns errSADeletedByPeer, so the session
// reconnects rather than going quiet.
//
// A rekey and a DPD probe each end their own hold. The first uses its retransmit
// budget, and the second uses the liveness budget. Both already fail the SA when they
// run out, so the guard below leaves them to it. What reaches here is a Delete or the
// INVALID_MESSAGE_ID notify.
func (ps *PeerSession) serviceRequestWindow(sa *SA, dpd *dpdState, now time.Time, log *slog.Logger) {
	if ps.pendingRekey != nil || dpd.awaitingReply() {
		return
	}
	if !sa.requestWindowStale(now) {
		return
	}
	log.Warn("ike: the peer never answered our request, failing the SA",
		"peer", ps.peerName, "msgid", sa.requestMsgID, "attempts", sa.requestAttempts)
	sa.releaseRequestWindow()
	sa.State = StateDead
}

// goodbyeWindowWait bounds how long an operator `clear` waits for the answer to a
// request that is already outstanding, before it gives up on saying goodbye.
//
// A wait is needed at all because RFC 7296 Section 2.3 MUST: "An IKE endpoint MUST wait
// for a response to each of its messages before sending a subsequent message". Ze
// declares a window of one, so the goodbye Delete cannot ride out beside an unanswered
// liveness probe. Abandoning the probe locally does not change what the endpoint has
// received, so it does not make the second request legal.
//
// It is SHORT because StopGraceful (reconcile.go) blocks the operator's command on this
// loop's return. An answer to a probe on a live peer arrives in one round trip. A peer
// that has not answered in two seconds is the case dead-peer detection exists for, and
// a Delete it cannot answer is worth nothing.
const goodbyeWindowWait = 2 * time.Second

// sayGoodbye tells the peer this SA is being destroyed (RFC 7296 Section 1.4), and
// waits first when Section 2.3's one request window is still held.
//
// This is the only Delete sender that can meet a held window. The two others run on the
// rekey response that freed it (inbound.go). A queue does not serve this one. The owner
// loop returns on the statement after this call, so no later tick can drain a queue.
func (ps *PeerSession) sayGoodbye(sa *SA, tr *transport.UDPTransport, dp dataplane.Dataplane, log *slog.Logger) {
	if sa.requestOutstanding {
		log.Debug("ike: waiting for the outstanding request before saying goodbye",
			"peer", ps.peerName, "msgid", sa.requestMsgID, "bound", goodbyeWindowWait)
		ps.waitForRequestWindow(sa, tr, dp, log)
	}
	if sa.requestOutstanding {
		log.Info("ike: goodbye not sent, the peer never answered our last request",
			"peer", ps.peerName, "msgid", sa.requestMsgID)
		return
	}
	ps.sendDeleteIKE(sa, tr, log)
}

// waitForRequestWindow reads inbound datagrams until the outstanding request is
// answered or goodbyeWindowWait passes. Only an AUTHENTICATED answer frees the window
// (answerAuthenticatedResponse, msgid.go), so a forged datagram cannot end the wait.
//
// It runs on the owner goroutine, the only reader of ps.inbound, so the normal loop's
// exclusive access to the SA still holds.
//
// The session is being destroyed, so an exchange that completes during the wait
// produces state nothing will use. An IKE SA from a rekey response is forgotten here
// rather than adopted. The caller's cleanupChild removes whatever Child SA is current.
func (ps *PeerSession) waitForRequestWindow(sa *SA, tr *transport.UDPTransport, dp dataplane.Dataplane, log *slog.Logger) {
	deadline := time.NewTimer(goodbyeWindowWait)
	defer deadline.Stop()
	for {
		select {
		case pkt := <-ps.inbound:
			out := ps.handleOwnedInbound(sa, pkt, tr, dp, log)
			if out.newSA != nil {
				// RFC 7296 Section 2.12: an SA nobody will adopt still holds SK_*
				// and the material that recomputes them.
				out.newSA.forgetKeys()
			}
			if !sa.requestOutstanding {
				return
			}
		case <-deadline.C:
			return
		}
	}
}

// sendRaw sends already-built wire bytes on the SA's OWN send path.
//
// The destination is sa.remoteUDPAddr. That is the endpoint of the last message this
// SA authenticated, or the configured remote when none has arrived. It is never the
// address of a datagram in hand.
//
// RFC 7296 Section 2.11 asks the response to reach the address the request came from.
// adoptAuthenticatedEndpoint stores that address before the response is built, so the
// response follows an AUTHENTICATED observation and not an attacker-chosen one.
//
// The source port follows sa.sendPath. RFC 7296 Section 2.23 MUST: an endpoint that
// discovers a NAT "MUST send all subsequent traffic from port 4500". This is the one
// site every post-establishment sender shares. Floating it here therefore floats the
// DPD probe, both rekey exchanges, the Delete, each error notify, and each cached
// replay. Before this, only the IKE_AUTH senders framed themselves for a NAT.
func sendRaw(sa *SA, tr *transport.UDPTransport, msg []byte, log *slog.Logger) {
	out, natT := sa.sendPath(tr)
	if out == nil {
		log.Warn("ike: no send path for the SA, message dropped",
			"peer", sa.PeerName, "local-port", sa.localPort)
		return
	}
	remote := sa.remoteUDPAddr()
	if remote == nil {
		return
	}
	if natT {
		// RFC 3948 Section 2.2: IKE on port 4500 carries the four-octet non-ESP marker.
		msg = transport.AddNonESPMarker(msg)
	}
	if err := out.Send(msg, remote); err != nil {
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
		// RFC 7296 Section 2.12: an abandoned IKE rekey still holds the DH private
		// value it generated for the request. Dropping the pointer leaves it in
		// memory for the garbage collector to move around; clear() zeroes it, which
		// is what every other path that ends a pendingRekey does (inbound.go).
		p.clear()
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
	// RFC 7296 Section 2.12: a half-open SA that reached IKE_SA_INIT already holds a
	// shared secret and the nonces behind it, so an abandoned handshake forgets them
	// exactly as a closed connection does.
	pending.forgetKeys()
	ps.setPendingSA(nil)
	if pc := ps.getPendingChild(); pc != nil {
		// The LIVE Child SA answers to the same policies whenever the parallel
		// handshake negotiated the same selector, which the ordinary 0.0.0.0/0
		// site-to-site selector always does. Dropping them here would blackhole the
		// tunnel that is still up, so they are kept for the survivor.
		removeChildSAExcept(pc, firstSharingSelector(pc, ps.getChildSA()), dp, log)
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
		// RFC 7296 Section 2.12, as in reapStalePending above.
		pending.forgetKeys()
		ps.setPendingSA(nil)
	}
	if pc := ps.getPendingChild(); pc != nil {
		removeChildSA(pc, dp, log)
		ps.setPendingChild(nil)
	}
}

func (ps *PeerSession) cleanupChild(dp dataplane.Dataplane, bus ze.EventBus, log *slog.Logger) {
	// The session is going away, so no INFORMATIONAL response will ever complete an
	// outstanding Delete. The records are dropped BEFORE the removals below, so nothing
	// names a pair these two calls have already freed.
	ps.abandonOwnDeletes()
	// Two Child SAs can outlive this one on the SAME pair of kernel policies, and the
	// removals below must keep the policies for either of them.
	//
	// The first is a rekey this session responded to, which leaves the retired pair
	// installed until the peer's Delete arrives (make-before-break).
	//
	// The second is a parallel RE-INITIATION, where finishResponderEstablish
	// (responder.go) has installed the new handshake's Child SA in the pending slot. It
	// UPSERTS the same selector, so the kernel holds one policy per direction shared by
	// both pairs. Removing the live pair's policy here left resolvePendingAfterOwnerLoop
	// (fsm.go) promoting a Child SA with states and NO policy: outbound traffic then left
	// the box in the clear and inbound ESP was dropped.
	//
	// THIS FUNCTION IS NOT THE SUPERSEDE ARM'S ALONE. maintainSA calls it from eight
	// exits -- operator clear, supersede, the Section 4 reestablish fallback, the SA
	// marked dead, DPD timeout, both hard lifetimes, and an unanswered IKE rekey -- and a
	// pending child can be installed on any of them. An earlier version of this comment
	// named only the supersede arm, which read as a guarantee that the OTHER seven could
	// not meet a shared policy. That reading is what let the peer-Delete path
	// (closeDesignatedChildSAs, delete.go) reach the reestablish exit having already
	// removed both shared policies, which this function cannot repair: it reinstalls
	// nothing, and by then there is nothing left to keep.
	pending := ps.getPendingChild()
	live := ps.getChildSA()
	if old := ps.supersededChild; old != nil {
		ps.supersededChild = nil
		// Whichever survivor shares the selector keeps the policies alive; they are
		// removed by that survivor's own teardown -- once, in total.
		removeChildSAExcept(old, firstSharingSelector(old, live, pending), dp, log)
	}
	if live != nil {
		removeChildSAExcept(live, firstSharingSelector(live, pending), dp, log)
		emitChildDown(bus, ps.peerName, live, log)
		emitRouteRemove(bus, live.TSRemote, log)
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
