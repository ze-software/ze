//go:build linux

package ifacenetlink

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

// neighUpdate builds one netlink neighbor notification on link 7.
func neighUpdate(msgType uint16, ip, mac string, state int) netlink.NeighUpdate {
	nu := netlink.NeighUpdate{Type: msgType, Neigh: netlink.Neigh{LinkIndex: 7, IP: net.ParseIP(ip), State: state}}
	if mac != "" {
		hw, err := net.ParseMAC(mac)
		if err != nil {
			panic("BUG: test MAC " + mac + ": " + err.Error())
		}
		nu.HardwareAddr = hw
	}
	return nu
}

// newTestNeighborWatch builds a watch with no kernel subscription. A stand-in
// for the subscription closes updates once stop is closed, as the netlink
// library does when release closes its socket.
func newTestNeighborWatch(t *testing.T) *neighborWatch {
	t.Helper()
	w := &neighborWatch{
		updates: make(chan netlink.NeighUpdate, neighborWatchQueueDepth),
		stop:    make(chan struct{}),
		changes: make(chan struct{}, 1),
		seen:    make(map[neighKey]neighborSeen),
	}
	stopped := make(chan struct{})
	go closeUpdatesOnStop(w, stopped)
	t.Cleanup(func() { <-stopped })
	return w
}

// closeUpdatesOnStop is the test stand-in for the subscription's socket close.
func closeUpdatesOnStop(w *neighborWatch, stopped chan<- struct{}) {
	<-w.stop
	close(w.updates)
	close(stopped)
}

// TestNeighbourWatchNotesPairingChanges proves the watch reports a change for
// a GLOBAL IPv6 neighbor as well as a Link-Local one, on add, MAC change, a
// resolved/unresolved crossing and delete, and stays silent on a flip inside
// one class (REACHABLE to STALE), on a repeat of the same state, on a delete
// of an entry it never saw, and for IPv4. BGP re-reads the neighbor table on
// each report for RFC 2545 Section 3's third-party Link-Local.
func TestNeighbourWatchNotesPairingChanges(t *testing.T) {
	w := newTestNeighborWatch(t)
	t.Cleanup(func() { close(w.stop) })

	const global, mac, other = "2001:db8:1::9", "02:00:00:00:00:09", "02:00:00:00:00:0a"
	steps := []struct {
		name    string
		update  netlink.NeighUpdate
		reports bool
	}{
		{"global add", neighUpdate(unix.RTM_NEWNEIGH, global, mac, netlink.NUD_REACHABLE), true},
		{"reachable to stale", neighUpdate(unix.RTM_NEWNEIGH, global, mac, netlink.NUD_STALE), false},
		{"same state again", neighUpdate(unix.RTM_NEWNEIGH, global, mac, netlink.NUD_STALE), false},
		{"mac change", neighUpdate(unix.RTM_NEWNEIGH, global, other, netlink.NUD_STALE), true},
		{"to failed", neighUpdate(unix.RTM_NEWNEIGH, global, "", netlink.NUD_FAILED), true},
		{"back to reachable", neighUpdate(unix.RTM_NEWNEIGH, global, other, netlink.NUD_REACHABLE), true},
		{"link-local add", neighUpdate(unix.RTM_NEWNEIGH, "fe80::9", other, netlink.NUD_PERMANENT), true},
		{"global delete", neighUpdate(unix.RTM_DELNEIGH, global, other, netlink.NUD_REACHABLE), true},
		{"unknown delete", neighUpdate(unix.RTM_DELNEIGH, "2001:db8:1::8", other, netlink.NUD_REACHABLE), false},
		{"ipv4 ignored", neighUpdate(unix.RTM_NEWNEIGH, "192.0.2.9", mac, netlink.NUD_REACHABLE), false},
	}
	for _, step := range steps {
		if got := w.note(step.update); got != step.reports {
			t.Fatalf("after %q: reported %v, want %v", step.name, got, step.reports)
		}
	}
	if _, kept := w.seen[neighKey{linkIndex: 7, ip: global}]; kept {
		t.Error("a deleted entry stays in the watch's table, which is then unbounded")
	}
}

// TestNeighbourWatchDumpEntryDeleteSignals proves an entry the initial dump
// reported is known to the watch, so its later RTM_DELNEIGH signals. The dump
// reaches note as an RTM_NEWNEIGH, the message type the kernel answers
// RTM_GETNEIGH with; the real-kernel half (that WatchNeighbors asks for the
// dump) is TestIntegrationNeighbourWatchSeesPreexistingEntry.
func TestNeighbourWatchDumpEntryDeleteSignals(t *testing.T) {
	w := newTestNeighborWatch(t)
	t.Cleanup(func() { close(w.stop) })

	const global, mac = "2001:db8:1::9", "02:00:00:00:00:09"
	if !w.note(neighUpdate(unix.RTM_NEWNEIGH, global, mac, netlink.NUD_PERMANENT)) {
		t.Fatal("the dump entry reported nothing: the watch did not learn it")
	}
	if !w.note(neighUpdate(unix.RTM_DELNEIGH, global, mac, netlink.NUD_PERMANENT)) {
		t.Fatal("deleting the dumped entry reported nothing, so BGP keeps its stale Link-Local")
	}
}

