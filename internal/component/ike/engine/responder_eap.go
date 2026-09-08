// Design: docs/architecture/ike/ipsec-14-responder.md -- IKE responder EAP authenticator
// Related: ts_narrow.go -- the narrowing this path shares with buildAuthResponse
// RFC: rfc/short/rfc7296.md -- EAP in IKE_AUTH (Section 2.16)

package engine

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"log/slog"
	"net"

	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/component/pki"
	"github.com/ze-software/ze/internal/core/eap"
)

// pemBlockCertificate is the PEM block type of an X.509 certificate
// (RFC 7468 Section 5). It is what a peer's TLS stack looks for when it parses
// the chain ze hands it, so the spelling is fixed by that RFC.
const pemBlockCertificate = "CERTIFICATE"

// eapMethodConfig builds the EAP method configuration (server side) from the peer's
// auth config: the MSCHAPv2 shared password, or the EAP-TLS server certificate chain.
func eapMethodConfig(sa *SA) (eap.MethodConfig, error) {
	// A password method reads the one configured secret. MD5-Challenge hashes it
	// with the challenge it issues (RFC 1994 Section 4.1), and MS-CHAPv2 hashes it
	// into the NT password hash. An empty value therefore authenticates every peer,
	// and is refused here rather than at the compare. ipsec.IsEAPPasswordMode
	// (ipsec/validate.go) is the one declaration of which modes carry one.
	if ipsec.IsEAPPasswordMode(sa.PeerCfg.Auth.Mode) {
		if sa.PeerCfg.Auth.PSK == "" {
			return eap.MethodConfig{}, fmt.Errorf("ike: %s requires a password", sa.PeerCfg.Auth.Mode)
		}
		return eap.MethodConfig{Password: sa.PeerCfg.Auth.PSK}, nil
	}
	if sa.PeerCfg.Auth.Mode == ipsec.AuthEAPTLS {
		return eapTLSServerConfig(sa)
	}
	return eap.MethodConfig{}, fmt.Errorf("ike: auth mode %s is not EAP", sa.PeerCfg.Auth.Mode)
}

// eapTLSServerConfig loads the EAP-TLS server certificate, key, and CA from the PKI
// store as PEM for the EAP-TLS authenticator.
func eapTLSServerConfig(sa *SA) (eap.MethodConfig, error) {
	certName := sa.PeerCfg.Auth.Certificate
	if certName == "" {
		return eap.MethodConfig{}, errNoCertificate
	}
	entry := pki.GetCertificate(certName)
	if entry == nil || entry.PrivateKey == nil {
		return eap.MethodConfig{}, fmt.Errorf("ike: EAP-TLS server certificate %q not found or has no private key", certName)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(entry.PrivateKey)
	if err != nil {
		return eap.MethodConfig{}, fmt.Errorf("ike: marshal EAP-TLS server key: %w", err)
	}
	cfg := eap.MethodConfig{
		ServerCertPEM: pem.EncodeToMemory(&pem.Block{Type: pemBlockCertificate, Bytes: entry.Raw}),
		ServerKeyPEM:  pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}),
	}
	// RFC 5216 Section 5.3: "Both sides MUST perform certificate path validation."
	// The authenticator validates the client chain against this trust anchor and has
	// no other means to do so. Refuse rather than proceed without one: newTLSMethod
	// sets ClientAuth to RequireAndVerifyClientCert over whatever pool it is given,
	// and an empty pool rejects every client with an opaque "certificate signed by
	// unknown authority" that names neither the peer nor the CA that failed to load.
	// Denying while saying nothing is the failure this guards (ai/rules/evidence.md).
	caName := sa.PeerCfg.Auth.CACertificate
	if caName == "" {
		return eap.MethodConfig{}, fmt.Errorf(
			"ike: EAP-TLS peer %q requires a ca-certificate to validate the client chain (RFC 5216 Section 5.3)",
			sa.PeerName)
	}
	ca := pki.GetCA(caName)
	if ca == nil {
		return eap.MethodConfig{}, fmt.Errorf(
			"ike: EAP-TLS ca-certificate %q not found in PKI store (peer %q)", caName, sa.PeerName)
	}
	cfg.CACertPEM = pem.EncodeToMemory(&pem.Block{Type: pemBlockCertificate, Bytes: ca.Raw})

	// RFC 9190 Section 5.4: "When EAP-TLS is used with TLS 1.3, the revocation
	// status of all the certificates in the certificate chains MUST be checked
	// (except the trust anchor)." The lists this CA published are the
	// authenticator's means of doing so, and a CA that holds none leaves the
	// answer nil, which refuses a TLS 1.3 client rather than admitting one whose
	// status nobody read (eap.checkChainRevocation).
	cfg.CRLPEM = ca.CRLPEM()

	// RFC 9190 Section 2.1.2: "the EAP-TLS server MUST send one or more
	// post-handshake NewSessionTicket messages ... in the initial
	// authentication." The ticket is encrypted under a key that belongs to the
	// PEERING and not to this exchange, so the store the peer session holds is
	// what makes the ticket redeemable on the next authentication
	// (resumptionFor, resumption.go).
	//
	// An SA that reached here with no store is a wiring defect, never an
	// operator error, and it is refused rather than papered over with a
	// per-exchange key: that key would issue a conformant-looking ticket nothing
	// could ever redeem (ai/rules/principles.md).
	if sa.Resumption == nil {
		return eap.MethodConfig{}, fmt.Errorf(
			"ike: EAP-TLS peer %q has no session resumption state, so the RFC 9190 Section 2.1.2 session ticket would be unredeemable",
			sa.PeerName)
	}
	cfg.Resumption = sa.Resumption
	return cfg, nil
}

