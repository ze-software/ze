// Design: plan/learned/742-ipsec-8-ikev2-child-xfrm.md -- established SA lifecycle
// RFC: rfc/short/rfc7296.md -- Child SA, DPD, rekeying after IKE_AUTH

package engine

import (
	"log/slog"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/ike/dataplane"
	"codeberg.org/thomas-mangin/ze/internal/component/ike/ipsec"
	"codeberg.org/thomas-mangin/ze/internal/component/ike/transport"
	"codeberg.org/thomas-mangin/ze/pkg/ze"
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

	// maintainSA now owns the SA: route established-SA inbound to the owner loop.
	// Cleared on return so a reconnect handshake is handled inline again.
	ps.established.Store(true)
	defer ps.established.Store(false)

	ifID := resolveIfID(peer)

	child, err := createFirstChildSA(sa, ps.espGroup, peer.LocalAddress, peer.RemoteAddress, ifID, dp, log)
	if err != nil {
		log.Warn("ike: child SA creation failed", "peer", ps.peerName, "error", err)
		return err
	}
	ps.setChildSA(child)

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
			ps.cleanupChild(dp, bus, log)
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
				// RFC 7296 §2.8: rekeyed IKE SA has new SPIs; re-key the table so
				// dispatchInbound routes to it, then swap the loop's SA.
				table.Insert(out.newSA)
				table.Remove(sa.InitiatorSPI, sa.ResponderSPI)
				oldSA := sa
				sa = out.newSA
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

			if dpd != nil && dpd.shouldSend(now) {
				sendDPD(sa, tr, dpd, log)
			}

			// RFC 7296 §1.3.2: on soft-lifetime expiry, initiate a Child SA
			// rekey via a CREATE_CHILD_SA wire exchange. Completion (key install,
			// old-SA delete, childLT reset) happens in handleOwnedInbound when the
			// response arrives; here we only start it and manage retransmission.
			if childLT != nil && childLT.softExpired(now) && ps.pendingRekey == nil {
				if old := ps.getChildSA(); old != nil {
					msg, pending, err := initiateChildRekey(sa, old)
					if err != nil {
						log.Warn("child-sa: rekey init failed", "peer", ps.peerName, "error", err)
					} else {
						sendRaw(sa, tr, msg, log)
						ps.pendingRekey = pending
						log.Info("child-sa: rekey initiated", "peer", ps.peerName, "msgid", pending.messageID)
					}
				}
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
				msg, pending, err := initiateIKERekey(sa, ikeGroup)
				if err != nil {
					log.Warn("ike-sa: rekey init failed", "peer", ps.peerName, "error", err)
				} else {
					sendRaw(sa, tr, msg, log)
					ps.pendingRekey = pending
					log.Info("ike-sa: rekey initiated", "peer", ps.peerName, "msgid", pending.messageID)
				}
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
