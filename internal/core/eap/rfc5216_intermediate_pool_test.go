// Design: docs/architecture/ike/ipsec-11-interop-eap.md -- EAP-TLS configured intermediate certificates
// RFC: rfc/short/rfc5216.md -- Section 5.3, Certificate Validation
//
// This file drives the real EAP-TLS authenticator against the real EAP-TLS peer
// over a three-level chain (trust anchor, intermediate CA, client leaf) where
// the peer sends its leaf ALONE, so the intermediate path validation needs has
// to come from somewhere other than the peer's certificate message.
//
// VALIDATES: the operator's configured intermediate certificates complete the
// path the peer left incomplete, a configured certificate that is not on the
// path completes nothing, and a configured intermediate never acts as a trust
// anchor.
// PREVENTS: the authenticator relying on the peer alone for every intermediate,
// and an intermediate list that silently widens what the authenticator trusts.

package eap

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"strings"
	"testing"
	"time"
)

// leafOnlyChain is a client chain one level deeper than the harness default:
// trustedCA issues an intermediate CA, and the intermediate issues the client
// leaf. The peer config holds the leaf alone, so its certificate message
// carries no intermediate.
type leafOnlyChain struct {
	pki             *eapTLSPKI
	intermediatePEM []byte
	leafPEM         []byte
	crlPEM          []byte
	peer            *PeerSession
}

func newLeafOnlyChain(t *testing.T) *leafOnlyChain {
	t.Helper()
	return newLeafOnlyChainWith(t, []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth})
}

// leafOnlySerial is the client leaf's serial number, for a list revoking it.
const leafOnlySerial = 801

// newLeafOnlyChainWith is newLeafOnlyChain with the leaf's extended key usage
// named and serials the intermediate's list revokes, for the tests that prove
// the authenticator's own verification keeps every check crypto/tls applied.
func newLeafOnlyChainWith(t *testing.T, eku []x509.ExtKeyUsage, revokedBySub ...int64) *leafOnlyChain {
	t.Helper()
	pki := newEAPTLSPKI(t)
	sub, subKey, subPEM := newSubCA(t, pki.trustedCA, pki.trustedCAKey, "eap-tls-pool-sub-ca", 800)
	leafPEM, leafKeyPEM := newLeaf(t, sub, subKey, "eap-tls-leaf-only-client", leafOnlySerial, eku)

	// TLS 1.3 makes the authenticator check every certificate on the chain
	// against a list (RFC 9190 Section 5.4), so both CAs publish one that
	// revokes nothing. Without them a refusal could be a revocation refusal,
	// and the tests below could not tell it from a path validation one.
	crls := append(append([]byte{}, newCRL(t, pki.trustedCA, pki.trustedCAKey)...), newCRL(t, sub, subKey, revokedBySub...)...)

	return &leafOnlyChain{
		pki:             pki,
		intermediatePEM: subPEM,
		leafPEM:         leafPEM,
		crlPEM:          crls,
		peer: NewPeerSessionTLS("eap-tls-leaf-only-client", &PeerTLSConfig{
			CertPEM:   leafPEM,
			KeyPEM:    leafKeyPEM,
			CACertPEM: pki.trustedCAPEM,
			CRLPEM:    pki.trustedCRLPEM,
		}),
	}
}

func (c *leafOnlyChain) serverConfig(intermediatePEM []byte) MethodConfig {
	cfg := c.pki.serverConfig()
	cfg.CRLPEM = c.crlPEM
	cfg.IntermediateCertPEM = intermediatePEM
	return cfg
}

// requireRefusedUnknownAuthority asserts the authenticator refused the exchange
// because path validation found no route to the trust anchor.
func requireRefusedUnknownAuthority(t *testing.T, res *hsResult, why string) {
	t.Helper()
	if res.serverEAPSuccess {
		t.Fatalf("the authenticator sent EAP-Success: %s", why)
	}
	err := res.sess.Err()
	if err == nil {
		t.Fatal("the authenticator refused the exchange without recording a reason")
	}
	if !strings.Contains(err.Error(), "unknown authority") {
		t.Fatalf("the authenticator refused for %q, which is not a path validation failure", err)
	}
}

