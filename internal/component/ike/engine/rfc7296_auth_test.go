package engine

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/component/pki"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// autPeers builds the initiator and responder halves of one configured peer, both
// carrying the same auth material. It mirrors responderTestPeers and takes a whole
// AuthConfig, because the X.509 cases need a certificate name and a CA name.
func autPeers(auth ipsec.AuthConfig) (ini, resp ipsec.SiteToSitePeer) {
	ini = ipsec.SiteToSitePeer{
		Name: "ze", IKEGroup: "test-ike", ESPGroup: "test-esp",
		ConnectionType: ipsec.ConnectionInitiate,
		LocalAddress:   "10.0.0.1", RemoteAddress: "10.0.0.2", Auth: auth,
	}
	resp = ipsec.SiteToSitePeer{
		Name: "ze", IKEGroup: "test-ike", ESPGroup: "test-esp",
		ConnectionType: ipsec.ConnectionRespond,
		LocalAddress:   "10.0.0.2", RemoteAddress: "10.0.0.1", Auth: auth,
	}
	return ini, resp
}

// autSAInitPair runs the IKE_SA_INIT exchange only and stops there. Both SAs hold a
// full SK hierarchy, the responder sits at StateSAInitReceived, and the initiator's
// IKE_AUTH request waits in ini.LastSentMsg. The ordering tests need this halfway
// point, which establishPSK passes through and does not expose.
func autSAInitPair(t *testing.T, auth ipsec.AuthConfig) (ini, resp *SA, ps *PeerSession) {
	t.Helper()
	log := slogutil.DiscardLogger()
	ikeGroup := testIKEGroup()
	espGroup := testESPGroup()
	iniPeer, respPeer := autPeers(auth)

	table := NewSATable()
	ini, err := newInitiatorSA("ze", iniPeer, ikeGroup, espGroup)
	if err != nil {
		t.Fatalf("newInitiatorSA: %v", err)
	}
	table.Insert(ini)
	saInitReq := buildSAInitRequest(ini, ikeGroup)
	ini.InitiatorSAInitMsg = saInitReq
	ini.State = StateSAInitSent

	resp, err = newResponderSA("ze", respPeer, ikeGroup, espGroup, ini.InitiatorSPI)
	if err != nil {
		t.Fatalf("newResponderSA: %v", err)
	}
	handleSAInitRequest(resp, parseMsg(t, saInitReq), saInitReq, nil, nil, log)
	handleSAInitResponse(ini, parseMsg(t, resp.LastSentMsg), resp.LastSentMsg, table, nil, nil, log)
	if resp.State != StateSAInitReceived || len(ini.LastSentMsg) == 0 {
		t.Fatalf("autSAInitPair: resp=%v, initiator IKE_AUTH bytes=%d", resp.State, len(ini.LastSentMsg))
	}
	ps = &PeerSession{peerName: "ze", peerCfg: respPeer, ikeGroup: ikeGroup, espGroup: espGroup}
	return ini, resp, ps
}

// autCertNames are the PKI store names autLoadPKI installs.
const (
	autCAName    = "aut-ca"
	autCertName  = "aut-dev"
	autOtherName = "aut-other"
)

