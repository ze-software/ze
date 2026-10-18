// Design: docs/architecture/ike/ipsec-11-interop-eap.md -- EAP-TLS configured intermediate certificates
// Related: rfc5216_intermediate_pool_test.go -- the leaf-only chain these tests drive
//
// With an intermediate-certificate list configured, newTLSMethod takes client
// chain verification away from crypto/tls and runs it in VerifyConnection. These
// tests pin that the replacement runs on every exchange crypto/tls would have
// verified: a TLS 1.2 exchange, and a TLS 1.3 exchange that resumes a session.
//
// VALIDATES: the pooled verifier path-validates and checks revocation on TLS 1.2
// as on TLS 1.3, and re-validates a resumed session's restored client chain.
// PREVENTS: a VerifyConnection that skips a TLS version or a resumed handshake,
// which would let a revoked client, or one with no path to the trust anchor, in.

package eap

import (
	"crypto/tls"
	"crypto/x509"
	"testing"
	"time"
)

// TestEAPTLS12ConfiguredIntermediateValidatesThePath drives three TLS 1.2
// exchanges in pooled mode: a leaf-only chain the configured intermediate
// completes, the same chain with the leaf revoked by its intermediate, and the
// same chain with the authenticator anchored at a CA that issued neither.
// Only the first may complete.
func TestEAPTLS12ConfiguredIntermediateValidatesThePath(t *testing.T) {
	good := newLeafOnlyChain(t)
	accepted := driveEAPTLSFlight(t, good.serverConfig(good.intermediatePEM), good.peer, tls.VersionTLS12, 30)
	if accepted.successAt < 0 || accepted.sess.Err() != nil {
		t.Fatalf("TLS 1.2: the authenticator refused a chain its configured intermediate completes: %v", accepted.sess.Err())
	}
	method, isTLS := accepted.sess.method.(*tlsMethod)
	if !isTLS {
		t.Fatalf("the authenticator session runs %T, not the EAP-TLS method", accepted.sess.method)
	}
	if version := method.conn.ConnectionState().Version; version != tls.VersionTLS12 {
		t.Fatalf("the exchange negotiated TLS version %#x, not TLS 1.2", version)
	}

	revoked := newLeafOnlyChainWith(t, []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}, leafOnlySerial)
	refused := driveEAPTLSFlight(t, revoked.serverConfig(revoked.intermediatePEM), revoked.peer, tls.VersionTLS12, 30)
	if refused.successAt >= 0 {
		t.Fatal("TLS 1.2: the authenticator accepted a client leaf its configured intermediate revoked")
	}

	unanchored := newLeafOnlyChain(t)
	cfg := unanchored.serverConfig(unanchored.intermediatePEM)
	cfg.CACertPEM = unanchored.pki.untrustedServerCertPEM
	anchored := driveEAPTLSFlight(t, cfg, unanchored.peer, tls.VersionTLS12, 30)
	if anchored.successAt >= 0 {
		t.Fatal("TLS 1.2: a configured intermediate anchored the client chain")
	}
}

// TestEAPTLSResumedSessionRevalidatesThroughConfiguredIntermediate runs a full
// TLS 1.3 exchange in pooled mode, resumes it once unchanged, then resumes it
// again after the intermediate revoked the client leaf. The unchanged resumption
// must complete and the revoked one must be refused on the resumed handshake,
// so a ticket never outlives the chain it was minted for.
func TestEAPTLSResumedSessionRevalidatesThroughConfiguredIntermediate(t *testing.T) {
	pki := newEAPTLSPKI(t)
	sub, subKey, subPEM := newSubCA(t, pki.trustedCA, pki.trustedCAKey, "eap-tls-resumed-sub-ca", 800)
	leafPEM, leafKeyPEM := newLeaf(t, sub, subKey, "eap-tls-resumed-leaf", leafOnlySerial, []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth})
	serverStore := NewResumption(time.Now, true)
	peerStore := NewResumption(time.Now, true)
	serverWith := func(revoked ...int64) MethodConfig {
		cfg := pki.serverConfigResuming(serverStore)
		cfg.CRLPEM = append(append([]byte{}, pki.trustedCRLPEM...), newCRL(t, sub, subKey, revoked...)...)
		cfg.IntermediateCertPEM = subPEM
		return cfg
	}
	peer := func() *PeerSession {
		return NewPeerSessionTLS("eap-tls-resumed-peer", &PeerTLSConfig{
			CertPEM: leafPEM, KeyPEM: leafKeyPEM,
			CACertPEM: pki.trustedCAPEM, CRLPEM: pki.trustedCRLPEM,
			Resumption: peerStore,
		})
	}

	first := driveEAPTLSFlight(t, serverWith(), peer(), tls.VersionTLS13, eapTLS13Rounds)
	requireCompleted(t, first, "the full handshake")

	unchanged := driveEAPTLSFlight(t, serverWith(), peer(), tls.VersionTLS13, eapTLS13Rounds)
	requireCompleted(t, unchanged, "the unchanged resumption")
	if !unchanged.sess.Resumed() {
		t.Fatal("the second exchange did not resume, so the revoked case below proves nothing about resumption")
	}

	revoked := driveEAPTLSFlight(t, serverWith(leafOnlySerial), peer(), tls.VersionTLS13, eapTLS13Rounds)
	if revoked.successAt >= 0 {
		t.Fatal("a resumed session completed for a client leaf its configured intermediate revoked")
	}
	if !revoked.sess.Resumed() {
		t.Fatal("the refusal came from a full handshake, not the resumed one this test is about")
	}
}
