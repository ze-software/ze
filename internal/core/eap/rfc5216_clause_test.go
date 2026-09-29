// Design: docs/architecture/ike/ipsec-9-ikev2-eap-nat.md -- EAP-TLS (RFC 5216)
// RFC: rfc/short/rfc5216.md -- Section 2.1.1: the version each role offers
// Related: rfc5216_tls_config_test.go -- the config helpers this file reuses
//
// crypto/tls has no version below TLS 1.0, so a negotiated version read after
// the handshake cannot go red. What Ze controls is the floor it installs for
// the layer below, and this file reads that floor on both roles.
//
// VALIDATES: the peer and the authenticator each install MinVersion TLS 1.2.
// PREVENTS: a lowered floor on either role passing unnoticed.

package eap

import (
	"crypto/tls"
	"testing"
)

// TestRFC5216BothRolesInstallAVersionFloor reads the tls.Config each role
// hands to crypto/tls.
func TestRFC5216BothRolesInstallAVersionFloor(t *testing.T) {
	pki := newEAPTLSPKI(t)

	// RFC requirement: RFC5216-2.4-1 positive -- RFC 5216 Section 2.1.1: "The
	// version offered by the peer MUST correspond to TLS v1.0 or later." Ze's peer
	// installs MinVersion TLS 1.2, so crypto/tls offers nothing below it.
	peer := NewPeerSessionTLS("eap-tls-client", &PeerTLSConfig{
		CertPEM:   pki.clientCertPEM,
		KeyPEM:    pki.clientKeyPEM,
		CACertPEM: pki.trustedCAPEM,
		CRLPEM:    pki.trustedCRLPEM,
	})
	t.Cleanup(peer.Close)
	if floor := peerTLSConfigFor(t, peer).MinVersion; floor != tls.VersionTLS12 {
		t.Fatalf("the peer installs MinVersion %s, want TLS 1.2", tls.VersionName(floor))
	}

	// RFC requirement: RFC5216-2.4-2 positive -- RFC 5216 Section 2.1.1: "The
	// version offered by the server MUST correspond to TLS v1.0 or later." Ze's
	// authenticator installs MinVersion TLS 1.2 on the config its TLS server runs.
	if floor := authenticatorTLSConfigForTest(t, pki.serverConfig()).MinVersion; floor != tls.VersionTLS12 {
		t.Fatalf("the authenticator installs MinVersion %s, want TLS 1.2", tls.VersionName(floor))
	}
}