// autLoadPKI installs a CA and two device certificates signed by it, and returns the
// DER of the second one. The second certificate is the wrong key for every AUTH the
// first one signs, which is what the negative halves need.
func autLoadPKI(t *testing.T) (otherDER []byte) {
	t.Helper()
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate CA key: %v", err)
	}
	caTmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: autCAName},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTmpl, caTmpl, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("create CA certificate: %v", err)
	}
	caCert, err := x509.ParseCertificate(caDER)
	if err != nil {
		t.Fatalf("parse CA certificate: %v", err)
	}

	leaf := func(serial int64, cn string) (*x509.Certificate, []byte, *ecdsa.PrivateKey) {
		key, keyErr := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if keyErr != nil {
			t.Fatalf("generate %s key: %v", cn, keyErr)
		}
		tmpl := &x509.Certificate{
			SerialNumber: big.NewInt(serial),
			Subject:      pkix.Name{CommonName: cn},
			NotBefore:    time.Now().Add(-time.Hour),
			NotAfter:     time.Now().Add(time.Hour),
		}
		der, certErr := x509.CreateCertificate(rand.Reader, tmpl, caCert, &key.PublicKey, caKey)
		if certErr != nil {
			t.Fatalf("create %s certificate: %v", cn, certErr)
		}
		parsed, parseErr := x509.ParseCertificate(der)
		if parseErr != nil {
			t.Fatalf("parse %s certificate: %v", cn, parseErr)
		}
		return parsed, der, key
	}

	devCert, devDER, devKey := leaf(2, autCertName)
	otherCert, other, otherKey := leaf(3, autOtherName)

	if err := pki.Load(&pki.PKIConfig{
		CACerts: map[string]*pki.CACertEntry{
			autCAName: {Name: autCAName, Certificate: caCert, Raw: caDER},
		},
		Certificates: map[string]*pki.CertificateEntry{
			autCertName:  {Name: autCertName, Certificate: devCert, Raw: devDER, PrivateKey: devKey},
			autOtherName: {Name: autOtherName, Certificate: otherCert, Raw: other, PrivateKey: otherKey},
		},
	}); err != nil {
		t.Fatalf("pki.Load: %v", err)
	}
	t.Cleanup(func() { _ = pki.Load(nil) })
	return other
}

// autSplit returns the certificate payloads in wire order, plus the AUTH and the
// initiator ID from one decrypted payload chain.
func autSplit(inner []wire.PayloadEntry) (certs []*wire.PayloadCERT, auth *wire.PayloadAUTH, idi *wire.PayloadID) {
	for i := range inner {
		switch p := inner[i].Payload.(type) {
		case *wire.PayloadCERT:
			certs = append(certs, p)
		case *wire.PayloadAUTH:
			auth = p
		case *wire.PayloadID:
			if p.IDPayloadType == wire.PayloadTypeIDi {
				idi = p
			}
		}
	}
	return certs, auth, idi
}

