package dnsresolver

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/netip"
	"slices"
	"testing"
	"time"

	"github.com/miekg/dns"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/dnssource"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/pkg/plugin/sdk"
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

// learnedSnapshot is the system-dns snapshot JSON a plugin receives, with one
// record per address list given, learned on eth0 by DHCPv4.
func learnedSnapshot(t *testing.T, sequence uint64, lists ...[]string) string {
	t.Helper()
	snapshot := dnssource.Snapshot{Sequence: sequence}
	for _, list := range lists {
		record := dnssource.Record{Origin: "dhcpv4", Rank: 100, Interface: "eth0", Unit: "0", Received: time.Unix(1, 0).UTC()}
		for _, text := range list {
			record.Addresses = append(record.Addresses, netip.MustParseAddr(text))
		}
		snapshot.Records = append(snapshot.Records, record)
	}
	body, err := json.Marshal(&snapshot)
	if err != nil {
		t.Fatalf("marshal snapshot: %v", err)
	}
	return string(body)
}

// publishedForward answers the forward list the query path reads now.
func publishedForward(t *testing.T, res *resolver) []upstream {
	t.Helper()
	pol := res.policy.Load()
	if pol == nil {
		t.Fatal("no policy published")
	}
	return slices.Clone(pol.forward)
}

// TestForwardPassThroughDefault: a resolver with no forward server and no
// forward dns-source forwards to the learned servers (the D-5 default,
// configured then received, first-usable); once a forward server is
// committed it forwards to that server alone.
//
// VALIDATES: AC-20 (spec-home-cpe-23b). PREVENTS: a resolver with no forward
// server having no upstream while the WAN lease names one, and a learned
// server joining a configured one under the default.
func TestForwardPassThroughDefault(t *testing.T) {
	cfg := testConfig()
	// checkEnabled also refuses an enabled resolver with no listener, which
	// is not what this test is about.
	cfg.Listeners = []listenerEndpoint{{Name: "lan", IP: netip.MustParseAddr("127.0.0.1"), Port: 53}}
	if !cfg.Sources.Stated() {
		t.Fatal("the default config does not carry the built-in default list")
	}
	if err := checkEnabled(cfg); err != nil {
		t.Fatalf("an enabled resolver with no forward server under the default list was refused: %v", err)
	}
	res := newTestResolver(t, cfg, newTestClock())
	if got := publishedForward(t, res); len(got) != 0 {
		t.Fatalf("before any lease: forward = %v, want none", got)
	}
	if err := res.relearn(learnedSnapshot(t, 1, []string{"192.0.2.53"})); err != nil {
		t.Fatalf("relearn: %v", err)
	}
	if got, want := publishedForward(t, res), []upstream{{IP: netip.MustParseAddr("192.0.2.53"), Port: 53}}; !slices.Equal(got, want) {
		t.Fatalf("after the lease: forward = %v, want %v", got, want)
	}

	next := cfg
	next.Forward = []upstream{{IP: netip.MustParseAddr("198.51.100.53"), Port: 53}}
	res.apply(&next)
	if got, want := publishedForward(t, res), next.Forward; !slices.Equal(got, want) {
		t.Fatalf("after forward server: forward = %v, want %v alone", got, want)
	}
}

