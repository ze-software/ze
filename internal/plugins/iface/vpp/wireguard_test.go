package ifacevpp

import (
	"strings"
	"testing"

	"go.fd.io/govpp/binapi/wireguard"

	"codeberg.org/thomas-mangin/ze/internal/component/iface"
)

func newWireguardBackend(ch *progChannel) *vppBackendImpl {
	b := &vppBackendImpl{ch: ch, names: newNameMap()}
	b.populate.Do(func() {}) // short-circuit ensureChannel populate
	return b
}

func wgKey(seed byte) iface.WireguardKey {
	var k iface.WireguardKey
	for i := range k {
		k[i] = seed
	}
	return k
}

// TestConfigureWireguardCreatesInterface verifies AC-5: configuring a wireguard
// device issues wireguard_interface_create carrying the private key and listen
// port, and registers the name->SwIfIndex mapping.
// VALIDATES: AC-5 -- wireguard interface programmed via binapi.
// PREVENTS: regression to the errNotSupported stub.
func TestConfigureWireguardCreatesInterface(t *testing.T) {
	ch := &progChannel{swIfIndex: 11}
	b := newWireguardBackend(ch)

	// CreateWireguardDevice is a no-op on VPP (key comes at Configure time).
	if err := b.CreateWireguardDevice("wg0"); err != nil {
		t.Fatalf("CreateWireguardDevice: %v", err)
	}
	if len(ch.requests) != 0 {
		t.Errorf("CreateWireguardDevice should issue no VPP request, got %d", len(ch.requests))
	}

	spec := iface.WireguardSpec{Name: "wg0", PrivateKey: wgKey(0x11), ListenPort: 51820, ListenPortSet: true}
	if err := b.ConfigureWireguardDevice(spec); err != nil {
		t.Fatalf("ConfigureWireguardDevice: %v", err)
	}
	create, ok := ch.requests[0].(*wireguard.WireguardInterfaceCreate)
	if !ok {
		t.Fatalf("request[0] type: got %T, want *wireguard.WireguardInterfaceCreate", ch.requests[0])
	}
	if create.Interface.Port != 51820 {
		t.Errorf("Port: got %d, want 51820", create.Interface.Port)
	}
	if len(create.Interface.PrivateKey) != 32 || create.Interface.PrivateKey[0] != 0x11 {
		t.Errorf("PrivateKey not carried into create request")
	}
	if idx, ok := b.names.LookupIndex("wg0"); !ok || idx != 11 {
		t.Errorf("name map: got (%d,%v), want (11,true)", idx, ok)
	}
}

// TestConfigureWireguardAddsPeers verifies peers are programmed via
// wireguard_peer_add with public key, endpoint, and allowed-ips.
// VALIDATES: AC-5 -- peer set programmed.
func TestConfigureWireguardAddsPeers(t *testing.T) {
	ch := &progChannel{swIfIndex: 3, peerIndex: 7}
	b := newWireguardBackend(ch)

	spec := iface.WireguardSpec{
		Name:       "wg0",
		PrivateKey: wgKey(0x22),
		Peers: []iface.WireguardPeerSpec{{
			Name:         "peerA",
			PublicKey:    wgKey(0x33),
			EndpointIP:   "192.0.2.50",
			EndpointPort: 51820,
			AllowedIPs:   []string{"10.0.0.0/24"},
		}},
	}
	if err := b.ConfigureWireguardDevice(spec); err != nil {
		t.Fatalf("ConfigureWireguardDevice: %v", err)
	}
	var add *wireguard.WireguardPeerAdd
	for _, r := range ch.requests {
		if pa, ok := r.(*wireguard.WireguardPeerAdd); ok {
			add = pa
		}
	}
	if add == nil {
		t.Fatal("no WireguardPeerAdd issued")
	}
	if got := add.Peer.Endpoint.ToIP().String(); got != "192.0.2.50" {
		t.Errorf("Endpoint: got %s, want 192.0.2.50", got)
	}
	if add.Peer.NAllowedIps != 1 || len(add.Peer.AllowedIps) != 1 {
		t.Errorf("AllowedIps: got n=%d len=%d, want 1/1", add.Peer.NAllowedIps, len(add.Peer.AllowedIps))
	}
	if got := add.Peer.AllowedIps[0].String(); !strings.HasPrefix(got, "10.0.0.0/24") {
		t.Errorf("AllowedIps[0]: got %s, want 10.0.0.0/24", got)
	}
}

// TestConfigureWireguardRejectsPresharedKey verifies the honest exact-or-reject:
// this VPP wireguard API revision has no preshared-key field, so a PSK is
// rejected rather than silently dropped.
// VALIDATES: AC-5 -- no silent PSK drop.
func TestConfigureWireguardRejectsPresharedKey(t *testing.T) {
	ch := &progChannel{swIfIndex: 1}
	b := newWireguardBackend(ch)
	spec := iface.WireguardSpec{
		Name:       "wg0",
		PrivateKey: wgKey(0x44),
		Peers: []iface.WireguardPeerSpec{{
			Name:            "peerA",
			PublicKey:       wgKey(0x55),
			HasPresharedKey: true,
			PresharedKey:    wgKey(0x66),
		}},
	}
	err := b.ConfigureWireguardDevice(spec)
	if err == nil {
		t.Fatal("expected error for preshared key on VPP backend, got nil")
	}
	if !strings.Contains(err.Error(), "preshared") {
		t.Errorf("expected 'preshared' in error, got: %v", err)
	}
}

