// Design: docs/architecture/ike/ipsec-11-interop-eap.md -- EAP-TLS in-memory handshake harness
// RFC: rfc/short/rfc5216.md -- EAP-TLS mutual authentication, MSK derivation, cert validation
//
// This file wires the real EAP-TLS authenticator (tlsMethod via NewSession) and
// the real EAP-TLS peer (PeerSession) through their fragmenting transports and
// drives EAP-Request/EAP-Response messages between them until the embedded TLS
// handshake completes. It uses an in-memory two-CA PKI so tests can exercise a
// successful mutual-auth handshake, extract both sides' MSK, and force failures
// by presenting an untrusted client cert or an untrusted server cert.
//
// VALIDATES: EAP-TLS mutual authentication completes end to end, both sides
// derive an identical 64-octet MSK, and certificate path validation is enforced
// on the authenticator (client chain) and on the peer (server chain).
// PREVENTS: regressions where the handshake fails to complete (fragment
// shuttling / completion ordering), an unauthenticated peer is accepted, or an
// untrusted certificate chain is not rejected.

package eap

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"

	"errors"
	"math/big"
	"testing"
	"time"
)

// eapTLSPKI is an in-memory certificate authority set used by the handshake
// harness: a trusted CA that signs the server and client certificates, plus a
// second (untrusted) CA that signs the certificates used to force failures.
type eapTLSPKI struct {
	trustedCAPEM []byte

	// trustedCA and trustedCAKey issue the revocation lists RFC 9190 Section 5.4
	// requires on TLS 1.3, and trustedCRLPEM is the empty one every handshake
	// this harness expects to SUCCEED is configured with. An empty list is a
	// real answer -- "no certificate this CA issued is revoked" -- and it is what
	// a CA publishes while it has revoked nothing.
	trustedCA     *x509.Certificate
	trustedCAKey  *ecdsa.PrivateKey
	trustedCRLPEM []byte

	serverCertPEM []byte
	serverKeyPEM  []byte

	clientCertPEM []byte
	clientKeyPEM  []byte

	// Signed by a different CA that neither side trusts.
	untrustedClientCertPEM []byte
	untrustedClientKeyPEM  []byte
	untrustedServerCertPEM []byte
	untrustedServerKeyPEM  []byte
}

// newCA returns a self-signed CA certificate and its signing key, valid for an
// hour either side of now.
func newCA(t *testing.T, cn string, serial int64) (*x509.Certificate, *ecdsa.PrivateKey, []byte) {
	t.Helper()
	return newCAValidFor(t, cn, serial, time.Hour)
}

// newCAValidFor is newCA with the validity window named.
//
// A resumption test drives the authenticator's tls.Config.Time days ahead, to
// reach the RFC 9190 Section 5.7 ticket ceiling, and every certificate has to
// outlive that clock or the test would be measuring expiry instead.
func newCAValidFor(t *testing.T, cn string, serial int64, validity time.Duration) (*x509.Certificate, *ecdsa.PrivateKey, []byte) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("ca key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(serial),
		Subject:               pkix.Name{CommonName: cn},
		NotBefore:             time.Now().Add(-validity),
		NotAfter:              time.Now().Add(validity),
		IsCA:                  true,
		BasicConstraintsValid: true,
		// CRLSign is here because these CAs issue the revocation lists RFC 9190
		// Section 5.4 makes mandatory on TLS 1.3. x509.CreateRevocationList
		// refuses an issuer whose KeyUsage is set and omits it.
		KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("ca cert: %v", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("parse ca: %v", err)
	}
	return cert, key, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}

// newLeaf returns a leaf certificate + key (both PEM) signed by caCert/caKey,
// valid for an hour either side of now.
func newLeaf(t *testing.T, caCert *x509.Certificate, caKey *ecdsa.PrivateKey, cn string, serial int64, eku []x509.ExtKeyUsage) (certPEM, keyPEM []byte) {
	t.Helper()
	return newLeafValidFor(t, caCert, caKey, cn, serial, eku, time.Hour)
}

