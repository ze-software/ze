package dnsresolver

import (
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/miekg/dns"

	"github.com/ze-software/ze/internal/core/slogutil"
)

// freeLoopbackPort answers a port free for both TCP and UDP on 127.0.0.1 at
// the moment of the call.
func freeLoopbackPort(t *testing.T) uint16 {
	t.Helper()
	for range 20 {
		tcp, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("listen tcp: %v", err)
		}
		addr, ok := tcp.Addr().(*net.TCPAddr)
		if !ok {
			t.Fatalf("listener address %T is not TCP", tcp.Addr())
		}
		udp, err := net.ListenPacket("udp", addr.String())
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
