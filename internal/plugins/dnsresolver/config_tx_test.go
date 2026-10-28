package dnsresolver

import (
	"context"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/miekg/dns"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// freeLoopbackPort answers a port free for both TCP and UDP on 127.0.0.1 at
// the moment of the call.
func freeLoopbackPort(t *testing.T) uint16 {
	t.Helper()
	var lc net.ListenConfig
	ctx := context.Background()
	for range 20 {
		tcp, err := lc.Listen(ctx, "tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("listen tcp: %v", err)
		}
		addr, ok := tcp.Addr().(*net.TCPAddr)
		if !ok {
			t.Fatalf("listener address %T is not TCP", tcp.Addr())
		}
		udp, err := lc.ListenPacket(ctx, "udp", addr.String())
		_ = tcp.Close() //nolint:errcheck // probe socket
		if err != nil {
			continue
		}
		_ = udp.Close() //nolint:errcheck // probe socket
		return uint16(addr.Port)
	}
	t.Fatal("no port free for both TCP and UDP after 20 tries")
	return 0
}

// TestReloadKeepsListeners proves AC-30's listener half: a reload that
// changes only allow-from, the forward list and the limits applies the new
// policy on the next query and never rebinds the listener. Method: one TCP
// connection opened before the reload carries a query after it. A rebind
// shuts the listener down, which closes every connection it accepted, so the
// second exchange on the same connection only succeeds when the listener was
// kept; its REFUSED answer proves the new allow-from is live.
func TestReloadKeepsListeners(t *testing.T) {
	up := aUpstream(t, 0)
	cfg := testConfig(up)
	cfg.Listeners = []listenerEndpoint{{Name: "lan", IP: netip.MustParseAddr("127.0.0.1"), Port: freeLoopbackPort(t)}}
	res := newTestResolver(t, cfg, newTestClock())
	log := slogutil.DiscardLogger()
	in := &resolverInstaller{srv: newResolverServer(log, res), res: res, log: log}
	t.Cleanup(in.srv.stop)
	if err := in.install(cfg); err != nil {
		t.Fatalf("first install: %v", err)
	}

	addr := netip.AddrPortFrom(cfg.Listeners[0].IP, cfg.Listeners[0].Port).String()
	conn, err := dns.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		t.Fatalf("dial %s: %v", addr, err)
	}
	defer conn.Close() //nolint:errcheck // test cleanup
	exchange := func(name string) *dns.Msg {
		t.Helper()
		if err := conn.SetDeadline(time.Now().Add(2 * time.Second)); err != nil {
			t.Fatalf("deadline: %v", err)
		}
		if err := conn.WriteMsg(query(name, dns.TypeA)); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
		reply, err := conn.ReadMsg()
		if err != nil {
			t.Fatalf("read %s on the connection opened before the reload: %v", name, err)
		}
		return reply
	}
	if reply := exchange("a.test"); reply.Rcode != dns.RcodeSuccess {
		t.Fatalf("before the reload: %v, want NOERROR", reply)
	}

	next := cfg
	next.AllowFrom = []netip.Prefix{netip.MustParsePrefix("192.0.2.0/24")}
	next.Forward = append(next.Forward[:0:0], aUpstream(t, 0).upstream(), up.upstream())
	next.LimitPerClientRate = cfg.LimitPerClientRate + 1
	next.LimitOutstanding = cfg.LimitOutstanding + 1
	if err := in.install(next); err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reply := exchange("b.test"); reply.Rcode != dns.RcodeRefused {
		t.Fatalf("after the reload: %v, want REFUSED under the new allow-from", reply)
	}
}

// TestRefusedReloadKeepsRunningPolicy proves a reload refused because a
// listener cannot be bound leaves the whole running config live, its policy
// included. Method: a reload adds a listener on a port another socket holds
// and changes allow-from, the forward list, the limits and the cache size;
// the install fails, a query from 127.0.0.1 on the running listener is still
// answered NOERROR under the running allow-from, and show reports the
// running listener, upstream, limits and cache size.
func TestRefusedReloadKeepsRunningPolicy(t *testing.T) {
	up := aUpstream(t, 0)
	cfg := testConfig(up)
	cfg.Listeners = []listenerEndpoint{{Name: "lan", IP: netip.MustParseAddr("127.0.0.1"), Port: freeLoopbackPort(t)}}
	res := newTestResolver(t, cfg, newTestClock())
	log := slogutil.DiscardLogger()
	in := &resolverInstaller{srv: newResolverServer(log, res), res: res, log: log}
	t.Cleanup(in.srv.stop)
	if err := in.install(cfg); err != nil {
		t.Fatalf("first install: %v", err)
	}

	var lc net.ListenConfig
	held, err := lc.ListenPacket(context.Background(), "udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("hold a port: %v", err)
	}
	defer held.Close() //nolint:errcheck // test cleanup
	heldPort := netip.MustParseAddrPort(held.LocalAddr().String()).Port()

	next := cfg
	next.Listeners = append(next.Listeners[:0:0], cfg.Listeners[0],
		listenerEndpoint{Name: "taken", IP: netip.MustParseAddr("127.0.0.1"), Port: heldPort})
	next.AllowFrom = []netip.Prefix{netip.MustParsePrefix("192.0.2.0/24")}
	next.Forward = append(next.Forward[:0:0], aUpstream(t, 0).upstream())
	next.LimitOutstanding = cfg.LimitOutstanding + 1
	next.CacheSize = cfg.CacheSize + 1
	if err := in.install(next); err == nil {
		t.Fatal("reload onto a held port was accepted, want refused")
	}

	addr := netip.AddrPortFrom(cfg.Listeners[0].IP, cfg.Listeners[0].Port).String()
	c := &dns.Client{Net: "udp", Timeout: 2 * time.Second}
	reply, _, err := c.Exchange(query("a.test", dns.TypeA), addr)
	if err != nil {
		t.Fatalf("query the running listener after the refused reload: %v", err)
	}
	if reply.Rcode != dns.RcodeSuccess {
		t.Fatalf("after the refused reload: %s, want NOERROR under the running allow-from",
			dns.RcodeToString[reply.Rcode])
	}

	shown := res.show()
	if got, ok := shown["listeners"].([]plugin.Map); !ok || len(got) != 1 {
		t.Fatalf("show listeners %v, want the one running listener", shown["listeners"])
	}
	if got, ok := shown["upstreams"].([]string); !ok || len(got) != 1 || got[0] != up.upstream().addrPort().String() {
		t.Fatalf("show upstreams %v, want the running %s", shown["upstreams"], up.upstream().addrPort())
	}
	if got := shown["limit-outstanding"]; got != cfg.LimitOutstanding {
		t.Fatalf("show limit-outstanding %v, want the running %d", got, cfg.LimitOutstanding)
	}
	if got := res.shown.Load().CacheSize; got != cfg.CacheSize {
		t.Fatalf("shown cache size %d, want the running %d", got, cfg.CacheSize)
	}
}
