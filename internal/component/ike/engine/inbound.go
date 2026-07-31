// Design: plan/learned/742-ipsec-8-ikev2-child-xfrm.md -- inbound message handling for established SAs
// RFC: rfc/short/rfc7296.md -- INFORMATIONAL (Section 1.4), CREATE_CHILD_SA (Section 1.3)
// Related: notify_error.go -- the error notify this file sends when it refuses a request

package engine

import (
	"encoding/binary"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/component/ike/wire"
)

// ownedOutcome reports state changes the maintainSA loop must apply after a
// post-establishment exchange: a newly installed Child SA (reset childLT, emit),
// a replacement IKE SA (swap the loop SA, re-key the SATable, reset ikeLT),
// peerAlive (an in-window authenticated inbound proves liveness), and/or an
// authenticated INFORMATIONAL response's message ID (a possible DPD-probe reply
// the caller correlates against the outstanding probe before crediting liveness).
type ownedOutcome struct {
	newChild     *ChildSA
	newSA        *SA
	peerAlive    bool
	dpdResp      bool
	dpdRespMsgID uint32
}

// handleOwnedInbound processes a packet delivered to this session's maintainSA
// owner loop for an established SA. Running here (not on the shared dispatchInbound
// goroutine) makes maintainSA the single owner of all post-establishment SA and
// childSA state, which the CREATE_CHILD_SA rekey exchanges (spec-ipsec-13) require
// to avoid racing the shared goroutine. It returns an ownedOutcome describing any
// rekey that completed. RFC 7296 §2.3 message-ID validation is applied first.
func (ps *PeerSession) handleOwnedInbound(sa *SA, pkt transport.Packet, tr *transport.UDPTransport, dp dataplane.Dataplane, log *slog.Logger) ownedOutcome {
	var msg wire.Message
	if err := msg.ReadFrom(pkt.Data); err != nil {
		// This is the OUTER message, parsed before any decryption.
		// A failure here means Ze never located the SK payload.
		// Neither the Message ID nor the cryptographic checksum was ever validated.
		// RFC 7296 Section 3.10.1 permits INVALID_SYNTAX only
		// "for and in an encrypted packet if the Message ID and cryptographic checksum were valid".
		// This site therefore stays a silent drop.
		// An answer here turns a 28-byte forgery into a guaranteed reply.
		log.Debug("ike: owned inbound parse error", "peer", ps.peerName, "error", err)
		countErrorNotifySuppressed("outer-parse-unauthenticated")
		return ownedOutcome{}
	}

	isResponse := msg.Header.Flags&wire.FlagResponse != 0
	switch classifyInbound(sa, msg.Header.MessageID, isResponse, ps.pendingRekey) {
	case inboundRetransmit:
		// RFC 7296 §2.3: a duplicate request is answered from cache, not reprocessed.
		//
		// RFC 7296 Section 2.21.4 MUST NOT: "A peer receiving such an unprotected Notify
		// payload MUST NOT respond". classifyInbound runs before the message is
		// authenticated, so an unprotected forgery carrying the cached Message ID
		// reaches this branch. Both SPIs and the Message ID travel in the clear in every
		// IKE header, so an attacker who saw one datagram can build that forgery.
		//
		// Every genuine post-IKE_AUTH request is protected (RFC 7296 Section 1.4), so
		// the presence of an Encrypted payload separates a real retransmission from a
		// forgery. The test is structural and needs no key material. A full decrypt is
		// not an option here, because the cache exists precisely so a duplicate is never
		// decrypted twice.
		if !carriesSKPayload(&msg) {
			log.Debug("ike: unprotected message at the cached message id, not answered",
				"peer", ps.peerName, "msgid", msg.Header.MessageID)
			countErrorNotifySuppressed("unprotected-retransmit")
			return ownedOutcome{}
		}
		// RFC 7296 Section 2.21.4 asks for a rate limit on what unprotected traffic
		// can draw out of this node. The SK-presence test above raises the cost of a
		// forgery from a 28-byte header to about forty octets. It does not remove the
		// amplification: the cached response is several hundred octets.
		//
		// The token bucket is the second guard, and both are needed. The sibling
		// site in handleResponderInbound (responder.go) carries the identical pair.
		// A guard added to one replay site, with the other left open, is the
		// failure ai/rules/before-writing-code.md names.
		if !sa.cachedReplayAllowed() {
			log.Debug("ike: cached response replay rate limited",
				"peer", ps.peerName, "msgid", msg.Header.MessageID)
			countErrorNotifySuppressed("replay-rate-limited")
			return ownedOutcome{}
		}
		// The destination is the SA's STORED endpoint, through sendRaw, and never
		// pkt.RemoteAddr. This request did not decrypt, so nothing corroborates its
		// source. A cached replay is not "a response to a request Ze accepted": Ze
		// recognized a Message ID, it did not accept the message. RFC 7296
		// Section 2.11 therefore does not put this emission on the observed source.
		if sa.lastResponseSet {
			sendRaw(sa, tr, sa.lastResponse, log)
		}
		return ownedOutcome{}
	case inboundInvalid:
		// Responses to our own fire-and-forget requests (DPD probe, Delete) match no
		// pending exchange. Authenticate an INFORMATIONAL response and report its
		// message ID; the caller correlates it against the outstanding DPD probe (by
		// message ID) before crediting liveness, so a replayed/out-of-window response
		// cannot mask a dead peer.
		if isResponse && msg.Header.ExchangeType == wire.ExchangeInformational {
			if _, err := decryptAndParse(sa, &msg, pkt.Data); err == nil {
				// Release site one of two, after authentication. RFC 7296 §2.3: this
				// answer completes a request that left no pendingRekey, so the window
				// it holds frees here and nowhere else.
				sa.answerAuthenticatedResponse(msg.Header.MessageID)
				return ownedOutcome{dpdResp: true, dpdRespMsgID: msg.Header.MessageID}
			}
		}
		log.Debug("ike: owned inbound out of window",
			"peer", ps.peerName, "exchange", msg.Header.ExchangeType, "msgid", msg.Header.MessageID,
			"response", isResponse, "expected", sa.ExpectedMsgID)
		return ownedOutcome{}
	case inboundNewRequest, inboundResponse:
	}

	inner, err := decryptAndParse(sa, &msg, pkt.Data)
	if err != nil {
		log.Debug("ike: owned inbound decrypt failed", "peer", ps.peerName, "error", err)
		// RFC 7296 Section 3.10.1 lets INVALID_SYNTAX go out only "in an encrypted packet
		// if the Message ID and cryptographic checksum were valid". Both hold here and
		// only here: classifyInbound returned inboundNewRequest, which means the Message
		// ID equals sa.ExpectedMsgID, and errInnerParse is set only after decryptSKPayload
		// verified the integrity check. A decrypt failure carries no such proof, so it
		// stays silent. RFC 7296 Section 3.1 forbids answering a response at all.
		if !isResponse && errors.Is(err, errInnerParse) {
			ps.respondInnerParseError(sa, &msg, err, tr, log)
		}
		return ownedOutcome{}
	}
	// The message decrypted and its integrity check passed. classifyInbound already
	// applied the Message ID window. Both preconditions of adoptAuthenticatedEndpoint
	// therefore hold HERE and nowhere earlier in this function.
	//
	// The inboundRetransmit arm compared a Message ID and decrypted nothing. The
	// inboundInvalid arm decrypts a response the window already refused, which RFC
	// 7296 Section 2.23 calls out as replayable.
	//
	// This is what sends an established-SA response to the address and port the
	// request came from (RFC 7296 Section 2.11). sendRaw reads the endpoint stored
	// here. An UNAUTHENTICATED datagram never reaches this line, so it never moves
	// the peer.
	sa.adoptAuthenticatedEndpoint(pkt.RemoteAddr, pkt.NATT, log)

	// RFC 7296 Section 3.10.1: an unrecognized notify that is neither an error in a
	// response nor acted on elsewhere is ignored, and it is logged.
	logIgnoredNotifies(inner, ps.peerName, isResponse, log)

	// Release site two of two, after authentication. This one carries the rekey
	// response. A peer REQUEST also reaches here, and it must never free the window
	// our own outstanding request holds (RFC 7296 §2.3).
	if isResponse {
		sa.answerAuthenticatedResponse(msg.Header.MessageID)
	}

	// An authenticated inbound message from the peer proves it is alive (RFC 7296
	// §2.4 liveness): reset the DPD wait regardless of the exchange.
	var out ownedOutcome
	switch msg.Header.ExchangeType {
	case wire.ExchangeCreateChildSA:
		out = ps.handleCreateChildSAOwned(sa, &msg, inner, isResponse, tr, dp, log)
	case wire.ExchangeInformational:
		out = ps.handleInformationalOwned(sa, &msg, inner, isResponse, tr, dp, log)
	default:
		log.Debug("ike: unexpected owned exchange", "peer", ps.peerName, "exchange", msg.Header.ExchangeType)
	}
	out.peerAlive = true
	return out
}

