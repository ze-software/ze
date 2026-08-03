// Design: plan/learned/1072-ipsec-14-responder.md -- IKE responder EAP authenticator
// Related: ts_narrow.go -- the narrowing this path shares with buildAuthResponse
// RFC: rfc/short/rfc7296.md -- EAP in IKE_AUTH (Section 2.16)

package engine

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"log/slog"
	"net"

	"github.com/ze-software/ze/internal/component/ike/eap"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/component/pki"
)

// eapMethodConfig builds the EAP method configuration (server side) from the peer's
// auth config: the MSCHAPv2 shared password, or the EAP-TLS server certificate chain.
func eapMethodConfig(sa *SA) (eap.MethodConfig, error) {
	if sa.PeerCfg.Auth.Mode == ipsec.AuthEAPMSCHAPv2 {
		if sa.PeerCfg.Auth.PSK == "" {
			return eap.MethodConfig{}, fmt.Errorf("ike: EAP-MSCHAPv2 requires a password")
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
		ServerCertPEM: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: entry.Raw}),
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
	cfg.CACertPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: ca.Raw})
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
	sess, err := NewEAPSession(sa.PeerCfg.Auth.Mode, config)
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

	next := sess.Process(wireEAPToPacket(eapPayload))
	if next == nil {
		if sess.Succeeded() {
			sa.EAPMSK = sess.MSK()
		}
		return
	}
	if next.Code == eap.CodeSuccess {
		sa.EAPMSK = sess.MSK()
	}
	ps.sendResponderEAP(sa, msg.Header.MessageID, next, tr, remote, log)
	if next.Code == eap.CodeFailure {
		log.Warn("ike: EAP authentication failed", "peer", sa.PeerName)
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