// TestForwardListFromSelectionNoRebind: with forward dns-source source
// received, configured, mode first-usable, a learned server is the
// upstream; withdrawing the lease moves the resolver to the forward server
// entry, and a query on a TCP connection opened before both changes is
// still answered, so no listener was rebound. A stale snapshot changes
// nothing.
//
// VALIDATES: AC-6 and AC-13's ordering (spec-home-cpe-23b). PREVENTS: a
// learned-server change rebinding the listeners, and a replayed older
// snapshot restoring a withdrawn server.
func TestForwardListFromSelectionNoRebind(t *testing.T) {
	up := aUpstream(t, 0)
	cfg := testConfig(up)
	list, err := dnssource.NewList(forwardDNSSourcePath, dnssource.FamilyAll, dnssource.AudienceZe,
		dnssource.ModeFirstUsable, dnssource.ClassReceived, dnssource.ClassConfigured)
	if err != nil {
		t.Fatalf("NewList: %v", err)
	}
	cfg.Sources = list
	cfg.Listeners = []listenerEndpoint{{Name: "lan", IP: netip.MustParseAddr("127.0.0.1"), Port: freeLoopbackPort(t)}}
	res := newTestResolver(t, cfg, newTestClock())
	log := slogutil.DiscardLogger()
	in := &resolverInstaller{srv: newResolverServer(log, res), res: res, log: log}
	t.Cleanup(in.srv.stop)
	if err := in.install(cfg); err != nil {
		t.Fatalf("install: %v", err)
	}
	addr := netip.AddrPortFrom(cfg.Listeners[0].IP, cfg.Listeners[0].Port).String()
	conn, err := dns.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		t.Fatalf("dial %s: %v", addr, err)
	}
	defer conn.Close() //nolint:errcheck // test cleanup

	if err := res.relearn(learnedSnapshot(t, 5, []string{"192.0.2.53"})); err != nil {
		t.Fatalf("relearn lease: %v", err)
	}
	if got, want := publishedForward(t, res), []upstream{{IP: netip.MustParseAddr("192.0.2.53"), Port: 53}}; !slices.Equal(got, want) {
		t.Fatalf("with the lease: forward = %v, want %v", got, want)
	}
	if err := res.relearn(learnedSnapshot(t, 6)); err != nil {
		t.Fatalf("relearn withdrawal: %v", err)
	}
	if got, want := publishedForward(t, res), []upstream{up.upstream()}; !slices.Equal(got, want) {
		t.Fatalf("after the withdrawal: forward = %v, want the forward server %v", got, want)
	}
	if err := res.relearn(learnedSnapshot(t, 5, []string{"192.0.2.53"})); err != nil {
		t.Fatalf("relearn stale: %v", err)
	}
	if got, want := publishedForward(t, res), []upstream{up.upstream()}; !slices.Equal(got, want) {
		t.Fatalf("a stale snapshot moved the forward list to %v, want %v", got, want)
	}

	if err := conn.SetDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("deadline: %v", err)
	}
	if err := conn.WriteMsg(query("a.test", dns.TypeA)); err != nil {
		t.Fatalf("write: %v", err)
	}
	reply, err := conn.ReadMsg()
	if err != nil {
		t.Fatalf("read on the connection opened before the changes: %v", err)
	}
	if reply.Rcode != dns.RcodeSuccess {
		t.Fatalf("reply %v, want NOERROR from the forward server", reply)
	}
}

// TestModeRequiredEachConsumer (LAN resolver): forward dns-source naming two
// sources without mode is refused by the parse the offline verifier, the
// reload's verify and the boot configure all run, naming the resolver's
// path; with mode it is accepted, and one source needs none.
//
// VALIDATES: AC-16b for the LAN resolver. PREVENTS: an ambiguous list
// passing ze config validate and failing at start, or the reverse.
func TestModeRequiredEachConsumer(t *testing.T) {
	section := func(dnsSource string) []sdk.ConfigSection {
		return []sdk.ConfigSection{{Root: configRootService,
			Data: `{"service":{"dns-resolver":{"forward":{"dns-source":` + dnsSource + `}}}}`}}
	}
	var required *dnssource.ModeRequiredError
	err := verifyResolverConfig(section(`{"source":["received","configured"]}`))
	if !errors.As(err, &required) {
		t.Fatalf("two sources, no mode: err = %v, want *ModeRequiredError", err)
	}
	if required.Path != forwardDNSSourcePath {
		t.Fatalf("refusal names %q, want %q", required.Path, forwardDNSSourcePath)
	}
	if _, _, err := parseResolverSections(section(`{"source":["received","configured"]}`)); !errors.As(err, &required) {
		t.Fatalf("boot and reload parse disagree with the verifier: %v", err)
	}
	if err := verifyResolverConfig(section(`{"source":["received","configured"],"mode":"merge"}`)); err != nil {
		t.Fatalf("two sources with mode refused: %v", err)
	}
	if err := verifyResolverConfig(section(`{"source":["received"]}`)); err != nil {
		t.Fatalf("one source without mode refused: %v", err)
	}
}

// TestDNSSourceClassRefused (LAN resolver): self and radius under forward
// dns-source are refused with the resolver's path and a fix, because the
// resolver cannot be its own upstream and is not a subscriber.
//
// VALIDATES: AC-11 for forward dns-source, R-3. PREVENTS: the LAN resolver
// forwarding to itself.
func TestDNSSourceClassRefused(t *testing.T) {
	for _, class := range []string{"self", "radius"} {
		err := verifyResolverConfig([]sdk.ConfigSection{{Root: configRootService,
			Data: `{"service":{"dns-resolver":{"forward":{"dns-source":{"source":["` + class + `"]}}}}}`}})
		var refused *dnssource.ClassRefusedError
		if !errors.As(err, &refused) {
			t.Fatalf("%s: err = %v, want *ClassRefusedError", class, err)
		}
		if refused.Path != forwardDNSSourcePath || refused.Refusal.Fix == "" {
			t.Fatalf("%s: refusal %+v names the wrong path or no fix", class, refused)
		}
	}
}
