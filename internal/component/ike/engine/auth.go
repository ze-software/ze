// Design: plan/learned/740-ipsec-7-ikev2-engine.md -- AUTH payload computation
// Related: remote_id.go -- remote identity policy and certificate binding
// Detail: cert_payload.go -- CERT payload assembly, the received-chain bound, Hash and URL
// RFC: rfc/short/rfc7296.md -- Authentication of the IKE SA (Section 2.15)
// RFC: rfc/short/rfc7427.md -- Digital Signature AUTH method 14
package engine

import (
	"bytes"
	"crypto"
	"crypto/aes"
	gocipher "crypto/cipher"
	"crypto/ecdsa"
	"crypto/elliptic"
	crand "crypto/rand"
	"crypto/rsa"
	"crypto/sha1" //nolint:gosec // required by IKEv2 CERTREQ (RFC 7296 Section 3.7)
	"crypto/sha256"
	"crypto/sha512"
	"crypto/subtle"
	"encoding/asn1"
	"errors"
	"fmt"
	"hash"
	"net"
	"slices"
	"time"

	ikecrypto "github.com/ze-software/ze/internal/component/ike/crypto"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/component/pki"
)

var (
	errNoPSK           = errors.New("ike auth: no pre-shared key configured")
	errNoCertificate   = errors.New("ike auth: no certificate configured")
	errUnsupportedKey  = errors.New("ike auth: unsupported key type")
	errSignatureFailed = errors.New("ike auth: signature verification failed")
)

// computeSignedOctets builds the signed octets for AUTH payload computation.
// InitiatorSignedOctets = RealMessage1 | NonceRData | prf(SK_pi, IDi').
// ResponderSignedOctets = RealMessage2 | NonceIData | prf(SK_pr, IDr').
// computeSignedOctets builds the octets for the party identified by isInitiator
// (true = the IKE-SA initiator's AUTH octets, false = the responder's), using the
// ABSOLUTE nonce order and the correct ID: RFC 7296 Section 2.15 appends Nr (the
// responder's nonce) to the initiator's octets and Ni (the initiator's) to the
// responder's, each with that party's own ID payload. The nonce/ID selection is
// therefore relative to the role, not this side's Local/Remote — for an initiator
// SA these reduce to Remote/Local exactly as before, so the initiator is unchanged.
func computeSignedOctets(sa *SA, isInitiator bool) ([]byte, error) {
	var realMsg, peerNonce, skP []byte
	if isInitiator {
		realMsg = sa.InitiatorSAInitMsg
		peerNonce = sa.responderNonce() // Nr
		skP = sa.SKKeys.SK_pi
	} else {
		realMsg = sa.ResponderSAInitMsg
		peerNonce = sa.initiatorNonce() // Ni
		skP = sa.SKKeys.SK_pr
	}

	// The ID belongs to the party whose octets these are: our own ID when that
	// party is us, otherwise the peer's received ID payload.
	var idPayload *wire.PayloadID
	if isInitiator != sa.IsInitiator && sa.RemoteIDPayload != nil {
		idPayload = sa.RemoteIDPayload
	} else {
		idPayload = buildIDPayload(sa, isInitiator)
	}
	idBytes := make([]byte, 4+len(idPayload.IDData))
	n := idPayload.WriteTo(idBytes, 0)
	idBytes = idBytes[:n]

	prfID := sa.Proposal.PRF.ID
	prfOfID, err := ikecrypto.PRF(prfID, skP, idBytes)
	if err != nil {
		return nil, err
	}

	signed := make([]byte, 0, len(realMsg)+len(peerNonce)+len(prfOfID))
	signed = append(signed, realMsg...)
	signed = append(signed, peerNonce...)
	signed = append(signed, prfOfID...)
	return signed, nil
}

// mayBeReplicated reports whether the local end of this peer is one identity that two
// systems CAN authenticate as at the same time.
//
// RFC 7296 Section 2.4 bars such an entity from sending INITIAL_CONTACT. It names the
// case: "a roaming user's credentials where the user is allowed to connect to the
// corporate firewall from two remote systems at the same time". The notification
// asserts that this IKE SA is the ONLY one active between the two authenticated
// identities. A responder acts on it and deletes the others. A replicable identity
// that asserted it would stop the session its own second device holds.
//
// EAP is that case in ze's config model. An EAP credential names a USER. The
// remote-access surface keys its eap-user list by user name, and one such user CAN
// hold a session from a laptop and a phone at once. A pre-shared secret or an X.509
// device certificate names one configured DEVICE. Ze runs one SA per configured
// peer, so the assertion stays truthful there.
//
// The predicate reads the auth mode, not the connection type. A replicable credential
// stays replicable whether this node dialed out or waited. The responder path sends
// no INITIAL_CONTACT at all.
func mayBeReplicated(peer ipsec.SiteToSitePeer) bool {
	return ipsec.IsEAPMode(peer.Auth.Mode)
}

