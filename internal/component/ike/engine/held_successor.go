// Design: docs/architecture/ike/ipsec-14-responder.md -- accept-in-parallel, and the successor held beside the owner without a Child SA
// RFC: rfc/short/rfc7296.md -- Rekeying the IKE SA versus Reauthentication (Section 2.8.3), Section 2.25
// Related: responder.go -- finishResponderEstablish, which holds a successor whose IKE_AUTH Child SA was refused
// Related: reauth_receive.go -- retireHeldSuccessor, adoptPeerStartedSuccessor, the run-loop half of the held successor
// Related: fsm.go -- handleInbound, which hands a held successor's message here
//
// A re-initiated IKE SA that authenticated with no Child SA is held in the second slot
// (pendingSA) beside the IKE SA the owner loop runs, never adopted: nothing exists to
// switch to (make before break). The peer created it, so the peer may still use it.
// It can create the Child SA it lacked, probe it for liveness, or delete it, and
// RFC 7296 Section 1.4 has every one of those requests answered. This file answers
// them until the session adopts or retires the successor.
package engine

import (
	"log/slog"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/component/ike/wire"
)

// handleHeldSuccessorInbound answers a request the peer sends on the session's held
// successor, and reports whether sa is one. A false answer means sa is not held and the
// caller MUST pass the message to handleResponderInbound as before; a true one means
// the message was consumed here, answered or dropped, and the caller MUST NOT pass it
// on.
//
// A successor is held while it is the second-slot SA, established, staged with no Child
// SA, and an owner loop runs the session's IKE SA. A successor whose Child SA is
// staged has already signaled supersede, so its messages wait for the adoption the
// owner loop is completing: the peer retransmits (RFC 7296 Section 2.1) and the
// adopted SA's owner loop answers, from the same Message ID window.
//
// It runs on the dispatch goroutine under childLifecycleMu, as the IKE_AUTH that
// created the successor did. Until the session adopts it, the second slot belongs to
// the dispatch path: routeInbound hands an owner loop only the SA it owns, and the run
// loops read the slot under the same lock (adoptPeerStartedSuccessor,
// resolvePendingAfterOwnerLoop).
func (ps *PeerSession) handleHeldSuccessorInbound(sa *SA, msg *wire.Message, pkt transport.Packet, table *SATable, tr *transport.UDPTransport, log *slog.Logger) bool {
	ps.childLifecycleMu.Lock()
	defer ps.childLifecycleMu.Unlock()
	if !ps.holdsSuccessor(sa) {
		return false
	}
	if msg.Header.Flags&wire.FlagResponse != 0 {
		// Ze raises no request on a successor it has not adopted, so a response answers
		// nothing it asked. RFC 7296 Section 3.1 forbids answering a response.
		log.Debug("ike: response on the held successor matches no request, dropped", "peer", ps.peerName)
		return true
	}
	sa.replyLocal, sa.replyRemote, sa.replyNATT = pkt.LocalAddr, pkt.RemoteAddr, pkt.NATT
	defer func() { sa.replyLocal, sa.replyRemote, sa.replyNATT = nil, nil, false }()

	// RFC 7296 Section 2.3: a request is accepted only at the next expected Message ID,
	// and the previous one is a retransmission answered from the cache. IKE_AUTH left
	// ExpectedMsgID one past its own request (finishResponderEstablish).
	switch classifyInbound(sa, msg.Header.MessageID, false, nil) {
	case inboundRetransmit:
		// replayCachedResponse carries the two guards RFC 7296 Section 2.4 requires of an
		// answer drawn by a message nobody authenticated yet.
		replayCachedResponse(sa, msg, pkt, tr, log)
		return true
	case inboundInvalid:
		// An out-of-window request draws INVALID_MESSAGE_ID on an owned SA, which is a
		// request of Ze's own and holds this SA's request window until it is answered. The
		// successor is not Ze's to raise requests on before adoption, so the request is
		// dropped, counted, and the peer's retransmission finds the adopted SA.
		countErrorNotifySuppressed("held-successor-out-of-window")
		return true
	case inboundNewRequest:
	case inboundResponse:
		// classifyInbound answers inboundResponse only for a response, refused above.
		return true
	default:
		panic("BUG: unknown inbound IKE message classification")
	}

	// RFC 7296 Section 2.4. A request that fails its integrity check is dropped and
	// counted by openProtected, silent as on the owned path.
	inner, dropped, err := openProtected(sa, msg, pkt.Data, suppressedHeldSuccessorUnauthenticated, log)
	if dropped {
		return true
	}
	if err != nil {
		// RFC 7296 Section 3.10.1 lets INVALID_SYNTAX go out only "in an encrypted packet
		// if the Message ID and cryptographic checksum were valid", which errInnerParse,
		// the only error openProtected returns, proves.
		ps.respondInnerParseError(sa, msg, err, tr, log)
		ps.dropDeadHeldSuccessor(sa, table, log)
		return true
	}
	// The message decrypted under the successor's keys at the expected Message ID, the
	// two preconditions adoptAuthenticatedEndpoint names. The successor holds no Child
	// SA, so no encapsulation follows the endpoint.
	sa.adoptAuthenticatedEndpoint(pkt.RemoteAddr, pkt.NATT, log)
	logIgnoredNotifies(inner, ps.peerName, false, log)

	switch msg.Header.ExchangeType {
	case wire.ExchangeCreateChildSA:
		ps.answerHeldSuccessorCreateChild(sa, msg, inner, tr, log)
	case wire.ExchangeInformational:
		ps.answerHeldSuccessorInformational(sa, msg, inner, table, tr, log)
	default:
		log.Debug("ike: unexpected exchange on the held successor", "peer", ps.peerName, "exchange", msg.Header.ExchangeType)
	}
	return true
}

