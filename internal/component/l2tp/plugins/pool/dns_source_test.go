package l2tppool

import (
	"errors"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/l2tp"
	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/core/dnssource"
)

// learnedPlugin builds a plugin serving pool whose learned records are one
// unexpired DHCPv4 lease on wan0 carrying addresses.
func learnedPlugin(pool *ipv4Pool, addresses ...netip.Addr) *poolPlugin {
	p := &poolPlugin{pool: pool}
	if len(addresses) > 0 {
		p.learned.AcceptSnapshot(&dnssource.Snapshot{Sequence: 1, Records: []dnssource.Record{{
			Origin: "dhcpv4", Rank: 100, Interface: "wan0", Unit: "0",
			Addresses: addresses, Received: time.Now(), Expires: time.Now().Add(time.Hour),
		}}})
	}
	return p
}

// askIPCP runs one session's IPv4 request through the registered handler,
// with meta stored as the session's RADIUS metadata when it is not nil.
func askIPCP(t *testing.T, p *poolPlugin, sessionID uint16, meta *l2tp.AuthMetadata) ppp.IPResponseArgs {
	t.Helper()
	const tunnelID = 23
	if meta != nil {
		l2tp.StoreSessionMetadata(tunnelID, sessionID, meta)
		t.Cleanup(func() { l2tp.ClearSessionMetadata(tunnelID, sessionID) })
	}
	resp := p.handle(ppp.EventIPRequest{TunnelID: tunnelID, SessionID: sessionID, Family: ppp.AddressFamilyIPv4})
	if !resp.Accept {
		t.Fatalf("pool refused the session: %s", resp.Reason)
	}
	return resp
}

// TestPoolIPCPDNSFromRadius checks AC-9: under the default list (radius,
// configured, received, first-usable) an Access-Accept's
// MS-Primary-DNS-Server is offered as the IPCP primary, ahead of the pool's
// dns-primary and of a learned resolver. Method: a pool with dns-primary set
// and a learned lease, a session whose metadata names one MS DNS server, on
// the allocated path and on the Framed-IP path.
func TestPoolIPCPDNSFromRadius(t *testing.T) {
	radiusDNS := netip.MustParseAddr("198.51.100.53")
	pool := newIPv4Pool(testGW, netip.MustParseAddr("10.0.0.1"), netip.MustParseAddr("10.0.0.10"),
		netip.MustParseAddr("203.0.113.53"), netip.Addr{})
	p := learnedPlugin(pool, netip.MustParseAddr("192.0.2.53"))

	allocated := askIPCP(t, p, 1, &l2tp.AuthMetadata{DNSPrimary: radiusDNS})
	if allocated.DNSPrimary != radiusDNS {
		t.Errorf("allocated path DNSPrimary = %v, want the RADIUS server %v", allocated.DNSPrimary, radiusDNS)
	}
	if allocated.DNSSecondary.IsValid() {
		t.Errorf("allocated path DNSSecondary = %v, want none: first-usable stops at radius", allocated.DNSSecondary)
	}

	framed := askIPCP(t, p, 2, &l2tp.AuthMetadata{FramedIP: netip.MustParseAddr("198.51.100.5"),
		DNSPrimary: radiusDNS, DNSSecondary: netip.MustParseAddr("198.51.100.54")})
	if framed.DNSPrimary != radiusDNS {
		t.Errorf("Framed-IP path DNSPrimary = %v, want %v", framed.DNSPrimary, radiusDNS)
	}
	if want := netip.MustParseAddr("198.51.100.54"); framed.DNSSecondary != want {
		t.Errorf("Framed-IP path DNSSecondary = %v, want %v", framed.DNSSecondary, want)
	}
}

// TestPoolIPCPDNSRadiusAbsent checks AC-10: with no MS DNS in the
// Access-Accept the pool's dns-primary is offered, and with neither RADIUS
// nor pool servers the learned resolver is. Method: the same session request
// against a pool with dns-primary, then against one with none.
func TestPoolIPCPDNSRadiusAbsent(t *testing.T) {
	learnedDNS := netip.MustParseAddr("192.0.2.53")
	configured := netip.MustParseAddr("203.0.113.53")

	withServer := learnedPlugin(newIPv4Pool(testGW, netip.MustParseAddr("10.0.0.1"),
		netip.MustParseAddr("10.0.0.10"), configured, netip.Addr{}), learnedDNS)
	if got := askIPCP(t, withServer, 3, &l2tp.AuthMetadata{FilterID: "gold"}).DNSPrimary; got != configured {
		t.Errorf("pool with dns-primary: DNSPrimary = %v, want the configured %v", got, configured)
	}

	bare := learnedPlugin(newIPv4Pool(testGW, netip.MustParseAddr("10.0.0.1"),
		netip.MustParseAddr("10.0.0.10"), netip.Addr{}, netip.Addr{}), learnedDNS)
	if got := askIPCP(t, bare, 4, nil).DNSPrimary; got != learnedDNS {
		t.Errorf("pool with no DNS: DNSPrimary = %v, want the learned %v", got, learnedDNS)
	}

	none := learnedPlugin(newIPv4Pool(testGW, netip.MustParseAddr("10.0.0.1"),
		netip.MustParseAddr("10.0.0.10"), netip.Addr{}, netip.Addr{}))
	resp := askIPCP(t, none, 5, nil)
	if resp.DNSPrimary.IsValid() {
		t.Errorf("no source holds a server: DNSPrimary = %v, want none (today's answer)", resp.DNSPrimary)
	}
	if resp.DNSSecondary.IsValid() {
		t.Errorf("no source holds a server: DNSSecondary = %v, want none", resp.DNSSecondary)
	}
}

