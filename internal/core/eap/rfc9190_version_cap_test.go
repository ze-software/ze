// Design: docs/architecture/ike/ipsec-9-ikev2-eap-nat.md -- EAP-TLS method handler
// RFC: rfc/short/rfc9190.md -- EAP-TLS 1.3, Section 1 version ceiling

// RFC 9190 Section 1 puts a ceiling on the TLS version EAP-TLS runs over,
// because EAP-TLS couples two state machines and a version nobody integrated
// into the EAP half is a failure or a security issue rather than an upgrade.
//
// VALIDATES: both EAP-TLS roles name that ceiling in the tls.Config they build,
// and the ceiling leaves TLS 1.2 reachable underneath it.
// PREVENTS: the ceiling being whatever crypto/tls happens to top out at. Until
// 2026-09-08 newTLSMethod (eap_tls.go) and PeerSession.tlsClientConfig (peer.go)
// each set MinVersion and neither set MaxVersion, so a toolchain that added a
// version above 1.3 would have had ze negotiate it with no edit and no log line.

package eap

import (
	"crypto/tls"
	"crypto/x509"
	"testing"
)

// peerTLSConfigForTest builds the peer role's tls.Config through the production
// builder, over the harness PKI.
//
// It reaches tlsClientConfig rather than startTLSClient because that call also
// starts the TLS engine goroutine and needs a live conversation to feed it. The
// wire assertion in the positive test below covers the join between the two: a
// config startTLSClient did not use could not shape the ClientHello ze sent.
func peerTLSConfigForTest(t *testing.T, pki *eapTLSPKI) *tls.Config {
	t.Helper()

	peer := NewPeerSessionTLS("eap-tls-client", pki.peerConfigWithCRL(pki.trustedCRLPEM))
	t.Cleanup(peer.Close)

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

// TestEAPTLSCapsBothRolesAtTLS13 reads the tls.Config each EAP-TLS role builds
// and the versions the peer's ClientHello offered on the wire.
//
// RFC requirement: RFC9190-1-1 positive -- RFC 9190 Section 1: "Since EAP-TLS
// couples the TLS handshake state machine with the EAP state machine, it is
// possible that new versions of TLS will cause incompatibilities that introduce
// failures or security issues if they are not carefully integrated into the
// EAP-TLS protocol.  Therefore, implementations MUST limit the maximum TLS
// version they use to 1.3, unless later versions are explicitly enabled by the
// administrator." newTLSMethod (eap_tls.go) and PeerSession.tlsClientConfig
// (peer.go) each set MaxVersion to tls.VersionTLS13, and the peer's ClientHello
// offers no version above TLS 1.3. Ze exposes no leaf that raises the ceiling,
// so the administrator's exception has nothing to switch on and the cap is
// unconditional.
func TestEAPTLSCapsBothRolesAtTLS13(t *testing.T) {
	pki := newEAPTLSPKI(t)

	sess, err := NewSession(TypeTLS, pki.serverConfig())
	if err != nil {
		t.Fatalf("create the authenticator session: %v", err)
	}
	t.Cleanup(sess.Close)
	method, ok := sess.method.(*tlsMethod)
	if !ok {
		t.Fatalf("authenticator method is %T, want *tlsMethod", sess.method)
	}

	for _, role := range []struct {
		name string
		cfg  *tls.Config
	}{
		{name: "authenticator", cfg: method.tlsConfig},
		{name: "peer", cfg: peerTLSConfigForTest(t, pki)},
	} {
		if role.cfg.MaxVersion != tls.VersionTLS13 {
			t.Errorf("the %s caps its TLS version at %s, want %s", role.name,
				tls.VersionName(role.cfg.MaxVersion), tls.VersionName(tls.VersionTLS13))
		}
	}

	// What the ceiling produces on the wire. tls.ClientHelloInfo.SupportedVersions
	// is the supported_versions extension crypto/tls parsed out of the peer's
	// ClientHello, which is the offer an authenticator picks from.
	exchange := driveAttackExchange(t, pki, 0)
	if !exchange.hello.hasExtension(tlsExtSupportedVersions) {
		t.Fatal("the peer's ClientHello carries no supported_versions extension, so its offer cannot be read")
	}
	for _, version := range exchange.offer.SupportedVersions {
		if version > tls.VersionTLS13 {
			t.Errorf("the peer's ClientHello offers %s, above the TLS 1.3 ceiling RFC 9190 Section 1 sets",
				tls.VersionName(version))
		}
	}
	if exchange.peerState.Version != tls.VersionTLS13 || exchange.authState.Version != tls.VersionTLS13 {
		t.Errorf("an unconstrained exchange negotiated peer=%s authenticator=%s, want TLS 1.3 on both",
			tls.VersionName(exchange.peerState.Version), tls.VersionName(exchange.authState.Version))
	}
}

// TestEAPTLSVersionCapLeavesTLS12Reachable holds the authenticator at TLS 1.2
// and drives a whole exchange against the uncapped production peer.
//
// RFC requirement: RFC9190-1-1 negative -- Section 1 sets a MAXIMUM, so the
// ceiling is a range end and not a pin. The peer's own tls.Config still names
// MinVersion below it, and an exchange an authenticator holds at TLS 1.2
// completes, negotiates TLS 1.2 and derives the RFC 5216 Section 2.3 MSK both
// ends share. A role that met the MUST by setting MaxVersion equal to
// MinVersion would satisfy the positive above and would refuse every peer that
// RFC 5216 still governs.
func TestEAPTLSVersionCapLeavesTLS12Reachable(t *testing.T) {
	pki := newEAPTLSPKI(t)

	cfg := peerTLSConfigForTest(t, pki)
	if cfg.MinVersion >= cfg.MaxVersion {
		t.Fatalf("the peer accepts TLS %s only: MinVersion %s is not below MaxVersion %s",
			tls.VersionName(cfg.MaxVersion), tls.VersionName(cfg.MinVersion), tls.VersionName(cfg.MaxVersion))
	}

	flight := driveEAPTLSFlight(t, pki.serverConfig(), newAttackPeer(pki), tls.VersionTLS12, 40)
	if flight.peerErr != nil {
		t.Fatalf("the peer refused a TLS 1.2 exchange the version ceiling still permits: %v", flight.peerErr)
	}
	if !flight.peerDone {
		t.Fatal("the peer completed no TLS 1.2 exchange")
	}

	var zero [64]byte
	if flight.peerMSK == zero || flight.peerMSK != flight.serverMSK {
		t.Fatalf("the TLS 1.2 exchange derived no shared MSK: peer=%x authenticator=%x", flight.peerMSK, flight.serverMSK)
	}
}