// TestNeighbourWatchInterruptedDumpSignalsUnknownDelete proves that once the
// kernel flags the initial dump as interrupted (NLM_F_DUMP_INTR, which the
// netlink library reports as ErrDumpInterrupted), a delete of an entry the
// watch never saw signals: the dump may have skipped it. The interruption is
// not recorded as the watch's failure, because the subscription goes on.
func TestNeighbourWatchInterruptedDumpSignalsUnknownDelete(t *testing.T) {
	w := newTestNeighborWatch(t)
	t.Cleanup(func() { close(w.stop) })

	unknown := neighUpdate(unix.RTM_DELNEIGH, "2001:db8:1::8", "02:00:00:00:00:08", netlink.NUD_REACHABLE)
	if w.note(unknown) {
		t.Fatal("before any interrupted dump, a delete of an unknown entry reported a change")
	}
	w.recordFailure(netlink.ErrDumpInterrupted)
	if !w.note(unknown) {
		t.Fatal("after an interrupted dump, a delete of an entry the dump may have skipped reported nothing")
	}
	if w.failure != nil {
		t.Fatalf("an interrupted dump was recorded as the watch's failure: %v", w.failure)
	}
}

// TestNeighbourWatchSignalsAndCloses drives run as WatchNeighbors starts it:
// a burst of changes leaves one coalesced signal and never blocks the watch,
// and closing the backend closes the channel only after the subscription has
// been released. A caller relies on that close to know the table is no longer
// watched and to start a new watch.
func TestNeighbourWatchSignalsAndCloses(t *testing.T) {
	w := newTestNeighborWatch(t)
	var closed closeSignal
	finished := make(chan struct{})
	go runTestNeighborWatch(w, closed.channel(), finished)

	// Ten crossings between resolved and unresolved: each one is a change.
	for i := range 10 {
		state := netlink.NUD_REACHABLE
		if i%2 == 1 {
			state = netlink.NUD_FAILED
		}
		w.updates <- neighUpdate(unix.RTM_NEWNEIGH, "2001:db8:1::9", "02:00:00:00:00:09", state)
	}
	select {
	case _, open := <-w.changes:
		if !open {
			t.Fatal("the channel closed while the watch was live")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no signal after ten changes")
	}

	closed.close()
	closed.close() // a second Close of the backend does nothing
	select {
	case <-finished:
	case <-time.After(5 * time.Second):
		t.Fatal("the watch did not end when its backend closed")
	}
	for range w.changes {
		// A signal raised before the close may still be pending; the channel
		// must then close.
	}
	select {
	case <-w.stop:
	default:
		t.Fatal("the watch ended without releasing its subscription")
	}
}

// TestNeighbourWatchSignalCoalesces proves the watch's signal coalesces: a
// burst of changes while one signal is pending leaves exactly one pending,
// and never blocks the watch's read of the kernel subscription.
func TestNeighbourWatchSignalCoalesces(t *testing.T) {
	w := &neighborWatch{changes: make(chan struct{}, 1)}
	for range 100 {
		w.signal()
	}
	if got := len(w.changes); got != 1 {
		t.Fatalf("%d pending signals after a burst, want 1", got)
	}
}

// TestNeighbourWatchEndsWithContext proves the watch ends with its caller's
// context, closing the channel the caller ranges over.
func TestNeighbourWatchEndsWithContext(t *testing.T) {
	w := newTestNeighborWatch(t)
	ctx, cancel := context.WithCancel(context.Background())
	finished := make(chan struct{})
	go runTestNeighborWatchContext(ctx, w, finished)
	cancel()
	select {
	case <-finished:
	case <-time.After(5 * time.Second):
		t.Fatal("the watch did not end with its context")
	}
	if _, open := <-w.changes; open {
		t.Fatal("the channel stayed open after the watch ended")
	}
}

// TestNeighbourWatchEndsWhenSubscriptionFails proves a subscription that
// closes on its own (a receive error, an overrun) ends the watch and closes
// the channel, so the caller learns the table is no longer watched.
func TestNeighbourWatchEndsWhenSubscriptionFails(t *testing.T) {
	w := &neighborWatch{
		updates: make(chan netlink.NeighUpdate, 1),
		stop:    make(chan struct{}),
		changes: make(chan struct{}, 1),
		seen:    make(map[neighKey]neighborSeen),
	}
	w.recordFailure(unix.ENOBUFS)
	close(w.updates)
	finished := make(chan struct{})
	go runTestNeighborWatch(w, nil, finished)
	select {
	case <-finished:
	case <-time.After(5 * time.Second):
		t.Fatal("the watch did not end when its subscription closed")
	}
	if _, open := <-w.changes; open {
		t.Fatal("the channel stayed open after the subscription closed")
	}
}

// runTestNeighborWatch runs w under a context that never ends.
func runTestNeighborWatch(w *neighborWatch, closed <-chan struct{}, finished chan<- struct{}) {
	w.run(context.Background(), closed)
	close(finished)
}

// runTestNeighborWatchContext runs w under ctx with a backend that never closes.
func runTestNeighborWatchContext(ctx context.Context, w *neighborWatch, finished chan<- struct{}) {
	w.run(ctx, nil)
	close(finished)
}
