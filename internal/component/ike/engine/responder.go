// Design: plan/learned/1072-ipsec-14-responder.md -- IKE responder handshake (mirror of the initiator)
// RFC: rfc/short/rfc7296.md -- IKE_SA_INIT / IKE_AUTH responder (Sections 1.2, 2.4, 2.15)

package engine

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"time"

	"github.com/ze-software/ze/internal/component/ike/crypto"
	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/component/ike/wire"
)

// newResponderSA creates the SA for an inbound (responder) IKE exchange. The peer
// is the IKE-SA initiator, so InitiatorSPI is taken from the request and we
// generate our own ResponderSPI; IsInitiator is false, which flips SK key
// direction, AUTH octets, and Child SA key roles throughout the engine. The DH
// exchange is created later in handleSAInitRequest, once the group is negotiated.
func newResponderSA(peerName string, peer ipsec.SiteToSitePeer, ikeGroup ipsec.IKEGroup, espGroup ipsec.ESPGroup, initiatorSPI [8]byte) (*SA, error) {
	rspi, err := GenerateSPI()
	if err != nil {
		return nil, err
	}
	nonce, err := GenerateNonce(nonceLen)
	if err != nil {
		return nil, err
	}
	return &SA{
		PeerName:     peerName,
		PeerCfg:      peer,
		IKEGroup:     ikeGroup,
		ESPGroup:     espGroup,
		InitiatorSPI: initiatorSPI,
		ResponderSPI: rspi,
		IsInitiator:  false,
		State:        StateIdle,
		LocalNonce:   nonce,
		CreatedAt:    time.Now(),
	}, nil
}

// handleResponderInbound processes inbound handshake requests for a responder SA.
// It runs on the shared dispatch goroutine (established=false); post-establishment
// traffic is routed to the owner loop instead. Every message here is a request from
// the initiator. RFC 7296 Section 1.2, Section 2.16.
func (ps *PeerSession) handleResponderInbound(sa *SA, msg *wire.Message, pkt transport.Packet, tr *transport.UDPTransport, log *slog.Logger) {
	if msg.Header.Flags&wire.FlagResponse != 0 {
		log.Debug("ike: responder ignoring unexpected response", "peer", sa.PeerName, "state", sa.State)
		return
	}
	switch sa.State {
	case StateIdle:
		if msg.Header.ExchangeType == wire.ExchangeIKESAInit {
			handleSAInitRequest(sa, msg, pkt.Data, tr, pkt.RemoteAddr, log)
		}
	case StateSAInitReceived:
		switch msg.Header.ExchangeType {
		case wire.ExchangeIKESAInit:
			resendResponderSAInit(sa, tr, pkt.RemoteAddr, log)
		case wire.ExchangeIKEAuth:
			ps.handleAuthRequest(sa, msg, pkt.Data, tr, pkt.RemoteAddr, log)
		}
	case StateEAPInProgress:
		if msg.Header.ExchangeType == wire.ExchangeIKEAuth {
			ps.handleResponderEAP(sa, msg, pkt.Data, tr, pkt.RemoteAddr, log)
		}
	case StateEstablished:
		// Narrow window before runResponder adopts the SA into the owner loop: a
		// retransmitted final IKE_AUTH is answered from the cached response.
		if sa.lastResponseSet && msg.Header.MessageID == sa.lastResponseID && tr != nil && pkt.RemoteAddr != nil {
			if err := tr.Send(sa.lastResponse, pkt.RemoteAddr); err != nil {
				log.Debug("ike: resend cached IKE_AUTH response failed", "peer", sa.PeerName, "error", err)
			}
		}
	case StateAuthSent, StateAuthReceived, StateSAInitSent, StateDead:
		log.Debug("ike: responder message in unexpected state", "peer", sa.PeerName, "state", sa.State)
	}
}

