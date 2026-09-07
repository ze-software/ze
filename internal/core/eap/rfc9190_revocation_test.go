// Design: docs/architecture/ike/ipsec-11-interop-eap.md -- EAP-TLS certificate revocation
// RFC: rfc/short/rfc9190.md -- Section 5.4, Certificate Revocation
//
// This file drives the real EAP-TLS authenticator and the real EAP-TLS peer
// through complete conversations, and asserts what RFC 9190 Section 5.4 makes
// mandatory on TLS 1.3: the revocation status of every certificate on the
// presented chains is checked, except the trust anchor.
//
// VALIDATES: a revoked leaf, a revoked intermediate, and a chain nothing can
// answer for each refuse the exchange on both roles, while an unrevoked chain
// completes and derives an MSK.
// PREVENTS: a Ze end completing an EAP-TLS 1.3 exchange against a certificate a
// CA has withdrawn, and a check that only reads the leaf.

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
	"strings"
	"testing"
	"time"
)

// revocationRounds bounds a conversation that must not complete. A refused
// EAP-TLS exchange ends within a few rounds, and the bound stops a wedged
// harness from hanging the package.
const revocationRounds = 12

// peerConfigWithCRL builds the peer side of the trusted PKI with a chosen
// revocation list, so a test can hand each role a different one and see which
// role refused.
func (p *eapTLSPKI) peerConfigWithCRL(crlPEM []byte) *PeerTLSConfig {
	return &PeerTLSConfig{
		CertPEM:   p.clientCertPEM,
		KeyPEM:    p.clientKeyPEM,
		CACertPEM: p.trustedCAPEM,
		CRLPEM:    crlPEM,
	}
}

// TestEAPTLS13RefusesARevokedClientCertificate drives a TLS 1.3 EAP-TLS
// exchange whose client certificate the trusted CA has revoked, from the
// authenticator's seat.
//
// RFC requirement: RFC9190-5.4-1 positive -- the authenticator holds a CRL that
// carries the client leaf's serial number, and the exchange reaches no
// EAP-Success. The authenticator's own diagnosis names the revocation, so the
// refusal is the revocation check and not some other failure of the same
// conversation. The peer holds a CRL that revokes nothing, so the only chain
// either end can object to is the client's.
func TestEAPTLS13RefusesARevokedClientCertificate(t *testing.T) {
	pki := newEAPTLSPKI(t)

	serverCfg := pki.serverConfig()
	serverCfg.CRLPEM = pki.crlRevoking(t, eapTLSClientSerial)

	res := runEAPTLSHandshake(t, serverCfg, NewPeerSessionTLS("eap-tls-client", pki.peerConfigWithCRL(pki.trustedCRLPEM)))

	if res.serverEAPSuccess {
		t.Fatal("the authenticator sent EAP-Success for a client certificate its CRL lists as revoked")
	}
	err := res.sess.Err()
	if err == nil {
		t.Fatal("the authenticator refused the exchange without recording a reason")
	}
	if !strings.Contains(err.Error(), "was revoked") {
		t.Fatalf("the authenticator refused for %q, which does not name the revocation", err)
	}
}

// TestEAPTLS13RefusesARevokedServerCertificate drives the same exchange from
// the peer's seat, with the authenticator's own certificate revoked.
//
// RFC requirement: RFC9190-5.4-1 positive -- Section 5.4's obligation binds the
// EAP-TLS peer as well as the server, so a peer holding a CRL that carries the
// authenticator leaf's serial number aborts its TLS handshake with an error
// naming the revocation, concludes no exchange and keeps no MSK. The
// authenticator holds a CRL that revokes nothing, so the client chain is not
// what either end objected to.
func TestEAPTLS13RefusesARevokedServerCertificate(t *testing.T) {
	pki := newEAPTLSPKI(t)

	peer := NewPeerSessionTLS("eap-tls-client", pki.peerConfigWithCRL(pki.crlRevoking(t, eapTLSServerSerial)))
	res := runEAPTLSHandshake(t, pki.serverConfig(), peer)

	if res.peerDone {
		t.Fatal("the peer concluded an exchange whose authenticator certificate its CRL lists as revoked")
	}
	// The peer's own handshake error is the reason it refused. peerDone alone
	// would also be false for a peer that never started, so the cause is read
	// where startTLSClient records it (PeerSession.tlsErr, peer.go).
	tlsErr := peer.tlsErr.Load()
	if tlsErr == nil {
		t.Fatal("the peer refused the exchange without recording a TLS handshake error")
	}
	if !strings.Contains((*tlsErr).Error(), "was revoked") {
		t.Fatalf("the peer refused for %q, which does not name the revocation", *tlsErr)
	}
	var zero [64]byte
	if res.peerMSK != zero {
		t.Fatal("the peer kept an MSK from an exchange it refused")
	}
}

