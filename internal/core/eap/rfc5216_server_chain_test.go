// Design: docs/architecture/ike/ipsec-9-ikev2-eap-nat.md -- EAP-TLS (RFC 5216)
// RFC: rfc/short/rfc5216.md -- Section 5.3: the server chain minus the root, validated by the peer
// Related: rfc5216_flight_content_test.go -- the flight parser and the single-level chain case
// Related: rfc9190_revocation_test.go -- newSubCA
//
// VALIDATES: with a two-level authority, the authenticator sends its leaf and
// the intermediate CA and not the root, and ze's peer, trusting only the root,
// path-validates that chain; a server that leaves the intermediate out is
// refused by the peer, which cannot build a path to its anchor.
// PREVENTS: a server chain whose intermediate is dropped, and a peer that
// accepts a server certificate it cannot chain to its trust anchor.

package eap

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"testing"
)

// intermediateServerFlight drives a TLS 1.2 EAP-TLS conversation whose server
// certificate was issued by an intermediate CA under the peer's trust anchor.
// sendIntermediate decides whether the server's configured chain carries that
// intermediate. It returns the flight, the certificate list of the server's
// certificate message, and the DER of the leaf, the intermediate and the root.
func intermediateServerFlight(t *testing.T, sendIntermediate bool) (fl *eapTLSFlight, chain [][]byte, leaf, sub, root []byte) {
	t.Helper()
	pki := newEAPTLSPKI(t)
	subCA, subKey, subPEM := newSubCA(t, pki.trustedCA, pki.trustedCAKey, "eap-tls-server-sub-ca", 800)
	leafPEM, leafKeyPEM := newLeaf(t, subCA, subKey, "eap-tls-deep-server", 801, []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth})

	serverCfg := pki.serverConfig()
	serverCfg.ServerCertPEM = leafPEM
	if sendIntermediate {
		serverCfg.ServerCertPEM = append(append([]byte{}, leafPEM...), subPEM...)
	}
	serverCfg.ServerKeyPEM = leafKeyPEM

	peer := NewPeerSessionTLS("eap-tls-client", &PeerTLSConfig{
		CertPEM:   pki.clientCertPEM,
		KeyPEM:    pki.clientKeyPEM,
		CACertPEM: pki.trustedCAPEM,
		CRLPEM:    append(append([]byte{}, pki.trustedCRLPEM...), newCRL(t, subCA, subKey)...),
	})
	fl = driveEAPTLSFlight(t, serverCfg, peer, tls.VersionTLS12, eapTLS13Rounds)

	server := eapTLSMessages(t, fl.serverSent)
	if len(server) < 1 {
		t.Fatal("the exchange carried no server flight")
	}
	body, ok := parseTLSFlight(t, server[0]).bodies[tlsHSCertificate]
	if !ok || len(body) < 3 {
		t.Fatalf("the server flight carries no certificate message (present %v, %d octets)", ok, len(body))
	}
	for off := 3; off < len(body); {
		if off+3 > len(body) {
			t.Fatal("a truncated certificate length")
		}
		size := int(body[off])<<16 | int(body[off+1])<<8 | int(body[off+2])
		if off+3+size > len(body) {
			t.Fatal("a certificate overruns its message")
		}
		chain = append(chain, body[off+3:off+3+size])
		off += 3 + size
	}
	block, _ := pem.Decode(leafPEM)
	if block == nil {
		t.Fatal("the server leaf PEM does not decode")
	}
	return fl, chain, block.Bytes, subCA.Raw, pki.trustedCA.Raw
}

// TestRFC5216ServerSendsItsIntermediateButNotTheRoot reads the certificate list
// of a server whose certificate an intermediate CA issued, and what the peer did
// with it.
//
// RFC requirement: RFC5216-5.3-4 positive -- with a server certificate issued by
// an intermediate CA, the server's certificate message carries the leaf, then
// the intermediate, and not the root; the peer, trusting only the root,
// path-validates that chain and concludes with the authenticator's EAP-Success.
func TestRFC5216ServerSendsItsIntermediateButNotTheRoot(t *testing.T) {
	fl, chain, leaf, sub, root := intermediateServerFlight(t, true)
	if len(chain) != 2 {
		t.Fatalf("the server sent %d certificates, want its leaf and the intermediate", len(chain))
	}
	if !bytes.Equal(chain[0], leaf) {
		t.Fatal("the server's first certificate is not its leaf")
	}
	if !bytes.Equal(chain[1], sub) {
		t.Fatal("the server's second certificate is not the intermediate CA")
	}
	for _, cert := range chain {
		if bytes.Equal(cert, root) {
			t.Fatal("the server sent the root certificate")
		}
	}
	if fl.peerErr != nil {
		t.Fatalf("the peer refused a server chain it can validate: %v", fl.peerErr)
	}
	if fl.successAt < 0 || !fl.peerDone {
		t.Fatalf("the exchange ended without EAP-Success reaching a concluded peer (success at %d, peer done %v)", fl.successAt, fl.peerDone)
	}
}

// TestRFC5216PeerRefusesAServerChainMissingItsIntermediate drives a server whose
// configured chain leaves the intermediate CA out, the form Section 5.3 asks the
// server not to send.
//
// RFC requirement: RFC5216-5.3-4 negative -- a server that omits the
// intermediate puts its leaf alone on the wire, and the peer, trusting only the
// root, cannot build a path: it refuses with an unknown authority error, does
// not conclude, and no EAP-Success is sent.
func TestRFC5216PeerRefusesAServerChainMissingItsIntermediate(t *testing.T) {
	fl, chain, leaf, _, _ := intermediateServerFlight(t, false)
	if len(chain) != 1 || !bytes.Equal(chain[0], leaf) {
		t.Fatalf("the server sent %d certificates, want its leaf alone: the non-compliant chain under test", len(chain))
	}
	if fl.peerDone {
		t.Fatal("the peer concluded with a server certificate it cannot chain to its trust anchor")
	}
	if fl.successAt >= 0 {
		t.Fatal("the exchange reached EAP-Success with a server chain missing its intermediate")
	}
	if _, unknown := errors.AsType[x509.UnknownAuthorityError](fl.peerErr); !unknown {
		t.Fatalf("the peer refused with %v, want an unknown authority failure", fl.peerErr)
	}
}