// buildAuthRequest constructs an encrypted IKE_AUTH request message.
// RFC 7296 Section 2.16: when auth mode is EAP, the initiator omits AUTH
// in the first IKE_AUTH to signal willingness to use EAP.
func buildAuthRequest(sa *SA) ([]byte, error) {
	// One predicate answers "is this EAP" for the config gate and for the engine.
	// A private copy here goes stale the day a third mode arrives. This side then
	// sends an AUTH payload where the EAP signal belongs.
	// See ai/rules/evidence.md.
	isEAP := ipsec.IsEAPMode(sa.PeerCfg.Auth.Mode)

	idPayload := buildIDPayload(sa, true)

	innerPayloads := make([]wire.PayloadEntry, 0, 6)

	if isEAP {
		innerPayloads = append(innerPayloads, wire.PayloadEntry{Payload: idPayload})
		if certReq := buildCertRequest(sa); certReq != nil {
			innerPayloads = append(innerPayloads, wire.PayloadEntry{Payload: certReq})
		}
	} else {
		authPayload, err := computeLocalAuth(sa)
		if err != nil {
			return nil, err
		}
		innerPayloads = []wire.PayloadEntry{
			{Payload: idPayload},
			{Payload: authPayload},
		}
		if sa.PeerCfg.Auth.Mode == ipsec.AuthX509 {
			certPayloads, err := buildCertPayloads(sa)
			if err != nil {
				return nil, err
			}
			innerPayloads = append(certPayloads, innerPayloads...)
		}
	}

	// RFC 7296 Section 2.4: INITIAL_CONTACT "MUST be in the first IKE_AUTH request or
	// response" and "asserts that this IKE SA is the only IKE SA currently active
	// between the authenticated identities", letting the responder delete any stale SA
	// to us without waiting for a timeout. Rekey never reaches here (it uses
	// CREATE_CHILD_SA on the existing SA).
	//
	// The same section forbids the assertion outright for a replicable identity: the
	// notification "MUST NOT be sent by an entity that may be replicated". mayBeReplicated
	// decides that, so a device identity still asserts it and a user identity never does.
	if !mayBeReplicated(sa.PeerCfg) {
		innerPayloads = append(innerPayloads,
			wire.PayloadEntry{Payload: &wire.PayloadNotify{NotifyMsgType: wire.NotifyInitialContact}},
		)
	}

	// RFC 7296 Section 3.10 allows HTTP_CERT_LOOKUP_SUPPORTED in any message that can
	// carry a CERTREQ, and Section 3.7 puts one in the IKE_AUTH request.
	if notify := hashAndURLNotify(sa); notify != nil {
		innerPayloads = append(innerPayloads, wire.PayloadEntry{Payload: notify})
	}

	espSPI, saPayload, tsi, tsr, err := buildChildSAPayloads(sa)
	if err != nil {
		return nil, fmt.Errorf("ike auth: child SA payloads: %w", err)
	}
	sa.ChildInboundSPI = espSPI
	innerPayloads = append(innerPayloads, wire.PayloadEntry{Payload: saPayload})
	// RFC 7296 Section 1.3.1: the USE_TRANSPORT_MODE notification goes in a request that
	// also carries the SA payload requesting a Child SA. Ze sends it only when the
	// operator configured transport mode for this peer, so a tunnel-mode peer's request
	// is byte-identical to what it was before transport mode existed.
	if wantsTransportMode(sa) {
		innerPayloads = append(innerPayloads, wire.PayloadEntry{Payload: transportModeNotify()})
	}
	innerPayloads = append(innerPayloads,
		wire.PayloadEntry{Payload: tsi},
		wire.PayloadEntry{Payload: tsr},
	)

	return buildEncryptedMessage(sa, innerPayloads, sa.NextMsgID)
}

// buildEAPResponse constructs an encrypted IKE_AUTH message containing a single EAP payload.
func buildEAPResponse(sa *SA, eapData []byte) ([]byte, error) {
	if len(eapData) < 5 {
		return nil, fmt.Errorf("ike: EAP response data too short (%d bytes)", len(eapData))
	}
	innerPayloads := []wire.PayloadEntry{
		{Payload: &wire.PayloadEAP{
			Code:       wire.EAPCodeResponse,
			Identifier: eapData[1],
			EAPData:    eapData[4:],
		}},
	}
	return buildEncryptedMessage(sa, innerPayloads, sa.NextMsgID)
}