// TestEAPTLS13RefusesARevokedIntermediate builds a three-deep client chain and
// revokes its INTERMEDIATE, leaving the leaf unrevoked.
//
// RFC requirement: RFC9190-5.4-1 positive -- Section 5.4 says "all the
// certificates in the certificate chains", so a check that reads only the leaf
// does not satisfy it. The authenticator holds one CRL from the root that
// carries the intermediate's serial number and one from the intermediate that
// carries nothing, and it refuses the exchange with a diagnosis naming the
// revocation.
func TestEAPTLS13RefusesARevokedIntermediate(t *testing.T) {
	pki := newEAPTLSPKI(t)
	const intermediateSerial int64 = 700
	sub, subKey, subPEM := newSubCA(t, pki.trustedCA, pki.trustedCAKey, "eap-tls-sub-ca", intermediateSerial)
	leafPEM, leafKeyPEM := newLeaf(t, sub, subKey, "eap-tls-deep-client", 701, []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth})

	// The root's list revokes the intermediate; the intermediate's list revokes
	// nothing, so the leaf itself is answerable and clean.
	crls := append(append([]byte{}, newCRL(t, pki.trustedCA, pki.trustedCAKey, intermediateSerial)...),
		newCRL(t, sub, subKey)...)

	serverCfg := pki.serverConfig()
	serverCfg.CRLPEM = crls

	peer := NewPeerSessionTLS("eap-tls-deep-client", &PeerTLSConfig{
		CertPEM:   append(append([]byte{}, leafPEM...), subPEM...),
		KeyPEM:    leafKeyPEM,
		CACertPEM: pki.trustedCAPEM,
		CRLPEM:    pki.trustedCRLPEM,
	})

	res := runEAPTLSHandshake(t, serverCfg, peer)

	if res.serverEAPSuccess {
		t.Fatal("the authenticator sent EAP-Success for a chain whose intermediate CA its CRL lists as revoked")
	}
	err := res.sess.Err()
	if err == nil {
		t.Fatal("the authenticator refused the exchange without recording a reason")
	}
	if !strings.Contains(err.Error(), "was revoked") {
		t.Fatalf("the authenticator refused for %q, which does not name the revocation", err)
	}
}

// TestEAPTLS13RefusesAnUncheckableChain drives a TLS 1.3 exchange in which the
// authenticator holds no revocation list at all.
//
// RFC requirement: RFC9190-5.4-1 positive -- the requirement is that the status
// "MUST be checked", and an end with no source performs no check. The
// authenticator therefore refuses rather than completing an exchange whose
// revocation status nobody read, and its diagnosis names Section 5.4. Every
// certificate in this exchange is valid and unrevoked, so nothing but the
// absence of a source can have refused it.
func TestEAPTLS13RefusesAnUncheckableChain(t *testing.T) {
	pki := newEAPTLSPKI(t)

	serverCfg := pki.serverConfig()
	serverCfg.CRLPEM = nil

	res := runEAPTLSHandshake(t, serverCfg, NewPeerSessionTLS("eap-tls-client", pki.peerConfigWithCRL(pki.trustedCRLPEM)))

	if res.serverEAPSuccess {
		t.Fatal("the authenticator sent EAP-Success on TLS 1.3 without checking the client chain's revocation status")
	}
	err := res.sess.Err()
	if err == nil {
		t.Fatal("the authenticator refused the exchange without recording a reason")
	}
	if !strings.Contains(err.Error(), "RFC 9190 Section 5.4") {
		t.Fatalf("the authenticator refused for %q, which does not name the obligation it enforced", err)
	}
}

