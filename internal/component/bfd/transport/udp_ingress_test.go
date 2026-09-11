package transport

import (
	"net"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bfd/api"
	"github.com/ze-software/ze/internal/core/clock"
)

// VALIDATES: Inbound.Interface is stamped for a single-hop socket and NEVER for
// a multi-hop one, which is what the field's own documentation says and what
// the engine's first-packet index depends on.
// PREVENTS: the round-9 blocker returning. This is the ONLY discriminating
// proof of that rule: the engine now finds a session that named no interface
// whatever the packet carries (loop.go, the second lookup), so
// TestFirstPacketMultiHopIgnoresTheIngressInterface in the engine package
// cannot red if this stamp regresses. A multi-hop session is routed, so
// api.SessionRequest.Canonical clears its interface; an Inbound carrying one
// misses the exact five-field key and every RFC 5880 Section 6.8.6 lookup for a
// packet with Your Discriminator zero fails. Before round 8 this held because
// nothing set the field at all, and round 8 removed that accident.
func TestIngressInterfaceIsSingleHopOnly(t *testing.T) {
	links, err := net.Interfaces()
	if err != nil || len(links) == 0 {
		t.Skipf("no interfaces to resolve: %v", err)
	}
	real := links[0]

	single := &UDP{Mode: api.SingleHop}
	if got := single.ingressInterface(real.Index); got != real.Name {
		t.Errorf("single-hop ingress for index %d = %q, want %q", real.Index, got, real.Name)
	}

	multi := &UDP{Mode: api.MultiHop}
	if got := multi.ingressInterface(real.Index); got != "" {
		t.Errorf("multi-hop ingress = %q, want empty: a routed session's key carries no interface, so a stamped one makes every first-packet lookup miss", got)
	}
}

// VALIDATES: a failed ifindex lookup is not remembered. The cache holds answers,
// not the absence of one.
// PREVENTS: one transient netlink error permanently blanking an index, which
// would drop every zero-discriminator packet on that link for the life of the
// daemon. The stale-name half is a different repair with its own test,
// TestIngressInterfaceReresolvesAStaleName.
func TestIngressInterfaceDoesNotCacheAFailure(t *testing.T) {
	u := &UDP{Mode: api.SingleHop}
	// An index no kernel assigns, so the lookup fails.
	const absent = 1 << 24
	if got := u.ingressInterface(absent); got != "" {
		t.Fatalf("ingress for an absent index = %q, want empty", got)
	}
	u.ifNamesMu.RLock()
	_, cached := u.ifNames[absent]
	u.ifNamesMu.RUnlock()
	if cached {
		t.Error("the failed lookup was cached; the next packet on that index would be dropped without asking the kernel again")
	}
}

// VALIDATES: a cached ifindex-to-name answer expires, so a reused index is
// re-resolved rather than answered from the device that used to hold it.
// PREVENTS: finding 5 of round 11. An ifindex is reused after a veth or VLAN is
// deleted and recreated; a permanent cache then answers a name that no longer
// belongs to that index, the first-packet key stops matching, and every packet
// whose Your Discriminator is zero is dropped on that link.
func TestIngressInterfaceReresolvesAStaleName(t *testing.T) {
	links, err := net.Interfaces()
	if err != nil || len(links) == 0 {
		t.Skipf("no interfaces to resolve: %v", err)
	}
	real := links[0]

	// The injected clock drives expiry without sleeping, and without the test
	// depending on how long it takes to run.
	clk := &steppedClock{t: time.Date(2026, time.September, 11, 0, 0, 0, 0, time.UTC)}
	u := &UDP{Mode: api.SingleHop, Clock: clk, ifNames: map[int]ifName{
		// What a reused index looks like: the name of a device that held this
		// index before, resolved before the TTL elapsed.
		real.Index: {name: "gone0", at: clk.Now()},
	}}
	clk.advance(2 * ifNameTTL)
	if got := u.ingressInterface(real.Index); got != real.Name {
		t.Errorf("ingress for a stale entry = %q, want %q: the expired answer was served instead of re-resolving", got, real.Name)
	}

	// A fresh entry is still served without a netlink round trip, which is what
	// the cache is for.
	u.ifNames[real.Index] = ifName{name: "fresh0", at: clk.Now()}
	if got := u.ingressInterface(real.Index); got != "fresh0" {
		t.Errorf("ingress for a fresh entry = %q, want fresh0: the cache is not being used", got)
	}
}

// steppedClock is a clock.Clock whose Now only moves when the test moves it,
// so the ifindex cache's expiry is driven rather than waited for.
type steppedClock struct{ t time.Time }

func (c *steppedClock) Now() time.Time                              { return c.t }
func (c *steppedClock) Sleep(d time.Duration)                       { c.t = c.t.Add(d) }
func (c *steppedClock) After(d time.Duration) <-chan time.Time      { return time.After(d) }
func (c *steppedClock) AfterFunc(time.Duration, func()) clock.Timer { return nil }
func (c *steppedClock) NewTimer(time.Duration) clock.Timer          { return nil }
func (c *steppedClock) NewTicker(time.Duration) clock.Ticker        { return nil }
func (c *steppedClock) advance(d time.Duration)                     { c.t = c.t.Add(d) }
