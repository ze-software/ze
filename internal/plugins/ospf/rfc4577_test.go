// RFC 4577 Section 6 authentication on an ordinary OSPF/CE link. Native VPN-PE
// procedures are not selected; these tests retain the implemented link behavior.
package ospf

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

// RFC requirement: RFC4577-6-1 positive -- OSPF cryptographic authentication is implemented
// on this router: the key store resolves the configured chain to a cryptographic key
// (authStore.signKey, auth_keystore.go:292), the digest is computed over the packet
// (cryptoDigest/Sign, packet/auth_verify.go:141,157) and the receive side recomputes and
// accepts it (authStore.verify, auth_keystore.go:330 -> packet.Verify, auth_verify.go:211).
// RFC requirement: RFC4577-6-2 positive -- the configured link authenticates its
// outgoing Hello, and the receiving key chain accepts that protected packet.
func TestRFC4577CryptographicAuthImplemented(t *testing.T) {
	for _, algo := range []string{"md5", "hmac-sha-256"} {
		t.Run(algo, func(t *testing.T) {
			s := newAuthStore()
			s.configure(authCfg(keyConfig{KeyID: 1, Algorithm: algo, Secret: "topsecret"}))
			peer := ridOf("2.2.2.2")

			wire, src := signedHello(t, s, "eth0")
			reason, verified := s.verify("eth0", peer, src, wire)
			assert.True(t, verified, "a correctly signed OSPF packet verifies")
			assert.Empty(t, reason)
		})
	}
}

// RFC requirement: RFC4577-6-1 negative -- the implementation is a real cryptographic check,
// not an accept-all: a packet whose appended digest was altered is rejected by the
// constant-time compare in packet.Verify (auth_verify.go:211-242) reached through
// authStore.verify (auth_keystore.go:330), and a packet signed under a different secret is
// rejected too (no chain key recomputes its digest).
func TestRFC4577CryptographicAuthRejectsForgery(t *testing.T) {
	s := newAuthStore()
	s.configure(authCfg(keyConfig{KeyID: 1, Algorithm: "hmac-sha-256", Secret: "topsecret"}))
	peer := ridOf("2.2.2.2")

	wire, src := signedHello(t, s, "eth0")
	tampered := bytes.Clone(wire)
	tampered[len(tampered)-1] ^= 0xff
	_, ok := s.verify("eth0", peer, src, tampered)
	assert.False(t, ok, "a tampered digest must not verify")

	other := newAuthStore()
	other.configure(authCfg(keyConfig{KeyID: 1, Algorithm: "hmac-sha-256", Secret: "wrongkey"}))
	forged, fsrc := signedHello(t, other, "eth0")
	_, ok = s.verify("eth0", peer, fsrc, forged)
	assert.False(t, ok, "a packet signed with an unknown secret must not verify")
}
