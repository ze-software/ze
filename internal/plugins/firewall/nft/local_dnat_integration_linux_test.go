//go:build linux

// Design: docs/guide/firewall.md -- the destination-local match
// Related: lower_linux.go -- lowerDestinationLocalMatch, the rule under test
// Related: integration_linux_test.go -- withNftNetNS, newNftIntegrationBackend

package firewallnft

import (
	"net"
	"runtime"
	"testing"
	"time"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"

	"github.com/ze-software/ze/internal/component/firewall"
	"github.com/ze-software/ze/internal/test/userns"
)

// The lab: a router namespace and a client namespace joined by one veth pair.
// The router holds localDNATRouter; localDNATTransit lies beyond it, on no
// link the router has, so only a translation can answer a connection to it.
const (
	localDNATRouter     = "192.0.2.1"
	localDNATClient     = "192.0.2.2"
	localDNATTransit    = "198.51.100.7"
	localDNATPort       = "8000"
	localDNATService    = "9000"
	localDNATRouterEnd  = "zldr0"
	localDNATClientEnd  = "zldc0"
	localDNATDialWait   = 2 * time.Second
	localDNATRefuseWait = time.Second
)

// localDNATConfig is the operator's config for a published port: a prerouting
// nat chain whose one term translates TCP port localDNATPort to the router's
// service port, with the extra from leaves in from. It goes through
// firewall.ParseFirewallConfig and firewall.ValidateTables, the parse and
// verify the daemon runs.
func localDNATConfig(from string) string {
	return `{"firewall":{"table":{"localdnat":{"family":"ip","chain":{"prerouting":{` +
		`"type":"nat","hook":"prerouting","priority":"-100","policy":"accept",` +
		`"term":{"published":{"from":{"protocol":"tcp","destination-port":"` + localDNATPort + `"` + from + `},` +
		`"then":{"dnat":{"to":"` + localDNATRouter + `:` + localDNATService + `"}}}}}}}}}}`
}

// TestLocalDestinationDNATLeavesTransitAlone proves on the kernel that a
// destination translation carrying destination-local translates traffic
// addressed to the router and leaves transit traffic to the same port alone.
// Method: a client namespace behind a router namespace; the router listens on
// its service port only. With the match, a connection to the router's address
// on the published port reaches the service (the positive control: the rule
// translates), and a connection to an address beyond the router on the same
// port is not answered. The second control applies the same term without the
// match: the transit connection then reaches the service, which proves the
// transit check above could have gone the other way. Each dial uses its own
// source port, so no conntrack entry of an earlier dial decides a later one.
//
// Red walk: lowerMatch answers no expressions for MatchDestinationLocal. The
// transit connection is then translated with the match too, and this test
// goes red at the transit assertion.
func TestLocalDestinationDNATLeavesTransitAlone(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	withNftNetNS(t, func() {
		client := openLocalDNATClient(t)
		var listen net.ListenConfig
		listener, err := listen.Listen(t.Context(), "tcp4", net.JoinHostPort(localDNATRouter, localDNATService))
		if err != nil {
			t.Fatalf("listen on the router's service port: %v", err)
		}
		t.Cleanup(func() { listener.Close() }) //nolint:errcheck // test cleanup
		b := newNftIntegrationBackend(t)
		apply := func(from string) {
			t.Helper()
			tables, err := firewall.ParseFirewallConfig(localDNATConfig(from))
			if err != nil {
				t.Fatalf("parse the translation: %v", err)
			}
			if err := firewall.ValidateTables(tables); err != nil {
				t.Fatalf("validate the translation: %v", err)
			}
			if err := b.Apply(tables); err != nil {
				t.Fatalf("apply the translation: %v", err)
			}
		}

		apply(`,"destination-local":""`)
		if err := dialLocalDNAT(client, localDNATRouter, 40001, localDNATDialWait); err != nil {
			t.Fatalf("positive control: a connection to the router's address on the published port failed: %v", err)
		}
		if err := dialLocalDNAT(client, localDNATTransit, 40002, localDNATRefuseWait); err == nil {
			t.Fatal("transit traffic to the published port was translated to the router's service")
		}

		apply("")
		if err := dialLocalDNAT(client, localDNATTransit, 40003, localDNATDialWait); err != nil {
			t.Fatalf("control: without destination-local the transit connection was not translated either, so the transit check proves nothing: %v", err)
		}
	})
}

