// VALIDATES: RFC 9190 Section 5.4 certificate status: the authenticator answers
// a Certificate Status Request with the OCSP response the operator configured,
// and a peer that uses Certificate Status Requests refuses any CertificateEntry
// except the trust anchor that carries no valid CertificateStatus.
// PREVENTS: an EAP-TLS 1.3 server that cannot staple at all, and a peer that asks
// for a status and then accepts a chain whatever the answer says.

package eap

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ocsp"
)

// ocspRounds bounds a conversation that must not complete. A refused EAP-TLS
// exchange ends within a few rounds, and the bound stops a wedged harness from
// hanging the package.
const ocspRounds = 12

// parseLeafPEM answers the parsed certificate of a PEM document the harness
// built, for a test that has to name the certificate an OCSP response is about.
func parseLeafPEM(t *testing.T, certPEM []byte) *x509.Certificate {
	t.Helper()
	block, _ := pem.Decode(certPEM)
	if block == nil {
		t.Fatal("certificate PEM does not decode")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("parse certificate: %v", err)
	}
	return cert
}

// newOCSPResponse signs an OCSP response about cert with the issuer's own key,
// which is the undelegated form RFC 6960 Section 4.2.2.2 names first.
//
// The window is an hour either side of now, so a response this helper builds
// speaks for the present and a test that needs a stale one says so.
func newOCSPResponse(
	t *testing.T,
	issuer *x509.Certificate,
	issuerKey *ecdsa.PrivateKey,
	cert *x509.Certificate,
	status int,
) []byte {
	t.Helper()
	return newOCSPResponseValidFrom(t, issuer, issuerKey, cert, status,
		time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
}

// newOCSPResponseValidFrom is newOCSPResponse with the response's own validity
// window named, for the tests that need one the clock has left behind or one
// dated ahead of now.
func newOCSPResponseValidFrom(
	t *testing.T,
	issuer *x509.Certificate,
	issuerKey *ecdsa.PrivateKey,
	cert *x509.Certificate,
	status int,
	thisUpdate, nextUpdate time.Time,
) []byte {
	t.Helper()
	template := ocsp.Response{
		Status:       status,
		SerialNumber: cert.SerialNumber,
		ThisUpdate:   thisUpdate,
		NextUpdate:   nextUpdate,
	}
	if status == ocsp.Revoked {
		template.RevokedAt = time.Now().Add(-time.Minute)
		template.RevocationReason = ocsp.KeyCompromise
	}
	der, err := ocsp.CreateResponse(issuer, issuer, template, issuerKey)
	if err != nil {
		t.Fatalf("create ocsp response: %v", err)
	}
	return der
}

// newDelegatedResponder issues a certificate the CA delegates OCSP answering to,
// with the extended key usage RFC 6960 Section 4.2.2.2 requires when eku says so
// and without it when it does not.
func newDelegatedResponder(
	t *testing.T,
	issuer *x509.Certificate,
	issuerKey *ecdsa.PrivateKey,
	serial int64,
	delegated bool,
) (*x509.Certificate, *ecdsa.PrivateKey) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("responder key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(serial),
		Subject:      pkix.Name{CommonName: "eap-tls-ocsp-responder"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}
	if delegated {
		tmpl.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageOCSPSigning}
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, issuer, &key.PublicKey, issuerKey)
	if err != nil {
		t.Fatalf("responder cert: %v", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("parse responder cert: %v", err)
	}
	return cert, key
}

// newResponderSignedOCSP signs an OCSP response with a responder key and embeds
// the responder's certificate, which is the delegated form.
func newResponderSignedOCSP(
	t *testing.T,
	issuer, responder *x509.Certificate,
	responderKey *ecdsa.PrivateKey,
	cert *x509.Certificate,
) []byte {
	t.Helper()
	der, err := ocsp.CreateResponse(issuer, responder, ocsp.Response{
		Status:       ocsp.Good,
		SerialNumber: cert.SerialNumber,
		ThisUpdate:   time.Now().Add(-time.Hour),
		NextUpdate:   time.Now().Add(time.Hour),
		Certificate:  responder,
	}, responderKey)
	if err != nil {
		t.Fatalf("create delegated ocsp response: %v", err)
	}
	return der
}

// peerConfigRequiringStatus builds the peer side of the trusted PKI with the
// certificate-status-request leaf on, so the exchange is one RFC 9190
// Section 5.4's peer sentence binds.
func (p *eapTLSPKI) peerConfigRequiringStatus() *PeerTLSConfig {
	cfg := p.peerConfigWithCRL(p.trustedCRLPEM)
	cfg.CertificateStatusRequest = true
	return cfg
}

// TestEAPTLS13StaplesTheConfiguredOCSPResponse drives a TLS 1.3 exchange against
// an authenticator holding an OCSP response about its own certificate.
//
// RFC requirement: RFC9190-5.4-2 positive -- the authenticator implements
// Certificate Status Requests: crypto/tls sends the peer's status_request, and
// the peer's completed ConnectionState carries back exactly the DER response the
// operator configured on the authenticator, which is the stapled
// CertificateStatus of the leaf CertificateEntry.
func TestEAPTLS13StaplesTheConfiguredOCSPResponse(t *testing.T) {
	pki := newEAPTLSPKI(t)
	serverLeaf := parseLeafPEM(t, pki.serverCertPEM)
	staple := newOCSPResponse(t, pki.trustedCA, pki.trustedCAKey, serverLeaf, ocsp.Good)

	serverCfg := pki.serverConfig()
	serverCfg.OCSPStaple = staple
	peer := NewPeerSessionTLS("eap-tls-client", pki.peerConfigWithCRL(pki.trustedCRLPEM))

	res := runEAPTLSHandshake(t, serverCfg, peer)

	if !res.peerDone {
		t.Fatalf("the peer did not complete the exchange: %v", res.peerErr)
	}
	state := res.peerState()
	if state.Version != tls.VersionTLS13 {
		t.Fatalf("negotiated TLS %#04x, and this obligation is about TLS 1.3", state.Version)
	}
	if len(state.OCSPResponse) == 0 {
		t.Fatal("the peer received no stapled OCSP response, so the authenticator answered no Certificate Status Request")
	}
	if !bytes.Equal(state.OCSPResponse, staple) {
		t.Fatalf("the peer received %d octets of stapled status and the operator configured %d",
			len(state.OCSPResponse), len(staple))
	}
}

// TestEAPTLS13StaplesNothingWhenTheCertificateCarriesNoResponse is the
// counterpart of the test above.
//
// RFC requirement: RFC9190-5.4-2 negative -- the staple is the operator's
// response and not a fixed answer: an authenticator whose certificate carries no
// ocsp-response completes the same exchange and the peer's ConnectionState
// carries no OCSPResponse, so the positive test above cannot be satisfied by a
// stack that always sends something.
func TestEAPTLS13StaplesNothingWhenTheCertificateCarriesNoResponse(t *testing.T) {
	pki := newEAPTLSPKI(t)
	peer := NewPeerSessionTLS("eap-tls-client", pki.peerConfigWithCRL(pki.trustedCRLPEM))

	res := runEAPTLSHandshake(t, pki.serverConfig(), peer)

	if !res.peerDone {
		t.Fatalf("the peer did not complete the exchange: %v", res.peerErr)
	}
	if got := res.peerState().OCSPResponse; len(got) != 0 {
		t.Fatalf("the peer received %d octets of stapled status from an authenticator configured with none", len(got))
	}
}

// TestEAPTLS13PeerRefusesAnAuthenticatorThatStaplesNothing drives a TLS 1.3
// exchange in which the peer uses Certificate Status Requests and the
// authenticator answers with no status.
//
// RFC requirement: RFC9190-5.4-3 positive -- the peer treats the leaf
// CertificateEntry as invalid and aborts: the exchange reaches no EAP-Success,
// the peer's own error names the missing CertificateStatus, and the return from
// VerifyConnection is what crypto/tls turns into the fatal bad_certificate alert
// the requirement asks for. Every certificate here is valid and no list revokes
// anything, so nothing but the absent status can have refused it.
func TestEAPTLS13PeerRefusesAnAuthenticatorThatStaplesNothing(t *testing.T) {
	pki := newEAPTLSPKI(t)
	peer := NewPeerSessionTLS("eap-tls-client", pki.peerConfigRequiringStatus())

	res := runEAPTLSHandshake(t, pki.serverConfig(), peer)

	if res.peerDone {
		t.Fatal("the peer completed an exchange whose authenticator stapled no OCSP response")
	}
	// The peer's own handshake error is the reason it refused. peerDone alone
	// would also be false for a peer that never started, so the cause is read
	// where startTLSClient records it (PeerSession.tlsErr, peer.go).
	tlsErr := peer.tlsErr.Load()
	if tlsErr == nil {
		t.Fatal("the peer refused the exchange without recording a TLS handshake error")
	}
	if !strings.Contains((*tlsErr).Error(), "stapled no OCSP response") {
		t.Fatalf("the peer refused for %q, which does not name the missing CertificateStatus", *tlsErr)
	}
}

// TestEAPTLS13PeerCompletesWithAValidStapledStatus is the counterpart of the
// test above: the same peer, against an authenticator that staples a good
// response.
//
// RFC requirement: RFC9190-5.4-3 negative -- a peer that refused every chain
// would satisfy the positive test above and interoperate with nobody. With a
// valid CertificateStatus on the leaf CertificateEntry the exchange completes,
// both ends derive the same MSK, and the trust anchor is asked for no status of
// its own: the chain here is leaf plus anchor, and only the leaf carries one.
func TestEAPTLS13PeerCompletesWithAValidStapledStatus(t *testing.T) {
	pki := newEAPTLSPKI(t)
	serverLeaf := parseLeafPEM(t, pki.serverCertPEM)

	serverCfg := pki.serverConfig()
	serverCfg.OCSPStaple = newOCSPResponse(t, pki.trustedCA, pki.trustedCAKey, serverLeaf, ocsp.Good)
	peer := NewPeerSessionTLS("eap-tls-client", pki.peerConfigRequiringStatus())

	res := runEAPTLSHandshake(t, serverCfg, peer)

	if res.peerErr != nil {
		t.Fatalf("the peer refused a chain whose leaf carries a valid CertificateStatus: %v", res.peerErr)
	}
	if !res.peerDone || !res.serverEAPSuccess {
		t.Fatal("no EAP-Success: a chain with a valid stapled status was refused")
	}
	var zero [64]byte
	if res.peerMSK == zero || res.peerMSK != res.serverMSK {
		t.Fatalf("MSK mismatch:\n peer=  %x\n server=%x", res.peerMSK, res.serverMSK)
	}
}

// TestEAPTLS13PeerRefusesARevokedStapledStatus drives the same exchange against
// an authenticator whose stapled response reports its certificate revoked.
//
// RFC requirement: RFC9190-5.4-3 positive -- a CertificateStatus that reports
// the certificate revoked is not a VALID one for the entry that carries it, so
// the peer aborts: the exchange reaches no EAP-Success and the peer's error
// names the revocation the responder reported.
func TestEAPTLS13PeerRefusesARevokedStapledStatus(t *testing.T) {
	pki := newEAPTLSPKI(t)
	serverLeaf := parseLeafPEM(t, pki.serverCertPEM)

	serverCfg := pki.serverConfig()
	serverCfg.OCSPStaple = newOCSPResponse(t, pki.trustedCA, pki.trustedCAKey, serverLeaf, ocsp.Revoked)
	peer := NewPeerSessionTLS("eap-tls-client", pki.peerConfigRequiringStatus())

	res := runEAPTLSHandshake(t, serverCfg, peer)

	if res.peerDone {
		t.Fatal("the peer completed an exchange whose authenticator stapled a revoked status")
	}
	tlsErr := peer.tlsErr.Load()
	if tlsErr == nil {
		t.Fatal("the peer refused the exchange without recording a TLS handshake error")
	}
	if !strings.Contains((*tlsErr).Error(), "was revoked at") {
		t.Fatalf("the peer refused for %q, which does not name the revocation", *tlsErr)
	}
}

// TestEAPTLS13PeerWithoutTheStatusLeafAcceptsAnAuthenticatorThatStaplesNothing
// drives the exchange of the refusal test above with the
// certificate-status-request leaf off.
//
// RFC requirement: RFC9190-5.4-3 negative -- the requirement binds a peer that
// USES Certificate Status Requests, and the leaf is what makes ze one. With it
// off the same authenticator, stapling nothing, is accepted, so the refusal
// above is the requirement rather than a peer that always demands a staple.
func TestEAPTLS13PeerWithoutTheStatusLeafAcceptsAnAuthenticatorThatStaplesNothing(t *testing.T) {
	pki := newEAPTLSPKI(t)
	peer := NewPeerSessionTLS("eap-tls-client", pki.peerConfigWithCRL(pki.trustedCRLPEM))

	res := runEAPTLSHandshake(t, pki.serverConfig(), peer)

	if res.peerErr != nil {
		t.Fatalf("the peer refused an exchange it asks no certificate status for: %v", res.peerErr)
	}
	if !res.peerDone {
		t.Fatal("no EAP-Success: an authenticator stapling nothing was refused by a peer that asked for nothing")
	}
}

// TestEAPTLS13PeerRefusesAnIntermediateItCannotReadTheStatusOf drives a TLS 1.3
// exchange whose authenticator presents a chain of three: leaf, intermediate CA
// and trust anchor.
//
// RFC requirement: RFC9190-5.4-3 positive -- the intermediate is a
// CertificateEntry that is not the trust anchor, and ze reads a CertificateStatus
// for the leaf entry alone (crypto/tls parses the extensions of no other entry),
// so ze cannot see a valid one for it and treats it as invalid. The exchange
// reaches no EAP-Success and the peer's error names the intermediate.
func TestEAPTLS13PeerRefusesAnIntermediateItCannotReadTheStatusOf(t *testing.T) {
	pki := newEAPTLSPKI(t)
	const intermediateSerial int64 = 800
	sub, subKey, subPEM := newSubCA(t, pki.trustedCA, pki.trustedCAKey, "eap-tls-status-sub-ca", intermediateSerial)
	leafPEM, leafKeyPEM := newLeaf(t, sub, subKey, "eap-tls-deep-server", 801, []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth})
	deepLeaf := parseLeafPEM(t, leafPEM)

	// The root's list and the intermediate's list both revoke nothing, so the
	// revocation walk accepts this chain and only the status rule can refuse it.
	crls := append(append([]byte{}, pki.trustedCRLPEM...), newCRL(t, sub, subKey)...)

	serverCfg := pki.serverConfig()
	serverCfg.ServerCertPEM = append(append([]byte{}, leafPEM...), subPEM...)
	serverCfg.ServerKeyPEM = leafKeyPEM
	serverCfg.CRLPEM = crls
	serverCfg.OCSPStaple = newOCSPResponse(t, sub, subKey, deepLeaf, ocsp.Good)

	peerCfg := pki.peerConfigRequiringStatus()
	peerCfg.CRLPEM = crls
	peer := NewPeerSessionTLS("eap-tls-client", peerCfg)

	res := runEAPTLSHandshake(t, serverCfg, peer)

	if res.peerDone {
		t.Fatal("the peer completed an exchange carrying an intermediate whose CertificateStatus it cannot read")
	}
	tlsErr := peer.tlsErr.Load()
	if tlsErr == nil {
		t.Fatal("the peer refused the exchange without recording a TLS handshake error")
	}
	if !strings.Contains((*tlsErr).Error(), "eap-tls-status-sub-ca") {
		t.Fatalf("the peer refused for %q, which does not name the intermediate", *tlsErr)
	}
}