// holdsSuccessor reports whether sa is the session's held successor. The caller holds
// childLifecycleMu.
func (ps *PeerSession) holdsSuccessor(sa *SA) bool {
	if ps.getPendingSA() != sa {
		return false
	}
	if sa.State != StateEstablished {
		return false
	}
	if ps.getPendingChild() != nil {
		return false
	}
	return ps.ownedSA.Load() != nil
}

// answerHeldSuccessorCreateChild answers a CREATE_CHILD_SA request on the held
// successor. A new Child SA is the one the successor lacked: it is installed, staged in
// the second slot, and the owner loop is told to relinquish, exactly as when the Child
// SA arrives in IKE_AUTH (finishResponderEstablish). The caller holds childLifecycleMu.
//
// RFC 7296 Section 2.8.3: "Reauthentication is done by creating a new IKE SA from
// scratch (using IKE_SA_INIT/IKE_AUTH exchanges, without any REKEY_SA Notify
// payloads), creating new Child SAs within the new IKE SA (without REKEY_SA Notify
// payloads), and finally deleting the old IKE SA (which deletes the old Child SAs as
// well)."
func (ps *PeerSession) answerHeldSuccessorCreateChild(sa *SA, msg *wire.Message, inner []wire.PayloadEntry, tr *transport.UDPTransport, log *slog.Logger) {
	msgID := msg.Header.MessageID
	if rekey := rekeySANotify(inner); rekey != nil {
		// RFC 7296 Section 2.25.1: "If a peer receives a request to rekey a Child SA
		// that does not exist, it SHOULD reply with CHILD_SA_NOT_FOUND."
		ps.respondChildSANotFound(sa, msgID, rekey, tr, log)
		return
	}
	// The TS payloads tell a new Child SA from an IKE SA rekey (handleCreateChildSAOwned).
	if !hasTSPayload(inner) {
		// RFC 7296 Section 2.25: "A TEMPORARY_FAILURE notification SHOULD be sent when a
		// peer receives a request that cannot be completed due to a temporary condition
		// such as a rekeying operation." The successor is not adopted yet; the adopted
		// SA's owner loop answers the peer's retry.
		log.Info("ike: refusing an IKE SA rekey on the held successor until it is adopted", "peer", ps.peerName)
		ps.respondError(sa, msgID, wire.ExchangeCreateChildSA, wire.NotifyTemporaryFailure, nil, tr, log)
		return
	}
	// RFC 7296 Section 1.3.1: "A Child SA may be created by sending a CREATE_CHILD_SA
	// request." The successor holds none, so this request creates its one Child SA.
	resp, child, err := respondNewChild(sa, inner, ps.getESPGroup(), msgID, dataplane.Get(), log)
	if err != nil {
		// RFC 7296 Section 2.21.3 MUST: "After the IKE SA is authenticated, all requests
		// having errors MUST result in a response notifying the other end of the error."
		// A refused install answers the same way, before the response that would have
		// given the peer its half, so the peer holds no Child SA Ze does not.
		log.Warn("ike: refused the peer's Child SA on the held successor, the established SA keeps the session",
			"peer", ps.peerName, "error", err, "fix", childRefusedResponderFix(err))
		ps.respondError(sa, msgID, wire.ExchangeCreateChildSA, notifyForRefusal(err), nil, tr, log)
		return
	}
	cacheResponse(sa, msgID, resp)
	sendRaw(sa, tr, resp, log)
	if child == nil {
		log.Info("ike: refused the peer's Child SA on the held successor, KE group mismatch", "peer", ps.peerName)
		return
	}
	ps.setPendingChild(child)
	log.Info("child-sa: created by peer on the held successor, superseding the established SA", "peer", ps.peerName,
		"in", child.InboundSPI, "out", child.OutboundSPI)
	ps.signalSupersede(log)
}