// VALIDATES: RFC7296-1-1. The responder dispatch refuses an exchange that arrives out
// of order, and accepts the same bytes in the state the RFC allows.
// PREVENTS: an IKE_AUTH processed before IKE_SA_INIT completes, and an INFORMATIONAL
// processed before IKE_AUTH completes.
// RFC requirement: RFC7296-1-1 positive -- handleResponderInbound (responder.go:65-88) keys its
// dispatch on sa.State, so an IKE_AUTH at StateIdle reaches no handler.
// RFC requirement: RFC7296-1-1 negative -- those identical bytes establish the SA once the state
// advances, so the refusal is the ordering gate, not a bad message.
func TestAutExchangesRunInRFCOrder(t *testing.T) {
	log := slogutil.DiscardLogger()
	ini, resp, ps := autSAInitPair(t, ipsec.AuthConfig{Mode: ipsec.AuthPreSharedSecret, PSK: "order-secret"})

	authReq := ini.LastSentMsg
	// A DPD probe: an INFORMATIONAL request with no payloads, at the first message ID
	// after IKE_AUTH. Its keys come from IKE_SA_INIT, so it is valid from here on.
	info, err := buildEncryptedMessageEx(ini, nil, 2, wire.ExchangeInformational, wire.FlagInitiator)
	if err != nil {
		t.Fatalf("build INFORMATIONAL request: %v", err)
	}
	// The responder answered IKE_SA_INIT at message ID 0 and holds that answer.
	if !resp.lastResponseSet || resp.lastResponseID != 0 {
		t.Fatalf("after IKE_SA_INIT: response memory set=%v id=%d, want set=true id=0",
			resp.lastResponseSet, resp.lastResponseID)
	}

	// IKE_AUTH before IKE_SA_INIT completes. The SA holds every key the message needs,
	// so only the ordering gate can refuse it.
	resp.State = StateIdle
	ps.handleResponderInbound(resp, parseMsg(t, authReq), transport.Packet{Data: authReq}, nil, log)
	if resp.State != StateIdle {
		t.Errorf("IKE_AUTH at StateIdle moved the SA to %v, want StateIdle", resp.State)
	}
	if resp.lastResponseID != 0 {
		t.Errorf("IKE_AUTH at StateIdle was answered at id %d", resp.lastResponseID)
	}

	// INFORMATIONAL before IKE_AUTH completes.
	resp.State = StateSAInitReceived
	ps.handleResponderInbound(resp, parseMsg(t, info), transport.Packet{Data: info}, nil, log)
	if resp.State != StateSAInitReceived {
		t.Errorf("INFORMATIONAL before IKE_AUTH moved the SA to %v", resp.State)
	}
	if resp.lastResponseID != 0 {
		t.Errorf("INFORMATIONAL before IKE_AUTH was answered at id %d", resp.lastResponseID)
	}

	// A CREATE_CHILD_SA before IKE_AUTH completes is refused the same way.
	child, err := buildEncryptedMessageEx(ini, nil, 2, wire.ExchangeCreateChildSA, wire.FlagInitiator)
	if err != nil {
		t.Fatalf("build CREATE_CHILD_SA request: %v", err)
	}
	ps.handleResponderInbound(resp, parseMsg(t, child), transport.Packet{Data: child}, nil, log)
	if resp.State != StateSAInitReceived {
		t.Errorf("CREATE_CHILD_SA before IKE_AUTH moved the SA to %v", resp.State)
	}
	if resp.lastResponseID != 0 {
		t.Errorf("CREATE_CHILD_SA before IKE_AUTH was answered at id %d", resp.lastResponseID)
	}

	// Negative half. The same IKE_AUTH bytes establish the SA in the state that allows
	// them, so the refusal above was ordering and nothing else.
	ps.handleResponderInbound(resp, parseMsg(t, authReq), transport.Packet{Data: authReq}, nil, log)
	if resp.State != StateEstablished {
		t.Fatalf("IKE_AUTH in order left the SA at %v, want StateEstablished", resp.State)
	}
	if resp.lastResponseID != 1 {
		t.Fatalf("IKE_AUTH in order was answered at id %d, want 1", resp.lastResponseID)
	}

	// Negative half. The same INFORMATIONAL bytes reach the handler and draw an answer
	// once IKE_AUTH has completed.
	out := ps.handleOwnedInbound(resp, transport.Packet{Data: info}, nil, nil, log)
	if !out.peerAlive {
		t.Error("INFORMATIONAL after IKE_AUTH never reached the handler")
	}
	if resp.lastResponseID != 2 {
		t.Errorf("INFORMATIONAL after IKE_AUTH was answered at id %d, want 2", resp.lastResponseID)
	}
	if resp.State != StateEstablished {
		t.Errorf("INFORMATIONAL after IKE_AUTH moved the SA to %v", resp.State)
	}
}

// VALIDATES: RFC7296-1.2-2. The first CERT payload of an X.509 IKE_AUTH request holds
// the public key that verifies that message's AUTH payload.
// PREVENTS: a first certificate that is an issuer, which no peer can verify AUTH from.
// RFC requirement: RFC7296-1.2-2 positive -- buildCertPayloads (auth.go:476) reads the PKI entry
// that computeX509Auth (auth.go:262) signs with, so the first certificate verifies AUTH.
// RFC requirement: RFC7296-1.2-2 negative -- a second certificate from the same CA fails on that
// AUTH, so the pass names one key, not any trusted chain.
func TestAutFirstCertificateCarriesTheAuthKey(t *testing.T) {
	otherDER := autLoadPKI(t)
	auth := ipsec.AuthConfig{
		Mode:          ipsec.AuthX509,
		Certificate:   autCertName,
		CACertificate: autCAName,
	}
	ini, resp, _ := autSAInitPair(t, auth)

	raw := ini.LastSentMsg
	inner, err := decryptAndParse(resp, parseMsg(t, raw), raw)
	if err != nil {
		t.Fatalf("decrypt the X.509 IKE_AUTH request: %v", err)
	}
	certs, authPayload, idi := autSplit(inner)
	if len(certs) == 0 {
		t.Fatal("the X.509 IKE_AUTH request carried no CERT payload")
	}
	if authPayload == nil || idi == nil {
		t.Fatalf("the request is missing AUTH=%v or IDi=%v", authPayload != nil, idi != nil)
	}
	if authPayload.AuthMethod != wire.AuthMethodDigitalSig {
		t.Fatalf("AUTH method = %d, want %d (Digital Signature)",
			authPayload.AuthMethod, wire.AuthMethodDigitalSig)
	}
	// The responder learns the peer identity from the message, exactly as
	// handleAuthRequest does, so the signed octets match what the initiator signed.
	resp.RemoteIDPayload = idi

	// Positive. The FIRST certificate verifies the AUTH payload.
	resp.RemoteCertRaw = certs[0].CertData
	if err := verifyRemoteAuth(resp, authPayload); err != nil {
		t.Fatalf("the first certificate did not verify the AUTH payload: %v", err)
	}

	// Negative. Another certificate from the same CA does not verify that AUTH, so the
	// pass above identifies one key rather than accepting any trusted certificate.
	resp.RemoteCertRaw = otherDER
	if err := verifyRemoteAuth(resp, authPayload); err == nil {
		t.Fatal("a different certificate from the same CA verified the AUTH payload")
	}
}

