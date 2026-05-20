// Design: plan/spec-ipsec-7-ikev2-engine.md -- AUTH payload computation
// RFC: rfc/short/rfc7296.md -- Authentication of the IKE SA (Section 2.15)
// RFC: rfc/short/rfc7427.md -- Digital Signature AUTH method 14
package engine

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/subtle"
	"crypto/x509"
	"encoding/asn1"
	"errors"
	"fmt"
	"hash"
	"slices"

	ikecrypto "codeberg.org/thomas-mangin/ze/internal/component/ike/crypto"
	"codeberg.org/thomas-mangin/ze/internal/component/ike/wire"
	"codeberg.org/thomas-mangin/ze/internal/component/ipsec"
	"codeberg.org/thomas-mangin/ze/internal/component/pki"
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
func computeSignedOctets(sa *SA, isInitiator bool) ([]byte, error) {
	var realMsg, peerNonce, skP []byte
	if isInitiator {
		realMsg = sa.InitiatorSAInitMsg
		peerNonce = sa.RemoteNonce
		skP = sa.SKKeys.SK_pi
	} else {
		realMsg = sa.ResponderSAInitMsg
		peerNonce = sa.LocalNonce
		skP = sa.SKKeys.SK_pr
	}

	idPayload := buildIDPayload(sa, isInitiator)
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
func buildAuthRequest(sa *SA) ([]byte, error) {
	authPayload, err := computeLocalAuth(sa)
	if err != nil {
		return nil, err
	}

	idPayload := &wire.PayloadID{
		IDPayloadType: wire.PayloadTypeIDi,
		IDType:        wire.IDTypeFQDN,
		IDData:        []byte(sa.PeerName),
	}
	if sa.PeerCfg.Auth.LocalID != "" {
		idPayload.IDData = []byte(sa.PeerCfg.Auth.LocalID)
	}

	innerPayloads := []wire.PayloadEntry{
		{Payload: idPayload},
		{Payload: authPayload},
	}

	if sa.PeerCfg.Auth.Mode == ipsec.AuthX509 {
		certPayloads := buildCertPayloads(sa)
		innerPayloads = append(certPayloads, innerPayloads...)
	}

	innerBuf := make([]byte, 16384)
	off := 0
	for i := range innerPayloads {
		var gh wire.GenericHeader
		gh.Critical = innerPayloads[i].Critical
		if i+1 < len(innerPayloads) {
			gh.NextPayload = innerPayloads[i+1].Payload.Type()
		}
		ghOff := off
		off += wire.GenericHeaderLen
		bodyLen := innerPayloads[i].Payload.WriteTo(innerBuf, off)
		off += bodyLen
		gh.Length = uint16(wire.GenericHeaderLen + bodyLen)
		gh.WriteTo(innerBuf, ghOff)
	}
	plaintext := innerBuf[:off]

	var nextPayload uint8
	if len(innerPayloads) > 0 {
		nextPayload = innerPayloads[0].Payload.Type()
	}
	ciphertext, err := encryptPayload(sa, plaintext, nextPayload)
	if err != nil {
		return nil, err
	}

	msg := wire.Message{
		Header: wire.Header{
			InitiatorSPI: sa.InitiatorSPI,
			ResponderSPI: sa.ResponderSPI,
			MajorVersion: 2,
			MinorVersion: 0,
			ExchangeType: wire.ExchangeIKEAuth,
			Flags:        wire.FlagInitiator,
			MessageID:    1,
		},
		Payloads: []wire.PayloadEntry{
			{Payload: &wire.PayloadSK{CipherText: ciphertext}},
		},
	}

	buf := make([]byte, 16384)
	n := msg.WriteTo(buf, 0)
	return buf[:n], nil
}

// computeLocalAuth computes the local AUTH payload.
func computeLocalAuth(sa *SA) (*wire.PayloadAUTH, error) {
	switch sa.PeerCfg.Auth.Mode {
	case ipsec.AuthPreSharedSecret:
		return computePSKAuth(sa)
	case ipsec.AuthX509:
		return computeX509Auth(sa)
	case ipsec.AuthUnknown, ipsec.AuthEAPTLS, ipsec.AuthEAPMSCHAPv2:
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
func verifyRemoteAuth(sa *SA, authPayload *wire.PayloadAUTH) error {
	signedOctets, err := computeSignedOctets(sa, !sa.IsInitiator)
	if err != nil {
		return err
	}

	switch authPayload.AuthMethod {
	case wire.AuthMethodPSK:
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
	caName := sa.PeerCfg.Auth.CACertificate
	if caName == "" {
		return nil, fmt.Errorf("ike auth: no CA certificate configured for verification")
	}
	ca := pki.GetCA(caName)
	if ca == nil {
		return nil, fmt.Errorf("ike auth: CA %q not found", caName)
	}
	return ca.Certificate, nil
}

func buildIDPayload(sa *SA, isInitiator bool) *wire.PayloadID {
	ptype := wire.PayloadTypeIDi
	if !isInitiator {
		ptype = wire.PayloadTypeIDr
	}
	idData := []byte(sa.PeerName)
	if sa.PeerCfg.Auth.LocalID != "" {
		idData = []byte(sa.PeerCfg.Auth.LocalID)
	}
	return &wire.PayloadID{
		IDPayloadType: ptype,
		IDType:        wire.IDTypeFQDN,
		IDData:        idData,
	}
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

// encryptPayload encrypts inner payloads using the IKE SA's encryption keys.
func encryptPayload(sa *SA, plaintext []byte, nextPayload uint8) ([]byte, error) {
	if sa.Proposal.Encryption.IsAEAD {
		ct, err := ikecrypto.EncryptAESGCM(sa.SKKeys.SK_ei, plaintext, nil)
		if err != nil {
			return nil, err
		}
		result := make([]byte, 1+len(ct))
		result[0] = nextPayload
		copy(result[1:], ct)
		return result, nil
	}

	ct, err := ikecrypto.EncryptAESCBC(sa.SKKeys.SK_ei, plaintext)
	if err != nil {
		return nil, err
	}
	result := make([]byte, 1+len(ct))
	result[0] = nextPayload
	copy(result[1:], ct)
	return result, nil
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
		return rsa.SignPKCS1v15(rand.Reader, k, hashFunc, digest)
	case *ecdsa.PrivateKey:
		return ecdsa.SignASN1(rand.Reader, k, digest)
	}
	return nil, errUnsupportedKey
}

func hashFromAlgID(algID []byte) func() hash.Hash {
	switch {
	case bytes.Equal(algID, oidSHA256WithRSA) || bytes.Equal(algID, oidECDSASHA256):
		return sha256.New
	case bytes.Equal(algID, oidSHA384WithRSA) || bytes.Equal(algID, oidECDSASHA384):
		return sha512.New384
	case bytes.Equal(algID, oidSHA512WithRSA):
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
