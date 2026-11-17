// Design: docs/research/l2tpv2-ze-integration.md -- IPv6 factory prefix allocation

package ppp

import (
	"context"
	"log/slog"
	"net/netip"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/rtproto"
)

type allocatorCarrierReply struct {
	payload []byte
	peer    netip.AddrPort
}

// Keep the existing RA/RS/close lifetime recorder; only the DHCPv6 lane
// carries actual request and response bytes for the factory-created worker.
type allocatorServiceCarrier struct {
	*serviceTestCarrier
	requests chan []byte
	replies  chan allocatorCarrierReply
	started  sync.Once
}

func (carrier *allocatorServiceCarrier) ReadDHCPv6(ctx context.Context, payload []byte) (int, netip.AddrPort, error) {
	carrier.started.Do(func() { close(carrier.dhcpStarted) })
	select {
	case request := <-carrier.requests:
		return copy(payload, request), netip.MustParseAddrPort("[fe80::2]:546"), nil
	case <-ctx.Done():
		carrier.record("dhcp-stopped")
		return 0, netip.AddrPort{}, ctx.Err()
	}
}

func (carrier *allocatorServiceCarrier) WriteDHCPv6(payload []byte, peer netip.AddrPort) (int, error) {
	carrier.replies <- allocatorCarrierReply{payload: append([]byte(nil), payload...), peer: peer}
	return len(payload), nil
}

// Exercise the optional allocator through the real service factory, worker
// and DHCPv6 wire. This is a factory-mode proof, not production pool wiring.
func TestIPv6ServiceStartUsesPrefixAllocator(t *testing.T) {
	backend := &fakeBackend{}
	handle := newTestSessionHandle(backend, "allocator-ppp")
	carrier := &allocatorServiceCarrier{
		serviceTestCarrier: newServiceCarrier(),
		requests:           make(chan []byte, 1), replies: make(chan allocatorCarrierReply, 4),
	}
	acquisitions := 0
	handle.openIPv6Carrier = func(context.Context) (IPv6Carrier, error) {
		acquisitions++
		installed := backend.P2PCalls()
		if len(installed) != 1 || installed[0].name != handle.Identity().InterfaceName {
			t.Fatal("factory acquired carrier before installing its original owner's link-local pair")
		}
		return carrier, nil
	}
	prefix := netip.MustParsePrefix("2001:db8:abcd::/48")
	allocated := 0
	allocator := func() (netip.Prefix, bool) {
		allocated++
		return prefix, true
	}
	var released []netip.Prefix
	cfg := serviceHandleConfig(handle)
	cfg.ReleasePrefix = func(prefix netip.Prefix) { released = append(released, prefix) }
	server := testServerID()
	service, err := startIPv6Service(cfg, server, allocator, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := service.Stop(); err != nil {
			t.Error(err)
		}
	})
	awaitServiceCarrier(t, carrier.rsStarted)
	awaitServiceCarrier(t, carrier.dhcpStarted)
	awaitServiceCarrier(t, carrier.advertised)
	if acquisitions != 1 {
		t.Fatalf("original carrier acquisitions = %d, want 1", acquisitions)
	}
	client := DHCPv6DUID{Type: DUIDTypeLL, HWType: 1, ID: []byte{2, 0, 0, 0, 0, 2}}
	exchange := func(request []byte, wantType uint8, transaction [3]byte) *dHCPv6Message {
		t.Helper()
		carrier.requests <- request
		select {
		case response := <-carrier.replies:
			if response.peer != netip.MustParseAddrPort("[fe80::2]:546") {
				t.Fatalf("DHCPv6 response destination = %v, want original request peer", response.peer)
			}
			reply, err := parseDHCPv6(response.payload)
			if err != nil {
				t.Fatal(err)
			}
			if reply.Type != wantType || reply.TransactionID != transaction || !duidEqual(reply.ServerID, &server) || !duidEqual(reply.ClientID, &client) {
				t.Fatalf("DHCPv6 response lost its original type, transaction or client/server: %+v", reply)
			}
			return reply
		case <-time.After(2 * time.Second):
			t.Fatal("factory-created DHCPv6 worker did not answer the actual request")
			return nil
		}
	}
	transaction := [3]byte{1, 2, 3}
	advertise := exchange(buildSolicit(transaction, client, 1), DHCPv6Advertise, transaction)
	if advertise.IAPD == nil || advertise.IAPD.Prefix == nil || advertise.IAPD.Prefix.Prefix != prefix || advertise.IAPD.IAID != 1 {
		t.Fatalf("Advertise did not carry the allocated prefix and IAID: %+v", advertise)
	}
	// A retransmitted Solicit keeps the first lease, not a second
	// callback result that could replace the cleanup key.
	again := exchange(buildSolicit(transaction, client, 1), DHCPv6Advertise, transaction)
	if again.IAPD == nil || again.IAPD.Prefix == nil || again.IAPD.Prefix.Prefix != prefix {
		t.Fatal("Solicit retransmission lost the original allocated prefix")
	}
	transaction = [3]byte{1, 2, 4}
	request := buildRenew(transaction, client, server, 1)
	request[0] = DHCPv6Request
	reply := exchange(request, DHCPv6Reply, transaction)
	if reply.IAPD == nil || reply.IAPD.Prefix == nil || reply.IAPD.Prefix.Prefix != prefix || reply.IAPD.IAID != 1 {
		t.Fatalf("Reply did not carry the original allocated prefix and IAID: %+v", reply)
	}
	routes := backend.RouteAddCalls()
	if len(routes) != 1 || routes[0].name != handle.Identity().InterfaceName || routes[0].dest != prefix.String() || routes[0].gateway != "fe80::200:ff:fe00:2" || routes[0].metric != 0 || routes[0].proto != rtproto.Iface {
		t.Fatalf("delegated route did not belong to the original interface and peer: %+v", routes)
	}
	if allocated != 1 || len(released) != 0 {
		t.Fatalf("allocation/release before Stop = %d/%v, want one retained allocation attempt", allocated, released)
	}
	if err := service.Stop(); err != nil {
		t.Fatal(err)
	}
	steps := carrier.snapshot()
	final := slices.Index(steps, "final-ra")
	closed := slices.Index(steps, "carrier-close")
	for _, lane := range []string{"rs-stopped", "dhcp-stopped"} {
		joined := slices.Index(steps, lane)
		if joined < 0 || final <= joined || closed <= final {
			t.Fatalf("original carrier workers were not joined before final RA and close: %v", steps)
		}
	}
	removed := backend.RouteRemoveCalls()
	added := backend.RouteAddCalls()
	if len(removed) != 1 || removed[0] != added[0] || len(released) != 1 || released[0] != prefix {
		t.Fatalf("Stop did not withdraw/release the original delegated owner once: removed=%+v released=%v", removed, released)
	}
	addresses := backend.AddrRemoveCalls()
	if len(addresses) != 1 || addresses[0].name != handle.Identity().InterfaceName {
		t.Fatalf("Stop did not withdraw the original owner's link-local address once: %+v", addresses)
	}
	if err := service.Stop(); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(carrier.snapshot(), steps) || len(released) != 1 {
		t.Fatal("second Stop repeated carrier work or prefix release")
	}
}
