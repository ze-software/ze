// VALIDATES: RFC 5882 sec 4.4 -- multiple control protocols asking for a BFD
// session to the same remote system (same path/data protocol) share a SINGLE
// BFD session, and distinct remotes do not collapse onto one session.
// PREVENTS: creating a redundant second session per additional client (wasted
// state, duplicate Control packets) or, conversely, aliasing two peers onto one
// session so one peer's failure detection masks the other's.
package engine

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/bfd/api"
	"github.com/ze-software/ze/internal/component/bfd/transport"
	"github.com/ze-software/ze/internal/core/clock"
)

// RFC requirement: RFC5882-4.4-1 positive -- "If more than one control protocol
// desires a BFD session to a particular remote system, ... they MUST share a
// single BFD session" (RFC 5882 sec 4.4). EnsureSession
// (internal/component/bfd/engine/engine.go:344) keys a session by
// api.Key{Peer, Local, Interface, VRF, Mode} -- deliberately excluding timer
// parameters (internal/component/bfd/api/events.go:155-156) so two clients with
// the same path share one session. A second EnsureSession with the same Key
// bumps the existing session's refcount (machine.Acquire) and returns a handle
// to the SAME session; the session survives until the LAST client releases it.
func TestBFDSharedSessionSameKey(t *testing.T) {
	lb, _ := transport.Pair(api.SingleHop, netip.MustParseAddr(addrA), netip.MustParseAddr(addrB))
	loop := NewLoop(lb, clock.RealClock{})
	if err := loop.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() { _ = loop.Stop() }()

	req := reqFor("203.0.113.9", addrA)
	h1, err := loop.EnsureSession(req)
	if err != nil {
		t.Fatalf("EnsureSession #1: %v", err)
	}
	// A second control protocol asks for a session to the same remote/path.
	h2, err := loop.EnsureSession(req)
	if err != nil {
		t.Fatalf("EnsureSession #2 (same key): %v", err)
	}

	snap := loop.Snapshot()
	if len(snap) != 1 {
		t.Fatalf("two clients for the same remote created %d sessions, want 1 shared", len(snap))
	}
	if snap[0].Refcount != 2 {
		t.Fatalf("shared session refcount = %d, want 2 (one per client)", snap[0].Refcount)
	}

	// Releasing the first client MUST keep the shared session alive for the second.
	if err := loop.ReleaseSession(h1); err != nil {
		t.Fatalf("ReleaseSession h1: %v", err)
	}
	snap = loop.Snapshot()
	if len(snap) != 1 || snap[0].Refcount != 1 {
		t.Fatalf("after one release: sessions=%d refcount=%d, want 1 session at refcount 1", len(snap), refcountOf(snap))
	}

	// Only when the LAST client releases does the shared session tear down.
	if err := loop.ReleaseSession(h2); err != nil {
		t.Fatalf("ReleaseSession h2: %v", err)
	}
	if snap := loop.Snapshot(); len(snap) != 0 {
		t.Fatalf("after both releases: sessions=%d, want 0 (torn down at refcount 0)", len(snap))
	}
}

// RFC requirement: RFC5882-4.4-1 negative -- the single-session guarantee is
// scoped to the SAME remote system: two clients for DIFFERENT remotes get two
// distinct sessions, each at refcount 1. Sharing keys on the path (Peer), so it
// never aliases distinct peers onto one session (RFC 5882 sec 4.4).
func TestBFDDistinctSessionsDifferentKey(t *testing.T) {
	lb, _ := transport.Pair(api.SingleHop, netip.MustParseAddr(addrA), netip.MustParseAddr(addrB))
	loop := NewLoop(lb, clock.RealClock{})
	if err := loop.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() { _ = loop.Stop() }()

	if _, err := loop.EnsureSession(reqFor("203.0.113.9", addrA)); err != nil {
		t.Fatalf("EnsureSession peer .9: %v", err)
	}
	if _, err := loop.EnsureSession(reqFor("203.0.113.8", addrA)); err != nil {
		t.Fatalf("EnsureSession peer .8: %v", err)
	}

	snap := loop.Snapshot()
	if len(snap) != 2 {
		t.Fatalf("two DIFFERENT remotes produced %d sessions, want 2 distinct", len(snap))
	}
	for i := range snap {
		if snap[i].Refcount != 1 {
			t.Errorf("distinct session[%d] (%s) refcount = %d, want 1", i, snap[i].Peer, snap[i].Refcount)
		}
	}
}

// refcountOf returns the first session's refcount, or -1 when the slice is empty
// (so a failure message never panics on an out-of-range index).
func refcountOf(snap []api.SessionState) int {
	if len(snap) == 0 {
		return -1
	}
	return snap[0].Refcount
}
