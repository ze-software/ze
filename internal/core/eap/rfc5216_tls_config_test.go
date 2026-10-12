// VALIDATES: what each EAP-TLS role installs into the tls.Config that crypto/tls
// drives the RFC 5216 Section 2.1.1 and 2.1.2 handshake flights from (owner ruling
// 2026-08-31: a requirement met through a lower layer still carries a test asserting
// what Ze installs). The authenticator installs its certificate, demands and verifies
// the peer's, holds session ticket keys it can redeem, and installs its own
// UnwrapSession (redeemTLS13Only, which matches no TLS 1.2 session); the peer installs its
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
// RFC requirement: RFC5216-2.1.1-5 positive -- the authenticator installs its own UnwrapSession when resumption is on, so crypto/tls's default ticket decryption, which would match a TLS 1.2 session, is not the one that runs.
// RFC requirement: RFC5216-2.1.1-6 positive -- the authenticator installs exactly one certificate, the server_certificate of every full handshake.
// RFC requirement: RFC5216-2.1.1-9 positive -- the authenticator demands and verifies the peer certificate, and the peer installs exactly one certificate to send in that flight.
// RFC requirement: RFC5216-2.1.1-10 positive -- the peer installs a session cache when resumption is on, which is what lets it offer a resumed session and send only the resumed flight.
// RFC requirement: RFC5216-2.1.2-1 positive -- the authenticator installs its own UnwrapSession when resumption is on, the hook through which it refuses a TLS 1.2 session; the flight it then sends is asserted by TestRFC5216AuthenticatorAnswersATLS12TicketWithAFullHandshake.
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
	if server.UnwrapSession == nil {
		t.Error("authenticator installs no UnwrapSession while resumption is on, so crypto/tls would resume a TLS 1.2 session")
	}
	if server.MinVersion != tls.VersionTLS12 {
		t.Errorf("authenticator MinVersion %s, want TLS 1.2", tls.VersionName(server.MinVersion))
	}

	peer := NewPeerSessionTLS("eap-tls-client", pki.resumptionPeerConfig(NewResumption(time.Now, true)))
	t.Cleanup(peer.Close)
	cfg := peerTLSConfigFor(t, peer)
	// The peer answers every certificate_request through GetClientCertificate,
	// never through Certificates: crypto/tls picks from Certificates only a
	// certificate matching the request, and sends an empty list otherwise.
	if len(cfg.Certificates) != 0 {
		t.Errorf("peer installs %d certificates in Certificates, which crypto/tls withholds from a non-matching certificate_request", len(cfg.Certificates))
	}
	if cfg.GetClientCertificate == nil {
		t.Fatal("peer installs no GetClientCertificate, so it has no certificate to send")
	}
	answer, err := cfg.GetClientCertificate(&tls.CertificateRequestInfo{})
	if err != nil {
		t.Fatalf("peer GetClientCertificate: %v", err)
	}
	if len(answer.Certificate) != 1 {
		t.Errorf("peer answers a certificate_request with %d certificates, want 1", len(answer.Certificate))
	}
	if cfg.ClientSessionCache == nil {
		t.Error("peer installs no session cache with resumption on, so it could never offer a resumed session")
	}
	if cfg.VerifyPeerCertificate == nil {
		t.Error("peer installs no chain check")
	}
}

