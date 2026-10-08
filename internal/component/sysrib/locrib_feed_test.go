// Design: docs/architecture/core-design.md -- System RIB plugin, the Loc-RIB feed.
package sysrib

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/rib/locrib"
)

// A Loc-RIB change that does not fit in the feed still reaches the FIB.
//
// Method: a feed one change deep takes two, as a Reselect burst would hand it.
// The worker's step (process the queued change, then resync the overflow) must
// publish both prefixes, the second as the Loc-RIB holds it at the read. Before
// the feed recorded overflow, the second change was dropped with a warning and
// the kernel never followed that winner.
func TestFeedOverflowIsReReadNotDropped(t *testing.T) {
	loc := locrib.NewRIB()
	SetLocRIB(loc)
	t.Cleanup(func() { SetLocRIB(nil) })
	bus := newTestEventBus()
	setEventBus(bus)
	t.Cleanup(clearEventBus)
	s := newSysRIB()

	feed := newLocRIBFeed(1)
	unsubscribe := loc.OnChange(func(c locrib.Change) { feed.offer(&c) })
	t.Cleanup(unsubscribe)

	queued := netip.MustParsePrefix("198.51.100.0/24")
	overflowed := netip.MustParsePrefix("203.0.113.0/24")
	loc.Insert(family.IPv4Unicast, queued, recursivePath(netip.MustParseAddr("192.0.2.1"), 1))
	loc.Insert(family.IPv4Unicast, overflowed, recursivePath(netip.MustParseAddr("192.0.2.2"), 1))

	c := <-feed.ch
	s.processLocRIBChange(&c)
	s.resyncOverflow(loc, feed)

	published := map[netip.Prefix]netip.Addr{}
	for _, change := range publishedChanges(t, bus) {
		published[change.Prefix] = change.NextHop
	}
	if published[queued] != netip.MustParseAddr("192.0.2.1") {
		t.Fatalf("queued change: published %v", published)
	}
	if published[overflowed] != netip.MustParseAddr("192.0.2.2") {
		t.Fatalf("overflowed change never reached the FIB: published %v", published)
	}
}

// The overflow is not read while an older change is still queued.
//
// Method: overflow one prefix while the channel holds a change, and check that
// takeOverflow answers nothing until the channel is drained. Reading the Loc-RIB
// first and then applying the older queued change would leave the FIB on the
// older state.
func TestFeedOverflowWaitsForTheQueue(t *testing.T) {
	feed := newLocRIBFeed(1)
	first := locrib.Change{Family: family.IPv4Unicast, Prefix: netip.MustParsePrefix("198.51.100.0/24"), Kind: locrib.ChangeAdd}
	second := locrib.Change{Family: family.IPv4Unicast, Prefix: netip.MustParsePrefix("203.0.113.0/24"), Kind: locrib.ChangeAdd}
	feed.offer(&first)
	feed.offer(&second)

	if keys := feed.takeOverflow(); keys != nil {
		t.Fatalf("overflow taken with a change still queued: %v", keys)
	}
	<-feed.ch
	keys := feed.takeOverflow()
	if len(keys) != 1 || keys[0].prefix != second.Prefix {
		t.Fatalf("overflow after drain = %v, want the second prefix", keys)
	}
	if again := feed.takeOverflow(); again != nil {
		t.Fatalf("overflow taken twice: %v", again)
	}
}

// An overflow recorded after the worker drained the queue still wakes it.
//
// Method: fill the one-slot feed, drain it as the worker would, and only then
// record an overflow, the interleaving where no later queued change exists to
// trigger the resync. The feed must hold a wake for the worker.
func TestFeedOverflowWakesTheWorker(t *testing.T) {
	feed := newLocRIBFeed(1)
	first := locrib.Change{Family: family.IPv4Unicast, Prefix: netip.MustParsePrefix("198.51.100.0/24"), Kind: locrib.ChangeAdd}
	feed.offer(&first)
	<-feed.ch
	feed.ch <- first // the slot refills before the next offer, so that offer overflows
	second := locrib.Change{Family: family.IPv4Unicast, Prefix: netip.MustParsePrefix("203.0.113.0/24"), Kind: locrib.ChangeAdd}
	feed.offer(&second)
	<-feed.ch
	select {
	case <-feed.wake:
	default:
		t.Fatal("an overflow recorded after the queue drained left the worker asleep")
	}
	if keys := feed.takeOverflow(); len(keys) != 1 || keys[0].prefix != second.Prefix {
		t.Fatalf("overflow on wake = %v, want the second prefix", keys)
	}
}
