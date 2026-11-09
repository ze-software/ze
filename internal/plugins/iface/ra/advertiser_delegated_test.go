package ifacera

import (
	"net/netip"
	"slices"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/core/ndp"
)

// minimumLinkMTU is the IPv6 minimum link MTU, and ipv6HeaderOctets the fixed
// header an advertisement travels in.
const (
	minimumLinkMTU   = 1280
	ipv6HeaderOctets = 40
)

// delegatedAdverts answers n adverts numbering distinct /64s
// 2001:db8:<block>00:<i>::1/64 from the delegation 2001:db8:<block>00::/56.
func delegatedAdverts(block byte, n int) []iface.DelegatedAdvert {
	delegated := netip.PrefixFrom(netip.AddrFrom16([16]byte{0x20, 0x01, 0x0d, 0xb8, block}), 56)
	adverts := make([]iface.DelegatedAdvert, 0, n)
	for i := range n {
		raw := [16]byte{0x20, 0x01, 0x0d, 0xb8, block, 0, 0, byte(i + 1)}
		raw[15] = 1
		address := netip.PrefixFrom(netip.AddrFrom16(raw), 64)
		adverts = append(adverts, iface.NewDelegatedAdvert("pppoe0", address, delegated,
			testNow.Add(2*time.Hour), testNow.Add(time.Hour)))
	}
	return adverts
}

// subnets answers the /64 each advert numbers, in order.
func subnets(adverts []iface.DelegatedAdvert) []netip.Prefix {
	out := make([]netip.Prefix, 0, len(adverts))
	for _, d := range adverts {
		out = append(out, d.Address.Masked())
	}
	return out
}

// delegatedSpecOf is a CE-role unit numbering from the given adverts.
func delegatedSpecOf(t *testing.T, adverts []iface.DelegatedAdvert) iface.RASenderSpec {
	t.Helper()
	spec := ceSpec(t, 0, 0, false)
	spec.Delegated = slices.Clone(adverts)
	return spec
}

// sendDelegatedRA builds the advertisement at testNow, checks it withdraws at
// most delegatedWithdrawnPerRA /64s, fits the minimum link MTU and encodes in
// full, records it as sent, and answers the /64s it withdrew in order.
func sendDelegatedRA(t *testing.T, adv *advertiser) []netip.Prefix {
	t.Helper()
	cfg := adv.build(testNow, true)
	var withdrawn []netip.Prefix
	for _, pi := range cfg.Prefixes {
		if pi.ValidLifetime == 0 {
			withdrawn = append(withdrawn, pi.Prefix)
		}
	}
	if len(withdrawn) > delegatedWithdrawnPerRA {
		t.Fatalf("one advertisement withdraws %d /64s, want at most %d", len(withdrawn), delegatedWithdrawnPerRA)
	}
	if need := ndp.RALen(cfg); need > minimumLinkMTU-ipv6HeaderOctets {
		t.Fatalf("advertisement needs %d octets, past the %d the minimum link MTU leaves", need, minimumLinkMTU-ipv6HeaderOctets)
	}
	buf := make([]byte, ndp.RALen(cfg))
	if n := ndp.BuildRA(buf, 0, cfg); n == 0 || n != len(buf) {
		t.Fatalf("BuildRA wrote %d of %d octets: the unit sends nothing", n, len(buf))
	}
	adv.sent(cfg)
	return withdrawn
}

// VALIDATES: ten delegated /64s retired across two updates with no
// advertisement between them (the LAN link was down) are withdrawn over two
// advertisements, eight then two, oldest first, each of which fits the
// minimum link MTU and encodes; urgent holds while any is queued, and the
// unit then keeps advertising with nothing left to withdraw.
// PREVENTS: one advertisement carrying every queued withdrawal, and a
// withdrawal that repeats or is forgotten before it went out.
func TestDelegatedWithdrawalsSpreadAcrossAdvertisements(t *testing.T) {
	first, second, third := delegatedAdverts(1, 5), delegatedAdverts(2, 5), delegatedAdverts(3, 5)
	adv := &advertiser{spec: delegatedSpecOf(t, first)}
	sendDelegatedRA(t, adv)

	// The link goes down: two replacements, nothing sent.
	if dropped := adv.update(delegatedSpecOf(t, second)).delegated; len(dropped) != 0 {
		t.Fatalf("dropped %v from a queue of 5", subnets(dropped))
	}
	if dropped := adv.update(delegatedSpecOf(t, third)).delegated; len(dropped) != 0 {
		t.Fatalf("dropped %v from a queue of 10", subnets(dropped))
	}

	// The link comes back.
	want := subnets(slices.Concat(first, second))
	var withdrawn []netip.Prefix
	for range 2 {
		if !adv.urgent(testNow, true) {
			t.Fatalf("withdrawals still queued (%d sent of %d) but the advertisement is not urgent", len(withdrawn), len(want))
		}
		withdrawn = append(withdrawn, sendDelegatedRA(t, adv)...)
	}
	if !slices.Equal(withdrawn, want) {
		t.Fatalf("withdrawn %v, want %v in order", withdrawn, want)
	}
	if adv.urgent(testNow, true) {
		t.Fatal("nothing left to withdraw, yet the advertisement is still urgent")
	}
	for range 3 {
		if got := sendDelegatedRA(t, adv); len(got) != 0 {
			t.Fatalf("a withdrawal repeated: %v", got)
		}
	}
}

