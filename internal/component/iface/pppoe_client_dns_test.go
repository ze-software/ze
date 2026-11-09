// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- DNS servers learned from IPCP.
// Related: pppoe_changes_test.go -- changeBackend and the change session harness.

package iface

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/netip"
	"slices"
	"strings"
	"sync"
	"testing"
	"testing/synctest"

	"github.com/ze-software/ze/internal/component/config/system"
)

// The iface unit test binary links no DHCP client, so it registers a DHCPv4
// stand-in that ranks before the PPP origin as DHCPv4 does. It takes rank 101,
// not the DHCP client's 100: the integration tests link the real client
// (dhcp_v6_selector_renew_integration_linux_test.go), and a shared rank panics.
var testDHCPv4Origin = system.RegisterResolverOrigin("test-dhcpv4", 101)

type dnsSession struct {
	client    *PPPoEClient
	backend   *changeBackend
	resolvers *system.Resolvers
	changes   chan PPPoEResult
	done      chan struct{}
	ended     chan error
}

// newDNSSession runs a session that learned primary and secondary, beside a
// DHCPv4 lease on eth0 that the session MUST leave alone.
func newDNSSession(t *testing.T, primary, secondary string) *dnsSession {
	t.Helper()
	h := &dnsSession{
		backend:   &changeBackend{},
		resolvers: &system.Resolvers{},
		changes:   make(chan PPPoEResult, 1),
		done:      make(chan struct{}),
		ended:     make(chan error, 1),
	}
	if err := h.resolvers.SetLease(testDHCPv4Origin.Source("eth0", "0"), []string{"198.51.100.1"}, system.UntilWithdrawn); err != nil {
		t.Fatal(err)
	}
	sess := PPPoESession{
		SessionID: 42, UnitNum: 7, NegMTU: 1492,
		LocalIP: netip.MustParseAddr("192.0.2.2"),
		PeerIP:  netip.MustParseAddr("192.0.2.1"),
		Changes: h.changes, Done: h.done,
		Cleanup: func() {},
	}
	if primary != "" {
		sess.PrimaryDNS = netip.MustParseAddr(primary)
	}
	if secondary != "" {
		sess.SecondaryDNS = netip.MustParseAddr(secondary)
	}
	cfg := PPPoEClientConfig{Name: "wan", RoutePriority: 73, Timing: testPPPoETiming}
	h.client = NewPPPoEClient(cfg, &fakeDialer{sess: sess}, h.backend, slog.New(slog.NewTextHandler(io.Discard, nil)))
	h.client.resolvers = h.resolvers
	go func() { h.ended <- h.client.runSession() }()
	synctest.Wait()
	h.backend.calls = nil
	return h
}

func (h *dnsSession) stop() {
	close(h.client.stopCh)
	synctest.Wait()
}

func assertServers(t *testing.T, r *system.Resolvers, want ...string) {
	t.Helper()
	if got := r.Servers(); !slices.Equal(got, want) {
		t.Fatalf("resolv.conf servers = %v, want %v", got, want)
	}
}

// TestPPPoEClientSetsResolverLease: the servers IPCP learned reach the
// resolv.conf writer after the DHCP lease, primary first, and leave it when
// the session ends, while the DHCP lease stays.
//
// VALIDATES: AC-6 and AC-8 through runSession with a fake dialer.
// PREVENTS: learned servers never written, or left behind after WAN loss.
func TestPPPoEClientSetsResolverLease(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newDNSSession(t, "192.0.2.53", "192.0.2.54")
		assertServers(t, h.resolvers, "198.51.100.1", "192.0.2.53", "192.0.2.54")
		if got := h.client.status().DNSServers; !slices.Equal(got, []string{"192.0.2.53", "192.0.2.54"}) {
			t.Fatalf("status dns-servers = %v", got)
		}
		close(h.done)
		synctest.Wait()
		if err := <-h.ended; err == nil {
			t.Fatal("a lost session ended runSession with no error")
		}
		assertServers(t, h.resolvers, "198.51.100.1")
	})
}

