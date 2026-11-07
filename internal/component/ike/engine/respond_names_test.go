package engine

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/netip"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// respondByName is a `respond` peer whose remote-address is a DNS name.
func respondByName(name string) ipsec.SiteToSitePeer {
	peer := testPeer()
	peer.Name = "r"
	peer.ConnectionType = ipsec.ConnectionRespond
	peer.RemoteAddress = name
	return peer
}

// withLookups swaps the resolver for answers, counting the calls.
func withLookups(t *testing.T, answer func(name string) ([]netip.Addr, error)) *atomic.Int32 {
	t.Helper()
	var calls atomic.Int32
	previous := lookupNameFn
	lookupNameFn = func(_ context.Context, name string) ([]netip.Addr, error) {
		calls.Add(1)
		return answer(name)
	}
	t.Cleanup(func() { lookupNameFn = previous })
	return &calls
}

// withRespondPeer publishes a running `respond` session for peer and a fresh cache.
func withRespondPeer(t *testing.T, peer ipsec.SiteToSitePeer) {
	t.Helper()
	previousCache := respondNames
	respondNames = &respondNameCache{entries: map[string]respondNameEntry{}}
	SetActivePeersForTest(map[string]*PeerSession{peer.Name: {peerName: peer.Name, peerCfg: peer}})
	t.Cleanup(func() {
		SetActivePeersForTest(nil)
		respondNames = previousCache
	})
}

func from(ip string) *net.UDPAddr { return &net.UDPAddr{IP: net.ParseIP(ip), Port: 500} }

// VALIDATES: AC-16: a `respond` peer configured by name accepts an unsolicited IKE_SA_INIT
// from any address the name last resolved to, refuses any other source, and the packet
// path makes no DNS query.
//
// RED before the fix: matchResponderPeer compared the name to the source address text, so
// no datagram ever matched a peer configured by name.
func TestRespondPeerMatchesResolvedName(t *testing.T) {
	calls := withLookups(t, func(string) ([]netip.Addr, error) {
		return []netip.Addr{netip.MustParseAddr("198.51.100.7"), netip.MustParseAddr("198.51.100.8")}, nil
	})
	peer := respondByName("vpn.example.net")
	withRespondPeer(t, peer)

	cfg := testIPsecConfig(peer)
	if matchResponderPeer(from("198.51.100.7")) != nil {
		t.Fatal("matched before any resolution answered")
	}
	resolveRespondNames(respondNames.sync(cfg), nil, slog.Default())
	resolved := calls.Load()

	for _, src := range []string{"198.51.100.7", "198.51.100.8"} {
		if matchResponderPeer(from(src)) == nil {
			t.Fatalf("a datagram from %s, an address the name resolved to, matched nothing", src)
		}
	}
	if matchResponderPeer(from("203.0.113.66")) != nil {
		t.Fatal("a datagram from an address the name did not resolve to matched the peer")
	}
	if calls.Load() != resolved {
		t.Fatal("the packet path queried DNS")
	}
}

// VALIDATES: AC-19 and AC-20: a failed refresh keeps the previous set, a changed answer
// replaces it (the old address refused from then on), and a removed peer loses its entry.
func TestRespondNameCacheLifecycle(t *testing.T) {
	answer := []netip.Addr{netip.MustParseAddr("198.51.100.7")}
	var fail atomic.Bool
	withLookups(t, func(string) ([]netip.Addr, error) {
		if fail.Load() {
			return nil, errors.New("server misbehaving")
		}
		return answer, nil
	})
	peer := respondByName("vpn.example.net")
	withRespondPeer(t, peer)
	cfg := testIPsecConfig(peer)
	resolveRespondNames(respondNames.sync(cfg), nil, slog.Default())

	fail.Store(true)
	resolveRespondNames(respondNames.queries(), nil, slog.Default())
	if matchResponderPeer(from("198.51.100.7")) == nil {
		t.Fatal("a failed refresh dropped the addresses the name resolved to before")
	}

	fail.Store(false)
	answer = []netip.Addr{netip.MustParseAddr("198.51.100.9")}
	resolveRespondNames(respondNames.queries(), nil, slog.Default())
	if matchResponderPeer(from("198.51.100.7")) != nil {
		t.Fatal("the old address still matches after the name's answer changed")
	}
	if matchResponderPeer(from("198.51.100.9")) == nil {
		t.Fatal("the new address does not match")
	}

	respondNames.sync(testIPsecConfig())
	if q := respondNames.queries(); len(q) != 0 {
		t.Fatalf("a removed peer kept its entry: %v", q)
	}
}

// VALIDATES: AC-22: the doctor check ipsec-interface-address names each waiting peer, its
// interface and the fix, and says nothing when no peer waits.
func TestDoctorIPsecInterfaceAddressWaiting(t *testing.T) {
	previous := interfaceWaitPtr.Load()
	t.Cleanup(func() { interfaceWaitPtr.Store(previous) })

	interfaceWaitPtr.Store(&interfaceWait{iface: "pppoe0", peers: []string{"office"}})
	got := checkIPsecInterfaceAddress(registry.DoctorCheckContext{})
	if len(got) != 1 {
		t.Fatalf("diagnostics %+v, want one", got)
	}
	if got[0].Code != diagnostic.CodeDoctorIPsecInterfaceAddress {
		t.Fatalf("code %q", got[0].Code)
	}
	for _, want := range []string{"office", "pppoe0", "local-address", "Bring the interface up"} {
		if !strings.Contains(got[0].Message, want) {
			t.Fatalf("message %q does not name %q", got[0].Message, want)
		}
	}

	interfaceWaitPtr.Store(&interfaceWait{iface: "pppoe0"})
	if got := checkIPsecInterfaceAddress(registry.DoctorCheckContext{}); len(got) != 0 {
		t.Fatalf("diagnostics with no waiting peer: %+v", got)
	}
}
