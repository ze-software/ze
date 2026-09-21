// VALIDATES: what each EAP-TLS role installs into the tls.Config that crypto/tls
// drives the RFC 5216 Section 2.1.1 and 2.1.2 handshake flights from (owner ruling
// 2026-08-31: a requirement met through a lower layer still carries a test asserting
// what Ze installs). The authenticator installs its certificate, demands and verifies
// the peer's, and holds session ticket keys it can redeem; the peer installs its
// certificate, its chain check and, when resumption is on, the session cache that
// lets it offer a resumed session.
// PREVENTS: a role that would start a flight it cannot complete: an authenticator
// with no certificate or with ticket keys it cannot redeem, or a peer whose resumed
// flight rests on a cache it does not hold.

package eap

import (
	"crypto/tls"
	"crypto/x509"
	"testing"
	"time"
)

func authenticatorTLSConfigForTest(t *testing.T, cfg MethodConfig) *tls.Config {
	t.Helper()
	sess, err := NewSession(TypeTLS, cfg)
	if err != nil {
		t.Fatalf("create the authenticator session: %v", err)
	}
	t.Cleanup(sess.Close)
	method, ok := sess.method.(*tlsMethod)
	if !ok {
		t.Fatalf("authenticator method is %T, want *tlsMethod", sess.method)
	}
	return method.tlsConfig
}

// RFC requirement: RFC5216-2.1.1-4 positive -- the authenticator installs redeemable session ticket keys and leaves session tickets enabled, so a new session can be established under an identity it chose.
// RFC requirement: RFC5216-2.1.1-5 positive -- the authenticator installs no UnwrapSession refusal when resumption is on, so a matching session is redeemed under the ciphersuite crypto/tls recorded in the ticket.
// RFC requirement: RFC5216-2.1.1-6 positive -- the authenticator installs exactly one certificate, the server_certificate of every full handshake.
// RFC requirement: RFC5216-2.1.1-9 positive -- the authenticator demands and verifies the peer certificate, and the peer installs exactly one certificate to send in that flight.
// RFC requirement: RFC5216-2.1.1-10 positive -- the peer installs a session cache when resumption is on, which is what lets it offer a resumed session and send only the resumed flight.
// RFC requirement: RFC5216-2.1.2-1 positive -- the authenticator installs the ticket keys and no refusal, so a resumed session is answered with the resumed flight.
func TestRFC5216TLSConfigBothRolesInstallWhatTheFlightsCarry(t *testing.T) {
	pki := newEAPTLSPKI(t)

	server := authenticatorTLSConfigForTest(t, pki.serverConfig())
	if len(server.Certificates) != 1 {
		t.Errorf("authenticator installs %d certificates, want 1", len(server.Certificates))
	}
	if server.ClientAuth != tls.RequireAndVerifyClientCert {
		t.Errorf("authenticator ClientAuth %v, want RequireAndVerifyClientCert", server.ClientAuth)
	}
	if server.SessionTicketsDisabled {
		t.Error("authenticator disabled session tickets, so no session it establishes could be resumed")
	}
	if server.UnwrapSession != nil {
		t.Error("authenticator installs a resumption refusal while resumption is on")
	}
	if server.MinVersion != tls.VersionTLS12 {
		t.Errorf("authenticator MinVersion %s, want TLS 1.2", tls.VersionName(server.MinVersion))
	}

	peer := NewPeerSessionTLS("eap-tls-client", pki.resumptionPeerConfig(NewResumption(time.Now, true)))
	t.Cleanup(peer.Close)
	cfg := peerTLSConfigFor(t, peer)
	if len(cfg.Certificates) != 1 {
		t.Errorf("peer installs %d certificates, want 1", len(cfg.Certificates))
	}
	if cfg.ClientSessionCache == nil {
		t.Error("peer installs no session cache with resumption on, so it could never offer a resumed session")
	}
	if cfg.VerifyPeerCertificate == nil {
		t.Error("peer installs no chain check")
	}
}

// RFC requirement: RFC5216-2.1.1-4 negative -- an authenticator with no resumption state builds no tls.Config, so it never chooses a sessionId it could not redeem.
// RFC requirement: RFC5216-2.1.1-5 negative -- an authenticator whose operator turned resumption off installs an UnwrapSession refusal, so no session is ever matched and the ciphersuite comparison never has a wrong session to pass.
// RFC requirement: RFC5216-2.1.1-6 negative -- an authenticator whose certificate does not parse builds no tls.Config, so no full handshake without a server_certificate can start.
// RFC requirement: RFC5216-2.1.1-9 negative -- a peer with resumption off installs no session cache, so it never sends the shorter resumed flight in place of the full one.
// RFC requirement: RFC5216-2.1.1-10 negative -- a peer with resumption off installs no session cache, so it never claims a resumption the authenticator did not indicate.
// RFC requirement: RFC5216-2.1.2-1 negative -- an authenticator with resumption off installs the refusal, so it never sends the resumed flight.
func TestRFC5216TLSConfigRefusesWhatWouldBreakTheFlights(t *testing.T) {
	pki := newEAPTLSPKI(t)

	noResumption := pki.serverConfigResuming(nil)
	if _, err := NewSession(TypeTLS, noResumption); err == nil {
		t.Fatal("an authenticator with no resumption state built a tls.Config")
	}

	badCert := pki.serverConfig()
	badCert.ServerCertPEM = []byte("not a certificate")
	if _, err := NewSession(TypeTLS, badCert); err == nil {
		t.Fatal("an authenticator with no parsable certificate built a tls.Config")
	}

	off := authenticatorTLSConfigForTest(t, pki.serverConfigResuming(NewResumption(time.Now, false)))
	if off.UnwrapSession == nil {
		t.Error("an authenticator with resumption off installs no refusal, so it would resume")
	}

	peer := NewPeerSessionTLS("eap-tls-client", pki.resumptionPeerConfig(nil))
	t.Cleanup(peer.Close)
	if cfg := peerTLSConfigFor(t, peer); cfg.ClientSessionCache != nil {
		t.Error("a peer with resumption off installs a session cache")
	}
}

// peerTLSConfigFor builds the peer role's tls.Config for one PeerSession, the way
// peerTLSConfigForTest (rfc9190_version_cap_test.go) does for the default peer.
func peerTLSConfigFor(t *testing.T, peer *PeerSession) *tls.Config {
	t.Helper()
	cert, err := tls.X509KeyPair(peer.tlsCfg.CertPEM, peer.tlsCfg.KeyPEM)
	if err != nil {
		t.Fatalf("load the peer certificate: %v", err)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(peer.tlsCfg.CACertPEM) {
		t.Fatal("the harness trust anchor did not parse")
	}
	return peer.tlsClientConfig(cert, roots, &serverChainCheck{roots: roots})
}
