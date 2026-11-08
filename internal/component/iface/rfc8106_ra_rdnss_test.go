// Design: docs/features/dns-resolver.md -- where the router's servers come from
// Related: ra_rdnss.go -- the RA RDNSS producer under test

package iface

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/netip"
	"slices"
	"testing"
	"testing/synctest"
	"time"

	"github.com/ze-software/ze/internal/component/config/system"
	"github.com/ze-software/ze/internal/core/dnssource"
	"github.com/ze-software/ze/internal/core/ndp"
)

// rdnssOption encodes one RDNSS option with Ze's own RA builder, the bytes an
// RTM_NEWNDUSEROPT message carries after its header.
func rdnssOption(t *testing.T, lifetime uint32, servers ...string) []byte {
	t.Helper()
	cfg := ndp.RAConfig{RDNSSLifetime: lifetime}
	for _, s := range servers {
		cfg.RDNSS = append(cfg.RDNSS, netip.MustParseAddr(s))
	}
	buf := make([]byte, ndp.RALen(cfg))
	end := ndp.BuildRA(buf, 0, cfg)
	return buf[16:end]
}

// userOptEvent is the nd-user-option payload the netlink monitor emits for
// one option received on the kernel device name.
func userOptEvent(t *testing.T, name string, option []byte) string {
	t.Helper()
	data, err := json.Marshal(NDUserOptionPayload{Name: name, Router: "fe80::1", Option: option})
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// newTestRARDNSS builds the producer over a fresh registry, with eth0 unit 0
// (accept-ra 2) as its only target, and kernel names equal to logical ones.
func newTestRARDNSS(t *testing.T) (*raRDNSS, *system.Resolvers) {
	t.Helper()
	resolvers := &system.Resolvers{}
	p := newRARDNSS(resolvers, slog.New(slog.NewTextHandler(io.Discard, nil)))
	p.device = func(logical string) (string, error) { return logical, nil }
	acceptRA := 2
	cfg := &ifaceConfig{Ethernet: []ifaceEntry{{
		Name:  "eth0",
		Units: []unitEntry{{Label: "0", IPv6: &ipv6Settings{AcceptRA: &acceptRA}}},
	}}}
	p.setTargets(cfg)
	t.Cleanup(p.stop)
	return p, resolvers
}

// raRecords answers the registry's records of the ra origin.
func raRecords(r *system.Resolvers) []dnssource.Record {
	var out []dnssource.Record
	records := r.Snapshot().Records
	for i := range records {
		if records[i].Origin == "ra" {
			out = append(out, records[i])
		}
	}
	return out
}

// TestRARDNSSRecordFromUserOpt checks AC-21: an RDNSS option the kernel hands
// up from an accepted RA becomes an ra record for the unit, with the option's
// Lifetime as its expiry. Method: feed the nd-user-option payload and read
// the registry's snapshot.
//
// RFC requirement: RFC8106-1.2-1 positive --an RDNSS option received on a unit with accept-ra becomes an ra record carrying its addresses, so Ze extracts DNS information from RA messages.
func TestRARDNSSRecordFromUserOpt(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		p, resolvers := newTestRARDNSS(t)
		start := time.Now()
		p.handle(userOptEvent(t, "eth0", rdnssOption(t, 600, "2001:db8::53")))
		records := raRecords(resolvers)
		if len(records) != 1 {
			t.Fatalf("ra records %v, want one", records)
		}
		rec := records[0]
		if rec.Interface != "eth0" || rec.Unit != "0" {
			t.Fatalf("record names %s unit %s, want eth0 unit 0", rec.Interface, rec.Unit)
		}
		if !slices.Equal(rec.Addresses, []netip.Addr{netip.MustParseAddr("2001:db8::53")}) {
			t.Fatalf("addresses %v", rec.Addresses)
		}
		if want := start.Add(600 * time.Second); !rec.Expires.Equal(want) {
			t.Fatalf("expires %v, want %v", rec.Expires, want)
		}
		if rec.Rank != 400 {
			t.Fatalf("rank %d, want 400", rec.Rank)
		}
	})
}