// openLocalDNATClient makes the client namespace and the veth pair that joins
// it to the router namespace the calling thread is locked in, addresses both
// ends, and gives the client a default route through the router. It leaves
// the thread in the router namespace, and removes the client namespace, with
// the pair, in cleanup.
func openLocalDNATClient(t *testing.T) netns.NsHandle {
	t.Helper()
	router, err := netns.Get()
	if err != nil {
		t.Fatalf("read the router namespace: %v", err)
	}
	t.Cleanup(func() { router.Close() }) //nolint:errcheck // test cleanup
	name := nftNetNSName(t.Name()) + "c"
	client, err := netns.NewNamed(name)
	if err != nil {
		t.Fatalf("create the client namespace: %v", err)
	}
	if err := netns.Set(router); err != nil {
		t.Fatalf("return to the router namespace: %v", err)
	}
	t.Cleanup(func() {
		client.Close()          //nolint:errcheck // test cleanup
		netns.DeleteNamed(name) //nolint:errcheck // test cleanup
	})

	veth := &netlink.Veth{
		Name:          localDNATRouterEnd,
		PeerName:      localDNATClientEnd,
		PeerNamespace: netlink.NsFd(int(client)),
	}
	if err := netlink.LinkAdd(veth); err != nil {
		t.Fatalf("create the veth pair: %v", err)
	}
	configureLocalDNATEnd(t, netlinkHandleOrFail(t, netns.None()), localDNATRouterEnd, localDNATRouter, false)
	configureLocalDNATEnd(t, netlinkHandleOrFail(t, client), localDNATClientEnd, localDNATClient, true)
	return client
}

// netlinkHandleOrFail answers a netlink handle bound to ns, or to the calling
// thread's namespace for netns.None(), closed in cleanup.
func netlinkHandleOrFail(t *testing.T, ns netns.NsHandle) *netlink.Handle {
	t.Helper()
	var (
		h   *netlink.Handle
		err error
	)
	if ns == netns.None() {
		h, err = netlink.NewHandle()
	} else {
		h, err = netlink.NewHandleAt(ns)
	}
	if err != nil {
		t.Fatalf("open a netlink handle: %v", err)
	}
	t.Cleanup(h.Close)
	return h
}

// configureLocalDNATEnd raises the namespace's loopback device, gives the
// named end its /24 address and raises it, and with viaRouter adds a default
// route through the router's address.
func configureLocalDNATEnd(t *testing.T, h *netlink.Handle, device, address string, viaRouter bool) {
	t.Helper()
	lo, err := h.LinkByName("lo")
	if err != nil {
		t.Fatalf("find lo: %v", err)
	}
	if err := h.LinkSetUp(lo); err != nil {
		t.Fatalf("raise lo: %v", err)
	}
	end, err := h.LinkByName(device)
	if err != nil {
		t.Fatalf("find %s: %v", device, err)
	}
	prefix, err := netlink.ParseAddr(address + "/24")
	if err != nil {
		t.Fatalf("parse %s/24: %v", address, err)
	}
	if err := h.AddrAdd(end, prefix); err != nil {
		t.Fatalf("address %s: %v", device, err)
	}
	if err := h.LinkSetUp(end); err != nil {
		t.Fatalf("raise %s: %v", device, err)
	}
	if !viaRouter {
		return
	}
	route := &netlink.Route{LinkIndex: end.Attrs().Index, Gw: net.ParseIP(localDNATRouter)}
	if err := h.RouteAdd(route); err != nil {
		t.Fatalf("default route through the router: %v", err)
	}
}

// dialLocalDNAT connects from the client namespace to target on the published
// port, from sourcePort, and answers the dial's error. The dial runs on a
// thread of its own locked into the client namespace and never unlocked, so
// the thread is discarded when the goroutine ends and never returns to the
// scheduler in that namespace.
func dialLocalDNAT(client netns.NsHandle, target string, sourcePort int, wait time.Duration) error {
	result := make(chan error, 1)
	go func() {
		runtime.LockOSThread()
		if err := netns.Set(client); err != nil {
			result <- err
			return
		}
		dialer := net.Dialer{
			Timeout:   wait,
			LocalAddr: &net.TCPAddr{IP: net.ParseIP(localDNATClient), Port: sourcePort},
		}
		conn, err := dialer.Dial("tcp4", net.JoinHostPort(target, localDNATPort))
		if err == nil {
			conn.Close() //nolint:errcheck // the handshake is the evidence
		}
		result <- err
	}()
	return <-result
}
