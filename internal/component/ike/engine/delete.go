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
// incoming half "while processing the response". outgoingGone records that the first of
// the two already happened, so the second removes only what is left.
// removed records that the caller tore the whole pair down itself, right after sending
// the Delete. The make-before-break rekey does exactly that: the replacement pair is
// already carrying traffic, so the retired one is of no further use and nothing is
// gained by holding it to the schedule below. Both halves are therefore already gone
// before any crossing request can arrive, which is what the section asks for, and the
// record survives only to suppress the duplicate Delete payload the section forbids.
type pendingDelete struct {
	child        *ChildSA
	outgoingGone bool
	removed      bool
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
func (ps *PeerSession) crossOwnDelete(spi uint32, dp dataplane.Dataplane, log *slog.Logger) bool {
	for i := range ps.deleteRequested {
		p := &ps.deleteRequested[i]
		if !childNamedBy(p.child, spi) {
			continue
		}
		if !p.removed && !p.outgoingGone {
			removeChildSAOutgoing(p.child, dp, log)
			p.outgoingGone = true
		}
		log.Info("child-sa: peer delete crossed our own, answering without a Delete payload",
			"peer", ps.peerName, "out-spi", spi)
		return true
	}
	return false
}

// finishOwnDeletes completes every Delete this node issued, once the INFORMATIONAL
// response arrives.
//
// RFC 7296 Section 1.4.1 puts the second half of the crossing case here: the node
// deletes "the incoming SAs while processing the response". A pair the peer never
// crossed still has both halves installed at this point, so the whole pair goes.
func (ps *PeerSession) finishOwnDeletes(dp dataplane.Dataplane, log *slog.Logger) {
	for i := range ps.deleteRequested {
		p := &ps.deleteRequested[i]
		switch {
		case p.child == nil || p.removed:
			// The caller already tore the pair down. Nothing is installed to remove.
		case p.outgoingGone:
			removeChildSAIncoming(p.child, dp, log)
			p.child.Clear()
		default:
			removeChildSA(p.child, dp, log)
		}
	}
	ps.deleteRequested = nil
}

// markOwnDeleteRemoved records that the caller tore the pair down itself, right after
// sendDeleteESP. Nothing is left installed, so the crossing case and the response both
// skip the dataplane while the record still suppresses the duplicate Delete payload RFC
// 7296 Section 1.4.1 forbids.
func (ps *PeerSession) markOwnDeleteRemoved(child *ChildSA) {
	for i := range ps.deleteRequested {
		if ps.deleteRequested[i].child == child {
			ps.deleteRequested[i].removed = true
			return
		}
	}
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

// deleteSPIs decodes the SPI list a Delete payload carries.
//
// RFC 7296 Section 3.11 gives ESP and AH an SPI Size of four octets, and IKE a size of
// zero. Any other size names nothing this node can resolve to an SA, so it decodes to no
// SPI at all rather than to a misaligned guess.
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
		if ps.crossOwnDelete(spi, dp, log) {
			continue
		}
		switch child := ps.getChildSA(); {
		case childNamedBy(ps.supersededChild, spi):
			// The peer confirming a rekey it responded to. The replacement pair is
			// already carrying traffic, so the session stays up.
			old := ps.supersededChild
			ps.supersededChild = nil
			removeChildSA(old, dp, log)
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