// TestEAPTLSConfiguredIntermediateCompletesALeafOnlyChain drives a TLS 1.3
// exchange whose peer sends its leaf without the intermediate that issued it,
// with that intermediate configured on the authenticator.
//
// RFC requirement: RFC5216-5.3-5 positive -- RFC 5216 Section 5.3: the server
// "SHOULD support validating the peer certificate using RFC 3280 [RFC3280]
// compliant path validation, including the ability to retrieve intermediate
// certificates that may be necessary to validate the peer certificate." The
// intermediate here is necessary, the peer did not send it, and the
// authenticator completes the path from its configured intermediate
// certificates, which is the pre-configuration the same section names.
func TestEAPTLSConfiguredIntermediateCompletesALeafOnlyChain(t *testing.T) {
	chain := newLeafOnlyChain(t)

	res := runEAPTLSHandshake(t, chain.serverConfig(chain.intermediatePEM), chain.peer)

	if err := res.sess.Err(); err != nil {
		t.Fatalf("the authenticator refused a chain its configured intermediate completes: %v", err)
	}
	if !res.serverEAPSuccess {
		t.Fatal("no EAP-Success: the configured intermediate did not complete the path the peer left open")
	}
}

// TestEAPTLSLeafOnlyChainRefusedWithoutAnIntermediate is the same exchange with
// no intermediate configured.
//
// RFC requirement: RFC5216-5.3-5 negative -- the positive test above proves
// nothing unless the intermediate it configures is what completed the path. With
// none configured, the authenticator has only what the peer sent, which is the
// leaf, and path validation finds no route to the trust anchor.
func TestEAPTLSLeafOnlyChainRefusedWithoutAnIntermediate(t *testing.T) {
	chain := newLeafOnlyChain(t)

	res := runEAPTLSHandshake(t, chain.serverConfig(nil), chain.peer)

	requireRefusedUnknownAuthority(t, res, "a leaf-only chain with no intermediate anywhere was accepted")
}

// TestEAPTLSUnrelatedConfiguredIntermediateCompletesNothing configures an
// intermediate CA that the trust anchor issued but that did not issue the
// client leaf.
//
// RFC requirement: RFC5216-5.3-5 negative -- the configured list completes a
// path only through a certificate that is ON it. A list read as "the operator
// vouched for this peer" would accept here, because the configured certificate
// does chain to the trust anchor; the leaf's issuer is still missing, so the
// authenticator refuses.
func TestEAPTLSUnrelatedConfiguredIntermediateCompletesNothing(t *testing.T) {
	chain := newLeafOnlyChain(t)
	_, _, unrelatedPEM := newSubCA(t, chain.pki.trustedCA, chain.pki.trustedCAKey, "eap-tls-unrelated-sub-ca", 802)

	res := runEAPTLSHandshake(t, chain.serverConfig(unrelatedPEM), chain.peer)

	requireRefusedUnknownAuthority(t, res, "an intermediate that is not on the client's path completed it")
}

// TestEAPTLSConfiguredIntermediateIsNoTrustAnchor configures the leaf's real
// issuer as an intermediate while the authenticator's trust anchor is a CA that
// issued neither.
//
// RFC requirement: RFC5216-5.3-5 negative -- Section 5.3 asks for intermediate
// certificates "necessary to validate the peer certificate", and an
// intermediate is a link on the path, never its end. Pooled into the roots, the
// intermediate would anchor the leaf on its own and the exchange would
// complete; kept as an intermediate, it has no route to the configured anchor.
func TestEAPTLSConfiguredIntermediateIsNoTrustAnchor(t *testing.T) {
	chain := newLeafOnlyChain(t)
	cfg := chain.serverConfig(chain.intermediatePEM)
	cfg.CACertPEM = chain.pki.untrustedServerCertPEM

	res := runEAPTLSHandshake(t, cfg, chain.peer)

	requireRefusedUnknownAuthority(t, res, "a configured intermediate anchored the client chain")
}

// TestNewTLSMethodRefusesANonCAIntermediate hands the authenticator a leaf
// where an intermediate CA belongs.
//
// The commit-time check refuses this config before it reaches the EAP core
// (ValidateIntermediateRefs, ipsec/validate.go). This is the second check of the
// pair: a certificate that cannot sign another has no place on a path, and a
// caller that skipped the first check learns so at session build, with a
// message that names the certificate.
func TestNewTLSMethodRefusesANonCAIntermediate(t *testing.T) {
	pki := newEAPTLSPKI(t)
	cfg := pki.serverConfig()
	cfg.IntermediateCertPEM = pki.clientCertPEM

	_, err := newTLSMethod(cfg)
	if err == nil {
		t.Fatal("newTLSMethod accepted a leaf certificate as an intermediate CA")
	}
	if !strings.Contains(err.Error(), "not a CA certificate") {
		t.Fatalf("newTLSMethod refused for %q, which does not say the certificate is not a CA", err)
	}
}