// VALIDATES: RFC7296-1.2-3. An IKE_AUTH message carries two Message Authentication
// Codes, and both are checked before the message is acted on.
// PREVENTS: an IKE_AUTH accepted with a forged AUTH payload, or with a payload chain
// altered after the sender computed the checksum.
// RFC requirement: RFC7296-1.2-3 positive -- verifyRemoteAuth (auth.go:306) rejects an altered
// AUTH payload, and decryptSKPayload (auth.go:658) rejects an altered checksum.
// RFC requirement: RFC7296-1.2-3 negative -- the untouched AUTH payload and the untouched
// message both pass, so neither rejection comes from a verifier that refuses all input.
func TestAutIKEAuthVerifiesEveryMAC(t *testing.T) {
	ini, resp, _ := autSAInitPair(t, ipsec.AuthConfig{Mode: ipsec.AuthPreSharedSecret, PSK: "mac-secret"})

	raw := ini.LastSentMsg
	inner, err := decryptAndParse(resp, parseMsg(t, raw), raw)
	if err != nil {
		t.Fatalf("decrypt the IKE_AUTH request: %v", err)
	}
	_, authPayload, idi := autSplit(inner)
	if authPayload == nil || idi == nil {
		t.Fatalf("the request is missing AUTH=%v or IDi=%v", authPayload != nil, idi != nil)
	}
	resp.RemoteIDPayload = idi

	// Negative half, taken first. The untouched AUTH payload and the untouched message
	// both verify, so every rejection below is caused by the alteration.
	if err := verifyRemoteAuth(resp, authPayload); err != nil {
		t.Fatalf("the untouched AUTH payload failed verification: %v", err)
	}

	// The AUTH payload MAC. One flipped bit fails the compare.
	forged := &wire.PayloadAUTH{
		AuthMethod: authPayload.AuthMethod,
		AuthData:   bytes.Clone(authPayload.AuthData),
	}
	forged.AuthData[0] ^= 0x01
	if err := verifyRemoteAuth(resp, forged); err == nil {
		t.Error("an AUTH payload with a flipped bit verified")
	}

	// A second forgery, at the far end of the MAC, so the check covers the whole value.
	forged.AuthData = bytes.Clone(authPayload.AuthData)
	forged.AuthData[len(forged.AuthData)-1] ^= 0x80
	if err := verifyRemoteAuth(resp, forged); err == nil {
		t.Error("an AUTH payload with a flipped trailing bit verified")
	}

	// The message integrity checksum. The test IKE proposal is AES-CBC with
	// HMAC-SHA2-256-128, so the last 16 bytes of the datagram are the checksum.
	trunc := int(resp.Proposal.Integrity.TruncatedLength)
	if trunc == 0 {
		trunc = 16
	}
	tampered := bytes.Clone(raw)
	tampered[len(tampered)-1] ^= 0x01
	if _, err := decryptAndParse(resp, parseMsg(t, tampered), tampered); err == nil {
		t.Error("a message with a flipped checksum bit was accepted")
	}

	// A flipped ciphertext bit fails the same check, which proves the checksum covers
	// the payload chain and not only the trailing bytes.
	tampered = bytes.Clone(raw)
	tampered[len(tampered)-trunc-1] ^= 0x01
	if _, err := decryptAndParse(resp, parseMsg(t, tampered), tampered); err == nil {
		t.Error("a message with a flipped ciphertext bit was accepted")
	}
}