// VALIDATES: the bound. While the link is down the delegated withdrawal queue
// holds at most retiringDelegatedMax /64s; update answers the oldest it
// dropped beyond that, for the WARN line. After the link comes back the queue
// drains in the initial burst, newest kept, and every advertisement encodes.
// PREVENTS: a queue that grows without bound while nothing can be sent.
func TestDelegatedWithdrawalQueueBounded(t *testing.T) {
	const updates = 10
	adv := &advertiser{spec: delegatedSpecOf(t, delegatedAdverts(0, 4))}
	sendDelegatedRA(t, adv)

	retired := subnets(adv.spec.Delegated)
	var dropped []netip.Prefix
	for i := range updates {
		next := delegatedAdverts(byte(i+1), 4)
		dropped = append(dropped, subnets(adv.update(delegatedSpecOf(t, next)).delegated)...)
		if len(adv.retiring) > retiringDelegatedMax {
			t.Fatalf("update %d: queue holds %d, bound %d", i, len(adv.retiring), retiringDelegatedMax)
		}
		if i < updates-1 {
			retired = append(retired, subnets(next)...)
		}
	}
	over := len(retired) - retiringDelegatedMax
	if !slices.Equal(dropped, retired[:over]) {
		t.Fatalf("dropped %v, want the oldest %d: %v", dropped, over, retired[:over])
	}

	var withdrawn []netip.Prefix
	for range ndp.MaxInitialAdvertisements {
		withdrawn = append(withdrawn, sendDelegatedRA(t, adv)...)
	}
	if !slices.Equal(withdrawn, retired[over:]) {
		t.Fatalf("withdrawn %v, want the newest %d: %v", withdrawn, retiringDelegatedMax, retired[over:])
	}
	if got := sendDelegatedRA(t, adv); len(got) != 0 {
		t.Fatalf("the initial burst left withdrawals: %v", got)
	}
}

// VALIDATES: a retired /64 that a later update delegates again is live: it
// leaves the queue, and the advertisement carries it once, with its live
// lifetimes, never also with zero lifetimes.
// PREVENTS: hosts told to deprecate and keep the same prefix in one message.
func TestDelegatedAdvertisedAgainLeavesQueue(t *testing.T) {
	first, second := delegatedAdverts(1, 2), delegatedAdverts(2, 1)
	adv := &advertiser{spec: delegatedSpecOf(t, first)}
	sendDelegatedRA(t, adv)

	adv.update(delegatedSpecOf(t, second))
	adv.update(delegatedSpecOf(t, slices.Concat(second, first[:1])))

	cfg := adv.build(testNow, true)
	count := 0
	for _, pi := range cfg.Prefixes {
		if pi.Prefix != first[0].Address.Masked() {
			continue
		}
		count++
		if pi.ValidLifetime != 7200 {
			t.Fatalf("re-delegated /64 Valid Lifetime %d, want the live 7200", pi.ValidLifetime)
		}
	}
	if count != 1 {
		t.Fatalf("re-delegated /64 advertised %d times, want once", count)
	}
	if got := sendDelegatedRA(t, adv); !slices.Equal(got, subnets(first[1:])) {
		t.Fatalf("withdrawn %v, want only %v", got, subnets(first[1:]))
	}
}

// VALIDATES: an advertisement that could not be encoded never silences the
// unit. abandon drops every queued /64 withdrawal and answers them for the
// log, and the next advertisement carries the live delegation alone.
// PREVENTS: a send loop that returns before sent on every attempt, forever.
func TestDelegatedAbandonClearsWithdrawals(t *testing.T) {
	retired, live := delegatedAdverts(1, 3), delegatedAdverts(2, 1)
	adv := &advertiser{spec: delegatedSpecOf(t, retired)}
	sendDelegatedRA(t, adv)
	adv.update(delegatedSpecOf(t, live))

	if dropped := adv.abandon().delegated; !slices.Equal(subnets(dropped), subnets(retired)) {
		t.Fatalf("abandon answered %v, want the three queued %v", subnets(dropped), subnets(retired))
	}
	if adv.urgent(testNow, true) {
		t.Fatal("abandon left the advertisement urgent")
	}
	if got := sendDelegatedRA(t, adv); len(got) != 0 {
		t.Fatalf("after abandon the advertisement still withdraws %v", got)
	}
}
