// Design: docs/architecture/l2tp/subscriber-session-model.md -- read-only lifecycle ACK observations
package ppp

import (
	"context"
	"net/netip"
	"sync"
	"testing"
	"time"
)

type lifecycleAcknowledgedObserver interface {
	Acknowledged() <-chan struct{}
}

// Native operations use the existing FSM fixture. The actual NCP/TLD
// producers alone mint their receipt channels; no test writes private done,
// constructs Control/pending authority, or directly closes an ACK channel.
func newAcknowledgedEmitter(t *testing.T) (*pppSession, chan Event, func(func() bool) <-chan bool) {
	t.Helper()
	session, _, events := newRFC1661Session(LCPStateOpened)
	stopped, cancel := context.WithCancel(context.Background())
	session.stopCh = stopped.Done()
	session.ncpStarted = true
	session.localIPv4 = netip.MustParseAddr("192.0.2.1")
	session.peerIPv4 = netip.MustParseAddr("192.0.2.2")
	var workers sync.WaitGroup
	t.Cleanup(func() { cancel(); workers.Wait(); session.closeFiles() })
	produce := func(fn func() bool) <-chan bool {
		result := make(chan bool, 1)
		workers.Go(func() { result <- fn() })
		return result
	}
	return session, events, produce
}

func assertAcknowledgedReceipt(t *testing.T, original, copy any, acknowledge func(), done <-chan bool) {
	t.Helper()
	observer, ok := original.(lifecycleAcknowledgedObserver)
	if !ok {
		t.Fatalf("actual emitted %T lacks read-only Acknowledged observer", original)
	}
	copied, ok := copy.(lifecycleAcknowledgedObserver)
	if !ok {
		t.Fatalf("copied %T lost observer", copy)
	}
	channel := observer.Acknowledged()
	if channel == nil {
		t.Fatal("actual emitter did not retain original ACK channel")
	}
	if copied.Acknowledged() != channel {
		t.Fatal("event copy retargeted immutable ACK channel")
	}
	select {
	case <-channel:
		t.Fatal("original ACK closed before public Acknowledge")
	default:
	}
	select {
	case result := <-done:
		t.Fatalf("actual producer returned %v before acknowledgment", result)
	default:
	}
	acknowledge()
	select {
	case <-channel:
	default:
		t.Fatal("public Acknowledge did not close observed original channel")
	}
	if copied.Acknowledged() != channel || observer.Acknowledged() != channel {
		t.Fatal("acknowledgment changed event-copy channel identity")
	}
	select {
	case <-copied.Acknowledged():
	default:
		t.Fatal("event copy did not observe original acknowledgment")
	}
	select {
	case success := <-done:
		if !success {
			t.Fatal("actual producer failed after public acknowledgment")
		}
	case <-time.After(time.Second):
		t.Fatal("actual producer did not consume acknowledgment")
	}
}

func TestIPAssignedAcknowledgedObservesOriginalEmitterAndCopies(t *testing.T) {
	session, events, produce := newAcknowledgedEmitter(t)
	done := produce(func() bool { return session.onNCPOpened(AddressFamilyIPv4) })
	event := waitForEvent(t, events, time.Second)
	assigned, ok := event.(EventSessionIPAssigned)
	if !ok {
		t.Fatalf("actual NCP emitter=%T", event)
	}
	if assigned.Local != session.localIPv4 || assigned.Peer != session.peerIPv4 {
		t.Fatal("NCP emitter lost actual installed address inputs")
	}
	copied := assigned
	assertAcknowledgedReceipt(t, assigned, copied, assigned.Acknowledge, done)
	zero, ok := any(EventSessionIPAssigned{}).(lifecycleAcknowledgedObserver)
	if !ok || zero.Acknowledged() != nil {
		t.Fatal("zero assignment observer must return nil")
	}
}

func TestLCPDownAcknowledgedObservesOriginalEmitterAndCopies(t *testing.T) {
	session, events, produce := newAcknowledgedEmitter(t)
	assignmentDone := produce(func() bool { return session.onNCPOpened(AddressFamilyIPv4) })
	first := waitForEvent(t, events, time.Second)
	assigned, ok := first.(EventSessionIPAssigned)
	if !ok {
		t.Fatalf("actual NCP prerequisite=%T", first)
	}
	assigned.Acknowledge()
	select {
	case success := <-assignmentDone:
		if !success {
			t.Fatal("actual assignment prerequisite failed")
		}
	case <-time.After(time.Second):
		t.Fatal("actual assignment prerequisite did not complete")
	}
	done := produce(func() bool { return session.performAction(LCPActTLD, LCPPacket{}) })
	event := waitForEvent(t, events, time.Second)
	down, ok := event.(EventLCPDown)
	if !ok || !down.NetworkPhase {
		t.Fatalf("actual TLD network withdrawal=%T,%+v", event, event)
	}
	copied := down
	assertAcknowledgedReceipt(t, down, copied, down.Acknowledge, done)
	zero, ok := any(EventLCPDown{}).(lifecycleAcknowledgedObserver)
	if !ok || zero.Acknowledged() != nil {
		t.Fatal("zero withdrawal observer must return nil")
	}
}
