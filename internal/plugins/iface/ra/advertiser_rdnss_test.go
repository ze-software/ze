package ifacera

import (
	"net/netip"
	"slices"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/core/ndp"
)

// rdnssTestLifetime is the unit lifetime every case advertises.
const rdnssTestLifetime = 1800

func rdnssSpec(expires time.Time, servers ...string) iface.RASenderSpec {
	spec := iface.RASenderSpec{
		Advertisement: ndp.RAConfig{RouterLifetime: 1800, RDNSSLifetime: rdnssTestLifetime},
		RDNSSExpires:  expires,
	}
	for _, s := range servers {
		spec.Advertisement.RDNSS = append(spec.Advertisement.RDNSS, netip.MustParseAddr(s))
	}
	return spec
}

// VALIDATES: AC-8 and AC-15, the withdrawal half (RFC 8106 Section 5.1: "A
// value of zero means that the RDNSS addresses MUST no longer be used."). A
// resolver an update drops is sent once in an RDNSS option of Lifetime 0, at
// once (urgent), beside the servers still advertised, and not again after.
// PREVENTS: a host keeping a withdrawn resolver until its last advertised
// lifetime runs out, and a withdrawal that repeats forever.
//
// RFC requirement: RFC8106-5.1-1 positive -- a resolver an update stops advertising is carried once in the RDNSS withdrawals of Lifetime 0 beside the servers still advertised with their lifetime, the advertisement encodes, and the next advertisement carries no withdrawal.
func TestRDNSSWithdrawnAdvertisedLifetimeZero(t *testing.T) {
	now := time.Now()
	adv := &advertiser{spec: rdnssSpec(time.Time{}, "2001:db8::53")}
	adv.sent(adv.build(now, false))

	adv.update(rdnssSpec(time.Time{}, "2001:db8:1::53"))
	if !adv.urgent(now, false) {
		t.Fatal("a withdrawn resolver must be sent before the periodic schedule")
	}
	cfg := adv.build(now, false)
	if !slices.Equal(cfg.RDNSS, []netip.Addr{netip.MustParseAddr("2001:db8:1::53")}) || cfg.RDNSSLifetime != 1800 {
		t.Fatalf("live RDNSS = %v lifetime %d, want [2001:db8:1::53] 1800", cfg.RDNSS, cfg.RDNSSLifetime)
	}
	if !slices.Equal(cfg.RDNSSWithdrawn, []netip.Addr{netip.MustParseAddr("2001:db8::53")}) {
		t.Fatalf("withdrawn = %v, want [2001:db8::53]", cfg.RDNSSWithdrawn)
	}
	adv.sent(cfg)
	if next := adv.build(now, false); len(next.RDNSSWithdrawn) != 0 {
		t.Fatalf("withdrawal repeated: %v", next.RDNSSWithdrawn)
	}

	// The last resolver withdrawn, with no other source: one zero-lifetime option.
	adv.update(rdnssSpec(time.Time{}))
	cfg = adv.build(now, false)
	if len(cfg.RDNSS) != 0 || !slices.Equal(cfg.RDNSSWithdrawn, []netip.Addr{netip.MustParseAddr("2001:db8:1::53")}) {
		t.Fatalf("live %v withdrawn %v, want none and [2001:db8:1::53]", cfg.RDNSS, cfg.RDNSSWithdrawn)
	}
	buf := make([]byte, ndp.RALen(cfg))
	if n := ndp.BuildRA(buf, 0, cfg); n != len(buf) {
		t.Fatalf("BuildRA wrote %d of %d octets", n, len(buf))
	}

	// A resolver dropped and advertised again in one step is live, not withdrawn.
	adv.sent(cfg)
	adv.update(rdnssSpec(time.Time{}, "2001:db8::53"))
	adv.update(rdnssSpec(time.Time{}, "2001:db8::54"))
	adv.update(rdnssSpec(time.Time{}, "2001:db8::53", "2001:db8::54"))
	if got := adv.build(now, false).RDNSSWithdrawn; len(got) != 0 {
		t.Fatalf("a re-advertised resolver was withdrawn: %v", got)
	}
}