// answerHeldSuccessorInformational answers an INFORMATIONAL request on the held
// successor: a liveness probe, or a Delete. A Delete of the successor ends it alone; the
// established SA keeps the session. The successor holds no Child SA, so a Delete of an
// ESP SPI names nothing here. The caller holds childLifecycleMu.
//
// RFC 7296 Section 1.4.1: "The response to a request that deletes the IKE SA is an
// empty INFORMATIONAL response."
//
// RFC 7296 Section 2.25.1: "If a peer receives a request to close a Child SA that does
// not exist, it SHOULD reply without a Delete payload."
func (ps *PeerSession) answerHeldSuccessorInformational(sa *SA, msg *wire.Message, inner []wire.PayloadEntry, table *SATable, tr *transport.UDPTransport, log *slog.Logger) {
	msgID := msg.Header.MessageID
	ikeDeleted := false
	// RFC 7296 Section 2.21.3: a malformed Delete makes the request malformed, judged
	// over the whole chain before anything closes (handleInformationalOwned).
	for i := range inner {
		del, ok := inner[i].Payload.(*wire.PayloadDelete)
		if !ok {
			continue
		}
		if deleteMalformed(del) {
			log.Warn("ike: peer Delete payload on the held successor is malformed, answering INVALID_SYNTAX",
				"peer", ps.peerName, "protocol", del.ProtocolID, "spi-size", del.SPISize, "num-spis", del.NumSPIs)
			ps.respondError(sa, msgID, wire.ExchangeInformational, wire.NotifyInvalidSyntax, nil, tr, log)
			ps.dropDeadHeldSuccessor(sa, table, log)
			return
		}
		if del.ProtocolID == wire.ProtocolIKE {
			ikeDeleted = true
		}
	}
	resp, err := buildEncryptedMessageEx(sa, nil, msgID, wire.ExchangeInformational, initiatorFlag(sa)|wire.FlagResponse)
	if err != nil {
		log.Warn("ike: could not answer an INFORMATIONAL request on the held successor", "peer", ps.peerName, "error", err)
		return
	}
	cacheResponse(sa, msgID, resp)
	sendRaw(sa, tr, resp, log)
	if !ikeDeleted {
		return
	}
	log.Info("ike: the peer deleted the held successor, the established IKE SA keeps the session",
		"peer", ps.peerName, "ispi", SPIHex(sa.InitiatorSPI), "rspi", SPIHex(sa.ResponderSPI))
	ps.forgetSecondSlot(sa, table)
	// The slot is free, so the peer's next IKE_SA_INIT is admitted (tryResponderSAInit).
	ps.responderBusy.Store(false)
}

// dropDeadHeldSuccessor ends a held successor that an INVALID_SYNTAX answer just
// killed. RFC 7296 Section 2.21.3 makes that notification "fatal in both peers, meaning
// that the IKE SA is deleted without needing an explicit Delete payload", so nothing is
// sent. The caller holds childLifecycleMu.
func (ps *PeerSession) dropDeadHeldSuccessor(sa *SA, table *SATable, log *slog.Logger) {
	if sa.State != StateDead {
		return
	}
	log.Warn("ike: the held successor ended with INVALID_SYNTAX, the established IKE SA keeps the session", "peer", ps.peerName)
	ps.forgetSecondSlot(sa, table)
	ps.responderBusy.Store(false)
}

// rekeySANotify returns the request's REKEY_SA notify, or nil.
func rekeySANotify(inner []wire.PayloadEntry) *wire.PayloadNotify {
	for i := range inner {
		if n, ok := inner[i].Payload.(*wire.PayloadNotify); ok && n.NotifyMsgType == wire.NotifyRekeySA {
			return n
		}
	}
	return nil
}

// respondChildSANotFound answers a Child SA rekey that names no Child SA of this IKE SA.
// The caller holds the SA's owner, here childLifecycleMu.
//
// RFC 7296 Section 2.25: "The SA that the initiator attempted to rekey is indicated by
// the SPI field in the Notify payload, which is copied from the SPI field in the
// REKEY_SA notification."
func (ps *PeerSession) respondChildSANotFound(sa *SA, msgID uint32, rekey *wire.PayloadNotify, tr *transport.UDPTransport, log *slog.Logger) {
	notify := &wire.PayloadNotify{
		ProtocolID:    rekey.ProtocolID,
		SPISize:       rekey.SPISize,
		NotifyMsgType: wire.NotifyChildSANotFound,
		SPI:           rekey.SPI,
	}
	resp, err := buildEncryptedMessageEx(sa, []wire.PayloadEntry{{Payload: notify}}, msgID,
		wire.ExchangeCreateChildSA, initiatorFlag(sa)|wire.FlagResponse)
	if err != nil {
		log.Warn("ike: could not answer a Child SA rekey with CHILD_SA_NOT_FOUND", "peer", ps.peerName, "error", err)
		return
	}
	cacheResponse(sa, msgID, resp)
	sendRaw(sa, tr, resp, log)
	countErrorNotifySent(wire.NotifyChildSANotFound, true)
	log.Info("ike: answered a rekey of a Child SA the held successor does not hold with CHILD_SA_NOT_FOUND", "peer", ps.peerName)
}