// buildEAPAuthMessage constructs an IKE_AUTH message with only the AUTH payload,
// sent after EAP-Success (RFC 7296 Section 2.16).
func buildEAPAuthMessage(sa *SA) ([]byte, error) {
	authPayload, err := computeEAPAuth(sa)
	if err != nil {
		return nil, err
	}
	innerPayloads := []wire.PayloadEntry{
		{Payload: authPayload},
	}
	return buildEncryptedMessage(sa, innerPayloads, sa.NextMsgID)
}

// buildEncryptedMessage serializes inner payloads into an encrypted IKE_AUTH SK
// message (initiator request). Retained for existing IKE_AUTH callers.
func buildEncryptedMessage(sa *SA, innerPayloads []wire.PayloadEntry, messageID uint32) ([]byte, error) {
	return buildEncryptedMessageEx(sa, innerPayloads, messageID, wire.ExchangeIKEAuth, wire.FlagInitiator)
}

// buildEncryptedMessageEx serializes inner payloads into an encrypted SK message
// for an arbitrary exchange type and header flags. CREATE_CHILD_SA rekey requests
// and responses reuse the same SK wrapping as IKE_AUTH, differing only in the
// header ExchangeType and the Response flag.
func buildEncryptedMessageEx(sa *SA, innerPayloads []wire.PayloadEntry, messageID uint32, exchangeType, flags uint8) ([]byte, error) {
	// RFC 7296 Section 2.8: "When the lifetime of a Security Association expires, the
	// Security Association MUST NOT be used." Every message this node protects with the
	// IKE SA's keys is built here, so this is the one place that can refuse to use an
	// SA whose lifetime has run out. The owner loop also tears an expired SA down, but
	// it looks once a second and a send can be reached between two of its ticks.
	if sa.lifetimeExpired(time.Now()) {
		return nil, errSAExpired
	}

	const maxInnerSize = 65536
	innerBuf := make([]byte, maxInnerSize)
	off := 0
	for i := range innerPayloads {
		if off+wire.GenericHeaderLen > maxInnerSize {
			return nil, fmt.Errorf("ike: inner payloads exceed %d bytes", maxInnerSize)
		}
		var gh wire.GenericHeader
		gh.Critical = innerPayloads[i].Critical
		if i+1 < len(innerPayloads) {
			gh.NextPayload = innerPayloads[i+1].Payload.Type()
		}
		ghOff := off
		off += wire.GenericHeaderLen
		bodyLen := innerPayloads[i].Payload.WriteTo(innerBuf, off)
		off += bodyLen
		if off > maxInnerSize {
			return nil, fmt.Errorf("ike: inner payloads exceed %d bytes", maxInnerSize)
		}
		gh.Length = uint16(wire.GenericHeaderLen + bodyLen)
		gh.WriteTo(innerBuf, ghOff)
	}
	innerData := innerBuf[:off]

	var innerFirstType uint8
	if len(innerPayloads) > 0 {
		innerFirstType = innerPayloads[0].Payload.Type()
	}

	if sa.Proposal.Encryption.IsAEAD {
		return buildSKMessageAEADWithMsgID(sa, innerData, innerFirstType, messageID, exchangeType, flags)
	}
	return buildSKMessageCBCWithMsgID(sa, innerData, innerFirstType, messageID, exchangeType, flags)
}

// computeLocalAuth computes the local AUTH payload.
func computeLocalAuth(sa *SA) (*wire.PayloadAUTH, error) {
	switch sa.PeerCfg.Auth.Mode {
	case ipsec.AuthPreSharedSecret:
		return computePSKAuth(sa)
	case ipsec.AuthX509:
		return computeX509Auth(sa)
	case ipsec.AuthEAPTLS, ipsec.AuthEAPMSCHAPv2:
		return computeEAPAuth(sa)
	case ipsec.AuthUnknown:
		return nil, fmt.Errorf("ike auth: unsupported auth mode %s", sa.PeerCfg.Auth.Mode)
	}
	return nil, fmt.Errorf("ike auth: unsupported auth mode %s", sa.PeerCfg.Auth.Mode)
}

// computePSKAuth computes AUTH using pre-shared key per RFC 7296 Section 2.15.
func computePSKAuth(sa *SA) (*wire.PayloadAUTH, error) {
	psk := sa.PeerCfg.Auth.PSK
	if psk == "" {
		return nil, errNoPSK
	}

	signedOctets, err := computeSignedOctets(sa, sa.IsInitiator)
	if err != nil {
		return nil, err
	}

	prfID := sa.Proposal.PRF.ID
	keyPad := []byte("Key Pad for IKEv2")
	derivedKey, err := ikecrypto.PRF(prfID, []byte(psk), keyPad)
	if err != nil {
		return nil, err
	}

	authData, err := ikecrypto.PRF(prfID, derivedKey, signedOctets)
	if err != nil {
		return nil, err
	}

	return &wire.PayloadAUTH{
		AuthMethod: wire.AuthMethodPSK,
		AuthData:   authData,
	}, nil
}

