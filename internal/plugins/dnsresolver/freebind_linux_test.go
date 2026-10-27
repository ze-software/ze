// Design: docs/architecture/dns/lan-resolver.md -- listening on an address before it is up

//go:build linux

package dnsresolver

import (
	"context"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/miekg/dns"

	"github.com/ze-software/ze/internal/core/slogutil"
)

// notHeldAddr is an address no interface of the test host holds: TEST-NET-3
// (RFC 5737), never assigned to a host. It stands for a PPP pool gateway
// before the first subscriber session brings it up.
const notHeldAddr = "203.0.113.5"

// TestListenBeforeAddressIsUp proves the resolver binds a listener on an
// address the host does not hold yet, and keeps answering on the others.
// Method: a reload (BindStrict, which refuses the whole set when one listener
// cannot be bound) names a loopback listener and one on notHeldAddr. Without
// IP_FREEBIND on the resolver's sockets the second bind fails with
// EADDRNOTAVAIL and the install is refused; with it the install succeeds and
// a query to the loopback listener is answered. The test first proves a plain
// bind to notHeldAddr fails here, so a host that allows non-local binds
// (net.ipv4.ip_nonlocal_bind=1) cannot pass it vacuously. IP_FREEBIND needs
// no privilege, so the test runs unprivileged.
func TestListenBeforeAddressIsUp(t *testing.T) {
	var plain net.ListenConfig
	pc, err := plain.ListenPacket(context.Background(), "udp", net.JoinHostPort(notHeldAddr, "0"))
	if err == nil {
		_ = pc.Close() //nolint:errcheck // probe socket
		t.Skipf("this host binds %s without IP_FREEBIND (net.ipv4.ip_nonlocal_bind=1?), so the test proves nothing here", notHeldAddr)
	}

	up := aUpstream(t, 0)
	cfg := testConfig(up)
	port := freeLoopbackPort(t)
	cfg.Listeners = []listenerEndpoint{
		{Name: "lan", IP: netip.MustParseAddr("127.0.0.1"), Port: port},
		{Name: "pool-gateway", IP: netip.MustParseAddr(notHeldAddr), Port: port},
	}
	res := newTestResolver(t, cfg, newTestClock())
	log := slogutil.DiscardLogger()
	in := &resolverInstaller{srv: newResolverServer(log, res), res: res, log: log}
	t.Cleanup(in.srv.stop)
	if err := in.install(cfg); err != nil {
		t.Fatalf("install with a listener on %s, an address the host does not hold: %v", notHeldAddr, err)
	}

	addr := netip.AddrPortFrom(cfg.Listeners[0].IP, port).String()
	client := dns.Client{Net: "udp", Timeout: 2 * time.Second}
	reply, _, err := client.Exchange(query("www.example.test", dns.TypeA), addr)
	if err != nil {
		t.Fatalf("query %s: %v", addr, err)
	}
	if reply.Rcode != dns.RcodeSuccess || len(reply.Answer) != 1 {
		t.Fatalf("reply rcode %s answer %v, want NOERROR with one A", dns.RcodeToString[reply.Rcode], reply.Answer)
	}
}
