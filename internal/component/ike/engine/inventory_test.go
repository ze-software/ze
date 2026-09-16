// VALIDATES: the wiring row "The IKE engine's init() -> the registered inventory
// snapshot" of spec-path-mtu-diagnostic, and the value type it publishes
// PREVENTS: a reader outside the IKE component seeing the configured transform, or
// no tunnels, on a build that links the engine
package engine

import (
	"net"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/crypto"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/core/ipsecinventory"
)

// TestIPsecInventoryRegisteredByIKE proves that linking this package registers the
// inventory: the query answers no ErrNotRegistered, and it answers the installed
// Child SA state of every active session, including the NEGOTIATED transform and the
// three installed facts the ESP overhead depends on. The provider was registered by
// this package's init(), so no test code registers anything here.
//
// MUTATION: dropping ipsecinventory.Register from init() (register.go) makes the first
// assertion fail; reading ps.espGroup in Info makes the transform assertion fail.
func TestIPsecInventoryRegisteredByIKE(t *testing.T) {
	first := ipsec.ESPProposal{Number: 1, Encryption: ipsec.EncryptionAES256, Hash: ipsec.HashSHA256}
	second := ipsec.ESPProposal{Number: 2, Encryption: ipsec.EncryptionAES128GCM}
	configured := ipsec.ESPGroup{Name: "two", Lifetime: 3600, Proposals: []ipsec.ESPProposal{first, second}}
	negotiated := configured
	negotiated.Proposals = []ipsec.ESPProposal{second}

	up := &PeerSession{
		peerName: "site-b",
		peerCfg:  ipsec.SiteToSitePeer{Name: "site-b", RemoteAddress: "203.0.113.9", LocalAddress: "192.0.2.1"},
		espGroup: configured,
	}
	up.setChildSA(&ChildSA{
		InboundSPI:  0x1001,
		OutboundSPI: 0x2002,
		LocalAddr:   net.ParseIP("192.0.2.1"),
		RemoteAddr:  net.ParseIP("198.51.100.7"),
		IfID:        42,
		ESPGroup:    negotiated,
		Mode:        modeTunnel,
		UDPEncap:    true,
	})
	down := &PeerSession{
		peerName: "site-a",
		peerCfg:  ipsec.SiteToSitePeer{Name: "site-a", RemoteAddress: "any"},
		espGroup: configured,
	}
	previous := ActivePeers()
	SetActivePeersForTest(map[string]*PeerSession{"site-b": up, "site-a": down})
	t.Cleanup(func() { SetActivePeersForTest(previous) })

	tunnels, err := ipsecinventory.Tunnels()
	if err != nil {
		t.Fatalf("the engine is linked and the inventory answered: %v", err)
	}
	if len(tunnels) != 2 {
		t.Fatalf("got %d tunnels, want 2 (one up, one down): %+v", len(tunnels), tunnels)
	}

	want := ipsecinventory.Tunnel{
		Peer:              "site-b",
		ConfiguredRemote:  netip.MustParseAddr("203.0.113.9"),
		Up:                true,
		InstalledRemote:   netip.MustParseAddr("198.51.100.7"),
		InstalledLocal:    netip.MustParseAddr("192.0.2.1"),
		IfID:              42,
		UDPEncap:          true,
		Mode:              ipsecinventory.ModeTunnel,
		EncryptionName:    "aes128gcm",
		IntegrityName:     "none",
		Encryption:        ipsecinventory.EncryptionID(crypto.ENCR_AES_GCM_16),
		EncryptionKeyBits: 128,
		Integrity:         ipsecinventory.IntegrityID(crypto.AUTH_NONE),
	}
	if tunnels[1] != want {
		t.Errorf("up tunnel:\n got %+v\nwant %+v", tunnels[1], want)
	}

	wantDown := ipsecinventory.Tunnel{Peer: "site-a"}
	if tunnels[0] != wantDown {
		t.Errorf("down tunnel with a remote of any:\n got %+v\nwant %+v", tunnels[0], wantDown)
	}
	if tunnels[0].ConfiguredRemote.IsValid() {
		t.Error("a remote of any was reported as an address")
	}
}
