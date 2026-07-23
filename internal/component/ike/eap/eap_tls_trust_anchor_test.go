// Design: plan/learned/805-ipsec-11-interop-eap.md -- EAP-TLS trust anchor handling
// RFC: rfc/short/rfc5216.md -- EAP-TLS certificate path validation (Section 5.3)
//
// Pins the authenticator-side consequence of an ABSENT trust anchor, which the
// engine's eapTLSServerConfig now refuses to produce. The refusal is only
// correct because of the behavior asserted here: newTLSMethod hands
// crypto/tls a NON-NIL but EMPTY x509.CertPool as ClientCAs, and an empty
// non-nil pool rejects every chain rather than falling back to the host's root
// store (only a nil Roots does that). The authenticator therefore fails CLOSED,
// but silently and late -- every client is refused at handshake with an opaque
// error naming neither the peer nor the CA that failed to load.
//
// Without this test the engine-side fix reads as if it were closing a
// fail-OPEN, which it is not, and a future reader could "simplify" the guard
// away on the belief that the empty pool was already permissive.

package eap

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"testing"
)

// VALIDATES: an authenticator built with no CACertPEM refuses a client whose
// certificate is signed by an otherwise-valid CA.
// PREVENTS: believing an authenticator with no configured trust anchor accepts
// any client (it accepts none), and prevents a future change that swaps the
// empty pool for a nil one, which WOULD fall back to the host root store and
// turn this into a genuine fail-open.
func TestEAPTLSAuthenticatorWithoutCARejectsEveryClient(t *testing.T) {
	pki := newEAPTLSPKI(t)

	// Same as pki.serverConfig() but with the trust anchor withheld.
	serverCfg := MethodConfig{
		ServerCertPEM: pki.serverCertPEM,
		ServerKeyPEM:  pki.serverKeyPEM,
	}

	peer := NewPeerSessionTLS("eap-tls-client", &PeerTLSConfig{
		CertPEM:   pki.clientCertPEM,
		KeyPEM:    pki.clientKeyPEM,
		CACertPEM: pki.trustedCAPEM,
	})

	res := runEAPTLSHandshake(t, serverCfg, peer)

	if res.serverEAPSuccess {
		t.Fatal("authenticator with no trust anchor sent EAP-Success: an empty ClientCAs pool accepted a client chain")
	}
	if res.peerDone {
		t.Fatal("peer completed against an authenticator with no trust anchor")
	}
	if ss := res.serverState(); ss.HandshakeComplete {
		t.Fatal("authenticator completed the TLS handshake with an empty ClientCAs pool")
	}
}

// VALIDATES: newTLSMethod always hands crypto/tls a NON-NIL ClientCAs pool.
// PREVENTS: a refactor replacing `x509.NewCertPool()` with a nil pool on the
// assumption that the two are equivalent. They are not: a nil Roots means
// "trust the host's root store", which for an EAP authenticator would accept
// any client holding a certificate from any public CA -- a real fail-open,
// where the empty pool is merely a silent fail-closed.
//
// This assertion exists because the two behavioral tests in this file CANNOT
// catch that mutation: their fixtures are signed by a locally generated CA that
// is in no host root store, so a nil pool rejects them exactly as an empty pool
// does and both stay green. Found by review. The property has to be asserted on
// the constructed config, not inferred from a handshake outcome.
func TestNewTLSMethodNeverPassesNilClientCAs(t *testing.T) {
	pki := newEAPTLSPKI(t)

	for _, tc := range []struct {
		name string
		cfg  MethodConfig
	}{
		{"with a trust anchor", pki.serverConfig()},
		{"without a trust anchor", MethodConfig{
			ServerCertPEM: pki.serverCertPEM,
			ServerKeyPEM:  pki.serverKeyPEM,
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, err := newTLSMethod(tc.cfg)
			if err != nil {
				t.Fatalf("newTLSMethod: %v", err)
			}
			if m.tlsConfig.ClientCAs == nil {
				t.Fatal("ClientCAs is nil: crypto/tls would fall back to the host root store and accept any publicly-issued client certificate")
			}
			if m.tlsConfig.ClientAuth != tls.RequireAndVerifyClientCert {
				t.Fatalf("ClientAuth = %v, want RequireAndVerifyClientCert", m.tlsConfig.ClientAuth)
			}
		})
	}
}

// VALIDATES: the crypto/x509 rule the fix depends on -- an empty NON-NIL pool
// rejects.
// PREVENTS: reasoning about the empty pool as if it were permissive. It is not;
// the defect it caused was silence, not acceptance.
func TestEmptyClientCAPoolRejects(t *testing.T) {
	pki := newEAPTLSPKI(t)

	block, _ := pem.Decode(pki.clientCertPEM)
	if block == nil {
		t.Fatal("could not decode the client certificate PEM")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("parse client certificate: %v", err)
	}

	empty := x509.NewCertPool()
	if _, err := cert.Verify(x509.VerifyOptions{
		Roots:     empty,
		KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageAny},
	}); err == nil {
		t.Fatal("an empty non-nil CertPool accepted a certificate: the authenticator's ClientCAs would be permissive")
	}
}
