// Design: plan/spec-ipsec-7-ikev2-engine.md -- IKE SA finite state machine
// RFC: rfc/short/rfc7296.md -- IKE_SA_INIT and IKE_AUTH exchanges (Sections 1.2, 2.4)
package engine

import (
	"encoding/binary"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/ike/crypto"
	"codeberg.org/thomas-mangin/ze/internal/component/ike/transport"
	"codeberg.org/thomas-mangin/ze/internal/component/ike/wire"
	"codeberg.org/thomas-mangin/ze/internal/component/ipsec"
	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

var (
	errStopped        = errors.New("ike: stopped")
	errAuthFailed     = errors.New("ike: authentication failed")
	errTimeout        = errors.New("ike: retransmit timeout")
	errInvalidMessage = errors.New("ike: invalid message")
)

const (
	nonceLen           = 32
	retransmitBase     = 500 * time.Millisecond
	retransmitMax      = 60 * time.Second
	maxRetransmissions = 7
	reconnectBase      = 1 * time.Second
	reconnectMaxDelay  = 60 * time.Second
)

// afterFunc returns a channel that fires after the given duration.
var afterFunc = time.After

// reconnectDelay computes exponential backoff for peer reconnection.
func reconnectDelay(ps *PeerSession) time.Duration {
	if ps.sa == nil {
		return reconnectBase
	}
	attempt := ps.sa.RetransmitCount
	if attempt <= 0 {
		return reconnectBase
	}
	shift := min(attempt-1, 6)
	d := reconnectBase * time.Duration(1<<uint(shift))
	if d > reconnectMaxDelay {
		return reconnectMaxDelay
	}
	return d
}

// runOnce executes a single IKE SA lifecycle for a peer session.
func (ps *PeerSession) runOnce(
	peer ipsec.SiteToSitePeer,
	ikeGroup ipsec.IKEGroup,
	table *SATable,
	tr *transport.UDPTransport,
	bus ze.EventBus,
	log *slog.Logger,
) error {
	if peer.ConnectionType == ipsec.ConnectionInitiate {
		return ps.runInitiator(peer, ikeGroup, table, tr, bus, log)
	}
	return ps.runResponder(peer, ikeGroup, table, tr, bus, log)
}

// runInitiator drives the initiator side of the IKE exchange.
func (ps *PeerSession) runInitiator(
	peer ipsec.SiteToSitePeer,
	ikeGroup ipsec.IKEGroup,
	table *SATable,
	tr *transport.UDPTransport,
	bus ze.EventBus,
	log *slog.Logger,
) error {
	sa, err := newInitiatorSA(ps.peerName, peer, ikeGroup, ps.espGroup)
	if err != nil {
		return fmt.Errorf("ike: create SA: %w", err)
	}
	ps.sa = sa

	// RFC 7296 Section 2.6: initiator inserts SA with zero responder SPI initially.
	table.Insert(sa)

	remote, err := net.ResolveUDPAddr("udp4", peer.RemoteAddress+":500")
	if err != nil {
		return fmt.Errorf("ike: resolve remote: %w", err)
	}

	initMsg := buildSAInitRequest(sa, ikeGroup)
	sa.InitiatorSAInitMsg = initMsg
	sa.State = StateSAInitSent
	sa.LastSentMsg = initMsg

	if tr != nil {
		if err := tr.Send(initMsg, remote); err != nil {
			return fmt.Errorf("ike: send sa-init: %w", err)
		}
	}
	log.Debug("ike: sent IKE_SA_INIT", "peer", ps.peerName)

	// Wait for responses or retransmit.
	sa.RetransmitTime = time.Now().Add(retransmitBase)
	sa.RetransmitCount = 0

	for sa.State != StateEstablished {
		if ps.stopped() {
			return errStopped
		}

		timeout := time.Until(sa.RetransmitTime)
		if timeout <= 0 {
			if sa.RetransmitCount >= maxRetransmissions {
				return errTimeout
			}
			sa.RetransmitCount++
			// RFC 7296 Section 2.1: exponential backoff.
			delay := retransmitBackoff(sa.RetransmitCount)
			sa.RetransmitTime = time.Now().Add(delay)
			if tr != nil && sa.LastSentMsg != nil {
				if err := tr.Send(sa.LastSentMsg, remote); err != nil {
					log.Warn("ike: retransmit failed", "peer", ps.peerName, "error", err)
				}
			}
			log.Debug("ike: retransmit", "peer", ps.peerName, "attempt", sa.RetransmitCount)
			continue
		}

		select {
		case <-ps.stopCh:
			return errStopped
		case <-afterFunc(timeout):
		}
	}

	log.Info("ike: SA established", "peer", ps.peerName,
		"ispi", SPIHex(sa.InitiatorSPI), "rspi", SPIHex(sa.ResponderSPI))

	if bus != nil {
		if _, emitErr := SAUp.Emit(bus, &SAEvent{
			PeerName:      sa.PeerName,
			InitiatorSPI:  SPIHex(sa.InitiatorSPI),
			ResponderSPI:  SPIHex(sa.ResponderSPI),
			RemoteAddress: peer.RemoteAddress,
			AuthMethod:    peer.Auth.Mode.String(),
		}); emitErr != nil {
			log.Warn("ike: emit sa-up failed", "error", emitErr)
		}
	}

	return ps.runEstablished(sa, peer, ikeGroup, table, tr, bus, log)
}

// runResponder waits for incoming IKE_SA_INIT as a responder.
func (ps *PeerSession) runResponder(
	_ ipsec.SiteToSitePeer,
	_ ipsec.IKEGroup,
	_ *SATable,
	_ *transport.UDPTransport,
	_ ze.EventBus,
	log *slog.Logger,
) error {
	log.Debug("ike: responder waiting", "peer", ps.peerName)
	// Responder waits for inbound IKE_SA_INIT dispatched via handleInbound.
	<-ps.stopCh
	return nil
}

// handleInbound processes an inbound packet for an existing SA.
func handleInbound(sa *SA, pkt transport.Packet, table *SATable, tr *transport.UDPTransport, log *slog.Logger) {
	var msg wire.Message
	if err := msg.ReadFrom(pkt.Data); err != nil {
		log.Debug("ike: parse error", "error", err)
		return
	}

	switch sa.State {
	case StateSAInitSent:
		if msg.Header.ExchangeType == wire.ExchangeIKESAInit &&
			msg.Header.Flags&wire.FlagResponse != 0 {
			handleSAInitResponse(sa, &msg, pkt.Data, table, tr, pkt.RemoteAddr, log)
		}
	case StateAuthSent:
		if msg.Header.ExchangeType == wire.ExchangeIKEAuth &&
			msg.Header.Flags&wire.FlagResponse != 0 {
			handleAuthResponse(sa, &msg, pkt.Data, table, tr, log)
		}
	case StateEstablished:
		handleEstablishedInbound(sa, &msg, log)
	case StateIdle, StateSAInitReceived, StateAuthReceived, StateDead:
		log.Debug("ike: message in unexpected state", "state", sa.State)
	}
}

// handleSAInitResponse processes an IKE_SA_INIT response for an initiator.
func handleSAInitResponse(
	sa *SA,
	msg *wire.Message,
	rawMsg []byte,
	table *SATable,
	tr *transport.UDPTransport,
	remote *net.UDPAddr,
	log *slog.Logger,
) {
	copy(sa.ResponderSPI[:], msg.Header.ResponderSPI[:])
	sa.ResponderSAInitMsg = make([]byte, len(rawMsg))
	copy(sa.ResponderSAInitMsg, rawMsg)

	// Update table key now that we know the responder SPI.
	table.UpdateKey([8]byte{}, sa.ResponderSPI, sa)

	var remoteSA *wire.PayloadSA
	var remoteKE *wire.PayloadKE
	var remoteNonce *wire.PayloadNonce

	for _, pe := range msg.Payloads {
		switch p := pe.Payload.(type) {
		case *wire.PayloadSA:
			remoteSA = p
		case *wire.PayloadKE:
			remoteKE = p
		case *wire.PayloadNonce:
			remoteNonce = p
		case *wire.PayloadNotify:
			if p.NotifyMsgType == wire.NotifyNoProposalChosen {
				log.Warn("ike: remote sent NO_PROPOSAL_CHOSEN", "peer", sa.PeerName)
				sa.State = StateDead
				return
			}
			if p.NotifyMsgType == wire.NotifySignatureHashAlgorithms {
				sa.RemoteHashAlgos = parseHashAlgoNotify(p.NotificationData)
			}
			// RFC 7296 Section 2.23: check NAT detection notify payloads.
			// SOURCE_IP from responder: hash of responder's own address.
			// Mismatch means responder is behind NAT.
			if p.NotifyMsgType == wire.NotifyNATDetectionSourceIP {
				remoteIP := net.ParseIP(sa.PeerCfg.RemoteAddress)
				if remoteIP != nil {
					expected := transport.NATDetectionHash(sa.InitiatorSPI, sa.ResponderSPI, remoteIP, transport.IKEPort)
					if !natHashEqual(p.NotificationData, expected) {
						sa.NATDetected = true
					}
				}
			}
			// DESTINATION_IP from responder: hash of us as the responder sees us.
			// Mismatch means our address was translated (we are behind NAT).
			if p.NotifyMsgType == wire.NotifyNATDetectionDestIP {
				localIP := net.ParseIP(sa.PeerCfg.LocalAddress)
				if localIP != nil {
					expected := transport.NATDetectionHash(sa.InitiatorSPI, sa.ResponderSPI, localIP, transport.IKEPort)
					if !natHashEqual(p.NotificationData, expected) {
						sa.NATDetected = true
						sa.BehindNAT = true
					}
				}
			}
		}
	}

	if remoteSA == nil || remoteKE == nil || remoteNonce == nil {
		log.Warn("ike: incomplete IKE_SA_INIT response", "peer", sa.PeerName)
		sa.State = StateDead
		return
	}

	// Negotiate proposal.
	localProposals := buildIKEProposals(sa.IKEGroup)
	remoteProposals := wireProposalsToIKE(remoteSA.Proposals)
	chosen, err := crypto.NegotiateIKE(remoteProposals, localProposals)
	if err != nil {
		log.Warn("ike: proposal negotiation failed", "peer", sa.PeerName, "error", err)
		sa.State = StateDead
		return
	}
	sa.Proposal = chosen

	// Process DH exchange.
	sa.RemoteNonce = remoteNonce.NonceData
	sa.RemoteDHPub = remoteKE.KeyExchangeData

	sharedSecret, err := sa.LocalDH.SharedSecret(sa.RemoteDHPub)
	if err != nil {
		log.Warn("ike: DH shared secret failed", "peer", sa.PeerName, "error", err)
		sa.State = StateDead
		return
	}

	// Derive keys.
	skeyseed, err := crypto.DeriveSKEYSEED(chosen.PRF.ID, sa.LocalNonce, sa.RemoteNonce, sharedSecret)
	if err != nil {
		log.Warn("ike: SKEYSEED derivation failed", "peer", sa.PeerName, "error", err)
		sa.State = StateDead
		return
	}

	skKeys, err := crypto.DeriveSKKeys(
		chosen.PRF.ID, skeyseed,
		sa.LocalNonce, sa.RemoteNonce,
		sa.InitiatorSPI[:], sa.ResponderSPI[:],
		chosen.Encryption, chosen.Integrity,
	)
	if err != nil {
		log.Warn("ike: SK key derivation failed", "peer", sa.PeerName, "error", err)
		sa.State = StateDead
		return
	}
	sa.SKKeys = skKeys
	sa.LocalDH.Clear()
	clear(sharedSecret)
	clear(skeyseed)

	// Build and send IKE_AUTH request.
	authReq, err := buildAuthRequest(sa)
	if err != nil {
		log.Warn("ike: build IKE_AUTH failed", "peer", sa.PeerName, "error", err)
		sa.State = StateDead
		return
	}

	sa.State = StateAuthSent
	sa.LastSentMsg = authReq
	sa.NextMsgID = 1
	sa.RetransmitTime = time.Now().Add(retransmitBase)
	sa.RetransmitCount = 0

	if tr != nil && remote != nil {
		if err := sendWithNATT(sa, authReq, tr, remote); err != nil {
			log.Warn("ike: send IKE_AUTH failed", "peer", sa.PeerName, "error", err)
		}
	}
	log.Debug("ike: sent IKE_AUTH", "peer", sa.PeerName)
}

// handleAuthResponse processes an IKE_AUTH response for an initiator.
// Decrypts the SK payload, verifies AUTH, and extracts the negotiated
// Child SA parameters (SA, TSi, TSr) piggybacked on IKE_AUTH.
func handleAuthResponse(sa *SA, msg *wire.Message, rawMsg []byte, _ *SATable, _ *transport.UDPTransport, log *slog.Logger) {
	var skPayload *wire.PayloadSK
	for _, pe := range msg.Payloads {
		if sk, ok := pe.Payload.(*wire.PayloadSK); ok {
			skPayload = sk
		}
	}
	if skPayload == nil {
		log.Warn("ike: AUTH response has no SK payload", "peer", sa.PeerName)
		sa.State = StateDead
		return
	}

	plaintext, err := decryptSKPayload(sa, rawMsg, skPayload)
	if err != nil {
		log.Warn("ike: AUTH response decryption failed", "peer", sa.PeerName, "error", err)
		sa.State = StateDead
		return
	}

	innerPayloads, err := wire.ParsePayloadChain(plaintext, skPayload.InnerNextPayload)
	if err != nil {
		log.Warn("ike: AUTH response inner payload parse failed", "peer", sa.PeerName, "error", err)
		sa.State = StateDead
		return
	}

	authVerified := false
	for _, pe := range innerPayloads {
		switch p := pe.Payload.(type) {
		case *wire.PayloadNotify:
			if p.NotifyMsgType == wire.NotifyAuthenticationFailed {
				log.Warn("ike: remote sent AUTHENTICATION_FAILED", "peer", sa.PeerName)
				sa.State = StateDead
				return
			}
		case *wire.PayloadID:
			if p.IDPayloadType == wire.PayloadTypeIDr {
				sa.RemoteIDPayload = p
			}
		case *wire.PayloadAUTH:
			if err := verifyRemoteAuth(sa, p); err != nil {
				log.Warn("ike: remote AUTH verification failed", "peer", sa.PeerName, "error", err)
				sa.State = StateDead
				return
			}
			authVerified = true
		case *wire.PayloadSA:
			for _, prop := range p.Proposals {
				if prop.ProtocolID == wire.ProtocolESP && prop.SPISize == 4 && len(prop.SPI) >= 4 {
					sa.ChildOutboundSPI = binary.BigEndian.Uint32(prop.SPI[:4])
				}
			}
		case *wire.PayloadTS:
			switch p.TSPayloadType {
			case wire.PayloadTypeTSi:
				sa.NegotiatedTSi = tsToIPNet(p.TrafficSelectors)
			case wire.PayloadTypeTSr:
				sa.NegotiatedTSr = tsToIPNet(p.TrafficSelectors)
			}
		}
	}

	if !authVerified {
		log.Warn("ike: AUTH response missing AUTH payload", "peer", sa.PeerName)
		sa.State = StateDead
		return
	}

	sa.State = StateEstablished
	sa.EstablishedAt = time.Now()
}

// retransmitBackoff computes the delay for retransmission attempt n.
// RFC 7296 Section 2.1: exponential backoff capped at retransmitMax.
func retransmitBackoff(attempt int) time.Duration {
	if attempt <= 0 {
		return retransmitBase
	}
	shift := min(attempt-1, 7)
	d := retransmitBase * time.Duration(1<<uint(shift))
	if d > retransmitMax {
		return retransmitMax
	}
	return d
}