// TestEAPTLS13PeerKeepsTheChainItAcceptedForTheLaterCheck asserts the peer
// publishes the verified authenticator chain after a TLS 1.3 exchange.
//
// RFC requirement: RFC9190-5.4-4 positive -- the later check needs the chain
// this session accepted, and ServerChains is where it reads it: after a
// completed exchange it answers a chain running leaf first and trust anchor
// last, whose leaf is the authenticator's certificate.
func TestEAPTLS13PeerKeepsTheChainItAcceptedForTheLaterCheck(t *testing.T) {
	pki := newEAPTLSPKI(t)
	peer := NewPeerSessionTLS("eap-tls-client", pki.peerConfigWithCRL(pki.trustedCRLPEM))

	res := runEAPTLSHandshake(t, pki.serverConfig(), peer)
	if !res.peerDone {
		t.Fatalf("the peer did not complete the exchange: %v", res.peerErr)
	}

	assertServerChain(t, peer, "eap-tls-server", "eap-tls-trusted-ca")
}

// TestEAPTLS12PeerKeepsTheChainItAcceptedForTheLaterCheck is the TLS 1.2
// counterpart.
//
// RFC requirement: RFC5216-5.4-2 positive -- RFC 5216 Section 5.4 puts no TLS
// version on post-authentication revocation checking, so the chain the later
// check runs over is published on a TLS 1.2 exchange too: after one completes,
// ServerChains answers the authenticator's chain, leaf first and trust anchor
// last.
func TestEAPTLS12PeerKeepsTheChainItAcceptedForTheLaterCheck(t *testing.T) {
	pki := newEAPTLSPKI(t)
	peer := NewPeerSessionTLS("eap-tls-client", pki.peerConfigWithCRL(pki.trustedCRLPEM))

	fl := driveEAPTLSFlight(t, pki.serverConfig(), peer, tls.VersionTLS12, ocspRounds)
	if fl.successAt < 0 {
		t.Fatalf("no EAP-Success on the TLS 1.2 exchange: %v", fl.peerErr)
	}

	assertServerChain(t, peer, "eap-tls-server", "eap-tls-trusted-ca")
}