// decryptAndParse decrypts the SK payload of an established-SA message and parses
// its inner payload chain.
func decryptAndParse(sa *SA, msg *wire.Message, raw []byte) ([]wire.PayloadEntry, error) {
	var sk *wire.PayloadSK
	for i := range msg.Payloads {
		if p, ok := msg.Payloads[i].Payload.(*wire.PayloadSK); ok {
			sk = p
			break
		}
	}
	if sk == nil {
		return nil, fmt.Errorf("no SK payload")
	}
	plain, err := decryptSKPayload(sa, raw, sk)
	if err != nil {
		return nil, err
	}
	inner, err := wire.ParsePayloadChain(plain, sk.InnerNextPayload)
	if err != nil {
		// Mark the failure as an INNER one.
		// The distinction decides whether an error notify can go out.
		// Only a chain that failed AFTER the integrity check passed satisfies the
		// INVALID_SYNTAX precondition of RFC 7296 Section 3.10.1.
		// A decrypt failure returns unwrapped and stays silent.
		return nil, fmt.Errorf("%w: %w", errInnerParse, err)
	}
	return inner, nil
}

// errInnerParse marks a failure to parse the inner payload chain of a message whose
// SK payload already decrypted and passed its integrity check. RFC 7296 Section 3.10.1
// allows an error notification only in that case.
var errInnerParse = errors.New("ike: inner payload chain")

