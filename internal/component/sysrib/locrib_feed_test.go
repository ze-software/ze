// Design: docs/architecture/core-design.md -- current local routing-view capture.

package sysrib

import (
	"context"
	"net/netip"
	"sync"
	"testing"
	"time"

	sysribevents "github.com/ze-software/ze/internal/component/sysrib/events"
	"github.com/ze-software/ze/internal/core/bgp/routeaction"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/rib/locrib"
)

// heldFeedWorker holds the actual worker's initial publication while a test
// fills its bounded input. The owner MUST release it before client cleanup.
func heldFeedWorker(t *testing.T, ctx context.Context, prefix netip.Prefix, path locrib.Path) (*routeViewClient, *locrib.RIB, *testEventBus, func()) {
	t.Helper()
	rib := locrib.NewRIB()
	rib.Insert(family.IPv4Unicast, prefix, path)
	bus := newTestEventBus()
	useForwardingBus(t, bus)
	entered, release := make(chan struct{}), make(chan struct{})
	var first, unblock sync.Once
	releaseWorker := func() { unblock.Do(func() { close(release) }) }
	bus.Subscribe(sysribevents.Namespace, sysribevents.EventBestChange, func(payload any) {
		batch, ok := payload.(*outgoingBatch)
		if !ok {
			return
		}
		for i := range batch.Changes {
			if batch.Changes[i].Prefix == prefix && batch.Changes[i].Action == routeaction.Add {
				first.Do(func() {
					close(entered)
					select {
					case <-release:
					case <-ctx.Done():
					}
				})
			}
		}
	})
	client := openCurrentSysRIB(t, ctx, rib, newSysRIB(), bus)
	t.Cleanup(releaseWorker)
	if err := client.Start(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case <-entered:
	case <-ctx.Done():
		t.Fatalf("initial useful publication never reached the hold: %v", client.Failure())
	}
	return client, rib, bus, releaseWorker
}

