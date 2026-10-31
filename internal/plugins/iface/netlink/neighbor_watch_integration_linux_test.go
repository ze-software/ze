//go:build linux

// Design: docs/features/interfaces.md -- Bus Topics: the neighbor table watch
// BGP runs for RFC 2545 Section 3's third-party Link-Local. This test drives
// WatchNeighbors against a real kernel neighbor table. It runs userns.Enter
// first, so it needs no host privilege and a refused operation fails it.

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

// TestIntegrationNeighbourWatchSeesPreexistingEntry proves the watch knows
// an entry that existed before it started, so that entry's deletion signals.
// Method: in a namespace of its own, add a PERMANENT IPv6 neighbor on a veth,
// THEN start the watch; wait for the signal the initial dump raises, delete
// the entry, and expect a second signal. BGP re-reads the table on each signal
// to pair a third-party global with its router's Link-Local (RFC 2545
// Section 3), so a missed delete leaves a stale Link-Local on the wire.
//
// MUTATION: drop ListExisting from WatchNeighbors' subscription options: the
// watch never learns the entry, the dump signal never comes, and the delete of
// an entry seen never held reports nothing.
func TestIntegrationNeighbourWatchSeesPreexistingEntry(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	veth := &netlink.Veth{LinkAttrs: netlink.LinkAttrs{Name: "w0"}, PeerName: "w1"}
	if err := netlink.LinkAdd(veth); err != nil {
		t.Fatalf("add veth: %v", err)
	}
	for _, name := range []string{"w0", "w1"} {
		link, err := netlink.LinkByName(name)
		if err != nil {
			t.Fatalf("look up %s: %v", name, err)
		}
		if err := netlink.LinkSetUp(link); err != nil {
			t.Fatalf("set %s up: %v", name, err)
		}
	}
	link, err := netlink.LinkByName("w0")
	if err != nil {
		t.Fatalf("look up w0: %v", err)
	}
	mac, err := net.ParseMAC("02:00:00:00:00:09")
	if err != nil {
		t.Fatalf("parse MAC: %v", err)
	}
	entry := &netlink.Neigh{
		LinkIndex: link.Attrs().Index, Family: unix.AF_INET6, State: netlink.NUD_PERMANENT,
		IP: net.ParseIP("2001:db8:1::9"), HardwareAddr: mac,
	}
	if err := netlink.NeighAdd(entry); err != nil {
		t.Fatalf("add neighbor: %v", err)
	}

	backend := &netlinkBackend{}
	t.Cleanup(func() { backend.Close() }) //nolint:errcheck // Close always answers nil
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	changes, err := backend.WatchNeighbors(ctx)
	if err != nil {
		t.Fatalf("start the watch: %v", err)
	}

	awaitNeighbourSignal(t, changes, "the initial dump raised no signal: the watch did not learn the existing entry")
	// The next signal must come from the delete alone: a quiet table first
	// shows no other change is on the way to satisfy the second wait.
	select {
	case <-changes:
		t.Fatal("an unrelated neighbor change arrived before the delete, so the delete's signal cannot be told apart")
	case <-time.After(500 * time.Millisecond):
	}
	if err := netlink.NeighDel(entry); err != nil {
		t.Fatalf("delete neighbor: %v", err)
	}
	awaitNeighbourSignal(t, changes, "deleting an entry that existed before the watch started raised no signal")
}

// awaitNeighbourSignal waits for one signal on changes, failing with why when
// none comes or the watch ends.
func awaitNeighbourSignal(t *testing.T, changes <-chan struct{}, why string) {
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
