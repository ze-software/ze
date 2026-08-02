// Design: plan/learned/742-ipsec-8-ikev2-child-xfrm.md -- Child SA teardown over INFORMATIONAL
// RFC: rfc/short/rfc7296.md -- Deleting an SA with INFORMATIONAL Exchanges (Section 1.4.1)
// Overview: inbound.go -- the INFORMATIONAL handler that calls into this file
// Related: child.go -- installChildSA / removeChildSA, the dataplane half of a Child SA

package engine

import (
	"encoding/binary"
	"log/slog"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/component/ike/wire"
)

// pendingDelete is one Child SA pair this node has asked the peer to close, and whose
// INFORMATIONAL response has not arrived.
//
// RFC 7296 Section 1.4.1 splits the removal across two events when the peer deletes the
// same pair at the same time: the outgoing half goes "while processing the request", the
// incoming half "while processing the response".
//
// ZE REMOVES BOTH HALVES EARLIER THAN THAT, and the record exists for the section's OTHER
// obligation. The one path that issues a Delete is the make-before-break rekey
// (inbound.go): the replacement pair is already carrying traffic, so it sends the Delete
// and tears the retired pair down immediately, on the same goroutine, before any crossing
// request can be processed. Removing sooner than the section's two events satisfies them
// by the time each arrives, and it is why the retired pair cannot be leaked when the
// response never comes.
//
// So the ordering fields this struct used to carry could not be reached: every record was
// already fully removed. What is left is the fact the section's other MUST turns on --
// that this node HAS an unanswered Delete outstanding for the pair -- which is what
// suppresses the duplicate Delete payload the section forbids in the response.
type pendingDelete struct {
	child *ChildSA
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

// sendDeleteESP sends an INFORMATIONAL Delete for a Child SA pair (best-effort, no
// awaited response). RFC 7296 §1.4, §2.8: the rekey initiator deletes the old SA.
//
// The Delete names the pair's INBOUND SPI, because RFC 7296 Section 1.4.1 asks for the
// SPIs "as they would be expected in the headers of inbound packets".
//
// The caller records the pair with recordOwnDelete BEFORE calling this, so a Delete the
// peer sends for the same pair before the response arrives is recognized as the
// crossing case rather than answered with a duplicate Delete payload. Recording is the
// caller's step because the pair is no longer reachable from the session by the time
// the make-before-break rekey sends its Delete: setChildSA has already installed the
// replacement, so only the caller still holds the retired pair.
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
	msg, err := buildEncryptedMessageEx(sa, []wire.PayloadEntry{{Payload: espDeletePayload([]uint32{spi})}}, sa.NextMsgID, wire.ExchangeInformational, initiatorFlag(sa))
	if err != nil {
		log.Debug("ike: delete build failed", "peer", ps.peerName, "error", err)
		sa.releaseRequestWindow()
		return
	}
	sa.advanceMsgID()
	sendRaw(sa, tr, msg, log)
}