// respondInnerParseError answers a request whose decrypted inner chain would not
// parse. RFC 7296 Section 2.21.2 MUST: such a request "MUST only lead to an
// UNSUPPORTED_CRITICAL_PAYLOAD or INVALID_SYNTAX Notification sent as a response".
//
// RFC 7296 Section 2.5 MUST: an unrecognized critical payload draws
// UNSUPPORTED_CRITICAL_PAYLOAD whose "Notification Data contains the one-octet payload
// type". Every other malformation draws INVALID_SYNTAX, which Section 3.10.1 makes the
// answer "to any error not covered by one of the other status types".
func (ps *PeerSession) respondInnerParseError(sa *SA, msg *wire.Message, err error, tr *transport.UDPTransport, log *slog.Logger) {
	if ptype, ok := wire.CriticalPayloadType(err); ok {
		ps.respondError(sa, msg.Header.MessageID, msg.Header.ExchangeType,
			wire.NotifyUnsupportedCriticalPayload, []byte{ptype}, tr, log)
		return
	}
	ps.respondError(sa, msg.Header.MessageID, msg.Header.ExchangeType,
		wire.NotifyInvalidSyntax, nil, tr, log)
}

// handleCreateChildSAOwned drives Child SA and IKE SA rekeys. As initiator it
// completes our pending rekey (Child: install new + make-before-break Delete of the
// old; IKE: derive the new SA + Delete the old). As Child rekey responder it
// installs the replacement and replies. RFC 7296 §1.3.2, §1.3.3.
func (ps *PeerSession) handleCreateChildSAOwned(sa *SA, msg *wire.Message, inner []wire.PayloadEntry, isResponse bool, tr *transport.UDPTransport, dp dataplane.Dataplane, log *slog.Logger) ownedOutcome {
	if isResponse {
		p := ps.pendingRekey
		if p == nil {
			return ownedOutcome{}
		}
		// RFC 7296 Section 3.10.1 MUST:
		// "An implementation receiving a Notify payload with one of these types that it does not recognize in a response MUST assume that the corresponding request has failed entirely".
		// The rekey is that request.
		// An unrecognized error type therefore ends it.
		// The walk below never reads a response that carries no keys.
		if err := failIfUnrecognizedErrorNotify(inner, ps.peerName, log); err != nil {
			p.clear()
			ps.pendingRekey = nil
			return ownedOutcome{}
		}
		switch p.kind {
		case rekeyChild:
			newChild, err := applyChildRekeyResponse(sa, p, inner, dp, log)
			if err != nil {
				// RFC 7296 §2.25: a TEMPORARY_FAILURE answer means wait. The soft
				// lifetime is a level trigger. Without this hold the next one-second
				// tick retries against a peer that just asked for a delay.
				if errors.Is(err, errTemporaryFailure) {
					ps.childRekeyHoldUntil = time.Now().Add(temporaryFailureBackoff)
					log.Info("child-sa: rekey refused with TEMPORARY_FAILURE, waiting",
						"peer", ps.peerName, "backoff", temporaryFailureBackoff)
				} else {
					log.Warn("ike: child rekey response failed", "peer", ps.peerName, "error", err)
				}
				ps.pendingRekey = nil
				return ownedOutcome{}
			}
			old := p.oldChild
			ps.setChildSA(newChild)
			// Make-before-break: new SA is installed; delete the old now (§2.8).
			ps.sendDeleteESP(sa, tr, old.InboundSPI, log)
			removeChildSA(old, dp, log)
			ps.pendingRekey = nil
			log.Info("child-sa: rekeyed via CREATE_CHILD_SA", "peer", ps.peerName,
				"old-in", old.InboundSPI, "new-in", newChild.InboundSPI)
			return ownedOutcome{newChild: newChild}
		case rekeyIKE:
			newSA, err := applyIKERekeyResponse(sa, p, inner, log)
			if err != nil {
				// RFC 7296 §2.25, as on the Child SA path above.
				if errors.Is(err, errTemporaryFailure) {
					ps.ikeRekeyHoldUntil = time.Now().Add(temporaryFailureBackoff)
					log.Info("ike-sa: rekey refused with TEMPORARY_FAILURE, waiting",
						"peer", ps.peerName, "backoff", temporaryFailureBackoff)
				} else {
					log.Warn("ike: IKE rekey response failed", "peer", ps.peerName, "error", err)
				}
				p.clear()
				ps.pendingRekey = nil
				return ownedOutcome{}
			}
			// RFC 7296 §2.8: delete the old IKE SA (encrypted under the old keys)
			// before the caller swaps to the new one.
			ps.sendDeleteIKE(sa, tr, log)
			p.clear()
			ps.pendingRekey = nil
			return ownedOutcome{newSA: newSA}
		}
		return ownedOutcome{}
	}

	// Peer-initiated request.
	if hasRekeySANotify(inner) {
		// RFC 7296 §2.8.1: simultaneous Child SA rekey. The exchange that carries the
		// lower nonce is the one its creator closes. Our own exchange goes when our
		// nonce is the lower one. Only a well-formed request that carries a nonce
		// resolves a collision. A malformed request (no Ni) must never make us abandon
		// our in-flight rekey.
		if p := ps.pendingRekey; p != nil && p.kind == rekeyChild {
			if peerNi := nonceFromPayloads(inner); len(peerNi) > 0 {
				if !localNonceIsLower(p.localNonce, peerNi) {
					log.Info("ike: simultaneous child rekey, our nonce is higher, ignoring peer request", "peer", ps.peerName)
					return ownedOutcome{}
				}
				log.Info("ike: simultaneous child rekey, our nonce is lower, abandoning our exchange", "peer", ps.peerName)
				// Our own request will never be answered now, so free the request
				// window it holds (RFC 7296 §2.3). Without this the SA sends nothing
				// more.
				sa.releaseRequestWindow()
				ps.pendingRekey = nil
			}
		}
		old := ps.getChildSA()
		if old == nil {
			return ownedOutcome{}
		}
		resp, newChild, err := respondChildRekey(sa, inner, old, msg.Header.MessageID, dp, log)
		if err != nil {
			// RFC 7296 Section 2.21.3 MUST:
			// "After the IKE SA is authenticated, all requests having errors MUST result in a response notifying the other end of the error".
			// Silence here spends the peer's single request window on retransmissions.
			// It then closes a working IKE SA over one drifted algorithm.
			// RFC 7296 Section 2.7 names NO_PROPOSAL_CHOSEN for a refused offer.
			log.Warn("ike: child rekey respond failed", "peer", ps.peerName, "error", err)
			ps.respondError(sa, msg.Header.MessageID, wire.ExchangeCreateChildSA,
				notifyForRefusal(err), nil, tr, log)
			return ownedOutcome{}
		}
		cacheResponse(sa, msg.Header.MessageID, resp)
		sendRaw(sa, tr, resp, log)
		// Make-before-break: keep the old SA until the peer's Delete arrives.
		ps.supersededChild = old
		ps.setChildSA(newChild)
		log.Info("child-sa: rekeyed by peer via CREATE_CHILD_SA", "peer", ps.peerName,
			"old-in", old.InboundSPI, "new-in", newChild.InboundSPI)
		return ownedOutcome{newChild: newChild}
	}
	// A CREATE_CHILD_SA request with SA+KE and no TS/REKEY_SA is a peer-initiated
	// IKE SA rekey (RFC 7296 Section 1.3.3). Respond with the new IKE SA keys; the
	// owner loop swaps to the new SA when the peer's Delete of the old one arrives
	// (spec-ipsec-14, closes ipsec-13's deferred responder).
	if hasKEPayload(inner) {
		// RFC 7296 Section 2.8.2: simultaneous IKE SA rekey. Both peers run the same
		// nonce comparison, so they abandon opposite exchanges and one new IKE SA is
		// left. Its Child SAs hang off this session, so the survivor inherits them.
		// Only a well-formed request that carries a nonce resolves a collision. A
		// request without Ni must never make us abandon our own exchange.
		if p := ps.pendingRekey; p != nil && p.kind == rekeyIKE {
			if peerNi := nonceFromPayloads(inner); len(peerNi) > 0 {
				if !localNonceIsLower(p.localNonce, peerNi) {
					log.Info("ike: simultaneous IKE rekey, our nonce is higher, ignoring peer request", "peer", ps.peerName)
					return ownedOutcome{}
				}
				log.Info("ike: simultaneous IKE rekey, our nonce is lower, abandoning our exchange", "peer", ps.peerName)
				// Our own request will never be answered now, so free the request
				// window it holds (RFC 7296 Section 2.3) and release the DH half we
				// kept for it. Without this the SA sends nothing more.
				sa.releaseRequestWindow()
				p.clear()
				ps.pendingRekey = nil
			}
		}
		resp, newSA, err := respondIKERekey(sa, inner, msg.Header.MessageID, log)
		if err != nil {
			// RFC 7296 Section 2.21.3, as on the Child SA path above. respondIKERekey
			// already answers a KE-group mismatch with INVALID_KE_PAYLOAD and returns no
			// error, so every error that reaches here was answered with silence.
			log.Warn("ike: IKE rekey respond failed", "peer", ps.peerName, "error", err)
			ps.respondError(sa, msg.Header.MessageID, wire.ExchangeCreateChildSA,
				notifyForRefusal(err), nil, tr, log)
			return ownedOutcome{}
		}
		cacheResponse(sa, msg.Header.MessageID, resp)
		sendRaw(sa, tr, resp, log)
		if newSA == nil {
			// RFC 7296 Section 1.3: the request named a Diffie-Hellman group we did
			// not select, so the answer is an INVALID_KE_PAYLOAD Notify and no new SA
			// exists. The peer retries with the group the Notify names.
			log.Info("ike: refused peer IKE SA rekey, KE group mismatch", "peer", ps.peerName)
			return ownedOutcome{}
		}
		// Make-before-break: hold the new SA until the peer deletes the old IKE SA.
		ps.setPendingIKESwap(newSA)
		log.Info("ike: responded to peer IKE SA rekey", "peer", ps.peerName)
		return ownedOutcome{}
	}
	// A CREATE_CHILD_SA that is neither a Child rekey nor an IKE rekey asks for a NEW
	// Child SA, which Ze does not create. RFC 7296 Section 2.21.3 MUST still answer it.
	// RFC 7296 Section 3.10.1 blesses NO_PROPOSAL_CHOSEN as the "generic Child SA error
	// when Child SA cannot be created for some other reason".
	log.Info("ike: refusing a peer request for a new Child SA", "peer", ps.peerName)
	ps.respondError(sa, msg.Header.MessageID, wire.ExchangeCreateChildSA,
		wire.NotifyNoProposalChosen, nil, tr, log)
	return ownedOutcome{}
}