// TestRARDNSSStaticNameServerWins checks the exception RFC 8106 Section 1.2
// makes: with a static name-server configured, the router uses it and not
// the RA's server, although the ra record is kept. Method: configure the
// registry with the default router list and one name-server, then feed an
// RDNSS option.
//
// RFC requirement: RFC8106-1.2-1 negative -- with a system name-server configured, the router's selected servers are the configured one and exclude the address an RDNSS option carried.
func TestRARDNSSStaticNameServerWins(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		p, resolvers := newTestRARDNSS(t)
		if err := resolvers.Configure("", []string{"192.0.2.53"}, system.DefaultRouterDNSSource()); err != nil {
			t.Fatal(err)
		}
		p.handle(userOptEvent(t, "eth0", rdnssOption(t, 600, "2001:db8::53")))
		assertRAAddresses(t, resolvers, "2001:db8::53")
		if got := resolvers.Servers(); !slices.Equal(got, []string{"192.0.2.53"}) {
			t.Fatalf("router servers %v, want the static 192.0.2.53 only", got)
		}
	})
}

// TestRARDNSSRefreshKeeps checks RFC 8106 Section 6.2 step (c): a non-zero
// Lifetime for an address already held moves its Expiration-time, so the
// address outlives its first Lifetime.
//
// RFC requirement: RFC8106-5.1-1 negative -- an RDNSS option with a non-zero Lifetime keeps its address in the ra record, and a refresh before expiry keeps it past the first Lifetime.
func TestRARDNSSRefreshKeeps(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		p, resolvers := newTestRARDNSS(t)
		p.handle(userOptEvent(t, "eth0", rdnssOption(t, 600, "2001:db8::53")))
		time.Sleep(500 * time.Second)
		p.handle(userOptEvent(t, "eth0", rdnssOption(t, 600, "2001:db8::53")))
		time.Sleep(200 * time.Second)
		synctest.Wait()
		assertRAAddresses(t, resolvers, "2001:db8::53")
	})
}

// TestRARDNSSListedAfterDHCP checks that the router's servers keep one
// ordered list over every source, DHCP before RA. Method: a DHCPv4 lease
// stand-in (rank 101, pppoe_client_dns_test.go) and an RA record on the same
// registry, no name-server, then the router's selection.
//
// RFC requirement: RFC8106-1.2-2 positive -- with servers learned from a DHCP lease and from an RDNSS option, the router's servers are one ordered list, the DHCP server first.
// RFC requirement: RFC8106-5.3.1-7 positive -- a server learned from DHCP is listed before a server learned from an RDNSS option.
func TestRARDNSSListedAfterDHCP(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		p, resolvers := newTestRARDNSS(t)
		p.handle(userOptEvent(t, "eth0", rdnssOption(t, 600, "2001:db8::53")))
		if err := resolvers.SetLease(testDHCPv4Origin.Source("eth1", "0"), []string{"198.51.100.1"}, system.UntilWithdrawn); err != nil {
			t.Fatal(err)
		}
		want := []string{"198.51.100.1", "2001:db8::53"}
		if got := resolvers.Servers(); !slices.Equal(got, want) {
			t.Fatalf("router servers %v, want %v", got, want)
		}
	})
}

// TestRARDNSSLinkLocalCarriesZone checks that a link-local RDNSS address is
// recorded with the zone of the device the RA arrived on, and a global one
// with none.
//
// RFC requirement: RFC8106-5.1-2 positive -- a link-local address from an RDNSS option is recorded scoped to the receiving device, so a query to it uses that link.
// RFC requirement: RFC8106-5.1-4 positive -- a link-local RDNSS address is registered with the zone of the link that received the option.
func TestRARDNSSLinkLocalCarriesZone(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		p, resolvers := newTestRARDNSS(t)
		p.handle(userOptEvent(t, "eth0", rdnssOption(t, 600, "fe80::53", "2001:db8::53")))
		assertRAAddresses(t, resolvers, "fe80::53%eth0", "2001:db8::53")
	})
}

