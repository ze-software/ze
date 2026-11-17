// Design: docs/research/l2tpv2-ze-integration.md -- opaque subscriber admission identity

package ppp

import (
	"testing"
	"time"
)

// Distinct opaque subscriber IDs must admit independently even when access
// indices 7 and 65543 have the same legacy 16-bit coordinates. Each admitted
// owner must remain open and answer its own peer's Echo-Request.
func TestDriverDistinctSubscriberIDsShareWireCoordinates(t *testing.T) {
	driver := newSessionTestDriver()
	if err := driver.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Stop)
	first, firstPeer, firstOwner := frameLeaseStart(t, 9)
	first.TunnelID = 7
	first.SubscriberID = "opaque/access-7/session-9"
	firstAttempt, err := NewSessionAttempt(first.SubscriberID, first.Dataplane)
	if err != nil {
		t.Fatal(err)
	}
	first.Attempt = firstAttempt
	frameLeaseProxy(&first)
	driver.SessionsIn() <- first
	drainTwoEvents(t, driver.EventsOut(), time.Second)

	second, secondPeer, secondOwner := frameLeaseStart(t, 9)
	second.TunnelID = 7
	second.SubscriberID = "opaque/access-65543/session-9"
	secondAttempt, err := NewSessionAttempt(second.SubscriberID, second.Dataplane)
	if err != nil {
		t.Fatal(err)
	}
	second.Attempt = secondAttempt
	frameLeaseProxy(&second)
	driver.SessionsIn() <- second
	event := waitForEvent(t, driver.EventsOut(), time.Second)
	if _, opened := event.(EventLCPUp); !opened {
		t.Fatalf("second opaque subscriber event = %T, want independent LCP opening", event)
	}
	event = waitForEvent(t, driver.EventsOut(), time.Second)
	if _, active := event.(EventSessionUp); !active {
		t.Fatalf("second opaque subscriber event = %T, want independent active session", event)
	}
	if firstOwner.closes.Load() != 0 {
		t.Fatal("admitting the second subscriber closed the first owner")
	}
	if secondOwner.closes.Load() != 0 {
		t.Fatal("the second subscriber owner was refused or closed")
	}
	frameLeaseEcho(t, firstPeer)
	frameLeaseEcho(t, secondPeer)
}

// One admitted canonical subscriber ID cannot acquire another owner merely
// because its numeric transport labels changed. Refusal must preserve the
// original live peer and release only the incoming owner.
func TestDriverDuplicateSubscriberIDRejectsDifferentWireCoordinates(t *testing.T) {
	driver := newSessionTestDriver()
	if err := driver.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Stop)
	first, firstPeer, firstOwner := frameLeaseStart(t, 1)
	first.SubscriberID = "opaque/same-admitted-subscriber"
	firstAttempt, err := NewSessionAttempt(first.SubscriberID, first.Dataplane)
	if err != nil {
		t.Fatal(err)
	}
	first.Attempt = firstAttempt
	frameLeaseProxy(&first)
	driver.SessionsIn() <- first
	drainTwoEvents(t, driver.EventsOut(), time.Second)

	second, _, secondOwner := frameLeaseStart(t, 2)
	second.TunnelID = 65535
	second.SubscriberID = first.SubscriberID
	secondAttempt, err := NewSessionAttempt(second.SubscriberID, second.Dataplane)
	if err != nil {
		t.Fatal(err)
	}
	second.Attempt = secondAttempt
	frameLeaseProxy(&second)
	driver.SessionsIn() <- second
	event := waitForEvent(t, driver.EventsOut(), time.Second)
	rejected, ok := event.(EventSessionRejected)
	if !ok {
		t.Fatalf("duplicate canonical subscriber event = %T, want refusal", event)
	}
	if !rejected.ExistingSession {
		t.Fatal("duplicate canonical ID did not identify its admitted predecessor")
	}
	if secondOwner.closes.Load() != 1 {
		t.Fatalf("incoming duplicate owner closes = %d, want one", secondOwner.closes.Load())
	}
	if firstOwner.closes.Load() != 0 {
		t.Fatal("duplicate canonical admission closed the original owner")
	}
	frameLeaseEcho(t, firstPeer)
}