// nonceFromPayloads returns the Nonce payload data, or nil if absent.
func nonceFromPayloads(inner []wire.PayloadEntry) []byte {
	for i := range inner {
		if n, ok := inner[i].Payload.(*wire.PayloadNonce); ok {
			return n.NonceData
		}
	}
	return nil
}

// hasKEPayload reports whether the payload chain contains a Key Exchange payload.
func hasKEPayload(inner []wire.PayloadEntry) bool {
	for i := range inner {
		if _, ok := inner[i].Payload.(*wire.PayloadKE); ok {
			return true
		}
	}
	return false
}

// sendDeleteIKE sends an INFORMATIONAL Delete for the IKE SA (best-effort). RFC
// 7296 §1.4, §3.11: an IKE Delete carries no SPIs.
func (ps *PeerSession) sendDeleteIKE(sa *SA, tr *transport.UDPTransport, log *slog.Logger) {
	if tr == nil {
		return
	}
	// RFC 7296 §2.3: one self-initiated request at a time. A Delete is best-effort
	// and a teardown must never wait for a window, so a held window drops it. The
	// peer then learns of the loss through its own dead-peer detection.
	if !sa.reserveRequestWindow() {
		log.Debug("ike: IKE delete dropped, a request is outstanding", "peer", ps.peerName)
		return
	}
	del := &wire.PayloadDelete{ProtocolID: wire.ProtocolIKE}
	msg, err := buildEncryptedMessageEx(sa, []wire.PayloadEntry{{Payload: del}}, sa.NextMsgID, wire.ExchangeInformational, initiatorFlag(sa))
	if err != nil {
		log.Debug("ike: IKE delete build failed", "peer", ps.peerName, "error", err)
		sa.releaseRequestWindow()
		return
	}
	sa.advanceMsgID()
	sendRaw(sa, tr, msg, log)
}