// handleSAInitRequest processes an inbound IKE_SA_INIT request as the responder:
// negotiate a proposal, complete the DH exchange, derive the SK_* hierarchy, and
// send the IKE_SA_INIT response. Mirrors handleSAInitResponse (initiator).
// RFC 7296 Section 1.2, Section 2.7.
func handleSAInitRequest(sa *SA, msg *wire.Message, rawMsg []byte, tr *transport.UDPTransport, remote *net.UDPAddr, log *slog.Logger) {
	sa.InitiatorSAInitMsg = append([]byte(nil), rawMsg...)

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
			if p.NotifyMsgType == wire.NotifySignatureHashAlgorithms {
				sa.RemoteHashAlgos = parseHashAlgoNotify(p.NotificationData)
			}
		}
	}
	if remoteSA == nil || remoteKE == nil || remoteNonce == nil {
		log.Warn("ike: incomplete IKE_SA_INIT request", "peer", sa.PeerName)
		sa.State = StateDead
		return
	}
	// RFC 7296 Section 3.3: the first Proposal MUST have a Proposal Num of one.
	// Each later structure MUST be one greater than the previous one.
	// This message is a request, so its SA payload is always an offer.
	// A response carries the accepted number instead (Section 3.3.1).
	// That number is not checked here.
	if err := remoteSA.ValidateOfferNumbering(); err != nil {
		log.Warn("ike: IKE_SA_INIT request has misnumbered proposals",
			"peer", sa.PeerName, "error", err)
		sa.State = StateDead
		return
	}
	// RFC 7296 Section 3.3.1: for an initial IKE SA negotiation the SPI Size field MUST be
	// zero, because the SPI comes from the outer header. An IKE_SA_INIT IS that initial
	// negotiation, so the rule applies here and nowhere the exchange is unknown. A later
	// negotiation runs as a CREATE_CHILD_SA, and it carries an 8-octet SPI.
	if err := remoteSA.ValidateInitialSPISize(); err != nil {
		log.Warn("ike: IKE_SA_INIT request carries an SPI in its proposals",
			"peer", sa.PeerName, "error", err)
		sa.State = StateDead
		return
	}

	// RFC 7296 Section 2.7: responder selects exactly one proposal.
	localProposals := buildIKEProposals(sa.IKEGroup)
	remoteProposals := wireProposalsToIKE(remoteSA.Proposals)
	chosen, err := crypto.NegotiateIKE(remoteProposals, localProposals)
	if err != nil {
		log.Warn("ike: no acceptable proposal from initiator", "peer", sa.PeerName)
		sendSAInitNotify(sa, tr, remote, wire.NotifyNoProposalChosen, nil, log)
		sa.State = StateDead
		return
	}
	sa.Proposal = chosen
	logKeyLengthUpgrade(log, sa.PeerName, chosen)

	// RFC 7296 Section 1.2: the KEi group must equal the selected DH group, else
	// respond with INVALID_KE_PAYLOAD carrying the group we accept.
	dhGroupID := chosen.DHGroup.ID
	if uint16(dhGroupID) != remoteKE.DHGroup {
		log.Info("ike: initiator KE group mismatch", "peer", sa.PeerName, "want", uint16(dhGroupID), "got", remoteKE.DHGroup)
		grp := []byte{byte(uint16(dhGroupID) >> 8), byte(dhGroupID)}
		sendSAInitNotify(sa, tr, remote, wire.NotifyInvalidKEPayload, grp, log)
		sa.State = StateDead
		return
	}

	dh, err := crypto.NewDHExchange(dhGroupID)
	if err != nil {
		log.Warn("ike: responder DH create failed", "peer", sa.PeerName, "error", err)
		sa.State = StateDead
		return
	}
	sa.LocalDH = dh
	sa.RemoteNonce = remoteNonce.NonceData
	sa.RemoteDHPub = remoteKE.KeyExchangeData

	detectResponderNAT(sa, msg)

	sharedSecret, err := dh.SharedSecret(sa.RemoteDHPub)
	if err != nil {
		log.Warn("ike: responder DH shared secret failed", "peer", sa.PeerName, "error", err)
		sa.State = StateDead
		return
	}

	// RFC 7296 Section 2.14: SKEYSEED and SK_* use absolute Ni | Nr | SPIi | SPIr.
	skeyseed, err := crypto.DeriveSKEYSEED(chosen.PRF.ID, sa.initiatorNonce(), sa.responderNonce(), sharedSecret)
	if err != nil {
		clear(sharedSecret)
		log.Warn("ike: responder SKEYSEED failed", "peer", sa.PeerName, "error", err)
		sa.State = StateDead
		return
	}
	skKeys, err := crypto.DeriveSKKeys(
		chosen.PRF.ID, skeyseed,
		sa.initiatorNonce(), sa.responderNonce(),
		sa.InitiatorSPI[:], sa.ResponderSPI[:],
		chosen.Encryption, chosen.Integrity,
	)
	clear(sharedSecret)
	clear(skeyseed)
	if err != nil {
		log.Warn("ike: responder SK key derivation failed", "peer", sa.PeerName, "error", err)
		sa.State = StateDead
		return
	}
	sa.SKKeys = skKeys

	resp, err := buildSAInitResponse(sa, chosen)
	if err != nil {
		log.Warn("ike: IKE_SA_INIT response too large, dropping", "peer", sa.PeerName, "error", err)
		sa.State = StateDead
		return
	}
	sa.ResponderSAInitMsg = append([]byte(nil), resp...)
	sa.LocalDH.Clear()

	// RFC 7296 Section 2.3: cache the response and advance the expected request ID
	// so the peer's IKE_AUTH (message ID 1) is accepted next.
	cacheResponse(sa, msg.Header.MessageID, resp)
	sa.State = StateSAInitReceived
	sa.LastSentMsg = resp
	if tr != nil && remote != nil {
		if err := tr.Send(resp, remote); err != nil {
			log.Warn("ike: send IKE_SA_INIT response failed", "peer", sa.PeerName, "error", err)
		}
	}
	log.Debug("ike: sent IKE_SA_INIT response (responder)", "peer", sa.PeerName,
		"ispi", SPIHex(sa.InitiatorSPI), "rspi", SPIHex(sa.ResponderSPI))
}