// TestEAPTLSPeerPublishesNoChainWhenTheHandshakeRefusedIt is the negative
// counterpart of the two tests above.
//
// RFC requirement: RFC5216-5.4-2 negative -- a session that refused the
// authenticator's chain publishes none, so the later check reads no chain from a
// handshake that failed and cannot report a status about a peer ze never
// accepted.
func TestEAPTLSPeerPublishesNoChainWhenTheHandshakeRefusedIt(t *testing.T) {
	pki := newEAPTLSPKI(t)
	serverCfg := pki.serverConfig()
	serverCfg.ServerCertPEM = pki.untrustedServerCertPEM
	serverCfg.ServerKeyPEM = pki.untrustedServerKeyPEM
	peer := NewPeerSessionTLS("eap-tls-client", pki.peerConfigWithCRL(pki.trustedCRLPEM))

	res := runEAPTLSHandshake(t, serverCfg, peer)
	if res.peerDone {
		t.Fatal("the peer completed an exchange against an untrusted authenticator chain")
	}
	if chains := peer.ServerChains(); chains != nil {
		t.Fatalf("the peer published %d chains from a handshake it refused", len(chains))
	}
}

// assertServerChain checks that the peer published one chain running from the
// named leaf to the named trust anchor.
func assertServerChain(t *testing.T, peer *PeerSession, leafCN, anchorCN string) {
	t.Helper()
	chains := peer.ServerChains()
	if len(chains) == 0 {
		t.Fatal("the peer published no verified authenticator chain")
	}
	chain := chains[0]
	if len(chain) < 2 {
		t.Fatalf("the published chain holds %d certificates, want the leaf and its trust anchor", len(chain))
	}
	if chain[0].Subject.CommonName != leafCN {
		t.Fatalf("the published chain starts at %q, want the authenticator leaf %q", chain[0].Subject.CommonName, leafCN)
	}
	if chain[len(chain)-1].Subject.CommonName != anchorCN {
		t.Fatalf("the published chain ends at %q, want the trust anchor %q",
			chain[len(chain)-1].Subject.CommonName, anchorCN)
	}
}

