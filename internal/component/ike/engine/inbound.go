// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- inbound message handling for established SAs
// RFC: rfc/short/rfc7296.md -- INFORMATIONAL (Section 1.4), CREATE_CHILD_SA (Section 1.3)
// Related: notify_error.go -- the error notify this file sends when it refuses a request

package engine

import (
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/ze-software/ze/internal/component/ike/crypto"
	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/component/ike/wire"
)

// ownedOutcome reports state changes the maintainSA loop must apply after a
// post-establishment exchange: a newly installed Child SA (reset childLT, emit),
// a replacement IKE SA (swap the loop SA, re-key the SATable, reset ikeLT),
// peerAlive (an in-window authenticated inbound proves liveness), and/or an
// authenticated INFORMATIONAL response's message ID (a possible DPD-probe or
// path-probe reply the caller correlates against each outstanding probe by id).
type ownedOutcome struct {
	newChild     *ChildSA
	newSA        *SA
	peerAlive    bool
	dpdResp      bool
	dpdRespMsgID uint32
	// reestablish asks the owner loop to delete this session's SAs and build a fresh
	// one through the initial exchanges. RFC 7296 Section 4 requires that fallback
	// when a peer refuses a rekey with NO_ADDITIONAL_SAS.
	reestablish bool
}

// handleOwnedInbound processes a packet delivered to this session's maintainSA
// owner loop for an established SA. Running here (not on the shared dispatchInbound
// goroutine) makes maintainSA the single owner of all post-establishment SA and
// childSA state, which the CREATE_CHILD_SA rekey exchanges (spec-ipsec-13) require
// to avoid racing the shared goroutine. It returns an ownedOutcome describing any
// rekey that completed. RFC 7296 §2.3 message-ID validation is applied first.
func (ps *PeerSession) handleOwnedInbound(sa *SA, pkt transport.Packet, tr *transport.UDPTransport, dp dataplane.Dataplane, log *slog.Logger) ownedOutcome {
	ps.childLifecycleMu.Lock()
	defer ps.childLifecycleMu.Unlock()
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
		// RFC 7296 Section 2.4 MUST, requirement RFC7296-2.4-12: "Implementations MUST
		// limit the rate at which they take actions based on unprotected messages".
		// classifyInbound runs before the message is authenticated, so an unprotected
		// forgery carrying the cached Message ID reaches this branch. Both SPIs and the
		// Message ID travel in the clear in every IKE header, so an attacker who saw one
		// datagram can build that forgery.
		//
		// Section 2.21.4 is NOT the obligation here, though it is the section a reader
		// reaches for. Its MUST NOTs cover a message marked as a RESPONSE and a peer
		// receiving an unprotected INVALID_IKE_SPI Notify. For an unprotected REQUEST it
		// says the node "MAY send a response". Refusing is permitted, not mandated.
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
		// The same `RFC7296-2.4-12` MUST as the guard above, which is why both are
		// needed and neither is optional. Section 2.21.4 opens with "A node needs to
		// limit the rate ...", which is not RFC 2119 language, so Section 2.4 carries
		// the only obligation here.
		//
		// The SK-presence test above raises the cost of a forgery from a bare header
		// to about forty octets. It does not remove the amplification: the cached
		// response is several hundred octets.
		//
		// The token bucket is the second guard, and both are needed. The sibling
		// site is replayCachedResponse (responder.go). It carries the identical pair
		// and serves both pre-adoption arms of handleResponderInbound, the mid-EAP
		// one included. A guard added to one replay site, with the other left open,
		// is the failure ai/rules/architecture.md names.
		if !sa.cachedReplayAllowed() {
			log.Debug("ike: cached response replay rate limited",
				"peer", ps.peerName, "msgid", msg.Header.MessageID)
			countErrorNotifySuppressed("replay-rate-limited")
			return ownedOutcome{}
		}
		// A MOBIKE retransmission can use another address pair. Authenticate it
		// before routing the cached response there; never reapply its update.
		if sa.mobike.enabled {
			inner, err := decryptAndParse(sa, &msg, pkt.Data)
			if err != nil {
				return ownedOutcome{}
			}
			if sa.lastResponseSet {
				replyToMobikeRetransmit(sa, pkt, inner, msg.Header.MessageID, msg.Header.ExchangeType, tr, log)
			}
			return ownedOutcome{}
		}
		if sa.lastResponseSet {
			sendRaw(sa, tr, sa.lastResponse, log)
		}
		return ownedOutcome{}
	case inboundInvalid:
		// Responses to our own fire-and-forget requests (DPD probe, Delete, padded
		// path probe) match no pending exchange. Authenticate an INFORMATIONAL
		// response and report its message ID; the caller correlates it against the
		// outstanding DPD probe and, separately, against the outstanding path probe
		// (settleProbe, probe.go), each by message ID, so a replayed/out-of-window
		// response cannot mask a dead peer and a path probe's answer credits no
		// liveness.
		if isResponse && msg.Header.ExchangeType == wire.ExchangeInformational {
			if inner, err := decryptAndParse(sa, &msg, pkt.Data); err == nil {
				if sa.mobike.pending != nil && sa.mobike.pending.msgID == msg.Header.MessageID {
					return ps.handleMobikeResponse(sa, inner, msg.Header.MessageID, dp, tr, log)
				}
				if sa.requestOutstanding && sa.requestMsgID == msg.Header.MessageID {
					observeMobikeNATMapping(sa, inner)
				}
				// Release site one of two, after authentication. RFC 7296 §2.3: this
				// answer completes a request that left no pendingRekey, so the window
				// it holds frees here and nowhere else.
				sa.answerAuthenticatedResponse(msg.Header.MessageID)
				return ownedOutcome{dpdResp: true, dpdRespMsgID: msg.Header.MessageID}
			}
		}
		// RFC 7296 Section 2.3: an out-of-window REQUEST draws an INVALID_MESSAGE_ID.
		// The notification is not an answer to this request. It is a new INFORMATIONAL
		// request, and it carries a Message ID of its own. The invalid request therefore
		// stays unacknowledged, and no response goes out. That is the MUST NOT of the
		// same sentence, and RFC7296-2.3-5.
		//
		// The decrypt runs FIRST, and its failure is silent. classifyInbound judged this
		// message before any authentication. Both SPIs and the Message ID travel in the
		// clear. An off-path attacker can therefore build a datagram that reaches here.
		// An emission before the decrypt would let one forgery spend this SA's request
		// window. It would stall the liveness probe, the Delete and the rekey.
		//
		// RFC 7296 Section 3.1 forbids an answer to anything marked as a response. The
		// !isResponse guard therefore comes first.
		if !isResponse {
			if _, err := decryptAndParse(sa, &msg, pkt.Data); err == nil {
				ps.sendInvalidMessageID(sa, msg.Header.MessageID, tr, log)
			} else {
				// Deliberately not logged above debug. A louder line here is an oracle
				// that tells an off-path prober its guess reached a live SA.
				countErrorNotifySuppressed("invalid-msgid-unauthenticated")
			}
		}
		log.Debug("ike: owned inbound out of window",
			"peer", ps.peerName, "exchange", msg.Header.ExchangeType, "msgid", msg.Header.MessageID,
			"response", isResponse, "expected", sa.ExpectedMsgID)
		// No peerAlive. An out-of-window message is no evidence of liveness (RFC 7296
		// Section 2.4). A replay can therefore never mask a dead peer.
		return ownedOutcome{}
	case inboundNewRequest, inboundResponse:
	default:
		panic("BUG: unknown inbound IKE message classification")
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
	if !isResponse && sa.mobike.enabled {
		sa.replyLocal, sa.replyRemote, sa.replyNATT = pkt.LocalAddr, pkt.RemoteAddr, pkt.NATT
		defer func() { sa.replyLocal, sa.replyRemote, sa.replyNATT = nil, nil, false }()
		if notify := validateMobikeRequest(sa, inner, pkt.RemoteAddr, pkt.LocalAddr); notify != 0 {
			// RFC 4555 Section 3.9: after UNEXPECTED_NAT_DETECTED the responder
			// "MUST NOT use the contents of the NO_NATS_ALLOWED notification for
			// any other purpose than possibly logging". No address mutation precedes
			// this check; the answer uses the observed packet tuple exclusively.
			respondMobikeError(sa, inner, msg.Header.MessageID, msg.Header.ExchangeType, notify, tr, log)
			return ownedOutcome{}
		}
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

// refuseRekey records that the peer refused the pending rekey p, and decides what
// follows. named is the group an INVALID_KE_PAYLOAD answer named, else dhGroupNone.
//
// RFC 7296 Section 1.3.1: "A failed attempt to create a Child SA SHOULD NOT tear down
// the IKE SA". The SA the rekey would have replaced stays in use throughout. When a
// configured proposal the peer has not refused is left, the next owner-loop tick sends
// it, with no wait: the soft lifetime still stands and raises the rekey. When every
// configured proposal has been refused, the rekey waits rekeyRefusedWait and then
// starts over. The two kinds wait separately, as the TEMPORARY_FAILURE holds do, so a
// refused IKE SA rekey does not stop a Child SA rekey.
//
// It MUST be called before p.clear(), which drops the exchange that names the group
// the refused request carried.
func (ps *PeerSession) refuseRekey(sa *SA, p *pendingRekey, named crypto.DHGroupID, log *slog.Logger) {
	sent := dhGroupNone
	if p.dh != nil {
		sent = p.dh.GroupID
	}
	var record *rekeyRefusalRecord
	var hold *time.Time
	switch p.kind {
	case rekeyChild:
		record, hold = &sa.childRekeyRefusal, &ps.childRekeyRefusedUntil
	case rekeyIKE:
		record, hold = &sa.ikeRekeyRefusal, &ps.ikeRekeyRefusedUntil
	default:
		panic("BUG: unknown pending IKE rekey kind")
	}
	if record.refuse(sent, named, sa.IKEGroup) {
		log.Info("ike: rekey refused, retrying in the group the peer named",
			"peer", ps.peerName, "refused-group", sent, "group", record.next)
		return
	}
	wait := rekeyRefusedWait - lifetimeJitter(rekeyRefusedWait)
	*hold = time.Now().Add(wait)
	log.Warn("ike: rekey refused for every configured proposal, keeping the current SA",
		"peer", ps.peerName, "retry-after", wait)
}

// handleCreateChildSAOwned drives Child SA and IKE SA rekeys. As initiator it
// completes our pending rekey (Child: install new + make-before-break Delete of the
// old; IKE: derive the new SA + Delete the old). As Child rekey responder it
// installs the replacement and replies. RFC 7296 §1.3.2, §1.3.3.
func (ps *PeerSession) handleCreateChildSAOwned(sa *SA, msg *wire.Message, inner []wire.PayloadEntry, isResponse bool, tr *transport.UDPTransport, dp dataplane.Dataplane, log *slog.Logger) ownedOutcome {
	if ps.getPendingChild() != nil {
		// A parallel authenticated IKE SA now owns the shared policy templates.
		// This old SA is closing, not another writer permitted to rekey them.
		// RFC 7296 Sections 2.25.1 and 2.25.2: a rekey request for a Child or
		// IKE SA that is currently being closed SHOULD receive TEMPORARY_FAILURE.
		if !isResponse {
			ps.respondError(sa, msg.Header.MessageID, wire.ExchangeCreateChildSA,
				wire.NotifyTemporaryFailure, nil, tr, log)
			return ownedOutcome{}
		}
		if p := ps.pendingRekey; p != nil && p.kind == rekeyChild {
			p.clear()
			ps.pendingRekey = nil
			// The authenticated response has freed our request window. Its peer
			// may already have installed a replacement Child; closing this IKE SA
			// closes that Child too (RFC 7296 Section 2.4), without installing it
			// over the new owner's policies locally.
			ps.sendDeleteIKE(sa, tr, log)
			return ownedOutcome{}
		}
	}
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
		// The request failed, so it is held like any refused rekey: the soft lifetime is
		// a level trigger and would otherwise resend it on the next tick.
		if err := failIfUnrecognizedErrorNotify(inner, ps.peerName, log); err != nil {
			ps.refuseRekey(sa, p, dhGroupNone, log)
			p.clear()
			ps.pendingRekey = nil
			return ownedOutcome{}
		}
		switch p.kind {
		case rekeyChild:
			newChild, err := applyChildRekeyResponse(sa, p, inner, dp, log)
			if err != nil {
				var refused *rekeyRefusedError
				// RFC 7296 §2.25: a TEMPORARY_FAILURE answer means wait. The soft
				// lifetime is a level trigger. Without this hold the next one-second
				// tick retries against a peer that just asked for a delay.
				switch {
				case errors.Is(err, errTemporaryFailure):
					ps.childRekeyHoldUntil = time.Now().Add(temporaryFailureBackoff)
					log.Info("child-sa: rekey refused with TEMPORARY_FAILURE, waiting",
						"peer", ps.peerName, "backoff", temporaryFailureBackoff)
				case errors.Is(err, errNoAdditionalSAs):
					// RFC 7296 Section 4: this peer will never accept the rekey, so
					// retrying it is pointless and the old SA still has to be replaced.
					// The fallback the section requires is to delete the old SA and
					// create a new one, which is what re-establishment does.
					log.Info("child-sa: rekey refused with NO_ADDITIONAL_SAS, re-establishing",
						"peer", ps.peerName)
					ps.pendingRekey = nil
					return ownedOutcome{reestablish: true}
				case errors.As(err, &refused) && refused.notify == wire.NotifyChildSANotFound:
					// RFC 7296 Section 2.25: "A peer that receives a CHILD_SA_NOT_FOUND
					// notification SHOULD silently delete the Child SA (if it still
					// exists) and send a request to create a new Child SA from scratch
					// (if the Child SA does not yet exist)." Re-establishment is ze's
					// path that deletes the Child SA and builds a new one, the same exit
					// NO_ADDITIONAL_SAS takes above. A retried rekey of an SA the peer
					// does not hold can never succeed.
					log.Info("child-sa: rekey refused with CHILD_SA_NOT_FOUND, re-establishing",
						"peer", ps.peerName)
					p.clear()
					ps.pendingRekey = nil
					return ownedOutcome{reestablish: true}
				case errors.As(err, &refused):
					// RFC 7296 Section 1.3.1: the old Child SA stays in use while
					// refuseRekey picks the next attempt.
					log.Warn("child-sa: rekey refused", "peer", ps.peerName, "error", err)
					ps.refuseRekey(sa, p, refused.named, log)
				default:
					log.Warn("ike: child rekey response failed", "peer", ps.peerName, "error", err)
				}
				p.clear()
				ps.pendingRekey = nil
				return ownedOutcome{}
			}
			old := p.oldChild
			ps.setChildSA(newChild)
			// The replacement is a new SA, so nothing has been refused for it yet.
			sa.childRekeyRefusal = rekeyRefusalRecord{}
			// Make-before-break: new SA is installed; delete the old now (§2.8).
			// The Delete records the pair as awaiting a response, so a Delete the peer
			// sends for the same pair in the meantime is read as RFC 7296 Section
			// 1.4.1's crossing case rather than answered with a duplicate.
			ps.recordOwnDelete(old)
			ps.sendDeleteESP(sa, tr, old.InboundSPI, log)
			// newChild is already the live pair and shares these policies, so only the
			// retired states go.
			removeChildSAExcept(old, newChild, dp, log)
			ps.pendingRekey = nil
			log.Info("child-sa: rekeyed via CREATE_CHILD_SA", "peer", ps.peerName,
				"old-in", old.InboundSPI, "new-in", newChild.InboundSPI)
			return ownedOutcome{newChild: newChild}
		case rekeyIKE:
			newSA, err := applyIKERekeyResponse(sa, p, inner, log)
			if err != nil {
				var refused *rekeyRefusedError
				// RFC 7296 §2.25, as on the Child SA path above.
				switch {
				case errors.Is(err, errTemporaryFailure):
					ps.ikeRekeyHoldUntil = time.Now().Add(temporaryFailureBackoff)
					log.Info("ike-sa: rekey refused with TEMPORARY_FAILURE, waiting",
						"peer", ps.peerName, "backoff", temporaryFailureBackoff)
				case errors.Is(err, errNoAdditionalSAs):
					// RFC 7296 Section 4, as on the Child SA path above.
					log.Info("ike-sa: rekey refused with NO_ADDITIONAL_SAS, re-establishing",
						"peer", ps.peerName)
					p.clear()
					ps.pendingRekey = nil
					return ownedOutcome{reestablish: true}
				case errors.As(err, &refused):
					// As on the Child SA path above: the current IKE SA stays in use.
					log.Warn("ike-sa: rekey refused", "peer", ps.peerName, "error", err)
					ps.refuseRekey(sa, p, refused.named, log)
				default:
					log.Warn("ike: IKE rekey response failed", "peer", ps.peerName, "error", err)
				}
				p.clear()
				ps.pendingRekey = nil
				return ownedOutcome{}
			}
			// RFC 7296 §2.8: delete the old IKE SA (encrypted under the old keys)
			// before the caller swaps to the new one.
			ps.sendDeleteIKE(sa, tr, log)
			// The replacement is a new SA, so nothing has been refused for its rekey yet.
			newSA.ikeRekeyRefusal = rekeyRefusalRecord{}
			p.clear()
			ps.pendingRekey = nil
			return ownedOutcome{newSA: newSA}
		default:
			panic("BUG: unknown pending IKE rekey kind")
		}
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
	// The TS payloads tell a new Child SA from an IKE SA rekey, never the KE payload,
	// because a new Child SA with PFS carries KEi too.
	// RFC 7296 Section 1.3.1: a new Child SA request is "HDR, SK {SA, Ni, [KEi,] TSi, TSr}".
	// RFC 7296 Section 1.3.2: an IKE SA rekey request is "HDR, SK {SA, Ni, KEi}".
	if hasTSPayload(inner) {
		return ps.handleNewChildRequest(sa, msg, tr, log)
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
	// A CREATE_CHILD_SA with no REKEY_SA, no TS and no KE is neither a Child SA
	// request nor an IKE SA rekey. RFC 7296 Section 2.21.3 MUST still answer it.
	// RFC 7296 Section 3.10.1 blesses NO_PROPOSAL_CHOSEN as the "generic Child SA error
	// when Child SA cannot be created for some other reason".
	log.Info("ike: refusing a CREATE_CHILD_SA request that is neither a Child SA nor an IKE SA rekey",
		"peer", ps.peerName)
	ps.respondError(sa, msg.Header.MessageID, wire.ExchangeCreateChildSA,
		wire.NotifyNoProposalChosen, nil, tr, log)
	return ownedOutcome{}
}

// handleNewChildRequest answers a peer CREATE_CHILD_SA request for a new Child SA.
// Ze holds one Child SA per IKE SA (PeerSession.childSA). RFC 7296 Section 2.21.3
// MUST answer every errored request on an authenticated SA. MUST run on the owner
// loop, like handleCreateChildSAOwned.
func (ps *PeerSession) handleNewChildRequest(sa *SA, msg *wire.Message, tr *transport.UDPTransport,
	log *slog.Logger,
) ownedOutcome {
	if ps.getChildSA() != nil {
		// RFC 7296 Section 1.3: "The responder sends a NO_ADDITIONAL_SAS notification to
		// indicate that a CREATE_CHILD_SA request is unacceptable because the responder is
		// unwilling to accept any more Child SAs on this IKE SA."
		log.Info("ike: refusing a peer request for a second Child SA", "peer", ps.peerName)
		ps.respondError(sa, msg.Header.MessageID, wire.ExchangeCreateChildSA,
			wire.NotifyNoAdditionalSAs, nil, tr, log)
		return ownedOutcome{}
	}
	// RFC 7296 Section 3.10.1 blesses NO_PROPOSAL_CHOSEN as the "generic Child SA error
	// when Child SA cannot be created for some other reason".
	log.Info("ike: refusing a peer request for a new Child SA", "peer", ps.peerName)
	ps.respondError(sa, msg.Header.MessageID, wire.ExchangeCreateChildSA,
		wire.NotifyNoProposalChosen, nil, tr, log)
	return ownedOutcome{}
}

// hasTSPayload reports whether the payload chain carries a TSi or TSr payload.
func hasTSPayload(inner []wire.PayloadEntry) bool {
	for i := range inner {
		if _, ok := inner[i].Payload.(*wire.PayloadTS); ok {
			return true
		}
	}
	return false
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

// handleInformationalOwned processes INFORMATIONAL requests/responses on an
// established SA: DPD liveness and Delete. A request is answered (RFC 7296 §1.4).
//
// RFC 7296 Section 1.4.1: "Normally, the response in the INFORMATIONAL exchange will
// contain Delete payloads for the paired SAs going in the other direction." Each pair
// this node closes therefore names its own INBOUND SPI in the response, which is the
// half the peer still holds. Two cases carry no Delete payload at all. An IKE SA
// Delete, because the same section makes that response empty. And the crossing case,
// where the same section forbids one.
func (ps *PeerSession) handleInformationalOwned(sa *SA, msg *wire.Message, inner []wire.PayloadEntry, isResponse bool, tr *transport.UDPTransport, dp dataplane.Dataplane, log *slog.Logger) ownedOutcome {
	var out ownedOutcome
	var paired []uint32
	ikeDeleted := false
	// RFC 7296 Section 2.21.3: "After the IKE SA is authenticated, all requests having
	// errors MUST result in a response notifying the other end of the error." A Delete
	// payload whose SPI Size breaks Section 3.11 is such an error, and it is checked over
	// the WHOLE chain before anything is closed: a malformed payload makes the request
	// malformed, and Section 2.21.2 has a malformed request "rejected in their entirety".
	// Answering it with the empty response an unresolvable SPI list produces would tell the
	// peer its Delete succeeded.
	if !isResponse {
		for i := range inner {
			del, ok := inner[i].Payload.(*wire.PayloadDelete)
			if !ok || !deleteMalformed(del) {
				continue
			}
			log.Warn("ike: peer Delete payload is malformed, answering INVALID_SYNTAX",
				"peer", ps.peerName, "protocol", del.ProtocolID,
				"spi-size", del.SPISize, "num-spis", del.NumSPIs)
			ps.respondError(sa, msg.Header.MessageID, wire.ExchangeInformational,
				wire.NotifyInvalidSyntax, nil, tr, log)
			return out
		}
		if notify := ps.acceptMobikeUpdate(sa, inner); notify != 0 {
			respondMobikeError(sa, inner, msg.Header.MessageID, msg.Header.ExchangeType, notify, tr, log)
			return out
		}
	}
	for i := range inner {
		del, ok := inner[i].Payload.(*wire.PayloadDelete)
		if !ok {
			continue
		}
		if del.ProtocolID == wire.ProtocolIKE && ps.pendingIKESwap != nil {
			// RFC 7296 §2.8: the peer confirmed the IKE rekey by deleting the old SA.
			// Swap to the new SA instead of tearing the session down.
			out.newSA = ps.pendingIKESwap
			// Mobility may have started after the rekey response was built.
			// Carry its current path and queue a fresh check on the new SA,
			// rather than promoting a stale tuple or an old-SA Message ID.
			out.newSA.inheritSendPath(sa)
			ps.pendingIKESwap = nil
			log.Info("ike: peer deleted old IKE SA after rekey, swapping to new SA", "peer", ps.peerName)
			continue
		}
		if del.ProtocolID == wire.ProtocolIKE {
			ikeDeleted = true
		}
		closed, downed := ps.handleDeletePayload(sa, del, dp, log)
		paired = append(paired, closed...)
		if downed {
			out.reestablish = true
		}
	}
	if isResponse {
		// RFC 7296 Section 1.4.1 puts the second half of the crossing case here:
		// a node that issued a Delete request deletes
		// "the incoming SAs while processing the response".
		ps.finishOwnDeletes()
		return out
	}
	// RFC 7296 §1.4: every INFORMATIONAL request (DPD probe or Delete) is answered.
	// The response is still built under the current (old) SA keys.
	respPayloads := mobikeResponsePayloads(sa, inner)
	if len(paired) > 0 && !ikeDeleted {
		respPayloads = append(respPayloads, wire.PayloadEntry{Payload: espDeletePayload(paired)})
	}
	resp, err := buildEncryptedMessageEx(sa, respPayloads, msg.Header.MessageID, wire.ExchangeInformational, initiatorFlag(sa)|wire.FlagResponse)
	if err != nil {
		log.Debug("ike: informational response build failed", "peer", ps.peerName, "error", err)
		return out
	}
	cacheResponse(sa, msg.Header.MessageID, resp)
	sendRaw(sa, tr, resp, log)
	return out
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