// handleInformationalOwned processes INFORMATIONAL requests/responses on an
// established SA: DPD liveness and Delete. A request is answered (RFC 7296 §1.4).
func (ps *PeerSession) handleInformationalOwned(sa *SA, msg *wire.Message, inner []wire.PayloadEntry, isResponse bool, tr *transport.UDPTransport, dp dataplane.Dataplane, log *slog.Logger) ownedOutcome {
	var out ownedOutcome
	for i := range inner {
		del, ok := inner[i].Payload.(*wire.PayloadDelete)
		if !ok {
			continue
		}
		if del.ProtocolID == wire.ProtocolIKE && ps.pendingIKESwap != nil {
			// RFC 7296 §2.8: the peer confirmed the IKE rekey by deleting the old SA.
			// Swap to the new SA instead of tearing the session down.
			out.newSA = ps.pendingIKESwap
			ps.pendingIKESwap = nil
			log.Info("ike: peer deleted old IKE SA after rekey, swapping to new SA", "peer", ps.peerName)
			continue
		}
		ps.handleDeletePayload(sa, del, dp, log)
	}
	if isResponse {
		return out
	}
	// RFC 7296 §1.4: every INFORMATIONAL request (DPD probe or Delete) is answered.
	// The response is still built under the current (old) SA keys.
	resp, err := buildEncryptedMessageEx(sa, nil, msg.Header.MessageID, wire.ExchangeInformational, initiatorFlag(sa)|wire.FlagResponse)
	if err != nil {
		log.Debug("ike: informational response build failed", "peer", ps.peerName, "error", err)
		return out
	}
	cacheResponse(sa, msg.Header.MessageID, resp)
	sendRaw(sa, tr, resp, log)
	return out
}