// TestConfigureWireguardReplacesPeers verifies ReplacePeers semantics: a second
// Configure removes the peer indices installed by the first before adding the
// new set. VALIDATES: AC-5 -- peer reconciliation.
func TestConfigureWireguardReplacesPeers(t *testing.T) {
	ch := &progChannel{swIfIndex: 5, peerIndex: 9}
	b := newWireguardBackend(ch)

	base := iface.WireguardSpec{
		Name:       "wg0",
		PrivateKey: wgKey(0x77),
		Peers:      []iface.WireguardPeerSpec{{Name: "p1", PublicKey: wgKey(0x01)}},
	}
	if err := b.ConfigureWireguardDevice(base); err != nil {
		t.Fatalf("first configure: %v", err)
	}
	// Second apply with a different peer set must remove peer index 9 first.
	ch.requests = nil
	next := iface.WireguardSpec{
		Name:       "wg0",
		PrivateKey: wgKey(0x77),
		Peers:      []iface.WireguardPeerSpec{{Name: "p2", PublicKey: wgKey(0x02)}},
	}
	if err := b.ConfigureWireguardDevice(next); err != nil {
		t.Fatalf("second configure: %v", err)
	}
	var removed *wireguard.WireguardPeerRemove
	for _, r := range ch.requests {
		if pr, ok := r.(*wireguard.WireguardPeerRemove); ok {
			removed = pr
		}
	}
	if removed == nil {
		t.Fatal("second configure did not remove the previously installed peer")
	}
	if removed.PeerIndex != 9 {
		t.Errorf("removed PeerIndex: got %d, want 9", removed.PeerIndex)
	}
}

// TestGetWireguardRoundTrip verifies GetWireguardDevice reads back the interface
// key/port and peer set from the dump replies.
// VALIDATES: AC-5 -- GetWireguardDevice round-trips the spec.
func TestGetWireguardRoundTrip(t *testing.T) {
	ch := &progChannel{}
	b := newWireguardBackend(ch)
	b.names.Add("wg0", 8, "wg0")

	priv := wgKey(0x88)
	pub := wgKey(0x99)
	ch.wgIfaceDetails = []wireguard.WireguardInterfaceDetails{{
		Interface: wireguard.WireguardInterface{SwIfIndex: 8, PrivateKey: priv[:], Port: 51821},
	}}
	ch.wgPeerDetails = []wireguard.WireguardPeersDetails{{
		Peer: wireguard.WireguardPeer{SwIfIndex: 8, PublicKey: pub[:], Port: 4444},
	}}

	got, err := b.GetWireguardDevice("wg0")
	if err != nil {
		t.Fatalf("GetWireguardDevice: %v", err)
	}
	if got.ListenPort != 51821 || !got.ListenPortSet {
		t.Errorf("ListenPort: got %d/%v, want 51821/true", got.ListenPort, got.ListenPortSet)
	}
	if got.PrivateKey != priv {
		t.Errorf("PrivateKey did not round-trip")
	}
	if len(got.Peers) != 1 {
		t.Fatalf("Peers: got %d, want 1", len(got.Peers))
	}
	if got.Peers[0].PublicKey != pub {
		t.Errorf("peer PublicKey did not round-trip")
	}
	if got.Peers[0].EndpointPort != 4444 {
		t.Errorf("peer EndpointPort: got %d, want 4444", got.Peers[0].EndpointPort)
	}
}

// TestDeleteWireguardInterface verifies DeleteInterface issues
// wireguard_interface_delete and clears the name map.
// VALIDATES: AC-5 -- clean wireguard delete path.
func TestDeleteWireguardInterface(t *testing.T) {
	ch := &progChannel{swIfIndex: 6}
	b := newWireguardBackend(ch)
	if err := b.ConfigureWireguardDevice(iface.WireguardSpec{Name: "wg0", PrivateKey: wgKey(0xAA)}); err != nil {
		t.Fatalf("ConfigureWireguardDevice: %v", err)
	}
	if err := b.DeleteInterface("wg0"); err != nil {
		t.Fatalf("DeleteInterface: %v", err)
	}
	var del *wireguard.WireguardInterfaceDelete
	for _, r := range ch.requests {
		if d, ok := r.(*wireguard.WireguardInterfaceDelete); ok {
			del = d
		}
	}
	if del == nil {
		t.Fatal("no WireguardInterfaceDelete issued")
	}
	if _, ok := b.names.LookupIndex("wg0"); ok {
		t.Error("name map still has wg0 after delete")
	}
}
