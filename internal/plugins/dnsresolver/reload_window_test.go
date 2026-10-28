package dnsresolver

import (
	"net/netip"
	"sync"
	"testing"

	"github.com/miekg/dns"

	"github.com/ze-software/ze/internal/core/dnsserver"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// TestReloadWindowThroughInstall proves the reload window install opens is
// the narrowed one, observed while the reload binds a new listener rather
// than through narrow alone. Method: a running config with one listener
// allows 127.0.1.0/24 and 127.0.2.0/24; a reload adds a second listener and
// allows 127.0.2.0/24 and 127.0.3.0/24. The test resolver server's
// OnListenerChange hook, when the new listener's UDP up edge fires inside
// reconcile, asks res.answer (through the recursive wrapper, as a listener
// does) from one source of each /24 and records the rcodes. Only the source
// both lists admit is answered; the running-only and the candidate-only
// sources are REFUSED. A window that kept the running policy answers the
// running-only source, and one that published the candidate answers the
// candidate-only source.
// PREVENTS: a listener the reload binds answering a source the running config
// refuses, or a kept listener answering one the candidate refuses, before the
// reload is committed.
func TestReloadWindowThroughInstall(t *testing.T) {
	up := aUpstream(t, 0)
	cfg := testConfig(up)
	cfg.Listeners = []listenerEndpoint{{Name: "lan", IP: netip.MustParseAddr("127.0.0.1"), Port: freeLoopbackPort(t)}}
	cfg.AllowFrom = []netip.Prefix{netip.MustParsePrefix("127.0.1.0/24"), netip.MustParsePrefix("127.0.2.0/24")}
	res := newTestResolver(t, cfg, newTestClock())
	log := slogutil.DiscardLogger()

	next := cfg
	added := listenerEndpoint{Name: "pool", IP: netip.MustParseAddr("127.0.0.1"), Port: freeLoopbackPort(t)}
	next.Listeners = []listenerEndpoint{cfg.Listeners[0], added}
	next.AllowFrom = []netip.Prefix{netip.MustParsePrefix("127.0.2.0/24"), netip.MustParsePrefix("127.0.3.0/24")}
	addedAt := netip.AddrPortFrom(added.IP, added.Port)

	sources := []string{"127.0.1.9", "127.0.3.9", "127.0.2.9"}
	var mu sync.Mutex
	seen := map[string]int{}
	probed := false
	hook := func(protocol string, listen netip.AddrPort, isUp bool) {
		res.listeners.change(protocol, listen, isUp)
		if !isUp || protocol != "udp" || listen != addedAt {
			return
		}
		mu.Lock()
		defer mu.Unlock()
		probed = true
		for _, source := range sources {
			reply := ask(res, query("w.test", dns.TypeA), source, false)
			if reply == nil {
				seen[source] = -1
				continue
			}
			seen[source] = reply.Rcode
		}
	}
	srv := &resolverServer{mgr: dnsserver.New(log, recursiveHandler(log, res), dnsserver.Options{
		UDPSize:          dnsserver.RecursiveUDPSize,
		Freebind:         true,
		TCPConnMax:       resolverTCPConnMax,
		OnListenerChange: hook,
		MsgAcceptFunc:    dnsserver.RecursiveAccept,
	})}
	t.Cleanup(srv.stop)
	in := &resolverInstaller{srv: srv, res: res, log: log}
	if err := in.install(cfg); err != nil {
		t.Fatalf("first install: %v", err)
	}
	if err := in.install(next); err != nil {
		t.Fatalf("reload: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if !probed {
		t.Fatalf("the reload never fired the up edge of %s", addedAt)
	}
	want := map[string]int{
		"127.0.1.9": dns.RcodeRefused, // only the running config admits it
		"127.0.3.9": dns.RcodeRefused, // only the candidate admits it
		"127.0.2.9": dns.RcodeSuccess, // both admit it
	}
	for _, source := range sources {
		if seen[source] != want[source] {
			t.Errorf("during the reload, from %s rcode %d (%s), want %s", source, seen[source],
				dns.RcodeToString[seen[source]], dns.RcodeToString[want[source]])
		}
	}
}