// TestPoolDropsHostOnlyAddresses checks AC-27 for the subscriber: a loopback
// or zone-less link-local resolver is never offered in IPCP, and the next
// usable class is. Method: RADIUS names 127.0.0.53 alone, the pool names
// 127.0.0.1, and the learned lease holds a usable server.
func TestPoolDropsHostOnlyAddresses(t *testing.T) {
	learnedDNS := netip.MustParseAddr("192.0.2.53")
	p := learnedPlugin(newIPv4Pool(testGW, netip.MustParseAddr("10.0.0.1"), netip.MustParseAddr("10.0.0.10"),
		netip.MustParseAddr("127.0.0.1"), netip.Addr{}), netip.MustParseAddr("127.0.0.53"), learnedDNS)
	resp := askIPCP(t, p, 6, &l2tp.AuthMetadata{DNSPrimary: netip.MustParseAddr("127.0.0.53")})
	if resp.DNSPrimary != learnedDNS {
		t.Errorf("DNSPrimary = %v, want %v once every host-only server is dropped", resp.DNSPrimary, learnedDNS)
	}
	if resp.DNSSecondary.IsValid() {
		t.Errorf("DNSSecondary = %v, want none: the learned 127.0.0.53 is host-only", resp.DNSSecondary)
	}
}

// TestSelfIsAddressOnClientInterface checks D-6 for the subscriber: self is
// the pool's gateway, the LNS's own address on the PPP link. Method: a pool
// whose written list is source self.
func TestSelfIsAddressOnClientInterface(t *testing.T) {
	pool := newIPv4Pool(testGW, netip.MustParseAddr("10.0.0.1"), netip.MustParseAddr("10.0.0.10"),
		netip.MustParseAddr("203.0.113.53"), netip.Addr{})
	list, err := parsePoolSources(map[string]any{"dns-source": map[string]any{"source": "self"}},
		poolDNSSourcePath, poolDNSDefault)
	if err != nil {
		t.Fatalf("source self refused: %v", err)
	}
	pool.sources = list
	if got := askIPCP(t, learnedPlugin(pool), 7, nil).DNSPrimary; got != testGW {
		t.Errorf("DNSPrimary = %v, want the gateway %v", got, testGW)
	}
}

// TestPoolCutsAtTwoServers checks the IPCP bound: a merge list holding three
// servers offers the first two in list order. Method: radius then configured,
// merge, with one server from each and a second configured one.
func TestPoolCutsAtTwoServers(t *testing.T) {
	pool := newIPv4Pool(testGW, netip.MustParseAddr("10.0.0.1"), netip.MustParseAddr("10.0.0.10"),
		netip.MustParseAddr("203.0.113.53"), netip.MustParseAddr("203.0.113.54"))
	list, err := parsePoolSources(map[string]any{"dns-source": map[string]any{
		"source": []any{"radius", "configured"}, "mode": "merge"}}, poolDNSSourcePath, poolDNSDefault)
	if err != nil {
		t.Fatalf("list refused: %v", err)
	}
	pool.sources = list
	resp := askIPCP(t, learnedPlugin(pool), 8, &l2tp.AuthMetadata{DNSPrimary: netip.MustParseAddr("198.51.100.53")})
	if want := netip.MustParseAddr("198.51.100.53"); resp.DNSPrimary != want {
		t.Errorf("DNSPrimary = %v, want %v", resp.DNSPrimary, want)
	}
	if want := netip.MustParseAddr("203.0.113.53"); resp.DNSSecondary != want {
		t.Errorf("DNSSecondary = %v, want %v", resp.DNSSecondary, want)
	}
}

// TestModeRequiredEachConsumer checks AC-16b for both pool lists: two sources
// with no mode are refused through the plugin's verifier with the path the
// operator types, the same list with a mode and a one-source list are
// accepted. Method: verifyPoolConfig over l2tp config JSON.
func TestModeRequiredEachConsumer(t *testing.T) {
	const ipv4 = `"gateway":"10.0.0.254","start":"10.0.0.1","end":"10.0.0.10"`
	cases := []struct {
		name     string
		data     string
		wantPath string
	}{
		{"ipv4 no mode", `{"l2tp":{"pool":{"ipv4":{` + ipv4 + `,"dns-source":{"source":["radius","configured"]}}}}}`, poolDNSSourcePath},
		{"named no mode", `{"l2tp":{"pool":{"named-pool":{"gold":{` + ipv4 + `,"dns-source":{"source":["configured","received"]}}}}}}`, namedPoolDNSSourcePath},
		{"ipv4 with mode", `{"l2tp":{"pool":{"ipv4":{` + ipv4 + `,"dns-source":{"source":["radius","configured"],"mode":"merge"}}}}}`, ""},
		{"named one source", `{"l2tp":{"pool":{"named-pool":{"gold":{` + ipv4 + `,"dns-source":{"source":"received"}}}}}}`, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseFullPoolConfig(tc.data)
			if tc.wantPath == "" {
				if err != nil {
					t.Fatalf("refused: %v", err)
				}
				return
			}
			if _, ok := errors.AsType[*dnssource.ModeRequiredError](err); !ok {
				t.Fatalf("err = %v, want a ModeRequiredError", err)
			}
			if !strings.Contains(err.Error(), tc.wantPath) {
				t.Errorf("refusal %q does not name %s", err, tc.wantPath)
			}
		})
	}
}
