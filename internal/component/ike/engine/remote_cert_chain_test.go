package engine

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/component/pki"
	"github.com/ze-software/ze/internal/core/slogutil"
)

const (
	rccCAName   = "rcc-root"
	rccCertName = "rcc-leaf"
	rccIdentity = "chain.example.com"
)

// rccPKI builds a two-level authority: a root, an intermediate the root signed, and a
// leaf the intermediate signed. The PKI store holds the ROOT as the trust anchor.
// It holds the leaf as the device certificate, with the intermediate recorded beside it
// exactly as pki config.go records one.
// That is the ordinary corporate shape, and the leaf does not chain to the anchor alone.
func rccPKI(t *testing.T) (leafDER, interDER []byte, leafKey *ecdsa.PrivateKey) {
	t.Helper()

	newKey := func(what string) *ecdsa.PrivateKey {
		key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			t.Fatalf("generate the %s key: %v", what, err)
		}
		return key
	}
	sign := func(what string, tmpl, parent *x509.Certificate, pub any, parentKey *ecdsa.PrivateKey) ([]byte, *x509.Certificate) {
		der, err := x509.CreateCertificate(rand.Reader, tmpl, parent, pub, parentKey)
		if err != nil {
			t.Fatalf("create the %s certificate: %v", what, err)
		}
		cert, err := x509.ParseCertificate(der)
		if err != nil {
			t.Fatalf("parse the %s certificate: %v", what, err)
		}
		return der, cert
	}

	rootKey := newKey("root")
	rootTmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "rcc root"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign,
	}
	rootDER, rootCert := sign("root", rootTmpl, rootTmpl, &rootKey.PublicKey, rootKey)

	interKey := newKey("intermediate")
	interTmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(2),
		Subject:               pkix.Name{CommonName: "rcc intermediate"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign,
	}
	interDER, interCert := sign("intermediate", interTmpl, rootCert, &interKey.PublicKey, rootKey)

	leafKey = newKey("leaf")
	leafTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(3),
		Subject:      pkix.Name{CommonName: rccIdentity},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		DNSNames:     []string{rccIdentity},
	}
	leafDER, leafCert := sign("leaf", leafTmpl, interCert, &leafKey.PublicKey, interKey)

	if err := pki.Load(&pki.PKIConfig{
		CACerts: map[string]*pki.CACertEntry{
			rccCAName: {Name: rccCAName, Certificate: rootCert, Raw: rootDER},
		},
		Certificates: map[string]*pki.CertificateEntry{
			rccCertName: {
				Name: rccCertName, Certificate: leafCert, Raw: leafDER, PrivateKey: leafKey,
				Intermediate: interCert, RawInter: interDER,
			},
		},
	}); err != nil {
		t.Fatalf("load the PKI store: %v", err)
	}
	t.Cleanup(func() {
		if err := pki.Load(nil); err != nil {
			t.Errorf("clear the PKI store: %v", err)
		}
	})
	return leafDER, interDER, leafKey
}

// rccAuth is the X.509 configuration both ends of the fixture share.
func rccAuth() ipsec.AuthConfig {
	return ipsec.AuthConfig{
		Mode:          ipsec.AuthX509,
		Certificate:   rccCertName,
		CACertificate: rccCAName,
		LocalID:       rccIdentity,
		RemoteID:      rccIdentity,
	}
}

// rccCerts turns DER blobs into CERT payload entries in the given order.
func rccCerts(ders ...[]byte) []wire.PayloadEntry {
	out := make([]wire.PayloadEntry, 0, len(ders))
	for _, der := range ders {
		out = append(out, wire.PayloadEntry{
			Payload: &wire.PayloadCERT{CertEncoding: wire.CertEncodingX509Sig, CertData: der},
		})
	}
	return out
}

// rccAuthRequest encodes an IKE_AUTH request whose CERT payloads appear in the given
// order, followed by IDi, AUTH, and the Child SA payloads the responder needs.
func rccAuthRequest(t *testing.T, ini *SA, ders ...[]byte) []byte {
	t.Helper()
	authPayload, err := computeLocalAuth(ini)
	if err != nil {
		t.Fatalf("compute the initiator AUTH: %v", err)
	}
	espSPI, saPayload, tsi, tsr, err := buildChildSAPayloads(ini)
	if err != nil {
		t.Fatalf("build the child SA payloads: %v", err)
	}
	ini.ChildInboundSPI = espSPI

	inner := rccCerts(ders...)
	inner = append(inner,
		wire.PayloadEntry{Payload: buildIDPayload(ini, true)},
		wire.PayloadEntry{Payload: authPayload},
		wire.PayloadEntry{Payload: saPayload},
		wire.PayloadEntry{Payload: tsi},
		wire.PayloadEntry{Payload: tsr},
	)
	raw, err := buildEncryptedMessageEx(ini, inner, 1, wire.ExchangeIKEAuth, wire.FlagInitiator)
	if err != nil {
		t.Fatalf("build the IKE_AUTH request: %v", err)
	}
	return raw
}

// rccAuthResponse encodes an IKE_AUTH response whose CERT payloads appear in the given
// order, followed by IDr and AUTH.
func rccAuthResponse(t *testing.T, resp *SA, ders ...[]byte) []byte {
	t.Helper()
	authPayload, err := computeLocalAuth(resp)
	if err != nil {
		t.Fatalf("compute the responder AUTH: %v", err)
	}
	inner := rccCerts(ders...)
	inner = append(inner,
		wire.PayloadEntry{Payload: buildIDPayload(resp, false)},
		wire.PayloadEntry{Payload: authPayload},
	)
	raw, err := buildEncryptedMessageEx(resp, inner, 1, wire.ExchangeIKEAuth, wire.FlagResponse)
	if err != nil {
		t.Fatalf("build the IKE_AUTH response: %v", err)
	}
	return raw
}

