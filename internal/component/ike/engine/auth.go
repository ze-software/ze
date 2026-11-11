// Design: docs/architecture/ike/ipsec-7-ikev2-engine.md -- AUTH payload computation
// Related: remote_id.go -- remote identity policy and certificate binding
// Detail: cert_payload.go -- CERT payload assembly, the received-chain bound, Hash and URL
// Detail: auth_ecdsa.go -- RFC 4754 ECDSA AUTH methods 9, 10 and 11
// Detail: auth_signature_scheme.go -- the Digital Signature algorithm table method 14 signs and verifies through
// RFC: rfc/short/rfc7296.md -- Authentication of the IKE SA (Section 2.15)
// RFC: rfc/short/rfc7427.md -- Digital Signature AUTH method 14
// RFC: rfc/full/rfc8420.txt -- Ed25519 under the Identity hash (no summary: no release profile lists RFC 8420)
package engine

import (
	"crypto/aes"
	gocipher "crypto/cipher"
	crand "crypto/rand"
	"crypto/sha1" //nolint:gosec // required by IKEv2 CERTREQ (RFC 7296 Section 3.7)
	"errors"
	"fmt"
	"net"
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

	// RFC 7427 Section 4 requires the signer to pick an algorithm the peer sent.
	errNoMutualHashAlgo = errors.New(
		"ike auth: the peer offered no signature hash algorithm this key can sign with")
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
	isEAP := ipsec.IsEAPMode(sa.localAuth().Mode)

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
		if sa.localAuth().Mode == ipsec.AuthX509 {
			certPayloads, err := buildCertPayloads(sa)
			if err != nil {
				return nil, err
			}
			innerPayloads = append(certPayloads, innerPayloads...)
		}
		// RFC 7296 Section 3.7: CERTREQ "can appear in the IKE_INIT_SA response
		// and/or the IKE_AUTH request. Certificate Request payloads MAY be included
		// in an exchange when the sender needs to get the certificate of the
		// receiver." Ze needs it whenever it holds a CA to verify the responder
		// against (getRemoteCert refuses an IKE_AUTH response without CERT), and a
		// responder that sends its certificate only when asked (strongSwan's
		// default `send_cert = ifasked`, ike_cert_post.c build_certs) otherwise
		// answers without one.
		if certReq := buildCertRequest(sa); certReq != nil {
			innerPayloads = append(innerPayloads, wire.PayloadEntry{Payload: certReq})
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

	// RFC 4739 Section 2.1 and Section 3.2: MULTIPLE_AUTH_SUPPORTED in the first
	// IKE_AUTH request, and ANOTHER_AUTH_FOLLOWS only beside an AUTH payload.
	multipleAuth, err := multipleAuthRequestNotifies(sa, !isEAP)
	if err != nil {
		return nil, err
	}
	innerPayloads = append(innerPayloads, multipleAuth...)

	// The IKE_AUTH request carries no KE payload for its Child SA (RFC 7296
	// Section 1.2), and Section 2.17 keys that SA from the IKE_SA_INIT nonces. The offer
	// therefore names no Diffie-Hellman group. The esp-group's pfs leaf governs the
	// CREATE_CHILD_SA rekey instead (childRekeyDHGroup, rekey.go).
	espSPI, saPayload, tsi, tsr, err := buildChildSAPayloads(sa, sa.ESPGroup, dhGroupNone)
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
	innerPayloads = append(innerPayloads, mobikeAuthOffer(sa)...)
	if sa.PeerCfg.ProhibitNAT {
		out, _ := sa.sendPath(nil)
		notification, err := noNATsAllowedPayload(sa.localSendAddr(out), sa.remoteUDPAddr())
		if err != nil {
			return nil, err
		}
		innerPayloads = append(innerPayloads, wire.PayloadEntry{Payload: notification})
	}

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
	switch sa.localAuth().Mode {
	case ipsec.AuthPreSharedSecret:
		return computePSKAuth(sa)
	case ipsec.AuthX509:
		return computeX509Auth(sa)
	// Every EAP mode keys its AUTH from the exchange rather than from a
	// configured credential, MD5-Challenge included: it derives no MSK, so
	// eapAuthSecret (eap_auth.go) keys it from SK_pi and SK_pr instead. The modes
	// are named one by one because the exhaustive linter then refuses a mode added
	// to the enum and not answered here.
	case ipsec.AuthEAPTLS, ipsec.AuthEAPMSCHAPv2, ipsec.AuthEAPMD5:
		return computeEAPAuth(sa)
	case ipsec.AuthUnknown:
		return nil, fmt.Errorf("ike auth: unsupported auth mode %s", sa.localAuth().Mode)
	default:
		panic("BUG: unknown local authentication mode")
	}
}

// computePSKAuth computes AUTH using pre-shared key per RFC 7296 Section 2.15.
//
// A pre-shared key IS the "Shared Secret" that section names, so the formula is
// not written here: computeAuthFromSharedSecret (eap_auth.go) is the one place
// the pad string, the PRF order and the operand order are declared. This
// function supplies the secret and refuses an absent one.
func computePSKAuth(sa *SA) (*wire.PayloadAUTH, error) {
	psk := sa.localAuth().PSK
	if psk == "" {
		return nil, errNoPSK
	}

	signedOctets, err := computeSignedOctets(sa, sa.IsInitiator)
	if err != nil {
		return nil, err
	}

	authData, err := computeAuthFromSharedSecret(sa.Proposal.PRF.ID, []byte(psk), signedOctets)
	if err != nil {
		return nil, err
	}

	return &wire.PayloadAUTH{
		AuthMethod: wire.AuthMethodPSK,
		AuthData:   authData,
	}, nil
}

// computeX509Auth computes AUTH using an X.509 certificate's private key: the RFC
// 7427 Digital Signature method (14) when the peer sent SIGNATURE_HASH_ALGORITHMS,
// and the legacy method that fits the key when it did not.
//
// RFC 7427 Section 3 states method 14 as a conditional permission, and the condition
// is checked here: "As the authentication methods are not negotiated in IKEv2, the
// peer is only allowed to use this authentication method if the Notify payload of type
// SIGNATURE_HASH_ALGORITHMS has been sent and received by each peer." Ze sends that
// notify in every IKE_SA_INIT it writes, as an initiator (initiator.go) and as a
// responder (responder.go), so the half left to establish is reception.
// sa.RemoteHashAlgos is nil exactly when nothing arrived (its field comment, and
// parseHashAlgoNotify, which never answers nil).
func computeX509Auth(sa *SA) (*wire.PayloadAUTH, error) {
	certName := sa.localAuth().Certificate
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

	// No notify received: method 14 is not permitted, so the key signs with its
	// legacy method. RFC 8247 Section 3.1 keeps RSA Digital Signature (1) at MUST
	// because "RSA Digital Signature is widely deployed and, therefore, kept for
	// interoperability."
	if sa.RemoteHashAlgos == nil {
		return computeLegacySignatureAuth(sa, certName, entry.PrivateKey, signedOctets)
	}

	algID, hashFunc, err := selectSignatureAlgorithm(entry.PrivateKey, sa.RemoteHashAlgos)
	if err != nil {
		return nil, fmt.Errorf("ike auth: certificate %q cannot sign for peer %q: %w", certName, sa.PeerName, err)
	}

	// RFC 8420 Section 2: the Identity hash runs no hash, so an EdDSA key signs the
	// signed octets whole. Every other algorithm signs their digest.
	input := signedOctets
	if hashFunc != hashIdentity {
		h := hashFunc.New()
		h.Write(signedOctets)
		input = h.Sum(nil)
	}

	sig, err := signDigest(entry.PrivateKey, input, hashFunc)
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
// RFC 7296 Section 2.16: after EAP, the peer's AUTH is keyed by the secret
// eapAuthSecret names for the peer's role, which is the MSK for a key-generating
// method and SK_pi or SK_pr for one that generates no key.
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
	isEAP := ipsec.IsEAPMode(sa.remoteAuth().Mode)

	switch authPayload.AuthMethod {
	case wire.AuthMethodPSK:
		if !isEAP {
			return verifyPSKAuth(sa, authPayload.AuthData, signedOctets)
		}
		// Which secret keys an EAP peer's AUTH is a question for the METHOD, and
		// eapAuthSecret puts it to the session. This branch tested
		// `sa.EAPMSK != [64]byte{}` until 2026-09-01, and that test cannot answer
		// it: an all-zero MSK is the same value for a method that derives no key,
		// for one whose derivation failed, and for a field nobody has set, so the
		// zero was being read as a valid answer (ai/rules/principles.md). It also
		// sent every non-key-deriving method to the refusal below, which is the
		// case RFC 7296 Section 2.16 writes SK_pi and SK_pr for.
		//
		// eapAuthSecret carries the ordering the old test provided by accident:
		// it refuses until the EAP exchange has succeeded, so an AUTH that arrives
		// early still lands in the refusal.
		secret, secretErr := eapAuthSecret(sa, !sa.IsInitiator)
		if secretErr != nil {
			// The receive-side mirror of computeServerAuth (responder_eap.go).
			// RFC 7296 Section 2.16 says EAP methods "MUST be used in conjunction
			// with a public-key-signature-based authentication of the responder to
			// the initiator". Reaching here on an EAP SA means the remote sent a
			// shared-secret AUTH that no completed EAP exchange backs. That is the
			// responder AUTH of the first EAP message, and a pre-shared key does
			// not satisfy the obligation. Refuse it (ai/rules/evidence.md).
			return fmt.Errorf(
				"ike auth: EAP peer %q sent a shared-key AUTH no completed EAP exchange backs, "+
					"and RFC 7296 Section 2.16 requires a public-key signature from the "+
					"responder: %w", sa.PeerName, secretErr)
		}
		return verifyAuthFromSharedSecret(sa.Proposal.PRF.ID, secret, signedOctets, authPayload.AuthData)
	case wire.AuthMethodDigitalSig:
		return verifyX509Auth(sa, authPayload.AuthData, signedOctets)
	case wire.AuthMethodRSASig:
		return verifyLegacyRSAAuth(sa, authPayload.AuthData, signedOctets)
	// RFC 4754 Section 7: the ECDSA methods carry r || s, not RFC 7427's DER.
	case wire.AuthMethodECDSA256:
		return verifyECDSAAuth(sa, ecdsa256, authPayload.AuthData, signedOctets)
	case wire.AuthMethodECDSA384:
		return verifyECDSAAuth(sa, ecdsa384, authPayload.AuthData, signedOctets)
	case wire.AuthMethodECDSA521:
		return verifyECDSAAuth(sa, ecdsa521, authPayload.AuthData, signedOctets)
	}
	return fmt.Errorf("ike auth: unsupported remote auth method %d", authPayload.AuthMethod)
}

// verifyPSKAuth verifies a PSK AUTH payload against the value the configured
// pre-shared key produces over the same signed octets.
//
// verifyAuthFromSharedSecret (eap_auth.go) holds the construction and the
// constant-time comparison, so the two directions of one formula cannot drift
// apart: a change to the pad string or the PRF order moves the sender and the
// receiver together.
func verifyPSKAuth(sa *SA, authData, signedOctets []byte) error {
	psk := sa.remoteAuth().PSK
	if psk == "" {
		return errNoPSK
	}
	return verifyAuthFromSharedSecret(sa.Proposal.PRF.ID, []byte(psk), signedOctets, authData)
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

	// For the Identity hash of RFC 8420 the "digest" is the signed octets whole
	// (identityHash), and the certificate's family refuses a hash it does not sign
	// with, so an id-Ed25519 AUTH from an ECDSA certificate fails, and the reverse.
	h := hashFunc()
	h.Write(signedOctets)
	digest := h.Sum(nil)

	// RFC 7427 Section 3: "For hash truncation, the method specified in ANSI
	// X9.62:2005 [X9.62] MUST be used." The digest goes on whole: for an ECDSA key
	// whose order is shorter than the hash (P-256 with ecdsa-with-SHA384),
	// ecdsa.VerifyASN1 keeps its leftmost bits, which is that method.
	publicKey, err := certificatePublicKey(cert)
	if err != nil {
		return err
	}
	// RFC 7427 Section 3: the AUTH is computed "using a private key associated with the public key sent in the Certificate payload"
	if err := requireKeyFamilyNamesAlgorithm(publicKey, algID); err != nil {
		return err
	}
	return verifySignature(publicKey, digest, signature, hashFunc)
}

// buildCertRequest builds a CERTREQ payload with the SHA-1 hash of the configured CA's public key.
// RFC 7296 Section 3.7: CertAuthority is SHA-1(SubjectPublicKeyInfo DER).
//
// It reads the first round's CA on purpose. Ze sends CERTREQ in its first IKE_AUTH
// message only, before any further round has been announced.
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
	if localID := sa.localAuth().LocalID; localID != "" {
		idStr = localID
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
	// #nosec G407 -- crypto/rand.Read fills this exact IV range above; subsequent writes start at dataOff, past the IV.
	cbc := gocipher.NewCBCEncrypter(block, buf[ivOff:ivOff+blockSize])
	cbc.CryptBlocks(buf[dataOff:dataOff+paddedLen], buf[dataOff:dataOff+paddedLen])

	mac, err := ikecrypto.ComputeIntegrity(sa.Proposal.Integrity.ID, skSendIntegKey(sa), buf[:totalLen-integTrunc])
	if err != nil {
		return nil, err
	}
	copy(buf[totalLen-integTrunc:], mac)
	return buf, nil
}

// buildSKMessageAEADWithMsgID builds a complete IKE message whose Encrypted payload is
// sealed by an authenticated encryption algorithm.
// Wire: [Header 28][SK GH 4][IV 8][ciphertext][ICV].
//
// RFC 5282 Section 3.1: "The Initialization Vector (IV) MUST be eight octets." The ICV
// is inside the ciphertext rather than in a field of its own, because Section 3.2 says
// of both AES GCM and AES CCM that "the Ciphertext field consists of the output of the
// authenticated encryption algorithm. (Note that this field incorporates integrity
// check data.)" So this builder writes the IV and hands the rest to the AEAD, which
// returns the ciphertext with the ICV already inside it.
func buildSKMessageAEADWithMsgID(sa *SA, innerData []byte, firstType uint8, messageID uint32, exchangeType, flags uint8) ([]byte, error) {
	const ivLen = 8

	// RFC 5282 Section 3.2 gives each transform its own ICV length, and the Length
	// field of the fixed header counts it, so the length is read before the buffer is
	// sized. An encryption transform this build does not specify is refused here rather
	// than sized with another transform's ICV.
	icvLen, err := ikecrypto.AEADICVOctets(sa.Proposal.Encryption.ID)
	if err != nil {
		return nil, err
	}

	plaintext := make([]byte, len(innerData)+1)
	copy(plaintext, innerData)
	plaintext[len(innerData)] = 0

	totalLen := wire.HeaderLen + wire.GenericHeaderLen + ivLen + len(plaintext) + icvLen
	buf := make([]byte, totalLen)

	writeAuthHeaderWithMsgID(buf, sa, firstType, uint32(totalLen), messageID, exchangeType, flags)

	// RFC 5282 Section 5.1: the associated data runs "from the first octet of the
	// Fixed IKE Header through the last octet of the Payload Header of the Encrypted
	// Payload", and "The Initialization Vector and Ciphertext fields ... MUST NOT be
	// included in the associated data". writeAuthHeaderWithMsgID puts the SK generic
	// header directly after the fixed header, so this message carries no payload
	// between the two and the span is the first 32 octets.
	aad := buf[:wire.HeaderLen+wire.GenericHeaderLen]

	// SealIKEAEAD draws the eight fresh IV octets RFC 5282 Section 3.1 requires and
	// builds the nonce as the salt then the IV (Section 4). It answers IV || ciphertext,
	// which is exactly the Encrypted payload's data.
	sealed, err := ikecrypto.SealIKEAEAD(sa.Proposal.Encryption.ID, skSendEncKey(sa), plaintext, aad)
	if err != nil {
		return nil, err
	}
	copy(buf[wire.HeaderLen+wire.GenericHeaderLen:], sealed)
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
//
// Its one caller is openProtected (protected_input.go), which decides what a failure
// means. A handler that calls it directly could act on an unauthenticated failure,
// which RFC 7296 Section 2.4 forbids (TestOneFunctionOpensEveryProtectedMessage).
func decryptSKPayload(sa *SA, rawMsg []byte, skPayload *wire.PayloadSK) ([]byte, error) {
	if sa.Proposal.Encryption.IsAEAD {
		// RFC 5282 Section 5.1: "The associated data (A) MUST consist of the partial
		// contents of the IKEv2 message, starting from the first octet of the Fixed
		// IKE Header through the last octet of the Payload Header of the Encrypted
		// Payload (i.e., the fourth octet of the Encrypted Payload) ... This includes
		// any payloads that are between the Fixed IKE Header and the Encrypted
		// Payload." The same section: "The Initialization Vector and Ciphertext
		// fields ... MUST NOT be included in the associated data."
		//
		// The span therefore ends where this payload's DATA begins, which
		// Message.ReadFrom recorded while it walked the chain. A fixed 32 octets is
		// that span only for a message whose Encrypted payload follows the header
		// directly, and a peer that puts an unencrypted payload in front of it is
		// conformant.
		aadLen := skPayload.DataOffset
		if aadLen < wire.HeaderLen+wire.GenericHeaderLen || aadLen > len(rawMsg) {
			return nil, errInvalidMessage
		}
		return ikecrypto.OpenIKEAEAD(sa.Proposal.Encryption.ID, skRecvEncKey(sa), skPayload.CipherText, rawMsg[:aadLen])
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

	// The checksum verified, so from here on the message is the peer's own: only a holder
	// of SK_a computes it, and it covers the IV, the ciphertext, the Padding and the Pad
	// Length (RFC 7296 Section 3.14). Every failure below is therefore marked
	// errInnerParse, the mark of authenticated content that does not parse, and never
	// treated as the unprotected input RFC 7296 Section 2.4 lets nothing conclude from.
	ct := skPayload.CipherText
	if len(ct) < integTrunc {
		return nil, fmt.Errorf("%w: %w", errInnerParse, errInvalidMessage)
	}
	ct = ct[:len(ct)-integTrunc]
	padded, err := ikecrypto.DecryptAESCBCRaw(skRecvEncKey(sa), ct)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errInnerParse, err)
	}
	if len(padded) == 0 {
		return nil, fmt.Errorf("%w: %w", errInnerParse, errInvalidMessage)
	}
	padLen := int(padded[len(padded)-1])
	end := len(padded) - 1 - padLen
	if end < 0 {
		return nil, fmt.Errorf("%w: %w", errInnerParse, errInvalidMessage)
	}
	return padded[:end], nil
}
