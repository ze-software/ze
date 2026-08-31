package reactor

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/network"
)

// RFC 2385 leaves the digest to the kernel and leaves the DECISION to sign to
// the application. In ze that decision is the operator's config, so the tests
// below drive `connection { md5 { password; } }` to the two sockets a peering
// uses: the dialer ze connects with and the listener ze answers on.

// rfc2385PeerTree is one peer's config tree, with an md5 password when one is
// given and none at all when the password is empty.
func rfc2385PeerTree(password string) map[string]any {
	connection := map[string]any{
		"remote": map[string]any{"ip": "10.0.0.1"},
		"local":  map[string]any{"ip": "auto"},
	}
	if password != "" {
		connection["md5"] = map[string]any{"password": password}
	}
	return map[string]any{
		"connection": connection,
		"session":    map[string]any{"asn": map[string]any{"remote": "65001"}},
	}
}

// TestRFC2385ConfiguredKeyReachesBothSockets drives one configured password to
// the dialer ze connects with and to the key set the listener installs.
//
// VALIDATES: the operator's config is what turns signing on, and it reaches
// both the outbound and the inbound socket for the same peer.
// PREVENTS: a peering signed in one direction only, which establishes when ze
// dials and is dropped when the peer does.
// RFC requirement: RFC2385-2.0-5 positive -- signing is under the application's
// control: a peer carrying `md5 { password }` gets that key on the dialer
// (NewSession) and in the listener's key set (md5PeersForListener), with the
// peer's own address as the key's address.
func TestRFC2385ConfiguredKeyReachesBothSockets(t *testing.T) {
	const password = "rfc2385-configured-key"

	settings, err := parsePeerFromTree("peer1", rfc2385PeerTree(password), 65000, 0)
	if err != nil {
		t.Fatalf("parse a peer carrying an md5 password: %v", err)
	}
	if settings.MD5Key != password {
		t.Fatalf("PeerSettings.MD5Key = %q, want %q", settings.MD5Key, password)
	}

	session := NewSession(settings)
	dialer, isReal := session.dialer.(*network.RealDialer)
	if !isReal {
		t.Fatalf("session dialer is %T, want *network.RealDialer", session.dialer)
	}
	if dialer.MD5Key != password {
		t.Errorf("dialer MD5Key = %q, want %q", dialer.MD5Key, password)
	}
	if !dialer.PeerAddr.Equal(net.IP(settings.Address.AsSlice())) {
		t.Errorf("dialer PeerAddr = %v, want %v", dialer.PeerAddr, settings.Address)
	}

	reactor := New(&Config{Port: DefaultBGPPort})
	if err := reactor.AddPeer(settings); err != nil {
		t.Fatalf("add the peer: %v", err)
	}
	peers := reactor.md5PeersForListener(DefaultBGPPort)
	if len(peers) != 1 {
		t.Fatalf("listener key set holds %d peer(s), want 1", len(peers))
	}
	if peers[0].Key != password {
		t.Errorf("listener key = %q, want %q", peers[0].Key, password)
	}
	if !peers[0].Addr.Equal(net.IP(settings.Address.AsSlice())) {
		t.Errorf("listener key address = %v, want %v", peers[0].Addr, settings.Address)
	}
}

// TestRFC2385NoKeyWithoutConfiguration is the negative polarity: the same peer
// with no md5 stanza gets no key on either socket.
//
// VALIDATES: nothing installs a key the operator did not configure, so signing
// is the application's decision in both directions.
// PREVENTS: a key inherited from somewhere else being applied to a peer the
// operator left unsigned, which would drop that peer's session.
// RFC requirement: RFC2385-2.0-5 negative -- a peer with no `md5 { password }`
// gets no key on the dialer and contributes none to the listener's key set, so
// no signature is sent for a peering the application did not ask to protect.
func TestRFC2385NoKeyWithoutConfiguration(t *testing.T) {
	settings, err := parsePeerFromTree("peer1", rfc2385PeerTree(""), 65000, 0)
	if err != nil {
		t.Fatalf("parse a peer carrying no md5 password: %v", err)
	}
	if settings.MD5Key != "" {
		t.Fatalf("PeerSettings.MD5Key = %q, want empty", settings.MD5Key)
	}

	session := NewSession(settings)
	dialer, isReal := session.dialer.(*network.RealDialer)
	if !isReal {
		t.Fatalf("session dialer is %T, want *network.RealDialer", session.dialer)
	}
	if dialer.MD5Key != "" {
		t.Errorf("dialer MD5Key = %q, want empty", dialer.MD5Key)
	}
	if dialer.PeerAddr != nil {
		t.Errorf("dialer PeerAddr = %v, want none", dialer.PeerAddr)
	}

	reactor := New(&Config{Port: DefaultBGPPort})
	if err := reactor.AddPeer(settings); err != nil {
		t.Fatalf("add the peer: %v", err)
	}
	if peers := reactor.md5PeersForListener(DefaultBGPPort); len(peers) != 0 {
		t.Errorf("listener key set holds %d peer(s), want none", len(peers))
	}
}

// TestRFC2385FailedConnectDoesNotDisableSigning dials a port nobody listens on
// and reads the session's dialer afterwards.
//
// A connection ze signs is never answered by a peer that does not hold the key,
// so a failed attempt is exactly the moment an implementation could be tempted
// to retry unsigned. Ze holds the key on the dialer for the peer's whole life
// and rebuilds it from PeerSettings, so nothing the far end does removes it.
//
// VALIDATES: the key survives a connection attempt that failed.
// PREVENTS: a silent downgrade to an unsigned retry after a peer refuses or
// ignores a signed SYN.
// RFC requirement: RFC2385-2.0-4 positive -- a failed connection attempt, which
// is what the absence of a signed answer looks like to the dialer, leaves the
// key in place: the next attempt is signed with the same key and the same peer
// address.
func TestRFC2385FailedConnectDoesNotDisableSigning(t *testing.T) {
	const password = "rfc2385-retry-key"

	closed, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("open a listener to borrow a port from: %v", err)
	}
	address := closed.Addr().(*net.TCPAddr)
	if err := closed.Close(); err != nil {
		t.Fatalf("close the borrowed listener: %v", err)
	}

	settings := NewPeerSettings(mustParseAddr("127.0.0.1"), 65000, 65001, 0x01010101)
	settings.MD5Key = password
	settings.Port = uint16(address.Port)

	session := NewSession(settings)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := session.Connect(ctx); err == nil {
		t.Fatal("the dial to a closed port succeeded")
	}

	dialer, isReal := session.dialer.(*network.RealDialer)
	if !isReal {
		t.Fatalf("session dialer is %T, want *network.RealDialer", session.dialer)
	}
	if dialer.MD5Key != password {
		t.Errorf("after a failed attempt the dialer MD5Key = %q, want %q", dialer.MD5Key, password)
	}
	if !dialer.PeerAddr.Equal(net.IP(settings.Address.AsSlice())) {
		t.Errorf("after a failed attempt the dialer PeerAddr = %v, want %v",
			dialer.PeerAddr, settings.Address)
	}
}
