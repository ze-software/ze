// Design: docs/architecture/ike/ipsec-14-responder.md -- accept-in-parallel, and a reauthentication the gateway of an `initiate` peer starts
// RFC: rfc/short/rfc7296.md -- Rekeying the IKE SA versus Reauthentication (Section 2.8.3)
// Related: register.go -- tryResponderSAInit, the admission gate that asks matchReauthenticatingPeer
// Related: responder.go -- finishResponderEstablish, which signals supersede only once the successor's Child SA is installed
// Related: fsm.go -- runInitiator and runResponder, the two run loops that adopt or retire the successor
//
// RFC 7296 Section 2.8.3: "Reauthentication is done by creating a new IKE SA from
// scratch (using IKE_SA_INIT/IKE_AUTH exchanges, without any REKEY_SA Notify
// payloads), creating new Child SAs within the new IKE SA (without REKEY_SA Notify
// payloads), and finally deleting the old IKE SA (which deletes the old Child SAs as
// well)."
//
// `initiate` and `respond` say who dials first, not who may create the next IKE SA.
// This file holds what an `initiate` session needs to receive a reauthentication its
// gateway starts, and the second-slot resolution both run loops share.
package engine

import (
	"errors"
	"log/slog"
	"net"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/pkg/ze"
)

// errSuccessorNotAdopted reports that the owner loop relinquished its IKE SA to a
// successor and the session could not adopt it, so the session reconnects.
var errSuccessorNotAdopted = errors.New("ike: the IKE SA was superseded and its successor could not be adopted")

// matchReauthenticatingPeer finds a running `initiate` peer whose established IKE SA
// talks to the packet source, or nil, so the other end of that SA can reauthenticate
// it by creating a new IKE SA. tryResponderSAInit asks it after matchResponderPeer
// found no `respond` peer, so the admission gates that follow (COOKIE, one half-open
// handshake) apply unchanged.
//
// RFC 7296 Section 2.8.3: "While creation of a new IKE SA can be initiated by either
// party (initiator or responder in the original IKE SA), the use of EAP and/
// or Configuration payloads means in practice that reauthentication has
// to be initiated by the same party as the original IKE SA."
//
// A reauthentication exists only relative to an IKE SA, so a peer whose owner loop
// holds none (ownedSA nil) still refuses every unsolicited initiation. The source must
// be the address that SA talks to (saEndpoint, which the owner loop publishes from the
// authenticated peer endpoint), never only the configured remote: an `initiate` peer's
// remote names where Ze dials, not who may dial Ze. An EAP peer is refused: Ze was the
// EAP peer of the original SA and is no EAP server for its gateway. Ze as initiator
// sends no CFG_REQUEST, so the Configuration payload half never applies to it.
//
// Called on the dispatch goroutine. It reads the immutable session config under the
// peers lock and the atomic ownedSA and saEndpoint pointers, as establishedSessionFrom
// does.
func matchReauthenticatingPeer(remoteAddr *net.UDPAddr, log *slog.Logger) *PeerSession {
	if remoteAddr == nil {
		return nil
	}
	peersMu.RLock()
	defer peersMu.RUnlock()
	for _, ps := range activePeersMap {
		if ps.peerCfg.ConnectionType != ipsec.ConnectionInitiate {
			continue
		}
		if ps.ownedSA.Load() == nil {
			continue
		}
		endpoint := ps.saEndpoint.Load()
		if endpoint == nil {
			continue
		}
		if !endpoint.IP.Equal(remoteAddr.IP) {
			continue
		}
		if authModeIsEAP(ps.peerCfg.Auth.Mode) {
			log.Warn("ike: refusing a reauthentication the gateway started: this peer authenticates with EAP, and RFC 7296 Section 2.8.3 says reauthentication then has to be initiated by the same party as the original IKE SA",
				"peer", ps.peerName, "src", remoteAddr, "auth", ps.peerCfg.Auth.Mode.String(),
				"fix", "configure the gateway to leave reauthentication to Ze (strongSwan: reauth_time = 0s on this connection); Ze reconnects when the gateway deletes the IKE SA")
			return nil
		}
		return ps
	}
	return nil
}