// computeX509Auth computes AUTH using X.509 digital signature (RFC 7427 method 14).
func computeX509Auth(sa *SA) (*wire.PayloadAUTH, error) {
	certName := sa.PeerCfg.Auth.Certificate
	if certName == "" {
		return nil, errNoCertificate
	}

	entry := pki.GetCertificate(certName)
	if entry == nil {
		return nil, fmt.Errorf("ike auth: certificate %q not found in PKI store", certName)
	}

	signedOctets, err := computeSignedOctets(sa, sa.IsInitiator)
	if err != nil {
		return nil, err
	}

	algID, hashFunc, err := selectSignatureAlgorithm(entry.PrivateKey, sa.RemoteHashAlgos)
	if err != nil {
		return nil, err
	}

	h := hashFunc.New()
	h.Write(signedOctets)
	digest := h.Sum(nil)

	sig, err := signDigest(entry.PrivateKey, digest, hashFunc)
	if err != nil {
		return nil, err
	}

	// RFC 7427 Section 3: authData = ASN.1 Length | AlgorithmIdentifier | Signature.
	authData := make([]byte, 0, 1+len(algID)+len(sig))
	authData = append(authData, byte(len(algID)))
	authData = append(authData, algID...)
	authData = append(authData, sig...)

	return &wire.PayloadAUTH{
		AuthMethod: wire.AuthMethodDigitalSig,
		AuthData:   authData,
	}, nil
}

// verifyRemoteAuth verifies the remote peer's AUTH payload.
// RFC 7296 Section 2.16: after EAP, the responder's AUTH also uses MSK-derived key.
func verifyRemoteAuth(sa *SA, authPayload *wire.PayloadAUTH) error {
	// The policy half of remote-id, before any credential is read. The peer picks the
	// identity its AUTH covers, so a valid signature proves who holds the key and never
	// which peer this is. Both halves are needed, and getRemoteCert holds the other.
	// The wire answer is AUTHENTICATION_FAILED either way, so a refusal here tells an
	// unauthenticated caller nothing it did not already send.
	if err := checkRemoteIdentity(sa); err != nil {
		return err
	}

	signedOctets, err := computeSignedOctets(sa, !sa.IsInitiator)
	if err != nil {
		return err
	}

	// The canonical predicate, not a copy of it. A copy that missed a later mode
	// falls through to verifyPSKAuth. The shared-key AUTH that RFC 7296
	// Section 2.16 forbids on an EAP peer then verifies again.
	// See ai/rules/evidence.md and ai/rules/evidence.md.
	isEAP := ipsec.IsEAPMode(sa.PeerCfg.Auth.Mode)

	switch authPayload.AuthMethod {
	case wire.AuthMethodPSK:
		if isEAP && sa.EAPMSK != [64]byte{} {
			return VerifyAuthFromMSK(sa.Proposal.PRF.ID, sa.EAPMSK, signedOctets, authPayload.AuthData)
		}
		// The receive-side mirror of computeServerAuth (responder_eap.go).
		// RFC 7296 Section 2.16 says EAP methods "MUST be used in conjunction
		// with a public-key-signature-based authentication of the responder to
		// the initiator". Reaching here on an EAP SA means the remote sent a
		// shared-secret AUTH before any MSK existed. That is the responder AUTH
		// of the first EAP message, and a pre-shared key does not satisfy the
		// obligation. Refuse it (ai/rules/evidence.md).
		if isEAP {
			return fmt.Errorf(
				"ike auth: EAP peer %q sent a pre-shared-key AUTH, and RFC 7296 Section 2.16 "+
					"requires a public-key signature from the responder", sa.PeerName)
		}
		return verifyPSKAuth(sa, authPayload.AuthData, signedOctets)
	case wire.AuthMethodDigitalSig:
		return verifyX509Auth(sa, authPayload.AuthData, signedOctets)
	case wire.AuthMethodRSASig:
		return verifyLegacyRSAAuth(sa, authPayload.AuthData, signedOctets)
	}
	return fmt.Errorf("ike auth: unsupported remote auth method %d", authPayload.AuthMethod)
}

