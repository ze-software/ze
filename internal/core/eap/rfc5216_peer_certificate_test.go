// Design: docs/architecture/ike/ipsec-9-ikev2-eap-nat.md -- EAP-TLS (RFC 5216)
// RFC: rfc/short/rfc5216.md -- Section 2.1.1: the peer answers certificate_request with its certificate
// Related: rfc5216_flight_content_test.go -- the flight parser
// Related: rfc5216_certless_peer_test.go -- the certless flight the authenticator refuses
//
// VALIDATES: ze's peer answers a certificate_request with its configured
// certificate and a certificate_verify even when the certificate's issuer is not
// among the certificate_authorities the request names.
// PREVENTS: the peer falling back to an empty certificate_list, which crypto/tls
// does by default when the configured certificate does not match the request.

package eap

import (
	"bytes"
	"crypto/tls"
	"encoding/pem"
	"testing"
)

// TestRFC5216PeerSendsItsCertificateWhateverTheRequestNames drives a TLS 1.2
// EAP-TLS conversation in which the authenticator's certificate_request names
// only its own CA, and the peer's only certificate was issued by another CA.
//
// Method: the peer's answer flight is read off the wire. It must carry a
// certificate message whose first entry is the configured certificate, followed
// by a certificate_verify. Whether the authenticator then trusts that
// certificate is a separate question (RFC 5216 Section 5.3), asked by
// TestEAPTLSServerRejectsUntrustedClientChain.
//
// RFC requirement: RFC5216-2.1.1-1 positive -- after a certificate_request that
// names a CA other than the issuer of the peer's certificate, the peer's
// flight carries a certificate message whose first entry is the configured
// certificate, and a certificate_verify.
func TestRFC5216PeerSendsItsCertificateWhateverTheRequestNames(t *testing.T) {
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
	if !bytes.Contains(answer.types, []byte{tlsHSCertificateVerify}) {
		t.Fatalf("the peer flight carries %v and no certificate_verify", answer.types)
	}
	body, ok := answer.bodies[tlsHSCertificate]
	if !ok {
		t.Fatalf("the peer flight carries %v and no certificate message", answer.types)
	}
	block, _ := pem.Decode(pki.untrustedClientCertPEM)
	if block == nil {
		t.Fatal("the configured peer certificate is not PEM")
	}
	if got := firstCertificateEntry(t, body); !bytes.Equal(got, block.Bytes) {
		t.Fatalf("the peer's first certificate entry is %d octets and is not the configured certificate (%d octets)", len(got), len(block.Bytes))
	}
}

// firstCertificateEntry returns the DER of the first entry of a TLS 1.2
// Certificate message body: a 24-bit list length, then per entry a 24-bit
// length and the DER. An empty list returns nil.
func firstCertificateEntry(t *testing.T, body []byte) []byte {
	t.Helper()
	if len(body) < 3 {
		t.Fatalf("certificate message body is %d octets, shorter than its list length", len(body))
	}
	list := body[3:]
	if len(list) == 0 {
		return nil
	}
	if len(list) < 3 {
		t.Fatalf("certificate_list is %d octets, shorter than an entry length", len(list))
	}
	entryOctets := int(list[0])<<16 | int(list[1])<<8 | int(list[2])
	if len(list) < 3+entryOctets {
		t.Fatalf("certificate entry claims %d octets, the list holds %d", entryOctets, len(list)-3)
	}
	return list[3 : 3+entryOctets]
}