// authModeIsEAP reports whether a first-round authentication mode runs EAP. A second
// round is always pre-shared-secret or x509 (ipsec.SecondRoundAuth), so the first
// round decides. The modes are named one by one so the exhaustive linter refuses a
// mode added to the enum and not answered here.
func authModeIsEAP(mode ipsec.AuthMode) bool {
	switch mode {
	case ipsec.AuthEAPTLS, ipsec.AuthEAPMSCHAPv2, ipsec.AuthEAPMD5:
		return true
	case ipsec.AuthPreSharedSecret, ipsec.AuthX509, ipsec.AuthUnknown:
		return false
	}
	panic("BUG: unknown authentication mode")
}

// adoptPeerStartedSuccessor runs after an `initiate` session's owner loop returned, and
// answers the IKE SA the session continues on, or nil when the cycle ends. A successor
// the gateway started (its reauthentication, admitted by matchReauthenticatingPeer) is
// adopted when it authenticated and its Child SA is installed. A handshake still in
// flight is kept in the second slot across the reconnect, since the successor the
// gateway started wins over Ze's reconnect (relinquishToSuccessor); anything else there
// (a failed handshake, or a successor held without a Child SA) is retired, because a
// session never stays up with no Child SA. It is adopted whatever ended
// the owner loop, supersede or the gateway's Delete of the old IKE SA, because the two
// race: strongSwan deletes the old SA as soon as its reauthentication completes
// (ike_reauth_complete.c build_i).
//
// RFC 7296 Section 2.8.3: "While creation of a new IKE SA can be initiated by either
// party (initiator or responder in the original IKE SA), the use of EAP and/
// or Configuration payloads means in practice that reauthentication has
// to be initiated by the same party as the original IKE SA."
func (ps *PeerSession) adoptPeerStartedSuccessor(table *SATable, dp dataplane.Dataplane, bus ze.EventBus, log *slog.Logger) *SA {
	if ps.stopped() {
		ps.cleanupPendingSA(table, dp, bus, log)
		return nil
	}
	ps.childLifecycleMu.Lock()
	defer ps.childLifecycleMu.Unlock()
	ps.crossedSuccessor = nil
	pending := ps.getPendingSA()
	if pending == nil {
		ps.responderBusy.Store(false)
		return nil
	}
	if ps.getPendingChild() == nil {
		// A handshake still in flight is the gateway's reauthentication crossing the
		// reconnect this cycle's end starts. It stays in the second slot, its half-open
		// gate still held: Ze never ends the successor the gateway started, which wins
		// whichever of the two completes first (runInitiator, relinquishToSuccessor).
		// The reaper at the top of the next cycle bounds one the gateway abandons. A
		// handshake that already failed (StateDead) is no longer in flight, and is
		// retired below.
		if pending.State != StateEstablished && pending.State != StateDead {
			log.Info("ike: keeping the gateway's reauthentication in flight across the reconnect",
				"peer", ps.peerName, "ispi", SPIHex(pending.InitiatorSPI))
			return nil
		}
		ps.responderBusy.Store(false)
		ps.retireHeldSuccessor(pending, table, log)
		return nil
	}
	ps.responderBusy.Store(false)
	ps.promotePendingSA(pending)
	log.Info("ike: adopting the IKE SA the gateway created to reauthenticate, retiring the old one",
		"peer", ps.peerName, "ispi", SPIHex(pending.InitiatorSPI), "rspi", SPIHex(pending.ResponderSPI))
	return pending
}

// promotePendingSA moves the second slot into the primary one: pending becomes the
// session's SA and its Child SA, when it holds one, the session's Child SA. The caller
// holds childLifecycleMu.
func (ps *PeerSession) promotePendingSA(pending *SA) {
	ps.setPendingSA(nil)
	if pc := ps.getPendingChild(); pc != nil {
		ps.setChildSA(pc)
		ps.setPendingChild(nil)
		// The promoted child took its ports from the pending IKE SA's own endpoint, so a
		// rekey owed for the old pair no longer describes the current child.
		ps.rebindRekeyOwed = false
	}
	ps.setSA(pending)
}