// resendResponderSAInit replays the cached IKE_SA_INIT response for a retransmitted
// request (the initiator did not receive our first response). RFC 7296 Section 2.3.
func resendResponderSAInit(sa *SA, tr *transport.UDPTransport, remote *net.UDPAddr, log *slog.Logger) {
	if len(sa.ResponderSAInitMsg) == 0 || tr == nil || remote == nil {
		return
	}
	if err := tr.Send(sa.ResponderSAInitMsg, remote); err != nil {
		log.Debug("ike: resend IKE_SA_INIT response failed", "peer", sa.PeerName, "error", err)
	}
}

// buildSAInitResponse builds the plaintext IKE_SA_INIT response carrying the single
// chosen proposal, our KE, our nonce, the signature-hash notify, and NAT detection.
// It returns an error (without writing) if the encoded message would not fit the
// fixed buffer, so the caller aborts rather than send a truncated IKE message.
func buildSAInitResponse(sa *SA, chosen crypto.IKEProposal) ([]byte, error) {
	payloads := []wire.PayloadEntry{
		{Payload: &wire.PayloadSA{Proposals: chosenIKEProposalToWire(chosen)}},
		{Payload: &wire.PayloadKE{DHGroup: uint16(chosen.DHGroup.ID), KeyExchangeData: sa.LocalDH.PublicKey}},
		{Payload: &wire.PayloadNonce{NonceData: sa.LocalNonce}},
		{Payload: buildSignatureHashAlgosNotify()},
	}

	localIP := net.ParseIP(sa.PeerCfg.LocalAddress)
	remoteIP := net.ParseIP(sa.PeerCfg.RemoteAddress)
	if localIP != nil && remoteIP != nil {
		payloads = append(payloads, buildNATDetectionPayloads(sa.InitiatorSPI, sa.ResponderSPI, localIP, remoteIP, transport.IKEPort)...)
	}

	msg := wire.Message{
		Header: wire.Header{
			InitiatorSPI: sa.InitiatorSPI,
			ResponderSPI: sa.ResponderSPI,
			MajorVersion: 2,
			MinorVersion: 0,
			ExchangeType: wire.ExchangeIKESAInit,
			Flags:        wire.FlagResponse,
			MessageID:    0,
		},
		Payloads: payloads,
	}
	buf := make([]byte, 4096)
	n, err := msg.CheckedWriteTo(buf, 0)
	if err != nil {
		return nil, err
	}
	return buf[:n], nil
}