// computeServerAuth computes the responder's own AUTH for the EAP first message
// from a long-term public-key credential. It does not use the EAP MSK, which is
// not yet derived. The sole caller is startResponderEAP, so every call here is
// part of an EAP exchange.
//
// RFC 7296 Section 2.16 says EAP methods "MUST be used in conjunction with a
// public-key-signature-based authentication of the responder to the initiator".
// A pre-shared key is not a public-key signature. There is therefore no PSK
// fallback here, and the guard denies instead (ai/rules/evidence.md).
//
// The removed fall-through to computePSKAuth signed the responder AUTH with the
// same secret that eap-mschapv2 hands the user as a password. It also left the
// initiator no signature to verify. ValidatePKIRefs (ipsec/validate.go) rejects
// such a peer at config time, and this check is the runtime backstop.
func computeServerAuth(sa *SA) (*wire.PayloadAUTH, error) {
	if sa.PeerCfg.Auth.Certificate == "" {
		return nil, fmt.Errorf(
			"ike: EAP peer %q (auth mode %s) has no certificate, so the responder cannot sign its AUTH. "+
				"Set authentication certificate to a PKI store name (RFC 7296 Section 2.16)",
			sa.PeerName, sa.PeerCfg.Auth.Mode)
	}
	return computeX509Auth(sa)
}

// eapToWire converts an eap.Packet to a wire EAP payload. eap.Packet.Encode()
// produces [code][id][len][type][data] (or just [code][id][0][4] for
// Success/Failure), and wire.PayloadEAP carries everything after the 4-byte header.
func eapToWire(p *eap.Packet) *wire.PayloadEAP {
	enc := p.Encode()
	return &wire.PayloadEAP{
		Code:       enc[0],
		Identifier: enc[1],
		EAPData:    append([]byte(nil), enc[4:]...),
	}
}