// TestPPPoEClientLeaseNamesConfiguredInterface: the record IPCP's servers
// make carries the pppoe-client's configured name, the name a
// `received-interface` entry holds, and not the session device or the
// source interface.
//
// VALIDATES: spec-home-cpe-23-dns-source-fall-through A-8, the PPPoE half.
// PREVENTS: `received-interface pppoe0` never selecting the session's servers.
func TestPPPoEClientLeaseNamesConfiguredInterface(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newDNSSession(t, "192.0.2.53", "")
		defer h.stop()
		var names []string
		for _, rec := range h.resolvers.Snapshot().Records {
			if slices.Contains(rec.Addresses, netip.MustParseAddr("192.0.2.53")) {
				names = append(names, rec.Interface)
			}
		}
		if !slices.Equal(names, []string{"wan"}) {
			t.Fatalf("records holding the IPCP server name %v, want [wan]", names)
		}
	})
}

// TestPPPoEClientReplacesResolverLeaseOnDNSOnlyChange: a renegotiation that
// keeps the addresses and MTU but learns new servers replaces the lease and
// touches no address, route or MTU.
//
// VALIDATES: AC-12.
// PREVENTS: stale servers after the access concentrator changes them.
func TestPPPoEClientReplacesResolverLeaseOnDNSOnlyChange(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newDNSSession(t, "192.0.2.53", "192.0.2.54")
		defer h.stop()
		next := changedResult("192.0.2.2", "192.0.2.1", 1492)
		next.PrimaryDNS = netip.MustParseAddr("192.0.2.63")
		next.SecondaryDNS = netip.MustParseAddr("192.0.2.64")
		h.changes <- next
		synctest.Wait()
		if len(h.backend.calls) != 0 {
			t.Fatalf("a DNS-only change called the backend: %+v", h.backend.calls)
		}
		assertServers(t, h.resolvers, "198.51.100.1", "192.0.2.63", "192.0.2.64")
	})
}

// TestPPPoEClientWithdrawsLeaseWhenRenegotiationDropsDNS: a renegotiation
// whose server Rejected both DNS options withdraws the PPP lease.
//
// VALIDATES: AC-13.
// PREVENTS: servers the access concentrator no longer offers left in use.
func TestPPPoEClientWithdrawsLeaseWhenRenegotiationDropsDNS(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newDNSSession(t, "192.0.2.53", "192.0.2.54")
		defer h.stop()
		h.changes <- changedResult("192.0.2.2", "192.0.2.1", 1492)
		synctest.Wait()
		assertServers(t, h.resolvers, "198.51.100.1")
		if got := h.client.status().DNSServers; len(got) != 0 {
			t.Fatalf("status dns-servers = %v after withdrawal", got)
		}
	})
}

// TestShowInterfacePPPoEClientDNSServers: the plugin command answers each
// client's object with dns-servers primary first, and an empty list, never an
// absent key, when none were learned.
//
// VALIDATES: AC-11 at the iface plugin's command answer.
// PREVENTS: an operator unable to see which resolvers the WAN supplied.
func TestShowInterfacePPPoEClientDNSServers(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newDNSSession(t, "192.0.2.53", "192.0.2.54")
		defer h.stop()
		idle := NewPPPoEClient(PPPoEClientConfig{Name: "backup", SourceInterface: "eth2"}, &fakeDialer{}, nil,
			slog.New(slog.NewTextHandler(io.Discard, nil)))
		var mu sync.Mutex
		saved := globalPPPoEClients.Load()
		globalPPPoEClients.Store(&pppoeClientSet{mu: &mu, active: map[string]*PPPoEClient{"wan": h.client, "backup": idle}})
		defer globalPPPoEClients.Store(saved)

		status, data, err := executeCommand(PPPoEClientCommand, nil)
		if err != nil {
			t.Fatalf("%s: %v", PPPoEClientCommand, err)
		}
		if status != "done" {
			t.Fatalf("status = %q", status)
		}
		raw, err := json.Marshal(data)
		if err != nil {
			t.Fatal(err)
		}
		var clients []map[string]any
		if err := json.Unmarshal(raw, &clients); err != nil {
			t.Fatal(err)
		}
		if len(clients) != 2 {
			t.Fatalf("answer = %s, want two clients", raw)
		}
		if clients[0]["name"] != "backup" {
			t.Fatalf("answer not sorted by name: %s", raw)
		}
		if !strings.Contains(string(raw), `"dns-servers":[]`) {
			t.Fatalf("idle client has no empty dns-servers list: %s", raw)
		}
		if !strings.Contains(string(raw), `"dns-servers":["192.0.2.53","192.0.2.54"]`) {
			t.Fatalf("up client dns-servers not primary first: %s", raw)
		}
	})
}