// chosenIKEProposalToWire encodes a single negotiated IKE proposal for the SAr
// payload of the IKE_SA_INIT response (RFC 7296 Section 3.3: one proposal).
func chosenIKEProposalToWire(chosen crypto.IKEProposal) []wire.Proposal {
	integID := uint16(chosen.Integrity.ID)
	if chosen.Encryption.IsAEAD {
		integID = uint16(crypto.AUTH_NONE)
	}
	transforms := []wire.Transform{
		{Type: wire.TransformTypeENCR, ID: uint16(chosen.Encryption.ID), Attrs: encAttrs(chosen.Encryption)},
		{Type: wire.TransformTypePRF, ID: uint16(chosen.PRF.ID)},
		{Type: wire.TransformTypeINTG, ID: integID},
		{Type: wire.TransformTypeDH, ID: uint16(chosen.DHGroup.ID)},
	}
	return []wire.Proposal{{
		Number:     uint8(chosen.Number),
		ProtocolID: wire.ProtocolIKE,
		SPISize:    0,
		Transforms: transforms,
	}}
}

// detectResponderNAT inspects the initiator's NAT_DETECTION notifies. The initiator
// computed them with the responder SPI still zero (it did not know ours yet), so we
// verify against the SPIs as they appear in the request header. RFC 7296 Section 2.23.
func detectResponderNAT(sa *SA, msg *wire.Message) {
	peerIP := net.ParseIP(sa.PeerCfg.RemoteAddress)
	localIP := net.ParseIP(sa.PeerCfg.LocalAddress)
	for _, pe := range msg.Payloads {
		p, ok := pe.Payload.(*wire.PayloadNotify)
		if !ok {
			continue
		}
		switch p.NotifyMsgType {
		case wire.NotifyNATDetectionSourceIP:
			if peerIP != nil {
				expected := transport.NATDetectionHash(msg.Header.InitiatorSPI, msg.Header.ResponderSPI, peerIP, transport.IKEPort)
				if !natHashEqual(p.NotificationData, expected) {
					sa.NATDetected = true
				}
			}
		case wire.NotifyNATDetectionDestIP:
			if localIP != nil {
				expected := transport.NATDetectionHash(msg.Header.InitiatorSPI, msg.Header.ResponderSPI, localIP, transport.IKEPort)
				if !natHashEqual(p.NotificationData, expected) {
					sa.NATDetected = true
					sa.BehindNAT = true
				}
			}
		}
	}
}

// sendSAInitNotify sends an unencrypted IKE_SA_INIT response carrying a single
// notify (NO_PROPOSAL_CHOSEN / INVALID_KE_PAYLOAD). RFC 7296 Section 2.21.
func sendSAInitNotify(sa *SA, tr *transport.UDPTransport, remote *net.UDPAddr, notifyType uint16, data []byte, log *slog.Logger) {
	if tr == nil || remote == nil {
		return
	}
	msg := wire.Message{
		Header: wire.Header{
			InitiatorSPI: sa.InitiatorSPI,
			ResponderSPI: sa.ResponderSPI,
			MajorVersion: 2,
			ExchangeType: wire.ExchangeIKESAInit,
			Flags:        wire.FlagResponse,
			MessageID:    0,
		},
		Payloads: []wire.PayloadEntry{{Payload: &wire.PayloadNotify{NotifyMsgType: notifyType, NotificationData: data}}},
	}
	buf := make([]byte, 512)
	n, err := msg.CheckedWriteTo(buf, 0)
	if err != nil {
		// RFC 7296 Section 3: a truncated IKE message is malformed. Drop rather
		// than send a partial notify (ai/rules/no-workarounds-for-missing-behavior.md).
		log.Warn("ike: SA_INIT notify too large, dropping", "peer", sa.PeerName, "notify", notifyType, "error", err)
		return
	}
	if err := tr.Send(buf[:n], remote); err != nil {
		log.Debug("ike: send SA_INIT notify failed", "peer", sa.PeerName, "error", err)
	}
}