// sendIKESATeardown ends an AUTHENTICATED IKE SA that this node is abandoning because it
// found an error while processing the IKE_AUTH response, and TELLS the peer.
//
// RFC 7296 Section 2.21.2 (rfc/full/rfc7296.txt:3286-3289): "If the error occurs on the
// initiator, the notification MAY be returned in a separate INFORMATIONAL exchange, usually
// with no other payloads. This is an exception for the general rule of not starting new
// exchanges based on errors in responses."
//
// THE DELETE PAYLOAD IS NOT OPTIONAL HERE, and that is the whole reason this function
// exists. The same section (rfc/full/rfc7296.txt:3317-3321) closes the set:
// "the UNSUPPORTED_CRITICAL_PAYLOAD, INVALID_SYNTAX, and AUTHENTICATION_FAILED
// notifications are the only ones to cause the IKE SA to be deleted or not created, without
// a Delete payload."
// TS_UNACCEPTABLE and NO_PROPOSAL_CHOSEN are not in that set, so a bare notify would leave
// the peer holding an SA it believes is live. Both payloads therefore go in one
// INFORMATIONAL: the notify says WHY, and the Delete is what actually ends the SA.
//
// Before this existed, the initiator set State to StateDead and sent nothing at all. The
// peer kept both SAs and went on encrypting to a node that had none, and the log line
// claimed the SA was being deleted.
//
// notifyType 0 sends the Delete alone, for a teardown that is nobody's error: RFC 7296
// Section 1.3.1's "the initiator MUST delete the SA" after a declined transport-mode
// request is a policy decision, not a protocol violation by the peer.
//
// Best-effort, like every other Delete: a held request window drops it rather than delaying
// a teardown, and the peer then learns of the loss through its own dead-peer detection.
func sendIKESATeardown(sa *SA, tr *transport.UDPTransport, notifyType uint16, log *slog.Logger) {
	if tr == nil || sa == nil {
		return
	}
	// RFC 7296 Section 2.21.2 puts this exchange on an AUTHENTICATED SA, so the message is
	// encrypted under the SK keys the exchange just established. An SA with no keys never
	// reached that point and has nothing to say.
	if sa.SKKeys == nil {
		log.Debug("ike: teardown notify skipped, the SA has no keys", "peer", sa.PeerName)
		return
	}
	// RFC 7296 Section 2.2: this INFORMATIONAL is a NEW REQUEST, so it MUST carry a
	// Message ID that no earlier request on this SA has spent.
	//
	// IT ADVANCES BEFORE IT BUILDS, which is the opposite of every established-path sender
	// above. Both callers sit in handleAuthResponse (fsm.go), where NextMsgID still holds
	// the id of the IKE_AUTH REQUEST the response answers: handleSAInitResponse set it to
	// 1, and the only advance past it runs at the end of handleAuthResponse, on the
	// success path, AFTER both teardown arms. Building at NextMsgID therefore re-sent id 1.
	// Ze's own responder cached its IKE_AUTH response under that id
	// (finishResponderEstablish -> cacheResponse), so classifyInbound (msgid.go) read the
	// teardown as inboundRetransmit and REPLAYED the cached IKE_AUTH response. The Delete
	// was never processed and the peer kept the SA, which is the exact outcome this
	// function exists to prevent. The EAP senders in fsm.go advance first for this reason.
	//
	// An SA at the 32-bit ceiling has no id left to spend. advanceMsgID marks it exhausted
	// rather than wrapping, and reserveRequestWindow below then refuses, so the teardown is
	// dropped instead of reusing an id (RFC 7296 Section 2.2).
	sa.advanceMsgID()
	if !sa.reserveRequestWindow() {
		log.Debug("ike: teardown notify dropped, a request is outstanding", "peer", sa.PeerName)
		return
	}
	payloads := make([]wire.PayloadEntry, 0, 2)
	if notifyType != 0 {
		payloads = append(payloads, wire.PayloadEntry{Payload: &wire.PayloadNotify{
			ProtocolID:    wire.ProtocolIKE,
			NotifyMsgType: notifyType,
		}})
	}
	payloads = append(payloads, wire.PayloadEntry{Payload: &wire.PayloadDelete{ProtocolID: wire.ProtocolIKE}})

	msg, err := buildEncryptedMessageEx(sa, payloads, sa.NextMsgID, wire.ExchangeInformational, initiatorFlag(sa))
	if err != nil {
		log.Debug("ike: teardown notify build failed", "peer", sa.PeerName, "error", err)
		sa.releaseRequestWindow()
		return
	}
	log.Info("ike: telling the peer the SA is being deleted",
		"peer", sa.PeerName, "notify", wire.NotifyTypeName(notifyType))
	sendRaw(sa, tr, msg, log)
}

// recordOwnDelete notes that this node has an unanswered Delete request outstanding for
// a Child SA pair. RFC 7296 Section 1.4.1's crossing case turns on exactly that fact.
func (ps *PeerSession) recordOwnDelete(child *ChildSA) {
	if child == nil {
		return
	}
	for i := range ps.deleteRequested {
		if ps.deleteRequested[i].child == child {
			return
		}
	}
	ps.deleteRequested = append(ps.deleteRequested, pendingDelete{child: child})
}

// crossOwnDelete answers the crossing case of RFC 7296 Section 1.4.1 for one SPI, and
// reports whether the SPI named a pair this node had already asked the peer to close.
//
// The section: "If a node receives a delete request for SAs for which it has already
// issued a delete request, it MUST delete the outgoing SAs while processing the request
// and the incoming SAs while processing the response." This is the first half. It also
// tells the caller to answer WITHOUT a Delete payload for the pair, "since that would
// result in duplicate deletion and could in theory delete the wrong SA".
//
// The peer names the pair by the SPI of its own inbound SA, which is this node's
// OUTBOUND SPI, so that is what the record is matched on.
// It touches no dataplane state. The pair was already removed, both halves, by the caller
// that issued the Delete (see pendingDelete). The section's ordering MUST is met by that
// earlier removal; what is answered here is its "the responses MUST NOT include Delete
// payloads for the deleted SAs".
func (ps *PeerSession) crossOwnDelete(spi uint32, log *slog.Logger) bool {
	for i := range ps.deleteRequested {
		if !childNamedBy(ps.deleteRequested[i].child, spi) {
			continue
		}
		log.Info("child-sa: peer delete crossed our own, answering without a Delete payload",
			"peer", ps.peerName, "out-spi", spi)
		return true
	}
	return false
}