// TestCertificateStatusRefusesAnExpiredResponse checks one response whose
// nextUpdate has passed.
//
// RFC requirement: RFC9190-5.4-3 positive -- a CertificateStatus that expired is
// not a valid one: CheckCertificateStatus refuses a response whose nextUpdate is
// behind the clock, and its message names the expiry.
func TestCertificateStatusRefusesAnExpiredResponse(t *testing.T) {
	pki := newEAPTLSPKI(t)
	serverLeaf := parseLeafPEM(t, pki.serverCertPEM)
	der := newOCSPResponseValidFrom(t, pki.trustedCA, pki.trustedCAKey, serverLeaf, ocsp.Good,
		time.Now().Add(-48*time.Hour), time.Now().Add(-24*time.Hour))

	err := CheckCertificateStatus(der, serverLeaf, pki.trustedCA, time.Now())
	if err == nil {
		t.Fatal("an OCSP response whose nextUpdate has passed was accepted")
	}
	if !strings.Contains(err.Error(), "expired at") {
		t.Fatalf("refused for %q, which does not name the expiry", err)
	}
}

// TestCertificateStatusRefusesAResponseDatedAhead checks one response whose
// thisUpdate is in the future.
//
// RFC requirement: RFC9190-5.4-3 positive -- a response dated after the clock
// says nothing about the present, so CheckCertificateStatus refuses it and names
// the date it carries.
func TestCertificateStatusRefusesAResponseDatedAhead(t *testing.T) {
	pki := newEAPTLSPKI(t)
	serverLeaf := parseLeafPEM(t, pki.serverCertPEM)
	der := newOCSPResponseValidFrom(t, pki.trustedCA, pki.trustedCAKey, serverLeaf, ocsp.Good,
		time.Now().Add(24*time.Hour), time.Now().Add(48*time.Hour))

	err := CheckCertificateStatus(der, serverLeaf, pki.trustedCA, time.Now())
	if err == nil {
		t.Fatal("an OCSP response dated in the future was accepted")
	}
	if !strings.Contains(err.Error(), "in the future") {
		t.Fatalf("refused for %q, which does not name the date", err)
	}
}