// retireHeldSuccessor ends a second-slot SA that will not be adopted: it tells the peer
// when the SA authenticated (a half-open one has no keys to protect a Delete with),
// takes it out of the SATable and forgets its keys. The caller holds childLifecycleMu
// and has checked that no Child SA is staged for it.
//
// RFC 7296 Section 1.4.1: "Deleting an IKE SA implicitly closes any
// remaining Child SAs negotiated under it."
func (ps *PeerSession) retireHeldSuccessor(pending *SA, table *SATable, log *slog.Logger) {
	if pending.State == StateEstablished {
		log.Warn("ike: the established IKE SA ended while its re-initiated successor held no Child SA, deleting the successor",
			"peer", ps.peerName, "ispi", SPIHex(pending.InitiatorSPI), "rspi", SPIHex(pending.ResponderSPI),
			"fix", "make the peer's traffic selectors and ESP proposals match this peer's configuration; the session reconnects")
		ps.sendDeleteIKE(pending, ps.ike, log)
	}
	ps.forgetSecondSlot(pending, table)
}

// forgetSecondSlot empties the second slot of an SA that will not be adopted: it leaves
// the SATable, so no later datagram finds it, and forgets its keys. The caller holds
// childLifecycleMu and decides whether the peer is told and whether the half-open gate
// frees.
//
// RFC 7296 Section 2.12: the SA is closed here, so it forgets its keys.
func (ps *PeerSession) forgetSecondSlot(pending *SA, table *SATable) {
	if table != nil {
		table.Remove(pending.InitiatorSPI, pending.ResponderSPI)
	}
	pending.forgetKeys()
	ps.setPendingSA(nil)
}

// successorReady reports whether the second slot holds a successor that authenticated
// with its Child SA installed, the only state finishResponderEstablish signals supersede
// for. A Child SA is staged only after authentication, so the SA's state, which the
// dispatch goroutine writes, is not read here.
func (ps *PeerSession) successorReady() bool {
	return ps.getPendingSA() != nil && ps.getPendingChild() != nil
}

// recordCrossing notes that the owner's IKE SA has just become a replacement Ze started
// (its reconnect in runInitiator, or its own IKE SA rekey in maintainSA), against the
// peer-started successor in the second slot at that moment, or nil when there is none.
// relinquishToSuccessor deletes that replacement if the same successor supersedes.
// It runs on the session goroutine, which owns crossedSuccessor.
func (ps *PeerSession) recordCrossing() {
	ps.crossedSuccessor = ps.getPendingSA()
}

// relinquishToSuccessor ends the owner loop's hold on sa once a successor the peer
// started has authenticated with its Child SA installed (the supersede signal). Only
// the old Child SA is removed, keeping the policies the successor's Child SA shares
// (make before break). The run loop then promotes the successor.
//
// RFC 7296 Section 2.8.3: "IKEv2 does not have any special support for
// reauthentication." Nothing in the RFC picks a winner when the peer's reauthentication
// crosses a replacement Ze started (its own IKE SA rekey, or its reconnect), so the
// peer-started successor wins, decided by who started each SA and never by completion
// order: strongSwan deletes the old IKE SA once its reauthentication completes and never
// deletes a duplicate Ze created (ike_reauth_complete.c, ike_sa_manager.c
// check_uniqueness). When sa is such a crossing replacement, recorded in
// ps.crossedSuccessor against this same successor, Ze deletes it, because the peer would
// otherwise keep it beside the successor. The IKE SA the peer reauthenticated is the
// peer's to delete, and is not deleted here.
//
// RFC 7296 Section 1.4.1: "Deleting an IKE SA implicitly closes any
// remaining Child SAs negotiated under it."
//
// Runs on the owner goroutine, which owns sa's message IDs and keys.
func (ps *PeerSession) relinquishToSuccessor(sa *SA, table *SATable, tr *transport.UDPTransport, dp dataplane.Dataplane, bus ze.EventBus, log *slog.Logger) {
	crossed := ps.crossedSuccessor
	ps.crossedSuccessor = nil
	if crossed != nil && crossed == ps.getPendingSA() {
		log.Info("ike: the peer's reauthentication crossed a replacement Ze started, deleting Ze's replacement and keeping the peer's",
			"peer", ps.peerName, "ispi", SPIHex(sa.InitiatorSPI), "rspi", SPIHex(sa.ResponderSPI),
			"successor-ispi", SPIHex(crossed.InitiatorSPI))
		ps.sayGoodbye(sa, tr, dp, log)
		if table != nil {
			table.Remove(sa.InitiatorSPI, sa.ResponderSPI)
		}
	}
	ps.cleanupChild(dp, bus, log)
	log.Info("ike: superseded by a re-initiated SA, relinquishing owner loop", "peer", ps.peerName)
}
