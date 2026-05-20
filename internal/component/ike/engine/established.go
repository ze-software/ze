// Design: plan/spec-ipsec-8-ikev2-child-xfrm.md -- established SA lifecycle
// RFC: rfc/short/rfc7296.md -- Child SA, DPD, rekeying after IKE_AUTH

package engine

import (
	"log/slog"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/ike/dataplane"
	"codeberg.org/thomas-mangin/ze/internal/component/ike/transport"
	"codeberg.org/thomas-mangin/ze/internal/component/ipsec"
	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

// runEstablished handles the post-IKE_AUTH lifecycle: child SA, DPD, rekey.
func (ps *PeerSession) runEstablished(
	sa *SA,
	peer ipsec.SiteToSitePeer,
	ikeGroup ipsec.IKEGroup,
	_ *SATable,
	tr *transport.UDPTransport,
	bus ze.EventBus,
	log *slog.Logger,
) error {
	dp := dataplane.Get()

	ifID := resolveIfID(peer)

	child, err := createFirstChildSA(sa, ps.espGroup, peer.LocalAddress, peer.RemoteAddress, ifID, dp, log)
	if err != nil {
		log.Warn("ike: child SA creation failed", "peer", ps.peerName, "error", err)
		return err
	}
	ps.childSA = child

	emitChildUp(bus, ps.peerName, child, log)

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

	return ps.maintainSA(sa, dpd, childLT, ikeLT, ikeGroup, dp, tr, bus, log)
}

// maintainSA runs the DPD + rekey loop until stopped or peer dies.
func (ps *PeerSession) maintainSA(
	sa *SA,
	dpd *dpdState,
	childLT *lifetimeState,
	ikeLT *lifetimeState,
	ikeGroup ipsec.IKEGroup,
	dp dataplane.Dataplane,
	tr *transport.UDPTransport,
	bus ze.EventBus,
	log *slog.Logger,
) error {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ps.stopCh:
			ps.cleanupChild(dp, bus, log)
			return nil
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

			if childLT != nil && childLT.softExpired(now) {
				log.Info("child-sa: lifetime expired, rekeying", "peer", ps.peerName)
				newChild, rekeyErr := rekeyChildSA(sa, ps.childSA, ps.espGroup, dp, log)
				if rekeyErr != nil {
					log.Warn("child-sa: rekey failed", "peer", ps.peerName, "error", rekeyErr)
					continue
				}
				emitChildRekey(bus, ps.peerName, newChild, log)
				ps.childSA = newChild
				childLT = newLifetimeState(ps.espGroup.Lifetime)
			}

			if childLT != nil && childLT.hardExpired(now) {
				log.Warn("child-sa: hard lifetime expired", "peer", ps.peerName)
				ps.cleanupChild(dp, bus, log)
				return errTimeout
			}

			// RFC 7296 Section 1.3.3: IKE SA rekey via CREATE_CHILD_SA.
			if ikeLT != nil && ikeLT.softExpired(now) {
				log.Info("ike-sa: lifetime expired, rekeying", "peer", ps.peerName)
				newSA, rekeyErr := rekeyIKESA(sa, ikeGroup, log)
				if rekeyErr != nil {
					log.Warn("ike-sa: rekey failed", "peer", ps.peerName, "error", rekeyErr)
					continue
				}
				oldSA := sa
				sa = newSA
				oldSA.SKKeys.Clear()
				ikeLT = newLifetimeState(ikeGroup.Lifetime)
			}

			if ikeLT != nil && ikeLT.hardExpired(now) {
				log.Warn("ike-sa: hard lifetime expired", "peer", ps.peerName)
				ps.cleanupChild(dp, bus, log)
				return errTimeout
			}
		}
	}
}

func (ps *PeerSession) cleanupChild(dp dataplane.Dataplane, bus ze.EventBus, log *slog.Logger) {
	if ps.childSA != nil {
		removeChildSA(ps.childSA, dp, log)
		emitChildDown(bus, ps.peerName, ps.childSA, log)
		ps.childSA = nil
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
	if _, err := ChildUp.Emit(bus, &ChildSAEvent{
		PeerName:    peerName,
		InboundSPI:  child.InboundSPI,
		OutboundSPI: child.OutboundSPI,
		IfID:        child.IfID,
	}); err != nil {
		log.Debug("ike: emit child-up failed", "error", err)
	}
}

func emitChildDown(bus ze.EventBus, peerName string, child *ChildSA, log *slog.Logger) {
	if bus == nil || child == nil {
		return
	}
	if _, err := ChildDown.Emit(bus, &ChildSAEvent{
		PeerName:    peerName,
		InboundSPI:  child.InboundSPI,
		OutboundSPI: child.OutboundSPI,
		IfID:        child.IfID,
	}); err != nil {
		log.Debug("ike: emit child-down failed", "error", err)
	}
}

func emitChildRekey(bus ze.EventBus, peerName string, child *ChildSA, log *slog.Logger) {
	if bus == nil || child == nil {
		return
	}
	if _, err := ChildRekey.Emit(bus, &ChildSAEvent{
		PeerName:    peerName,
		InboundSPI:  child.InboundSPI,
		OutboundSPI: child.OutboundSPI,
		IfID:        child.IfID,
	}); err != nil {
		log.Debug("ike: emit child-rekey failed", "error", err)
	}
}