// newLeafValidFor is newLeaf with the validity window named, for the resumption
// tests that move the authenticator clock forward. See newCAValidFor.
func newLeafValidFor(t *testing.T, caCert *x509.Certificate, caKey *ecdsa.PrivateKey, cn string, serial int64, eku []x509.ExtKeyUsage, validity time.Duration) (certPEM, keyPEM []byte) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("leaf key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(serial),
		Subject:      pkix.Name{CommonName: cn},
		NotBefore:    time.Now().Add(-validity),
		NotAfter:     time.Now().Add(validity),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  eku,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, caCert, &key.PublicKey, caKey)
	if err != nil {
		t.Fatalf("leaf cert: %v", err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
		pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
}

// newCRL issues a certificate revocation list signed by caCert/caKey listing the
// serial numbers given, valid from an hour ago until an hour from now.
//
// An empty list is a valid CRL and is what a CA publishes while it has revoked
// nothing. It ANSWERS the question RFC 9190 Section 5.4 asks, which is what
// separates it from configuring no list at all: the first says "not revoked",
// the second says nothing and refuses the session (checkChainRevocation,
// revocation.go).
func newCRL(t *testing.T, caCert *x509.Certificate, caKey *ecdsa.PrivateKey, revoked ...int64) []byte {
	t.Helper()
	return newCRLValidFor(t, caCert, caKey, time.Hour, revoked...)
}

// newCRLValidFor is newCRL with the nextUpdate window named, so a list stays
// current for a test whose certificates outlive an hour.
func newCRLValidFor(t *testing.T, caCert *x509.Certificate, caKey *ecdsa.PrivateKey, validity time.Duration, revoked ...int64) []byte {
	t.Helper()
	entries := make([]x509.RevocationListEntry, 0, len(revoked))
	for _, serial := range revoked {
		entries = append(entries, x509.RevocationListEntry{
			SerialNumber:   big.NewInt(serial),
			RevocationTime: time.Now().Add(-time.Minute),
		})
	}
	der, err := x509.CreateRevocationList(rand.Reader, &x509.RevocationList{
		Number:                    big.NewInt(1),
		ThisUpdate:                time.Now().Add(-validity),
		NextUpdate:                time.Now().Add(validity),
		RevokedCertificateEntries: entries,
	}, caCert, caKey)
	if err != nil {
		t.Fatalf("crl: %v", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "X509 CRL", Bytes: der})
}

func newEAPTLSPKI(t *testing.T) *eapTLSPKI {
	t.Helper()
	return newEAPTLSPKIValidFor(t, time.Hour)
}

// newEAPTLSPKIValidFor builds the harness PKI with every certificate and
// revocation list valid for the window named, for the resumption tests that
// drive the authenticator clock days ahead of now.
func newEAPTLSPKIValidFor(t *testing.T, validity time.Duration) *eapTLSPKI {
	t.Helper()
	trustedCA, trustedKey, trustedPEM := newCAValidFor(t, "eap-tls-trusted-ca", 1, validity)
	untrustedCA, untrustedKey, _ := newCAValidFor(t, "eap-tls-untrusted-ca", 100, validity)

	p := &eapTLSPKI{
		trustedCAPEM:  trustedPEM,
		trustedCA:     trustedCA,
		trustedCAKey:  trustedKey,
		trustedCRLPEM: newCRLValidFor(t, trustedCA, trustedKey, validity),
	}
	serverAuth := []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}
	clientAuth := []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}
	p.serverCertPEM, p.serverKeyPEM = newLeafValidFor(t, trustedCA, trustedKey, "eap-tls-server", 2, serverAuth, validity)
	p.clientCertPEM, p.clientKeyPEM = newLeafValidFor(t, trustedCA, trustedKey, "eap-tls-client", 3, clientAuth, validity)
	p.untrustedClientCertPEM, p.untrustedClientKeyPEM = newLeafValidFor(t, untrustedCA, untrustedKey, "rogue-client", 4, clientAuth, validity)
	p.untrustedServerCertPEM, p.untrustedServerKeyPEM = newLeafValidFor(t, untrustedCA, untrustedKey, "rogue-server", 5, serverAuth, validity)
	return p
}