// verifyPSKAuth verifies a PSK AUTH payload.
func verifyPSKAuth(sa *SA, authData, signedOctets []byte) error {
	psk := sa.PeerCfg.Auth.PSK
	if psk == "" {
		return errNoPSK
	}

	prfID := sa.Proposal.PRF.ID
	keyPad := []byte("Key Pad for IKEv2")
	derivedKey, err := ikecrypto.PRF(prfID, []byte(psk), keyPad)
	if err != nil {
		return err
	}

	expected, err := ikecrypto.PRF(prfID, derivedKey, signedOctets)
	if err != nil {
		return err
	}

	if subtle.ConstantTimeCompare(authData, expected) != 1 {
		return errAuthFailed
	}
	return nil
}

// verifyX509Auth verifies a Digital Signature (method 14) AUTH payload.
func verifyX509Auth(sa *SA, authData, signedOctets []byte) error {
	if len(authData) < 2 {
		return errInvalidMessage
	}
	algIDLen := int(authData[0])
	if len(authData) < 1+algIDLen {
		return errInvalidMessage
	}
	algID := authData[1 : 1+algIDLen]
	signature := authData[1+algIDLen:]

	hashFunc := hashFromAlgID(algID)
	if hashFunc == nil {
		return fmt.Errorf("ike auth: unsupported algorithm identifier")
	}

	cert, err := getRemoteCert(sa)
	if err != nil {
		return err
	}

	h := hashFunc()
	h.Write(signedOctets)
	digest := h.Sum(nil)

	return verifySignature(cert.PublicKey, digest, signature, hashFunc)
}

// verifyLegacyRSAAuth verifies a legacy RSA signature (method 1) AUTH payload.
func verifyLegacyRSAAuth(sa *SA, authData, signedOctets []byte) error {
	cert, err := getRemoteCert(sa)
	if err != nil {
		return err
	}

	h := sha256.New()
	h.Write(signedOctets)
	digest := h.Sum(nil)

	rsaPub, ok := cert.PublicKey.(*rsa.PublicKey)
	if !ok {
		return errUnsupportedKey
	}
	return rsa.VerifyPKCS1v15(rsaPub, crypto.SHA256, digest, authData)
}

// buildCertRequest builds a CERTREQ payload with the SHA-1 hash of the configured CA's public key.
// RFC 7296 Section 3.7: CertAuthority is SHA-1(SubjectPublicKeyInfo DER).
func buildCertRequest(sa *SA) *wire.PayloadCERTREQ {
	caName := sa.PeerCfg.Auth.CACertificate
	if caName == "" {
		return nil
	}
	ca := pki.GetCA(caName)
	if ca == nil {
		return nil
	}
	h := sha1.Sum(ca.Certificate.RawSubjectPublicKeyInfo) //nolint:gosec // required by IKEv2 CERTREQ
	return &wire.PayloadCERTREQ{
		CertEncoding:  wire.CertEncodingX509Sig,
		CertAuthority: h[:],
	}
}

func buildIDPayload(sa *SA, isInitiator bool) *wire.PayloadID {
	ptype := wire.PayloadTypeIDi
	if !isInitiator {
		ptype = wire.PayloadTypeIDr
	}
	idStr := sa.PeerName
	if sa.PeerCfg.Auth.LocalID != "" {
		idStr = sa.PeerCfg.Auth.LocalID
	}
	idType, idData := encodeIKEID(idStr)
	return &wire.PayloadID{
		IDPayloadType: ptype,
		IDType:        idType,
		IDData:        idData,
	}
}

// encodeIKEID selects the IKE ID type for a configured identity string: an IPv4 or
// IPv6 literal becomes ID_IPV4_ADDR / ID_IPV6_ADDR carrying the packed address
// bytes, anything else ID_FQDN. RFC 7296 Section 3.5. Peers commonly constrain the
// remote identity by type, so an IP-literal id MUST be sent as the address type
// (strongSwan rejects an IP value sent as ID_FQDN with "constraint check failed").
func encodeIKEID(id string) (uint8, []byte) {
	if ip := net.ParseIP(id); ip != nil {
		if v4 := ip.To4(); v4 != nil {
			return wire.IDTypeIPv4Addr, v4
		}
		return wire.IDTypeIPv6Addr, ip.To16()
	}
	return wire.IDTypeFQDN, []byte(id)
}

// skSendEncKey/skSendIntegKey/skRecvEncKey/skRecvIntegKey select the SK_* key for
// this SA's role. RFC 7296 Section 2.14: the IKE-SA initiator protects the
// messages it SENDS with SK_ei/SK_ai and verifies+decrypts what it RECEIVES with
// SK_er/SK_ar; the responder is the mirror. Selecting by sa.IsInitiator lets one
// SK encrypt/decrypt path serve both roles. For an initiator SA these return the
// SK_e{i}/SK_a{i} (send) and SK_e{r}/SK_a{r} (recv) keys, identical to the former
// hardcoded behavior.
func skSendEncKey(sa *SA) []byte {
	if sa.IsInitiator {
		return sa.SKKeys.SK_ei
	}
	return sa.SKKeys.SK_er
}