// handleAuthRequest processes an inbound IKE_AUTH request as the responder:
// decrypt, verify the initiator's AUTH (PSK/X.509) or begin EAP, install the first
// Child SA, and send the IKE_AUTH response, establishing the SA. RFC 7296 Section 1.2.
func (ps *PeerSession) handleAuthRequest(sa *SA, msg *wire.Message, rawMsg []byte, tr *transport.UDPTransport, remote *net.UDPAddr, log *slog.Logger) {
	inner, err := decryptAndParse(sa, msg, rawMsg)
	if err != nil {
		log.Warn("ike: IKE_AUTH request decrypt failed", "peer", sa.PeerName, "error", err)
		sa.State = StateDead
		return
	}

	var authPayload *wire.PayloadAUTH
	var remoteSAi2 *wire.PayloadSA
	var tsi, tsr *wire.PayloadTS
	var certPayloads []*wire.PayloadCERT
	var setWindowSize *wire.PayloadNotify
	// RFC 7296 Section 2.5 forbids rejecting a message over payload order, so this walk
	// only collects and every check runs after it.
	for _, pe := range inner {
		switch p := pe.Payload.(type) {
		case *wire.PayloadID:
			if p.IDPayloadType == wire.PayloadTypeIDi {
				sa.RemoteIDPayload = p
			}
		case *wire.PayloadAUTH:
			authPayload = p
		case *wire.PayloadCERT:
			if p.CertEncoding == wire.CertEncodingX509Sig && len(p.CertData) > 0 {
				certPayloads = append(certPayloads, p)
			}
		case *wire.PayloadNotify:
			// RFC 7296 Section 2.4: honor INITIAL_CONTACT -- the peer asserts this is the
			// only IKE SA to its identity, so we may drop any stale SA to it once this
			// IKE_AUTH authenticates (finishResponderEstablish supersede).
			if p.NotifyMsgType == wire.NotifyInitialContact {
				sa.InitialContact = true
			}
			// RFC 7296 Section 2.3: the peer states how many outstanding requests it
			// keeps. IKE_AUTH is the read point, because "The window size is always one
			// until the initial exchanges complete".
			if p.NotifyMsgType == wire.NotifySetWindowSize {
				setWindowSize = p
			}
		case *wire.PayloadSA:
			remoteSAi2 = p
		case *wire.PayloadTS:
			switch p.TSPayloadType {
			case wire.PayloadTypeTSi:
				tsi = p
			case wire.PayloadTypeTSr:
				tsr = p
			}
		}
	}

	storeRemoteCerts(sa, certPayloads)

	if err := recordPeerWindowSize(sa, setWindowSize); err != nil {
		log.Warn("ike: peer SET_WINDOW_SIZE refused", "peer", sa.PeerName, "error", err)
		sa.State = StateDead
		return
	}

	// RFC 7296 Section 2.16: no AUTH payload signals the initiator wants EAP.
	if authPayload == nil {
		if ipsec.IsEAPMode(sa.PeerCfg.Auth.Mode) {
			ps.startResponderEAP(sa, msg.Header.MessageID, remoteSAi2, tsi, tsr, tr, remote, log)
			return
		}
		log.Warn("ike: IKE_AUTH request without AUTH and not EAP", "peer", sa.PeerName)
		sa.State = StateDead
		return
	}

	if err := verifyRemoteAuth(sa, authPayload); err != nil {
		log.Warn("ike: initiator AUTH verification failed", "peer", sa.PeerName, "error", err)
		ps.sendAuthFailed(sa, msg.Header.MessageID, tr, remote, log)
		sa.State = StateDead
		return
	}

	resp, child, err := ps.buildAuthResponse(sa, msg.Header.MessageID, remoteSAi2, tsi, tsr, false, log)
	if err != nil {
		log.Warn("ike: build IKE_AUTH response failed", "peer", sa.PeerName, "error", err)
		sa.State = StateDead
		return
	}
	ps.finishResponderEstablish(sa, msg.Header.MessageID, resp, child, tr, remote, log)
}