// serverConfig builds the authenticator MethodConfig from the trusted PKI.
//
// Resumption is set because newTLSMethod refuses a config without one: RFC 9190
// Section 2.1.2 makes the NewSessionTicket a MUST, and the keys behind it belong
// to the peering. A test that wants two exchanges to share a store passes it to
// serverConfigResuming instead.
func (p *eapTLSPKI) serverConfig() MethodConfig {
	return p.serverConfigResuming(NewResumption(time.Now, true))
}

// serverConfigResuming builds the authenticator MethodConfig over a named
// resumption store, so a second exchange can be driven against the ticket keys
// the first one issued under.
func (p *eapTLSPKI) serverConfigResuming(r *Resumption) MethodConfig {
	return MethodConfig{
		ServerCertPEM: p.serverCertPEM,
		ServerKeyPEM:  p.serverKeyPEM,
		CACertPEM:     p.trustedCAPEM,
		CRLPEM:        p.trustedCRLPEM,
		Resumption:    r,
	}
}

// eapTLSClientSerial and eapTLSServerSerial are the serial numbers
// newEAPTLSPKI gives the two certificates a successful exchange uses. A test
// that needs one of them revoked names it here rather than repeating the number
// (newEAPTLSPKI, above).
const (
	eapTLSServerSerial int64 = 2
	eapTLSClientSerial int64 = 3
)

// crlRevoking issues a fresh trusted-CA revocation list naming the serial
// numbers given, for a test that needs a revoked certificate.
func (p *eapTLSPKI) crlRevoking(t *testing.T, serials ...int64) []byte {
	t.Helper()
	return newCRL(t, p.trustedCA, p.trustedCAKey, serials...)
}

// hsResult captures everything observable after driving one EAP-TLS exchange.
type hsResult struct {
	server *tlsMethod
	peer   *PeerSession

	// sess is the authenticator Session the harness drove. It carries the
	// method's refusal reason, which the EAP-Failure packet cannot
	// (Session.Err, eap.go).
	sess *Session

	serverEAPSuccess bool
	serverEAPFailure bool
	serverMSK        [64]byte

	peerDone bool
	peerMSK  [64]byte
	peerErr  error

	rounds int
}

// serverState returns the authenticator's TLS ConnectionState (zero value if the
// authenticator never built a connection).
func (r *hsResult) serverState() tls.ConnectionState {
	if r.server == nil || r.server.conn == nil {
		return tls.ConnectionState{}
	}
	return r.server.conn.ConnectionState()
}

// peerState returns the peer's TLS ConnectionState.
func (r *hsResult) peerState() tls.ConnectionState {
	if r.peer == nil || r.peer.tlsConn == nil {
		return tls.ConnectionState{}
	}
	return r.peer.tlsConn.ConnectionState()
}

// runEAPTLSHandshake drives a full EAP-TLS exchange between a freshly created
// authenticator Session (from serverCfg) and the supplied peer, returning the
// observed outcome. It needs no per-round pause: each side waits for its own TLS
// engine to settle before it answers, so every round carries real data rather
// than an empty polling ACK. The peer's maxEAPRounds cap guarantees termination
// even when the handshake fails.
func runEAPTLSHandshake(t *testing.T, serverCfg MethodConfig, peer *PeerSession) *hsResult {
	t.Helper()

	sess, err := NewSession(TypeTLS, serverCfg)
	if err != nil {
		t.Fatalf("create authenticator session: %v", err)
	}
	// Release both sides' TLS engine goroutines when the test ends. Every failure
	// case this harness drives (untrusted client chain, untrusted server chain, no
	// trust anchor) leaves the handshake incomplete, and an incomplete handshake
	// parks a goroutine in eapTLSTransport.Read on each side until something
	// closes the transport. No assertion changes: this only frees what the test
	// already finished with.
	t.Cleanup(func() {
		sess.Close()
		peer.Close()
	})
	method, ok := sess.method.(*tlsMethod)
	if !ok {
		t.Fatalf("authenticator method is %T, want *tlsMethod", sess.method)
	}

	res := &hsResult{server: method, peer: peer, sess: sess}
	req := sess.Begin()

	for i := range 60 {
		res.rounds = i + 1
		pres := peer.Process(req)
		if pres.Err != nil {
			res.peerErr = pres.Err
			break
		}
		if pres.Done {
			res.peerDone = true
			res.peerMSK = pres.MSK
			break
		}
		if pres.Response == nil {
			break
		}

		next := sess.Process(pres.Response)
		if next == nil {
			break
		}
		if next.Code == CodeSuccess {
			res.serverEAPSuccess = true
			res.serverMSK = sess.MSK()
		}
		if next.Code == CodeFailure {
			res.serverEAPFailure = true
			peer.Process(next)
			break
		}
		req = next
	}
	return res
}

