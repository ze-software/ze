// VALIDATES: api.SessionHandle.Subscribe delivers the session's CURRENT state
// as its first value, marked Initial, and takes that snapshot and the
// subscriber registration as one atomic step.
// PREVENTS: two defects that each stall a client indefinitely. A client that
// joins a session another client already brought Up learns the state only from
// the next transition, which for a stable session never comes; and a
// transition landing between a released snapshot and the registration reaches
// no subscriber at all, leaving a stale snapshot in place forever.
package engine

import (
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bfd/api"
	"github.com/ze-software/ze/internal/component/bfd/packet"
	"github.com/ze-software/ze/internal/component/bfd/transport"
	"github.com/ze-software/ze/internal/core/clock"
)

// startTestLoop builds a running loop for one address pair.
func startTestLoop(t *testing.T) *Loop {
	t.Helper()
	lb, _ := transport.Pair(api.SingleHop, netip.MustParseAddr(addrA), netip.MustParseAddr(addrB))
	loop := NewLoop(lb, clock.RealClock{})
	if err := loop.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { _ = loop.Stop() })
	return loop
}

// firstChange reads one value with a deadline, so a missing snapshot fails the
// case rather than hanging it.
func firstChange(t *testing.T, ch <-chan api.StateChange) api.StateChange {
	t.Helper()
	select {
	case change := <-ch:
		return change
	case <-time.After(2 * time.Second):
		t.Fatal("Subscribe delivered no value")
	}
	return api.StateChange{}
}

// TestSubscribeDeliversTheCurrentStateFirst covers the never-transitioned time
// path: a session created and not yet moved reports the state it was created
// in, timed from its creation.
//
// VALIDATES: The first value carries Initial, the session's current state, and
// a When equal to the session's creation time rather than "now".
//
// PREVENTS: A snapshot timed at subscription, which would restart every
// draft-ietf-idr-bgp-bfd-strict-mode Section 10 hold-down interval on each new
// client, since that interval measures how long the session has been Up.
func TestSubscribeDeliversTheCurrentStateFirst(t *testing.T) {
	loop := startTestLoop(t)
	handle, err := loop.EnsureSession(reqFor("203.0.113.9", addrA))
	if err != nil {
		t.Fatalf("EnsureSession: %v", err)
	}

	loop.mu.Lock()
	created := loop.sessions[handle.Key()].createdAt
	loop.mu.Unlock()

	change := firstChange(t, handle.Subscribe())
	if !change.Initial {
		t.Fatal("the first value is a transition, so a client cannot tell it from a real one")
	}
	if change.State != packet.StateDown {
		t.Fatalf("snapshot state = %v, want Down for a session that has not come up", change.State)
	}
	if !change.When.Equal(created) {
		t.Fatalf("snapshot When = %v, want the creation time %v", change.When, created)
	}
	if change.Key != handle.Key() {
		t.Fatalf("snapshot key = %v, want %v", change.Key, handle.Key())
	}
}

// TestSubscribeReportsAStateAnotherClientAlreadyReached is the round-2 blocker
// at the producer: EnsureSession on an existing key only bumps a refcount, so
// the second client's whole knowledge of the session comes from this snapshot.
//
// VALIDATES: After a transition, a NEW subscriber's first value carries that
// state, its diagnostic, and the moment of the transition.
//
// PREVENTS: A client reading Down for a session that is Up. A BGP peer running
// BFD strict mode against a neighbor that also has a pinned `bfd { session
// ... }` entry is exactly that client, and it held its session out of
// Established for the life of the process.
func TestSubscribeReportsAStateAnotherClientAlreadyReached(t *testing.T) {
	loop := startTestLoop(t)
	handle, err := loop.EnsureSession(reqFor("203.0.113.9", addrA))
	if err != nil {
		t.Fatalf("EnsureSession: %v", err)
	}
	key := handle.Key()

	// The first client is already subscribed and the session moves under it.
	existing := handle.Subscribe()
	firstChange(t, existing) // its own snapshot

	loop.mu.Lock()
	entry := loop.sessions[key]
	notify := loop.makeNotify(key, entry)
	loop.mu.Unlock()

	loop.mu.Lock()
	notify(packet.StateUp, packet.DiagNone)
	loop.mu.Unlock()

	loop.mu.Lock()
	changed := entry.lastChange
	loop.mu.Unlock()

	// A second client joins the SAME session, as a strict BGP peer does.
	second, err := loop.EnsureSession(reqFor("203.0.113.9", addrA))
	if err != nil {
		t.Fatalf("EnsureSession #2: %v", err)
	}
	change := firstChange(t, second.Subscribe())
	if !change.Initial {
		t.Fatal("the joining client's first value is not marked as a snapshot")
	}
	if change.State != packet.StateUp {
		t.Fatalf("snapshot state = %v, want Up: the session was already up", change.State)
	}
	if !change.When.Equal(changed) {
		t.Fatalf("snapshot When = %v, want the transition time %v", change.When, changed)
	}
}

