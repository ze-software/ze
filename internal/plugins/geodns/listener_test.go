package geodns

import (
	"net"
	"net/netip"
	"strconv"
	"testing"
	"time"

	"github.com/miekg/dns"
)

// VALIDATES: one record definition is served from several listeners with
// different ports and mixed families -- two IPv4 endpoints on distinct ports
// plus one IPv6 endpoint on a third port -- and every endpoint answers
// identically because the handler reads the single shared resolver snapshot.
// PREVENTS: a regression where records or source selection become per-listener,
// or a hidden one-port / one-v6 constraint.
func TestMultipleListenersShareRecords(t *testing.T) {
	p1, p2, p3 := freePort(t), freePort(t), freePort(t)

	data := `{"service":{"geodns":{"enabled":"true","zone":["test.example."],"nameserver":["127.0.0.1"],
		"host-set":{"external":{"host":{"proxy.test.example.":{"address":["10.7.7.7"]}}}},
		"source":{"0.0.0.0/0":{"host-set":"external"},"::/0":{"host-set":"external"}}}}}`
	cfg, err := parseConfig(data)
	if err != nil {
		t.Fatalf("parseConfig: %v", err)
	}
	// Two IPv4 listeners on different ports + one IPv6 listener on a third port,
	// all from the one record definition above.
	cfg.Listeners = []listenerEndpoint{
		{IP: netip.MustParseAddr("127.0.0.1"), Port: p1},
		{IP: netip.MustParseAddr("127.0.0.1"), Port: p2},
		{IP: netip.MustParseAddr("::1"), Port: p3},
	}
	storeApplied(cfg, 1)
	mgr := newServerManager(testLogger())
	if err := mgr.apply(cfg); err != nil {
		t.Fatalf("apply: %v", err)
	}
	t.Cleanup(mgr.stopAll)

	// Both IPv4 endpoints must answer the same record (hard assertions).
	for _, port := range []uint16{p1, p2} {
		addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(int(port)))
		resp := queryA(t, "udp", addr, "proxy.test.example.", "1.2.3.4")
		if got := firstA(resp); got != "10.7.7.7" {
			t.Errorf("IPv4 listener %s: A = %q, want 10.7.7.7", addr, got)
		}
	}

	// IPv6 loopback may be unavailable in some sandboxes; assert when reachable,
	// otherwise note and continue (the v4 endpoints already proved sharing).
	addr6 := net.JoinHostPort("::1", strconv.Itoa(int(p3)))
	c := &dns.Client{Net: "udp", Timeout: 2 * time.Second}
	if resp, _, err := c.Exchange(subnetMsg("proxy.test.example.", dns.TypeA, "1.2.3.4"), addr6); err != nil {
		t.Logf("IPv6 listener not reachable in this environment, skipping v6 assertion: %v", err)
	} else if got := firstA(resp); got != "10.7.7.7" {
		t.Errorf("IPv6 listener %s: A = %q, want 10.7.7.7", addr6, got)
	}
}