// TestEAPTLSMutualAuthHandshakeSucceeds drives a complete EAP-TLS mutual
// authentication handshake and asserts both endpoints authenticate each other
// and derive an identical MSK.
//
// RFC requirement: RFC5216-2.1.1-1 positive -- the authenticator sends a TLS
// CertificateRequest and the peer supplies a client certificate: after the
// handshake the authenticator's ConnectionState carries the peer's certificate
// (PeerCertificates non-empty) and the peer's ConnectionState carries the
// authenticator's certificate, i.e. mutual certificate authentication occurred.
//
// RFC requirement: RFC5216-5.3-1 positive -- with valid certificate chains on
// both sides the handshake is accepted: the authenticator path-validates the
// peer chain (RequireAndVerifyClientCert against its CA) and the peer
// path-validates the authenticator chain (VerifyPeerCertificate against its CA).
//
// RFC requirement: RFC5216-2.3-1 positive -- the MSK derived on each side with
// the RFC 5216 exporter label "client EAP encryption" is non-zero, exactly 64
// octets, and identical on the authenticator and the peer.
//
// RFC requirement: RFC5216-2.4-1 positive -- the peer negotiates a TLS version
// of at least TLS 1.0.
//
// RFC requirement: RFC5216-2.4-2 positive -- the authenticator negotiates a TLS
// version of at least TLS 1.0.
//
// RFC requirement: RFC5216-2.4-3 positive -- the completed handshake uses no TLS
// record compression: Go's crypto/tls never offers or negotiates compression
// (its ConnectionState exposes no compression method), so a completed handshake
// is uncompressed by construction.
//
// RFC requirement: RFC5216-2.4-4 positive -- the key material handed to the
// lower layer is a fixed 64-octet MSK produced by the TLS exporter, whose length
// is set by RFC 5216 and is independent of the negotiated TLS ciphersuite; the
// ciphersuite protects only the TLS handshake and is not reused as the
// lower-layer key.
func TestEAPTLSMutualAuthHandshakeSucceeds(t *testing.T) {
	pki := newEAPTLSPKI(t)
	peer := NewPeerSessionTLS("eap-tls-client", &PeerTLSConfig{
		CertPEM:   pki.clientCertPEM,
		KeyPEM:    pki.clientKeyPEM,
		CACertPEM: pki.trustedCAPEM,
		CRLPEM:    pki.trustedCRLPEM,
	})

	res := runEAPTLSHandshake(t, pki.serverConfig(), peer)
	if res.peerErr != nil {
		t.Fatalf("handshake failed: %v", res.peerErr)
	}
	if !res.serverEAPSuccess {
		t.Fatalf("authenticator did not send EAP-Success (rounds=%d)", res.rounds)
	}
	if !res.peerDone {
		t.Fatalf("peer did not complete (rounds=%d)", res.rounds)
	}

	// RFC5216-2.1.1-1: mutual certificate authentication actually happened.
	ss := res.serverState()
	if len(ss.PeerCertificates) == 0 {
		t.Fatal("authenticator holds no peer certificate: client cert was not requested/provided")
	}
	ps := res.peerState()
	if len(ps.PeerCertificates) == 0 {
		t.Fatal("peer holds no authenticator certificate")
	}

	// RFC5216-2.4-1 / RFC5216-2.4-2: TLS version at least 1.0 on both sides.
	if ps.Version < tls.VersionTLS10 {
		t.Fatalf("peer negotiated TLS version 0x%04x, want >= TLS 1.0", ps.Version)
	}
	if ss.Version < tls.VersionTLS10 {
		t.Fatalf("authenticator negotiated TLS version 0x%04x, want >= TLS 1.0", ss.Version)
	}

	// RFC5216-2.4-3: a completed Go TLS handshake carries no compression.
	if !ps.HandshakeComplete || !ss.HandshakeComplete {
		t.Fatalf("handshake not complete on both sides: peer=%v server=%v", ps.HandshakeComplete, ss.HandshakeComplete)
	}

	// RFC5216-2.3-1 / RFC5216-2.4-4: identical, non-zero, exactly 64-octet MSK.
	var zero [64]byte
	if res.peerMSK == zero {
		t.Fatal("peer MSK is all zero")
	}
	if res.serverMSK == zero {
		t.Fatal("authenticator MSK is all zero")
	}
	if res.peerMSK != res.serverMSK {
		t.Fatalf("MSK mismatch:\n peer=  %x\n server=%x", res.peerMSK, res.serverMSK)
	}
	if len(res.peerMSK) != 64 {
		t.Fatalf("MSK length %d, want 64", len(res.peerMSK))
	}
}