// TestSubscribeCarriesTheDiagnosticWithTheState holds the pair together.
//
// VALIDATES: A snapshot reports the diagnostic that came with the state.
//
// PREVENTS: A session sitting Down after a detection-time expiry reporting
// DiagNone to a joining client, which reads as "down for no reason" and is the
// one field that says WHY.
func TestSubscribeCarriesTheDiagnosticWithTheState(t *testing.T) {
	loop := startTestLoop(t)
	handle, err := loop.EnsureSession(reqFor("203.0.113.9", addrA))
	if err != nil {
		t.Fatalf("EnsureSession: %v", err)
	}
	key := handle.Key()

	loop.mu.Lock()
	notify := loop.makeNotify(key, loop.sessions[key])
	notify(packet.StateDown, packet.DiagControlDetectExpired)
	loop.mu.Unlock()

	change := firstChange(t, handle.Subscribe())
	if change.Diag != packet.DiagControlDetectExpired {
		t.Fatalf("snapshot diag = %v, want the diagnostic that came with the state", change.Diag)
	}
}

// TestSubscribeHoldsBothLocksAcrossTheRegistration is the round-3 blocker,
// asserted as the invariant rather than raced for.
//
// The snapshot read and the subscriber append must be ONE critical section. A
// transition that lands between them is written to a subscriber list the new
// channel is not yet in, so the subscriber keeps a snapshot that is already
// wrong and never learns it. For a BGP peer running BFD strict mode the handle
// outlives every connection retry, so a missed Up is not re-read until BFD next
// flaps, which for a stable link is never.
//
// VALIDATES: While a Subscribe is parked waiting for subsMu, l.mu is STILL
// HELD, so nothing can drive a session transition meanwhile. That is the
// property, and it is what a released-and-reacquired lock destroys.
//
// PREVENTS: A regression test that only races for the window. An earlier
// version of this case ran a transition from a second goroutine and asserted
// the subscriber caught up; it passed with the two steps split apart, because
// the window is nanoseconds wide and which goroutine wins the second mutex is a
// coin flip. Parking the subscription on subsMu makes the question decidable:
// with the steps split, l.mu is free and the check below acquires it at once.
func TestSubscribeHoldsBothLocksAcrossTheRegistration(t *testing.T) {
	loop := startTestLoop(t)
	handle, err := loop.EnsureSession(reqFor("203.0.113.9", addrA))
	if err != nil {
		t.Fatalf("EnsureSession: %v", err)
	}

	// Park every subscription: Subscribe cannot finish without subsMu.
	loop.subsMu.Lock()

	type subscription struct{ ch <-chan api.StateChange }
	subscribing := make(chan subscription, 1)
	go func() { subscribing <- subscription{ch: handle.Subscribe()} }()

	// Give the subscription time to reach the subsMu wait. It cannot proceed
	// past it, so a longer wait only makes the check surer.
	//
	// This is the one timing assumption here, and its failure direction is
	// safe: a goroutine too slow to reach the wait leaves l.mu free, so the
	// check below fires and the case FAILS. A false red, never a false green.
	time.Sleep(100 * time.Millisecond)

	// The session registry lock must be UNAVAILABLE: Subscribe holds it across
	// the append, which is what stops makeNotify running in the gap. makeNotify
	// is driven under this same lock (Loop.receive, Loop.tick).
	held := make(chan struct{})
	go func() {
		// Taken and released at once on purpose: the question is whether l.mu
		// is AVAILABLE while a subscription is parked on subsMu, so the
		// acquisition is the measurement and holding it would prove nothing.
		loop.mu.Lock()
		loop.mu.Unlock() //nolint:gocritic,staticcheck // the acquisition is the assertion, see above
		close(held)
	}()

	select {
	case <-held:
		loop.subsMu.Unlock()
		t.Fatal("l.mu was free while a Subscribe was mid-registration: a transition landing there " +
			"reaches a subscriber list the new channel is not yet in, and the subscriber keeps a stale snapshot forever")
	case <-time.After(300 * time.Millisecond):
	}

	// Release, and confirm the subscription completes and still carries its
	// snapshot: the invariant must not have been bought with a deadlock.
	loop.subsMu.Unlock()
	select {
	case got := <-subscribing:
		change := firstChange(t, got.ch)
		if !change.Initial {
			t.Fatal("the completed subscription delivered no snapshot")
		}
		handle.Unsubscribe(got.ch)
	case <-time.After(2 * time.Second):
		t.Fatal("Subscribe never completed after subsMu was released")
	}
	<-held
}