// VALIDATES: AC-8, the lifetime half. A learned resolver is advertised with a
// Lifetime no larger than what remains of its lease at each send, rounded
// down, and 0 once the lease has ended; a server with no end keeps the
// unit's lifetime.
// PREVENTS: a host told to use a resolver past the lease that supplied it.
func TestRDNSSLifetimeFollowsLease(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name    string
		expires time.Time
		want    uint32
	}{
		{name: "no end", expires: time.Time{}, want: 1800},
		{name: "lease longer than lifetime", expires: now.Add(time.Hour), want: 1800},
		{name: "lease shorter, rounded down", expires: now.Add(600*time.Second + 900*time.Millisecond), want: 600},
		{name: "lease ended", expires: now.Add(-time.Second), want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adv := &advertiser{spec: rdnssSpec(tt.expires, "2001:db8::53")}
			if got := adv.build(now, false).RDNSSLifetime; got != tt.want {
				t.Fatalf("RDNSS lifetime = %d, want %d", got, tt.want)
			}
		})
	}
}

// rdnssAddrs answers n distinct resolver addresses 2001:db8:<block>::<i>.
func rdnssAddrs(block byte, n int) []netip.Addr {
	addrs := make([]netip.Addr, 0, n)
	for i := range n {
		raw := [16]byte{0x20, 0x01, 0x0d, 0xb8, 0, block}
		raw[15] = byte(i + 1)
		addrs = append(addrs, netip.AddrFrom16(raw))
	}
	return addrs
}

// rdnssSpecOf is rdnssSpec with the servers given as addresses.
func rdnssSpecOf(servers []netip.Addr) iface.RASenderSpec {
	spec := rdnssSpec(time.Time{})
	spec.Advertisement.RDNSS = slices.Clone(servers)
	return spec
}

// sendRA builds the advertisement at now, checks it encodes in full and
// carries at most rdnssWithdrawnPerRA withdrawals, records it as sent, and
// answers what it withdrew.
func sendRA(t *testing.T, adv *advertiser, now time.Time) []netip.Addr {
	t.Helper()
	cfg := adv.build(now, false)
	if len(cfg.RDNSSWithdrawn) > rdnssWithdrawnPerRA {
		t.Fatalf("one advertisement withdraws %d resolvers, want at most %d", len(cfg.RDNSSWithdrawn), rdnssWithdrawnPerRA)
	}
	buf := make([]byte, ndp.RALen(cfg))
	if n := ndp.BuildRA(buf, 0, cfg); n == 0 || n != len(buf) {
		t.Fatalf("BuildRA wrote %d of %d octets: the unit sends nothing", n, len(buf))
	}
	adv.sent(cfg)
	return cfg.RDNSSWithdrawn
}

// VALIDATES: finding 3 of the 23b review. Ten resolvers retired across two
// updates with no advertisement between them (the LAN link was down) go out
// over two advertisements, eight then two, each of which encodes; the unit
// then keeps advertising with nothing left to withdraw.
// PREVENTS: one advertisement that withdraws more than BuildRA accepts, after
// which the queue is never cleared and the unit never advertises again.
func TestRDNSSWithdrawalsSpreadAcrossAdvertisements(t *testing.T) {
	now := time.Now()
	first, second, third := rdnssAddrs(1, 5), rdnssAddrs(2, 5), rdnssAddrs(3, 5)
	adv := &advertiser{spec: rdnssSpecOf(first)}
	sendRA(t, adv, now)

	// The link goes down: two selection changes, nothing sent.
	if dropped := adv.update(rdnssSpecOf(second)); len(dropped) != 0 {
		t.Fatalf("dropped %v from a queue of 5", dropped)
	}
	if dropped := adv.update(rdnssSpecOf(third)); len(dropped) != 0 {
		t.Fatalf("dropped %v from a queue of 10", dropped)
	}

	// The link comes back.
	want := slices.Concat(first, second)
	var withdrawn []netip.Addr
	for range 2 {
		if !adv.urgent(now, false) {
			t.Fatalf("withdrawals still queued (%d sent of %d) but the advertisement is not urgent", len(withdrawn), len(want))
		}
		withdrawn = append(withdrawn, sendRA(t, adv, now)...)
	}
	if !slices.Equal(withdrawn, want) {
		t.Fatalf("withdrawn %v, want %v in order", withdrawn, want)
	}
	if adv.urgent(now, false) {
		t.Fatal("nothing left to withdraw, yet the advertisement is still urgent")
	}
	for range 3 {
		if got := sendRA(t, adv, now); len(got) != 0 {
			t.Fatalf("a withdrawal repeated: %v", got)
		}
	}
}