// RFC requirement: RFC5216-2.1.1-4 negative -- an authenticator with no resumption state builds no tls.Config, so it never chooses a sessionId it could not redeem.
// RFC requirement: RFC5216-2.1.1-5 negative -- an authenticator whose operator turned resumption off answers no session from its UnwrapSession, at TLS 1.2 and at TLS 1.3, for a ticket encrypted under its own ticket keys from a session a real exchange established; a resumption-on authenticator redeems the same session at TLS 1.3, so the ticket is one a matching authenticator would resume.
// RFC requirement: RFC5216-2.1.1-6 negative -- an authenticator whose certificate does not parse builds no tls.Config, so no full handshake without a server_certificate can start.
// RFC requirement: RFC5216-2.1.1-9 negative -- a peer with resumption off installs no session cache, so it never sends the shorter resumed flight in place of the full one.
// RFC requirement: RFC5216-2.1.1-10 negative -- a peer with resumption off installs no session cache, so it never claims a resumption the authenticator did not indicate.
// RFC requirement: RFC5216-2.1.2-1 negative -- for a ticket encrypted under its own ticket keys, a resumption-off authenticator's UnwrapSession answers no session at TLS 1.2 and at TLS 1.3, and a resumption-on authenticator's answers no session at TLS 1.2 while it redeems the same session at TLS 1.3.
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

	state := establishedSessionState(t, pki)

	off := authenticatorTLSConfigForTest(t, pki.serverConfigResuming(NewResumption(time.Now, false)))
	for _, version := range []uint16{tls.VersionTLS12, tls.VersionTLS13} {
		if unwrapOwnTicket(t, off, state, version) != nil {
			t.Errorf("an authenticator with resumption off redeemed a %s ticket under its own keys, so it would resume", tls.VersionName(version))
		}
	}

	on := authenticatorTLSConfigForTest(t, pki.serverConfigResuming(NewResumption(time.Now, true)))
	if unwrapOwnTicket(t, on, state, tls.VersionTLS12) != nil {
		t.Error("an authenticator with resumption on redeemed a ticket at TLS 1.2, where crypto/tls follows the resumed server_hello with a NewSessionTicket")
	}
	if unwrapOwnTicket(t, on, state, tls.VersionTLS13) == nil {
		t.Error("an authenticator with resumption on did not redeem its own TLS 1.3 ticket, so the refusals above prove nothing")
	}

	peer := NewPeerSessionTLS("eap-tls-client", pki.resumptionPeerConfig(nil))
	t.Cleanup(peer.Close)
	if cfg := peerTLSConfigFor(t, peer); cfg.ClientSessionCache != nil {
		t.Error("a peer with resumption off installs a session cache")
	}
}

// establishedSessionState answers the session state of one completed EAP-TLS
// 1.3 exchange, read back out of the ticket the peer stored, so every ticket
// the test encrypts carries a session crypto/tls really established.
func establishedSessionState(t *testing.T, pki *eapTLSPKI) *tls.SessionState {
	t.Helper()
	peerStore := NewResumption(time.Now, true)
	fl, peer := driveResumingExchange(t, pki, NewResumption(time.Now, true), peerStore)
	t.Cleanup(peer.Close)
	requireCompleted(t, fl, "the exchange that establishes the session")
	_, state, err := cachedTicket(t, peerStore, "the established session").ResumptionState()
	if err != nil {
		t.Fatalf("read the stored session back: %v", err)
	}
	if state == nil {
		t.Fatal("the peer stored a ticket with no session state")
	}
	return state
}

// unwrapOwnTicket encrypts state under cfg's own ticket keys and answers what
// cfg's UnwrapSession makes of that ticket on a connection at version.
func unwrapOwnTicket(t *testing.T, cfg *tls.Config, state *tls.SessionState, version uint16) *tls.SessionState {
	t.Helper()
	if cfg.UnwrapSession == nil {
		t.Fatal("the authenticator installs no UnwrapSession, so crypto/tls's default ticket decryption runs")
	}
	cs := tls.ConnectionState{Version: version}
	ticket, err := cfg.EncryptTicket(cs, state)
	if err != nil {
		t.Fatalf("encrypt a %s ticket under the authenticator's keys: %v", tls.VersionName(version), err)
	}
	session, err := cfg.UnwrapSession(ticket, cs)
	if err != nil {
		t.Fatalf("UnwrapSession at %s: %v", tls.VersionName(version), err)
	}
	return session
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
	peer.tlsCertificate = cert
	return peer.tlsClientConfig(roots, &serverChainCheck{roots: roots})
}