// VALIDATES: RFC7296-1.2-3. A digital signature in an AUTH payload is verified against
// the sender's certificate, and a forged signature is refused.
// PREVENTS: signature verification reduced to a length or format check.
// RFC requirement: RFC7296-1.2-3 positive -- verifyX509Auth (auth.go:354) hashes the signed
// octets and calls verifySignature (auth.go:754), so one flipped bit fails.
// RFC requirement: RFC7296-1.2-3 negative -- the unflipped signature verifies over the same
// octets, so the refusal tracks the forgery, not a verifier that refuses all.
func TestAutIKEAuthVerifiesSignatures(t *testing.T) {
	autLoadPKI(t)
	auth := ipsec.AuthConfig{
		Mode:          ipsec.AuthX509,
		Certificate:   autCertName,
		CACertificate: autCAName,
	}
	ini, resp, _ := autSAInitPair(t, auth)

	raw := ini.LastSentMsg
	inner, err := decryptAndParse(resp, parseMsg(t, raw), raw)
	if err != nil {
		t.Fatalf("decrypt the X.509 IKE_AUTH request: %v", err)
	}
	certs, authPayload, idi := autSplit(inner)
	if len(certs) == 0 || authPayload == nil || idi == nil {
		t.Fatalf("the request is missing certs=%d AUTH=%v IDi=%v",
			len(certs), authPayload != nil, idi != nil)
	}
	resp.RemoteIDPayload = idi
	resp.RemoteCertRaw = certs[0].CertData

	// Negative half. The signature as sent verifies.
	if err := verifyRemoteAuth(resp, authPayload); err != nil {
		t.Fatalf("the untouched signature failed verification: %v", err)
	}

	// Positive half. The signature sits after the ASN.1 algorithm identifier, whose
	// length the first byte carries (RFC 7427 Section 3). Flip a bit inside it.
	sigOffset := 1 + int(authPayload.AuthData[0])
	if sigOffset >= len(authPayload.AuthData) {
		t.Fatalf("AUTH data holds no signature: algID length %d of %d bytes",
			authPayload.AuthData[0], len(authPayload.AuthData))
	}
	forged := &wire.PayloadAUTH{
		AuthMethod: authPayload.AuthMethod,
		AuthData:   bytes.Clone(authPayload.AuthData),
	}
	forged.AuthData[len(forged.AuthData)-1] ^= 0x01
	if err := verifyRemoteAuth(resp, forged); err == nil {
		t.Error("an AUTH signature with a flipped bit verified")
	}

	// A different signed value must not verify against the same signature either, which
	// binds the signature to the octets and not only to the certificate.
	resp.RemoteIDPayload = &wire.PayloadID{
		IDPayloadType: wire.PayloadTypeIDi,
		IDType:        wire.IDTypeFQDN,
		IDData:        []byte("aut-not-the-signer"),
	}
	if err := verifyRemoteAuth(resp, authPayload); err == nil {
		t.Error("the signature verified over octets the sender never signed")
	}
}

