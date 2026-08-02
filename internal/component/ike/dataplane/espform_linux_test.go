//go:build linux

package dataplane

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/slogutil"
)

// VALIDATES: the receiver opens its sockets on the first watched SPI, keeps them while any
// SPI is watched, and releases them when the last one is forgotten.
// PREVENTS: holding a raw IPPROTO_ESP socket in a deployment whose SAs all run without an
// encapsulation template. That socket makes the kernel clone every bare ESP packet on the
// box, which taxes exactly the fast path this design exists to preserve.
func TestESPFormReceiverHoldsSocketsOnlyWhileWatching(t *testing.T) {
	peer := netip.MustParseAddr("198.51.100.7")
	local := netip.MustParseAddr("203.0.113.9")
	r := newESPFormReceiver(slogutil.DiscardLogger())

	if r.running() {
		t.Fatal("a new receiver already holds sockets")
	}

	if err := r.Watch(0x1111, peer, local); err != nil {
		t.Skipf("cannot open the raw ESP sockets (needs CAP_NET_RAW): %v", err)
	}
	if !r.running() {
		t.Fatal("the first watch did not start the receiver")
	}

	if err := r.Watch(0x2222, peer, local); err != nil {
		t.Fatalf("second watch: %v", err)
	}
	if !r.running() {
		t.Fatal("a second watch stopped the receiver")
	}

	r.Forget(0x1111)
	if !r.running() {
		t.Fatal("forgetting one of two SPIs released the sockets; the other SA loses its bare ESP form")
	}

	r.Forget(0x2222)
	if r.running() {
		t.Fatal("forgetting the last SPI left the sockets open")
	}

	// Close after a full release must not double-close, and must stay safe to call.
	if err := r.Close(); err != nil {
		t.Errorf("close after release: %v", err)
	}
}

// VALIDATES: a nil receiver has defined behavior on every method. Watch FAILS CLOSED, and
// the teardown methods are no-ops.
// PREVENTS: the panic this guard was written for. xfrmBackend is built as a bare
// &xfrmBackend{} literal in ten places, which leaves espForms nil, and RemoveSA and Close
// call into it unconditionally on a production path. Before the guard, tearing down any SA
// on such a backend crashed the process.
//
// Watch must not become a silent no-op. An SA that asked to receive both ESP forms and got
// one anyway drops the other form, and the tunnel then establishes and carries no traffic.
func TestESPFormReceiverNilIsFailClosed(t *testing.T) {
	var r *espFormReceiver

	if err := r.Watch(0x1111, netip.MustParseAddr("198.51.100.7"), netip.MustParseAddr("203.0.113.9")); err == nil {
		t.Error("a nil receiver accepted a watch; the SA would silently receive one ESP form only")
	}
	if r.running() {
		t.Error("a nil receiver reported that it is running")
	}

	// The teardown path must never be the thing that crashes.
	r.Forget(0x1111)
	if err := r.Close(); err != nil {
		t.Errorf("close on a nil receiver: %v", err)
	}
}

// VALIDATES: Close releases the sockets and is safe while SPIs are still watched.
// PREVENTS: a backend shutdown leaking the reader goroutine and its two raw sockets.
func TestESPFormReceiverCloseWhileWatching(t *testing.T) {
	peer := netip.MustParseAddr("198.51.100.7")
	local := netip.MustParseAddr("203.0.113.9")
	r := newESPFormReceiver(slogutil.DiscardLogger())

	if err := r.Watch(0x3333, peer, local); err != nil {
		t.Skipf("cannot open the raw ESP sockets (needs CAP_NET_RAW): %v", err)
	}
	if err := r.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if r.running() {
		t.Fatal("close left the sockets open")
	}
	if _, ok := r.reg.target(0x3333); ok {
		t.Error("close left an SPI watched, so a later Watch would not restart the receiver")
	}
}
