// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- In-session interface changes.
// Related: pppoe_changes_test.go -- changeBackend and the change session harness.
// Related: pppoe_client_dns_test.go -- testDHCPv4Origin and assertServers.

package iface

import (
	"io"
	"log/slog"
	"net/netip"
	"reflect"
	"slices"
	"strings"
	"testing"
	"testing/synctest"

	"github.com/ze-software/ze/internal/component/config/system"
	"github.com/ze-software/ze/internal/core/rtproto"
)

// linkLocalChangeBackend adds the link-local install to changeBackend, so
// a session result that carries an IPv6CP link-local can be applied. The
// link-local removal stays recorded as changeBackend's RemoveAddress.
type linkLocalChangeBackend struct {
	*changeBackend
}

func (b linkLocalChangeBackend) AddAddress(device, cidr string) error {
	return b.record(changeCall{op: "link-local-add", device: device, local: cidr})
}

// TestPPPoEClientWithdrawsIPv4WhileIPv6CPHolds drives runSession through a
// complete result, one without IPv4 that keeps the IPv6CP link-local, and a
// complete one again, and checks the exact backend calls, the resolv.conf
// lease and the status after each.
//
// VALIDATES: RFC 7084 WLL-3 at the consumer: IPCP Stopped while IPv6CP holds
// the session removes the default route, then the address pair, and
// withdraws the DNS lease, with no install of an invalid prefix and the
// link-local untouched; IPCP reopening installs the pair and route again.
// PREVENTS: AddAddressP2P of "invalid Prefix" ending the session, and a
// RemoveAddressP2P of the empty pair when IPCP reopens.
func TestPPPoEClientWithdrawsIPv4WhileIPv6CPHolds(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		local := netip.MustParseAddr("192.0.2.2")
		peer := netip.MustParseAddr("192.0.2.1")
		dns := netip.MustParseAddr("192.0.2.53")
		linkLocal := netip.MustParseAddr("fe80::2")

		recorder := &changeBackend{}
		resolvers := &system.Resolvers{}
		if err := resolvers.SetLease(testDHCPv4Origin.Source("eth0", "0"), []string{"198.51.100.1"}, system.UntilWithdrawn); err != nil {
			t.Fatal(err)
		}
		changes := make(chan PPPoEResult, 1)
		ended := make(chan error, 1)
		sess := PPPoESession{
			SessionID: 42, UnitNum: 7, NegMTU: 1492,
			LocalIP: local, PeerIP: peer, PrimaryDNS: dns,
			Changes: changes, Done: make(chan struct{}),
			Cleanup: func() {},
		}
		cfg := PPPoEClientConfig{Name: "wan", RoutePriority: 73, Timing: testPPPoETiming}
		client := NewPPPoEClient(cfg, &fakeDialer{sess: sess}, linkLocalChangeBackend{recorder}, slog.New(slog.NewTextHandler(io.Discard, nil)))
		client.resolvers = resolvers
		go func() { ended <- client.runSession() }()
		synctest.Wait()
		defer func() {
			close(client.stopCh)
			synctest.Wait()
		}()

		address := func(op string) changeCall {
			return changeCall{op: op, device: "wan", local: "192.0.2.2/32", peer: "192.0.2.1/32"}
		}
		route := func(op string) changeCall {
			return changeCall{op: op, device: "wan", local: "0.0.0.0/0", peer: "192.0.2.1", metric: 73, proto: rtproto.Iface}
		}
		full := PPPoEResult{LocalIP: local, PeerIP: peer, NegMTU: 1492, PrimaryDNS: dns, LinkLocal: linkLocal}
		withdrawn := PPPoEResult{NegMTU: 1492, LinkLocal: linkLocal}

		steps := []struct {
			name    string
			result  PPPoEResult
			calls   []changeCall
			servers []string
		}{
			{"ipv6cp-opens", full, []changeCall{
				{op: "link-local-add", device: "wan", local: pppoeLinkLocalPrefix(linkLocal)},
			}, []string{"198.51.100.1", "192.0.2.53"}},
			{"ipcp-stopped", withdrawn, []changeCall{
				route("route-remove"), address("address-remove"),
			}, []string{"198.51.100.1"}},
			{"ipcp-reopens", full, []changeCall{
				address("address-add"), route("route-add"),
			}, []string{"198.51.100.1", "192.0.2.53"}},
		}
		for _, step := range steps {
			recorder.calls = nil
			changes <- step.result
			synctest.Wait()
			if len(ended) != 0 {
				t.Fatalf("%s: runSession ended: %v", step.name, <-ended)
			}
			for _, call := range recorder.calls {
				if strings.Contains(call.local, "invalid") {
					t.Errorf("%s: backend called with an invalid prefix: %+v", step.name, call)
				}
				if strings.Contains(call.peer, "invalid") {
					t.Errorf("%s: backend called with an invalid prefix: %+v", step.name, call)
				}
			}
			if !reflect.DeepEqual(recorder.calls, step.calls) {
				t.Errorf("%s: calls = %+v, want %+v", step.name, recorder.calls, step.calls)
			}
			assertServers(t, resolvers, step.servers...)
			status := client.status()
			if status.LocalIP != step.result.LocalIP {
				t.Errorf("%s: status local = %s, want %s", step.name, status.LocalIP, step.result.LocalIP)
			}
			if status.PeerIP != step.result.PeerIP {
				t.Errorf("%s: status peer = %s, want %s", step.name, status.PeerIP, step.result.PeerIP)
			}
			if want := step.servers[1:]; !slices.Equal(status.DNSServers, want) {
				t.Errorf("%s: status dns-servers = %v, want %v", step.name, status.DNSServers, want)
			}
			if status.State != "up" {
				t.Errorf("%s: session no longer up: %+v", step.name, status)
			}
		}
	})
}