// TestEAPTLSAuthenticatorRequiresClientCert asserts the authenticator's real
// tls.Config (built by newTLSMethod) requires and verifies a client certificate
// against a trust anchor, so a peer that omits its certificate cannot
// authenticate.
//
// RFC requirement: RFC5216-2.1.1-1 negative -- the authenticator sets
// ClientAuth = RequireAndVerifyClientCert with a non-empty ClientCAs pool, so a
// peer that presents no certificate (or an unverifiable one) is refused; this is
// what forces mutual authentication. The behavioral counterpart -- an actually
// unacceptable client certificate being rejected mid-handshake -- is exercised
// by TestEAPTLSServerRejectsUntrustedClientChain.
func TestEAPTLSAuthenticatorRequiresClientCert(t *testing.T) {
	pki := newEAPTLSPKI(t)
	method, err := newTLSMethod(pki.serverConfig())
	if err != nil {
		t.Fatalf("newTLSMethod: %v", err)
	}
	if method.tlsConfig.ClientAuth != tls.RequireAndVerifyClientCert {
		t.Fatalf("ClientAuth = %v, want RequireAndVerifyClientCert: a peer without a valid certificate would not be rejected", method.tlsConfig.ClientAuth)
	}
	if method.tlsConfig.ClientCAs == nil {
		t.Fatal("ClientCAs is nil: no trust anchor to verify the peer certificate against")
	}
}

// TestEAPTLSServerRejectsUntrustedClientChain drives a full EAP-TLS exchange in
// which the peer presents a client certificate signed by a CA the authenticator
// does not trust.
//
// RFC requirement: RFC5216-5.3-1 negative -- the authenticator path-validates
// the peer's certificate chain and rejects a client certificate signed by an
// untrusted CA: the handshake never reaches EAP-Success and the authenticator's
// TLS handshake does not complete.
func TestEAPTLSServerRejectsUntrustedClientChain(t *testing.T) {
	pki := newEAPTLSPKI(t)
	peer := NewPeerSessionTLS("rogue-client", &PeerTLSConfig{
		CertPEM: pki.untrustedClientCertPEM,
		KeyPEM:  pki.untrustedClientKeyPEM,
		// The peer trusts the authenticator's CA, so it does not reject the server
		// first; the point of this test is the authenticator rejecting the
		// untrusted CLIENT chain.
		CACertPEM: pki.trustedCAPEM,
		CRLPEM:    pki.trustedCRLPEM,
	})

	res := runEAPTLSHandshake(t, pki.serverConfig(), peer)
	if res.serverEAPSuccess {
		t.Fatal("authenticator sent EAP-Success for a client cert signed by an untrusted CA")
	}
	if res.serverState().HandshakeComplete {
		t.Fatal("authenticator completed TLS handshake with an untrusted client chain")
	}
}