func skSendIntegKey(sa *SA) []byte {
	if sa.IsInitiator {
		return sa.SKKeys.SK_ai
	}
	return sa.SKKeys.SK_ar
}

func skRecvEncKey(sa *SA) []byte {
	if sa.IsInitiator {
		return sa.SKKeys.SK_er
	}
	return sa.SKKeys.SK_ei
}

func skRecvIntegKey(sa *SA) []byte {
	if sa.IsInitiator {
		return sa.SKKeys.SK_ar
	}
	return sa.SKKeys.SK_ai
}

// buildSKMessageCBCWithMsgID builds a complete IKE_AUTH message with AES-CBC + HMAC.
// Wire: [Header 28][SK GH 4][IV blockSize][Encrypted(content+pad+padlen)][ICV truncLen].
func buildSKMessageCBCWithMsgID(sa *SA, innerData []byte, firstType uint8, messageID uint32, exchangeType, flags uint8) ([]byte, error) {
	const blockSize = 16 // AES block size

	contentLen := len(innerData)
	padLen := blockSize - ((contentLen + 1) % blockSize)
	if padLen == blockSize {
		padLen = 0
	}
	paddedLen := contentLen + padLen + 1

	integTrunc := int(sa.Proposal.Integrity.TruncatedLength)
	if integTrunc == 0 {
		integTrunc = 16
	}

	totalLen := wire.HeaderLen + wire.GenericHeaderLen + blockSize + paddedLen + integTrunc
	buf := make([]byte, totalLen)

	writeAuthHeaderWithMsgID(buf, sa, firstType, uint32(totalLen), messageID, exchangeType, flags)

	ivOff := wire.HeaderLen + wire.GenericHeaderLen
	if _, err := crand.Read(buf[ivOff : ivOff+blockSize]); err != nil {
		return nil, err
	}

	dataOff := ivOff + blockSize
	copy(buf[dataOff:], innerData)
	buf[dataOff+contentLen+padLen] = byte(padLen)

	block, err := aes.NewCipher(skSendEncKey(sa))
	if err != nil {
		return nil, err
	}
	cbc := gocipher.NewCBCEncrypter(block, buf[ivOff:ivOff+blockSize])
	cbc.CryptBlocks(buf[dataOff:dataOff+paddedLen], buf[dataOff:dataOff+paddedLen])

	mac, err := ikecrypto.ComputeIntegrity(sa.Proposal.Integrity.ID, skSendIntegKey(sa), buf[:totalLen-integTrunc])
	if err != nil {
		return nil, err
	}
	copy(buf[totalLen-integTrunc:], mac)
	return buf, nil
}

// buildSKMessageAEADWithMsgID builds a complete IKE_AUTH message with AES-GCM.
// Wire: [Header 28][SK GH 4][IV 8][ciphertext][GCM tag 16].
func buildSKMessageAEADWithMsgID(sa *SA, innerData []byte, firstType uint8, messageID uint32, exchangeType, flags uint8) ([]byte, error) {
	const ivLen = 8
	const tagLen = 16

	plaintext := make([]byte, len(innerData)+1)
	copy(plaintext, innerData)
	plaintext[len(innerData)] = 0

	totalLen := wire.HeaderLen + wire.GenericHeaderLen + ivLen + len(plaintext) + tagLen
	buf := make([]byte, totalLen)

	writeAuthHeaderWithMsgID(buf, sa, firstType, uint32(totalLen), messageID, exchangeType, flags)

	ivOff := wire.HeaderLen + wire.GenericHeaderLen
	if _, err := crand.Read(buf[ivOff : ivOff+ivLen]); err != nil {
		return nil, err
	}

	aad := buf[:wire.HeaderLen+wire.GenericHeaderLen]
	sendKey := skSendEncKey(sa)
	key := sendKey[:len(sendKey)-4]
	salt := sendKey[len(sendKey)-4:]
	nonce := make([]byte, 12)
	copy(nonce, salt)
	copy(nonce[4:], buf[ivOff:ivOff+ivLen])

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := gocipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	sealed := gcm.Seal(nil, nonce, plaintext, aad)
	copy(buf[ivOff+ivLen:], sealed)
	return buf, nil
}

