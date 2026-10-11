// Design: docs/architecture/ike/ipsec-14-responder.md -- the authenticator's EAP-TLS method
// RFC: rfc/short/rfc9190.md -- anonymous NAI support and cached authorization data (Sections 2.1.8, 5.7)
//
// VALIDATES: the authenticator decides on the certificate and never on the
// anonymous NAI, so an anonymous peer whose certificate is revoked is refused;
// and the chain cached from the initial full handshake is what a resumed
// session is authorized on when that chain is still good.
// PREVENTS: an authenticator that admits a peer because its identity looks
// acceptable, and a resumption that authorizes on nothing because the initial
// handshake cached no chain.

package eap

import (
	"crypto/tls"
	"testing"
	"time"
)

// TestEAPTLS13AuthenticatorRefusesAnAnonymousPeerWithARevokedCertificate sends
// the anonymous NAI the positive test accepts, from a peer whose client
// certificate the authenticator's revocation list names.
//
// RFC requirement: RFC9190-2.1.8-1 negative -- "EAP-TLS peer and server
// implementations supporting TLS 1.3 MUST support anonymous Network Access
// Identifiers (NAIs)". Supporting the anonymous NAI means taking nothing from it:
// the authenticator records "@example.com" and still refuses, because the
// certificate is what authenticates, so the acceptance the positive test shows
// is not an authenticator that admits any peer with a well-formed identity.
func TestEAPTLS13AuthenticatorRefusesAnAnonymousPeerWithARevokedCertificate(t *testing.T) {
	pki := newEAPTLSPKI(t)
	cfg := pki.serverConfig()
	cfg.CRLPEM = pki.crlRevoking(t, eapTLSClientSerial)
	peer := NewPeerSessionTLS("unused", pki.peerConfigWithCRL(pki.trustedCRLPEM))
	peer.identity = "@example.com"

	fl := driveEAPTLSFlight(t, cfg, peer, tls.VersionTLS13, naiExchangeRounds)

	if got := fl.sess.Identity(); got != "@example.com" {
		t.Fatalf("the authenticator recorded identity %q, want the anonymous NAI it was sent", got)
	}
	if fl.sess.Succeeded() {
		t.Fatal("the authenticator concluded with success for a peer whose certificate it revoked")
	}
	if fl.peerDone && fl.peerMSK == fl.serverMSK {
		t.Fatal("the peer and the authenticator share an MSK although the certificate was revoked")
	}
}

// TestEAPTLS13ResumptionAuthorizesOnTheCachedChain drives a full and a resumed
// exchange while the client certificate stays good, and reads the chain the
// authenticator holds for the resumed session.
//
// RFC requirement: RFC9190-5.7-3 negative -- "The EAP-TLS server or EAP client
// MUST cache data during the initial full handshake sufficient to allow
// authorization decisions to be made during resumption". The positive test
// shows the cached chain refusing a revoked client; here the same cached chain
// grants a good one: the resumed session sees no certificate on the wire, and
// the authenticator still holds the client leaf the full handshake verified.
func TestEAPTLS13ResumptionAuthorizesOnTheCachedChain(t *testing.T) {
	pki := newEAPTLSPKI(t)
	server := NewResumption(time.Now, true)
	peerStore := NewResumption(time.Now, true)

	first, _ := driveResumingExchange(t, pki, server, peerStore)
	requireCompleted(t, first, "the initial authentication")

	second, secondPeer := driveResumingExchange(t, pki, server, peerStore)
	requireCompleted(t, second, "the resumed authentication")
	if !secondPeer.Resumed() || !second.sess.Resumed() {
		t.Fatal("the second exchange did not resume, so it cannot show what the cache authorizes")
	}
	if !second.sess.Succeeded() {
		t.Fatal("the authenticator did not authorize the resumed session on its cached chain")
	}
	if got := serverPeerLeafName(t, second); got != "eap-tls-client" {
		t.Fatalf("the resumed session holds %q, not the client the full handshake cached", got)
	}
}
