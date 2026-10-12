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

// RFC requirement: RFC5882-4.4-1 positive -- "If multiple control protocols
// wish to establish BFD sessions with the same remote system for the same data
// protocol, all MUST share a single BFD session" (RFC 5882 sec 4.4).
// EnsureSession
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

// RFC requirement: RFC5882-4.4-1 positive -- "If multiple control protocols
// wish to establish BFD sessions with the same remote system for the same data
// protocol, all MUST share a single BFD session" (RFC 5882 sec 4.4).
// A client pinned to a local address that joins a shared session narrows the
// identity the session answers for; when that client leaves, the narrowing
// leaves with it (Loop.ReleaseSession recomputes the entry's joined key from
// the clients still attached). A third client pinned to another local address
// then shares the one session rather than opening a second session to the same
// remote system. The negative half keeps RFC 5883 sec 4.1: while the pinned
// client is still attached, the session runs on its address pair, and a client
// pinned elsewhere is a distinct address pair with its own session.
func TestSharedSessionWidensOnRelease(t *testing.T) {
	const peer = "203.0.113.9"
	a, b, c := multiHop(peer, ""), multiHop(peer, "172.30.0.2"), multiHop(peer, "10.1.0.2")

	t.Run("pinned client left", func(t *testing.T) {
		loop := joinLoop(api.MultiHop)
		if _, err := loop.EnsureSession(a); err != nil {
			t.Fatalf("EnsureSession A: %v", err)
		}
		hb, err := loop.EnsureSession(b)
		if err != nil {
			t.Fatalf("EnsureSession B: %v", err)
		}
		if err := loop.ReleaseSession(hb); err != nil {
			t.Fatalf("ReleaseSession B: %v", err)
		}
		entry, ok := loop.sessions[a.Key()]
		if !ok {
			t.Fatalf("session opened by A is gone after B left")
		}
		if entry.joined != a.Key() {
			t.Errorf("joined after B left = %+v, want A's key %+v", entry.joined, a.Key())
		}
		hc, err := loop.EnsureSession(c)
		if err != nil {
			t.Fatalf("EnsureSession C: %v", err)
		}
		if hc.Key() != a.Key() {
			t.Errorf("C's handle names %+v, want the shared session %+v", hc.Key(), a.Key())
		}
		if snap := loop.Snapshot(); len(snap) != 1 || snap[0].Refcount != 2 {
			t.Fatalf("after C joined: refcounts %v, want ONE session at 2 (RFC 5882 sec 4.4)", refcounts(snap))
		}
	})

	t.Run("pinned client still attached", func(t *testing.T) {
		snap := ensureAll(t, joinLoop(api.MultiHop), a, b, c)
		if len(snap) != 2 {
			t.Fatalf("B on 172.30.0.2 attached, C on 10.1.0.2: %d sessions, want 2 (RFC 5883 sec 4.1), refcounts %v", len(snap), refcounts(snap))
		}
	})
}
