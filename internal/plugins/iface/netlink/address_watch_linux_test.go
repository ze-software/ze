//go:build linux

package ifacenetlink

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/test/userns"
)

// TestIntegrationAddressWatchSignalsAddAndDelete proves the registered address
// watcher hears the kernel address table with no interface monitor running.
// Method: in a namespace of its own, start watchKernelAddresses, add an IPv4
// address to a dummy link and expect a signal, then delete it and expect a
// second. BGP re-reads the table on each signal to start a listener for an
// address added after start and to re-settle RFC 2545 Section 3's connected
// subnets, so a watch that hears nothing leaves both stale.
//
// MUTATION: make watchKernelAddresses' run loop drop updates without calling
// signal: neither signal comes.
func TestIntegrationAddressWatchSignalsAddAndDelete(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	dummy := &netlink.Dummy{LinkAttrs: netlink.LinkAttrs{Name: "aw0"}}
	if err := netlink.LinkAdd(dummy); err != nil {
		t.Fatalf("add dummy: %v", err)
	}
	link, err := netlink.LinkByName("aw0")
	if err != nil {
		t.Fatalf("look up aw0: %v", err)
	}
	if err := netlink.LinkSetUp(link); err != nil {
		t.Fatalf("set aw0 up: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	changes, err := watchKernelAddresses(ctx)
	if err != nil {
		t.Fatalf("start the watch: %v", err)
	}
	// Bringing the link up can itself report addresses (none on IPv4 for a
	// dummy, an IPv6 link-local maybe), so drain until the table is quiet and
	// the next signal can only come from the change below.
	drainAddressSignals(changes)

	address, err := netlink.ParseAddr("192.0.2.1/24")
	if err != nil {
		t.Fatalf("parse address: %v", err)
	}
	if err := netlink.AddrAdd(link, address); err != nil {
		t.Fatalf("add address: %v", err)
	}
	awaitAddressSignal(t, changes, "adding an address raised no signal")
	drainAddressSignals(changes)
	if err := netlink.AddrDel(link, address); err != nil {
		t.Fatalf("delete address: %v", err)
	}
	awaitAddressSignal(t, changes, "deleting an address raised no signal")

	cancel()
	for range changes {
		// A signal raised before the cancel may still be pending; the channel
		// must then close.
	}
}

// drainAddressSignals takes every signal that arrives until none has for half
// a second.
func drainAddressSignals(changes <-chan struct{}) {
	for {
		select {
		case <-changes:
		case <-time.After(500 * time.Millisecond):
			return
		}
	}
}

// awaitAddressSignal waits for one signal on changes, failing with why when
// none comes or the watch ends.
func awaitAddressSignal(t *testing.T, changes <-chan struct{}, why string) {
	t.Helper()
	select {
	case _, open := <-changes:
		if !open {
			t.Fatal("the watch ended while it should still run")
		}
	case <-time.After(5 * time.Second):
		t.Fatal(why)
	}
}

// newTestAddressWatch builds a watch with no kernel subscription. A stand-in
// for the subscription closes updates once stop is closed, as the netlink
// library does when release closes its socket.
func newTestAddressWatch(t *testing.T) *addressWatch {
	t.Helper()
	w := newAddressWatch()
	stopped := make(chan struct{})
	go closeAddressUpdatesOnStop(w, stopped)
	t.Cleanup(func() { <-stopped })
	return w
}

// closeAddressUpdatesOnStop is the test stand-in for the subscription's socket
// close.
func closeAddressUpdatesOnStop(w *addressWatch, stopped chan<- struct{}) {
	<-w.stop
	close(w.updates)
	close(stopped)
}

// TestAddressWatchCoalescesAndEndsWithContext proves a burst of updates leaves
// one pending signal and never blocks the read of the subscription, and that
// the watch ends with its context, releasing the subscription before it
// closes the channel.
func TestAddressWatchCoalescesAndEndsWithContext(t *testing.T) {
	w := newTestAddressWatch(t)
	ctx, cancel := context.WithCancel(context.Background())
	finished := make(chan struct{})
	go runTestAddressWatch(ctx, w, finished)
	ip := net.ParseIP("192.0.2.1")
	for range addressWatchQueueDepth {
		w.updates <- netlink.AddrUpdate{LinkAddress: net.IPNet{IP: ip, Mask: net.CIDRMask(24, 32)}, NewAddr: true}
	}
	select {
	case _, open := <-w.changes:
		if !open {
			t.Fatal("the channel closed while the watch was live")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no signal after a burst of updates")
	}
	cancel()
	select {
	case <-finished:
	case <-time.After(5 * time.Second):
		t.Fatal("the watch did not end with its context")
	}
	for range w.changes {
		// A signal raised before the cancel may still be pending.
	}
	select {
	case <-w.stop:
	default:
		t.Fatal("the watch ended without releasing its subscription")
	}
}

// TestAddressWatchEndsWhenSubscriptionFails proves a subscription that closes
// on its own (a receive error, an overrun) ends the watch and closes the
// channel, so the caller learns the table is no longer watched.
func TestAddressWatchEndsWhenSubscriptionFails(t *testing.T) {
	w := newAddressWatch()
	w.recordFailure(unix.ENOBUFS)
	close(w.updates)
	finished := make(chan struct{})
	go runTestAddressWatch(context.Background(), w, finished)
	select {
	case <-finished:
	case <-time.After(5 * time.Second):
		t.Fatal("the watch did not end when its subscription closed")
	}
	if _, open := <-w.changes; open {
		t.Fatal("the channel stayed open after the subscription closed")
	}
}

// runTestAddressWatch runs w under ctx and closes finished after it.
func runTestAddressWatch(ctx context.Context, w *addressWatch, finished chan<- struct{}) {
	w.run(ctx)
	close(finished)
}
