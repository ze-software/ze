package engine

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"testing"
	"time"

	ikecrypto "github.com/ze-software/ze/internal/component/ike/crypto"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/component/pki"
)

func testSAWithKeys(t *testing.T) *SA {
	t.Helper()

	peer := testPeer()
	ikeGroup := testIKEGroup()

	sa, err := newInitiatorSA("test-peer", peer, ikeGroup, ipsec.ESPGroup{})
	if err != nil {
		t.Fatalf("newInitiatorSA: %v", err)
	}

	sa.RemoteNonce = make([]byte, 32)
	for i := range sa.RemoteNonce {
		sa.RemoteNonce[i] = byte(i)
	}
	sa.ResponderSPI = [8]byte{9, 8, 7, 6, 5, 4, 3, 2}
	sa.InitiatorSAInitMsg = make([]byte, 28)
	sa.ResponderSAInitMsg = make([]byte, 28)

	sa.Proposal = ikecrypto.IKEProposal{
		Encryption: ikecrypto.EncryptionTransform{ID: ikecrypto.ENCR_AES_CBC, KeyLength: 256},
		PRF:        ikecrypto.PRFTransform{ID: ikecrypto.PRF_HMAC_SHA2_256, KeyLength: 32, OutputLength: 32},
		Integrity:  ikecrypto.IntegrityTransform{ID: ikecrypto.AUTH_HMAC_SHA2_256_128, KeyLength: 32, TruncatedLength: 16},
		DHGroup:    ikecrypto.DHGroupTransform{ID: ikecrypto.DH_MODP_2048},
	}

	skeyseed, err := ikecrypto.DeriveSKEYSEED(
		sa.Proposal.PRF.ID,
		sa.LocalNonce, sa.RemoteNonce,
		make([]byte, 32), // dummy shared secret
	)
	if err != nil {
		t.Fatalf("DeriveSKEYSEED: %v", err)
	}

	skKeys, err := ikecrypto.DeriveSKKeys(
		sa.Proposal.PRF.ID, skeyseed,
		sa.LocalNonce, sa.RemoteNonce,
		sa.InitiatorSPI[:], sa.ResponderSPI[:],
		sa.Proposal.Encryption, sa.Proposal.Integrity,
	)
	if err != nil {
		t.Fatalf("DeriveSKKeys: %v", err)
	}
	sa.SKKeys = skKeys

	return sa
}

// TestAuthX509UsesMethod14 proves ze's certificate (signature) authentication uses RFC 7427
// AUTH method 14 (Digital Signature) -- the method used when both peers exchange
// SIGNATURE_HASH_ALGORITHMS, which ze always sends (buildSignatureHashAlgosNotify).
//
// RFC requirement: RFC7427-3-4 positive -- signature (certificate) authentication produces an
// AUTH payload with method 14 (Digital Signature), not a legacy RSA/DSA method.
func TestAuthX509UsesMethod14(t *testing.T) {
	// A CA and a device certificate signed by it (pki.Load validates the chain).
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	caTmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "rfc7427-ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTmpl, caTmpl, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	caCert, err := x509.ParseCertificate(caDER)
	if err != nil {
		t.Fatal(err)
	}

	devKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	devTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "rfc7427-dev"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}
	devDER, err := x509.CreateCertificate(rand.Reader, devTmpl, caCert, &devKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	devCert, err := x509.ParseCertificate(devDER)
	if err != nil {
		t.Fatal(err)
	}

	if err := pki.Load(&pki.PKIConfig{
		CACerts: map[string]*pki.CACertEntry{
			"rfc7427-ca": {Name: "rfc7427-ca", Certificate: caCert, Raw: caDER},
		},
		Certificates: map[string]*pki.CertificateEntry{
			"rfc7427-dev": {Name: "rfc7427-dev", Certificate: devCert, Raw: devDER, PrivateKey: devKey},
		},
	}); err != nil {
		t.Fatal(err)
	}

	sa := testSAWithKeys(t)
	sa.PeerCfg.Auth.Mode = ipsec.AuthX509
	sa.PeerCfg.Auth.Certificate = "rfc7427-dev"
	// The peer's SIGNATURE_HASH_ALGORITHMS notify. RFC 7427 Section 3 permits method 14
	// only once one has been sent and received, and the device key above is P-256.
	sa.RemoteHashAlgos = []uint16{hashAlgoSHA2256}

	auth, err := computeX509Auth(sa)
	if err != nil {
		t.Fatalf("computeX509Auth: %v", err)
	}
	if auth.AuthMethod != wire.AuthMethodDigitalSig {
		t.Fatalf("expected AUTH method 14 (Digital Signature), got %d", auth.AuthMethod)
	}
}