// rccResponderState drives one IKE_AUTH request through the responder and reports the
// state the SA reached.
func rccResponderState(t *testing.T, ders ...[]byte) SAState {
	t.Helper()
	log := slogutil.DiscardLogger()
	ini, resp, ps := autSAInitPair(t, rccAuth())
	raw := rccAuthRequest(t, ini, ders...)
	ps.handleAuthRequest(resp, parseMsg(t, raw), raw, nil, nil, log)
	return resp.State
}

// rccInitiatorState drives one IKE_AUTH response through the initiator and reports the
// state the SA reached.
func rccInitiatorState(t *testing.T, ders ...[]byte) SAState {
	t.Helper()
	log := slogutil.DiscardLogger()
	ini, resp, _ := autSAInitPair(t, rccAuth())
	ini.State = StateAuthSent
	raw := rccAuthResponse(t, resp, ders...)
	handleAuthResponse(ini, parseMsg(t, raw), raw, nil, nil, log)
	return ini.State
}

// VALIDATES: a peer whose leaf certificate is signed by an intermediate authenticates
// when it sends the leaf and the intermediate, on both the responder and the initiator
// path.
// PREVENTS: a two-level PKI failing to authenticate at all. getRemoteCert built
// x509.VerifyOptions with no Intermediates pool, and the anchor pool held exactly one
// certificate (pki.CACertEntry carries a single *x509.Certificate).
// A leaf signed by an intermediate had no path, so the corporate deployment hard-denied.
// RFC requirement: RFC7296-1.2-2 positive -- the receive side reads the FIRST CERT payload
// as the peer certificate and every later one as a chain-building intermediate, so a
// conformant peer sending leaf then intermediate authenticates.
// RFC requirement: RFC7296-1.2-2 negative -- the same two certificates in the reverse
// order are refused, so the pass names the first certificate rather than any certificate
// the message happened to carry.
func TestRccTwoLevelChainAuthenticates(t *testing.T) {
	leafDER, interDER, _ := rccPKI(t)

	t.Run("responder", func(t *testing.T) {
		if got := rccResponderState(t, leafDER, interDER); got != StateEstablished {
			t.Fatalf("a leaf and its intermediate left the responder at %v, want StateEstablished", got)
		}
		if got := rccResponderState(t, interDER, leafDER); got == StateEstablished {
			t.Fatal("the responder authenticated a peer whose FIRST certificate was the issuer")
		}
	})

	t.Run("initiator", func(t *testing.T) {
		if got := rccInitiatorState(t, leafDER, interDER); got != StateEstablished {
			t.Fatalf("a leaf and its intermediate left the initiator at %v, want StateEstablished", got)
		}
		if got := rccInitiatorState(t, interDER, leafDER); got == StateEstablished {
			t.Fatal("the initiator authenticated a peer whose FIRST certificate was the issuer")
		}
	})
}

// VALIDATES: a leaf that arrives with no intermediate is refused, and the refusal names
// the missing chain rather than the identity.
// PREVENTS: the untruthful remediation. A peer that sent leaf and intermediate used to
// read back "issue a certificate whose subject alternative name or common name is X",
// which was false: the certificate carried that name and the chain was the problem
// (ai/rules/error-messages.md, leg 3).
func TestRccMissingIntermediateNamesTheChain(t *testing.T) {
	leafDER, _, leafKey := rccPKI(t)

	sa := testSAWithKeys(t)
	sa.PeerCfg.Auth = rccAuth()
	sa.RemoteCertRaw = leafDER
	sa.RemoteCertChainRaw = nil
	ridAssert(sa, wire.IDTypeFQDN, []byte(rccIdentity))

	err := verifyRemoteAuth(sa, rctDigitalSigAuth(t, sa, leafKey))
	if err == nil {
		t.Fatal("a leaf with no path to the anchor authenticated")
	}
	if strings.Contains(err.Error(), "Issue a certificate") {
		t.Errorf("the refusal %q blames the identity, and the chain is what is missing", err)
	}
	if !strings.Contains(err.Error(), "validation") {
		t.Errorf("the refusal %q does not name certificate validation", err)
	}
}

// VALIDATES: Ze sends its own configured intermediate, leaf first.
// PREVENTS: the mirror of the receive-side defect. pki config.go records an intermediate
// on the certificate entry and pki store.go chains through it, and buildCertPayloads sent
// only entry.Raw. A peer anchored on the root then had no path to Ze's leaf.
func TestRccSentCertPayloadsCarryTheIntermediate(t *testing.T) {
	leafDER, interDER, _ := rccPKI(t)

	sa := testSAWithKeys(t)
	sa.PeerCfg.Auth = rccAuth()
	entries := buildCertPayloads(sa)
	if len(entries) != 2 {
		t.Fatalf("buildCertPayloads emitted %d payloads, want the leaf and its intermediate", len(entries))
	}
	got := make([][]byte, 0, 2)
	for _, e := range entries {
		cert, ok := e.Payload.(*wire.PayloadCERT)
		if !ok {
			t.Fatalf("payload %T is not a CERT payload", e.Payload)
		}
		got = append(got, cert.CertData)
	}
	if !bytes.Equal(got[0], leafDER) {
		t.Error("the first CERT payload is not the leaf, so no peer can verify AUTH from it")
	}
	if !bytes.Equal(got[1], interDER) {
		t.Error("the second CERT payload is not the configured intermediate")
	}
}