// TestEAPTLSPeerRejectsUntrustedServerChain drives a full EAP-TLS exchange in
// which the authenticator presents a server certificate signed by a CA the peer
// does not trust, while the peer is configured with a trust anchor.
//
// RFC requirement: RFC5216-5.3-1 negative -- the peer path-validates the
// authenticator's certificate chain and rejects a server certificate signed by
// an untrusted CA: the handshake never reaches EAP-Success. This also hardens
// the peer verification gate (peer.go startTLSClient): when a CA is configured
// the peer does NOT skip verification, so an untrusted authenticator is refused.
func TestEAPTLSPeerRejectsUntrustedServerChain(t *testing.T) {
	pki := newEAPTLSPKI(t)
	// Authenticator uses a server cert signed by the untrusted CA.
	serverCfg := MethodConfig{
		ServerCertPEM: pki.untrustedServerCertPEM,
		ServerKeyPEM:  pki.untrustedServerKeyPEM,
		CACertPEM:     pki.trustedCAPEM, // still verifies the (valid) client
		CRLPEM:        pki.trustedCRLPEM,
		Resumption:    NewResumption(time.Now, true),
	}
	peer := NewPeerSessionTLS("eap-tls-client", &PeerTLSConfig{
		CertPEM:   pki.clientCertPEM,
		KeyPEM:    pki.clientKeyPEM,
		CACertPEM: pki.trustedCAPEM, // peer trusts only the trusted CA
		CRLPEM:    pki.trustedCRLPEM,
	})

	res := runEAPTLSHandshake(t, serverCfg, peer)
	if res.serverEAPSuccess {
		t.Fatal("handshake succeeded despite the authenticator presenting an untrusted server cert")
	}
	if res.peerDone {
		t.Fatal("peer completed against an untrusted server certificate")
	}
}

// TestEAPTLSPeerWithoutCARefusesToStart asserts the RFC-required outcome: with
// no trust anchor the peer cannot path-validate the authenticator, so it refuses
// to start EAP-TLS rather than proceeding unauthenticated.
//
// RFC requirement: RFC5216-5.3-1 negative -- a peer that cannot perform
// certificate path validation (no trust anchor to validate against) does not
// authenticate the authenticator: startTLSClient returns an error, no
// ClientHello is sent, and the session never reaches EAP-Success.
func TestEAPTLSPeerWithoutCARefusesToStart(t *testing.T) {
	pki := newEAPTLSPKI(t)
	serverCfg := MethodConfig{
		ServerCertPEM: pki.untrustedServerCertPEM,
		ServerKeyPEM:  pki.untrustedServerKeyPEM,
		CACertPEM:     pki.trustedCAPEM,
		CRLPEM:        pki.trustedCRLPEM,
		Resumption:    NewResumption(time.Now, true),
	}
	noAnchor := func() *PeerSession {
		return NewPeerSessionTLS("eap-tls-client", &PeerTLSConfig{
			CertPEM: pki.clientCertPEM,
			KeyPEM:  pki.clientKeyPEM,
			// No CACertPEM: the peer has nothing to path-validate against.
		})
	}

	// Assert the guard directly and through the full exchange: a refactor that
	// moved the check elsewhere would still have to keep the session from
	// completing.
	peer := noAnchor()
	if err := peer.startTLSClient(); !errors.Is(err, errNoPeerTrustAnchor) {
		t.Fatalf("startTLSClient() = %v, want errNoPeerTrustAnchor", err)
	}
	if peer.tlsConn != nil {
		t.Fatal("a refused session must not have built a TLS client")
	}

	res := runEAPTLSHandshake(t, serverCfg, noAnchor())
	if res.peerDone {
		t.Fatalf("peer completed EAP-TLS with no trust anchor (rounds=%d)", res.rounds)
	}
	if res.serverEAPSuccess {
		t.Fatalf("session reached EAP-Success with no trust anchor (rounds=%d)", res.rounds)
	}
}