// TestAuthPSKCompute pins that pre-shared-key authentication emits AUTH method 2
// (Shared Key Message Integrity Code) rather than method 14.
//
// It carries NO RFC requirement tag, and the reason is worth stating because it
// held one until 2026-08-30. As `RFC7427-3-4 negative` it could not fail if that
// requirement were violated: `computePSKAuth` never reads `sa.RemoteHashAlgos`,
// and returns `wire.AuthMethodPSK` as a constant selected by
// `sa.PeerCfg.Auth.Mode`, so deleting the Section 3 gate in `computeX509Auth`
// leaves every assertion here green. RFC7427-3-4 is proven in both polarities by
// `TestRFC7427DigitalSignatureNeedsTheNotify`, which asserts the refusal itself.
func TestAuthPSKCompute(t *testing.T) {
	sa := testSAWithKeys(t)
	sa.PeerCfg.Auth.Mode = ipsec.AuthPreSharedSecret
	sa.PeerCfg.Auth.PSK = "test-secret-key"

	auth, err := computePSKAuth(sa)
	if err != nil {
		t.Fatalf("computePSKAuth: %v", err)
	}

	if auth.AuthMethod != 2 { // AuthMethodPSK
		t.Fatalf("expected auth method 2 (PSK), got %d", auth.AuthMethod)
	}
	if len(auth.AuthData) == 0 {
		t.Fatal("expected non-empty auth data")
	}
}

func TestAuthPSKVerify(t *testing.T) {
	sa := testSAWithKeys(t)
	sa.PeerCfg.Auth.Mode = ipsec.AuthPreSharedSecret
	sa.PeerCfg.Auth.PSK = "test-secret-key"

	auth, err := computePSKAuth(sa)
	if err != nil {
		t.Fatalf("computePSKAuth: %v", err)
	}

	// Verify using the same signed octets the signer used (initiator perspective).
	signedOctets, err := computeSignedOctets(sa, sa.IsInitiator)
	if err != nil {
		t.Fatalf("computeSignedOctets: %v", err)
	}

	err = verifyPSKAuth(sa, auth.AuthData, signedOctets)
	if err != nil {
		t.Fatalf("verifyPSKAuth should succeed: %v", err)
	}
}

func TestAuthPSKVerifyFailsWithWrongKey(t *testing.T) {
	sa := testSAWithKeys(t)
	sa.PeerCfg.Auth.Mode = ipsec.AuthPreSharedSecret
	sa.PeerCfg.Auth.PSK = "correct-key"

	auth, err := computePSKAuth(sa)
	if err != nil {
		t.Fatalf("computePSKAuth: %v", err)
	}

	// Verify with wrong key: recompute signed octets with the wrong PSK.
	sa.PeerCfg.Auth.PSK = "wrong-key"
	signedOctets, err := computeSignedOctets(sa, sa.IsInitiator)
	if err != nil {
		t.Fatalf("computeSignedOctets: %v", err)
	}

	err = verifyPSKAuth(sa, auth.AuthData, signedOctets)
	if err == nil {
		t.Fatal("verifyPSKAuth should fail with wrong key")
	}
}

func TestAuthPSKNoPSKConfigured(t *testing.T) {
	sa := testSAWithKeys(t)
	sa.PeerCfg.Auth.Mode = ipsec.AuthPreSharedSecret
	sa.PeerCfg.Auth.PSK = ""

	_, err := computePSKAuth(sa)
	if err == nil {
		t.Fatal("computePSKAuth should fail with empty PSK")
	}
}

func testSAWithGCMKeys(t *testing.T) *SA {
	t.Helper()

	peer := testPeer()
	ikeGroup := testIKEGroup()

	sa, err := newInitiatorSA("test-gcm-peer", peer, ikeGroup, ipsec.ESPGroup{})
	if err != nil {
		t.Fatalf("newInitiatorSA: %v", err)
	}

	sa.RemoteNonce = make([]byte, 32)
	for i := range sa.RemoteNonce {
		sa.RemoteNonce[i] = byte(i)
	}
	sa.ResponderSPI = [8]byte{9, 8, 7, 6, 5, 4, 3, 2}
	sa.InitiatorSAInitMsg = make([]byte, 28)
	sa.ResponderSAInitMsg = make([]byte, 28)

	sa.Proposal = ikecrypto.IKEProposal{
		Encryption: ikecrypto.EncryptionTransform{ID: ikecrypto.ENCR_AES_GCM_16, KeyLength: 256, IsAEAD: true},
		PRF:        ikecrypto.PRFTransform{ID: ikecrypto.PRF_HMAC_SHA2_256, KeyLength: 32, OutputLength: 32},
		Integrity:  ikecrypto.IntegrityTransform{ID: ikecrypto.AUTH_NONE},
		DHGroup:    ikecrypto.DHGroupTransform{ID: ikecrypto.DH_MODP_2048},
	}

	skeyseed, err := ikecrypto.DeriveSKEYSEED(
		sa.Proposal.PRF.ID,
		sa.LocalNonce, sa.RemoteNonce,
		make([]byte, 32),
	)
	if err != nil {
		t.Fatalf("DeriveSKEYSEED: %v", err)
	}

	skKeys, err := ikecrypto.DeriveSKKeys(
		sa.Proposal.PRF.ID, skeyseed,
		sa.LocalNonce, sa.RemoteNonce,
		sa.InitiatorSPI[:], sa.ResponderSPI[:],
		sa.Proposal.Encryption, sa.Proposal.Integrity,
	)
	if err != nil {
		t.Fatalf("DeriveSKKeys: %v", err)
	}
	sa.SKKeys = skKeys

	return sa
}