// handleDeletePayload removes the superseded Child SA when the peer confirms a
// rekey with a Delete, or marks the IKE SA dead on an IKE Delete. RFC 7296 §1.4.
func (ps *PeerSession) handleDeletePayload(sa *SA, del *wire.PayloadDelete, dp dataplane.Dataplane, log *slog.Logger) {
	switch del.ProtocolID {
	case wire.ProtocolIKE:
		log.Info("ike: peer deleted IKE SA", "peer", ps.peerName)
		sa.State = StateDead
	case wire.ProtocolESP:
		if ps.supersededChild != nil {
			removeChildSA(ps.supersededChild, dp, log)
			ps.supersededChild = nil
		}
	}
}

// sendDeleteESP sends an INFORMATIONAL Delete for an ESP SPI (best-effort, no
// awaited response). RFC 7296 §1.4, §2.8: the rekey initiator deletes the old SA.
func (ps *PeerSession) sendDeleteESP(sa *SA, tr *transport.UDPTransport, spi uint32, log *slog.Logger) {
	if tr == nil {
		return
	}
	// RFC 7296 §2.3: one self-initiated request at a time. The make-before-break
	// Delete runs right after the rekey response freed the window, so it usually
	// takes it at once. A held window drops the Delete, because it is best-effort
	// and the peer removes the old Child SA on its own lifetime.
	if !sa.reserveRequestWindow() {
		log.Debug("ike: ESP delete dropped, a request is outstanding", "peer", ps.peerName)
		return
	}
	spiBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(spiBytes, spi)
	del := &wire.PayloadDelete{ProtocolID: wire.ProtocolESP, SPISize: 4, NumSPIs: 1, SPIs: spiBytes}
	msg, err := buildEncryptedMessageEx(sa, []wire.PayloadEntry{{Payload: del}}, sa.NextMsgID, wire.ExchangeInformational, initiatorFlag(sa))
	if err != nil {
		log.Debug("ike: delete build failed", "peer", ps.peerName, "error", err)
		sa.releaseRequestWindow()
		return
	}
	sa.advanceMsgID()
	sendRaw(sa, tr, msg, log)
}

// hasRekeySANotify reports whether the payload chain contains a REKEY_SA notify.
func hasRekeySANotify(inner []wire.PayloadEntry) bool {
	for i := range inner {
		if n, ok := inner[i].Payload.(*wire.PayloadNotify); ok && n.NotifyMsgType == wire.NotifyRekeySA {
			return true
		}
	}
	return false
}