func writeAuthHeaderWithMsgID(buf []byte, sa *SA, firstType uint8, totalLen, messageID uint32, exchangeType, flags uint8) {
	hdr := wire.Header{
		InitiatorSPI: sa.InitiatorSPI,
		ResponderSPI: sa.ResponderSPI,
		MajorVersion: 2,
		ExchangeType: exchangeType,
		Flags:        flags,
		MessageID:    messageID,
		NextPayload:  wire.PayloadTypeSK,
		Length:       totalLen,
	}
	hdr.WriteTo(buf, 0)
	skGH := wire.GenericHeader{
		NextPayload: firstType,
		Length:      uint16(totalLen - wire.HeaderLen),
	}
	skGH.WriteTo(buf, wire.HeaderLen)
}

// decryptSKPayload decrypts an SK payload from a raw IKE message.
// rawMsg is the complete message bytes. skPayload is the parsed SK payload.
func decryptSKPayload(sa *SA, rawMsg []byte, skPayload *wire.PayloadSK) ([]byte, error) {
	if sa.Proposal.Encryption.IsAEAD {
		aadLen := wire.HeaderLen + wire.GenericHeaderLen
		var aad []byte
		if len(rawMsg) >= aadLen {
			aad = rawMsg[:aadLen]
		}
		return ikecrypto.DecryptIKEAEAD(skRecvEncKey(sa), skPayload.CipherText, aad)
	}

	integTrunc := int(sa.Proposal.Integrity.TruncatedLength)
	if integTrunc == 0 {
		integTrunc = 16
	}
	if len(rawMsg) < integTrunc {
		return nil, errInvalidMessage
	}
	macData := rawMsg[:len(rawMsg)-integTrunc]
	macExpected := rawMsg[len(rawMsg)-integTrunc:]
	if err := ikecrypto.VerifyIntegrity(sa.Proposal.Integrity.ID, skRecvIntegKey(sa), macData, macExpected); err != nil {
		return nil, fmt.Errorf("ike: integrity verification failed: %w", err)
	}

	ct := skPayload.CipherText
	if len(ct) < integTrunc {
		return nil, errInvalidMessage
	}
	ct = ct[:len(ct)-integTrunc]
	padded, err := ikecrypto.DecryptAESCBCRaw(skRecvEncKey(sa), ct)
	if err != nil {
		return nil, err
	}
	if len(padded) == 0 {
		return nil, errInvalidMessage
	}
	padLen := int(padded[len(padded)-1])
	end := len(padded) - 1 - padLen
	if end < 0 {
		return nil, errInvalidMessage
	}
	return padded[:end], nil
}

// OID constants for ASN.1 AlgorithmIdentifier (RFC 7427 Appendix A).
//
// These are the BARE OID encodings. They are the match keys hashFromAlgID
// compares against, after it has pulled the OID out of whichever form the peer
// sent. What Ze EMITS is the wrapped form built below, never one of these.
var (
	oidSHA256WithRSA, _ = asn1.Marshal(asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 11})
	oidSHA384WithRSA, _ = asn1.Marshal(asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 12})
	oidSHA512WithRSA, _ = asn1.Marshal(asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 13})
	oidECDSASHA256, _   = asn1.Marshal(asn1.ObjectIdentifier{1, 2, 840, 10045, 4, 3, 2})
	oidECDSASHA384, _   = asn1.Marshal(asn1.ObjectIdentifier{1, 2, 840, 10045, 4, 3, 3})
)

// rsaAlgorithmIdentifier and ecAlgorithmIdentifier are the two DER shapes of
// RFC 5280 Section 4.1.1.2 AlgorithmIdentifier that IKEv2 signature auth uses.
// They differ only in the parameters field, and the difference is required
// rather than cosmetic: RFC 4055 Section 5 says the parameters of an
// RSASSA-PKCS1-v1_5 algorithm MUST be present and MUST be NULL, and RFC 5758
// Section 3.2 says the parameters of ecdsa-with-SHA256 and ecdsa-with-SHA384
// MUST be absent.
type rsaAlgorithmIdentifier struct {
	Algorithm  asn1.ObjectIdentifier
	Parameters asn1.RawValue
}

type ecAlgorithmIdentifier struct {
	Algorithm asn1.ObjectIdentifier
}

// AlgorithmIdentifier encodings Ze emits in the AUTH payload.
//
// RFC 7427 Section 3 defines the Authentication Data as an ASN.1 Length octet,
// then an "ASN.1 AlgorithmIdentifier object", then the signature. An
// AlgorithmIdentifier is a SEQUENCE that wraps the OID, so the bare OID Ze used
// to emit was not one: strongSwan parses that field as ASN.1 and refuses the
// payload with "digital signature authentication payload invalid", which failed
// every certificate-authenticated exchange against it.
var (
	algIDSHA256WithRSA = algorithmIdentifierRSA(asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 11})
	algIDSHA384WithRSA = algorithmIdentifierRSA(asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 12})
	algIDSHA512WithRSA = algorithmIdentifierRSA(asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 13})
	algIDECDSASHA256   = algorithmIdentifierEC(asn1.ObjectIdentifier{1, 2, 840, 10045, 4, 3, 2})
	algIDECDSASHA384   = algorithmIdentifierEC(asn1.ObjectIdentifier{1, 2, 840, 10045, 4, 3, 3})
)