// TestCertificateStatusRefusesAResponseAboutAnotherCertificate checks a response
// the same CA signed about a different certificate.
//
// RFC requirement: RFC9190-5.4-3 positive -- a CertificateStatus is valid for
// the entry it answers about, so a response carrying the client certificate's
// serial number is refused for the server certificate even though the same CA
// signed it.
func TestCertificateStatusRefusesAResponseAboutAnotherCertificate(t *testing.T) {
	pki := newEAPTLSPKI(t)
	serverLeaf := parseLeafPEM(t, pki.serverCertPEM)
	clientLeaf := parseLeafPEM(t, pki.clientCertPEM)
	der := newOCSPResponse(t, pki.trustedCA, pki.trustedCAKey, clientLeaf, ocsp.Good)

	if err := CheckCertificateStatus(der, serverLeaf, pki.trustedCA, time.Now()); err == nil {
		t.Fatal("an OCSP response about the client certificate was accepted for the server certificate")
	}
}

// TestCertificateStatusRefusesAnUnknownStatus checks a response that reports the
// responder cannot answer.
//
// RFC requirement: RFC9190-5.4-3 positive -- "unknown" says the responder has no
// status for this certificate, which is not a statement that it is valid, so
// CheckCertificateStatus refuses it and its message says so.
func TestCertificateStatusRefusesAnUnknownStatus(t *testing.T) {
	pki := newEAPTLSPKI(t)
	serverLeaf := parseLeafPEM(t, pki.serverCertPEM)
	der := newOCSPResponse(t, pki.trustedCA, pki.trustedCAKey, serverLeaf, ocsp.Unknown)

	err := CheckCertificateStatus(der, serverLeaf, pki.trustedCA, time.Now())
	if err == nil {
		t.Fatal("an OCSP response reporting \"unknown\" was accepted")
	}
	if !strings.Contains(err.Error(), "unknown") {
		t.Fatalf("refused for %q, which does not name the unknown status", err)
	}
}