// TestEAPTLS13RefusesAStaleRevocationList hands the authenticator a CRL whose
// nextUpdate has passed.
//
// RFC requirement: RFC9190-5.4-1 positive -- a list that has expired states
// nothing about the present, so the status is unchecked and the exchange is
// refused. RFC 5280 Section 5.1.2.5 defines nextUpdate as "the date by which the
// next CRL will be issued". The list is otherwise correct, signed by the right
// CA and naming no revoked serial number, so its staleness is the only thing
// this exchange can have failed on.
func TestEAPTLS13RefusesAStaleRevocationList(t *testing.T) {
	pki := newEAPTLSPKI(t)

	serverCfg := pki.serverConfig()
	serverCfg.CRLPEM = expiredCRL(t, pki.trustedCA, pki.trustedCAKey)

	res := runEAPTLSHandshake(t, serverCfg, NewPeerSessionTLS("eap-tls-client", pki.peerConfigWithCRL(pki.trustedCRLPEM)))

	if res.serverEAPSuccess {
		t.Fatal("the authenticator sent EAP-Success on the word of a revocation list that had expired")
	}
	err := res.sess.Err()
	if err == nil {
		t.Fatal("the authenticator refused the exchange without recording a reason")
	}
	if !strings.Contains(err.Error(), "no current certificate revocation list") {
		t.Fatalf("the authenticator refused for %q, which does not name the expired list", err)
	}
}

// TestEAPTLS13ExceptsTheTrustAnchorFromRevocation puts the trust anchor's OWN
// serial number on the list both ends hold, and drives the exchange anyway.
//
// RFC requirement: RFC9190-5.4-1 positive -- the requirement excepts the trust
// anchor in so many words, "(except the trust anchor)". The anchor here is
// self-signed, so a list it signed CAN name it, and an implementation that
// walked the whole chain would refuse. The exchange completes and both ends
// derive the same MSK, which is what proves the exception is applied rather
// than merely unreachable.
func TestEAPTLS13ExceptsTheTrustAnchorFromRevocation(t *testing.T) {
	pki := newEAPTLSPKI(t)

	// Serial 1 is the trusted CA's own, from newEAPTLSPKI.
	const trustAnchorSerial int64 = 1
	anchorRevoked := pki.crlRevoking(t, trustAnchorSerial)

	serverCfg := pki.serverConfig()
	serverCfg.CRLPEM = anchorRevoked

	res := runEAPTLSHandshake(t, serverCfg, NewPeerSessionTLS("eap-tls-client", pki.peerConfigWithCRL(anchorRevoked)))

	if !res.serverEAPSuccess {
		t.Fatalf("the authenticator refused an exchange whose only revoked certificate is the trust anchor: %v", res.sess.Err())
	}
	if !res.peerDone {
		t.Fatalf("the peer refused the same exchange: %v", res.peerErr)
	}
	var zero [64]byte
	if res.peerMSK == zero || res.peerMSK != res.serverMSK {
		t.Fatalf("MSK mismatch after an exchange both ends should accept:\n peer=  %x\n server=%x", res.peerMSK, res.serverMSK)
	}
}

// TestEAPTLS13CompletesWithAnUnrevokedChain is the check's negative polarity: a
// gate that refused every exchange would pass every test above.
//
// RFC requirement: RFC9190-5.4-1 negative -- both ends hold a current CRL from
// the CA that issued the other's certificate, neither list names a serial
// number on the chains presented, so the status of every certificate except the
// trust anchor is CHECKED and found good. The exchange completes on TLS 1.3 and
// both ends derive the same non-zero MSK.
func TestEAPTLS13CompletesWithAnUnrevokedChain(t *testing.T) {
	pki := newEAPTLSPKI(t)

	peer := NewPeerSessionTLS("eap-tls-client", pki.peerConfigWithCRL(pki.trustedCRLPEM))
	fl := driveEAPTLSFlight(t, pki.serverConfig(), peer, tls.VersionTLS13, revocationRounds)

	if fl.peerErr != nil {
		t.Fatalf("the peer failed an exchange both ends should accept: %v", fl.peerErr)
	}
	if fl.successAt < 0 {
		t.Fatal("no EAP-Success: an unrevoked chain was refused")
	}
	var zero [64]byte
	if fl.peerMSK == zero || fl.peerMSK != fl.serverMSK {
		t.Fatalf("MSK mismatch:\n peer=  %x\n server=%x", fl.peerMSK, fl.serverMSK)
	}
}

