// Design: docs/architecture/ike/ipsec-9-ikev2-eap-nat.md -- EAP-TLS (RFC 5216)
// RFC: rfc/short/rfc5216.md -- Section 2.1.1: a peer that answers certificate_request without a certificate
// Related: rfc5216_flight_content_test.go -- the flight parser and the compliant peer flight
//
// VALIDATES: a peer flight that answers the server's certificate_request with
// an empty certificate message and no certificate_verify is refused by ze's
// authenticator: no EAP-Success, no success state, no completed TLS handshake.
// PREVENTS: an authenticator that accepts a peer which skipped client
// authentication after it was asked for it.

package eap

import (
	"bytes"
	"crypto/tls"
	"testing"
)

// TestRFC5216AuthenticatorRefusesAPeerThatSendsNoCertificate drives a TLS 1.2
// EAP-TLS conversation with a peer whose only certificate was issued by a CA the
// authenticator's certificate_request does not name.
//
// Method: crypto/tls on the peer side finds no certificate acceptable to that
// certificate_request, so it answers with an empty certificate message and no
// certificate_verify, which is the flight RFC 5216 Section 2.1.1 forbids. The
// test reads that flight off the wire, so the non-compliant form is shown to be
// on the wire, and then reads what the authenticator did with it.
//
// RFC requirement: RFC5216-2.1.1-1 negative -- after a certificate_request, a
// peer flight carrying an empty certificate list and no certificate_verify
// draws no EAP-Success: the authenticator Session does not succeed and its TLS
// handshake does not complete.
func TestRFC5216AuthenticatorRefusesAPeerThatSendsNoCertificate(t *testing.T) {
	pki := newEAPTLSPKI(t)
	peer := NewPeerSessionTLS("rogue-client", &PeerTLSConfig{
		CertPEM:   pki.untrustedClientCertPEM,
		KeyPEM:    pki.untrustedClientKeyPEM,
		CACertPEM: pki.trustedCAPEM,
		CRLPEM:    pki.trustedCRLPEM,
	})
	fl := driveEAPTLSFlight(t, pki.serverConfig(), peer, tls.VersionTLS12, eapTLS13Rounds)

	server := eapTLSMessages(t, fl.serverSent)
	sent := eapTLSMessages(t, fl.peerSent)
	if len(server) < 1 || len(sent) < 2 {
		t.Fatalf("the exchange carried %d server and %d peer messages, want a full server flight and a peer answer", len(server), len(sent))
	}
	if !bytes.Contains(parseTLSFlight(t, server[0]).types, []byte{tlsHSCertificateRequest}) {
		t.Fatal("the server flight carries no certificate_request, so this peer was never asked for a certificate")
	}

	answer := parseTLSFlight(t, sent[1])
	if bytes.Contains(answer.types, []byte{tlsHSCertificateVerify}) {
		t.Fatalf("the peer flight carries %v, including certificate_verify: it is not the certless flight under test", answer.types)
	}
	// A certificate message whose certificate_list is empty is three zero
	// octets: the 24-bit list length.
	if body, ok := answer.bodies[tlsHSCertificate]; !ok || !bytes.Equal(body, []byte{0, 0, 0}) {
		t.Fatalf("the peer's certificate message body is %x (present %v), want an empty certificate_list", body, ok)
	}

	for _, packet := range fl.serverSent {
		if packet.Code == CodeSuccess {
			t.Fatal("the authenticator sent EAP-Success to a peer that sent no certificate")
		}
	}
	if fl.sess.Succeeded() {
		t.Fatal("the authenticator Session succeeded for a peer that sent no certificate")
	}
	method, ok := fl.sess.method.(*tlsMethod)
	if !ok {
		t.Fatalf("authenticator method is %T", fl.sess.method)
	}
	if method.conn.ConnectionState().HandshakeComplete {
		t.Fatal("the authenticator completed its TLS handshake with a peer that sent no certificate")
	}
}