func TestBuildSKMessageAEADRoundTrip(t *testing.T) {
	sa := testSAWithGCMKeys(t)

	innerData := []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}
	firstType := uint8(0x21) // ID payload type

	msg, err := buildSKMessageAEADWithMsgID(sa, innerData, firstType, 1, wire.ExchangeIKEAuth, wire.FlagInitiator)
	if err != nil {
		t.Fatalf("buildSKMessageAEAD: %v", err)
	}

	if len(msg) < 28+4+8+len(innerData)+1+16 {
		t.Fatalf("AEAD message too short: %d", len(msg))
	}

	// Verify the SK generic header NextPayload matches firstType.
	skGHOff := 28 // after IKE header
	if msg[skGHOff] != firstType {
		t.Errorf("SK NextPayload = %d, want %d", msg[skGHOff], firstType)
	}
}

func TestBuildAuthRequestEAPOmitsAuth(t *testing.T) {
	sa := testSAWithGCMKeys(t)
	sa.ESPGroup = testESPGroup()
	sa.PeerCfg.Auth.Mode = ipsec.AuthEAPMSCHAPv2
	sa.PeerCfg.Auth.PSK = "testpassword"

	msg, err := buildAuthRequest(sa)
	if err != nil {
		t.Fatalf("buildAuthRequest: %v", err)
	}
	if len(msg) < wire.HeaderLen {
		t.Fatalf("message too short: %d", len(msg))
	}

	// The SK generic header NextPayload tells us the first inner payload type.
	// For EAP: first inner = IDi (type 35). For PSK: may start with IDi too,
	// but AUTH (type 39) is also present. We can't decrypt our own message
	// (SK_ei vs SK_er), so verify the message was built without error and
	// is a valid IKE_AUTH message.
	if msg[18] != wire.ExchangeIKEAuth {
		t.Fatalf("expected exchange type %d, got %d", wire.ExchangeIKEAuth, msg[18])
	}
}

func TestBuildAuthRequestPSKIncludesAuth(t *testing.T) {
	sa := testSAWithGCMKeys(t)
	sa.ESPGroup = testESPGroup()
	sa.PeerCfg.Auth.Mode = ipsec.AuthPreSharedSecret
	sa.PeerCfg.Auth.PSK = "test-secret"

	msg, err := buildAuthRequest(sa)
	if err != nil {
		t.Fatalf("buildAuthRequest: %v", err)
	}
	if len(msg) < wire.HeaderLen {
		t.Fatalf("message too short: %d", len(msg))
	}
	if msg[18] != wire.ExchangeIKEAuth {
		t.Fatalf("expected exchange type %d, got %d", wire.ExchangeIKEAuth, msg[18])
	}
}

func TestBuildAuthRequestEAPVsPSKSize(t *testing.T) {
	saEAP := testSAWithGCMKeys(t)
	saEAP.ESPGroup = testESPGroup()
	saEAP.PeerCfg.Auth.Mode = ipsec.AuthEAPMSCHAPv2
	saEAP.PeerCfg.Auth.PSK = "testpassword"

	saPSK := testSAWithGCMKeys(t)
	saPSK.ESPGroup = testESPGroup()
	saPSK.PeerCfg.Auth.Mode = ipsec.AuthPreSharedSecret
	saPSK.PeerCfg.Auth.PSK = "test-secret"

	eapMsg, err := buildAuthRequest(saEAP)
	if err != nil {
		t.Fatalf("EAP buildAuthRequest: %v", err)
	}
	pskMsg, err := buildAuthRequest(saPSK)
	if err != nil {
		t.Fatalf("PSK buildAuthRequest: %v", err)
	}

	// EAP message should be smaller than PSK because it omits AUTH payload.
	if len(eapMsg) >= len(pskMsg) {
		t.Fatalf("EAP message (%d bytes) should be smaller than PSK (%d bytes) due to omitted AUTH",
			len(eapMsg), len(pskMsg))
	}
}

func TestContainsHashAlgo(t *testing.T) {
	algos := []uint16{2, 3, 4}
	if !containsHashAlgo(algos, 2) {
		t.Fatal("should contain 2")
	}
	if !containsHashAlgo(algos, 4) {
		t.Fatal("should contain 4")
	}
	if containsHashAlgo(algos, 1) {
		t.Fatal("should not contain 1")
	}
	if containsHashAlgo(nil, 99) {
		t.Fatal("a peer that sent no notify offered nothing, so no algorithm is in its list")
	}
}