// selectResponderESP picks one ESP proposal from the initiator's SAi2 that our
// esp-group accepts and narrows sa.ESPGroup to it, so the IKE_AUTH response carries
// exactly one proposal (RFC 7296 Section 2.7, Section 3.3) and the installed Child SA
// uses the negotiated algorithm (createFirstChildSA keys from Proposals[0]).
//
// It also records the Proposal Num the peer put on that proposal. RFC 7296
// Section 3.3.1 makes the response carry the peer's number, not our config key.
//
// A nil remoteSAi2 (the EAP final IKE_AUTH) is a no-op. Selection already ran on the
// first IKE_AUTH (startResponderEAP).
func selectResponderESP(sa *SA, remoteSAi2 *wire.PayloadSA) error {
	if remoteSAi2 == nil {
		return nil
	}
	for i := range sa.ESPGroup.Proposals {
		our := sa.ESPGroup.Proposals[i]
		rp, ok := matchOfferedESPProposal(remoteSAi2, our)
		if !ok {
			continue
		}
		// RFC 7296 Section 3.3 numbers the first proposal of an offer one, so a
		// proposal numbered zero is malformed. Refuse it rather than answer with a
		// number the peer cannot match (ai/rules/fail-closed-guards.md).
		if rp.Number == 0 {
			return crypto.ErrNoProposalChosen
		}
		sa.ESPGroup.Proposals = []ipsec.ESPProposal{our}
		sa.ChildProposalNum = rp.Number
		return nil
	}
	return crypto.ErrNoProposalChosen
}

// matchOfferedESPProposal returns the ESP proposal in the peer's offer that agrees
// with one configured proposal, and reports whether the offer holds one. The caller
// reads its Proposal Num, which RFC 7296 Section 3.3.1 makes the response echo.
func matchOfferedESPProposal(offer *wire.PayloadSA, our ipsec.ESPProposal) (wire.Proposal, bool) {
	if offer == nil {
		return wire.Proposal{}, false
	}
	enc := lookupEncryption(our.Encryption)
	aead := our.Encryption.IsAEAD()
	var integID uint16
	if !aead {
		integID = uint16(lookupIntegrity(our.Hash).ID)
	}
	for _, rp := range offer.Proposals {
		if rp.ProtocolID == wire.ProtocolESP &&
			espProposalMatches(rp, uint16(enc.ID), enc.KeyLength, integID, aead) {
			return rp, true
		}
	}
	return wire.Proposal{}, false
}

// logKeyLengthUpgrade reports an encryption key that the responder accepted above its own
// policy. RFC 7296 Section 3.3.5 lets a responder accept a key that supplies greater
// security, and crypto.NegotiateIKE records the configured length when it does. The
// operator asked for the shorter key, so the running key is stated rather than silent
// (ai/rules/exact-or-reject.md). A responder that accepts the configured length logs
// nothing.
func logKeyLengthUpgrade(log *slog.Logger, peer string, chosen crypto.IKEProposal) {
	if chosen.PolicyKeyLength == 0 {
		return
	}
	log.Warn("ike: accepted an encryption key longer than the configured one",
		"peer", peer,
		"configured-bits", chosen.PolicyKeyLength,
		"accepted-bits", chosen.Encryption.KeyLength)
}

// espProposalMatches reports whether a wire ESP proposal offers exactly the given
// ENCR id + key length and integrity (integrity NONE for AEAD).
//
// RFC 7296 Section 3.3.6 makes a proposal that carries a Transform Type the responder does
// not understand unacceptable. RFC 7296 Section 3.3.3 gives ESP the types ENCR and ESN,
// with INTEG and D-H optional. A proposal that carries any other type does not match.
//
// DELIBERATE DEVIATION, Section 3.3.5. The implementation note there says an implementer
// "SHOULD accept values that they deem to supply greater security", which covers ESP as
// much as IKE. The IKE responder does accept a longer key (crypto.NegotiateIKE under
// keyLengthAtLeast, reported by logKeyLengthUpgrade). This ESP comparison does not: it
// requires gotKeyLen to equal the configured length exactly.
//
// The reason is that accepting one here would key the two ends differently. Section 3.3.5
// also states that the Key Length attribute "is always returned unchanged". A responder
// that accepts a longer key MUST therefore echo it, and MUST derive at it. Ze does
// neither. selectResponderESP narrows sa.ESPGroup to the CONFIGURED proposal, and
// createFirstChildSA reads its encryption from espGroup.Proposals[0] (child.go:127). A
// peer that offered 256 would therefore derive at 256 while ze derived at 128, and the
// Child SA would carry no traffic.
//
// The deviation is in the safe direction: ze refuses rather than mis-keys, and the
// operator gets exactly the cipher configured. Closing it means plumbing the accepted key
// length through ESP proposal selection and Child SA key derivation, which is the
// dataplane keying path. It is not a comparison change.
func espProposalMatches(p wire.Proposal, encID, keyLen, integID uint16, aead bool) bool {
	var gotEnc, gotKeyLen, gotInteg uint16
	hasEnc, hasInteg := false, false
	for _, t := range p.Transforms {
		if !crypto.TransformTypeUnderstoodESP(crypto.TransformType(t.Type)) {
			return false
		}
		switch t.Type {
		case wire.TransformTypeENCR:
			hasEnc = true
			gotEnc = t.ID
			for _, a := range t.Attrs {
				if a.Type == wire.AttrTypeKeyLength {
					gotKeyLen = a.Value
				}
			}
		case wire.TransformTypeINTG:
			hasInteg = true
			gotInteg = t.ID
		case wire.TransformTypeDH, wire.TransformTypeESN:
			// Both are types ESP uses, and neither takes part in the ENCR and INTG
			// comparison. A pseudorandom function transform never reaches here, because
			// ESP does not use that type.
		}
	}
	if !hasEnc || gotEnc != encID || gotKeyLen != keyLen {
		return false
	}
	if aead {
		return !hasInteg || gotInteg == 0
	}
	return hasInteg && gotInteg == integID
}