// VALIDATES: RFC7296-1.2-4. A shared-secret AUTH payload is bound to the name in the ID
// payload, and to the secret that produced it.
// PREVENTS: an AUTH payload accepted under a name its sender never claimed.
// RFC requirement: RFC7296-1.2-4 positive -- computeSignedOctets (auth.go:50) folds the ID into
// the octets computePSKAuth (auth.go:232) authenticates, so a swapped name fails.
// RFC requirement: RFC7296-1.2-4 negative -- the same AUTH payload verifies under the name its
// sender sent, so the two failures come from the swap alone.
func TestAutSharedSecretAuthBindsTheIDName(t *testing.T) {
	const secret = "id-binding-secret"
	auth := ipsec.AuthConfig{Mode: ipsec.AuthPreSharedSecret, PSK: secret, LocalID: "gw-alpha"}
	ini, resp, _ := autSAInitPair(t, auth)

	raw := ini.LastSentMsg
	inner, err := decryptAndParse(resp, parseMsg(t, raw), raw)
	if err != nil {
		t.Fatalf("decrypt the IKE_AUTH request: %v", err)
	}
	_, authPayload, idi := autSplit(inner)
	if authPayload == nil || idi == nil {
		t.Fatalf("the request is missing AUTH=%v or IDi=%v", authPayload != nil, idi != nil)
	}
	if authPayload.AuthMethod != wire.AuthMethodPSK {
		t.Fatalf("AUTH method = %d, want %d (shared key MIC)", authPayload.AuthMethod, wire.AuthMethodPSK)
	}
	if string(idi.IDData) != auth.LocalID {
		t.Fatalf("IDi carried %q, want %q", idi.IDData, auth.LocalID)
	}

	// Negative half. The name the sender asserted verifies the AUTH payload.
	resp.RemoteIDPayload = idi
	if err := verifyRemoteAuth(resp, authPayload); err != nil {
		t.Fatalf("the asserted name failed to verify the AUTH payload: %v", err)
	}

	// Positive half, name. A different name over the same secret fails.
	otherType, otherData := encodeIKEID("gw-beta")
	resp.RemoteIDPayload = &wire.PayloadID{
		IDPayloadType: wire.PayloadTypeIDi,
		IDType:        otherType,
		IDData:        otherData,
	}
	if err := verifyRemoteAuth(resp, authPayload); err == nil {
		t.Error("an AUTH payload verified under a name its sender never asserted")
	}

	// Positive half, secret. The asserted name over a different secret fails too, so
	// the AUTH payload names one key and one identity together.
	resp.RemoteIDPayload = idi
	resp.PeerCfg.Auth.PSK = secret + "-wrong"
	if err := verifyRemoteAuth(resp, authPayload); err == nil {
		t.Error("an AUTH payload verified under a secret that did not produce it")
	}
}

// autRound records one message delivery in the EAP handshake. It holds the exchange
// type and message ID of the datagram, and whether both SAs established.
type autRound struct {
	exchange        uint8
	msgID           uint32
	bothEstablished bool
}

// autEAPHandshake drives a full in-process EAP-MSCHAPv2 handshake and records every
// delivery after IKE_SA_INIT. It follows TestResponderEAPSessionWired, and adds the
// per-delivery record the EAP tests assert on.
func autEAPHandshake(t *testing.T) (ini, resp *SA, rounds []autRound) {
	t.Helper()
	log := slogutil.DiscardLogger()
	ikeGroup := testIKEGroup()
	espGroup := testESPGroup()
	// Auth.PSK is the EAP-MSCHAPv2 password on both sides. The certificate and the
	// CA are separate from it. RFC 7296 Section 2.16 has the responder
	// authenticate back with a public-key signature. computeServerAuth signs with
	// the certificate, and the initiator checks it against the CA. Without them
	// the responder refuses the exchange, which is the conformant outcome.
	autLoadPKI(t)
	iniPeer, respPeer := autPeers(ipsec.AuthConfig{
		Mode:          ipsec.AuthEAPMSCHAPv2,
		PSK:           "eap-pass",
		Certificate:   autCertName,
		CACertificate: autCAName,
	})

	table := NewSATable()
	ini, err := newInitiatorSA("ze", iniPeer, ikeGroup, espGroup)
	if err != nil {
		t.Fatalf("newInitiatorSA: %v", err)
	}
	table.Insert(ini)
	saInitReq := buildSAInitRequest(ini, ikeGroup)
	ini.InitiatorSAInitMsg = saInitReq
	ini.State = StateSAInitSent

	resp, err = newResponderSA("ze", respPeer, ikeGroup, espGroup, ini.InitiatorSPI)
	if err != nil {
		t.Fatalf("newResponderSA: %v", err)
	}
	ps := &PeerSession{peerName: "ze", peerCfg: respPeer, ikeGroup: ikeGroup, espGroup: espGroup}
	ps.setSA(resp)
	setActivePeers(map[string]*PeerSession{"ze": ps})
	t.Cleanup(func() { setActivePeers(nil) })

	handleSAInitRequest(resp, parseMsg(t, saInitReq), saInitReq, nil, nil, log)
	handleSAInitResponse(ini, parseMsg(t, resp.LastSentMsg), resp.LastSentMsg, table, nil, nil, log)

	cur := ini.LastSentMsg
	toResponder := true
	for range 24 {
		delivered := parseMsg(t, cur)
		if toResponder {
			handleInbound(resp, transport.Packet{Data: cur}, table, nil, log)
			cur = resp.LastSentMsg
		} else {
			handleInbound(ini, transport.Packet{Data: cur}, table, nil, log)
			cur = ini.LastSentMsg
		}
		toResponder = !toResponder
		if resp.State == StateDead || ini.State == StateDead {
			t.Fatalf("the EAP handshake died at round %d (ini=%v resp=%v)", len(rounds), ini.State, resp.State)
		}
		both := ini.State == StateEstablished && resp.State == StateEstablished
		rounds = append(rounds, autRound{
			exchange:        delivered.Header.ExchangeType,
			msgID:           delivered.Header.MessageID,
			bothEstablished: both,
		})
		if both {
			return ini, resp, rounds
		}
	}
	t.Fatalf("the EAP handshake did not establish (ini=%v resp=%v)", ini.State, resp.State)
	return nil, nil, nil
}