// TestRARDNSSIgnoresUnitWithoutAcceptRA checks R-10: an RDNSS option on a
// device no unit set accept-ra for produces no record.
func TestRARDNSSIgnoresUnitWithoutAcceptRA(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		p, resolvers := newTestRARDNSS(t)
		p.handle(userOptEvent(t, "eth1", rdnssOption(t, 600, "2001:db8::53")))
		if records := raRecords(resolvers); len(records) != 0 {
			t.Fatalf("ra records %v from a unit without accept-ra", records)
		}
	})
}

// TestRARDNSSLifetimeZeroWithdraws checks AC-22: Lifetime 0 withdraws the
// address at once, and an address not refreshed is withdrawn at its expiry.
// Method: two addresses with different Lifetimes in one bubble, then a
// Lifetime-0 option for the long-lived one.
//
// RFC requirement: RFC8106-5.1-1 positive --a received RDNSS option with Lifetime 0 removes its addresses from the ra record at once, and an address whose Lifetime passed with no refresh is removed at its expiry.
func TestRARDNSSLifetimeZeroWithdraws(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		p, resolvers := newTestRARDNSS(t)
		p.handle(userOptEvent(t, "eth0", rdnssOption(t, 600, "2001:db8::53")))
		p.handle(userOptEvent(t, "eth0", rdnssOption(t, 1200, "2001:db8::54")))
		assertRAAddresses(t, resolvers, "2001:db8::54", "2001:db8::53")

		time.Sleep(601 * time.Second)
		synctest.Wait()
		assertRAAddresses(t, resolvers, "2001:db8::54")

		p.handle(userOptEvent(t, "eth0", rdnssOption(t, 0, "2001:db8::54")))
		if records := raRecords(resolvers); len(records) != 0 {
			t.Fatalf("ra records %v after Lifetime 0", records)
		}
	})
}

// TestRARDNSSMalformedDiscarded checks AC-23: an option whose Length fails
// the RFC 8106 check changes no record and is logged with the interface and
// the length.
//
// RFC requirement: RFC8106-5.3.1-1 positive --an RDNSS option received with Length below 3 or an even Length leaves the ra record unchanged and logs the interface and the Length.
func TestRARDNSSMalformedDiscarded(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		p, resolvers := newTestRARDNSS(t)
		var logged capturedLog
		p.log = slog.New(&logged)
		p.handle(userOptEvent(t, "eth0", rdnssOption(t, 600, "2001:db8::53")))
		bad := rdnssOption(t, 600, "2001:db8::99", "2001:db8::98")
		bad[1] = 4
		p.handle(userOptEvent(t, "eth0", bad))
		assertRAAddresses(t, resolvers, "2001:db8::53")
		if !logged.has("eth0", int64(4)) {
			t.Fatalf("no discard log naming eth0 and length 4: %v", logged.records)
		}
	})
}

// TestRARDNSSWithdrawnWhenAcceptRARemoved checks that a unit leaving the
// targets withdraws its record: the operator took the WAN out of RA.
func TestRARDNSSWithdrawnWhenAcceptRARemoved(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		p, resolvers := newTestRARDNSS(t)
		p.handle(userOptEvent(t, "eth0", rdnssOption(t, ndp.LifetimeInfinity, "2001:db8::53")))
		p.setTargets(&ifaceConfig{})
		if records := raRecords(resolvers); len(records) != 0 {
			t.Fatalf("ra records %v after accept-ra removed", records)
		}
	})
}

// pppoeRATargets answers an ifaceConfig whose eth0 unit 0 accepts RAs (as
// newTestRARDNSS sets) beside three pppoe-clients: pppoe0 with ipv6 accept-ra
// 2, pppoe1 with IPv6 but no accept-ra, and pppoe2 with accept-ra 2 but
// disabled.
func pppoeRATargets() *ifaceConfig {
	acceptRA := 2
	return &ifaceConfig{
		Ethernet: []ifaceEntry{{
			Name:  "eth0",
			Units: []unitEntry{{Label: "0", IPv6: &ipv6Settings{AcceptRA: &acceptRA}}},
		}},
		PPPoEClient: []pppoeClientEntry{
			{Name: "pppoe0", SourceInterface: "eth1", IPv6: PPPoEClientIPv6{Enabled: true, AcceptRA: 2}},
			{Name: "pppoe1", SourceInterface: "eth2", IPv6: PPPoEClientIPv6{Enabled: true}},
			{Name: "pppoe2", SourceInterface: "eth3", Disable: true, IPv6: PPPoEClientIPv6{Enabled: true, AcceptRA: 2}},
		},
	}
}