// finishOwnDeletes closes out every Delete this node issued, once the INFORMATIONAL
// response arrives.
//
// RFC 7296 Section 1.4.1 puts the second half of the crossing case here: the node deletes
// "the incoming SAs while processing the response". Nothing is installed to remove by this
// point, because the caller that issued the Delete removed both halves before sending it
// (see pendingDelete). What ends here is the RECORD, so a later peer Delete naming the same
// SPI is answered normally rather than treated as crossing an exchange that is over.
func (ps *PeerSession) finishOwnDeletes() {
	ps.deleteRequested = nil
}

// abandonOwnDeletes drops every outstanding Delete record without touching the
// dataplane. The caller is tearing the whole session down, so cleanupChild and
// cleanupSupersededChild own the removal and a record left behind would name freed
// state.
func (ps *PeerSession) abandonOwnDeletes() {
	ps.deleteRequested = nil
}

// espDeletePayload builds one ESP Delete payload naming every SPI given.
//
// RFC 7296 Section 3.11 fixes the SPI Size for ESP at four octets.
func espDeletePayload(spis []uint32) *wire.PayloadDelete {
	buf := make([]byte, 4*len(spis))
	for i, spi := range spis {
		binary.BigEndian.PutUint32(buf[i*4:], spi)
	}
	return &wire.PayloadDelete{
		ProtocolID: wire.ProtocolESP,
		SPISize:    4,
		NumSPIs:    uint16(len(spis)),
		SPIs:       buf,
	}
}

// deleteMalformed reports whether a Delete payload violates the SPI Size RFC 7296 Section
// 3.11 fixes for its protocol, or names a count its SPI field cannot hold.
//
// RFC 7296 Section 3.11 (rfc/full/rfc7296.txt): the SPI Size "MUST be zero for IKE (SPI is
// in message header) or four for AH and ESP". A payload that breaks this is not a request
// naming SAs ze failed to find. It is a MALFORMED request, and Section 2.21.3 requires an
// answer that says so: "After the IKE SA is authenticated, all requests having errors MUST
// result in a response notifying the other end of the error."
//
// Before this existed, deleteSPIs returned no SPI for such a payload, closeDesignatedChildSAs
// looped over nothing, and the peer got an EMPTY INFORMATIONAL response. That reads as
// "your Delete succeeded and closed nothing", which is the one answer the section forbids.
func deleteMalformed(del *wire.PayloadDelete) bool {
	switch del.ProtocolID {
	case wire.ProtocolIKE:
		return del.SPISize != 0
	case wire.ProtocolESP, wire.ProtocolAH:
		if del.SPISize != 4 {
			return true
		}
		// The declared count must be backed by the octets actually carried, or the
		// payload names SAs it did not send. NO WIRE PAYLOAD REACHES THIS ARM:
		// PayloadDelete.ReadFrom (wire/payload_delete.go) returns ErrTruncated unless the
		// datagram holds NumSPIs*SPISize octets, and it copies exactly that many. It is
		// kept as the fail-closed backstop for a payload built in process and for a future
		// decoder that relaxes the check, because deleteSPIs below would otherwise silently
		// resolve fewer SPIs than the peer named (ai/rules/fail-closed-guards.md).
		return len(del.SPIs) < 4*int(del.NumSPIs)
	default:
		// RFC 7296 Section 3.11 assigns Protocol ID from the Security Protocol
		// Identifiers registry; a value naming none of the three protocols ze speaks
		// designates nothing it could close.
		return true
	}
}

// deleteSPIs decodes the SPI list a Delete payload carries.
//
// RFC 7296 Section 3.11 gives ESP and AH an SPI Size of four octets, and IKE a size of
// zero. Any other size names nothing this node can resolve to an SA, so it decodes to no
// SPI at all rather than to a misaligned guess. deleteMalformed above rejects such a
// payload before it reaches here, so this is the second line rather than the only one.
func deleteSPIs(del *wire.PayloadDelete) []uint32 {
	if del.SPISize != 4 {
		return nil
	}
	n := min(len(del.SPIs)/4, int(del.NumSPIs))
	out := make([]uint32, 0, n)
	for i := range n {
		out = append(out, binary.BigEndian.Uint32(del.SPIs[i*4:]))
	}
	return out
}

