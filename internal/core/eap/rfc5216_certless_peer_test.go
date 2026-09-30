// Design: docs/architecture/ike/ipsec-9-ikev2-eap-nat.md -- EAP-TLS (RFC 5216)
// RFC: rfc/short/rfc5216.md -- Section 2.1.1: a peer that answers certificate_request without a certificate
// Related: rfc5216_flight_content_test.go -- the flight parser and the compliant peer flight
// Related: rfc5216_peer_certificate_test.go -- ze's peer always sends its certificate
//
// VALIDATES: a peer flight that answers the server's certificate_request with
// an empty certificate message and no certificate_verify is refused by ze's
// authenticator: no EAP-Success, no success state, no completed TLS handshake,
// and a refusal that is not a certificate verification failure.
// PREVENTS: an authenticator that accepts a peer which skipped client
// authentication after it was asked for it.

package eap

import (
	"bytes"
	"crypto/tls"
	"errors"
	"testing"
)

// TestRFC5216AuthenticatorRefusesAPeerThatSendsNoCertificate drives a TLS 1.2
// EAP-TLS conversation with a peer that answers the certificate_request with an
// empty certificate_list.
//
// Method: ze's peer always sends its configured certificate
// (answerCertificateRequest), so the non-compliant flight is made by the test:
// once the peer has started its TLS client, the test empties the certificate the
// peer answers with, before the server flight carrying the certificate_request
// arrives. The peer was configured with a certificate the authenticator trusts,
// so the only thing wrong with the flight is the missing certificate. The test
// reads that flight off the wire, so the non-compliant form is shown to be on
// the wire, and then reads what the authenticator did with it.
//
// RFC requirement: RFC5216-2.1.1-1 negative -- after a certificate_request, a
// peer flight carrying an empty certificate list and no certificate_verify
// draws no EAP-Success: the authenticator Session does not succeed, its TLS
// handshake does not complete, and its refusal is not a certificate
// verification failure.
func TestRFC5216AuthenticatorRefusesAPeerThatSendsNoCertificate(t *testing.T) {
	pki := newEAPTLSPKI(t)
	peer := NewPeerSessionTLS("eap-tls-client", &PeerTLSConfig{
		CertPEM:   pki.clientCertPEM,
		KeyPEM:    pki.clientKeyPEM,
		CACertPEM: pki.trustedCAPEM,
		CRLPEM:    pki.trustedCRLPEM,
	})
	sess, serverSent, peerSent := driveCertlessPeer(t, pki.serverConfig(), peer)

	server := eapTLSMessages(t, serverSent)
	sent := eapTLSMessages(t, peerSent)
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

	for _, packet := range serverSent {
		if packet.Code == CodeSuccess {
			t.Fatal("the authenticator sent EAP-Success to a peer that sent no certificate")
		}
	}
	if sess.Succeeded() {
		t.Fatal("the authenticator Session succeeded for a peer that sent no certificate")
	}
	method, ok := sess.method.(*tlsMethod)
	if !ok {
		t.Fatalf("authenticator method is %T", sess.method)
	}
	if method.conn.ConnectionState().HandshakeComplete {
		t.Fatal("the authenticator completed its TLS handshake with a peer that sent no certificate")
	}
	err := sess.Err()
	if err == nil {
		t.Fatal("the authenticator refused the peer and recorded no reason")
	}
	if _, verifyFailed := errors.AsType[*tls.CertificateVerificationError](err); verifyFailed {
		t.Fatalf("the authenticator refused with a certificate verification failure (%v), but no certificate was sent to verify", err)
	}
}

// driveCertlessPeer runs the real authenticator Session, capped at TLS 1.2 so
// the peer's certificate message crosses in the clear, against a peer whose
// certificate the test empties as soon as its TLS client has started.
//
// The emptying is safe against the TLS engine goroutine: startTLSClient writes
// the certificate before it starts that goroutine, and the goroutine reads it
// only after the next Process call has handed it the server flight.
func driveCertlessPeer(t *testing.T, cfg MethodConfig, peer *PeerSession) (*Session, []*Packet, []*Packet) {
	t.Helper()

	sess, err := NewSession(TypeTLS, cfg)
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	t.Cleanup(func() {
		sess.Close()
		peer.Close()
	})
	method, ok := sess.method.(*tlsMethod)
	if !ok {
		t.Fatalf("authenticator method is %T, want *tlsMethod", sess.method)
	}
	method.tlsConfig.MaxVersion = tls.VersionTLS12

	var serverSent, peerSent []*Packet
	emptied := false
	req := sess.Begin()
	for range eapTLS13Rounds {
		serverSent = append(serverSent, req)
		pres := peer.Process(req)
		if !emptied && peer.tlsStarted.Load() {
			peer.tlsCertificate = tls.Certificate{}
			emptied = true
		}
		if pres.Response == nil {
			break
		}
		peerSent = append(peerSent, pres.Response)
		if pres.Err != nil {
			break
		}
		if pres.Done {
			break
		}
		next := sess.Process(pres.Response)
		if next == nil {
			break
		}
		req = next
	}
	if !emptied {
		t.Fatal("the peer never started its TLS client, so no certificate was emptied")
	}
	return sess, serverSent, peerSent
}