// buildAuthResponse negotiates and installs the first Child SA and builds the
// SK-encrypted IKE_AUTH response (IDr, [CERT], AUTH, SAr2, TSi, TSr). When fromEAP
// is true the AUTH is derived from the EAP MSK, otherwise from the configured
// credential. RFC 7296 Section 1.2, Section 2.17.
func (ps *PeerSession) buildAuthResponse(sa *SA, msgID uint32, remoteSAi2 *wire.PayloadSA, tsi, tsr *wire.PayloadTS, fromEAP bool, log *slog.Logger) ([]byte, *ChildSA, error) {
	// SAi2/TS come in the first IKE_AUTH. For a direct (PSK/X.509) exchange they are
	// passed here; for EAP they were parsed and stored on the SA during the first
	// IKE_AUTH (startResponderEAP), so the final call passes nil and reuses them.
	if remoteSAi2 != nil {
		outSPI, err := espSPIFromSA(remoteSAi2)
		if err != nil {
			return nil, nil, err
		}
		sa.ChildOutboundSPI = outSPI
	}
	if sa.ChildOutboundSPI == 0 {
		return nil, nil, errors.New("ike auth: no initiator ESP SPI (missing SAi2)")
	}

	// Record the initiator's proposed selectors so the installed Child SA and the
	// echoed TS agree (createFirstChildSA maps TSi/TSr to local/remote by role).
	if tsi != nil {
		sa.NegotiatedTSi = tsToIPNet(tsi.TrafficSelectors)
	}
	if tsr != nil {
		sa.NegotiatedTSr = tsToIPNet(tsr.TrafficSelectors)
	}

	// Select exactly one ESP proposal (narrows sa.ESPGroup); no-op on the EAP final
	// (already narrowed on the first IKE_AUTH). RFC 7296 Section 2.7.
	if err := selectResponderESP(sa, remoteSAi2); err != nil {
		return nil, nil, err
	}

	// The IKE_AUTH response is a RESPONSE, so SAr2 names one proposal and carries
	// the number the initiator put on it (RFC 7296 Sections 3.3, 3.3.1).
	espSPI, saPayload, respTSi, respTSr, err := buildChildSAResponsePayloads(sa)
	if err != nil {
		return nil, nil, err
	}
	sa.ChildInboundSPI = espSPI

	dp := dataplane.Get()
	ifID := resolveIfID(sa.PeerCfg)
	// Install with the negotiated single-proposal group (sa.ESPGroup), not the full
	// configured ps.espGroup, so the Child SA keys the algorithm the peer accepted.
	child, err := createFirstChildSA(sa, sa.ESPGroup, sa.PeerCfg.LocalAddress, sa.PeerCfg.RemoteAddress, ifID, dp, log)
	if err != nil {
		return nil, nil, err
	}

	var authPayload *wire.PayloadAUTH
	if fromEAP {
		authPayload, err = computeEAPAuth(sa)
	} else {
		authPayload, err = computeLocalAuth(sa)
	}
	if err != nil {
		if dp != nil {
			removeChildSA(child, dp, log)
		}
		return nil, nil, err
	}

	inner := make([]wire.PayloadEntry, 0, 6)
	inner = append(inner, wire.PayloadEntry{Payload: buildIDPayload(sa, false)})
	if sa.PeerCfg.Auth.Mode == ipsec.AuthX509 {
		inner = append(inner, buildCertPayloads(sa)...)
	}
	inner = append(inner,
		wire.PayloadEntry{Payload: authPayload},
		wire.PayloadEntry{Payload: saPayload},
		wire.PayloadEntry{Payload: respTSi},
		wire.PayloadEntry{Payload: respTSr},
	)

	resp, err := buildEncryptedMessageEx(sa, inner, msgID, wire.ExchangeIKEAuth, wire.FlagResponse)
	if err != nil {
		if dp != nil {
			removeChildSA(child, dp, log)
		}
		return nil, nil, fmt.Errorf("ike auth: build response: %w", err)
	}
	return resp, child, nil
}