// VALIDATES: finding 3 of the 23b review, the bound. While the link is down
// the withdrawal queue holds at most retiringRDNSSMax resolvers; update
// answers the oldest it dropped beyond that, for the WARN line. After the
// link comes back the queue drains in the initial burst, every advertisement
// encodes, and the unit keeps advertising.
// PREVENTS: a queue that grows without bound while nothing can be sent.
func TestRDNSSWithdrawalQueueBounded(t *testing.T) {
	now := time.Now()
	const updates = 10
	adv := &advertiser{spec: rdnssSpecOf(rdnssAddrs(0, 4))}
	sendRA(t, adv, now)

	var retired, dropped []netip.Addr
	retired = append(retired, adv.spec.Advertisement.RDNSS...)
	for i := range updates {
		next := rdnssAddrs(byte(i+1), 4)
		dropped = append(dropped, adv.update(rdnssSpecOf(next))...)
		if len(adv.retiringRDNSS) > retiringRDNSSMax {
			t.Fatalf("update %d: queue holds %d, bound %d", i, len(adv.retiringRDNSS), retiringRDNSSMax)
		}
		if i < updates-1 {
			retired = append(retired, next...)
		}
	}
	over := len(retired) - retiringRDNSSMax
	if !slices.Equal(dropped, retired[:over]) {
		t.Fatalf("dropped %v, want the oldest %d: %v", dropped, over, retired[:over])
	}

	var withdrawn []netip.Addr
	for range ndp.MaxInitialAdvertisements {
		withdrawn = append(withdrawn, sendRA(t, adv, now)...)
	}
	if !slices.Equal(withdrawn, retired[over:]) {
		t.Fatalf("withdrawn %v, want the newest %d: %v", withdrawn, retiringRDNSSMax, retired[over:])
	}
	if got := sendRA(t, adv, now); len(got) != 0 {
		t.Fatalf("the initial burst left withdrawals: %v", got)
	}
}

// VALIDATES: an advertisement that could not be encoded never silences the
// unit. abandon drops every queued withdrawal and answers them for the log,
// and the next advertisement carries the live servers alone and encodes.
// PREVENTS: a send loop that returns before sent on every attempt, forever.
func TestRDNSSAbandonClearsWithdrawals(t *testing.T) {
	now := time.Now()
	live := rdnssAddrs(2, 2)
	adv := &advertiser{spec: rdnssSpecOf(rdnssAddrs(1, 3))}
	sendRA(t, adv, now)
	adv.update(rdnssSpecOf(live))

	if dropped := adv.abandon(); !slices.Equal(dropped, rdnssAddrs(1, 3)) {
		t.Fatalf("abandon answered %v, want the three queued", dropped)
	}
	cfg := adv.build(now, false)
	if len(cfg.RDNSSWithdrawn) != 0 || !slices.Equal(cfg.RDNSS, live) {
		t.Fatalf("after abandon: live %v withdrawn %v, want %v and none", cfg.RDNSS, cfg.RDNSSWithdrawn, live)
	}
	sendRA(t, adv, now)
}