// TestEAPTLS12CompletesWithNoRevocationList caps the exchange at TLS 1.2 and
// configures no revocation list on either end.
//
// RFC requirement: RFC9190-5.4-1 negative -- the requirement opens "When EAP-TLS
// is used with TLS 1.3", so it does not bind a TLS 1.2 session. RFC 5216
// Section 5.4 governs that version and asks only that an implementation "MUST
// support the use of Certificate Revocation Lists (CRLs)". The exchange
// therefore completes and derives an MSK, which is what keeps the TLS 1.3
// refusal above scoped to the version the sentence names.
func TestEAPTLS12CompletesWithNoRevocationList(t *testing.T) {
	pki := newEAPTLSPKI(t)

	serverCfg := pki.serverConfig()
	serverCfg.CRLPEM = nil
	peer := NewPeerSessionTLS("eap-tls-client", pki.peerConfigWithCRL(nil))

	fl := driveEAPTLSFlight(t, serverCfg, peer, tls.VersionTLS12, revocationRounds)

	if fl.peerErr != nil {
		t.Fatalf("the peer failed a TLS 1.2 exchange Section 5.4 does not govern: %v", fl.peerErr)
	}
	if fl.successAt < 0 {
		t.Fatal("no EAP-Success: a TLS 1.2 exchange was refused for a TLS 1.3 obligation")
	}
	var zero [64]byte
	if fl.peerMSK == zero || fl.peerMSK != fl.serverMSK {
		t.Fatalf("MSK mismatch:\n peer=  %x\n server=%x", fl.peerMSK, fl.serverMSK)
	}
}

// TestEAPTLS12RefusesARevokedClientCertificate drives a TLS 1.2 EAP-TLS
// exchange whose client certificate the trusted CA has revoked.
//
// RFC requirement: RFC5216-5.4-1 positive -- RFC 5216 Section 5.4: "EAP-TLS peer
// and server implementations MUST support the use of Certificate Revocation
// Lists (CRLs)". Supporting them means acting on one, so the authenticator holds
// a list carrying the client leaf's serial number and the exchange reaches no
// EAP-Success. The obligation carries no version condition, which is what
// separates it from RFC 9190 Section 5.4 above: that sentence opens "When
// EAP-TLS is used with TLS 1.3" and this one does not.
func TestEAPTLS12RefusesARevokedClientCertificate(t *testing.T) {
	pki := newEAPTLSPKI(t)

	serverCfg := pki.serverConfig()
	serverCfg.CRLPEM = pki.crlRevoking(t, eapTLSClientSerial)
	peer := NewPeerSessionTLS("eap-tls-client", pki.peerConfigWithCRL(pki.trustedCRLPEM))

	fl := driveEAPTLSFlight(t, serverCfg, peer, tls.VersionTLS12, revocationRounds)

	if fl.successAt >= 0 {
		t.Fatal("the authenticator sent EAP-Success to a client certificate the CA had revoked")
	}
	err := fl.sess.Err()
	if err == nil {
		t.Fatal("the authenticator refused the exchange without recording a reason")
	}
	if !strings.Contains(err.Error(), "was revoked by") {
		t.Fatalf("the authenticator refused for %q, which does not name the revocation", err)
	}
}