// TestRARDNSSPPPoEClientSession checks the home-CPE WAN: an RDNSS option on
// the session device of a pppoe-client with ipv6 accept-ra above 0 becomes an
// ra record named after the client, under the unit label its DHCPv6 lease
// carries; a client without accept-ra, or disabled, gives none, and removing
// the client's accept-ra withdraws the record. Method: the producer's targets
// from a config holding three pppoe-clients, an option fed for each session
// device, then the registry's snapshot.
//
// VALIDATES: spec-home-cpe-23 Phase 2a, a pppoe-client session is an RDNSS
// target (interface pppoe-client <n> ipv6 accept-ra 2).
// PREVENTS: the main home-CPE uplink's RA resolvers being dropped because the
// session device is no interface unit.
func TestRARDNSSPPPoEClientSession(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		p, resolvers := newTestRARDNSS(t)
		p.setTargets(pppoeRATargets())
		for _, device := range []string{"pppoe0", "pppoe1", "pppoe2"} {
			p.handle(userOptEvent(t, device, rdnssOption(t, 600, "2001:db8::53")))
		}
		records := raRecords(resolvers)
		if len(records) != 1 {
			t.Fatalf("ra records %+v, want one, for pppoe0 only", records)
		}
		if records[0].Interface != "pppoe0" || records[0].Unit != pppoeClientDHCPUnit {
			t.Fatalf("record names %s unit %q, want pppoe0 unit %q", records[0].Interface, records[0].Unit, pppoeClientDHCPUnit)
		}
		if got := resolvers.Servers(); !slices.Equal(got, []string{"2001:db8::53"}) {
			t.Fatalf("router servers %v, want the session's RA server", got)
		}

		withdrawn := pppoeRATargets()
		withdrawn.PPPoEClient[0].IPv6.AcceptRA = 0
		p.setTargets(withdrawn)
		if records := raRecords(resolvers); len(records) != 0 {
			t.Fatalf("ra records %+v after the pppoe-client's accept-ra was removed", records)
		}
	})
}

// TestRARDNSSLinkLocalScopedToItsOwnLink checks that a link-local RDNSS
// address is never recorded unscoped, nor scoped to a link other than the one
// that received it: the same fe80::53 heard on eth0 and on a pppoe-client
// session yields fe80::53%eth0 and fe80::53%pppoe0, each in its own record.
// Method: one option on each device, then each record's addresses.
//
// RFC requirement: RFC8106-5.1-2 negative -- a link-local RDNSS address received on one link is never recorded without a zone or with the zone of another link.
func TestRARDNSSLinkLocalScopedToItsOwnLink(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		p, resolvers := newTestRARDNSS(t)
		p.setTargets(pppoeRATargets())
		p.handle(userOptEvent(t, "eth0", rdnssOption(t, 600, "fe80::53")))
		p.handle(userOptEvent(t, "pppoe0", rdnssOption(t, 600, "fe80::53")))
		records := raRecords(resolvers)
		if len(records) != 2 {
			t.Fatalf("ra records %+v, want one per link", records)
		}
		for _, rec := range records {
			want := []netip.Addr{netip.MustParseAddr("fe80::53%" + rec.Interface)}
			if !slices.Equal(rec.Addresses, want) {
				t.Fatalf("record of %s holds %v, want %v only", rec.Interface, rec.Addresses, want)
			}
		}
	})
}

