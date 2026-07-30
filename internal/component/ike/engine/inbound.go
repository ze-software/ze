// Design: plan/learned/742-ipsec-8-ikev2-child-xfrm.md -- inbound message handling for established SAs
// RFC: rfc/short/rfc7296.md -- INFORMATIONAL (Section 1.4), CREATE_CHILD_SA (Section 1.3)

package engine

import (
	"encoding/binary"
	"fmt"
	"log/slog"

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
		log.Debug("ike: owned inbound parse error", "peer", ps.peerName, "error", err)
		return ownedOutcome{}
	}

	isResponse := msg.Header.Flags&wire.FlagResponse != 0
	switch classifyInbound(sa, msg.Header.MessageID, isResponse, ps.pendingRekey) {
	case inboundRetransmit:
		// RFC 7296 §2.3: a duplicate request is answered from cache, not reprocessed.
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
		return ownedOutcome{}
	}

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
	return wire.ParsePayloadChain(plain, sk.InnerNextPayload)
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
		switch p.kind {
		case rekeyChild:
			newChild, err := applyChildRekeyResponse(sa, p, inner, dp, log)
			if err != nil {
				log.Warn("ike: child rekey response failed", "peer", ps.peerName, "error", err)
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
				log.Warn("ike: IKE rekey response failed", "peer", ps.peerName, "error", err)
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
		// RFC 7296 §2.8.1: simultaneous Child SA rekey. If we already initiated one,
		// the lower nonce wins; the loser abandons its own exchange. Only resolve a
		// collision against a well-formed request that actually carries a nonce; a
		// malformed request (no Ni) must not make us abandon our in-flight rekey.
		if p := ps.pendingRekey; p != nil && p.kind == rekeyChild {
			if peerNi := nonceFromPayloads(inner); len(peerNi) > 0 {
				if resolveRekeyCollision(p.localNonce, peerNi) {
					log.Info("ike: simultaneous child rekey, we win (lower nonce), ignoring peer request", "peer", ps.peerName)
					return ownedOutcome{}
				}
				log.Info("ike: simultaneous child rekey, peer wins, abandoning our exchange", "peer", ps.peerName)
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
			log.Warn("ike: child rekey respond failed", "peer", ps.peerName, "error", err)
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
		resp, newSA, err := respondIKERekey(sa, inner, msg.Header.MessageID, log)
		if err != nil {
			log.Warn("ike: IKE rekey respond failed", "peer", ps.peerName, "error", err)
			return ownedOutcome{}
		}
		cacheResponse(sa, msg.Header.MessageID, resp)
		sendRaw(sa, tr, resp, log)
		// Make-before-break: hold the new SA until the peer deletes the old IKE SA.
		ps.setPendingIKESwap(newSA)
		log.Info("ike: responded to peer IKE SA rekey", "peer", ps.peerName)
		return ownedOutcome{}
	}
	log.Debug("ike: peer new-child request unsupported, ignoring", "peer", ps.peerName)
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
	sa.NextMsgID++
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
	sa.NextMsgID++
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

// handleEstablishedInbound processes inbound messages on an established IKE SA.
func handleEstablishedInbound(sa *SA, msg *wire.Message, log *slog.Logger) {
	switch msg.Header.ExchangeType {
	case wire.ExchangeInformational:
		handleInformational(sa, msg, log)
	case wire.ExchangeCreateChildSA:
		handleCreateChildSA(sa, msg, log)
	default:
		log.Debug("ike: unexpected exchange on established SA",
			"peer", sa.PeerName, "exchange", msg.Header.ExchangeType)
	}
}

// handleInformational processes INFORMATIONAL exchanges (DPD, DELETE).
// RFC 7296 Section 1.4: empty INFORMATIONAL is a DPD probe/response.
func handleInformational(sa *SA, msg *wire.Message, log *slog.Logger) {
	isResponse := msg.Header.Flags&wire.FlagResponse != 0

	var hasDelete bool
	for _, pe := range msg.Payloads {
		switch p := pe.Payload.(type) {
		case *wire.PayloadDelete:
			hasDelete = true
			if p.ProtocolID == wire.ProtocolIKE {
				log.Info("ike: peer requested IKE SA delete", "peer", sa.PeerName)
				sa.State = StateDead
				return
			}
			log.Info("ike: peer deleted child SA",
				"peer", sa.PeerName, "proto", p.ProtocolID, "spis", len(p.SPIs))
		case *wire.PayloadNotify:
			log.Debug("ike: informational notify",
				"peer", sa.PeerName, "type", p.NotifyMsgType)
		}
	}

	if !hasDelete && isResponse {
		log.Debug("ike: DPD response received", "peer", sa.PeerName)
	}
	if !hasDelete && !isResponse {
		log.Debug("ike: DPD probe received", "peer", sa.PeerName)
	}
}

// handleCreateChildSA processes CREATE_CHILD_SA exchanges.
// RFC 7296 Section 1.3: new child SA, child rekey, or IKE rekey.
func handleCreateChildSA(sa *SA, msg *wire.Message, log *slog.Logger) {
	isResponse := msg.Header.Flags&wire.FlagResponse != 0

	var hasRekeySA bool
	for _, pe := range msg.Payloads {
		if n, ok := pe.Payload.(*wire.PayloadNotify); ok {
			if n.NotifyMsgType == wire.NotifyRekeySA {
				hasRekeySA = true
			}
		}
	}

	if isResponse {
		if hasRekeySA {
			log.Debug("ike: child SA rekey response", "peer", sa.PeerName)
		} else {
			log.Debug("ike: CREATE_CHILD_SA response", "peer", sa.PeerName)
		}
		return
	}

	if hasRekeySA {
		log.Info("ike: peer initiated child SA rekey", "peer", sa.PeerName)
	} else {
		hasSAPayload := false
		hasKE := false
		hasTS := false
		for _, pe := range msg.Payloads {
			switch pe.Payload.(type) {
			case *wire.PayloadSA:
				hasSAPayload = true
			case *wire.PayloadKE:
				hasKE = true
			case *wire.PayloadTS:
				hasTS = true
			}
		}
		if hasSAPayload && hasKE && !hasTS {
			log.Info("ike: peer initiated IKE SA rekey", "peer", sa.PeerName)
		} else {
			log.Info("ike: peer initiated new child SA", "peer", sa.PeerName)
		}
	}
}
