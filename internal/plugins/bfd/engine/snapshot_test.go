package engine

import (
	"net/netip"
	"sync"
	"testing"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/core/clock"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bfd/api"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bfd/transport"
)

// VALIDATES: Loop.Snapshot returns an empty slice when no sessions
// have been created yet. The method must not panic on a freshly-
// initialized Loop.
// PREVENTS: nil-slice surprises in the callers that format the output
// (encoding/json emits "null" for a nil slice but "[]" for an empty).
func TestLoopSnapshotEmpty(t *testing.T) {
	lb, _ := transport.Pair(api.SingleHop, netip.MustParseAddr(addrA), netip.MustParseAddr(addrB))
	loop := NewLoop(lb, clock.RealClock{})
	if err := loop.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() { _ = loop.Stop() }()

	snap := loop.Snapshot()
	if len(snap) != 0 {
		t.Fatalf("Snapshot len = %d, want 0", len(snap))
	}
}

// VALIDATES: Loop.Snapshot returns one SessionState per live session,
// sorted deterministically (mode, vrf, peer), with the profile name
// and timer fields propagated from the request.
// PREVENTS: regression where multi-session Snapshot is non-deterministic
// or drops entries.
func TestLoopSnapshotTwoSessions(t *testing.T) {
	lb, _ := transport.Pair(api.SingleHop, netip.MustParseAddr(addrA), netip.MustParseAddr(addrB))
	loop := NewLoop(lb, clock.RealClock{})
	if err := loop.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() { _ = loop.Stop() }()

	for _, peer := range []string{"203.0.113.9", "203.0.113.8"} {
		req := reqFor(peer, addrA)
		req.Profile = "fast"
		if _, err := loop.EnsureSession(req); err != nil {
			t.Fatalf("EnsureSession %s: %v", peer, err)
		}
	}

	snap := loop.Snapshot()
	if len(snap) != 2 {
		t.Fatalf("Snapshot len = %d, want 2", len(snap))
	}
	// Sorted ascending by peer within the single-hop bucket.
	if snap[0].Peer != "203.0.113.8" || snap[1].Peer != "203.0.113.9" {
		t.Fatalf("sort: got %q, %q; want 203.0.113.8, 203.0.113.9", snap[0].Peer, snap[1].Peer)
	}
	for i := range snap {
		s := &snap[i]
		if s.Profile != "fast" {
			t.Errorf("[%d] Profile = %q, want fast", i, s.Profile)
		}
		if s.DetectMult != 3 {
			t.Errorf("[%d] DetectMult = %d, want 3", i, s.DetectMult)
		}
		if s.State != "down" {
			t.Errorf("[%d] State = %q, want down", i, s.State)
		}
		if s.LocalDiscr == 0 {
			t.Errorf("[%d] LocalDiscr = 0 (must be nonzero)", i)
		}
	}
}

// VALIDATES: Snapshot is safe to call concurrently with
// EnsureSession/ReleaseSession. The -race detector trips if the
// method walks the session map without holding l.mu, so this test
// catches any regression that drops the lock.
// PREVENTS: concurrent map iteration panics.
func TestLoopSnapshotConcurrent(t *testing.T) {
	lb, _ := transport.Pair(api.SingleHop, netip.MustParseAddr(addrA), netip.MustParseAddr(addrB))
	loop := NewLoop(lb, clock.RealClock{})
	if err := loop.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() { _ = loop.Stop() }()

	stop := make(chan struct{})
	var wg sync.WaitGroup

	wg.Go(func() {
		i := 0
		for {
			select {
			case <-stop:
				return
			default:
				peer := netip.AddrFrom4([4]byte{203, 0, 113, byte(9 + (i % 10))})
				req := api.SessionRequest{
					Peer:                  peer,
					Local:                 netip.MustParseAddr(addrA),
					Interface:             "loop",
					Mode:                  api.SingleHop,
					DesiredMinTxInterval:  10_000,
					RequiredMinRxInterval: 10_000,
					DetectMult:            3,
				}
				h, err := loop.EnsureSession(req)
				if err == nil {
					_ = loop.ReleaseSession(h)
				}
				i++
			}
		}
	})

	wg.Go(func() {
		for {
			select {
			case <-stop:
				return
			default:
				_ = loop.Snapshot()
			}
		}
	})

	time.Sleep(40 * time.Millisecond)
	close(stop)
	wg.Wait()
}

// VALIDATES: SessionDetail returns false for an unknown peer and the
// correct SessionState for a known peer (case-insensitive match).
// PREVENTS: lookup regressions when an operator types an address with
// any case variation or IPv6 abbreviation.
func TestLoopSessionDetail(t *testing.T) {
	lb, _ := transport.Pair(api.SingleHop, netip.MustParseAddr(addrA), netip.MustParseAddr(addrB))
	loop := NewLoop(lb, clock.RealClock{})
	if err := loop.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() { _ = loop.Stop() }()

	if _, err := loop.EnsureSession(reqFor("203.0.113.9", addrA)); err != nil {
		t.Fatalf("EnsureSession: %v", err)
	}

	if _, ok := loop.SessionDetail("198.51.100.1"); ok {
		t.Fatal("SessionDetail unknown peer returned ok=true")
	}
	s, ok := loop.SessionDetail("203.0.113.9")
	if !ok {
		t.Fatal("SessionDetail known peer returned ok=false")
	}
	if s.Peer != "203.0.113.9" {
		t.Fatalf("Peer = %q, want 203.0.113.9", s.Peer)
	}
}