// finishResponderEstablish caches and sends the IKE_AUTH response, adopts the
// installed Child SA, advances the message-ID counters (RFC 7296 Section 2.2:
// post-IKE_AUTH each side's request counter resumes at 2), and marks the SA
// established. runResponder observes the state change and takes over the owner loop.
func (ps *PeerSession) finishResponderEstablish(sa *SA, msgID uint32, resp []byte, child *ChildSA, tr *transport.UDPTransport, remote *net.UDPAddr, log *slog.Logger) {
	// A different SA already owns the loop => this is a PARALLEL re-initiation that has
	// now authenticated (we reached finishResponderEstablish only after verifyRemoteAuth).
	// RFC 7296 Section 2.4: the old SA is superseded ONLY here, on this authenticated
	// message -- never on the unauthenticated IKE_SA_INIT. Keep the new Child SA in the
	// second slot until the owner loop relinquishes and runResponder promotes it, so the
	// old owner's cleanupChild removes only its own child (make-before-break, R-2).
	parallel := ps.ownedSA.Load() != nil
	if parallel {
		ps.setPendingChild(child)
	} else {
		ps.setChildSA(child)
	}
	cacheResponse(sa, msgID, resp) // advances ExpectedMsgID to msgID+1 (=2)
	sa.NextMsgID = msgID + 1       // our next self-initiated request (DPD/Delete)
	sa.LastSentMsg = resp
	if tr != nil && remote != nil {
		if err := sendWithNATT(sa, resp, tr, remote); err != nil {
			log.Warn("ike: send IKE_AUTH response failed", "peer", sa.PeerName, "error", err)
		}
	}
	sa.State = StateEstablished
	sa.EstablishedAt = time.Now()
	log.Info("ike: responder established SA", "peer", sa.PeerName,
		"ispi", SPIHex(sa.InitiatorSPI), "rspi", SPIHex(sa.ResponderSPI),
		"parallel", parallel, "initial-contact", sa.InitialContact)
	if parallel {
		// RFC 7296 Section 2.4: "The INITIAL_CONTACT notification asserts that this IKE
		// SA is the only IKE SA currently active between the authenticated identities";
		// the recipient "MAY use this information to delete any other IKE SAs it has to
		// the same authenticated identity without waiting for a timeout." ze is
		// one-SA-per-configured-peer, so an authenticated re-initiation always supersedes.
		ps.signalSupersede(log)
	}
}

// sendAuthFailed sends an SK-encrypted IKE_AUTH response carrying
// AUTHENTICATION_FAILED. RFC 7296 Section 2.21.
func (ps *PeerSession) sendAuthFailed(sa *SA, msgID uint32, tr *transport.UDPTransport, remote *net.UDPAddr, log *slog.Logger) {
	notify := &wire.PayloadNotify{NotifyMsgType: wire.NotifyAuthenticationFailed}
	resp, err := buildEncryptedMessageEx(sa, []wire.PayloadEntry{{Payload: notify}}, msgID, wire.ExchangeIKEAuth, wire.FlagResponse)
	if err != nil {
		log.Debug("ike: build AUTHENTICATION_FAILED failed", "peer", sa.PeerName, "error", err)
		return
	}
	if tr != nil && remote != nil {
		if err := sendWithNATT(sa, resp, tr, remote); err != nil {
			log.Debug("ike: send AUTHENTICATION_FAILED failed", "peer", sa.PeerName, "error", err)
		}
	}
}