// algorithmIdentifierRSA encodes SEQUENCE { OID, NULL }.
func algorithmIdentifierRSA(oid asn1.ObjectIdentifier) []byte {
	der, err := asn1.Marshal(rsaAlgorithmIdentifier{Algorithm: oid, Parameters: asn1.NullRawValue})
	if err != nil {
		panic("BUG: cannot DER-encode a constant RSA signature AlgorithmIdentifier")
	}
	return der
}

// algorithmIdentifierEC encodes SEQUENCE { OID }.
func algorithmIdentifierEC(oid asn1.ObjectIdentifier) []byte {
	der, err := asn1.Marshal(ecAlgorithmIdentifier{Algorithm: oid})
	if err != nil {
		panic("BUG: cannot DER-encode a constant ECDSA signature AlgorithmIdentifier")
	}
	return der
}

func selectSignatureAlgorithm(key crypto.PrivateKey, remoteHashAlgos []uint16) (algID []byte, h crypto.Hash, err error) {
	switch k := key.(type) {
	case *rsa.PrivateKey:
		_ = k
		if containsHashAlgo(remoteHashAlgos, 4) {
			return algIDSHA512WithRSA, crypto.SHA512, nil
		}
		if containsHashAlgo(remoteHashAlgos, 3) {
			return algIDSHA384WithRSA, crypto.SHA384, nil
		}
		return algIDSHA256WithRSA, crypto.SHA256, nil
	case *ecdsa.PrivateKey:
		switch k.Curve {
		case elliptic.P384():
			return algIDECDSASHA384, crypto.SHA384, nil
		default:
			return algIDECDSASHA256, crypto.SHA256, nil
		}
	}
	return nil, 0, errUnsupportedKey
}

func containsHashAlgo(algos []uint16, target uint16) bool {
	if len(algos) == 0 {
		return true
	}
	return slices.Contains(algos, target)
}

func signDigest(key crypto.PrivateKey, digest []byte, hashFunc crypto.Hash) ([]byte, error) {
	switch k := key.(type) {
	case *rsa.PrivateKey:
		return rsa.SignPKCS1v15(crand.Reader, k, hashFunc, digest)
	case *ecdsa.PrivateKey:
		return ecdsa.SignASN1(crand.Reader, k, digest)
	}
	return nil, errUnsupportedKey
}

func hashFromAlgID(algID []byte) func() hash.Hash {
	// RFC 7427: algID may be a raw OID (Ze format) or a full AlgorithmIdentifier
	// SEQUENCE { OID [, params] } (strongSwan format). Extract the OID for matching.
	oid := algID
	if len(algID) > 2 && algID[0] == 0x30 {
		var inner asn1.RawValue
		if _, err := asn1.Unmarshal(algID, &inner); err == nil {
			var oidVal asn1.RawValue
			if _, err := asn1.Unmarshal(inner.Bytes, &oidVal); err == nil && oidVal.Tag == asn1.TagOID {
				oid = oidVal.FullBytes
			}
		}
	}
	switch {
	case bytes.Equal(oid, oidSHA256WithRSA) || bytes.Equal(oid, oidECDSASHA256):
		return sha256.New
	case bytes.Equal(oid, oidSHA384WithRSA) || bytes.Equal(oid, oidECDSASHA384):
		return sha512.New384
	case bytes.Equal(oid, oidSHA512WithRSA):
		return sha512.New
	}
	return nil
}

func verifySignature(pubKey crypto.PublicKey, digest, signature []byte, hashFunc func() hash.Hash) error {
	switch k := pubKey.(type) {
	case *rsa.PublicKey:
		h := cryptoHashFromFunc(hashFunc)
		return rsa.VerifyPKCS1v15(k, h, digest, signature)
	case *ecdsa.PublicKey:
		if !ecdsa.VerifyASN1(k, digest, signature) {
			return errSignatureFailed
		}
		return nil
	}
	return errUnsupportedKey
}

func cryptoHashFromFunc(h func() hash.Hash) crypto.Hash {
	switch h().Size() {
	case 32:
		return crypto.SHA256
	case 48:
		return crypto.SHA384
	case 64:
		return crypto.SHA512
	}
	return crypto.SHA256
}