// TestEAPTLS12CompletesWithAnUnrevokedChain is the counterpart of the test
// above: the same TLS 1.2 conversation, with a list that names nobody.
//
// RFC requirement: RFC5216-5.4-1 negative -- a gate that refused every TLS 1.2
// session carrying a list would satisfy the positive test above and support
// nothing. A list that revokes nothing is a real answer, "no certificate this CA
// issued is withdrawn", so the exchange completes and both ends derive the same
// MSK.
func TestEAPTLS12CompletesWithAnUnrevokedChain(t *testing.T) {
	pki := newEAPTLSPKI(t)

	peer := NewPeerSessionTLS("eap-tls-client", pki.peerConfigWithCRL(pki.trustedCRLPEM))

	fl := driveEAPTLSFlight(t, pki.serverConfig(), peer, tls.VersionTLS12, revocationRounds)

	if fl.peerErr != nil {
		t.Fatalf("the peer failed an exchange whose list revokes nothing: %v", fl.peerErr)
	}
	if fl.successAt < 0 {
		t.Fatal("no EAP-Success: a chain no list names was refused")
	}
	var zero [64]byte
	if fl.peerMSK == zero || fl.peerMSK != fl.serverMSK {
		t.Fatalf("MSK mismatch:\n peer=  %x\n server=%x", fl.peerMSK, fl.serverMSK)
	}
}

// TestEAPTLS12RefusesAStaleRevocationList pins the one place the two versions
// agree that reads as though they should differ.
//
// A TLS 1.2 session with NO list completes, which
// TestEAPTLS12CompletesWithNoRevocationList proves. A TLS 1.2 session with an
// EXPIRED list does NOT, because crlSet.configured answers yes for it: the
// operator asked for the check, and crlSet.currentListFrom then finds nothing
// that still speaks for now. RFC 5280 Section 5.1.2.5 defines nextUpdate as "the
// date by which the next CRL will be issued", so an expired list is not a usable
// one and treating it as an answer would report "not revoked" on the strength of
// a document that has stopped speaking.
//
// PREVENTS docs/guide/ipsec.md going back to saying a stale list establishes on
// TLS 1.2, which it said until 2026-09-07.
func TestEAPTLS12RefusesAStaleRevocationList(t *testing.T) {
	pki := newEAPTLSPKI(t)

	serverCfg := pki.serverConfig()
	serverCfg.CRLPEM = expiredCRL(t, pki.trustedCA, pki.trustedCAKey)
	peer := NewPeerSessionTLS("eap-tls-client", pki.peerConfigWithCRL(pki.trustedCRLPEM))

	fl := driveEAPTLSFlight(t, serverCfg, peer, tls.VersionTLS12, revocationRounds)

	if fl.successAt >= 0 {
		t.Fatal("the authenticator sent EAP-Success on the word of a revocation list that had expired")
	}
	err := fl.sess.Err()
	if err == nil {
		t.Fatal("the authenticator refused the exchange without recording a reason")
	}
	if !strings.Contains(err.Error(), "no current certificate revocation list") {
		t.Fatalf("the authenticator refused for %q, which does not name the expired list", err)
	}
}

// newSubCA returns an intermediate CA certificate, its key and its PEM, signed
// by the parent CA.
func newSubCA(t *testing.T, parent *x509.Certificate, parentKey *ecdsa.PrivateKey, cn string, serial int64) (*x509.Certificate, *ecdsa.PrivateKey, []byte) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("sub ca key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(serial),
		Subject:               pkix.Name{CommonName: cn},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, parent, &key.PublicKey, parentKey)
	if err != nil {
		t.Fatalf("sub ca cert: %v", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("parse sub ca: %v", err)
	}
	return cert, key, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}

// expiredCRL issues a revocation list whose nextUpdate has already passed.
func expiredCRL(t *testing.T, caCert *x509.Certificate, caKey *ecdsa.PrivateKey) []byte {
	t.Helper()
	der, err := x509.CreateRevocationList(rand.Reader, &x509.RevocationList{
		Number:     big.NewInt(1),
		ThisUpdate: time.Now().Add(-48 * time.Hour),
		NextUpdate: time.Now().Add(-24 * time.Hour),
	}, caCert, caKey)
	if err != nil {
		t.Fatalf("expired crl: %v", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: pemBlockCRL, Bytes: der})
}
