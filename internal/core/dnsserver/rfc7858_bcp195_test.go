// Design: docs/architecture/dns/secure-transports.md -- the DoT listener's TLS policy
// RFC: rfc/short/rfc7858.md -- RFC7858-8-1, BCP 195 (RFC 9325) cipher suites

package dnsserver

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/selfcert"
	"github.com/ze-software/ze/internal/test/tlsprobe"
)

// VALIDATES: RFC 7858 section 8 beyond the TLS 1.2 floor that
// TestDoTRejectsBelowTLS12 proves: the DoT listener negotiates only what the
// RFC 9325 guidance allows. Method: for an ECDSA certificate (the self-signed
// default) and an RSA one (an operator's), the server tls.Config comes from
// selfcert.NewTLSConfig, the constructor buildSecureTLS uses, and a hand-built
// ClientHello (tlsprobe) offers each suite on its own, including the NULL,
// export and single-DES suites Go's client cannot offer.
// PREVENTS: a DoT server that selects a NULL, RC4, export, DES, 3DES, static
// RSA, static ECDH or DHE suite, selects DEFLATE compression, or omits
// renegotiation_info, while the version-floor test stays green.
// RFC requirement: RFC7858-8-1 positive -- with an ECDSA and with an RSA certificate, the DoT listener selects the ECDHE AES-128-GCM suite, null compression and answers with renegotiation_info, also when that suite is offered after every forbidden one.
// RFC requirement: RFC7858-8-1 negative -- each NULL, RC4, export, DES, 3DES, static-RSA, static-ECDH and DHE suite offered alone is never selected (alert or close), and DEFLATE offered alone is refused.
func TestRFC7858DoTNegotiatesOnlyBCP195Suites(t *testing.T) {
	ecdsaCert, ecdsaKey, err := selfcert.GenerateWebCertWithNames("127.0.0.1", []string{"localhost"}, 0)
	if err != nil {
		t.Fatalf("ECDSA certificate: %v", err)
	}
	rsaCert, rsaKey, err := tlsprobe.SelfSignedRSA("127.0.0.1")
	if err != nil {
		t.Fatalf("RSA certificate: %v", err)
	}
	for _, tc := range []struct {
		name      string
		key       tlsprobe.KeyKind
		cert, pem []byte
	}{
		{"ecdsa", tlsprobe.KeyECDSA, ecdsaCert, ecdsaKey},
		{"rsa", tlsprobe.KeyRSA, rsaCert, rsaKey},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srvTLS, err := selfcert.NewTLSConfig(tc.cert, tc.pem)
			if err != nil {
				t.Fatalf("server tls config: %v", err)
			}
			port := freePort(t)
			mgr := New(testLogger(), echoHandler("10.0.0.7"), Options{})
			if err := mgr.applyListeners(true, Listeners{
				DoT:       []Endpoint{{IP: netip.MustParseAddr("127.0.0.1"), Port: port}},
				TLSConfig: srvTLS,
			}); err != nil {
				t.Fatalf("ApplyListeners: %v", err)
			}
			t.Cleanup(mgr.Stop)
			tlsprobe.AssertBCP195(t, hostPort(port), tc.key)
		})
	}
}
