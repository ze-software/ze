// Design: plan/learned/740-ipsec-7-ikev2-engine.md -- AUTH payload computation
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
	"crypto/x509"
	"encoding/asn1"
	"errors"
	"fmt"
	"hash"
	"net"
	"slices"

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

// buildAuthRequest constructs an encrypted IKE_AUTH request message.
// RFC 7296 Section 2.16: when auth mode is EAP, the initiator omits AUTH
// in the first IKE_AUTH to signal willingness to use EAP.
func buildAuthRequest(sa *SA) ([]byte, error) {
	isEAP := sa.PeerCfg.Auth.Mode == ipsec.AuthEAPTLS || sa.PeerCfg.Auth.Mode == ipsec.AuthEAPMSCHAPv2

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
			certPayloads := buildCertPayloads(sa)
			innerPayloads = append(certPayloads, innerPayloads...)
		}
	}

	// RFC 7296 Section 2.4: INITIAL_CONTACT "MUST be in the first IKE_AUTH request or
	// response" and "asserts that this IKE SA is the only IKE SA currently active
	// between the authenticated identities", letting the responder delete any stale SA
	// to us without waiting for a timeout. ze is one-SA-per-configured-peer, so this
	// assertion is truthful on every first IKE_AUTH and is emitted unconditionally
	// (spec-fixit-ipsec-clear-reestablish, open question 3). Rekey never reaches here
	// (it uses CREATE_CHILD_SA on the existing SA).
	innerPayloads = append(innerPayloads,
		wire.PayloadEntry{Payload: &wire.PayloadNotify{NotifyMsgType: wire.NotifyInitialContact}},
	)

	espSPI, saPayload, tsi, tsr, err := buildChildSAPayloads(sa)
	if err != nil {
		return nil, fmt.Errorf("ike auth: child SA payloads: %w", err)
	}
	sa.ChildInboundSPI = espSPI
	innerPayloads = append(innerPayloads,
		wire.PayloadEntry{Payload: saPayload},
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
	signedOctets, err := computeSignedOctets(sa, !sa.IsInitiator)
	if err != nil {
		return err
	}

	isEAP := sa.PeerCfg.Auth.Mode == ipsec.AuthEAPTLS || sa.PeerCfg.Auth.Mode == ipsec.AuthEAPMSCHAPv2

	switch authPayload.AuthMethod {
	case wire.AuthMethodPSK:
		if isEAP && sa.EAPMSK != [64]byte{} {
			return VerifyAuthFromMSK(sa.Proposal.PRF.ID, sa.EAPMSK, signedOctets, authPayload.AuthData)
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

func getRemoteCert(sa *SA) (*x509.Certificate, error) {
	if len(sa.RemoteCertRaw) == 0 {
		return nil, fmt.Errorf("ike auth: no remote certificate received in IKE_AUTH")
	}

	cert, err := x509.ParseCertificate(sa.RemoteCertRaw)
	if err != nil {
		return nil, fmt.Errorf("ike auth: parse remote certificate: %w", err)
	}

	caName := sa.PeerCfg.Auth.CACertificate
	if caName != "" {
		ca := pki.GetCA(caName)
		if ca == nil {
			return nil, fmt.Errorf("ike auth: CA %q not found in PKI store", caName)
		}
		pool := x509.NewCertPool()
		pool.AddCert(ca.Certificate)
		if _, err := cert.Verify(x509.VerifyOptions{Roots: pool}); err != nil {
			return nil, fmt.Errorf("ike auth: remote certificate validation: %w", err)
		}
	}

	return cert, nil
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

func buildCertPayloads(sa *SA) []wire.PayloadEntry {
	certName := sa.PeerCfg.Auth.Certificate
	if certName == "" {
		return nil
	}
	entry := pki.GetCertificate(certName)
	if entry == nil {
		return nil
	}

	payloads := make([]wire.PayloadEntry, 0, 1)
	payloads = append(payloads, wire.PayloadEntry{
		Payload: &wire.PayloadCERT{
			CertEncoding: wire.CertEncodingX509Sig,
			CertData:     entry.Raw,
		},
	})
	return payloads
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
var (
	oidSHA256WithRSA, _ = asn1.Marshal(asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 11})
	oidSHA384WithRSA, _ = asn1.Marshal(asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 12})
	oidSHA512WithRSA, _ = asn1.Marshal(asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 13})
	oidECDSASHA256, _   = asn1.Marshal(asn1.ObjectIdentifier{1, 2, 840, 10045, 4, 3, 2})
	oidECDSASHA384, _   = asn1.Marshal(asn1.ObjectIdentifier{1, 2, 840, 10045, 4, 3, 3})
)

func selectSignatureAlgorithm(key crypto.PrivateKey, remoteHashAlgos []uint16) (algID []byte, h crypto.Hash, err error) {
	switch k := key.(type) {
	case *rsa.PrivateKey:
		_ = k
		if containsHashAlgo(remoteHashAlgos, 4) {
			return oidSHA512WithRSA, crypto.SHA512, nil
		}
		if containsHashAlgo(remoteHashAlgos, 3) {
			return oidSHA384WithRSA, crypto.SHA384, nil
		}
		return oidSHA256WithRSA, crypto.SHA256, nil
	case *ecdsa.PrivateKey:
		switch k.Curve {
		case elliptic.P384():
			return oidECDSASHA384, crypto.SHA384, nil
		default:
			return oidECDSASHA256, crypto.SHA256, nil
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
