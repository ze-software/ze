// Design: plan/learned/805-ipsec-11-interop-eap.md -- EAP-TLS in-memory handshake harness
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
	"math/big"
	"testing"
	"time"
)

// eapTLSPKI is an in-memory certificate authority set used by the handshake
// harness: a trusted CA that signs the server and client certificates, plus a
// second (untrusted) CA that signs the certificates used to force failures.
type eapTLSPKI struct {
	trustedCAPEM []byte

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

// newCA returns a self-signed CA certificate and its signing key.
func newCA(t *testing.T, cn string, serial int64) (*x509.Certificate, *ecdsa.PrivateKey, []byte) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("ca key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(serial),
		Subject:               pkix.Name{CommonName: cn},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign,
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

// newLeaf returns a leaf certificate + key (both PEM) signed by caCert/caKey.
func newLeaf(t *testing.T, caCert *x509.Certificate, caKey *ecdsa.PrivateKey, cn string, serial int64, eku []x509.ExtKeyUsage) (certPEM, keyPEM []byte) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("leaf key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(serial),
		Subject:      pkix.Name{CommonName: cn},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
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

func newEAPTLSPKI(t *testing.T) *eapTLSPKI {
	t.Helper()
	trustedCA, trustedKey, trustedPEM := newCA(t, "eap-tls-trusted-ca", 1)
	untrustedCA, untrustedKey, _ := newCA(t, "eap-tls-untrusted-ca", 100)

	p := &eapTLSPKI{trustedCAPEM: trustedPEM}
	p.serverCertPEM, p.serverKeyPEM = newLeaf(t, trustedCA, trustedKey, "eap-tls-server", 2, []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth})
	p.clientCertPEM, p.clientKeyPEM = newLeaf(t, trustedCA, trustedKey, "eap-tls-client", 3, []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth})
	p.untrustedClientCertPEM, p.untrustedClientKeyPEM = newLeaf(t, untrustedCA, untrustedKey, "rogue-client", 4, []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth})
	p.untrustedServerCertPEM, p.untrustedServerKeyPEM = newLeaf(t, untrustedCA, untrustedKey, "rogue-server", 5, []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth})
	return p
}

// serverConfig builds the authenticator MethodConfig from the trusted PKI.
func (p *eapTLSPKI) serverConfig() MethodConfig {
	return MethodConfig{
		ServerCertPEM: p.serverCertPEM,
		ServerKeyPEM:  p.serverKeyPEM,
		CACertPEM:     p.trustedCAPEM,
	}
}

// hsResult captures everything observable after driving one EAP-TLS exchange.
type hsResult struct {
	server *tlsMethod
	peer   *PeerSession

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
// observed outcome. A short per-round pause lets the two background TLS
// goroutines produce their output between EAP rounds; the peer's own
// maxEAPRounds cap guarantees termination even when the handshake fails.
func runEAPTLSHandshake(t *testing.T, serverCfg MethodConfig, peer *PeerSession) *hsResult {
	t.Helper()

	sess, err := NewSession(TypeTLS, serverCfg)
	if err != nil {
		t.Fatalf("create authenticator session: %v", err)
	}
	method, ok := sess.method.(*tlsMethod)
	if !ok {
		t.Fatalf("authenticator method is %T, want *tlsMethod", sess.method)
	}

	res := &hsResult{server: method, peer: peer}
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

		// Give the authenticator and peer TLS handshake goroutines time to
		// consume the freshly delivered flight and emit the next one, so the
		// exchange advances on real data instead of empty polling ACKs.
		time.Sleep(time.Millisecond)
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
		// No CA on the peer so it does not reject the server first; the point of
		// this test is the authenticator rejecting the untrusted client chain.
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
	}
	peer := NewPeerSessionTLS("eap-tls-client", &PeerTLSConfig{
		CertPEM:   pki.clientCertPEM,
		KeyPEM:    pki.clientKeyPEM,
		CACertPEM: pki.trustedCAPEM, // peer trusts only the trusted CA
	})

	res := runEAPTLSHandshake(t, serverCfg, peer)
	if res.serverEAPSuccess {
		t.Fatal("handshake succeeded despite the authenticator presenting an untrusted server cert")
	}
	if res.peerDone {
		t.Fatal("peer completed against an untrusted server certificate")
	}
}

// TestEAPTLSPeerWithoutCASkipsServerValidation complements the peer verification
// gate: with no trust anchor configured the peer performs no server-certificate
// validation, so a handshake against an untrusted authenticator certificate
// still completes. Together with TestEAPTLSPeerRejectsUntrustedServerChain this
// shows server-certificate validation is gated on the presence of a configured
// CA (the intent of the peer.go InsecureSkipVerify/VerifyPeerCertificate logic).
//
// RFC requirement: RFC5216-5.3-1 positive -- server-side certificate path
// validation still accepts the peer's valid chain here (authenticator reaches
// EAP-Success), independent of the peer's own server-validation policy.
func TestEAPTLSPeerWithoutCASkipsServerValidation(t *testing.T) {
	pki := newEAPTLSPKI(t)
	serverCfg := MethodConfig{
		ServerCertPEM: pki.untrustedServerCertPEM,
		ServerKeyPEM:  pki.untrustedServerKeyPEM,
		CACertPEM:     pki.trustedCAPEM,
	}
	peer := NewPeerSessionTLS("eap-tls-client", &PeerTLSConfig{
		CertPEM: pki.clientCertPEM,
		KeyPEM:  pki.clientKeyPEM,
		// No CACertPEM: peer does not validate the server chain.
	})

	res := runEAPTLSHandshake(t, serverCfg, peer)
	if res.peerErr != nil {
		t.Fatalf("handshake failed: %v", res.peerErr)
	}
	if !res.serverEAPSuccess {
		t.Fatalf("authenticator did not accept the valid client chain (rounds=%d)", res.rounds)
	}
	if !res.peerDone {
		t.Fatalf("peer did not complete without a configured CA (rounds=%d)", res.rounds)
	}
}
