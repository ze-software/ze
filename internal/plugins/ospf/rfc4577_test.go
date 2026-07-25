// VALIDATES: RFC 4577 -- the OSPF-side capabilities the PE/CE spec leans on that ze
// actually implements: OSPF cryptographic authentication on the PE-CE link (sec 6) and an
// area 0 adjacency reached over an OSPF virtual link (sec 4.1.4).
// PREVENTS: a regression where a configured key chain stops driving cryptographic
// authentication on an interface, where a forged digest verifies, or where a resolved
// virtual link stops surfacing as a backbone (area 0) link.
package ospf

import (
	"bytes"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	ospflsdb "github.com/ze-software/ze/internal/plugins/ospf/lsdb"
	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// RFC requirement: RFC4577-6-1 positive -- OSPF cryptographic authentication is implemented
// on this router: the key store resolves the configured chain to a cryptographic key
// (authStore.signKey, auth_keystore.go:292), the digest is computed over the packet
// (cryptoDigest/Sign, packet/auth_verify.go:141,157) and the receive side recomputes and
// accepts it (authStore.verify, auth_keystore.go:330 -> packet.Verify, auth_verify.go:211).
func TestRFC4577CryptographicAuthImplemented(t *testing.T) {
	for _, algo := range []string{"md5", "hmac-sha-256"} {
		t.Run(algo, func(t *testing.T) {
			s := newAuthStore()
			s.configure(authCfg(keyConfig{KeyID: 1, Algorithm: algo, Secret: "topsecret"}))
			peer := ridOf("2.2.2.2")

			_, au, _, _, ok := s.signKey("eth0")
			require.True(t, ok, "the configured chain resolves a signing key")
			assert.Equal(t, packet.AuTypeCryptographic, au, "cryptographic authentication, not simple password")

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

// RFC requirement: RFC4577-6-2 positive -- cryptographic authentication is actually USED on
// the link to the neighbor, not merely available: an interface whose `authentication` mode
// is `inherit` picks up the area's key chain and every packet it sends on that link is
// signed with the cryptographic AuType (authStore.configure resolving area inheritance,
// auth_keystore.go:195-238; authStore.signKey, auth_keystore.go:292; the engine signs each
// transmitted packet through it in signPacket, auth_wiring.go:43).
func TestRFC4577InterfaceUsesConfiguredCryptoAuth(t *testing.T) {
	s := newAuthStore()
	s.configure(authCfg(keyConfig{KeyID: 7, Algorithm: "hmac-sha-256", Secret: "pe-ce-secret"}))

	key, au, _, _, ok := s.signKey("eth0")
	require.True(t, ok, "the PE-CE interface inherits the area key chain")
	assert.Equal(t, packet.AuTypeCryptographic, au)
	assert.Equal(t, uint32(7), key.KeyID)
	assert.Equal(t, []byte("pe-ce-secret"), key.Secret)

	wire, src := signedHello(t, s, "eth0")
	assert.Greater(t, len(wire), packet.CommonHeaderLen, "the packet carries an appended digest")
	reason, verified := s.verify("eth0", ridOf("2.2.2.2"), src, wire)
	assert.True(t, verified, "the neighbor on the link verifies what this interface signs")
	assert.Empty(t, reason)
}

// RFC requirement: RFC4577-4.1.4-2 positive -- an area 0 adjacency may be reached over an
// OSPF virtual link: a configured virtual link that the transit-area SPF has resolved as
// reachable surfaces as a synthetic point-to-point interface whose area is the BACKBONE,
// carrying the transit area and the computed transit cost (virtualLinkTopology,
// virtual_link.go; the runtime is driven by onVirtualLinksResolved). The neighbor across it
// therefore holds an area 0 link to this router.
func TestRFC4577VirtualLinkGivesArea0Adjacency(t *testing.T) {
	e := newEngine(nil)
	transit := vlArea(t, "0.0.0.3")
	neighbor := vlRID(t, "9.9.9.9")
	key := virtualLinkKey{transit: transit, neighbor: neighbor}
	e.virtualLinks = map[virtualLinkKey]*virtualLinkRuntime{
		key: {
			cfg:       virtualLinkConfig{TransitArea: transit, RemoteRouterID: neighbor},
			name:      virtualLinkName(key),
			reachable: true,
			cost:      17,
			localAddr: netip.MustParseAddr("172.16.0.1"),
		},
	}
	e.cfg.RouterID = vlRID(t, "1.1.1.1")

	got := e.virtualLinkTopology()
	require.Len(t, got, 1, "a reachable virtual link surfaces one synthetic interface")
	assert.Equal(t, types.BackboneArea, got[0].AreaID, "the virtual link is an area 0 link")
	assert.Equal(t, ospflsdb.NetworkVirtual, got[0].NetworkType)
	assert.Equal(t, transit, got[0].VirtualTransitArea)
	assert.Equal(t, uint16(17), got[0].Cost)
}