// VALIDATES: RFC7296-2.16-6. EAP runs as extra IKE_AUTH exchanges, and the IKE SA is
// initialized only once all of them complete.
// PREVENTS: EAP carried on a new exchange type, or an SA established mid-EAP.
// RFC requirement: RFC7296-2.16-6 positive -- buildEAPResponse (auth.go:143) and sendResponderEAP
// (responder_eap.go:248) both stamp wire.ExchangeIKEAuth, so every delivery is IKE_AUTH.
// RFC requirement: RFC7296-2.16-6 negative -- handleResponderEAP (responder_eap.go:199) does
// establish the SA on the final AUTH round, so earlier rounds are merely incomplete.
func TestAutEAPRunsAsExtraIKEAuthExchanges(t *testing.T) {
	_, _, rounds := autEAPHandshake(t)

	t.Logf("the EAP handshake took %d IKE_AUTH deliveries", len(rounds))

	// A direct IKE_AUTH handshake is one request and one response. EAP needs more.
	if len(rounds) <= 2 {
		t.Fatalf("the EAP handshake took %d deliveries, want more than the 2 of a direct IKE_AUTH", len(rounds))
	}

	for i := range rounds {
		if rounds[i].exchange != wire.ExchangeIKEAuth {
			t.Errorf("delivery %d used exchange type %d, want IKE_AUTH (%d)",
				i, rounds[i].exchange, wire.ExchangeIKEAuth)
		}
	}

	// Positive half. Every round before the last leaves the IKE SA uninitialized.
	for i := range rounds[:len(rounds)-1] {
		if rounds[i].bothEstablished {
			t.Errorf("delivery %d of %d established the IKE SA while EAP was outstanding",
				i, len(rounds))
		}
	}

	// Negative half. The last round does establish it.
	if !rounds[len(rounds)-1].bothEstablished {
		t.Fatal("the final EAP delivery left the IKE SA uninitialized")
	}

	// RFC 7296 Section 2.2: each pair of IKE_AUTH messages takes the next message ID.
	if rounds[0].msgID != 1 {
		t.Errorf("the first IKE_AUTH delivery used message ID %d, want 1", rounds[0].msgID)
	}
	if last := rounds[len(rounds)-1].msgID; last <= rounds[0].msgID {
		t.Errorf("the last IKE_AUTH delivery used message ID %d, want more than %d",
			last, rounds[0].msgID)
	}
}