// childNamedBy reports whether an SPI in a Delete payload designates this Child SA pair.
//
// RFC 7296 Section 1.4.1 binds the SENDER: it lists the SPIs
// "as they would be expected in the headers of inbound packets"
// of the SAs to be deleted, which are its own inbound SAs and therefore this node's
// OUTBOUND halves. It puts no matching rule on the RECIPIENT, whose obligation is only
// to "close the designated SAs".
//
// A ChildSA here IS the pair, so either of its two SPIs designates it and neither can
// designate a different pair. Resolving from both therefore closes exactly the SA the
// peer named, and it also closes it for a peer that writes the other direction's SPI.
// Accepting both widens nothing: a peer can only ever name SPIs of a pair it shares
// with this node.
func childNamedBy(child *ChildSA, spi uint32) bool {
	return child != nil && (child.OutboundSPI == spi || child.InboundSPI == spi)
}

// handleDeletePayload closes the SAs a peer's Delete payload designates. It returns the
// INBOUND SPI of every Child SA pair it closed, which the response names in its own
// Delete payload, and whether the pair carrying this session's traffic was one of them.
//
// RFC 7296 Section 1.4.1 MUST: the recipient
// "MUST close the designated SAs".
// childNamedBy above resolves each SPI to the pair it designates.
func (ps *PeerSession) handleDeletePayload(sa *SA, del *wire.PayloadDelete, dp dataplane.Dataplane, log *slog.Logger) (paired []uint32, sessionChildDown bool) {
	switch del.ProtocolID {
	case wire.ProtocolIKE:
		log.Info("ike: peer deleted IKE SA", "peer", ps.peerName)
		sa.State = StateDead
	case wire.ProtocolESP:
		return ps.closeDesignatedChildSAs(del, dp, log)
	}
	return nil, false
}

// closeDesignatedChildSAs closes each Child SA pair the peer named by SPI.
//
// RFC 7296 Section 1.4.1 gives two answers, and which one applies turns on whether this
// node has already issued a Delete request for the same pair.
//
//	crossing: crossOwnDelete above carries the section's ordering MUST, and the pair
//	          gets no Delete payload in the response.
//	normal:   the pair is closed here and the response names its inbound half, which is
//	          the "paired SAs going in the other direction" the section asks for.
func (ps *PeerSession) closeDesignatedChildSAs(del *wire.PayloadDelete, dp dataplane.Dataplane, log *slog.Logger) (paired []uint32, sessionChildDown bool) {
	for _, spi := range deleteSPIs(del) {
		if ps.crossOwnDelete(spi, log) {
			continue
		}
		switch child := ps.getChildSA(); {
		case childNamedBy(ps.supersededChild, spi):
			// The peer confirming a rekey it responded to. The replacement pair is
			// already carrying traffic, so the session stays up.
			old := ps.supersededChild
			ps.supersededChild = nil
			// The live pair shares this pair's policies (make-before-break), so the
			// retirement takes only the states with it.
			removeChildSAExcept(old, child, dp, log)
			paired = append(paired, old.InboundSPI)
		case childNamedBy(child, spi):
			// The pair carrying this session's traffic. RFC 7296 Section 1.4.1 puts the
			// close on the recipient of the request, so it happens HERE rather than
			// being handed to a caller that might not act on the flag.
			//
			// The pair stays ATTACHED to the session on purpose. sessionChildDown takes
			// the owner loop's tunnel-down exit, and that exit's cleanupChild is what
			// emits child-down and withdraws the tunnel routes. Detaching here would
			// leave those routes advertised over a tunnel that no longer exists. Its
			// second removeChildSA is harmless: ChildSA.Clear touches only the keys, so
			// the SPIs still name the right state and the repeat is two dataplane calls
			// that find nothing and log at Debug.
			removeChildSA(child, dp, log)
			paired = append(paired, child.InboundSPI)
			sessionChildDown = true
			log.Info("child-sa: peer deleted the live Child SA", "peer", ps.peerName,
				"in-spi", child.InboundSPI, "out-spi", child.OutboundSPI)
		default:
			// RFC 7296 Section 1.4.1 has no answer for an SPI naming nothing, and
			// Section 1.4 still requires a response. It carries no Delete payload for a
			// pair that was never closed here.
			log.Debug("child-sa: peer deleted an SPI this session does not hold",
				"peer", ps.peerName, "out-spi", spi)
		}
	}
	return paired, sessionChildDown
}