// startResponderEAP begins the EAP authenticator exchange after receiving the
// initiator's first IKE_AUTH (IDi, no AUTH). RFC 7296 Section 2.16: the responder
// authenticates itself (long-term credential), then sends the first EAP-Request.
// SAi2/TS are stashed on the SA for the final IKE_AUTH that carries SAr2/TSi/TSr.
func (ps *PeerSession) startResponderEAP(sa *SA, msgID uint32, remoteSAi2 *wire.PayloadSA, tsi, tsr *wire.PayloadTS, tr *transport.UDPTransport, remote *net.UDPAddr, log *slog.Logger) {
	if remoteSAi2 != nil {
		if outSPI, err := espSPIFromSA(remoteSAi2); err == nil {
			sa.ChildOutboundSPI = outSPI
		}
	}
	// RFC 7296 Section 2.9: the EAP path is the SECOND responder producer of traffic
	// selectors, and it stashes them here for the final IKE_AUTH that carries SAr2. It
	// narrows through the same entry point as buildAuthResponse, so an EAP peer gets the
	// same policy a PSK or X.509 peer gets. Skipping it here would leave EAP answering
	// with the old wildcard while the direct path narrowed.
	if tsi != nil && tsr != nil {
		if err := narrowChildSelectors(sa, tsi, tsr, nil); err != nil {
			log.Warn("ike: no acceptable traffic selector from initiator", "peer", sa.PeerName, "error", err)
			sa.State = StateDead
			return
		}
	}

	// Negotiate the ESP proposal now (narrows sa.ESPGroup); the final IKE_AUTH after
	// EAP success reuses the narrowed group to build SAr2 and install the Child SA.
	if err := selectResponderESP(sa, remoteSAi2); err != nil {
		log.Warn("ike: no acceptable ESP proposal from initiator", "peer", sa.PeerName, "error", err)
		sa.State = StateDead
		return
	}

	config, err := eapMethodConfig(sa)
	if err != nil {
		log.Warn("ike: EAP method config failed", "peer", sa.PeerName, "error", err)
		sa.State = StateDead
		return
	}
	sess, err := newEAPSession(sa.PeerCfg.Auth.Mode, config)
	if err != nil {
		log.Warn("ike: create EAP session failed", "peer", sa.PeerName, "error", err)
		sa.State = StateDead
		return
	}
	sa.EAPSession = sess

	// RFC 7296 Section 2.16: the responder authenticates with its own long-term
	// credential (certificate/PSK) in this first message, not from the not-yet-known
	// EAP MSK.
	serverAuth, err := computeServerAuth(sa)
	if err != nil {
		log.Warn("ike: responder EAP server AUTH failed", "peer", sa.PeerName, "error", err)
		sa.State = StateDead
		return
	}

	first := sess.Begin()

	inner := make([]wire.PayloadEntry, 0, 4)
	inner = append(inner, wire.PayloadEntry{Payload: buildIDPayload(sa, false)})
	if sa.PeerCfg.Auth.Certificate != "" {
		certPayloads, cErr := buildCertPayloads(sa)
		if cErr != nil {
			log.Warn("ike: responder EAP certificate payloads failed",
				"peer", sa.PeerName, "error", cErr)
			sa.State = StateDead
			return
		}
		inner = append(inner, certPayloads...)
	}
	inner = append(inner,
		wire.PayloadEntry{Payload: serverAuth},
		wire.PayloadEntry{Payload: eapToWire(first)},
	)

	resp, err := buildEncryptedMessageEx(sa, inner, msgID, wire.ExchangeIKEAuth, wire.FlagResponse)
	if err != nil {
		log.Warn("ike: build EAP first response failed", "peer", sa.PeerName, "error", err)
		sa.State = StateDead
		return
	}
	cacheResponse(sa, msgID, resp)
	sa.LastSentMsg = resp
	// RFC 7296 Section 2.11: the reply goes back to the address and port the request
	// came from, on the socket it arrived on. The initiator has NOT authenticated
	// yet, so nothing is stored on the SA here.
	if err := sendReply(tr, resp, remote); err != nil {
		log.Warn("ike: send EAP first response failed", "peer", sa.PeerName, "error", err)
	}
	sa.State = StateEAPInProgress
	log.Debug("ike: responder EAP started", "peer", sa.PeerName)
}