// TestCertificateStatusRefusesAnUndelegatedResponder checks a response signed by
// a certificate the CA issued but never delegated OCSP answering to.
//
// RFC requirement: RFC9190-5.4-3 positive -- RFC 6960 Section 4.2.2.2 makes the
// OCSP signing extended key usage the delegation, so a response signed by a
// sibling certificate of the same CA is refused, and the message names the
// missing usage.
func TestCertificateStatusRefusesAnUndelegatedResponder(t *testing.T) {
	pki := newEAPTLSPKI(t)
	serverLeaf := parseLeafPEM(t, pki.serverCertPEM)
	responder, responderKey := newDelegatedResponder(t, pki.trustedCA, pki.trustedCAKey, 900, false)
	der := newResponderSignedOCSP(t, pki.trustedCA, responder, responderKey, serverLeaf)

	err := CheckCertificateStatus(der, serverLeaf, pki.trustedCA, time.Now())
	if err == nil {
		t.Fatal("an OCSP response signed by an undelegated certificate was accepted")
	}
	if !strings.Contains(err.Error(), "did not delegate to") {
		t.Fatalf("refused for %q, which does not name the delegation", err)
	}
}

// TestCertificateStatusAcceptsADelegatedResponder is the counterpart of the test
// above.
//
// RFC requirement: RFC9190-5.4-3 negative -- a check that refused every embedded
// responder certificate would satisfy the positive test above and reject the
// delegated responders RFC 6960 Section 4.2.2.2 defines. The same response,
// signed by a certificate carrying the OCSP signing extended key usage, is
// accepted.
func TestCertificateStatusAcceptsADelegatedResponder(t *testing.T) {
	pki := newEAPTLSPKI(t)
	serverLeaf := parseLeafPEM(t, pki.serverCertPEM)
	responder, responderKey := newDelegatedResponder(t, pki.trustedCA, pki.trustedCAKey, 901, true)
	der := newResponderSignedOCSP(t, pki.trustedCA, responder, responderKey, serverLeaf)

	if err := CheckCertificateStatus(der, serverLeaf, pki.trustedCA, time.Now()); err != nil {
		t.Fatalf("a response signed by a delegated responder was refused: %v", err)
	}
}