func TestFeedOverflowIsReReadNotDropped(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	queued := netip.MustParsePrefix("198.51.100.0/24")
	overflowed := netip.MustParsePrefix("203.0.113.0/24")
	path := recursivePath(netip.MustParseAddr("192.0.2.1"), 1)
	path.Source = redistevents.RegisterProtocol("test-nhresolver")
	client, rib, bus, release := heldFeedWorker(t, ctx, queued, path)
	capture := client.worker.local
	for range cap(capture.keys) {
		capture.offer(locrib.Change{Family: family.IPv4Unicast, Prefix: queued, Kind: locrib.ChangeAdd})
	}
	second := path
	second.NextHop = netip.MustParseAddr("192.0.2.2")
	rib.Insert(family.IPv4Unicast, overflowed, second)
	if !capture.overflow.Load() || len(capture.keys) != cap(capture.keys) {
		t.Fatal("second authoritative input did not overflow the bounded queue")
	}
	release()
	if _, ok := waitForEntry(t, bus, func(entry sysribevents.BestChangeEntry) bool {
		return entry.Prefix == overflowed && entry.Action == routeaction.Add && entry.NextHop == second.NextHop
	}); !ok {
		t.Fatalf("overflowed prefix never reached the FIB: %+v", bestChangeEntries(bus))
	}
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

// Capture holds keys rather than borrowed route values. A newer authority
// value must survive the queued key and the overflow scan in either order.
func TestFeedOverflowWaitsForTheQueue(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	first := locrib.Change{Family: family.IPv4Unicast, Prefix: netip.MustParsePrefix("198.51.100.0/24"), Kind: locrib.ChangeAdd}
	second := locrib.Change{Family: family.IPv4Unicast, Prefix: netip.MustParsePrefix("203.0.113.0/24"), Kind: locrib.ChangeAdd}
	path := recursivePath(netip.MustParseAddr("192.0.2.1"), 1)
	path.Source = redistevents.RegisterProtocol("test-nhresolver")
	client, rib, bus, release := heldFeedWorker(t, ctx, first.Prefix, path)
	capture := client.worker.local
	for range cap(capture.keys) {
		capture.offer(first)
	}
	other := path
	other.NextHop = netip.MustParseAddr("192.0.2.2")
	rib.Insert(second.Family, second.Prefix, other)
	newer := path
	newer.NextHop = netip.MustParseAddr("192.0.2.3")
	rib.Insert(first.Family, first.Prefix, newer)
	if !capture.overflow.Load() || len(capture.keys) != cap(capture.keys) {
		t.Fatal("test did not retain queued old input beside a newer authority value")
	}
	release()
	if _, ok := waitForEntry(t, bus, func(entry sysribevents.BestChangeEntry) bool {
		return entry.Prefix == first.Prefix && entry.Action == routeaction.Update && entry.NextHop == newer.NextHop
	}); !ok {
		t.Fatalf("queued old input overwrote the authority's latest value: %+v", bestChangeEntries(bus))
	}
	if _, ok := waitForEntry(t, bus, func(entry sysribevents.BestChangeEntry) bool {
		return entry.Prefix == second.Prefix && entry.Action == routeaction.Add && entry.NextHop == other.NextHop
	}); !ok {
		t.Fatalf("second overflowed input was lost: %+v", bestChangeEntries(bus))
	}
	waitForCurrentSysRIB(t, client)
	client.rib.mu.RLock()
	selected := client.rib.best[prefixKey{family: first.Family, prefix: first.Prefix}]
	client.rib.mu.RUnlock()
	if selected == nil || selected.nextHop != newer.NextHop || len(capture.keys) != 0 || capture.overflow.Load() {
		t.Fatalf("drained queue/overflow did not retain the latest selected value: %+v", selected)
	}
}

// Complete the real full-queue overflow operation only after the actual worker
// drained its keys. No later routing event may wake it for the second prefix.
func TestFeedOverflowWakesTheWorker(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	first := locrib.Change{Family: family.IPv4Unicast, Prefix: netip.MustParsePrefix("198.51.100.0/24"), Kind: locrib.ChangeAdd}
	second := locrib.Change{Family: family.IPv4Unicast, Prefix: netip.MustParsePrefix("203.0.113.0/24"), Kind: locrib.ChangeAdd}
	path := recursivePath(netip.MustParseAddr("192.0.2.1"), 1)
	path.Source = redistevents.RegisterProtocol("test-nhresolver")
	client, rib, bus, release := heldFeedWorker(t, ctx, first.Prefix, path)
	capture := client.worker.local
	capture.offer(first)
	<-capture.keys
	capture.offer(first)
	for range cap(capture.keys) - 1 {
		capture.offer(first)
	}
	if len(capture.keys) != cap(capture.keys) {
		t.Fatal("the interrupted second offer did not observe a full queue")
	}
	// Suspend the offer at its full-queue branch, as the controlled late-
	// overflow regression does. The authority changes without a second delivery.
	capture.unsubscribe()
	other := path
	other.NextHop = netip.MustParseAddr("192.0.2.2")
	rib.Insert(second.Family, second.Prefix, other)
	newer := path
	newer.NextHop = netip.MustParseAddr("192.0.2.3")
	rib.Insert(first.Family, first.Prefix, newer)
	release()
	if _, ok := waitForEntry(t, bus, func(entry sysribevents.BestChangeEntry) bool {
		return entry.Prefix == first.Prefix && entry.Action == routeaction.Update && entry.NextHop == newer.NextHop
	}); !ok {
		t.Fatalf("queued healthy control was not processed: %+v", bestChangeEntries(bus))
	}
	if len(capture.keys) != 0 {
		t.Fatal("worker did not drain the queue before late overflow")
	}
	capture.recordOverflow(prefixKey{family: second.Family, prefix: second.Prefix})
	if _, ok := waitForEntry(t, bus, func(entry sysribevents.BestChangeEntry) bool {
		return entry.Prefix == second.Prefix && entry.Action == routeaction.Add && entry.NextHop == other.NextHop
	}); !ok {
		t.Fatalf("late overflow left the worker asleep without another input: %+v", bestChangeEntries(bus))
	}
	client.rib.mu.RLock()
	control := client.rib.best[prefixKey{family: first.Family, prefix: first.Prefix}]
	client.rib.mu.RUnlock()
	if control == nil || control.nextHop != newer.NextHop {
		t.Fatal("overflow recovery lost the useful queued control")
	}
}