// handleResponderEAP drives one EAP round (or the concluding AUTH-from-MSK) on an
// inbound IKE_AUTH while StateEAPInProgress. RFC 7296 Section 2.16.
func (ps *PeerSession) handleResponderEAP(sa *SA, msg *wire.Message, rawMsg []byte, tr *transport.UDPTransport, remote *net.UDPAddr, log *slog.Logger) {
	inner, err := decryptAndParse(sa, msg, rawMsg)
	if err != nil {
		log.Warn("ike: EAP round decrypt failed", "peer", sa.PeerName, "error", err)
		sa.State = StateDead
		return
	}

	var eapPayload *wire.PayloadEAP
	var authPayload *wire.PayloadAUTH
	for i := range inner {
		switch p := inner[i].Payload.(type) {
		case *wire.PayloadEAP:
			eapPayload = p
		case *wire.PayloadAUTH:
			authPayload = p
		}
	}

	// After EAP-Success the initiator sends its AUTH derived from the MSK, with no
	// EAP payload. Verify it and send our final IKE_AUTH (AUTH-from-MSK + SAr2).
	if authPayload != nil {
		if err := verifyRemoteAuth(sa, authPayload); err != nil {
			log.Warn("ike: EAP AUTH-from-MSK verification failed", "peer", sa.PeerName, "error", err)
			ps.sendAuthFailed(sa, msg.Header.MessageID, tr, remote, log)
			sa.State = StateDead
			return
		}
		resp, child, err := ps.buildAuthResponse(sa, msg.Header.MessageID, nil, nil, nil, true, log)
		if err != nil {
			log.Warn("ike: EAP final response build failed", "peer", sa.PeerName, "error", err)
			sa.State = StateDead
			return
		}
		ps.finishResponderEstablish(sa, msg.Header.MessageID, resp, child, tr, remote, log)
		return
	}

	if eapPayload == nil {
		log.Warn("ike: EAP round missing EAP payload", "peer", sa.PeerName)
		sa.State = StateDead
		return
	}

	sess, ok := sa.EAPSession.(*eap.Session)
	if !ok || sess == nil {
		log.Warn("ike: no EAP authenticator session", "peer", sa.PeerName)
		sa.State = StateDead
		return
	}

	// The cause the session held before this round, which an earlier round has
	// already written to the log. Nothing else calls Process for a responder
	// session, so reading it here is what keeps one refusal to one line.
	announced := sess.Err()

	next := sess.Process(wireEAPToPacket(eapPayload))
	if next == nil {
		if sess.Succeeded() {
			sa.EAPMSK = sess.MSK()
			recordEAPTLSAuthentication(sa, sess.Resumed(), log)
		}
		return
	}
	if next.Code == eap.CodeSuccess {
		sa.EAPMSK = sess.MSK()
		recordEAPTLSAuthentication(sa, sess.Resumed(), log)
	}
	ps.sendResponderEAP(sa, msg.Header.MessageID, next, tr, remote, log)

	// One account per refusal, written on the round the session RECORDS the cause.
	//
	// A method that owes a refused peer a last word records it while it sends that
	// EAP-Request (MethodResult.FinalRequest, internal/core/eap), one round before
	// the EAP-Failure. RFC 5216 Section 2.1.3 makes the server wait for the peer's
	// reply in between, and the peer decides whether that reply ever comes:
	// strongSwan abandons an EAP-TLS exchange after ze's fatal alert. Logging under
	// next.Code == CodeFailure alone therefore left every EAP-TLS certificate
	// refusal unreported, and the operator read the 30s handshake timeout and
	// nothing else
	// (plan/journal/diagnosis-parked-until-a-round-the-peer-may-never-send.md).
	//
	// An EAP-Failure packet carries no reason of its own (RFC 3748 Section 4.2,
	// requirement RFC3748-4.2-2: Code, Identifier and Length, no Type field), so
	// the session's cause is the only account there is. The initiator half logs its
	// equivalent in handleEAPResponse (fsm.go).
	//
	// The comparison is by VALUE and not by presence, and Session.nakUnexpected is
	// why: it records a cause on a round that discards the peer's packet and
	// returns nothing, so no line is written there. One forbidden Nak would
	// otherwise leave a cause standing on the session for the rest of the
	// exchange, and silence the report of the certificate refusal that follows it.
	switch cause := sess.Err(); {
	case cause != nil && cause != announced:
		log.Warn("ike: EAP authentication failed", "peer", sa.PeerName, "error", cause)
	case cause == nil && next.Code == eap.CodeFailure:
		log.Warn("ike: EAP authentication failed", "peer", sa.PeerName)
	}

	if next.Code == eap.CodeFailure {
		sa.State = StateDead
	}
}

// sendResponderEAP builds and sends an SK-encrypted IKE_AUTH response carrying a
// single EAP payload (an EAP-Request, EAP-Success, or EAP-Failure).
func (ps *PeerSession) sendResponderEAP(sa *SA, msgID uint32, pkt *eap.Packet, tr *transport.UDPTransport, remote *net.UDPAddr, log *slog.Logger) {
	inner := []wire.PayloadEntry{{Payload: eapToWire(pkt)}}
	resp, err := buildEncryptedMessageEx(sa, inner, msgID, wire.ExchangeIKEAuth, wire.FlagResponse)
	if err != nil {
		log.Warn("ike: build EAP response failed", "peer", sa.PeerName, "error", err)
		sa.State = StateDead
		return
	}
	cacheResponse(sa, msgID, resp)
	sa.LastSentMsg = resp
	// RFC 7296 Section 2.11: an EAP round is answered on its arrival socket, to its
	// observed source. The peer authenticates only at the end of the EAP exchange.
	if err := sendReply(tr, resp, remote); err != nil {
		log.Warn("ike: send EAP response failed", "peer", sa.PeerName, "error", err)
	}
}