// TestStapledChainStatusExceptsTheTrustAnchor checks the chain walk over a trust
// anchor on its own.
//
// RFC requirement: RFC9190-5.4-3 negative -- Section 5.4's rule excepts the
// trust anchor, so a chain holding nothing else needs no CertificateStatus at
// all and the walk accepts it with no staple in hand. Without the exception this
// chain would be refused for the anchor's missing status.
func TestStapledChainStatusExceptsTheTrustAnchor(t *testing.T) {
	pki := newEAPTLSPKI(t)

	chains := [][]*x509.Certificate{{pki.trustedCA}}
	if err := checkStapledChainStatus(chains, nil, time.Now()); err != nil {
		t.Fatalf("a chain holding only the trust anchor was refused: %v", err)
	}
}

// TestStapledChainStatusRefusesAnEmptyChainSet checks the walk against no chain.
//
// RFC requirement: RFC9190-5.4-3 positive -- the chain is what the rule is
// about, so an empty set is a failure to check rather than a clean answer, and
// the walk refuses it.
func TestStapledChainStatusRefusesAnEmptyChainSet(t *testing.T) {
	if err := checkStapledChainStatus(nil, nil, time.Now()); err == nil {
		t.Fatal("an empty chain set was accepted as a checked certificate status")
	}
}