// TestRARDNSSLaterRAKeepsDHCPFirst checks that the router's list stays one
// ordered list as sources arrive: an RA heard after a DHCP lease neither
// displaces the DHCP server from the head nor replaces it, and withdrawing
// the lease leaves the RA server. Method: the DHCPv4 stand-in lease first,
// then an RA record, then the lease withdrawn.
//
// RFC requirement: RFC8106-1.2-2 negative -- a server learned from an RDNSS option after a DHCP lease never displaces or replaces the DHCP server in the router's list, and withdrawing the lease keeps the RDNSS server.
func TestRARDNSSLaterRAKeepsDHCPFirst(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		p, resolvers := newTestRARDNSS(t)
		lease := testDHCPv4Origin.Source("eth1", "0")
		if err := resolvers.SetLease(lease, []string{"198.51.100.1"}, system.UntilWithdrawn); err != nil {
			t.Fatal(err)
		}
		p.handle(userOptEvent(t, "eth0", rdnssOption(t, 600, "2001:db8::53")))
		if got, want := resolvers.Servers(), []string{"198.51.100.1", "2001:db8::53"}; !slices.Equal(got, want) {
			t.Fatalf("router servers %v, want %v", got, want)
		}
		if err := resolvers.SetLease(lease, nil, system.UntilWithdrawn); err != nil {
			t.Fatal(err)
		}
		if got, want := resolvers.Servers(), []string{"2001:db8::53"}; !slices.Equal(got, want) {
			t.Fatalf("after the lease was withdrawn: router servers %v, want %v", got, want)
		}
	})
}

// TestRAServerListOrderAndBound checks RFC 8106 Section 6.2 step (d): a new
// address goes first, option order kept, and a full list drops the entry that
// expires first.
func TestRAServerListOrderAndBound(t *testing.T) {
	now := time.Unix(1000, 0)
	var list raServerList
	first := ndp.RDNSS{Lifetime: 100}
	for i := range raServerListMax {
		first.Servers = append(first.Servers, netip.AddrFrom16([16]byte{0x20, 0x01, 0x0d, 0xb8, 15: byte(i + 1)}))
	}
	list.apply(first, now)
	if got := list.addresses(); len(got) != raServerListMax || got[0] != "2001:db8::1" {
		t.Fatalf("addresses %v", got)
	}
	list.apply(ndp.RDNSS{Lifetime: 50, Servers: []netip.Addr{netip.MustParseAddr("2001:db8::2")}}, now)
	list.apply(ndp.RDNSS{Lifetime: 300, Servers: []netip.Addr{netip.MustParseAddr("2001:db8::aa")}}, now)
	got := list.addresses()
	if got[0] != "2001:db8::aa" {
		t.Fatalf("new address not first: %v", got)
	}
	if slices.Contains(got, "2001:db8::2") {
		t.Fatalf("the entry expiring first survived a full list: %v", got)
	}
	if len(got) != raServerListMax {
		t.Fatalf("list holds %d, want %d", len(got), raServerListMax)
	}
}

func assertRAAddresses(t *testing.T, r *system.Resolvers, want ...string) {
	t.Helper()
	records := raRecords(r)
	if len(records) != 1 {
		t.Fatalf("ra records %v, want one", records)
	}
	var got []string
	for _, a := range records[0].Addresses {
		got = append(got, a.String())
	}
	if !slices.Equal(got, want) {
		t.Fatalf("ra addresses %v, want %v", got, want)
	}
}

// capturedLog is a slog handler that keeps every record's attributes.
type capturedLog struct {
	records []map[string]any
}

func (c *capturedLog) Enabled(_ context.Context, _ slog.Level) bool { return true }

func (c *capturedLog) Handle(_ context.Context, r slog.Record) error { //nolint:gocritic // slog.Handler takes the record by value
	attrs := map[string]any{"msg": r.Message}
	r.Attrs(func(a slog.Attr) bool {
		attrs[a.Key] = a.Value.Any()
		return true
	})
	c.records = append(c.records, attrs)
	return nil
}

func (c *capturedLog) WithAttrs([]slog.Attr) slog.Handler { return c }
func (c *capturedLog) WithGroup(string) slog.Handler      { return c }

// has reports whether one record names iface and length.
func (c *capturedLog) has(iface string, length int64) bool {
	for _, r := range c.records {
		if r["iface"] == iface && r["length"] == length {
			return true
		}
	}
	return false
}