// autMSKAppears reports whether a 16-byte window of the MSK occurs in the key.
func autMSKAppears(msk [64]byte, key []byte) bool {
	for i := 0; i+16 <= len(msk); i++ {
		if bytes.Contains(key, msk[i:i+16]) {
			return true
		}
	}
	return false
}

// VALIDATES: RFC7296-2.16-7. The shared key EAP generates produces the AUTH payload and
// nothing else the session keys with.
// PREVENTS: MSK bytes folded into the SK hierarchy or into Child SA KEYMAT.
// RFC requirement: RFC7296-2.16-7 positive -- a changed sa.EAPMSK leaves the SK hierarchy and the
// KEYMAT that createFirstChildSA (child.go:114) derives identical byte for byte.
// RFC requirement: RFC7296-2.16-7 negative -- that same change does alter the AUTH payload
// computeEAPAuth (eap_auth.go:89) returns, so the untouched keys are a real result.
func TestAutEAPSharedKeyServesAuthAlone(t *testing.T) {
	log := slogutil.DiscardLogger()
	ini, resp, _ := autEAPHandshake(t)

	if resp.EAPMSK == ([64]byte{}) {
		t.Fatal("the responder holds no EAP MSK after the handshake")
	}
	// RFC 7296 Section 2.16: both sides generate AUTH from the same EAP MSK.
	if ini.EAPMSK != resp.EAPMSK {
		t.Fatal("the two sides hold different EAP MSK values")
	}

	childKeys := func(what string) [][]byte {
		t.Helper()
		child, err := createFirstChildSA(resp, resp.ESPGroup,
			resp.PeerCfg.LocalAddress, resp.PeerCfg.RemoteAddress, 0, nil, log)
		if err != nil {
			t.Fatalf("createFirstChildSA (%s): %v", what, err)
		}
		return [][]byte{
			bytes.Clone(child.Keys.EncryptKeyI), bytes.Clone(child.Keys.IntegKeyI),
			bytes.Clone(child.Keys.EncryptKeyR), bytes.Clone(child.Keys.IntegKeyR),
		}
	}
	skKeys := func() [][]byte {
		k := resp.SKKeys
		return [][]byte{
			bytes.Clone(k.SK_d), bytes.Clone(k.SK_ai), bytes.Clone(k.SK_ar),
			bytes.Clone(k.SK_ei), bytes.Clone(k.SK_er),
			bytes.Clone(k.SK_pi), bytes.Clone(k.SK_pr),
		}
	}

	firstAuth, err := computeEAPAuth(resp)
	if err != nil {
		t.Fatalf("computeEAPAuth with the negotiated MSK: %v", err)
	}
	firstChild := childKeys("negotiated MSK")
	firstSK := skKeys()

	// The MSK is the only input that changes.
	var swapped [64]byte
	for i := range swapped {
		swapped[i] = resp.EAPMSK[i] ^ 0xA5
	}
	resp.EAPMSK = swapped

	secondAuth, err := computeEAPAuth(resp)
	if err != nil {
		t.Fatalf("computeEAPAuth with the swapped MSK: %v", err)
	}
	secondChild := childKeys("swapped MSK")
	secondSK := skKeys()

	// Negative half. The MSK does reach the AUTH payload, so the comparisons below are
	// made against a value the code demonstrably reads.
	if bytes.Equal(firstAuth.AuthData, secondAuth.AuthData) {
		t.Fatal("the AUTH payload did not change when the MSK changed, so the MSK is unread")
	}

	// Positive half. It reaches nothing else.
	for i := range firstSK {
		if !bytes.Equal(firstSK[i], secondSK[i]) {
			t.Errorf("SK key %d changed when the MSK changed", i)
		}
	}
	for i := range firstChild {
		if !bytes.Equal(firstChild[i], secondChild[i]) {
			t.Errorf("Child SA key %d changed when the MSK changed", i)
		}
	}
	for i := range firstSK {
		if autMSKAppears(swapped, firstSK[i]) {
			t.Errorf("SK key %d contains MSK bytes", i)
		}
	}
	for i := range firstChild {
		if autMSKAppears(swapped, firstChild[i]) {
			t.Errorf("Child SA key %d contains MSK bytes", i)
		}
	}
}