// TestNewTLSMethodKeepsCryptoTLSVerificationWithoutIntermediates pins the
// authenticator's mode choice: with no configured intermediate, crypto/tls
// verifies the client chain itself, and with one, newTLSMethod does.
func TestNewTLSMethodKeepsCryptoTLSVerificationWithoutIntermediates(t *testing.T) {
	chain := newLeafOnlyChain(t)

	plain, err := newTLSMethod(chain.serverConfig(nil))
	if err != nil {
		t.Fatalf("newTLSMethod: %v", err)
	}
	if plain.tlsConfig.ClientAuth != tls.RequireAndVerifyClientCert {
		t.Fatalf("ClientAuth without intermediates = %v, want RequireAndVerifyClientCert", plain.tlsConfig.ClientAuth)
	}

	pooled, err := newTLSMethod(chain.serverConfig(chain.intermediatePEM))
	if err != nil {
		t.Fatalf("newTLSMethod: %v", err)
	}
	if pooled.tlsConfig.ClientAuth != tls.RequireAnyClientCert {
		t.Fatalf("ClientAuth with intermediates = %v, want RequireAnyClientCert, whose chain newTLSMethod verifies", pooled.tlsConfig.ClientAuth)
	}
}

// requireRefusedFor asserts the authenticator refused the exchange, and that
// the recorded reason holds want.
func requireRefusedFor(t *testing.T, res *hsResult, want, why string) {
	t.Helper()
	if res.serverEAPSuccess {
		t.Fatalf("the authenticator sent EAP-Success: %s", why)
	}
	err := res.sess.Err()
	if err == nil {
		t.Fatal("the authenticator refused the exchange without recording a reason")
	}
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("the authenticator refused for %q, want a reason holding %q", err, want)
	}
}

// TestEAPTLSConfiguredIntermediateKeepsClientAuthUsage drives the pooled mode
// with a leaf whose extended key usage is serverAuth alone.
//
// With intermediates configured the authenticator verifies the chain itself in
// place of crypto/tls, so it MUST keep the policy crypto/tls applied to a
// client chain: the leaf is usable for client authentication. A verification
// that dropped KeyUsages would accept a server certificate as a client one.
func TestEAPTLSConfiguredIntermediateKeepsClientAuthUsage(t *testing.T) {
	chain := newLeafOnlyChainWith(t, []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth})

	res := runEAPTLSHandshake(t, chain.serverConfig(chain.intermediatePEM), chain.peer)

	requireRefusedFor(t, res, "incompatible key usage", "a leaf without clientAuth usage authenticated a client")
}

// TestEAPTLSConfiguredIntermediateKeepsRevocation revokes the client leaf on
// the intermediate's list and drives the pooled mode.
//
// The paths the authenticator builds itself MUST reach the same revocation
// check the crypto/tls paths reach (RFC 9190 Section 5.4). A pooled mode that
// verified the path and skipped the list would accept a revoked leaf.
func TestEAPTLSConfiguredIntermediateKeepsRevocation(t *testing.T) {
	chain := newLeafOnlyChainWith(t, []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}, leafOnlySerial)

	res := runEAPTLSHandshake(t, chain.serverConfig(chain.intermediatePEM), chain.peer)

	requireRefusedFor(t, res, "was revoked by", "a revoked leaf completed the path through a configured intermediate")
}

// TestClientChainVerifierRefusesExpiredAndEmptyChains calls the verifier
// directly, at a clock past the leaf's notAfter and with no certificate at all.
//
// A pooled mode replaces crypto/tls's verification, so it MUST check validity
// time at the clock it is handed, and MUST refuse an empty chain rather than
// index into it or answer no path as success.
func TestClientChainVerifierRefusesExpiredAndEmptyChains(t *testing.T) {
	chain := newLeafOnlyChain(t)
	configured, err := parseIntermediateCAs(chain.intermediatePEM)
	if err != nil {
		t.Fatalf("parseIntermediateCAs: %v", err)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(chain.pki.trustedCAPEM) {
		t.Fatal("parse trust anchor")
	}
	verifier := &clientChainVerifier{roots: roots, configured: configured}
	block, _ := pem.Decode(chain.leafPEM)
	if block == nil {
		t.Fatal("decode client leaf")
	}
	leaf, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("parse client leaf: %v", err)
	}

	if _, err := verifier.verify([]*x509.Certificate{leaf}, time.Now()); err != nil {
		t.Fatalf("a current leaf failed verification: %v", err)
	}

	_, err = verifier.verify([]*x509.Certificate{leaf}, leaf.NotAfter.Add(time.Minute))
	var invalid x509.CertificateInvalidError
	if !errors.As(err, &invalid) {
		t.Fatalf("an expired leaf answered %v, want a CertificateInvalidError", err)
	}
	if invalid.Reason != x509.Expired {
		t.Fatalf("an expired leaf was refused for reason %v, want Expired", invalid.Reason)
	}

	if _, err := verifier.verify(nil, time.Now()); err == nil {
		t.Fatal("an empty client chain verified")
	}
}
